package notifications

import "errors"

// Repository errors.
var (
	ErrChannelNotFound      = errors.New("notification channel not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)
