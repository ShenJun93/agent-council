package preflight

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckSubscriptionEnvironmentRejectsMeteredCredentials(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_AUTH_TOKEN",
		"OPENAI_API_KEY",
		"CODEX_API_KEY",
		"CLAUDE_CODE_OAUTH_TOKEN",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := CheckSubscriptionEnvironment([]string{"PATH=/usr/bin", name + "=secret"})
			if !errors.Is(err, ErrBillingPolicyViolation) {
				t.Fatalf("CheckSubscriptionEnvironment() error = %v, want billing policy violation", err)
			}
		})
	}
}

func TestSafeEnvironmentUsesExplicitAllowlist(t *testing.T) {
	t.Parallel()

	env, err := SafeEnvironment([]string{
		"PATH=/usr/bin",
		"HOME=/home/council",
		"LANG=en_US.UTF-8",
		"RANDOM_SECRET=do-not-inherit",
		"OPENAI_API_KEY=do-not-inherit",
	}, map[string]string{"TMPDIR": "/tmp/council"})
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(env, "\n")
	for _, want := range []string{"PATH=/usr/bin", "HOME=/home/council", "LANG=en_US.UTF-8", "TMPDIR=/tmp/council"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("safe env missing %q: %q", want, joined)
		}
	}
	for _, denied := range []string{"RANDOM_SECRET=", "OPENAI_API_KEY="} {
		if strings.Contains(joined, denied) {
			t.Fatalf("safe env leaked %q: %q", denied, joined)
		}
	}
}

func TestSafeEnvironmentRejectsUnknownOverride(t *testing.T) {
	t.Parallel()

	if _, err := SafeEnvironment(nil, map[string]string{"MY_SECRET": "x"}); err == nil {
		t.Fatal("SafeEnvironment() accepted unknown override")
	}
}

func TestValidateClaudeAuthRequiresFirstPartySubscription(t *testing.T) {
	t.Parallel()

	good := []byte(`{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"pro"}`)
	if err := ValidateClaudeAuth(good); err != nil {
		t.Fatalf("ValidateClaudeAuth(valid) = %v", err)
	}

	bad := []byte(`{"loggedIn":true,"authMethod":"apiKey","apiProvider":"firstParty","subscriptionType":""}`)
	if err := ValidateClaudeAuth(bad); !errors.Is(err, ErrAuthFailure) {
		t.Fatalf("ValidateClaudeAuth(api key) = %v, want auth failure", err)
	}
}

func TestValidateCodexAuthRequiresChatGPTLogin(t *testing.T) {
	t.Parallel()

	if err := ValidateCodexAuth("Logged in using ChatGPT\n"); err != nil {
		t.Fatalf("ValidateCodexAuth(ChatGPT) = %v", err)
	}
	if err := ValidateCodexAuth("Logged in using an API key\n"); !errors.Is(err, ErrAuthFailure) {
		t.Fatalf("ValidateCodexAuth(API key) = %v, want auth failure", err)
	}
}
