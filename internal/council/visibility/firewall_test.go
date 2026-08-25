package visibility

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeResearcherOnlyGetsGrantedArtifacts(t *testing.T) {
	t.Parallel()

	runRoot := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	tempRoot := t.TempDir()

	workspace, err := Materialize(Request{
		RunRoot:  runRoot,
		TempRoot: tempRoot,
		Viewer:   Viewer{Participant: "researcher-a", Phase: "research"},
		Artifacts: []Artifact{
			{ID: "problem", RelativePath: "inputs/problem.md", Content: []byte("problem")},
			{ID: "report-a", RelativePath: "reports/a.json", Content: []byte(`{"answer":"a"}`)},
			{ID: "report-b", RelativePath: "reports/b.json", Content: []byte(`{"answer":"b"}`)},
		},
		Policy: Policy{Grants: []Grant{
			{Participant: "researcher-a", Phase: "research", ArtifactID: "problem"},
			{Participant: "researcher-a", Phase: "research", ArtifactID: "report-a"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Cleanup()

	mustExist(t, filepath.Join(workspace.Root, "inputs", "problem.md"))
	mustExist(t, filepath.Join(workspace.Root, "reports", "a.json"))
	mustNotExist(t, filepath.Join(workspace.Root, "reports", "b.json"))
	if got := strings.Join(workspace.VisibleArtifactIDs, ","); got != "problem,report-a" {
		t.Fatalf("visible artifact ids = %q", got)
	}
}

func TestBlindReviewDoesNotExposePeerReview(t *testing.T) {
	t.Parallel()

	runRoot := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace, err := Materialize(Request{
		RunRoot:  runRoot,
		TempRoot: t.TempDir(),
		Viewer:   Viewer{Participant: "reviewer-a", Phase: "blind-review"},
		Artifacts: []Artifact{
			{ID: "problem", RelativePath: "problem.md", Content: []byte("problem")},
			{ID: "target-report", RelativePath: "target/report.json", Content: []byte("target")},
			{ID: "peer-review", RelativePath: "reviews/peer.json", Content: []byte("peer review")},
		},
		Policy: Policy{Grants: []Grant{
			{Participant: "reviewer-a", Phase: "blind-review", ArtifactID: "problem"},
			{Participant: "reviewer-a", Phase: "blind-review", ArtifactID: "target-report"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Cleanup()

	mustExist(t, filepath.Join(workspace.Root, "problem.md"))
	mustExist(t, filepath.Join(workspace.Root, "target", "report.json"))
	mustNotExist(t, filepath.Join(workspace.Root, "reviews", "peer.json"))
}

func TestMaterializeFailsClosedWithoutGrant(t *testing.T) {
	t.Parallel()

	runRoot := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace, err := Materialize(Request{
		RunRoot:  runRoot,
		TempRoot: t.TempDir(),
		Viewer:   Viewer{Participant: "judge-a", Phase: "judge"},
		Artifacts: []Artifact{
			{ID: "secret", RelativePath: "secret.json", Content: []byte("denied")},
		},
		Policy: Policy{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Cleanup()

	mustNotExist(t, filepath.Join(workspace.Root, "secret.json"))
	if len(workspace.VisibleArtifactIDs) != 0 {
		t.Fatalf("visible artifact ids = %#v", workspace.VisibleArtifactIDs)
	}
}

func TestJudgeWorkspaceDoesNotMaterializeProviderMetadata(t *testing.T) {
	t.Parallel()

	runRoot := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace, err := Materialize(Request{
		RunRoot:  runRoot,
		TempRoot: t.TempDir(),
		Viewer:   Viewer{Participant: "judge-a", Phase: "judge"},
		Artifacts: []Artifact{
			{ID: "report-a", RelativePath: "reports/a.json", Provider: "claude", Content: []byte(`{"recommendation":"ship"}`)},
			{ID: "report-b", RelativePath: "reports/b.json", Provider: "codex", Content: []byte(`{"recommendation":"hold"}`)},
		},
		Policy: Policy{
			MaskProviderIdentity: true,
			Grants: []Grant{
				{Participant: "judge-a", Phase: "judge", ArtifactID: "report-a"},
				{Participant: "judge-a", Phase: "judge", ArtifactID: "report-b"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Cleanup()

	var materialized strings.Builder
	err = filepath.WalkDir(workspace.Root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		materialized.WriteString(filepath.ToSlash(path))
		if d.Type().IsRegular() {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			materialized.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(materialized.String())
	for _, provider := range []string{"claude", "anthropic", "codex", "openai"} {
		if strings.Contains(text, provider) {
			t.Fatalf("provider identity %q leaked into judge workspace: %q", provider, text)
		}
	}
}

func TestMaterializeRejectsTraversal(t *testing.T) {
	t.Parallel()

	runRoot := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Materialize(Request{
		RunRoot:  runRoot,
		TempRoot: t.TempDir(),
		Viewer:   Viewer{Participant: "researcher-a", Phase: "research"},
		Artifacts: []Artifact{
			{ID: "escape", RelativePath: "../leak.txt", Content: []byte("leak")},
		},
		Policy: Policy{Grants: []Grant{{Participant: "researcher-a", Phase: "research", ArtifactID: "escape"}}},
	})
	if err == nil {
		t.Fatal("Materialize() accepted path traversal")
	}
}

func TestMaterializeRejectsSymlinkTempRootIntoRun(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	runRoot := filepath.Join(base, "run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkRoot := filepath.Join(base, "temp-link")
	if err := os.Symlink(runRoot, symlinkRoot); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := Materialize(Request{
		RunRoot:  runRoot,
		TempRoot: symlinkRoot,
		Viewer:   Viewer{Participant: "researcher-a", Phase: "research"},
	})
	if err == nil {
		t.Fatal("Materialize() accepted symlink temp root into run root")
	}
}

func TestWorkspaceIsOutsideFullRunRoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	runRoot := filepath.Join(base, "run")
	tempRoot := filepath.Join(base, "isolated")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace, err := Materialize(Request{RunRoot: runRoot, TempRoot: tempRoot})
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Cleanup()

	inside, err := IsWithin(runRoot, workspace.Root)
	if err != nil {
		t.Fatal(err)
	}
	if inside {
		t.Fatalf("workspace %q is inside run root %q", workspace.Root, runRoot)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be absent, stat error = %v", path, err)
	}
}
