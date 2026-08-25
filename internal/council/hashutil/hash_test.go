package hashutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileReturnsSHA256(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("agent council\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := File(path)
	if err != nil {
		t.Fatal(err)
	}

	const want = "f8c4de8fec12d2dc7cb2ffd9679ac3f7ce916d0d9303152b340c79d999f41997"
	if got != want {
		t.Fatalf("File() = %q, want %q", got, want)
	}
}

func TestTreeIsDeterministicAndContentSensitive(t *testing.T) {
	t.Parallel()

	left := t.TempDir()
	right := t.TempDir()

	write := func(root, rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(left, "b.txt", "two")
	write(left, "a/one.txt", "one")
	write(right, "a/one.txt", "one")
	write(right, "b.txt", "two")

	leftHash, err := Tree(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := Tree(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("Tree hashes differ for equivalent trees: %q != %q", leftHash, rightHash)
	}

	write(right, "b.txt", "changed")
	changedHash, err := Tree(right)
	if err != nil {
		t.Fatal(err)
	}
	if changedHash == leftHash {
		t.Fatal("Tree hash did not change after file content changed")
	}
}
