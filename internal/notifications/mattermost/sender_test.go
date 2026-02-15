package mattermost

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
)

func TestNewSender_Defaults(t *testing.T) {
	sender := NewSender(Config{})

	assert.Equal(t, defaultUsername, sender.config.DefaultUsername)
	assert.Equal(t, defaultTimeout, sender.config.Timeout)
	assert.NotNil(t, sender.httpClient)
}

func TestNewSender_CustomConfig(t *testing.T) {
	config := Config{
		DefaultUsername: "CustomBot",
		DefaultIconURL:  "https://example.com/icon.png",
		Timeout:         30 * time.Second,
	}

	sender := NewSender(config)

	assert.Equal(t, "CustomBot", sender.config.DefaultUsername)
	assert.Equal(t, "https://example.com/icon.png", sender.config.DefaultIconURL)
	assert.Equal(t, 30*time.Second, sender.config.Timeout)
}

func TestSender_Type(t *testing.T) {
	sender := NewSender(Config{})
	assert.Equal(t, domain.ChannelTypeMattermost, sender.Type())
}

func TestSender_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload webhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "Test message", payload.Text)
		assert.Equal(t, "StatusPage", payload.Username)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	assert.NoError(t, err)
}

func TestSender_Send_WithSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload webhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		assert.Equal(t, "### Incident Alert\n\nService is down", payload.Text)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:      server.URL,
		Subject: "Incident Alert",
		Body:    "Service is down",
	})

	assert.NoError(t, err)
}

func TestSender_Send_WithIconURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload webhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com/icon.png", payload.IconURL)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(Config{
		DefaultIconURL: "https://example.com/icon.png",
	})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	assert.NoError(t, err)
}

func TestSender_Send_EmptyWebhook(t *testing.T) {
	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   "",
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Contains(t, permErr.Message, "webhook URL is empty")
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid payload"))
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, http.StatusBadRequest, permErr.Code)
	assert.Contains(t, permErr.Message, "bad request")
	assert.Contains(t, permErr.Message, "invalid payload")
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, http.StatusUnauthorized, permErr.Code)
	assert.Contains(t, permErr.Message, "invalid or expired webhook")
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, http.StatusForbidden, permErr.Code)
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var permErr *PermanentError
	require.ErrorAs(t, err, &permErr)
	assert.Equal(t, http.StatusNotFound, permErr.Code)
	assert.Contains(t, permErr.Message, "webhook not found")
	assert.False(t, permErr.IsRetryable())
}

func TestSender_Send_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var retryErr *RetryableError
	require.ErrorAs(t, err, &retryErr)
	assert.Equal(t, http.StatusTooManyRequests, retryErr.Code)
	assert.Contains(t, retryErr.Message, "rate limited")
	assert.True(t, retryErr.IsRetryable())
}

func TestSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var retryErr *RetryableError
	require.ErrorAs(t, err, &retryErr)
	assert.Equal(t, http.StatusInternalServerError, retryErr.Code)
	assert.Contains(t, retryErr.Message, "server error")
	assert.True(t, retryErr.IsRetryable())
}

func TestSender_Send_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	var retryErr *RetryableError
	require.ErrorAs(t, err, &retryErr)
	assert.Equal(t, http.StatusServiceUnavailable, retryErr.Code)
	assert.True(t, retryErr.IsRetryable())
}

func TestSender_Send_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
		_, _ = w.Write([]byte("I'm a teapot"))
	}))
	defer server.Close()

	sender := NewSender(Config{})
	err := sender.Send(context.Background(), notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 418")
	assert.Contains(t, err.Error(), "I'm a teapot")
}

func TestSender_Send_NetworkError(t *testing.T) {
	sender := NewSender(Config{
		Timeout: 100 * time.Millisecond,
	})

	err := sender.Send(context.Background(), notifications.Notification{
		To:   "http://localhost:59999", // Non-existent server
		Body: "Test message",
	})

	require.Error(t, err)
	var retryErr *RetryableError
	require.ErrorAs(t, err, &retryErr)
	assert.Contains(t, retryErr.Message, "send request")
	assert.True(t, retryErr.IsRetryable())
}

func TestMaskWebhookURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "short URL under 40 chars",
			url:      "http://example.com/hook",
			expected: "http://example.com/hook",
		},
		{
			name:     "exactly 40 chars - not masked",
			url:      "http://example.com/hooks/abcdefghijklmno", // 40 chars
			expected: "http://example.com/hooks/abcdefghijklmno",
		},
		{
			name:     "41 chars - gets masked",
			url:      "http://example.com/hooks/abcdefghijklmnop", // 41 chars
			expected: "http://example.com/h...ghijklmnop",
		},
		{
			name:     "long URL",
			url:      "https://mattermost.example.com/hooks/abc123def456ghi789jkl012mno345pqr678stu901vwx234",
			expected: "https://mattermost.e...u901vwx234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskWebhookURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPermanentError(t *testing.T) {
	t.Run("with code", func(t *testing.T) {
		err := &PermanentError{
			Code:    400,
			Message: "bad request",
		}
		assert.Equal(t, "mattermost error 400: bad request", err.Error())
		assert.False(t, err.IsRetryable())
	})

	t.Run("without code", func(t *testing.T) {
		err := &PermanentError{
			Message: "webhook URL is empty",
		}
		assert.Equal(t, "mattermost error: webhook URL is empty", err.Error())
		assert.False(t, err.IsRetryable())
	})
}

func TestRetryableError(t *testing.T) {
	t.Run("with code", func(t *testing.T) {
		err := &RetryableError{
			Code:    500,
			Message: "server error",
		}
		assert.Equal(t, "mattermost error 500: server error", err.Error())
		assert.True(t, err.IsRetryable())
	})

	t.Run("without code", func(t *testing.T) {
		err := &RetryableError{
			Message: "connection refused",
		}
		assert.Equal(t, "mattermost error: connection refused", err.Error())
		assert.True(t, err.IsRetryable())
	})
}

func TestSender_Send_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := sender.Send(ctx, notifications.Notification{
		To:   server.URL,
		Body: "Test message",
	})

	require.Error(t, err)
	// Should be a retryable error since it's a network/context issue
	var retryErr *RetryableError
	require.ErrorAs(t, err, &retryErr)
	assert.True(t, retryErr.IsRetryable())
}
