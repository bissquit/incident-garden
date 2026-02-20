// Package telegram provides Telegram notification sending via Bot API.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
	"golang.org/x/time/rate"
)

const (
	defaultAPIURL    = "https://api.telegram.org/bot%s/sendMessage"
	defaultRateLimit = 25.0 // messages per second
	defaultBurstSize = 30
	requestTimeout   = 10 * time.Second
)

// Config holds Telegram sender configuration.
type Config struct {
	Enabled   bool
	BotToken  string
	RateLimit float64 // messages per second, default 25
	APIUrl    string  // custom API URL template, default: https://api.telegram.org/bot%s/sendMessage
}

// Sender implements Telegram notification sender via Bot API.
type Sender struct {
	config     Config
	httpClient *http.Client
	limiter    *rate.Limiter
	apiURL     string // for testing
}

// NewSender creates a new Telegram sender.
// Returns error if enabled but required config is missing.
func NewSender(config Config) (*Sender, error) {
	if config.Enabled {
		if config.BotToken == "" {
			return nil, fmt.Errorf("telegram sender: bot token is required when enabled")
		}
	}

	rateLimit := config.RateLimit
	if rateLimit <= 0 {
		rateLimit = defaultRateLimit
	}

	apiURL := defaultAPIURL
	if config.APIUrl != "" {
		apiURL = config.APIUrl
	}

	slog.Info("telegram sender configured",
		"enabled", config.Enabled,
		"rate_limit", rateLimit,
	)

	return &Sender{
		config: config,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		limiter: rate.NewLimiter(rate.Limit(rateLimit), defaultBurstSize),
		apiURL:  apiURL,
	}, nil
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeTelegram
}

// Send sends a Telegram notification.
func (s *Sender) Send(ctx context.Context, notification notifications.Notification) error {
	if !s.config.Enabled {
		slog.Warn("telegram sender disabled, skipping send",
			"to", notification.To,
		)
		return nil
	}

	// Rate limiting
	if err := s.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}

	payload := sendMessageRequest{
		ChatID:    notification.To,
		Text:      notification.Body,
		ParseMode: "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(s.apiURL, s.config.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return s.handleResponse(resp, notification.To)
}

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after,omitempty"`
	} `json:"parameters,omitempty"`
}

func (s *Sender) handleResponse(resp *http.Response, chatID string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var result telegramResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if result.OK {
		slog.Debug("telegram message sent", "chat_id", chatID)
		return nil
	}

	switch result.ErrorCode {
	case 429: // Too Many Requests
		retryAfter := 1
		if result.Parameters != nil && result.Parameters.RetryAfter > 0 {
			retryAfter = result.Parameters.RetryAfter
		}
		return &RateLimitError{
			RetryAfter: time.Duration(retryAfter) * time.Second,
			Message:    result.Description,
		}

	case 400: // Bad Request (invalid chat_id, etc.)
		return &PermanentError{
			Code:    result.ErrorCode,
			Message: result.Description,
		}

	case 401: // Unauthorized (invalid token)
		return &PermanentError{
			Code:    result.ErrorCode,
			Message: "invalid bot token",
		}

	case 403: // Forbidden (bot blocked by user)
		return &PermanentError{
			Code:    result.ErrorCode,
			Message: result.Description,
		}

	case 404: // Chat not found
		return &PermanentError{
			Code:    result.ErrorCode,
			Message: "chat not found",
		}

	default:
		if resp.StatusCode >= 500 {
			return &RetryableError{
				Code:    result.ErrorCode,
				Message: result.Description,
			}
		}
		return fmt.Errorf("telegram error %d: %s", result.ErrorCode, result.Description)
	}
}

// RateLimitError indicates rate limit was exceeded.
type RateLimitError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v: %s", e.RetryAfter, e.Message)
}

// IsRetryable returns true as rate limit errors are temporary.
func (e *RateLimitError) IsRetryable() bool { return true }

// PermanentError indicates a permanent error that should not be retried.
type PermanentError struct {
	Code    int
	Message string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("telegram error %d: %s", e.Code, e.Message)
}

// IsRetryable returns false as permanent errors should not be retried.
func (e *PermanentError) IsRetryable() bool { return false }

// RetryableError indicates a temporary error that can be retried.
type RetryableError struct {
	Code    int
	Message string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("telegram error %d: %s", e.Code, e.Message)
}

// IsRetryable returns true as these errors are temporary.
func (e *RetryableError) IsRetryable() bool { return true }

// IsRetryable checks if an error can be retried.
func IsRetryable(err error) bool {
	type retryable interface {
		IsRetryable() bool
	}
	if r, ok := err.(retryable); ok {
		return r.IsRetryable()
	}
	return false
}

// GetRetryAfter returns the retry-after duration for rate limit errors.
func GetRetryAfter(err error) time.Duration {
	if rle, ok := err.(*RateLimitError); ok {
		return rle.RetryAfter
	}
	return 0
}
