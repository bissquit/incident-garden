package notifications

import (
	"crypto/subtle"
	"regexp"
	"testing"
)

var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

func TestGenerateVerificationCode(t *testing.T) {
	// Test that generated code is exactly 6 digits
	for i := 0; i < 100; i++ {
		code := generateVerificationCode()
		if len(code) != 6 {
			t.Errorf("expected code length 6, got %d: %s", len(code), code)
		}

		// Verify all characters are digits
		if !codePattern.MatchString(code) {
			t.Errorf("code does not match digit pattern: %s", code)
		}
	}
}

func TestGenerateVerificationCode_Uniqueness(t *testing.T) {
	// Generate multiple codes and verify they're not all the same
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := generateVerificationCode()
		codes[code] = true
	}

	// With 6 digits, probability of 100 identical codes is negligible
	// We should have at least 50 unique codes
	if len(codes) < 50 {
		t.Errorf("expected at least 50 unique codes out of 100, got %d", len(codes))
	}
}

func TestConstantTimeCompare_Timing(t *testing.T) {
	// This is more of a documentation test showing how we use constant-time comparison
	code1 := "123456"
	code2 := "123456"
	code3 := "654321"

	// Same codes should match
	if subtle.ConstantTimeCompare([]byte(code1), []byte(code2)) != 1 {
		t.Error("identical codes should match")
	}

	// Different codes should not match
	if subtle.ConstantTimeCompare([]byte(code1), []byte(code3)) != 0 {
		t.Error("different codes should not match")
	}

	// Different length codes should not match
	if subtle.ConstantTimeCompare([]byte(code1), []byte("12345")) != 0 {
		t.Error("different length codes should not match")
	}
}
