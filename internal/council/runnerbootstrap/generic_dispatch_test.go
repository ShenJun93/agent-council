package runnerbootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericDispatchCreatesExactlyOneRunAndMarker(t *testing.T) {
	bin, log, state := setupFakeDispatchGH(t, "success")
	cmd := genericDispatchCommand(t, bin, log, state)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dispatch failed: %v\n%s", err, output)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(calls)
	if got := strings.Count(text, "workflow run h5-frozen-execution.yml"); got != 1 {
		t.Fatalf("workflow dispatch count=%d want 1\n%s", got, text)
	}
	if !strings.Contains(text, "[h5-fresh-dispatch-created attempt=2]") {
		t.Fatalf("issue marker missing from gh calls:\n%s", text)
	}
	if !strings.Contains(string(output), "run_id=400") {
		t.Fatalf("new run id missing from output: %s", output)
	}
}
func TestGenericDispatchRejectsBeforeMutation(t *testing.T) {
	for _, scenario := range []string{"marker", "active", "count-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			bin, log, state := setupFakeDispatchGH(t, scenario)
			cmd := genericDispatchCommand(t, bin, log, state)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("%s scenario unexpectedly succeeded: %s", scenario, output)
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "workflow run h5-frozen-execution.yml") {
				t.Fatalf("%s scenario mutated workflow:\n%s", scenario, calls)
			}
		})
	}
}

func TestGenericDispatchDoesNotRetryFailedDispatch(t *testing.T) {
	bin, log, state := setupFakeDispatchGH(t, "dispatch-fail")
	cmd := genericDispatchCommand(t, bin, log, state)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("failed dispatch unexpectedly succeeded: %s", output)
	}
	calls, _ := os.ReadFile(log)
	if got := strings.Count(string(calls), "workflow run h5-frozen-execution.yml"); got != 1 {
		t.Fatalf("dispatch attempts=%d want 1\n%s", got, calls)
	}
}
func TestGenericDispatchRejectsAmbiguousDiscoveryWithoutRedispatch(t *testing.T) {
	bin, log, state := setupFakeDispatchGH(t, "ambiguous")
	cmd := genericDispatchCommand(t, bin, log, state)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("ambiguous discovery unexpectedly succeeded: %s", output)
	}
	calls, _ := os.ReadFile(log)
	text := string(calls)
	if got := strings.Count(text, "workflow run h5-frozen-execution.yml"); got != 1 {
		t.Fatalf("dispatch attempts=%d want 1\n%s", got, text)
	}
	if strings.Contains(text, "issue comment 28") {
		t.Fatalf("ambiguous discovery wrote issue marker:\n%s", text)
	}
}

func genericDispatchCommand(t *testing.T, bin, log, state string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/bash", genericDispatchScript(t),
		"--benchmark", "h5", "--issue", "28", "--attempt", "2",
		"--workflow", "h5-frozen-execution.yml")
	cmd.Env = []string{
		"PATH=" + bin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GH_LOG=" + log,
		"GH_STATE=" + state,
		"FAKE_SCENARIO=" + os.Getenv("FAKE_SCENARIO"),
		"BENCHMARK_DISPATCH_POLL_SECONDS=0",
	}
	return cmd
}
func setupFakeDispatchGH(t *testing.T, scenario string) (string, string, string) {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "gh.log")
	state := filepath.Join(t.TempDir(), "dispatched")
	writeFakeCLI(t, bin, "gh", fakeDispatchGH)
	t.Setenv("FAKE_SCENARIO", scenario)
	return bin, log, state
}

const fakeDispatchGH = `#!/bin/sh
set -eu
echo "$*" >> "$GH_LOG"
scenario="${FAKE_SCENARIO:-success}"
if [ "$1 $2" = "auth status" ]; then exit 0; fi
if [ "$1 $2" = "api --paginate" ]; then
  if [ "$scenario" = "marker" ]; then echo '[h5-fresh-dispatch-created attempt=2]'; fi
  exit 0
fi
if [ "$1 $2" = "run list" ]; then
  if [ "$scenario" = "active" ] && [ ! -f "$GH_STATE" ]; then
    printf '33077484701\tin_progress\n'; exit 0
  fi
  printf '33077484701\tcompleted\n'
  if [ "$scenario" = "count-mismatch" ] && [ ! -f "$GH_STATE" ]; then printf '33077484702\tcompleted\n'; fi
  if [ -f "$GH_STATE" ]; then
    printf '400\tqueued\n'
    if [ "$scenario" = "ambiguous" ]; then printf '401\tqueued\n'; fi
  fi
  exit 0
fi
if [ "$1 $2" = "workflow run" ]; then
  if [ "$scenario" = "dispatch-fail" ]; then exit 1; fi
  : > "$GH_STATE"
  exit 0
fi
if [ "$1 $2" = "issue comment" ]; then exit 0; fi
exit 1
`

func genericDispatchScript(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "scripts", "dispatch-frozen-benchmark.sh")
}
