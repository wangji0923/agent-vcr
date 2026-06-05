package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := Default()
	if err := Validate(cfg); err != nil {
		t.Fatalf("Default() did not validate: %v", err)
	}
	if got, want := cfg.Storage.Dir, ".agent-vcr/runs"; got != want {
		t.Fatalf("Storage.Dir = %q, want %q", got, want)
	}
	if !cfg.Redaction.Enabled {
		t.Fatal("redaction should be enabled by default")
	}
}

func TestLoadMissingConfigReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, path, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load missing config returned error: %v", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	if got, want := cfg.Capture.ToolOutput, Default().Capture.ToolOutput; got != want {
		t.Fatalf("ToolOutput = %q, want %q", got, want)
	}
}

func TestLoadExplicitMissingConfigReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, path, err := Load(dir, "missing.yml")
	if err != nil {
		t.Fatalf("Load explicit missing config returned error: %v", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	if got, want := cfg.Version, Default().Version; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
}

func TestLoadProjectConfigOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".agent-vcr")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yml")
	data := []byte(`
capture:
  prompt: full
  tool_output: inline
storage:
  retention_days: 7
redaction:
  enabled: false
rules:
  max_changed_files: 3
report:
  markdown: true
`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, loaded, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded != configPath {
		t.Fatalf("loaded path = %q, want %q", loaded, configPath)
	}
	if got, want := cfg.Capture.Prompt, "full"; got != want {
		t.Fatalf("Capture.Prompt = %q, want %q", got, want)
	}
	if got, want := cfg.Capture.ToolInput, Default().Capture.ToolInput; got != want {
		t.Fatalf("Capture.ToolInput = %q, want default %q", got, want)
	}
	if cfg.Redaction.Enabled {
		t.Fatal("Redaction.Enabled should be overridden to false")
	}
	if !cfg.Capture.GitDiff {
		t.Fatal("Capture.GitDiff should keep default true")
	}
	if !cfg.Report.Markdown {
		t.Fatal("Report.Markdown should be overridden to true")
	}
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte("capture: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Load(dir, path); err == nil {
		t.Fatal("Load invalid YAML returned nil error")
	}
}

func TestValidateRejectsInvalidEnumAndRegex(t *testing.T) {
	cfg := Default()
	cfg.Capture.ToolOutput = "stdout"
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted invalid tool_output")
	}

	cfg = Default()
	cfg.Redaction.Patterns = append(cfg.Redaction.Patterns, PatternConfig{Name: "bad", Regex: "["})
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted invalid regex")
	}
}
