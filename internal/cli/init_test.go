package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCodexWritesProjectFilesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	stdout, stderr, err := executeCommand(t, "--project-dir", dir, "init", "codex")
	if err != nil {
		t.Fatalf("init codex returned error: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "Installed Codex hooks.") {
		t.Fatalf("stdout = %q", stdout)
	}

	hooksPath := filepath.Join(dir, ".codex", "hooks.json")
	if _, err := os.Stat(hooksPath); err != nil {
		t.Fatalf("hooks.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agent-vcr", "config.yml")); err != nil {
		t.Fatalf("config.yml missing: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	if !strings.Contains(string(gitignore), ".agent-vcr/") {
		t.Fatalf(".gitignore = %q", gitignore)
	}

	firstCount := countAgentVCRHooks(t, hooksPath)
	if firstCount != 10 {
		t.Fatalf("agent-vcr hook count = %d, want 10", firstCount)
	}
	if _, _, err := executeCommand(t, "--project-dir", dir, "init", "codex"); err != nil {
		t.Fatalf("second init codex returned error: %v", err)
	}
	secondCount := countAgentVCRHooks(t, hooksPath)
	if secondCount != firstCount {
		t.Fatalf("second init duplicated hooks: got %d, want %d", secondCount, firstCount)
	}
}

func TestInitCodexPreservesExistingHook(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(hooksDir, "hooks.json")
	existing := []byte(`{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"echo user-hook","timeout":1}]}]}}`)
	if err := os.WriteFile(hooksPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := executeCommand(t, "--project-dir", dir, "init", "codex"); err != nil {
		t.Fatalf("init codex returned error: %v", err)
	}
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "echo user-hook") {
		t.Fatalf("existing hook was not preserved: %s", data)
	}
	if countAgentVCRHooks(t, hooksPath) != 10 {
		t.Fatalf("agent-vcr hooks not installed as expected: %s", data)
	}
}

func countAgentVCRHooks(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing: %#v", root)
	}
	count := 0
	for _, eventEntries := range hooks {
		entries, _ := eventEntries.([]any)
		for _, entry := range entries {
			entryMap, _ := entry.(map[string]any)
			hookList, _ := entryMap["hooks"].([]any)
			for _, hook := range hookList {
				hookMap, _ := hook.(map[string]any)
				if hookMap["command"] == "agent-vcr hook --adapter codex" {
					count++
				}
			}
		}
	}
	return count
}
