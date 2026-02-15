package notifications

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorker_CalculateNextAttempt(t *testing.T) {
	config := WorkerConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}

	worker := &Worker{config: config}

	tests := []struct {
		name            string
		attempt         int
		expectedBackoff time.Duration
	}{
		{"first retry", 1, 1 * time.Second},
		{"second retry", 2, 2 * time.Second},
		{"third retry", 3, 4 * time.Second},
		{"fourth retry", 4, 8 * time.Second},
		{"fifth retry", 5, 16 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			result := worker.calculateNextAttempt(tt.attempt)
			after := time.Now()

			// Result should be between now+expectedBackoff and after+expectedBackoff
			expectedMin := before.Add(tt.expectedBackoff)
			expectedMax := after.Add(tt.expectedBackoff)

			assert.True(t, result.After(expectedMin) || result.Equal(expectedMin),
				"result %v should be >= %v", result, expectedMin)
			assert.True(t, result.Before(expectedMax) || result.Equal(expectedMax),
				"result %v should be <= %v", result, expectedMax)
		})
	}
}

func TestWorker_CalculateNextAttempt_MaxBackoff(t *testing.T) {
	config := WorkerConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
	}

	worker := &Worker{config: config}

	// After many attempts, backoff should be capped at MaxBackoff
	before := time.Now()
	result := worker.calculateNextAttempt(100)

	expectedBackoff := config.MaxBackoff
	expectedMin := before.Add(expectedBackoff)

	assert.True(t, result.After(expectedMin) || result.Equal(expectedMin),
		"result should be at least %v after now", expectedBackoff)

	// Should not exceed MaxBackoff significantly
	expectedMax := time.Now().Add(expectedBackoff + time.Second)
	assert.True(t, result.Before(expectedMax),
		"result should not exceed MaxBackoff")
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable error",
			err:      NewRetryableError(errors.New("temporary error")),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      NewNonRetryableError(errors.New("permanent error")),
			expected: false,
		},
		{
			name:     "generic error defaults to retryable",
			err:      errors.New("unknown error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryableError(t *testing.T) {
	originalErr := errors.New("original error")

	t.Run("retryable error", func(t *testing.T) {
		err := NewRetryableError(originalErr)

		assert.Equal(t, "original error", err.Error())
		assert.True(t, err.IsRetryable())
		assert.Equal(t, originalErr, errors.Unwrap(err))
	})

	t.Run("non-retryable error", func(t *testing.T) {
		err := NewNonRetryableError(originalErr)

		assert.Equal(t, "original error", err.Error())
		assert.False(t, err.IsRetryable())
		assert.Equal(t, originalErr, errors.Unwrap(err))
	})
}

func TestDefaultWorkerConfig(t *testing.T) {
	config := DefaultWorkerConfig()

	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 5*time.Second, config.PollInterval)
	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialBackoff)
	assert.Equal(t, 5*time.Minute, config.MaxBackoff)
	assert.Equal(t, 2.0, config.BackoffMultiplier)
	assert.Equal(t, 5, config.NumWorkers)
}
