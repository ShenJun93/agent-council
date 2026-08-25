package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesV0Config(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "council.yaml")
	body := []byte(`runs:
  root: .council/custom-runs
billing:
  mode: subscription_only
  fail_closed: true
  allow_metered_fallback: false
`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runs.Root != ".council/custom-runs" {
		t.Fatalf("Runs.Root = %q", cfg.Runs.Root)
	}
	if cfg.Billing.Mode != "subscription_only" {
		t.Fatalf("Billing.Mode = %q", cfg.Billing.Mode)
	}
	if !cfg.Billing.FailClosed {
		t.Fatal("Billing.FailClosed = false, want true")
	}
	if cfg.Billing.AllowMeteredFallback {
		t.Fatal("Billing.AllowMeteredFallback = true, want false")
	}
}

func TestLoadEmptyPathUsesSafeDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runs.Root != ".council/runs" {
		t.Fatalf("Runs.Root = %q", cfg.Runs.Root)
	}
	if cfg.Billing.Mode != "subscription_only" || !cfg.Billing.FailClosed || cfg.Billing.AllowMeteredFallback {
		t.Fatalf("unsafe billing defaults: %+v", cfg.Billing)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "council.yaml")
	if err := os.WriteFile(path, []byte("billing:\n  surprise: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown key")
	}
}
