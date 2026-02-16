//go:build integration

package integration

import (
	"context"
	"sync"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/notifications"
)

// SentNotification represents a notification that was sent via mock sender.
type SentNotification struct {
	To        string
	Subject   string
	Body      string
	SentAt    time.Time
	ChannelType domain.ChannelType
}

// MockSender is a test implementation of notifications.Sender.
type MockSender struct {
	mu          sync.Mutex
	channelType domain.ChannelType
	sent        []SentNotification
	failNext    bool
	failErr     error
	failCount   int // Number of times to fail before succeeding
	callCount   int
}

// NewMockSender creates a new mock sender for the given channel type.
func NewMockSender(channelType domain.ChannelType) *MockSender {
	return &MockSender{
		channelType: channelType,
		sent:        make([]SentNotification, 0),
	}
}

// Send implements notifications.Sender.
func (m *MockSender) Send(_ context.Context, n notifications.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++

	if m.failCount > 0 {
		m.failCount--
		return m.failErr
	}

	if m.failNext {
		m.failNext = false
		return m.failErr
	}

	m.sent = append(m.sent, SentNotification{
		To:          n.To,
		Subject:     n.Subject,
		Body:        n.Body,
		SentAt:      time.Now(),
		ChannelType: m.channelType,
	})

	return nil
}

// Type implements notifications.Sender.
func (m *MockSender) Type() domain.ChannelType {
	return m.channelType
}

// GetSent returns a copy of sent notifications.
func (m *MockSender) GetSent() []SentNotification {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]SentNotification, len(m.sent))
	copy(result, m.sent)
	return result
}

// SentCount returns the number of sent notifications.
func (m *MockSender) SentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// CallCount returns the total number of Send calls.
func (m *MockSender) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Reset clears all sent notifications and resets state.
func (m *MockSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = make([]SentNotification, 0)
	m.failNext = false
	m.failErr = nil
	m.failCount = 0
	m.callCount = 0
}

// FailNext makes the next Send call fail with the given error.
func (m *MockSender) FailNext(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext = true
	m.failErr = err
}

// FailNextN makes the next N Send calls fail with the given error.
func (m *MockSender) FailNextN(n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount = n
	m.failErr = err
}

// WaitForNotifications waits until at least n notifications are sent or timeout.
func (m *MockSender) WaitForNotifications(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.SentCount() >= n {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return m.SentCount() >= n
}

// MockSenderRegistry holds mock senders for all channel types.
type MockSenderRegistry struct {
	Email      *MockSender
	Telegram   *MockSender
	Mattermost *MockSender
}

// NewMockSenderRegistry creates a new registry with mock senders.
func NewMockSenderRegistry() *MockSenderRegistry {
	return &MockSenderRegistry{
		Email:      NewMockSender(domain.ChannelTypeEmail),
		Telegram:   NewMockSender(domain.ChannelTypeTelegram),
		Mattermost: NewMockSender(domain.ChannelTypeMattermost),
	}
}

// GetSenders returns all mock senders as a slice.
func (r *MockSenderRegistry) GetSenders() []notifications.Sender {
	return []notifications.Sender{r.Email, r.Telegram, r.Mattermost}
}

// Reset resets all mock senders.
func (r *MockSenderRegistry) Reset() {
	r.Email.Reset()
	r.Telegram.Reset()
	r.Mattermost.Reset()
}

// TotalSentCount returns total sent count across all senders.
func (r *MockSenderRegistry) TotalSentCount() int {
	return r.Email.SentCount() + r.Telegram.SentCount() + r.Mattermost.SentCount()
}

// WaitForAnyNotification waits until any notification is sent.
func (r *MockSenderRegistry) WaitForAnyNotification(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.TotalSentCount() > 0 {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return r.TotalSentCount() > 0
}

// WaitForNotifications waits until at least n notifications are sent total.
func (r *MockSenderRegistry) WaitForNotifications(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.TotalSentCount() >= n {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return r.TotalSentCount() >= n
}
