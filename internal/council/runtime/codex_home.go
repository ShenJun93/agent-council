package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ShenJun93/agent-council/internal/council/preflight"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

const maxCodexAuthFileBytes int64 = 2 << 20

type codexAuthEnvelope struct {
	AuthMode     string `json:"auth_mode"`
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
}

func prepareCodexAuthEnvironment(parentEnv, safeEnv []string) []string {
	if value := strings.TrimSpace(environmentValues(parentEnv)["CODEX_HOME"]); value != "" {
		return overrideEnvironment(safeEnv, map[string]string{"CODEX_HOME": value})
	}
	return safeEnv
}

func prepareCodexExecutionEnvironment(parentEnv, safeEnv []string, req AgentRequest) ([]string, func() error, error) {
	sourceHome, err := sourceCodexHome(parentEnv)
	if err != nil {
		return nil, nil, err
	}
	auth, err := readMirrorableCodexAuth(filepath.Join(sourceHome, "auth.json"))
	if err != nil {
		return nil, nil, err
	}

	root, err := os.MkdirTemp("", "agent-council-codex-exec-")
	if err != nil {
		return nil, nil, fmt.Errorf("create isolated Codex runtime root: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(root) }
	fail := func(cause error) ([]string, func() error, error) {
		_ = cleanup()
		return nil, nil, cause
	}

	if overlap, overlapErr := pathTreesOverlap(root, req.RunRoot); overlapErr != nil {
		return fail(fmt.Errorf("validate Codex home against run root: %w", overlapErr))
	} else if overlap {
		return fail(fmt.Errorf("isolated Codex home overlaps full run root"))
	}
	if overlap, overlapErr := pathTreesOverlap(root, req.Workdir); overlapErr != nil {
		return fail(fmt.Errorf("validate Codex home against participant workspace: %w", overlapErr))
	} else if overlap {
		return fail(fmt.Errorf("isolated Codex home overlaps participant workspace"))
	}

	codexHome := filepath.Join(root, "codex-home")
	userHome := filepath.Join(root, "user-home")
	tempHome := filepath.Join(root, "tmp")
	appData := filepath.Join(userHome, "AppData", "Roaming")
	localAppData := filepath.Join(userHome, "AppData", "Local")
	xdgConfig := filepath.Join(userHome, ".config")
	xdgData := filepath.Join(userHome, ".local", "share")
	xdgCache := filepath.Join(userHome, ".cache")

	for _, dir := range []string{codexHome, userHome, tempHome, appData, localAppData, xdgConfig, xdgData, xdgCache} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fail(fmt.Errorf("create isolated Codex directory: %w", err))
		}
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), auth, 0o600); err != nil {
		return fail(fmt.Errorf("mirror ChatGPT auth into isolated Codex home: %w", err))
	}

	executionEnv := overrideEnvironment(safeEnv, map[string]string{
		"CODEX_HOME":      codexHome,
		"HOME":            userHome,
		"USERPROFILE":     userHome,
		"APPDATA":         appData,
		"LOCALAPPDATA":    localAppData,
		"XDG_CONFIG_HOME": xdgConfig,
		"XDG_DATA_HOME":   xdgData,
		"XDG_CACHE_HOME":  xdgCache,
		"TMPDIR":          tempHome,
		"TMP":             tempHome,
		"TEMP":            tempHome,
	})
	return executionEnv, cleanup, nil
}

func sourceCodexHome(environ []string) (string, error) {
	values := environmentValues(environ)
	if value := strings.TrimSpace(values["CODEX_HOME"]); value != "" {
		return filepath.Abs(value)
	}
	if value := strings.TrimSpace(values["HOME"]); value != "" {
		return filepath.Abs(filepath.Join(value, ".codex"))
	}
	if value := strings.TrimSpace(values["USERPROFILE"]); value != "" {
		return filepath.Abs(filepath.Join(value, ".codex"))
	}
	return "", fmt.Errorf("cannot isolate Codex auth: ambient Codex home is unknown")
}

func readMirrorableCodexAuth(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot isolate Codex auth: file-backed ChatGPT auth is unavailable")
		}
		return nil, fmt.Errorf("inspect Codex auth for isolation: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("cannot isolate Codex auth: auth.json must be a regular file")
	}
	if info.Size() > maxCodexAuthFileBytes {
		return nil, fmt.Errorf("cannot isolate Codex auth: auth.json exceeds size limit")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Codex auth for isolation: %w", err)
	}
	var envelope codexAuthEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("cannot isolate Codex auth: malformed auth.json")
	}
	if strings.TrimSpace(envelope.OpenAIAPIKey) != "" {
		return nil, fmt.Errorf("%w: metered credential is present in Codex auth storage", preflight.ErrBillingPolicyViolation)
	}
	if mode := strings.TrimSpace(envelope.AuthMode); mode != "" && !strings.EqualFold(mode, "chatgpt") {
		return nil, fmt.Errorf("%w: isolated Codex auth is not ChatGPT login", preflight.ErrAuthFailure)
	}
	return data, nil
}

func pathTreesOverlap(a, b string) (bool, error) {
	aWithinB, err := visibility.IsWithin(b, a)
	if err != nil {
		return false, err
	}
	bWithinA, err := visibility.IsWithin(a, b)
	if err != nil {
		return false, err
	}
	return aWithinB || bWithinA, nil
}

func environmentValues(environ []string) map[string]string {
	values := make(map[string]string)
	for _, entry := range environ {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		values[strings.ToUpper(strings.TrimSpace(parts[0]))] = parts[1]
	}
	return values
}

func overrideEnvironment(base []string, overrides map[string]string) []string {
	values := environmentValues(base)
	for key, value := range overrides {
		values[strings.ToUpper(strings.TrimSpace(key))] = value
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}
