package codex

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/adapters"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const JSONLAdapterName = "codex-jsonl"

type JSONLAdapter struct{}

func init() {
	adapters.Register(JSONLAdapter{})
}

func (JSONLAdapter) Name() string {
	return JSONLAdapterName
}

func (JSONLAdapter) DisplayName() string {
	return "Codex JSONL"
}

func (JSONLAdapter) Probe(ctx context.Context) (*adapters.ProbeResult, error) {
	path, err := exec.LookPath("codex")
	if err != nil {
		return &adapters.ProbeResult{Found: false}, nil
	}
	return &adapters.ProbeResult{Found: true, Details: map[string]string{"path": path}}, nil
}

func (JSONLAdapter) Install(ctx context.Context, opts adapters.InstallOptions) error {
	return nil
}

func (JSONLAdapter) Uninstall(ctx context.Context, opts adapters.InstallOptions) error {
	return nil
}

func (JSONLAdapter) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{
		PromptCapture:      true,
		ModelCallCapture:   true,
		ModelResultCapture: true,
		ToolCallCapture:    true,
		ToolResultCapture:  true,
		ShellCapture:       true,
		FileDiffCapture:    true,
		PermissionCapture:  true,
		CanRunAsWrapper:    true,
		CanImportTrace:     true,
	}
}

func (JSONLAdapter) MatchesCommand(command string, args []string) bool {
	return isCodexCommand(command) && hasArg(args, "exec") && hasArg(args, "--json")
}

func (JSONLAdapter) CommandHint(command string, args []string) string {
	if isCodexCommand(command) && hasArg(args, "exec") && !hasArg(args, "--json") {
		return "codex exec without --json will be recorded with generic-cli; use codex exec --json for structured JSONL capture"
	}
	return ""
}

func (JSONLAdapter) Normalize(ctx context.Context, raw trace.RawEvent) ([]trace.Event, error) {
	if raw.Payload == nil {
		return nil, nil
	}
	return NormalizeJSONL(map[string]any(raw.Payload)), nil
}

func (adapter JSONLAdapter) NormalizeLine(ctx context.Context, line []byte) ([]trace.Event, trace.RawEvent, bool, error) {
	raw, err := ParseJSONLLine(line)
	if err != nil {
		return nil, trace.RawEvent{
			Source: trace.Source{
				Adapter:      JSONLAdapterName,
				Agent:        "codex",
				RawEventType: "invalid_jsonl",
			},
			Data: line,
			Payload: trace.Payload{
				"normalize_error": err.Error(),
			},
		}, true, nil
	}

	rawEvent := trace.RawEvent{
		Source:  SourceForJSONL(raw),
		Data:    line,
		Payload: trace.Payload(raw),
	}
	events, normalizeErr := adapter.Normalize(ctx, rawEvent)
	if normalizeErr != nil {
		rawEvent.Payload = trace.Payload{
			"normalize_error": normalizeErr.Error(),
			"raw_event_type":  RawEventType(raw),
		}
		return nil, rawEvent, true, nil
	}
	if len(events) == 0 {
		rawEvent.Payload = trace.Payload{
			"normalize_error": "unknown jsonl event",
			"raw_event_type":  RawEventType(raw),
		}
		return nil, rawEvent, true, nil
	}
	return events, trace.RawEvent{}, false, nil
}

