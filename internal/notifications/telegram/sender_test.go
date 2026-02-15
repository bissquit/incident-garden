package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestNewSender_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "enabled without bot token",
			config: Config{
				Enabled: true,
			},
			wantErr: "bot token is required",
		},
		{
			name: "disabled - no validation",
			config: Config{
				Enabled: false,
			},
			wantErr: "",
		},
		{
			name: "valid config",
			config: Config{
				Enabled:  true,
				BotToken: "123456:ABC-DEF",
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, err := NewSender(tt.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, sender)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, sender)
			}
		})
	}
}

func TestNewSender_Defaults(t *testing.T) {
	config := Config{
		Enabled:  true,
		BotToken: "test-token",
	}

	sender, err := NewSender(config)
	require.NoError(t, err)

	// Rate limiter should use default rate
	assert.NotNil(t, sender.limiter)
	assert.NotNil(t, sender.httpClient)
	assert.Equal(t, defaultAPIURL, sender.apiURL)
}

func TestNewSender_CustomRateLimit(t *testing.T) {
	config := Config{
		Enabled:   true,
		BotToken:  "test-token",
		RateLimit: 10.0,
	}

	sender, err := NewSender(config)
	require.NoError(t, err)
	assert.NotNil(t, sender.limiter)
}

func TestSender_Type(t *testing.T) {
	sender, err := NewSender(Config{
		Enabled:  true,
		BotToken: "test-token",
	})
	require.NoError(t, err)

	assert.Equal(t, domain.ChannelTypeTelegram, sender.Type())
}

func TestSender_Send_Disabled(t *testing.T) {
	sender, err := NewSender(Config{
		Enabled: false,
	})
	require.NoError(t, err)

	err = sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})
	assert.NoError(t, err)
}

func TestSender_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req sendMessageRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "123456789", req.ChatID)
		assert.Equal(t, "Test message", req.Text)
		assert.Equal(t, "HTML", req.ParseMode)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(telegramResponse{OK: true})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})
	assert.NoError(t, err)
}

func TestSender_Send_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   429,
			Description: "Too Many Requests: retry after 30",
			Parameters: &struct {
				RetryAfter int `json:"retry_after,omitempty"`
			}{
				RetryAfter: 30,
			},
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})

	require.Error(t, err)
	var rateLimitErr *RateLimitError
	require.ErrorAs(t, err, &rateLimitErr)
	assert.Equal(t, 30*time.Second, rateLimitErr.RetryAfter)
	assert.True(t, rateLimitErr.IsRetryable())
}

func TestSender_Send_ChatNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   404,
			Description: "Bad Request: chat not found",
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "999999999",
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, 404, permErr.Code)
	assert.Contains(t, permErr.Message, "chat not found")
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_BotBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   403,
			Description: "Forbidden: bot was blocked by the user",
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, 403, permErr.Code)
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_InvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   401,
			Description: "Unauthorized",
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "invalid-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, 401, permErr.Code)
	assert.Contains(t, permErr.Message, "invalid bot token")
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   400,
			Description: "Bad Request: message text is empty",
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, 400, permErr.Code)
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   500,
			Description: "Internal Server Error",
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})

	require.Error(t, err)
	var retryErr *RetryableError
	require.ErrorAs(t, err, &retryErr)
	assert.Equal(t, 500, retryErr.Code)
	assert.True(t, retryErr.IsRetryable())
}

func TestSender_Send_ContextCancellation(t *testing.T) {
	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: http.DefaultClient,
		limiter:    rate.NewLimiter(0.001, 1), // Very slow rate
		apiURL:     "http://localhost:12345/%s/sendMessage",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := sender.Send(ctx, notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit wait")
}

func TestRateLimitError_IsRetryable(t *testing.T) {
	err := &RateLimitError{
		RetryAfter: 30 * time.Second,
		Message:    "Too Many Requests",
	}

	assert.True(t, err.IsRetryable())
	assert.True(t, IsRetryable(err))
	assert.Equal(t, 30*time.Second, GetRetryAfter(err))
}

func TestPermanentError_IsRetryable(t *testing.T) {
	err := &PermanentError{
		Code:    403,
		Message: "Bot blocked",
	}

	assert.False(t, err.IsRetryable())
	assert.False(t, IsRetryable(err))
	assert.Equal(t, time.Duration(0), GetRetryAfter(err))
}

func TestRetryableError_IsRetryable(t *testing.T) {
	err := &RetryableError{
		Code:    500,
		Message: "Internal error",
	}

	assert.True(t, err.IsRetryable())
	assert.True(t, IsRetryable(err))
}

func TestIsRetryable_GenericError(t *testing.T) {
	err := assert.AnError
	assert.False(t, IsRetryable(err))
}

func TestIsRetryable_Nil(t *testing.T) {
	assert.False(t, IsRetryable(nil))
}

func TestGetRetryAfter_NonRateLimitError(t *testing.T) {
	err := &PermanentError{Code: 400, Message: "Bad request"}
	assert.Equal(t, time.Duration(0), GetRetryAfter(err))
}

func TestErrorMessages(t *testing.T) {
	t.Run("RateLimitError", func(t *testing.T) {
		err := &RateLimitError{
			RetryAfter: 30 * time.Second,
			Message:    "Too Many Requests",
		}
		assert.Contains(t, err.Error(), "rate limited")
		assert.Contains(t, err.Error(), "30s")
		assert.Contains(t, err.Error(), "Too Many Requests")
	})

	t.Run("PermanentError", func(t *testing.T) {
		err := &PermanentError{
			Code:    403,
			Message: "Bot blocked",
		}
		assert.Contains(t, err.Error(), "telegram error 403")
		assert.Contains(t, err.Error(), "Bot blocked")
	})

	t.Run("RetryableError", func(t *testing.T) {
		err := &RetryableError{
			Code:    500,
			Message: "Internal error",
		}
		assert.Contains(t, err.Error(), "telegram error 500")
		assert.Contains(t, err.Error(), "Internal error")
	})
}

func TestSender_RateLimiter(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(telegramResponse{OK: true})
	}))
	defer server.Close()

	// Create sender with very high rate limit for testing
	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Limit(1000), 100),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	// Send multiple messages
	for i := 0; i < 5; i++ {
		err := sender.Send(context.Background(), notifications.Notification{
			To:   "123456789",
			Body: "Test message",
		})
		require.NoError(t, err)
	}

	assert.Equal(t, 5, callCount)
}

func TestSender_Send_RateLimitWithoutRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(telegramResponse{
			OK:          false,
			ErrorCode:   429,
			Description: "Too Many Requests",
			// No Parameters field
		})
	}))
	defer server.Close()

	sender := &Sender{
		config:     Config{Enabled: true, BotToken: "test-token"},
		httpClient: server.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		apiURL:     server.URL + "/%s/sendMessage",
	}

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "123456789",
		Body: "Test message",
	})

	require.Error(t, err)
	var rateLimitErr *RateLimitError
	require.ErrorAs(t, err, &rateLimitErr)
	// Default retry after should be 1 second
	assert.Equal(t, 1*time.Second, rateLimitErr.RetryAfter)
}
