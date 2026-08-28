package preflight

import (
	"errors"
	"testing"
)

func TestCheckSubscriptionEnvironmentRejectsGoogleMeteredCredentials(t *testing.T) {
	for _, name := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"} {
		t.Run(name, func(t *testing.T) {
			err := CheckSubscriptionEnvironment([]string{"PATH=/usr/bin", name + "=secret"})
			if !errors.Is(err, ErrBillingPolicyViolation) {
				t.Fatalf("CheckSubscriptionEnvironment() error=%v want billing violation", err)
			}
		})
	}
}
