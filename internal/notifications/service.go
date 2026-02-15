package notifications

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Verification constants.
const (
	verificationCodeLength  = 6
	verificationCodeTTL     = 24 * time.Hour
	maxVerificationAttempts = 5
	resendCooldown          = 60 * time.Second
)

// Service errors.
var (
	ErrChannelNotOwned = errChannelNotOwned
)

var errChannelNotOwned = errorString("channel does not belong to user")

type errorString string

func (e errorString) Error() string { return string(e) }

// ServiceValidator validates service IDs exist.
type ServiceValidator interface {
	ValidateServicesExist(ctx context.Context, ids []string) (missingIDs []string, err error)
}

// Service provides notifications business logic.
type Service struct {
	repo             Repository
	dispatcher       *Dispatcher
	serviceValidator ServiceValidator
}

// NewService creates a new notifications service.
func NewService(repo Repository, dispatcher *Dispatcher, serviceValidator ServiceValidator) *Service {
	return &Service{
		repo:             repo,
		dispatcher:       dispatcher,
		serviceValidator: serviceValidator,
	}
}

// CreateChannel creates a new notification channel for user.
func (s *Service) CreateChannel(ctx context.Context, userID string, channelType domain.ChannelType, target string) (*domain.NotificationChannel, error) {
	channel := &domain.NotificationChannel{
		UserID:                 userID,
		Type:                   channelType,
		Target:                 target,
		IsEnabled:              true,
		IsVerified:             false,
		SubscribeToAllServices: false,
	}

	if err := s.repo.CreateChannel(ctx, channel); err != nil {
		return nil, err
	}

	// For email channels, send verification code
	if channelType == domain.ChannelTypeEmail {
		if err := s.sendVerificationCode(ctx, channel); err != nil {
			slog.Error("failed to send verification code", "channel_id", channel.ID, "error", err)
			// Don't return error - channel is created, code can be resent
		}
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

// VerifyChannel verifies a channel with the provided code.
func (s *Service) VerifyChannel(ctx context.Context, userID, channelID, inputCode string) (*domain.NotificationChannel, error) {
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return nil, err
	}

	if channel.UserID != userID {
		return nil, ErrChannelNotOwned
	}

	if channel.IsVerified {
		return channel, nil // Already verified
	}

	// For email - verify with code
	if channel.Type == domain.ChannelTypeEmail {
		return s.verifyEmailCode(ctx, channel, inputCode)
	}

	// For Telegram/Mattermost - send test message (to be implemented separately)
	return s.verifyByTestMessage(ctx, channel)
}

// verifyEmailCode verifies email channel with the provided code.
func (s *Service) verifyEmailCode(ctx context.Context, channel *domain.NotificationChannel, inputCode string) (*domain.NotificationChannel, error) {
	storedCode, err := s.repo.GetVerificationCode(ctx, channel.ID)
	if err != nil {
		return nil, err
	}

	// Check attempt limit
	if storedCode.Attempts >= maxVerificationAttempts {
		return nil, ErrTooManyAttempts
	}

	// Increment attempt counter
	if err := s.repo.IncrementCodeAttempts(ctx, channel.ID); err != nil {
		slog.Error("failed to increment attempts", "error", err)
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(inputCode), []byte(storedCode.Code)) != 1 {
		return nil, ErrVerificationCodeInvalid
	}

	// Code is valid - verify the channel
	channel.IsVerified = true
	if err := s.repo.UpdateChannel(ctx, channel); err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}

	// Delete used code
	_ = s.repo.DeleteVerificationCode(ctx, channel.ID)

	slog.Info("channel verified", "channel_id", channel.ID)
	return channel, nil
}

// verifyByTestMessage verifies non-email channels by sending a test message.
func (s *Service) verifyByTestMessage(ctx context.Context, channel *domain.NotificationChannel) (*domain.NotificationChannel, error) {
	if s.dispatcher == nil {
		return nil, errors.New("dispatcher not configured")
	}

	sender, ok := s.dispatcher.senders[channel.Type]
	if !ok {
		return nil, fmt.Errorf("no sender for channel type: %s", channel.Type)
	}

	// Send test message
	notification := Notification{
		To:      channel.Target,
		Subject: "Channel Verification",
		Body:    "This is a test message to verify your notification channel. If you received this message, your channel is working correctly.",
	}

	if err := sender.Send(ctx, notification); err != nil {
		return nil, fmt.Errorf("send test message: %w", err)
	}

	// Mark as verified
	channel.IsVerified = true
	if err := s.repo.UpdateChannel(ctx, channel); err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}

	slog.Info("channel verified via test message", "channel_id", channel.ID, "type", channel.Type)
	return channel, nil
}

