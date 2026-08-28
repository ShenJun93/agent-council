package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/preflight"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

const claudeCouncilSystemPrompt = "You are an Agent Council participant. Use only the context in the user prompt. Do not access files, tools, plugins, skills, MCP servers, browsers, or prior sessions."

const codexCouncilFilesystemProfile = `permissions.council.filesystem={":root"="deny",":minimal"="read",":workspace_roots"={"."="read"}}`

type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
)

type FailureClass string

const (
	FailureTimeout                FailureClass = "timeout"
	FailureQuotaExhausted         FailureClass = "quota_exhausted"
	FailureAuth                   FailureClass = "auth_failure"
	FailureBillingPolicyViolation FailureClass = "billing_policy_violation"
	FailureIsolation              FailureClass = "isolation_failure"
	FailureProcess                FailureClass = "process_failure"
	FailureMalformedOutput        FailureClass = "malformed_output"
	FailureAdapterUnavailable     FailureClass = "adapter_unavailable"
	FailureAdapterPoolExhausted   FailureClass = "adapter_pool_exhausted"
)

type RunError struct {
	Class FailureClass
	Err   error
}

func (e *RunError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Class)
	}
	return fmt.Sprintf("%s: %v", e.Class, e.Err)
}

func (e *RunError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type AgentRequest struct {
	RunID           string
	RunRoot         string
	Participant     string
	Role            string
	Phase           string
	Prompt          string
	Workdir         string
	Timeout         time.Duration
	Env             map[string]string
	OutputSchema    json.RawMessage
	MaxAttempts     int
	SlotID          string
	AdapterID       string
	FailoverIndex   int
	FailoverTrigger FailureClass
}

type AgentResponse struct {
	Provider        Provider
	AdapterID       string
	SlotID          string
	FailoverIndex   int
	FailoverTrigger FailureClass
	Stdout          string
	Stderr          string
	ExitCode        int
	Attempts        int
	StartedAt       time.Time
	FinishedAt      time.Time
}

type AgentRuntime interface {
	Run(ctx context.Context, req AgentRequest) (AgentResponse, error)
}

type processSpec struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
}

type processResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
	Err        error
}

type processRunner interface {
	Run(ctx context.Context, spec processSpec) processResult
}

type osProcessRunner struct{}

func (osProcessRunner) Run(ctx context.Context, spec processSpec) processResult {
	started := time.Now().UTC()
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	finished := time.Now().UTC()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
	}

	return processResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		StartedAt:  started,
		FinishedAt: finished,
		Err:        err,
	}
}

type cliRuntime struct {
	provider  Provider
	binary    string
	runner    processRunner
	environ   func() []string
	goos      string
	lookPath  func(string) (string, error)
	authArgs  []string
	runArgs   func(AgentRequest, string) []string
	checkAuth func(stdout, stderr string) error
}

func NewClaudeCLI(binary string) AgentRuntime {
	return newClaudeCLI(binary, osProcessRunner{}, os.Environ)
}

func NewCodexCLI(binary string) AgentRuntime {
	return newCodexCLI(binary, osProcessRunner{}, os.Environ)
}

func newClaudeCLI(binary string, runner processRunner, environ func() []string) AgentRuntime {
	if strings.TrimSpace(binary) == "" {
		binary = "claude"
	}
	return &cliRuntime{
		provider: ProviderClaude,
		binary:   binary,
		runner:   runner,
		environ:  environ,
		goos:     goruntime.GOOS,
		lookPath: exec.LookPath,
		authArgs: []string{"auth", "status", "--json"},
		runArgs: func(req AgentRequest, _ string) []string {
			args := []string{
				"--setting-sources", "",
				"--settings", `{"autoMemoryEnabled":false}`,
				"--tools", "",
				"--strict-mcp-config",
				"--disallowedTools", "mcp__*",
				"--disable-slash-commands",
				"--no-session-persistence",
				"--no-chrome",
				"--system-prompt", claudeCouncilSystemPrompt,
				"-p", req.Prompt,
			}
			outputFormat := "text"
			if len(req.OutputSchema) > 0 {
				args = append(args, "--json-schema", string(req.OutputSchema))
				outputFormat = "json"
			}
			return append(args, "--output-format", outputFormat, "--permission-mode", "plan")
		},
		checkAuth: func(stdout, _ string) error {
			return preflight.ValidateClaudeAuth([]byte(stdout))
		},
	}
}

