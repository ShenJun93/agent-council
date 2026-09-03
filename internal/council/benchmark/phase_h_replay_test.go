package benchmark

import (
	"path/filepath"
	"testing"
)

func TestLoadPhaseHReplayAcceptsCommittedFrozenReplay(t *testing.T) {
	ds, err := LoadPhaseHReplay(filepath.Join("..", "..", "..", "benchmarks", "phase-h"))
	if err != nil {
		t.Fatal(err)
	}
	if ds.Manifest.BenchmarkID != PhaseHBenchmarkID {
		t.Fatalf("benchmark=%q", ds.Manifest.BenchmarkID)
	}
	if ds.Manifest.SourceH8RunID != PhaseHSourceH8RunID || ds.Manifest.SourceH8ArtifactDigest != PhaseHSourceH8ArtifactDigest {
		t.Fatalf("source=%+v", ds.Manifest)
	}
	if len(ds.Cases) != PhaseHReplayCaseCount {
		t.Fatalf("cases=%d", len(ds.Cases))
	}
	if PhaseHExpectedSuccessfulInvocations != 120 {
		t.Fatalf("expected calls=%d", PhaseHExpectedSuccessfulInvocations)
	}
	for _, c := range ds.Cases {
		if len(c.Arms) != 6 {
			t.Fatalf("case %s arms=%d", c.ID, len(c.Arms))
		}
	}
	if err := validatePhaseHAdapterPolicy(ds.AdapterPolicy); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePhaseHReplayManifestRejectsSessionOrTopologyMutation(t *testing.T) {
	ds, err := LoadPhaseHReplay(filepath.Join("..", "..", "..", "benchmarks", "phase-h"))
	if err != nil {
		t.Fatal(err)
	}
	m := ds.Manifest
	m.RequireFreshSession = true
	if err := validatePhaseHReplayManifest(m); err == nil {
		t.Fatal("expected fresh-session policy rejection")
	}
	m = ds.Manifest
	m.ExpectedSuccessfulInvocations--
	if err := validatePhaseHReplayManifest(m); err == nil {
		t.Fatal("expected invocation-topology rejection")
	}
}

func TestValidatePhaseHAdapterPolicyRejectsNonWebAdapter(t *testing.T) {
	ds, err := LoadPhaseHReplay(filepath.Join("..", "..", "..", "benchmarks", "phase-h"))
	if err != nil {
		t.Fatal(err)
	}
	policy := ds.AdapterPolicy
	policy.Adapters[0].ProviderFamily = "codex"
	if err := validatePhaseHAdapterPolicy(policy); err == nil {
		t.Fatal("expected non-web adapter rejection")
	}
}
