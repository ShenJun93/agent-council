package benchmark

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testH5Policy() H5AdapterPolicy {
	a := []string{"claude-max", "codex-chatgpt", "human-chatgpt-session"}
	b := []string{"codex-chatgpt", "claude-max", "human-chatgpt-session"}
	policy := H5AdapterPolicy{
		SchemaVersion: H5AdapterPolicySchemaVersion,
		Adapters: []H5AdapterDescriptor{
			{ID: "claude-max", ProviderFamily: "claude", Transport: "claude-cli", AuthClass: "subscription", Interaction: "automated"},
			{ID: "codex-chatgpt", ProviderFamily: "codex", Transport: "codex-cli", AuthClass: "chatgpt-subscription", Interaction: "automated"},
			{ID: "human-chatgpt-session", ProviderFamily: "chatgpt", Transport: "human-chatgpt-session", AuthClass: "chatgpt-subscription", Interaction: "human-broker"},
		},
		Slots: map[string][]string{}, ChallengerByCase: map[string][]string{},
	}
	for _, slot := range []string{"baseline-a", "researcher-1", "reviewer-1", "judge-1", "eval-judge-1"} {
		policy.Slots[slot] = append([]string(nil), a...)
	}
	for _, slot := range []string{"baseline-b", "researcher-2", "reviewer-2", "judge-2", "eval-judge-2"} {
		policy.Slots[slot] = append([]string(nil), b...)
	}
	for i, id := range h1CaseIDs {
		chain := a
		if (i+1)%2 == 0 {
			chain = b
		}
		policy.ChallengerByCase[id] = append([]string(nil), chain...)
	}
	return policy
}