// ResendVerificationCode sends a new verification code for email channels.
func (s *Service) ResendVerificationCode(ctx context.Context, userID, channelID string) error {
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return err
	}

	if channel.UserID != userID {
		return ErrChannelNotOwned
	}

	if channel.IsVerified {
		return ErrChannelAlreadyVerified
	}

	if channel.Type != domain.ChannelTypeEmail {
		return ErrResendNotSupported
	}

	// Check cooldown
	existingCode, err := s.repo.GetVerificationCode(ctx, channel.ID)
	if err == nil && time.Since(existingCode.CreatedAt) < resendCooldown {
		return ErrResendTooSoon
	}

	return s.sendVerificationCode(ctx, channel)
}

// sendVerificationCode generates and sends a verification code.
func (s *Service) sendVerificationCode(ctx context.Context, channel *domain.NotificationChannel) error {
	code := generateVerificationCode()
	expiresAt := time.Now().Add(verificationCodeTTL)

	// Save code to DB
	if err := s.repo.CreateVerificationCode(ctx, channel.ID, code, expiresAt); err != nil {
		return fmt.Errorf("create verification code: %w", err)
	}

	// Build and send email
	subject := "Verify your email for StatusPage"
	body := fmt.Sprintf(`Your verification code is: %s

This code will expire in 24 hours.

If you did not request this code, please ignore this email.`, code)

	notification := Notification{
		To:      channel.Target,
		Subject: subject,
		Body:    body,
	}

	if s.dispatcher == nil {
		slog.Warn("dispatcher not configured, verification email not sent", "channel_id", channel.ID)
		return nil
	}

	emailSender, ok := s.dispatcher.senders[domain.ChannelTypeEmail]
	if !ok {
		return errors.New("email sender not configured")
	}

	if err := emailSender.Send(ctx, notification); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	slog.Info("verification code sent", "channel_id", channel.ID)
	return nil
}

// generateVerificationCode generates a cryptographically secure 6-digit code.
func generateVerificationCode() string {
	var code [verificationCodeLength]byte
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		code[i] = byte('0' + n.Int64())
	}
	return string(code[:])
}

// NotifySubscribers sends notifications about an event.
// Returns nil if notifications are disabled (dispatcher is nil).
func (s *Service) NotifySubscribers(ctx context.Context, serviceIDs []string, subject, body string) error {
	if s.dispatcher == nil {
		return nil
	}
	return s.dispatcher.Dispatch(ctx, DispatchInput{
		ServiceIDs: serviceIDs,
		Subject:    subject,
		Body:       body,
	})
}

// SubscriptionsMatrix represents all channels with their subscription settings.
type SubscriptionsMatrix struct {
	Channels []ChannelWithSubscriptions `json:"channels"`
}

// GetSubscriptionsMatrix returns all channels with their subscription settings for a user.
func (s *Service) GetSubscriptionsMatrix(ctx context.Context, userID string) (*SubscriptionsMatrix, error) {
	channelsWithSubs, err := s.repo.GetUserChannelsWithSubscriptions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get channels with subscriptions: %w", err)
	}

	return &SubscriptionsMatrix{
		Channels: channelsWithSubs,
	}, nil
}

// SetChannelSubscriptions sets subscription settings for a channel.
func (s *Service) SetChannelSubscriptions(ctx context.Context, userID, channelID string, subscribeAll bool, serviceIDs []string) error {
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return err
	}

	if channel.UserID != userID {
		return ErrChannelNotOwned
	}

	if !channel.IsVerified {
		return ErrChannelNotVerified
	}

	// Validate service IDs if not subscribing to all
	if !subscribeAll && len(serviceIDs) > 0 {
		if s.serviceValidator == nil {
			return errors.New("service validator not configured")
		}
		missingIDs, err := s.serviceValidator.ValidateServicesExist(ctx, serviceIDs)
		if err != nil {
			return fmt.Errorf("validate services: %w", err)
		}
		if len(missingIDs) > 0 {
			return ErrServicesNotFound
		}
	}

	return s.repo.SetChannelSubscriptions(ctx, channelID, subscribeAll, serviceIDs)
}

// GetChannelSubscriptions returns subscription settings for a channel.
func (s *Service) GetChannelSubscriptions(ctx context.Context, channelID string) (bool, []string, error) {
	return s.repo.GetChannelSubscriptions(ctx, channelID)
}
