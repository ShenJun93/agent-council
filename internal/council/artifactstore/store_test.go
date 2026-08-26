package artifactstore

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/protocol"
	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func TestWriteProtocolResultPersistsStructuredArtifactsAndSeparateProvenance(t *testing.T) {
	t.Parallel()

	runRoot := t.TempDir()
	result := sampleResult()
	index, err := WriteProtocolResult(context.Background(), WriteRequest{
		RunID:              "run-123",
		RunRoot:            runRoot,
		Result:             result,
		ChallengerProvider: councilruntime.ProviderCodex,
		Now: func() time.Time {
			return time.Date(2026, 8, 26, 11, 45, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("WriteProtocolResult() error = %v", err)
	}

	if index.SchemaVersion != IndexSchemaVersion || index.RunID != "run-123" {
		t.Fatalf("index = %+v", index)
	}
	if len(index.Entries) != 11 {
		t.Fatalf("provenance entries = %d, want 11", len(index.Entries))
	}

	researchPath := filepath.Join(runRoot, "artifacts", "research", "research-1.json")
	researchBytes, err := os.ReadFile(researchPath)
	if err != nil {
		t.Fatalf("read research artifact: %v", err)
	}
	var research protocol.ResearchArtifact
	if err := json.Unmarshal(researchBytes, &research); err != nil {
		t.Fatalf("unmarshal research artifact: %v", err)
	}
	if research.Recommendation != "alpha" {
		t.Fatalf("research recommendation = %q", research.Recommendation)
	}
	if strings.Contains(strings.ToLower(string(researchBytes)), "claude") || strings.Contains(strings.ToLower(string(researchBytes)), "codex") {
		t.Fatalf("provider identity leaked into artifact content: %s", researchBytes)
	}

	entries := map[string]Provenance{}
	for _, entry := range index.Entries {
		entries[entry.ArtifactID] = entry
	}
	assertProvenance(t, entries["research-1"], Provenance{
		ArtifactID:  "research-1",
		Phase:       protocol.PhaseResearch,
		Participant: "researcher-1",
		Role:        "researcher",
		Provider:    councilruntime.ProviderClaude,
		Inputs:      []string{"problem"},
	})
	assertProvenance(t, entries["review-1"], Provenance{
		ArtifactID:  "review-1",
		Phase:       protocol.PhaseReview,
		Participant: "reviewer-1",
		Role:        "reviewer",
		Provider:    councilruntime.ProviderClaude,
		Inputs:      []string{"problem", "research-2"},
	})
	assertProvenance(t, entries["challenge"], Provenance{
		ArtifactID:  "challenge",
		Phase:       protocol.PhaseChallenge,
		Participant: "challenger",
		Role:        "challenger",
		Provider:    councilruntime.ProviderCodex,
		Inputs:      []string{"problem", "research-1", "research-2", "review-1", "review-2"},
	})
	assertProvenance(t, entries["decision-record"], Provenance{
		ArtifactID:  "decision-record",
		Phase:       "decision",
		Participant: "engine",
		Role:        "engine",
		Inputs:      []string{"judge-1", "judge-2"},
	})

	expectedHash := sha256.Sum256(researchBytes)
	if entries["research-1"].SHA256 != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("research provenance sha256 = %q, want %q", entries["research-1"].SHA256, hex.EncodeToString(expectedHash[:]))
	}
	if entries["research-1"].RecordedAt != "2026-08-26T11:45:00Z" {
		t.Fatalf("recorded_at = %q", entries["research-1"].RecordedAt)
	}

	provenancePath := filepath.Join(runRoot, "artifacts", "provenance.jsonl")
	file, err := os.Open(provenancePath)
	if err != nil {
		t.Fatalf("open provenance log: %v", err)
	}
	defer func() { _ = file.Close() }()

	lines := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines++
		var entry Provenance
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid provenance line %d: %v", lines, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan provenance: %v", err)
	}
	if lines != len(index.Entries) {
		t.Fatalf("provenance lines = %d, want %d", lines, len(index.Entries))
	}
}

