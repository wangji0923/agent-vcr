package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestJSONLFixturesNormalize(t *testing.T) {
	tests := []struct {
		name string
		file string
		want trace.EventType
	}{
		{name: "thread started", file: "thread_started.jsonl", want: trace.EventRunStart},
		{name: "tool call", file: "tool_call.jsonl", want: trace.EventToolCall},
		{name: "file change", file: "file_change.jsonl", want: trace.EventFilePatch},
		{name: "error", file: "error.jsonl", want: trace.EventToolError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := ParseJSONLLine(readJSONLFixture(t, tt.file))
			if err != nil {
				t.Fatalf("ParseJSONLLine: %v", err)
			}
			events := NormalizeJSONL(raw)
			if len(events) != 1 {
				t.Fatalf("len(events) = %d, want 1: %#v", len(events), events)
			}
			if events[0].Type != tt.want {
				t.Fatalf("event type = %s, want %s", events[0].Type, tt.want)
			}
			if events[0].Source.Adapter != JSONLAdapterName {
				t.Fatalf("source adapter = %q", events[0].Source.Adapter)
			}
		})
	}
}

func TestJSONLUnknownAndInvalidBecomeRawFallback(t *testing.T) {
	adapter := JSONLAdapter{}

	events, raw, saveRaw, err := adapter.NormalizeLine(context.Background(), []byte(`not-json`))
	if err != nil {
		t.Fatalf("NormalizeLine invalid: %v", err)
	}
	if len(events) != 0 || !saveRaw || raw.Source.RawEventType != "invalid_jsonl" {
		t.Fatalf("invalid line result = events:%#v raw:%#v save:%v", events, raw, saveRaw)
	}

	events, raw, saveRaw, err = adapter.NormalizeLine(context.Background(), []byte(`{"type":"new.event","value":1}`))
	if err != nil {
		t.Fatalf("NormalizeLine unknown: %v", err)
	}
	if len(events) != 0 || !saveRaw || raw.Source.RawEventType != "new.event" {
		t.Fatalf("unknown line result = events:%#v raw:%#v save:%v", events, raw, saveRaw)
	}
}

func TestJSONLAdapterCapabilitiesAndCommandMatch(t *testing.T) {
	adapter := JSONLAdapter{}
	caps := adapter.Capabilities()
	if !caps.PromptCapture || !caps.ToolCallCapture || !caps.CanRunAsWrapper {
		t.Fatalf("capabilities missing expected JSONL support: %#v", caps)
	}
	if !adapter.MatchesCommand("codex", []string{"exec", "--json", "task"}) {
		t.Fatal("codex exec --json should match JSONL adapter")
	}
	if adapter.MatchesCommand("codex", []string{"exec", "task"}) {
		t.Fatal("codex exec without --json should not match JSONL adapter")
	}
	if adapter.CommandHint("codex", []string{"exec", "task"}) == "" {
		t.Fatal("expected hint for codex exec without --json")
	}
}

func readJSONLFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "codex-jsonl", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