func ParseJSONLLine(line []byte) (map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func NormalizeJSONL(raw map[string]any) []trace.Event {
	rawType := RawEventType(raw)
	source := SourceForJSONL(raw)
	switch rawType {
	case "thread.started":
		event := trace.NewEvent("", trace.EventRunStart, source)
		event.Payload = trace.Payload{
			"thread_id": firstString(raw, "thread_id", "threadId", "id"),
			"status":    "started",
		}
		removeEmpty(event.Payload)
		return []trace.Event{event}
	case "turn.started":
		event := trace.NewEvent("", trace.EventUserPrompt, source)
		event.Payload = trace.Payload{
			"turn_id": firstString(raw, "turn_id", "turnId", "id"),
			"prompt":  firstString(raw, "prompt", "input", "message"),
			"status":  "started",
		}
		removeEmpty(event.Payload)
		return []trace.Event{event}
	case "turn.completed":
		event := trace.NewEvent("", trace.EventRunStop, source)
		event.Payload = trace.Payload{
			"turn_id": firstString(raw, "turn_id", "turnId", "id"),
			"status":  "completed",
		}
		removeEmpty(event.Payload)
		return []trace.Event{event}
	case "turn.failed":
		event := trace.NewEvent("", trace.EventRunStop, source)
		event.Payload = trace.Payload{
			"turn_id": firstString(raw, "turn_id", "turnId", "id"),
			"status":  "failed",
			"error":   errorString(raw),
		}
		removeEmpty(event.Payload)
		return []trace.Event{event}
	case "error":
		event := trace.NewEvent("", trace.EventToolError, source)
		event.Payload = trace.Payload{
			"error": errorString(raw),
		}
		removeEmpty(event.Payload)
		return []trace.Event{event}
	default:
		if strings.HasPrefix(rawType, "item.") {
			if event, ok := normalizeItem(rawType, raw, source); ok {
				return []trace.Event{event}
			}
		}
		return nil
	}
}

func RawEventType(raw map[string]any) string {
	for _, key := range []string{"type", "event", "kind"} {
		if value := stringValue(raw[key]); value != "" {
			return value
		}
	}
	return "unknown"
}

func SourceForJSONL(raw map[string]any) trace.Source {
	return trace.Source{
		Adapter:      JSONLAdapterName,
		Agent:        "codex",
		RawEventType: RawEventType(raw),
	}
}

func normalizeItem(rawType string, raw map[string]any, source trace.Source) (trace.Event, bool) {
	item := nestedMap(raw, "item")
	if len(item) == 0 {
		item = nestedMap(raw, "payload")
	}
	if len(item) == 0 {
		item = raw
	}

	itemType := strings.ToLower(firstString(item, "type", "kind", "item_type"))
	switch {
	case isToolResult(itemType) || (strings.Contains(rawType, "completed") && hasAnyString(item, "output", "result")):
		event := trace.NewEvent("", trace.EventToolResult, source)
		event.Payload = trace.Payload{
			"item_type": itemType,
			"tool_name": firstString(item, "tool_name", "name"),
			"call_id":   firstString(item, "call_id", "id"),
			"status":    "completed",
			"output":    firstString(item, "output", "result"),
		}
		removeEmpty(event.Payload)
		return event, true
	case isToolCall(itemType):
		event := trace.NewEvent("", trace.EventToolCall, source)
		event.Payload = trace.Payload{
			"item_type": itemType,
			"tool_name": firstString(item, "tool_name", "name"),
			"call_id":   firstString(item, "call_id", "id"),
			"arguments": valueOrNil(item, "arguments", "input"),
		}
		if command := firstString(item, "command", "cmd"); command != "" {
			event.Payload["command"] = command
		}
		removeEmpty(event.Payload)
		return event, true
	case isShellCommand(itemType):
		event := trace.NewEvent("", trace.EventShellCommand, source)
		event.Payload = trace.Payload{
			"item_type": itemType,
			"command":   firstString(item, "command", "cmd"),
			"args":      valueOrNil(item, "args", "arguments"),
		}
		removeEmpty(event.Payload)
		return event, true
	case isFileChange(itemType):
		event := trace.NewEvent("", trace.EventFilePatch, source)
		event.Payload = trace.Payload{
			"item_type": itemType,
			"path":      firstString(item, "path", "file", "filename"),
			"operation": firstString(item, "operation", "op", "action"),
			"status":    firstString(item, "status"),
		}
		removeEmpty(event.Payload)
		return event, true
	case isModelMessage(itemType):
		eventType := trace.EventModelResult
		if firstString(item, "role") == "user" {
			eventType = trace.EventUserPrompt
		}
		event := trace.NewEvent("", eventType, source)
		event.Payload = trace.Payload{
			"item_type": itemType,
			"role":      firstString(item, "role"),
			"content":   firstString(item, "content", "text", "message"),
		}
		removeEmpty(event.Payload)
		return event, true
	default:
		return trace.Event{}, false
	}
}

func isCodexCommand(command string) bool {
	name := strings.ToLower(filepath.Base(command))
	name = strings.TrimSuffix(name, ".exe")
	return name == "codex"
}

func hasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func isToolCall(itemType string) bool {
	return itemType == "tool_call" ||
		itemType == "function_call" ||
		strings.Contains(itemType, "tool_call") ||
		strings.Contains(itemType, "function_call")
}

func isToolResult(itemType string) bool {
	return itemType == "tool_result" ||
		itemType == "function_call_output" ||
		strings.Contains(itemType, "tool_result") ||
		strings.Contains(itemType, "tool_output")
}

func isShellCommand(itemType string) bool {
	return itemType == "shell_command" ||
		itemType == "exec_command" ||
		itemType == "command" ||
		strings.Contains(itemType, "shell")
}

func isFileChange(itemType string) bool {
	return itemType == "file_change" ||
		itemType == "file_patch" ||
		itemType == "patch" ||
		strings.Contains(itemType, "file") ||
		strings.Contains(itemType, "patch")
}

func isModelMessage(itemType string) bool {
	return itemType == "message" ||
		itemType == "assistant_message" ||
		itemType == "model_message" ||
		strings.Contains(itemType, "message")
}

func nestedMap(raw map[string]any, key string) map[string]any {
	value, ok := raw[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(raw[key]); value != "" {
			return value
		}
	}
	return ""
}

func hasAnyString(raw map[string]any, keys ...string) bool {
	return firstString(raw, keys...) != ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func valueOrNil(raw map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return value
		}
	}
	return nil
}

func errorString(raw map[string]any) string {
	if value := firstString(raw, "error", "message"); value != "" {
		return value
	}
	if errMap := nestedMap(raw, "error"); len(errMap) > 0 {
		return firstString(errMap, "message", "detail", "type")
	}
	return ""
}

func removeEmpty(payload trace.Payload) {
	for key, value := range payload {
		if value == nil || value == "" {
			delete(payload, key)
		}
	}
}
