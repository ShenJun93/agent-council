package safestore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriteExclusiveRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	for _, rel := range []string{"../escape.json", outside} {
		if _, err := WriteExclusive(root, rel, []byte("x")); err == nil {
			t.Fatalf("WriteExclusive(%q) unexpectedly succeeded", rel)
		}
	}
}

func TestWriteExclusiveRejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteExclusive(root, "link/value.json", []byte("x")); err == nil {
		t.Fatal("WriteExclusive() unexpectedly followed symlinked parent")
	}
}

func TestWriteExclusiveRejectsFinalSymlinkAndExistingFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteExclusive(root, "target.json", []byte("new")); err == nil {
		t.Fatal("WriteExclusive() unexpectedly overwrote existing file")
	}

	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "final.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteExclusive(root, "final.json", []byte("new")); err == nil {
		t.Fatal("WriteExclusive() unexpectedly followed final symlink")
	}
}

func TestWriteExclusiveCreatesNestedFileOnce(t *testing.T) {
	root := t.TempDir()
	path, err := WriteExclusive(root, "a/b/value.json", []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "a", "b", "value.json")
	if path != want {
		t.Fatalf("path = %q want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("content = %q", got)
	}
}

func TestWriteExclusiveAllowsConcurrentCreationUnderSharedParent(t *testing.T) {
	root := t.TempDir()
	const count = 64
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := WriteExclusive(root, filepath.Join("a", "b", fmt.Sprintf("%03d.json", i)), []byte("x"))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
