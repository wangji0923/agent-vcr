package trace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const SchemaVersion = "0.2"

type EventType string

const (
	EventRunStart          EventType = "run_start"
	EventRunStop           EventType = "run_stop"
	EventUserPrompt        EventType = "user_prompt"
	EventModelCall         EventType = "model_call"
	EventModelResult       EventType = "model_result"
	EventToolCall          EventType = "tool_call"
	EventToolResult        EventType = "tool_result"
	EventToolError         EventType = "tool_error"
	EventFileRead          EventType = "file_read"
	EventFileWrite         EventType = "file_write"
	EventFilePatch         EventType = "file_patch"
	EventShellCommand      EventType = "shell_command"
	EventShellResult       EventType = "shell_result"
	EventPermissionRequest EventType = "permission_request"
	EventSubagentStart     EventType = "subagent_start"
	EventSubagentStop      EventType = "subagent_stop"
	EventContextCompact    EventType = "context_compact"
	EventProcessStart      EventType = "process_start"
	EventProcessResult     EventType = "process_result"
	EventRaw               EventType = "raw_event"
)

type Payload map[string]any

type Source struct {
	Adapter      string `json:"adapter"`
	Agent        string `json:"agent,omitempty"`
	RawEventType string `json:"raw_event_type,omitempty"`
	Version      string `json:"version,omitempty"`
}

type EventSource = Source

type Event struct {
	SchemaVersion string        `json:"schema_version"`
	EventID       string        `json:"event_id"`
	RunID         string        `json:"run_id"`
	ParentID      string        `json:"parent_id,omitempty"`
	SpanID        string        `json:"span_id,omitempty"`
	EventIndex    int64         `json:"event_index"`
	Type          EventType     `json:"type"`
	Source        Source        `json:"source"`
	Timestamp     time.Time     `json:"timestamp"`
	Payload       Payload       `json:"payload,omitempty"`
	Artifacts     []ArtifactRef `json:"artifacts,omitempty"`
	RawRef        *ArtifactRef  `json:"raw_ref,omitempty"`
}

func NewEvent(runID string, typ EventType, source Source) Event {
	return Event{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		Type:          typ,
		Source:        source,
		Timestamp:     time.Now().UTC(),
	}
}

func NewRawEvent(runID string, source Source, raw ArtifactRef) Event {
	event := NewEvent(runID, EventRaw, source)
	event.RawRef = &raw
	return event
}

func IsKnownEventType(typ EventType) bool {
	switch typ {
	case EventRunStart,
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
		EventRaw:
		return true
	default:
		return false
	}
}

func IsAgentSpecificEventType(typ EventType) bool {
	value := string(typ)
	return strings.HasPrefix(value, "codex_") ||
		strings.HasPrefix(value, "kimi_") ||
		strings.HasPrefix(value, "claude_")
}

func newID(prefix string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(random[:]))
}
