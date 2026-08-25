package preflight

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrBillingPolicyViolation = errors.New("billing policy violation")
	ErrAuthFailure            = errors.New("authentication failure")
)

var blockedEnvironment = map[string]struct{}{
	"ANTHROPIC_API_KEY":       {},
	"ANTHROPIC_AUTH_TOKEN":    {},
	"OPENAI_API_KEY":          {},
	"CODEX_API_KEY":           {},
	"CLAUDE_CODE_OAUTH_TOKEN": {},
}

var allowedEnvironment = map[string]struct{}{
	"PATH":                      {},
	"HOME":                      {},
	"USER":                      {},
	"LOGNAME":                   {},
	"SHELL":                     {},
	"TMPDIR":                    {},
	"TMP":                       {},
	"TEMP":                      {},
	"LANG":                      {},
	"LC_ALL":                    {},
	"LC_CTYPE":                  {},
	"TZ":                        {},
	"USERPROFILE":               {},
	"APPDATA":                   {},
	"LOCALAPPDATA":              {},
	"PROGRAMDATA":               {},
	"SYSTEMROOT":                {},
	"WINDIR":                    {},
	"COMSPEC":                   {},
	"PATHEXT":                   {},
	"XDG_CONFIG_HOME":           {},
	"XDG_DATA_HOME":             {},
	"XDG_CACHE_HOME":            {},
	"SSL_CERT_FILE":             {},
	"SSL_CERT_DIR":              {},
	"NODE_EXTRA_CA_CERTS":       {},
	"CLAUDE_CODE_GIT_BASH_PATH": {},
}

func CheckSubscriptionEnvironment(environ []string) error {
	for _, entry := range environ {
		key, value, ok := splitEnvironment(entry)
		if !ok || value == "" {
			continue
		}
		if _, blocked := blockedEnvironment[strings.ToUpper(key)]; blocked {
			return fmt.Errorf("%w: metered credential %s is present", ErrBillingPolicyViolation, key)
		}
	}
	return nil
}

func SafeEnvironment(parent []string, overrides map[string]string) ([]string, error) {
	values := make(map[string]string)
	keys := make(map[string]string)

	for _, entry := range parent {
		key, value, ok := splitEnvironment(entry)
		if !ok {
			continue
		}
		normalized := strings.ToUpper(key)
		if _, allowed := allowedEnvironment[normalized]; !allowed {
			continue
		}
		values[normalized] = value
		keys[normalized] = key
	}

	for key, value := range overrides {
		normalized := strings.ToUpper(strings.TrimSpace(key))
		if _, blocked := blockedEnvironment[normalized]; blocked {
			return nil, fmt.Errorf("%w: override %s is not allowed", ErrBillingPolicyViolation, key)
		}
		if _, allowed := allowedEnvironment[normalized]; !allowed {
			return nil, fmt.Errorf("environment override %s is not allowlisted", key)
		}
		values[normalized] = value
		keys[normalized] = key
	}

	names := make([]string, 0, len(values))
	for normalized := range values {
		names = append(names, normalized)
	}
	sort.Strings(names)

	result := make([]string, 0, len(names))
	for _, normalized := range names {
		result = append(result, keys[normalized]+"="+values[normalized])
	}
	return result, nil
}

type claudeAuthStatus struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	APIProvider      string `json:"apiProvider"`
	SubscriptionType string `json:"subscriptionType"`
}

func ValidateClaudeAuth(data []byte) error {
	var status claudeAuthStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("%w: malformed Claude auth status: %v", ErrAuthFailure, err)
	}
	if !status.LoggedIn {
		return fmt.Errorf("%w: Claude is not logged in", ErrAuthFailure)
	}
	if status.AuthMethod != "claude.ai" || status.APIProvider != "firstParty" || strings.TrimSpace(status.SubscriptionType) == "" {
		return fmt.Errorf("%w: Claude is not using a first-party subscription login", ErrAuthFailure)
	}
	return nil
}

func ValidateCodexAuth(text string) error {
	if strings.TrimSpace(text) != "Logged in using ChatGPT" {
		return fmt.Errorf("%w: Codex is not using ChatGPT login", ErrAuthFailure)
	}
	return nil
}

func splitEnvironment(entry string) (string, string, bool) {
	parts := strings.SplitN(entry, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