func TestWriteProtocolResultIsImmutable(t *testing.T) {
	t.Parallel()

	runRoot := t.TempDir()
	req := WriteRequest{
		RunID:              "immutable-run",
		RunRoot:            runRoot,
		Result:             sampleResult(),
		ChallengerProvider: councilruntime.ProviderClaude,
	}
	if _, err := WriteProtocolResult(context.Background(), req); err != nil {
		t.Fatalf("first write error = %v", err)
	}

	path := filepath.Join(runRoot, "artifacts", "research", "research-1.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteProtocolResult(context.Background(), req); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second write error = %v, want immutable already-exists failure", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("immutable artifact was modified by second write")
	}
}

func TestWriteProtocolResultRejectsRunIDMismatchAndUnsafeRecordID(t *testing.T) {
	t.Parallel()

	result := sampleResult()
	result.Research[0].ID = "../escape"
	_, err := WriteProtocolResult(context.Background(), WriteRequest{
		RunID:              "run-unsafe",
		RunRoot:            t.TempDir(),
		Result:             result,
		ChallengerProvider: councilruntime.ProviderClaude,
	})
	if err == nil || !strings.Contains(err.Error(), "artifact id") {
		t.Fatalf("unsafe artifact id error = %v", err)
	}
}

func assertProvenance(t *testing.T, got, want Provenance) {
	t.Helper()
	if got.ArtifactID != want.ArtifactID || got.Phase != want.Phase || got.Participant != want.Participant || got.Role != want.Role || got.Provider != want.Provider {
		t.Fatalf("provenance = %+v, want core fields %+v", got, want)
	}
	if strings.Join(got.Inputs, ",") != strings.Join(want.Inputs, ",") {
		t.Fatalf("provenance inputs = %v, want %v", got.Inputs, want.Inputs)
	}
	if got.Path == "" || got.SHA256 == "" || got.RecordedAt == "" || got.SchemaVersion != ProvenanceSchemaVersion {
		t.Fatalf("provenance missing audit fields: %+v", got)
	}
}

func sampleResult() protocol.Result {
	return protocol.Result{
		Research: []protocol.ResearchRecord{
			{ID: "research-1", Artifact: protocol.ResearchArtifact{Recommendation: "alpha", Confidence: 0.9}},
			{ID: "research-2", Artifact: protocol.ResearchArtifact{Recommendation: "beta", Confidence: 0.8}},
		},
		Reviews: []protocol.ReviewRecord{
			{ID: "review-1", TargetID: "research-2", Artifact: protocol.ReviewArtifact{Confidence: 0.8}},
			{ID: "review-2", TargetID: "research-1", Artifact: protocol.ReviewArtifact{Confidence: 0.8}},
		},
		ChallengeDecision: protocol.ChallengeDecision{
			Mode:                    protocol.ChallengeFull,
			MaterialAgreement:       false,
			HighConfidenceThreshold: 0.9,
			ResearchConfidences:     [2]float64{0.9, 0.8},
		},
		Challenge: protocol.ChallengeRecord{
			ID:       "challenge",
			Artifact: protocol.ChallengeArtifact{Confidence: 0.8},
		},
		Rebuttals: []protocol.RebuttalRecord{
			{ID: "rebuttal-1", TargetID: "research-1", Artifact: protocol.RebuttalArtifact{UpdatedRecommendation: "alpha", UpdatedConfidence: 0.8}},
			{ID: "rebuttal-2", TargetID: "research-2", Artifact: protocol.RebuttalArtifact{UpdatedRecommendation: "beta", UpdatedConfidence: 0.8}},
		},
		Judges: []protocol.JudgeRecord{
			{ID: "judge-1", Artifact: protocol.JudgeArtifact{Decision: "alpha", Confidence: 0.8}},
			{ID: "judge-2", Artifact: protocol.JudgeArtifact{Decision: "beta", Confidence: 0.8}},
		},
		Decision: protocol.DecisionRecord{
			Status:         protocol.DecisionJudgeDisagreement,
			JudgeAgreement: false,
			JudgeDecisions: [2]string{"alpha", "beta"},
		},
	}
}
