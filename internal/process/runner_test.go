package process

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestRunCapturesStdoutStderrAndLines(t *testing.T) {
	store, err := trace.CreateRun(t.TempDir(), "process-test")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	result, err := Run(context.Background(), store, RunOptions{
		Command:      os.Args[0],
		Args:         []string{"-test.run=TestFakeProcessMain"},
		Cwd:          t.TempDir(),
		Env:          []string{"AGENT_VCR_FAKE_PROCESS=ok"},
		StdoutMode:   OutputModeBlob,
		StderrMode:   OutputModeBlob,
		MaxBlobBytes: 1024,
		OnStdoutLine: func(line []byte) error {
			lines = append(lines, string(line))
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("lines = %#v", lines)
	}
	assertBlobContains(t, store, result.StdoutRef, "hello\nworld\n")
	assertBlobContains(t, store, result.StderrRef, "warning\n")
}

func TestRunRecordsNonZeroExitCode(t *testing.T) {
	store, err := trace.CreateRun(t.TempDir(), "process-test")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), store, RunOptions{
		Command:      os.Args[0],
		Args:         []string{"-test.run=TestFakeProcessMain"},
		Cwd:          t.TempDir(),
		Env:          []string{"AGENT_VCR_FAKE_PROCESS=exit"},
		StdoutMode:   OutputModeBlob,
		StderrMode:   OutputModeBlob,
		MaxBlobBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.ExitCode)
	}
}

func TestFakeProcessMain(t *testing.T) {
	switch os.Getenv("AGENT_VCR_FAKE_PROCESS") {
	case "ok":
		fmt.Println("hello")
		fmt.Println("world")
		fmt.Fprintln(os.Stderr, "warning")
		os.Exit(0)
	case "exit":
		fmt.Fprintln(os.Stderr, "failed")
		os.Exit(7)
	default:
		return
	}
}

func assertBlobContains(t *testing.T, store *trace.Store, ref *trace.ArtifactRef, want string) {
	t.Helper()
	if ref == nil {
		t.Fatal("missing artifact ref")
	}
	data, err := os.ReadFile(filepath.Join(store.RunDir, filepath.FromSlash(ref.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("blob = %q, want %q", string(data), want)
	}
}
