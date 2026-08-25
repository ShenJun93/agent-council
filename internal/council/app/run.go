package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ShenJun93/agent-council/internal/council/config"
	"github.com/ShenJun93/agent-council/internal/council/hashutil"
)

const ManifestSchemaVersion = "council.run.v0"

type CreateRunRequest struct {
	ProblemPath      string
	ConfigPath       string
	PromptsDir       string
	ReferenceSetPath string
	RunsRoot         string
	RunID            string
	Now              func() time.Time
}

type Manifest struct {
	SchemaVersion string `json:"schema_version"`
	RunID         string `json:"run_id"`
	CreatedAt     string `json:"created_at"`
	Inputs        Inputs `json:"inputs"`
	RunDir        string `json:"-"`
}

type Inputs struct {
	Problem      InputDigest `json:"problem"`
	Config       InputDigest `json:"config"`
	PromptBundle InputDigest `json:"prompt_bundle"`
	ReferenceSet InputDigest `json:"reference_set,omitempty"`
}

type InputDigest struct {
	Source     string `json:"source,omitempty"`
	FrozenPath string `json:"frozen_path,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

func CreateRun(ctx context.Context, req CreateRunRequest) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	if req.ProblemPath == "" {
		return Manifest{}, fmt.Errorf("problem path is required")
	}

	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("load config: %w", err)
	}

	runsRoot := req.RunsRoot
	if runsRoot == "" {
		runsRoot = cfg.Runs.Root
	}
	if runsRoot == "" {
		return Manifest{}, fmt.Errorf("runs root is empty")
	}

	now := time.Now
	if req.Now != nil {
		now = req.Now
	}
	createdAt := now().UTC()

	runID := req.RunID
	if runID == "" {
		runID, err = newRunID(createdAt)
		if err != nil {
			return Manifest{}, err
		}
	}
	if err := validateRunID(runID); err != nil {
		return Manifest{}, err
	}

	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create runs root: %w", err)
	}
	runDir := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create run directory %q: %w", runDir, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(runDir)
		}
	}()

	artifactsDir := filepath.Join(runDir, "artifacts")
	inputsDir := filepath.Join(runDir, "inputs")
	frozenPrompts := filepath.Join(inputsDir, "prompts")
	for _, dir := range []string{artifactsDir, inputsDir, frozenPrompts} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Manifest{}, fmt.Errorf("create run directory %q: %w", dir, err)
		}
	}

	frozenProblem := filepath.Join(inputsDir, "problem.md")
	if err := copyRegularFile(req.ProblemPath, frozenProblem); err != nil {
		return Manifest{}, fmt.Errorf("freeze problem: %w", err)
	}

	frozenConfig := filepath.Join(inputsDir, "council.yaml")
	if req.ConfigPath != "" {
		if err := copyRegularFile(req.ConfigPath, frozenConfig); err != nil {
			return Manifest{}, fmt.Errorf("freeze config: %w", err)
		}
	} else if err := os.WriteFile(frozenConfig, config.CanonicalYAML(cfg), 0o600); err != nil {
		return Manifest{}, fmt.Errorf("freeze default config: %w", err)
	}

	if req.PromptsDir != "" {
		if err := copyTree(req.PromptsDir, frozenPrompts); err != nil {
			return Manifest{}, fmt.Errorf("freeze prompt bundle: %w", err)
		}
	}

	var frozenReference string
	if req.ReferenceSetPath != "" {
		frozenReference = filepath.Join(inputsDir, "reference-set.json")
		if err := copyRegularFile(req.ReferenceSetPath, frozenReference); err != nil {
			return Manifest{}, fmt.Errorf("freeze reference set: %w", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}

	problemHash, err := hashutil.File(frozenProblem)
	if err != nil {
		return Manifest{}, err
	}
	configHash, err := hashutil.File(frozenConfig)
	if err != nil {
		return Manifest{}, err
	}
	promptHash, err := hashutil.Tree(frozenPrompts)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		RunID:         runID,
		CreatedAt:     createdAt.Format(time.RFC3339Nano),
		RunDir:        runDir,
		Inputs: Inputs{
			Problem: InputDigest{
				Source:     filepath.Base(req.ProblemPath),
				FrozenPath: "inputs/problem.md",
				SHA256:     problemHash,
			},
			Config: InputDigest{
				Source:     configSource(req.ConfigPath),
				FrozenPath: "inputs/council.yaml",
				SHA256:     configHash,
			},
			PromptBundle: InputDigest{
				Source:     promptSource(req.PromptsDir),
				FrozenPath: "inputs/prompts",
				SHA256:     promptHash,
			},
		},
	}
	if frozenReference != "" {
		referenceHash, err := hashutil.File(frozenReference)
		if err != nil {
			return Manifest{}, err
		}
		manifest.Inputs.ReferenceSet = InputDigest{
			Source:     filepath.Base(req.ReferenceSetPath),
			FrozenPath: "inputs/reference-set.json",
			SHA256:     referenceHash,
		}
	}

	if err := writeManifest(filepath.Join(runDir, "manifest.json"), manifest); err != nil {
		return Manifest{}, err
	}
	if err := writeEvent(filepath.Join(runDir, "events.jsonl"), map[string]any{
		"timestamp": manifest.CreatedAt,
		"event":     "run_created",
		"run_id":    runID,
	}); err != nil {
		return Manifest{}, err
	}

	committed = true
	return manifest, nil
}

func newRunID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}

func validateRunID(runID string) error {
	if runID == "" || runID == "." || runID == ".." || filepath.Base(runID) != runID || strings.ContainsAny(runID, `/\\`) {
		return fmt.Errorf("invalid run id %q", runID)
	}
	return nil
}

func copyRegularFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", src)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok {
			_ = os.Remove(dst)
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source tree contains symlink %q", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("source tree contains non-regular file %q", path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyRegularFile(path, target)
	})
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func writeEvent(path string, event map[string]any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write event log: %w", err)
	}
	return nil
}

func configSource(path string) string {
	if path == "" {
		return "defaults"
	}
	return filepath.Base(path)
}

func promptSource(path string) string {
	if path == "" {
		return "empty"
	}
	return filepath.Base(path)
}
