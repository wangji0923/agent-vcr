package trace

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEventJSONRoundTrip(t *testing.T) {
	event := NewEvent("run-1", EventToolCall, Source{Adapter: "fixture", RawEventType: "tool"})
	event.EventID = "evt-1"
	event.EventIndex = 2
	event.Payload = Payload{"tool_name": "bash"}
	event.Artifacts = []ArtifactRef{{Kind: ArtifactBlob, Path: "blobs/out.txt"}}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal event: %v", err)
	}
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal event: %v", err)
	}
	if decoded.Type != EventToolCall {
		t.Fatalf("Type = %q, want %q", decoded.Type, EventToolCall)
	}
	if decoded.Payload["tool_name"] != "bash" {
		t.Fatalf("Payload tool_name = %v", decoded.Payload["tool_name"])
	}
}

func TestCoreEventTypeStringsStableAndGeneric(t *testing.T) {
	types := []EventType{
		EventRunStart,
		EventRunStop,
		EventUserPrompt,
		EventModelCall,
		EventModelResult,
		EventToolCall,
		EventToolResult,
		EventToolError,
		EventFileRead,
		EventFileWrite,
		EventFilePatch,
		EventShellCommand,
		EventShellResult,
		EventPermissionRequest,
		EventSubagentStart,
		EventSubagentStop,
		EventContextCompact,
		EventProcessStart,
		EventProcessResult,
		EventRaw,
	}
	for _, typ := range types {
		if !IsKnownEventType(typ) {
			t.Fatalf("%q should be a known event type", typ)
		}
		value := string(typ)
		for _, forbidden := range []string{"codex_", "kimi_", "claude_"} {
			if strings.HasPrefix(value, forbidden) {
				t.Fatalf("event type %q has forbidden agent-specific prefix", value)
			}
		}
	}
	if !IsAgentSpecificEventType(EventType("codex_tool_call")) {
		t.Fatal("codex_ event type should be detected as agent-specific")
	}
}

func TestRawEventCarriesRawRefAndPayload(t *testing.T) {
	ref := ArtifactRef{Kind: ArtifactRaw, Path: "raw/input.json", SHA256: "abc", SizeBytes: 3}
	event := NewRawEvent("run-1", Source{Adapter: "fixture", RawEventType: "unknown"}, ref)
	event.Payload = Payload{"reason": "normalize_failed"}

	if event.Type != EventRaw {
		t.Fatalf("Type = %q, want raw_event", event.Type)
	}
	if event.RawRef == nil || event.RawRef.Path != ref.Path {
		t.Fatalf("RawRef = %#v, want %#v", event.RawRef, ref)
	}
}

func TestMetadataJSONRoundTrip(t *testing.T) {
	ended := time.Now().UTC()
	meta := Metadata{
		SchemaVersion: SchemaVersion,
		RunID:         "run-1",
		Source:        "fixture",
		Status:        RunStatusCompleted,
		Cwd:           "C:/work",
		StartedAt:     ended.Add(-time.Second),
		EndedAt:       &ended,
		Capabilities:  map[string]bool{"tool": true},
		Summary:       Payload{"events": float64(2)},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal metadata: %v", err)
	}
	var decoded Metadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}
	if decoded.RunID != meta.RunID || decoded.Status != RunStatusCompleted {
		t.Fatalf("decoded metadata = %#v", decoded)
	}
}
