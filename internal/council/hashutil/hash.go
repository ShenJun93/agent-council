package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// File returns the lowercase hexadecimal SHA-256 digest of a regular file.
func File(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("hash file %q: not a regular file", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Tree returns a deterministic SHA-256 digest for all regular files below root.
// Paths are normalized to forward slashes and symlinks are rejected.
func Tree(root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("hash tree %q: not a directory", root)
	}

	type entry struct {
		rel    string
		digest string
	}
	var entries []entry

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("hash tree %q: symlink %q is not allowed", root, path)
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("hash tree %q: non-regular file %q is not allowed", root, path)
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest, err := File(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{rel: filepath.ToSlash(rel), digest: digest})
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	h := sha256.New()
	for _, e := range entries {
		_, _ = io.WriteString(h, e.rel)
		_, _ = h.Write([]byte{0})
		_, _ = io.WriteString(h, e.digest)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
