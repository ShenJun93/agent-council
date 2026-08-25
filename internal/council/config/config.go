package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultRunsRoot = ".council/runs"
)

type Config struct {
	Runs    RunsConfig
	Billing BillingConfig
}

type RunsConfig struct {
	Root string
}

type BillingConfig struct {
	Mode                 string
	FailClosed           bool
	AllowMeteredFallback bool
}

func Default() Config {
	return Config{
		Runs: RunsConfig{Root: DefaultRunsRoot},
		Billing: BillingConfig{
			Mode:                 "subscription_only",
			FailClosed:           true,
			AllowMeteredFallback: false,
		},
	}
}

// Load parses the intentionally small v0 council YAML surface.
// Unknown keys are rejected so configuration drift fails closed.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var section string
	s := bufio.NewScanner(f)
	lineNo := 0
	for s.Scan() {
		lineNo++
		raw := stripComment(s.Text())
		if strings.TrimSpace(raw) == "" {
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		trimmed := strings.TrimSpace(raw)
		if indent == 0 {
			if !strings.HasSuffix(trimmed, ":") {
				return Config{}, fmt.Errorf("%s:%d: expected section name", path, lineNo)
			}
			section = strings.TrimSuffix(trimmed, ":")
			switch section {
			case "runs", "billing":
			default:
				return Config{}, fmt.Errorf("%s:%d: unknown section %q", path, lineNo, section)
			}
			continue
		}

		if section == "" {
			return Config{}, fmt.Errorf("%s:%d: key outside a section", path, lineNo)
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return Config{}, fmt.Errorf("%s:%d: expected key: value", path, lineNo)
		}
		key := strings.TrimSpace(parts[0])
		value := unquote(strings.TrimSpace(parts[1]))

		switch section + "." + key {
		case "runs.root":
			if value == "" {
				return Config{}, fmt.Errorf("%s:%d: runs.root must not be empty", path, lineNo)
			}
			cfg.Runs.Root = value
		case "billing.mode":
			cfg.Billing.Mode = value
		case "billing.fail_closed":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: billing.fail_closed: %w", path, lineNo, err)
			}
			cfg.Billing.FailClosed = parsed
		case "billing.allow_metered_fallback":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: billing.allow_metered_fallback: %w", path, lineNo, err)
			}
			cfg.Billing.AllowMeteredFallback = parsed
		default:
			return Config{}, fmt.Errorf("%s:%d: unknown key %q in section %q", path, lineNo, key, section)
		}
	}
	if err := s.Err(); err != nil {
		return Config{}, err
	}

	if cfg.Billing.Mode != "subscription_only" {
		return Config{}, fmt.Errorf("%s: unsupported v0 billing mode %q", path, cfg.Billing.Mode)
	}
	if !cfg.Billing.FailClosed {
		return Config{}, fmt.Errorf("%s: billing.fail_closed must be true in v0", path)
	}
	if cfg.Billing.AllowMeteredFallback {
		return Config{}, fmt.Errorf("%s: billing.allow_metered_fallback must be false in v0", path)
	}
	return cfg, nil
}

func CanonicalYAML(cfg Config) []byte {
	return []byte(fmt.Sprintf(
		"runs:\n  root: %s\nbilling:\n  mode: %s\n  fail_closed: %t\n  allow_metered_fallback: %t\n",
		cfg.Runs.Root,
		cfg.Billing.Mode,
		cfg.Billing.FailClosed,
		cfg.Billing.AllowMeteredFallback,
	))
}

func stripComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i]
	}
	return line
}

func unquote(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
