package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestCapabilities(t *testing.T) {
	caps := New().Capabilities()
	if !caps.PromptCapture || !caps.ToolCallCapture || !caps.ToolResultCapture || !caps.ShellCapture || !caps.CanInstallHooks {
		t.Fatalf("capabilities missing expected capture support: %#v", caps)
	}
	if caps.ModelCallCapture || caps.ModelResultCapture {
		t.Fatalf("model internals should not be captured by hooks: %#v", caps)
	}
}

func TestParseHookInputPreservesRaw(t *testing.T) {
	raw := []byte(`{"hook_event_name":"SessionStart","session_id":"s_1","extra":"kept"}`)
	input, rawMap, err := ParseHookInput(raw)
	if err != nil {
		t.Fatalf("ParseHookInput: %v", err)
	}
	if input.HookEventName != eventSessionStart || input.SessionID != "s_1" {
		t.Fatalf("input = %#v", input)
	}
	if rawMap["extra"] != "kept" || input.Raw["extra"] != "kept" {
		t.Fatalf("raw fields not preserved: %#v", rawMap)
	}
}

func TestNormalizeFixtures(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		types []trace.EventType
	}{
		{name: "session start", file: "session_start.json", types: []trace.EventType{trace.EventRunStart}},
		{name: "user prompt", file: "user_prompt.json", types: []trace.EventType{trace.EventUserPrompt}},
		{name: "pre bash", file: "pre_tool_bash.json", types: []trace.EventType{trace.EventToolCall, trace.EventShellCommand}},
		{name: "post bash", file: "post_tool_bash.json", types: []trace.EventType{trace.EventToolResult, trace.EventShellResult}},
		{name: "permission", file: "permission_request.json", types: []trace.EventType{trace.EventPermissionRequest}},
		{name: "stop", file: "stop.json", types: []trace.EventType{trace.EventRunStop}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := readFixture(t, tt.file)
			events, err := New().Normalize(context.Background(), trace.RawEvent{
				Source: trace.Source{Adapter: SourceAdapter, Agent: SourceAgent},
				Data:   data,
				RawRef: &trace.ArtifactRef{Kind: trace.ArtifactRaw, Path: "raw/" + tt.file},
			})
			if err != nil {
				t.Fatalf("Normalize: %v", err)
			}
			if len(events) != len(tt.types) {
				t.Fatalf("len(events) = %d, want %d: %#v", len(events), len(tt.types), events)
			}
			for i, want := range tt.types {
				if events[i].Type != want {
					t.Fatalf("events[%d].Type = %s, want %s", i, events[i].Type, want)
				}
				if events[i].Source.Adapter != SourceAdapter {
					t.Fatalf("source adapter = %q", events[i].Source.Adapter)
				}
				if events[i].RawRef == nil {
					t.Fatalf("event %s missing raw_ref", events[i].Type)
				}
			}
		})
	}
}

func TestNormalizeUserPromptRedactsAndHashes(t *testing.T) {
	events := NormalizeHook(trace.RawEvent{Data: readFixture(t, "user_prompt.json")})
	if len(events) != 1 || events[0].Type != trace.EventUserPrompt {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Payload["prompt"] != "[REDACTED:prompt]" {
		t.Fatalf("prompt payload = %#v", events[0].Payload)
	}
	if events[0].Payload["prompt_sha256"] == "" {
		t.Fatalf("missing prompt hash: %#v", events[0].Payload)
	}
}

func TestNormalizeUnknownEventReturnsRawEvent(t *testing.T) {
	events := NormalizeHook(trace.RawEvent{
		Data:       []byte(`{"hook_event_name":"SomethingNew","session_id":"s_1"}`),
		ReceivedAt: time.Now().UTC(),
		RawRef:     &trace.ArtifactRef{Kind: trace.ArtifactRaw, Path: "raw/unknown.json"},
	})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d", len(events))
	}
	if events[0].Type != trace.EventRaw {
		t.Fatalf("type = %s, want raw_event", events[0].Type)
	}
	if events[0].RawRef == nil {
		t.Fatal("raw event missing raw_ref")
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
