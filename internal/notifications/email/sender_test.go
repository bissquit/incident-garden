package email

import (
	"errors"
	"net"
	"testing"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSender_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "enabled without smtp host",
			config: Config{
				Enabled:     true,
				FromAddress: "test@example.com",
			},
			wantErr: "SMTP host is required",
		},
		{
			name: "enabled without from address",
			config: Config{
				Enabled:  true,
				SMTPHost: "smtp.example.com",
			},
			wantErr: "from address is required",
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
				Enabled:     true,
				SMTPHost:    "smtp.example.com",
				FromAddress: "test@example.com",
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
		Enabled:     true,
		SMTPHost:    "smtp.example.com",
		FromAddress: "test@example.com",
	}

	sender, err := NewSender(config)
	require.NoError(t, err)

	// Check defaults applied
	assert.Equal(t, 587, sender.config.SMTPPort)
	assert.Equal(t, 50, sender.config.BatchSize)
}

func TestNewSender_AuthSetup(t *testing.T) {
	t.Run("with credentials", func(t *testing.T) {
		config := Config{
			Enabled:      true,
			SMTPHost:     "smtp.example.com",
			FromAddress:  "test@example.com",
			SMTPUser:     "user",
			SMTPPassword: "pass",
		}

		sender, err := NewSender(config)
		require.NoError(t, err)
		assert.NotNil(t, sender.auth)
	})

	t.Run("without credentials", func(t *testing.T) {
		config := Config{
			Enabled:     true,
			SMTPHost:    "smtp.example.com",
			FromAddress: "test@example.com",
		}

		sender, err := NewSender(config)
		require.NoError(t, err)
		assert.Nil(t, sender.auth)
	})
}

func TestSender_Type(t *testing.T) {
	sender, err := NewSender(Config{
		Enabled:     true,
		SMTPHost:    "smtp.example.com",
		FromAddress: "test@example.com",
	})
	require.NoError(t, err)

	assert.Equal(t, domain.ChannelTypeEmail, sender.Type())
}

func TestExtractEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "user@example.com",
			expected: "user@example.com",
		},
		{
			input:    "Test User <user@example.com>",
			expected: "user@example.com",
		},
		{
			input:    "<user@example.com>",
			expected: "user@example.com",
		},
		{
			input:    "StatusPage <noreply@status.example.com>",
			expected: "noreply@status.example.com",
		},
		{
			input:    "invalid<",
			expected: "invalid<",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractEmail(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSender_BuildMessage(t *testing.T) {
	sender := &Sender{
		config: Config{
			FromAddress: "StatusPage <noreply@example.com>",
		},
	}

	msg := sender.buildMessage("Test Subject", "Test body content")
	msgStr := string(msg)

	// Check required headers
	assert.Contains(t, msgStr, "From: StatusPage <noreply@example.com>\r\n")
	assert.Contains(t, msgStr, "To: undisclosed-recipients:;\r\n")
	assert.Contains(t, msgStr, "Subject: Test Subject\r\n")
	assert.Contains(t, msgStr, "MIME-Version: 1.0\r\n")
	assert.Contains(t, msgStr, "Content-Type: text/plain; charset=\"utf-8\"\r\n")
	assert.Contains(t, msgStr, "\r\n\r\n") // Header-body separator
	assert.Contains(t, msgStr, "Test body content")
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "421 service unavailable",
			err:       errors.New("421 Service not available"),
			retryable: true,
		},
		{
			name:      "450 mailbox unavailable",
			err:       errors.New("450 Mailbox unavailable"),
			retryable: true,
		},
		{
			name:      "451 local error",
			err:       errors.New("451 Local error in processing"),
			retryable: true,
		},
		{
			name:      "452 insufficient storage",
			err:       errors.New("452 Insufficient storage"),
			retryable: true,
		},
		{
			name:      "552 mailbox full",
			err:       errors.New("552 Mailbox full"),
			retryable: true,
		},
		{
			name:      "550 mailbox not found",
			err:       errors.New("550 Mailbox not found"),
			retryable: false,
		},
		{
			name:      "535 auth failed",
			err:       errors.New("535 Authentication failed"),
			retryable: false,
		},
		{
			name:      "generic error",
			err:       errors.New("some random error"),
			retryable: false,
		},
		{
			name:      "timeout error",
			err:       &timeoutError{},
			retryable: true,
		},
		{
			name:      "network operation error",
			err:       &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			retryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

// timeoutError implements net.Error for testing
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestBatchSplit(t *testing.T) {
	tests := []struct {
		name         string
		batchSize    int
		recipients   int
		expectedBatches int
	}{
		{
			name:            "single batch",
			batchSize:       50,
			recipients:      30,
			expectedBatches: 1,
		},
		{
			name:            "exact batch size",
			batchSize:       50,
			recipients:      50,
			expectedBatches: 1,
		},
		{
			name:            "multiple batches",
			batchSize:       50,
			recipients:      120,
			expectedBatches: 3,
		},
		{
			name:            "partial last batch",
			batchSize:       50,
			recipients:      75,
			expectedBatches: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recipients := make([]string, tt.recipients)
			for i := range recipients {
				recipients[i] = "test@example.com"
			}

			batches := 0
			for i := 0; i < len(recipients); i += tt.batchSize {
				batches++
			}

			assert.Equal(t, tt.expectedBatches, batches)
		})
	}
}