func newCodexCLI(binary string, runner processRunner, environ func() []string) AgentRuntime {
	if strings.TrimSpace(binary) == "" {
		binary = "codex"
	}
	return &cliRuntime{
		provider: ProviderCodex,
		binary:   binary,
		runner:   runner,
		environ:  environ,
		goos:     goruntime.GOOS,
		lookPath: exec.LookPath,
		authArgs: []string{"login", "status"},
		runArgs: func(req AgentRequest, schemaPath string) []string {
			args := []string{
				"exec",
				"--ephemeral",
				"--skip-git-repo-check",
				"--ignore-user-config",
				"--ignore-rules",
				"--strict-config",
				"--disable", "shell_tool",
				"--disable", "code_mode",
				"--disable", "code_mode_host",
				"--disable", "apps",
				"--disable", "plugins",
				"--disable", "multi_agent",
				"--disable", "tool_suggest",
				"-c", `skills.include_instructions=false`,
				"-c", `skills.bundled.enabled=false`,
				"-c", `project_doc_max_bytes=0`,
				"-c", `default_permissions="council"`,
				"-c", codexCouncilFilesystemProfile,
				"-c", `agents.enabled=false`,
			}
			if schemaPath != "" {
				args = append(args, "--output-schema", schemaPath)
			}
			return append(args, req.Prompt)
		},
		checkAuth: func(stdout, stderr string) error {
			return preflight.ValidateCodexAuth(stdout + "\n" + stderr)
		},
	}
}

func (r *cliRuntime) Run(ctx context.Context, req AgentRequest) (response AgentResponse, runErr error) {
	if err := validateWorkdir(req); err != nil {
		return AgentResponse{}, &RunError{Class: FailureIsolation, Err: err}
	}
	if r.provider == ProviderCodex && strings.EqualFold(r.goos, "windows") {
		return AgentResponse{}, &RunError{
			Class: FailureIsolation,
			Err:   errors.New("native Windows Codex cannot guarantee Agent Council host-context isolation; run Agent Council and Codex inside WSL2/Linux"),
		}
	}
	if r.provider == ProviderCodex && strings.EqualFold(r.goos, "linux") && r.lookPath != nil {
		if resolved, err := r.lookPath(r.binary); err == nil && looksLikeWindowsInteropExecutable(resolved) {
			return AgentResponse{}, &RunError{
				Class: FailureIsolation,
				Err:   fmt.Errorf("codex resolves to Windows interop executable %q; install and use a native Linux Codex binary inside WSL2", resolved),
			}
		}
	}

	normalizedSchema, err := normalizeOutputSchema(req.OutputSchema)
	if err != nil {
		return AgentResponse{}, &RunError{Class: FailureProcess, Err: err}
	}
	req.OutputSchema = normalizedSchema
	schemaPath := ""
	if r.provider == ProviderCodex && len(normalizedSchema) > 0 {
		var cleanupSchema func() error
		schemaPath, cleanupSchema, err = materializeOutputSchema(req, normalizedSchema)
		if err != nil {
			return AgentResponse{}, &RunError{Class: FailureIsolation, Err: err}
		}
		defer func() {
			if cleanupErr := cleanupSchema(); cleanupErr != nil {
				isolationErr := &RunError{Class: FailureIsolation, Err: fmt.Errorf("clean output schema: %w", cleanupErr)}
				if runErr == nil {
					runErr = isolationErr
				} else {
					runErr = errors.Join(runErr, isolationErr)
				}
			}
		}()
	}

	parentEnv := r.environ()
	if err := preflight.CheckSubscriptionEnvironment(parentEnv); err != nil {
		return AgentResponse{}, &RunError{Class: FailureBillingPolicyViolation, Err: err}
	}

	safeEnv, err := preflight.SafeEnvironment(parentEnv, req.Env)
	if err != nil {
		class := FailureProcess
		if errors.Is(err, preflight.ErrBillingPolicyViolation) {
			class = FailureBillingPolicyViolation
		}
		return AgentResponse{}, &RunError{Class: class, Err: err}
	}

	runCtx, cancel := requestContext(ctx, req.Timeout)
	defer cancel()

	auth := r.runner.Run(runCtx, processSpec{
		Command: r.binary,
		Args:    append([]string(nil), r.authArgs...),
		Dir:     req.Workdir,
		Env:     safeEnv,
	})
	if auth.Err != nil || auth.ExitCode != 0 {
		class := classifyFailure(auth.Err, auth.Stdout, auth.Stderr)
		if class == FailureProcess {
			class = FailureAuth
		}
		return AgentResponse{}, &RunError{Class: class, Err: processError("auth preflight", auth)}
	}
	if err := r.checkAuth(auth.Stdout, auth.Stderr); err != nil {
		return AgentResponse{}, &RunError{Class: FailureAuth, Err: err}
	}

	executionEnv := safeEnv
	if r.provider == ProviderCodex {
		var cleanup func() error
		executionEnv, cleanup, err = prepareCodexExecutionEnvironment(parentEnv, safeEnv, req)
		if err != nil {
			class := FailureIsolation
			switch {
			case errors.Is(err, preflight.ErrBillingPolicyViolation):
				class = FailureBillingPolicyViolation
			case errors.Is(err, preflight.ErrAuthFailure):
				class = FailureAuth
			}
			return AgentResponse{}, &RunError{Class: class, Err: err}
		}
		defer func() {
			if cleanupErr := cleanup(); cleanupErr != nil {
				isolationErr := &RunError{Class: FailureIsolation, Err: fmt.Errorf("clean isolated Codex runtime home: %w", cleanupErr)}
				if runErr == nil {
					runErr = isolationErr
				} else {
					runErr = errors.Join(runErr, isolationErr)
				}
			}
		}()
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 2
	}
	if maxAttempts < 1 || maxAttempts > 2 {
		return AgentResponse{}, &RunError{Class: FailureProcess, Err: fmt.Errorf("max attempts must be 1 or 2, got %d", maxAttempts)}
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result := r.runner.Run(runCtx, processSpec{
			Command: r.binary,
			Args:    r.runArgs(req, schemaPath),
			Dir:     req.Workdir,
			Env:     executionEnv,
		})
		attemptResponse := AgentResponse{
			Provider:   r.provider,
			Stdout:     result.Stdout,
			Stderr:     result.Stderr,
			ExitCode:   result.ExitCode,
			Attempts:   attempt,
			StartedAt:  result.StartedAt,
			FinishedAt: result.FinishedAt,
		}
		if result.Err == nil && result.ExitCode == 0 {
			return attemptResponse, nil
		}

		class := classifyFailure(result.Err, result.Stdout, result.Stderr)
		if class != FailureProcess || attempt == maxAttempts {
			return attemptResponse, &RunError{Class: class, Err: processError("agent execution", result)}
		}
	}

	return AgentResponse{}, &RunError{Class: FailureProcess, Err: errors.New("unreachable runtime state")}
}

