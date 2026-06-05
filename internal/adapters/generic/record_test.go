package generic

import (
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/process"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestCapabilities(t *testing.T) {
	caps := Adapter{}.Capabilities()
	if !caps.FileDiffCapture || !caps.CanRunAsWrapper {
		t.Fatalf("generic adapter should capture final diffs as a wrapper: %#v", caps)
	}
	if caps.ToolCallCapture || caps.PromptCapture {
		t.Fatalf("generic adapter should not claim structured agent capture: %#v", caps)
	}
}

func TestProcessResultEventPayload(t *testing.T) {
	started := time.Now().UTC()
	stdout := &trace.ArtifactRef{Kind: trace.ArtifactBlob, Path: "blobs/process_stdout.txt"}
	stderr := &trace.ArtifactRef{Kind: trace.ArtifactBlob, Path: "blobs/process_stderr.txt"}
	diff := &trace.ArtifactRef{Kind: trace.ArtifactPatch, Path: "patches/final.diff"}
	event := ProcessResultEvent(ProcessResultInput{
		Command:      "agent",
		Args:         []string{"task"},
		Cwd:          "/repo",
		ChangedFiles: []string{"src/a.go"},
		StdoutRef:    stdout,
		StderrRef:    stderr,
		FinalDiffRef: diff,
		Result: process.RunResult{
			ExitCode:  0,
			StartedAt: started,
			EndedAt:   started.Add(10 * time.Millisecond),
		},
	})
	if event.Type != trace.EventProcessResult {
		t.Fatalf("type = %s", event.Type)
	}
	if event.Payload["stdout_blob"] != stdout.Path || event.Payload["stderr_blob"] != stderr.Path || event.Payload["final_diff_blob"] != diff.Path {
		t.Fatalf("blob payload = %#v", event.Payload)
	}
}
