package notifications

import "errors"

// Repository errors.
var (
	ErrChannelNotFound      = errors.New("notification channel not found")
	ErrChannelAlreadyExists = errors.New("channel with this target already exists")
)

// Verification errors.
var (
	ErrVerificationCodeNotFound = errors.New("verification code not found or expired")
	ErrVerificationCodeInvalid  = errors.New("invalid verification code")
	ErrTooManyAttempts          = errors.New("too many verification attempts")
	ErrResendTooSoon            = errors.New("please wait before requesting a new code")
	ErrChannelAlreadyVerified   = errors.New("channel already verified")
	ErrResendNotSupported       = errors.New("resend only available for email channels")
)

// Subscription errors.
var (
	ErrChannelNotVerified = errors.New("channel must be verified to manage subscriptions")
	ErrServicesNotFound   = errors.New("one or more services not found")
)

// Deletion errors.
var (
	ErrCannotDeleteDefaultChannel = errors.New("cannot delete default channel")
)

// Channel type errors.
var (
	ErrChannelTypeDisabled = errors.New("channel type is not available")
)
