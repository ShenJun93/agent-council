package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaseHFrozenWorkflowPinsGovernedTechnicalPilot(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "phase-h-frozen-execution.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	required := []string{
		"name: Phase H Frozen Execution",
		"workflow_dispatch:",
		"frozen_sha:",
		"dispatch_marker:",
		"current_session_attestation:",
		"PHASE_H_FROZEN_SHA: 6d9aa5283180c7a08d7ccd69d6fbbd67171c8380",
		"PHASE_H_TREE_SHA: 8b27995e8efaff578ba1f4e668c18aa59dc4e03d",
		"PHASE_H_MANIFEST_SHA256: 208b2e5fe1887edf7bd5e8b473de1788f7e08211de62c941eaba7569a61f93d3",
		"PHASE_H_RUBRIC_SHA256: 644d0ee576c4564b124c2303a9208c142e261b9ba6ef96d78ad706616c17b952",
		"PHASE_H_ADAPTER_POLICY_SHA256: b2562f4842c1828746f5337e0300966eeda4c0a1f0afdd0a392991fba11ab11c",
	}
	required = append(required,
		"PHASE_H_DISPATCH_MARKER: issue-56-phase-h-real-run-1",
		"- self-hosted",
		"- linux",
		"- phase-h-benchmark",
		"ref: 6d9aa5283180c7a08d7ccd69d6fbbd67171c8380",
		"test \"$GITHUB_RUN_NUMBER\" = \"1\"",
		"test \"$GITHUB_RUN_ATTEMPT\" = \"1\"",
		"TestLoadPhaseHReplayAcceptsCommittedFrozenReplay",
		"TestPhaseHRegistryUsesCurrentOrchestratorSessionBroker",
		"OPENAI_API_KEY CODEX_API_KEY ANTHROPIC_API_KEY",
		"council benchmark phase-h",
		"--dataset benchmarks/phase-h",
		"\"source_h8_artifact_id\":9745340503",
		"\"expected_successful_invocations\":120",
		"\"require_current_session\":true",
		"\"require_fresh_session\":false",
		"all-phase-h-files.sha256",
		"successful_current_session_requests=120",
		"successful_current_session_responses=120",
		"phase-h-frozen-${{ github.run_id }}",
	)
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Errorf("workflow missing %q", needle)
		}
	}
	for _, forbidden := range []string{"schedule:", "pull_request:", "push:"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("workflow must be manual-only; found %q", forbidden)
		}
	}
}
