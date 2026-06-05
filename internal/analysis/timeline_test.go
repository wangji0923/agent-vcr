package analysis

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestBuildTimelineMergesToolCallAndResult(t *testing.T) {
	base := time.Date(2026, 6, 4, 2, 0, 0, 0, time.UTC)
	events := []trace.Event{
		{
			EventIndex: 1,
			Type:       trace.EventToolCall,
			Timestamp:  base,
			Payload: trace.Payload{
				"tool_use_id": "u1",
				"tool_name":   "Bash",
			},
		},
		{
			EventIndex: 2,
			Type:       trace.EventShellCommand,
			Timestamp:  base,
			Payload: trace.Payload{
				"tool_use_id": "u1",
				"tool_name":   "Bash",
				"command":     "go test ./...",
			},
		},
		{
			EventIndex: 3,
			Type:       trace.EventToolResult,
			Timestamp:  base.Add(time.Second),
			Payload: trace.Payload{
				"tool_use_id": "u1",
				"tool_name":   "Bash",
				"result": map[string]any{
					"exit_code": float64(0),
				},
			},
		},
	}

	timeline := BuildTimeline(events)
	if len(timeline) != 1 {
		t.Fatalf("len(timeline) = %d, want 1: %#v", len(timeline), timeline)
	}
	item := timeline[0]
	if item.Type != "tool" || item.ToolUseID != "u1" || item.ToolName != "Bash" {
		t.Fatalf("merged item = %#v", item)
	}
	if item.ExitCode == nil || *item.ExitCode != 0 {
		t.Fatalf("exit code = %#v, want 0", item.ExitCode)
	}
	if !hasEventTypeString(item, string(trace.EventToolCall)) ||
		!hasEventTypeString(item, string(trace.EventShellCommand)) ||
		!hasEventTypeString(item, string(trace.EventToolResult)) {
		t.Fatalf("event types = %#v", item.EventTypes)
	}
}

func TestBuildTimelineMergesByParentSpan(t *testing.T) {
	events := []trace.Event{
		{
			EventIndex: 1,
			Type:       trace.EventToolCall,
			SpanID:     "span-a",
			Payload:    trace.Payload{"tool_name": "shell"},
		},
		{
			EventIndex: 2,
			Type:       trace.EventToolResult,
			ParentID:   "span-a",
			Payload:    trace.Payload{"tool_name": "shell", "exit_code": 7},
		},
	}

	timeline := BuildTimeline(events)
	if len(timeline) != 1 {
		t.Fatalf("len(timeline) = %d, want 1", len(timeline))
	}
	if timeline[0].ExitCode == nil || *timeline[0].ExitCode != 7 {
		t.Fatalf("merged item = %#v", timeline[0])
	}
}

func TestBuildTimelineDisplaysRawEvent(t *testing.T) {
	events := []trace.Event{
		{
			EventID:    "evt-raw",
			EventIndex: 1,
			Type:       trace.EventRaw,
			RawRef: &trace.ArtifactRef{
				Kind:      trace.ArtifactRaw,
				Path:      "raw/unknown.json",
				SizeBytes: 12,
			},
		},
	}

	timeline := BuildTimeline(events)
	if len(timeline) != 1 || timeline[0].Type != "raw" {
		t.Fatalf("timeline = %#v", timeline)
	}
	if timeline[0].RawEventID != "evt-raw" || timeline[0].Detail != "raw/unknown.json (12 bytes)" {
		t.Fatalf("raw item = %#v", timeline[0])
	}
}

func TestLoadReplayMissingMetadataDoesNotPanic(t *testing.T) {
	project := t.TempDir()
	runDir := filepath.Join(project, ".agent-vcr", "runs", "missing-meta")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "trace.ndjson"), []byte(`{"schema_version":"0.2","event_index":1,"type":"run_start","source":{"adapter":"fixture"},"timestamp":"2026-06-04T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	replay, err := LoadReplay(project, "missing-meta")
	if err != nil {
		t.Fatalf("LoadReplay: %v", err)
	}
	if !replay.MetadataMissing {
		t.Fatalf("MetadataMissing = false, replay = %#v", replay)
	}
	if replay.Metadata.Source != "fixture" || replay.Metadata.Status != trace.RunStatusRunning {
		t.Fatalf("metadata fallback = %#v", replay.Metadata)
	}
}
