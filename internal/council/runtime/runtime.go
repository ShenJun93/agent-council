package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/preflight"
	"github.com/ShenJun93/agent-council/internal/council/visibility"
)

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
	RunID       string
	RunRoot     string
	Participant string
	Role        string
	Phase       string
	Prompt      string
	Workdir     string
	Timeout     time.Duration
	Env         map[string]string
}

type AgentResponse struct {
	Provider   Provider
	Stdout     string
	Stderr     string
	ExitCode   int
	Attempts   int
	StartedAt  time.Time
	FinishedAt time.Time
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
	authArgs  []string
	runArgs   func(AgentRequest) []string
	checkAuth func(string) error
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
		authArgs: []string{"auth", "status", "--json"},
		runArgs: func(req AgentRequest) []string {
			return []string{"-p", req.Prompt, "--output-format", "text", "--permission-mode", "plan"}
		},
		checkAuth: func(stdout string) error {
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
		authArgs: []string{"login", "status"},
		runArgs: func(req AgentRequest) []string {
			return []string{"exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", req.Prompt}
		},
		checkAuth: preflight.ValidateCodexAuth,
	}
}

func (r *cliRuntime) Run(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if err := validateWorkdir(req); err != nil {
		return AgentResponse{}, &RunError{Class: FailureIsolation, Err: err}
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
	if err := r.checkAuth(auth.Stdout); err != nil {
		return AgentResponse{}, &RunError{Class: FailureAuth, Err: err}
	}

	for attempt := 1; attempt <= 2; attempt++ {
		result := r.runner.Run(runCtx, processSpec{
			Command: r.binary,
			Args:    r.runArgs(req),
			Dir:     req.Workdir,
			Env:     safeEnv,
		})
		response := AgentResponse{
			Provider:   r.provider,
			Stdout:     result.Stdout,
			Stderr:     result.Stderr,
			ExitCode:   result.ExitCode,
			Attempts:   attempt,
			StartedAt:  result.StartedAt,
			FinishedAt: result.FinishedAt,
		}
		if result.Err == nil && result.ExitCode == 0 {
			return response, nil
		}

		class := classifyFailure(result.Err, result.Stdout, result.Stderr)
		if class != FailureProcess || attempt == 2 {
			return response, &RunError{Class: class, Err: processError("agent execution", result)}
		}
	}

	return AgentResponse{}, &RunError{Class: FailureProcess, Err: errors.New("unreachable runtime state")}
}

func validateWorkdir(req AgentRequest) error {
	if strings.TrimSpace(req.Workdir) == "" {
		return fmt.Errorf("isolated workdir is required")
	}
	info, err := os.Stat(req.Workdir)
	if err != nil {
		return fmt.Errorf("stat isolated workdir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("isolated workdir %q is not a directory", req.Workdir)
	}
	if strings.TrimSpace(req.RunRoot) == "" {
		return nil
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
