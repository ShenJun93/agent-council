package visibility

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Viewer struct {
	Participant string
	Phase       string
}

type Artifact struct {
	ID           string
	RelativePath string
	Provider     string
	Content      []byte
}

type Grant struct {
	Participant string
	Phase       string
	ArtifactID  string
}

type Policy struct {
	Grants               []Grant
	MaskProviderIdentity bool
}

type Request struct {
	RunRoot   string
	TempRoot  string
	Viewer    Viewer
	Artifacts []Artifact
	Policy    Policy
}

type Workspace struct {
	Root               string
	VisibleArtifactIDs []string
}

func Materialize(req Request) (Workspace, error) {
	if strings.TrimSpace(req.RunRoot) == "" {
		return Workspace{}, fmt.Errorf("run root is required")
	}
	if strings.TrimSpace(req.TempRoot) == "" {
		return Workspace{}, fmt.Errorf("temp root is required")
	}

	if err := os.MkdirAll(req.TempRoot, 0o700); err != nil {
		return Workspace{}, fmt.Errorf("create temp root: %w", err)
	}

	tempInsideRun, err := IsWithin(req.RunRoot, req.TempRoot)
	if err != nil {
		return Workspace{}, fmt.Errorf("validate temp root isolation: %w", err)
	}
	if tempInsideRun {
		return Workspace{}, fmt.Errorf("temp root %q must be outside run root %q", req.TempRoot, req.RunRoot)
	}

	root, err := os.MkdirTemp(req.TempRoot, "council-workspace-")
	if err != nil {
		return Workspace{}, fmt.Errorf("create isolated workspace: %w", err)
	}
	workspace := Workspace{Root: root}
	committed := false
	defer func() {
		if !committed {
			_ = workspace.Cleanup()
		}
	}()

	insideRun, err := IsWithin(req.RunRoot, root)
	if err != nil {
		return Workspace{}, fmt.Errorf("validate workspace isolation: %w", err)
	}
	if insideRun {
		return Workspace{}, fmt.Errorf("workspace %q is inside run root %q", root, req.RunRoot)
	}

	granted := grantSet(req.Policy, req.Viewer)
	seenIDs := make(map[string]struct{}, len(req.Artifacts))
	for _, artifact := range req.Artifacts {
		if strings.TrimSpace(artifact.ID) == "" {
			return Workspace{}, fmt.Errorf("artifact id is required")
		}
		if _, duplicate := seenIDs[artifact.ID]; duplicate {
			return Workspace{}, fmt.Errorf("duplicate artifact id %q", artifact.ID)
		}
		seenIDs[artifact.ID] = struct{}{}

		if _, allowed := granted[artifact.ID]; !allowed {
			continue
		}
		if err := validateRelativePath(artifact.RelativePath); err != nil {
			return Workspace{}, fmt.Errorf("artifact %q: %w", artifact.ID, err)
		}

		target := filepath.Join(root, filepath.Clean(artifact.RelativePath))
		withinWorkspace, err := IsWithin(root, filepath.Dir(target))
		if err != nil {
			return Workspace{}, fmt.Errorf("artifact %q path validation: %w", artifact.ID, err)
		}
		if !withinWorkspace {
			return Workspace{}, fmt.Errorf("artifact %q escapes workspace", artifact.ID)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return Workspace{}, fmt.Errorf("artifact %q create parent: %w", artifact.ID, err)
		}
		if err := os.WriteFile(target, artifact.Content, 0o600); err != nil {
			return Workspace{}, fmt.Errorf("artifact %q materialize: %w", artifact.ID, err)
		}
		workspace.VisibleArtifactIDs = append(workspace.VisibleArtifactIDs, artifact.ID)
	}

	committed = true
	return workspace, nil
}

func (w Workspace) Cleanup() error {
	if strings.TrimSpace(w.Root) == "" {
		return nil
	}
	return os.RemoveAll(w.Root)
}

func IsWithin(root, candidate string) (bool, error) {
	rootPath, err := canonicalExisting(root)
	if err != nil {
		return false, fmt.Errorf("canonicalize root: %w", err)
	}
	candidatePath, err := canonicalExisting(candidate)
	if err != nil {
		return false, fmt.Errorf("canonicalize candidate: %w", err)
	}

	rel, err := filepath.Rel(rootPath, candidatePath)
	if err != nil {
		return false, err
	}
	if rel == "." {
		return true, nil
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

func grantSet(policy Policy, viewer Viewer) map[string]struct{} {
	granted := make(map[string]struct{})
	for _, grant := range policy.Grants {
		if grant.Participant == viewer.Participant && grant.Phase == viewer.Phase {
			granted[grant.ArtifactID] = struct{}{}
		}
	}
	return granted
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("relative path is required")
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return fmt.Errorf("absolute path %q is not allowed", path)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path traversal %q is not allowed", path)
	}
	return nil
}

func canonicalExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
