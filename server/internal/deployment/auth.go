package deployment

import (
	"fmt"
	"strconv"
)

// EmailVerificationPolicy is startup policy, never a caller-supplied flag.
// Community requires verification unless the operator explicitly disables it.
func EmailVerificationPolicy(raw string) (bool, error) {
	if raw == "" {
		return true, nil
	}
	required, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("AUTH_EMAIL_VERIFICATION_REQUIRED must be true or false")
	}
	if !required && !AllowUnverifiedSignup {
		return false, fmt.Errorf("EE requires email verification")
	}
	return required, nil
}
