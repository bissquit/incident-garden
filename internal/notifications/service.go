package notifications

import (
	"context"
	"errors"

	"github.com/bissquit/incident-management/internal/domain"
)

// Service errors.
var (
	ErrChannelNotOwned = errors.New("channel does not belong to user")
)

// Service provides notifications business logic.
type Service struct {
	repo       Repository
	dispatcher *Dispatcher
}

// NewService creates a new notifications service.
func NewService(repo Repository, dispatcher *Dispatcher) *Service {
	return &Service{
		repo:       repo,
		dispatcher: dispatcher,
	}
}

// CreateChannel creates a new notification channel for user.
func (s *Service) CreateChannel(ctx context.Context, userID string, channelType domain.ChannelType, target string) (*domain.NotificationChannel, error) {
	channel := &domain.NotificationChannel{
		UserID:     userID,
		Type:       channelType,
		Target:     target,
		IsEnabled:  true,
		IsVerified: false,
	}

	if err := s.repo.CreateChannel(ctx, channel); err != nil {
		return nil, err
	}

	return channel, nil
}

// ListUserChannels returns all channels for a user.
func (s *Service) ListUserChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error) {
	return s.repo.ListUserChannels(ctx, userID)
}

// UpdateChannel updates a channel (enable/disable).
func (s *Service) UpdateChannel(ctx context.Context, userID, channelID string, isEnabled bool) (*domain.NotificationChannel, error) {
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return nil, err
	}

	if channel.UserID != userID {
		return nil, ErrChannelNotOwned
	}

	channel.IsEnabled = isEnabled

	if err := s.repo.UpdateChannel(ctx, channel); err != nil {
		return nil, err
	}

	return channel, nil
}

// DeleteChannel deletes a notification channel.
func (s *Service) DeleteChannel(ctx context.Context, userID, channelID string) error {
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return err
	}

	if channel.UserID != userID {
		return ErrChannelNotOwned
	}

	return s.repo.DeleteChannel(ctx, channelID)
}

// VerifyChannel marks a channel as verified.
func (s *Service) VerifyChannel(ctx context.Context, userID, channelID string) (*domain.NotificationChannel, error) {
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return nil, err
	}

	if channel.UserID != userID {
		return nil, ErrChannelNotOwned
	}

	channel.IsVerified = true

	if err := s.repo.UpdateChannel(ctx, channel); err != nil {
		return nil, err
	}

	return channel, nil
}

// GetOrCreateSubscription gets or creates a subscription for user.
func (s *Service) GetOrCreateSubscription(ctx context.Context, userID string) (*domain.Subscription, error) {
	sub, err := s.repo.GetUserSubscription(ctx, userID)
	if err == nil {
		return sub, nil
	}

	if !errors.Is(err, ErrSubscriptionNotFound) {
		return nil, err
	}

	sub = &domain.Subscription{
		UserID:     userID,
		ServiceIDs: []string{},
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// UpdateSubscriptionServices updates the services a user is subscribed to.
func (s *Service) UpdateSubscriptionServices(ctx context.Context, userID string, serviceIDs []string) (*domain.Subscription, error) {
	sub, err := s.GetOrCreateSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SetSubscriptionServices(ctx, sub.ID, serviceIDs); err != nil {
		return nil, err
	}

	sub.ServiceIDs = serviceIDs
	return sub, nil
}

// DeleteSubscription removes user's subscription.
func (s *Service) DeleteSubscription(ctx context.Context, userID string) error {
	sub, err := s.repo.GetUserSubscription(ctx, userID)
	if err != nil {
		return err
	}

	return s.repo.DeleteSubscription(ctx, sub.ID)
}

// NotifySubscribers sends notifications about an event.
func (s *Service) NotifySubscribers(ctx context.Context, serviceIDs []string, subject, body string) error {
	return s.dispatcher.Dispatch(ctx, DispatchInput{
		ServiceIDs: serviceIDs,
		Subject:    subject,
		Body:       body,
	})
}
