package deployment

import "testing"

func TestEmailVerificationPolicy(t *testing.T) {
	if required, err := EmailVerificationPolicy(""); err != nil || !required {
		t.Fatal("verification must default to required")
	}
	required, err := EmailVerificationPolicy("false")
	if AllowUnverifiedSignup {
		if err != nil || required {
			t.Fatal("community explicit local policy rejected")
		}
	} else if err == nil {
		t.Fatal("EE policy weakened")
	}
	if _, err := EmailVerificationPolicy("typo"); err == nil {
		t.Fatal("invalid policy accepted")
	}
}
