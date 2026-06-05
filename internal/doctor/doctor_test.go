package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDoctorInTemporaryGitRepo(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	project := t.TempDir()
	if err := exec.Command(gitPath, "-C", project, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	writeFile(t, filepath.Join(project, ".gitignore"), ".agent-vcr/\n")
	writeFile(t, filepath.Join(project, "AGENTS.md"), "# instructions\n")
	writeFile(t, filepath.Join(project, ".agent-vcr", "config.yml"), "version: \"0.2\"\n")
	writeFile(t, filepath.Join(project, ".codex", "hooks.json"), `{"hooks":[{"command":"agent-vcr hook --adapter codex-hooks"}]}`)

	result, err := Run(Options{ProjectDir: project})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Core.GitRepo {
		t.Fatalf("expected git repo: %#v", result.Core)
	}
	if !result.Core.ConfigExists || !result.Core.AgentVCRGitignored || !result.Core.RunsDirWritable {
		t.Fatalf("unexpected core result: %#v", result.Core)
	}
	if !result.Codex.HooksExists || !result.Codex.AgentVCRHookInstalled {
		t.Fatalf("unexpected codex hook result: %#v", result.Codex)
	}
}

func TestDoctorJSONOutputIsParseable(t *testing.T) {
	result, err := Run(Options{ProjectDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := RenderJSON(result)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("doctor JSON was not parseable: %v\n%s", err, string(data))
	}
	if decoded.Cwd == "" || len(decoded.Checks) == 0 {
		t.Fatalf("decoded result is incomplete: %#v", decoded)
	}
}

func TestMissingCodexIsWarning(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".gitignore"), ".agent-vcr/\n")
	bin := t.TempDir()
	writeFakeCommand(t, bin, "go", "go version go1.23.0 test")
	writeFakeCommand(t, bin, "git", "true")
	writeFakeCommand(t, bin, "agent-vcr", "agent-vcr test")

	result, err := Run(Options{ProjectDir: project, EnvPath: bin})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, check := range result.Checks {
		if check.Name == "codex binary" {
			if check.Status != StatusWarning {
				t.Fatalf("codex check status = %s, want warning", check.Status)
			}
			return
		}
	}
	t.Fatalf("codex binary check not found: %#v", result.Checks)
}

func TestScanArchitectureFindsAdapterImportViolation(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "internal", "report", "bad.go"), `package report

import _ "github.com/example/project/internal/adapters/codex"
`)
	result := ScanArchitecture(project)
	if !result.ReportImportsAdapters {
		t.Fatalf("expected report import violation: %#v", result)
	}
	if len(result.Violations) == 0 || !strings.Contains(result.Violations[0], "internal/adapters") {
		t.Fatalf("unexpected violations: %#v", result.Violations)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFakeCommand(t *testing.T, dir, name, output string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		writeFile(t, filepath.Join(dir, name+".bat"), "@echo off\r\necho "+output+"\r\n")
		return
	}
	path := filepath.Join(dir, name)
	writeFile(t, path, "#!/bin/sh\necho "+output+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
