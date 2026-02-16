package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
)

// WorkerConfig contains worker configuration.
type WorkerConfig struct {
	BatchSize         int
	PollInterval      time.Duration
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	NumWorkers        int
}

// DefaultWorkerConfig returns default worker configuration.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize:         100,
		PollInterval:      5 * time.Second,
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
		NumWorkers:        5,
	}
}

// Worker processes notifications from the queue.
type Worker struct {
	config     WorkerConfig
	repo       Repository
	dispatcher *Dispatcher
	renderer   *Renderer

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewWorker creates a new notification worker.
func NewWorker(config WorkerConfig, repo Repository, dispatcher *Dispatcher, renderer *Renderer) *Worker {
	return &Worker{
		config:     config,
		repo:       repo,
		dispatcher: dispatcher,
		renderer:   renderer,
		stopCh:     make(chan struct{}),
	}
}

// Start launches worker goroutines.
func (w *Worker) Start(ctx context.Context) {
	slog.Info("starting notification worker",
		"workers", w.config.NumWorkers,
		"batch_size", w.config.BatchSize,
		"poll_interval", w.config.PollInterval,
	)

	for i := 0; i < w.config.NumWorkers; i++ {
		w.wg.Add(1)
		go w.run(ctx, i)
	}
}

// Stop gracefully stops all workers.
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	slog.Info("notification worker stopped")
}

func (w *Worker) run(ctx context.Context, workerID int) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processBatch(ctx, workerID)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context, workerID int) {
	items, err := w.repo.FetchPendingNotifications(ctx, w.config.BatchSize)
	if err != nil {
		slog.Error("failed to fetch pending notifications", "worker", workerID, "error", err)
		return
	}

	if len(items) == 0 {
		return
	}

	slog.Debug("processing notifications", "worker", workerID, "count", len(items))
	recordQueueProcessed(len(items))

	for _, item := range items {
		w.processItem(ctx, item)
	}
}

func (w *Worker) processItem(ctx context.Context, item *QueueItem) {
	start := time.Now()

	// Get channel info
	channel, err := w.repo.GetChannelByID(ctx, item.ChannelID)
	if err != nil {
		slog.Error("channel not found", "channel_id", item.ChannelID, "error", err)
		if markErr := w.repo.MarkAsFailed(ctx, item.ID, err); markErr != nil {
			slog.Error("failed to mark as failed", "item_id", item.ID, "error", markErr)
		}
		recordNotificationSent("unknown", "failed")
		return
	}

	// Skip unverified channels
	if !channel.IsVerified {
		slog.Debug("skipping unverified channel", "channel_id", item.ChannelID)
		if markErr := w.repo.MarkAsFailed(ctx, item.ID, fmt.Errorf("channel not verified")); markErr != nil {
			slog.Error("failed to mark as failed", "item_id", item.ID, "error", markErr)
		}
		recordNotificationSent(string(channel.Type), "skipped_unverified")
		return
	}

	// Skip disabled channels
	if !channel.IsEnabled {
		slog.Debug("skipping disabled channel", "channel_id", item.ChannelID)
		if markErr := w.repo.MarkAsFailed(ctx, item.ID, fmt.Errorf("channel disabled")); markErr != nil {
			slog.Error("failed to mark as failed", "item_id", item.ID, "error", markErr)
		}
		recordNotificationSent(string(channel.Type), "skipped_disabled")
		return
	}

	// Render message
	subject, body, err := w.renderer.Render(channel.Type, item.Payload)
	if err != nil {
		slog.Error("failed to render", "item_id", item.ID, "error", err)
		if markErr := w.repo.MarkAsFailed(ctx, item.ID, err); markErr != nil {
			slog.Error("failed to mark as failed", "item_id", item.ID, "error", markErr)
		}
		recordNotificationSent(string(channel.Type), "failed")
		return
	}

	// Send notification
	notification := Notification{
		To:      channel.Target,
		Subject: subject,
		Body:    body,
	}

	err = w.dispatcher.SendToChannel(ctx, channel.Type, notification)
	duration := time.Since(start)

	if err != nil {
		w.handleSendError(ctx, item, channel.Type, err)
		return
	}

	// Success
	if err := w.repo.MarkAsSent(ctx, item.ID); err != nil {
		slog.Error("failed to mark as sent", "item_id", item.ID, "error", err)
	}

	recordNotificationSent(string(channel.Type), "success")
	recordNotificationDuration(string(channel.Type), duration)

	slog.Debug("notification sent",
		"item_id", item.ID,
		"channel_type", channel.Type,
		"duration", duration,
	)
}

func (w *Worker) handleSendError(ctx context.Context, item *QueueItem, channelType domain.ChannelType, err error) {
	slog.Warn("send failed",
		"item_id", item.ID,
		"attempt", item.Attempts+1,
		"max_attempts", item.MaxAttempts,
		"error", err,
	)

	// Check if error is retryable
	if !isRetryable(err) {
		if markErr := w.repo.MarkAsFailed(ctx, item.ID, err); markErr != nil {
			slog.Error("failed to mark as failed", "item_id", item.ID, "error", markErr)
		}
		recordNotificationSent(string(channelType), "failed")
		return
	}

	// Check attempt limit
	if item.Attempts+1 >= item.MaxAttempts {
		if markErr := w.repo.MarkAsFailed(ctx, item.ID, fmt.Errorf("max attempts exceeded: %w", err)); markErr != nil {
			slog.Error("failed to mark as failed", "item_id", item.ID, "error", markErr)
		}
		recordNotificationSent(string(channelType), "failed")
		return
	}

	// Schedule retry
	nextAttempt := w.calculateNextAttempt(item.Attempts + 1)
	if markErr := w.repo.MarkForRetry(ctx, item.ID, err, nextAttempt); markErr != nil {
		slog.Error("failed to mark for retry", "item_id", item.ID, "error", markErr)
	}
	recordNotificationSent(string(channelType), "retry")

	slog.Info("notification scheduled for retry",
		"item_id", item.ID,
		"next_attempt", nextAttempt,
	)
}

func (w *Worker) calculateNextAttempt(attempt int) time.Time {
	backoff := float64(w.config.InitialBackoff)
	for i := 1; i < attempt; i++ {
		backoff *= w.config.BackoffMultiplier
	}

	if backoff > float64(w.config.MaxBackoff) {
		backoff = float64(w.config.MaxBackoff)
	}

	return time.Now().Add(time.Duration(backoff))
}

// isRetryable checks if an error is retryable.
func isRetryable(err error) bool {
	type retryable interface {
		IsRetryable() bool
	}
	if r, ok := err.(retryable); ok {
		return r.IsRetryable()
	}

	// Default: retry unknown errors
	return true
}

// RetryableError wraps an error and marks it as retryable or not.
type RetryableError struct {
	Err       error
	Retryable bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// IsRetryable returns whether the error is retryable.
func (e *RetryableError) IsRetryable() bool {
	return e.Retryable
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError creates a retryable error.
func NewRetryableError(err error) *RetryableError {
	return &RetryableError{Err: err, Retryable: true}
}

// NewNonRetryableError creates a non-retryable error.
func NewNonRetryableError(err error) *RetryableError {
	return &RetryableError{Err: err, Retryable: false}
}
