package notifications

import "errors"

// Repository errors.
var (
	ErrChannelNotFound = errors.New("notification channel not found")
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
