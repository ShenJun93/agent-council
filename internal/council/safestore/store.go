package safestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteExclusive creates one immutable file below root without following
// symlinks in any newly traversed path component.
func WriteExclusive(root, rel string, data []byte) (string, error) {
	root = strings.TrimSpace(root)
	rel = strings.TrimSpace(rel)
	if root == "" {
		return "", fmt.Errorf("root is required")
	}
	if rel == "" {
		return "", fmt.Errorf("relative path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path is not allowed: %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %q", rel)
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("path traversal is not allowed: %q", rel)
		}
	}
	if err := requireRealDir(root); err != nil {
		return "", err
	}
	path := filepath.Join(root, clean)
	if err := ensureParent(root, filepath.Dir(path)); err != nil {
		return "", err
	}
	if info, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("path already exists: %s (%s)", path, info.Mode())
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect destination: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = f.Close()
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	committed = true
	return path, nil
}

func requireRealDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("root must be a real directory: %s", path)
	}
	return nil
}
func ensureParent(root, parent string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("absolute root: %w", err)
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return fmt.Errorf("absolute parent: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, parentAbs)
	if err != nil {
		return fmt.Errorf("relativize parent: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("parent escapes root: %s", parent)
	}
	if rel == "." {
		return nil
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		switch {
		case statErr == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("path component is not a real directory: %s", current)
			}
		case errors.Is(statErr, os.ErrNotExist):
			if err := os.Mkdir(current, 0o750); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}
		default:
			return fmt.Errorf("inspect directory: %w", statErr)
		}
	}
	return nil
}