func normalizeOutputSchema(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, fmt.Errorf("output schema must be one JSON object: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("output schema must be one JSON object")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, fmt.Errorf("compact output schema: %w", err)
	}
	return json.RawMessage(compact.Bytes()), nil
}

func materializeOutputSchema(req AgentRequest, schema json.RawMessage) (string, func() error, error) {
	dir, err := os.MkdirTemp("", "agent-council-output-schema-*")
	if err != nil {
		return "", nil, fmt.Errorf("create output schema directory: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	for label, root := range map[string]string{"run root": req.RunRoot, "workspace": req.Workdir} {
		inside, checkErr := visibility.IsWithin(root, dir)
		if checkErr != nil {
			_ = cleanup()
			return "", nil, fmt.Errorf("validate output schema against %s: %w", label, checkErr)
		}
		if inside {
			_ = cleanup()
			return "", nil, fmt.Errorf("output schema directory must be outside %s", label)
		}
	}
	path := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(path, schema, 0o600); err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("write output schema: %w", err)
	}
	return path, cleanup, nil
}

func looksLikeWindowsInteropExecutable(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(path), `\`, "/"))
	if !strings.HasPrefix(normalized, "/mnt/") {
		return false
	}
	return strings.Contains(normalized, "/appdata/roaming/npm/") ||
		strings.HasSuffix(normalized, ".exe") ||
		strings.HasSuffix(normalized, ".cmd") ||
		strings.HasSuffix(normalized, ".bat")
}

func validateWorkdir(req AgentRequest) error {
	if strings.TrimSpace(req.Workdir) == "" {
		return fmt.Errorf("isolated workdir is required")
	}
	if strings.TrimSpace(req.RunRoot) == "" {
		return fmt.Errorf("full run root is required for isolation validation")
	}
	info, err := os.Stat(req.Workdir)
	if err != nil {
		return fmt.Errorf("stat isolated workdir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("isolated workdir %q is not a directory", req.Workdir)
	}
	inside, err := visibility.IsWithin(req.RunRoot, req.Workdir)
	if err != nil {
		return fmt.Errorf("validate workdir against run root: %w", err)
	}
	if inside {
		return fmt.Errorf("workdir %q must be outside full run root %q", req.Workdir, req.RunRoot)
	}
	return nil
}

func requestContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func processError(stage string, result processResult) error {
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if result.Err != nil && message != "" {
		return fmt.Errorf("%s: %w: %s", stage, result.Err, message)
	}
	if result.Err != nil {
		return fmt.Errorf("%s: %w", stage, result.Err)
	}
	if message != "" {
		return fmt.Errorf("%s: exit code %d: %s", stage, result.ExitCode, message)
	}
	return fmt.Errorf("%s: exit code %d", stage, result.ExitCode)
}

func classifyFailure(err error, stdout, stderr string) FailureClass {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return FailureTimeout
	}

	text := strings.ToLower(stdout + "\n" + stderr)
	for _, marker := range []string{
		"quota exhausted",
		"quota exceeded",
		"usage limit",
		"rate limit",
		"rate_limit",
		"too many requests",
	} {
		if strings.Contains(text, marker) {
			return FailureQuotaExhausted
		}
	}
	for _, marker := range []string{
		"401",
		"unauthorized",
		"authentication failed",
		"not logged in",
		"please login",
		"please log in",
		"invalid token",
	} {
		if strings.Contains(text, marker) {
			return FailureAuth
		}
	}
	return FailureProcess
}
