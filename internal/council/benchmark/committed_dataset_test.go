package benchmark

import (
	"path/filepath"
	"testing"
)

func TestCommittedH1DatasetLoads(t *testing.T) {
	root := filepath.Join("..", "..", "..", "benchmarks", "h1")
	dataset, err := LoadH1(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dataset.Cases) != H1CaseCount {
		t.Fatalf("got %d cases, want %d", len(dataset.Cases), H1CaseCount)
	}
}
