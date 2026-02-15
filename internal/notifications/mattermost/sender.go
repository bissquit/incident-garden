// Package mattermost provides Mattermost notification sending via Incoming Webhooks.
package mattermost

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
)

const (
	defaultTimeout  = 10 * time.Second
	defaultUsername = "StatusPage"
)

// Config holds Mattermost sender configuration.
// Note: webhook URL is stored in notification_channel.target,
// so global configuration is minimal.
// Unlike Email/Telegram, there is no Enabled flag - Mattermost sender
// is always available since webhook URL is configured per-channel.
type Config struct {
	DefaultUsername string        // username for display, default "StatusPage"
	DefaultIconURL  string        // icon URL (optional)
	Timeout         time.Duration // request timeout
}

// Sender implements Mattermost notification sender via Incoming Webhooks.
type Sender struct {
	config     Config
	httpClient *http.Client
}

// NewSender creates a new Mattermost sender.
func NewSender(config Config) *Sender {
	if config.DefaultUsername == "" {
		config.DefaultUsername = defaultUsername
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}

	return &Sender{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Type returns the channel type.
func (s *Sender) Type() domain.ChannelType {
	return domain.ChannelTypeMattermost
}

// Send sends a notification to Mattermost.
// notification.To contains the webhook URL.
func (s *Sender) Send(ctx context.Context, notification notifications.Notification) error {
	webhookURL := notification.To
	if webhookURL == "" {
		return &PermanentError{Message: "webhook URL is empty"}
	}

	payload := webhookPayload{
		Username: s.config.DefaultUsername,
	}

	if s.config.DefaultIconURL != "" {
		payload.IconURL = s.config.DefaultIconURL
	}

	// If subject is provided, add as markdown heading
	if notification.Subject != "" {
		payload.Text = fmt.Sprintf("### %s\n\n%s", notification.Subject, notification.Body)
	} else {
		payload.Text = notification.Body
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return &RetryableError{Message: fmt.Sprintf("send request: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	return s.handleResponse(resp, webhookURL)
}

type webhookPayload struct {
	Text     string `json:"text"`
	Username string `json:"username,omitempty"`
	IconURL  string `json:"icon_url,omitempty"`
}

func (s *Sender) handleResponse(resp *http.Response, webhookURL string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		slog.Debug("mattermost message sent", "webhook", maskWebhookURL(webhookURL))
		return nil

	case http.StatusBadRequest:
		return &PermanentError{
			Code:    resp.StatusCode,
			Message: fmt.Sprintf("bad request: %s", string(body)),
		}

	case http.StatusUnauthorized, http.StatusForbidden:
		return &PermanentError{
			Code:    resp.StatusCode,
			Message: "invalid or expired webhook",
		}

	case http.StatusNotFound:
		return &PermanentError{
			Code:    resp.StatusCode,
			Message: "webhook not found",
		}

	case http.StatusTooManyRequests:
		return &RetryableError{
			Code:    resp.StatusCode,
			Message: "rate limited",
		}

	default:
		if resp.StatusCode >= 500 {
			return &RetryableError{
				Code:    resp.StatusCode,
				Message: fmt.Sprintf("server error: %s", string(body)),
			}
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// maskWebhookURL hides part of the URL for logging.
func maskWebhookURL(url string) string {
	if len(url) > 40 {
		return url[:20] + "..." + url[len(url)-10:]
	}
	return url
}

// PermanentError indicates a permanent error that should not be retried.
type PermanentError struct {
	Code    int
	Message string
}

func (e *PermanentError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("mattermost error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("mattermost error: %s", e.Message)
}

// IsRetryable returns false as permanent errors should not be retried.
func (e *PermanentError) IsRetryable() bool { return false }

// RetryableError indicates a temporary error that can be retried.
type RetryableError struct {
	Code    int
	Message string
}

func (e *RetryableError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("mattermost error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("mattermost error: %s", e.Message)
}

// IsRetryable returns true as these errors are temporary.
func (e *RetryableError) IsRetryable() bool { return true }