func writeValidH5Fixture(t *testing.T) string {
	t.Helper()
	root := writeValidH4Fixture(t)
	var rubric rubricDocument
	readJSONFile(t, filepath.Join(root, "rubric.json"), &rubric)
	rubric.SchemaVersion = H5RubricSchemaVersion
	rubricBytes := writeJSONFile(t, filepath.Join(root, "rubric.json"), rubric)
	doc := readFixtureCases(t, root)
	doc.SchemaVersion = H5CasesSchemaVersion
	for i := range doc.Cases {
		doc.Cases[i].ChallengerProvider = ""
	}
	casesBytes := writeJSONFile(t, filepath.Join(root, "cases.json"), doc)
	policyBytes := writeJSONFile(t, filepath.Join(root, "adapter-policy.json"), testH5Policy())
	manifest := readFixtureManifest(t, root)
	manifest.SchemaVersion = H5DatasetSchemaVersion
	manifest.BenchmarkID = H5BenchmarkID
	manifest.RubricSHA256 = digestBytes(rubricBytes)
	manifest.CasesSHA256 = digestBytes(casesBytes)
	manifest.AdapterPolicySHA256 = digestBytes(policyBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	return root
}

func TestLoadH5AcceptsProviderAgnosticDatasetAndPolicy(t *testing.T) {
	t.Parallel()
	dataset, err := LoadH5(writeValidH5Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Manifest.BenchmarkID != H5BenchmarkID || dataset.AdapterPolicy == nil {
		t.Fatalf("dataset=%+v", dataset.Manifest)
	}
	if dataset.AdapterPolicySHA256 != dataset.Manifest.AdapterPolicySHA256 {
		t.Fatalf("policy hash mismatch")
	}
	for _, c := range dataset.Cases {
		if c.ChallengerProvider != "" {
			t.Fatalf("case %s retained provider binding %q", c.ID, c.ChallengerProvider)
		}
	}
}

func TestLoadH5RejectsTamperedAdapterPolicy(t *testing.T) {
	t.Parallel()
	root := writeValidH5Fixture(t)
	policy := testH5Policy()
	policy.Slots["reviewer-1"] = []string{"unknown-adapter"}
	writeJSONFile(t, filepath.Join(root, "adapter-policy.json"), policy)
	if _, err := LoadH5(root); err == nil {
		t.Fatal("expected tampered adapter policy rejection")
	}
}

func TestCommittedH5MatchesH4SemanticPayloads(t *testing.T) {
	t.Parallel()
	h4, err := LoadH4(filepath.Join("..", "..", "..", "benchmarks", "h4"))
	if err != nil {
		t.Fatal(err)
	}
	h5, err := LoadH5(filepath.Join("..", "..", "..", "benchmarks", "h5"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h4.Cases) != len(h5.Cases) {
		t.Fatal("case count drift")
	}
	for i := range h4.Cases {
		a, b := h4.Cases[i], h5.Cases[i]
		if a.ID != b.ID || a.Category != b.Category {
			t.Fatalf("case %d identity drift", i)
		}
		if !bytes.Equal(a.Problem, b.Problem) || !bytes.Equal(a.ReferenceSet, b.ReferenceSet) {
			t.Fatalf("case %s payload drift", a.ID)
		}
		if a.ProblemSHA256 != b.ProblemSHA256 || a.ReferenceSetSHA256 != b.ReferenceSetSHA256 {
			t.Fatalf("case %s payload hash drift", a.ID)
		}
	}
	for _, pair := range [][2]string{{"h4", "h5"}} {
		_ = pair
	}
	r4, err := os.ReadFile(filepath.Join("..", "..", "..", "benchmarks", "h4", "rubric.json"))
	if err != nil {
		t.Fatal(err)
	}
	r5, err := os.ReadFile(filepath.Join("..", "..", "..", "benchmarks", "h5", "rubric.json"))
	if err != nil {
		t.Fatal(err)
	}
	var a, b rubricDocument
	if err := decodeStrict("h4 rubric", r4, &a); err != nil {
		t.Fatal(err)
	}
	if err := decodeStrict("h5 rubric", r5, &b); err != nil {
		t.Fatal(err)
	}
	a.SchemaVersion, b.SchemaVersion = "", ""
	if !reflect.DeepEqual(a, b) {
		t.Fatal("rubric semantic drift")
	}
}

func TestCommittedH5PolicyKeepsH4PrimaryOrientationWithFallback(t *testing.T) {
	t.Parallel()
	dataset, err := LoadH5(filepath.Join("..", "..", "..", "benchmarks", "h5"))
	if err != nil {
		t.Fatal(err)
	}
	a := []string{"claude-max", "codex-chatgpt", "human-chatgpt-session"}
	b := []string{"codex-chatgpt", "claude-max", "human-chatgpt-session"}
	for _, slot := range []string{"baseline-a", "researcher-1", "reviewer-1", "judge-1", "eval-judge-1"} {
		if !reflect.DeepEqual(dataset.AdapterPolicy.Slots[slot], a) {
			t.Fatalf("slot %s", slot)
		}
	}
	for _, slot := range []string{"baseline-b", "researcher-2", "reviewer-2", "judge-2", "eval-judge-2"} {
		if !reflect.DeepEqual(dataset.AdapterPolicy.Slots[slot], b) {
			t.Fatalf("slot %s", slot)
		}
	}
	for i, id := range h1CaseIDs {
		want := a
		if (i+1)%2 == 0 {
			want = b
		}
		if !reflect.DeepEqual(dataset.AdapterPolicy.ChallengerByCase[id], want) {
			t.Fatalf("challenger %s", id)
		}
	}
}

func TestCommittedH5PolicyMarksHumanChatGPTBroker(t *testing.T) {
	t.Parallel()
	dataset, err := LoadH5(filepath.Join("..", "..", "..", "benchmarks", "h5"))
	if err != nil {
		t.Fatal(err)
	}
	var human *H5AdapterDescriptor
	for i := range dataset.AdapterPolicy.Adapters {
		if dataset.AdapterPolicy.Adapters[i].ID == "human-chatgpt-session" {
			human = &dataset.AdapterPolicy.Adapters[i]
			break
		}
	}
	if human == nil {
		t.Fatal("human-chatgpt-session missing")
	}
	if human.ProviderFamily != "chatgpt" || human.Transport != "human-chatgpt-session" || human.Interaction != "human-broker" {
		t.Fatalf("human adapter=%+v", *human)
	}
}

func TestLoadH5RejectsUnknownAdapterWithValidPolicyHash(t *testing.T) {
	t.Parallel()
	root := writeValidH5Fixture(t)
	policy := testH5Policy()
	policy.Slots["reviewer-1"] = []string{"unknown-adapter"}
	policyBytes := writeJSONFile(t, filepath.Join(root, "adapter-policy.json"), policy)
	manifest := readFixtureManifest(t, root)
	manifest.AdapterPolicySHA256 = digestBytes(policyBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	if _, err := LoadH5(root); err == nil {
		t.Fatal("expected unknown adapter rejection")
	}
}

func TestLoadH5RejectsMissingSlotWithValidPolicyHash(t *testing.T) {
	t.Parallel()
	root := writeValidH5Fixture(t)
	policy := testH5Policy()
	delete(policy.Slots, "reviewer-2")
	policyBytes := writeJSONFile(t, filepath.Join(root, "adapter-policy.json"), policy)
	manifest := readFixtureManifest(t, root)
	manifest.AdapterPolicySHA256 = digestBytes(policyBytes)
	writeJSONFile(t, filepath.Join(root, "manifest.json"), manifest)
	if _, err := LoadH5(root); err == nil {
		t.Fatal("expected missing slot rejection")
	}
}
