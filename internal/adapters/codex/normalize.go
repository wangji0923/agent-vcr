package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/redact"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const (
	eventSessionStart     = "SessionStart"
	eventUserPromptSubmit = "UserPromptSubmit"
	eventPreToolUse       = "PreToolUse"
	eventPostToolUse      = "PostToolUse"
	eventPermission       = "PermissionRequest"
	eventPreCompact       = "PreCompact"
	eventPostCompact      = "PostCompact"
	eventSubagentStart    = "SubagentStart"
	eventSubagentStop     = "SubagentStop"
	eventStop             = "Stop"
)

func NormalizeHook(raw trace.RawEvent) []trace.Event {
	input, rawMap, err := hookInputFromRaw(raw)
	source := raw.Source
	if source.Adapter == "" {
		source.Adapter = SourceAdapter
	}
	if source.Agent == "" {
		source.Agent = SourceAgent
	}
	if source.RawEventType == "" && input.HookEventName != "" {
		source.RawEventType = input.HookEventName
	}
	runID := hookFirstString(rawMap, "run_id")

	if err != nil {
		return []trace.Event{rawEvent(runID, source, raw, trace.Payload{
			"reason": "parse_error",
			"error":  err.Error(),
		})}
	}

	switch input.HookEventName {
	case eventSessionStart:
		return []trace.Event{normalEvent(runID, trace.EventRunStart, source, raw, trace.Payload{
			"session_id":      input.SessionID,
			"model":           input.Model,
			"permission_mode": input.PermissionMode,
			"transcript_path": cleanPath(input.TranscriptPath),
		})}
	case eventUserPromptSubmit:
		payload := trace.Payload{
			"turn_id":       input.TurnID,
			"prompt_sha256": sha256String(input.Prompt),
		}
		if captured := capturePrompt(input.Prompt); captured != nil {
			payload["prompt"] = captured
		}
		return []trace.Event{normalEvent(runID, trace.EventUserPrompt, source, raw, payload)}
	case eventPreToolUse:
		return normalizePreToolUse(runID, source, raw, input)
	case eventPostToolUse:
		return normalizePostToolUse(runID, source, raw, input)
	case eventPermission:
		return []trace.Event{normalEvent(runID, trace.EventPermissionRequest, source, raw, trace.Payload{
			"tool_name":       input.ToolName,
			"tool_input":      summarizeToolInput(input.ToolName, input.ToolInput),
			"permission_mode": input.PermissionMode,
		})}
	case eventPreCompact:
		return []trace.Event{normalEvent(runID, trace.EventContextCompact, source, raw, trace.Payload{
			"phase": "pre",
			"mode":  compactMode(rawMap),
		})}
	case eventPostCompact:
		return []trace.Event{normalEvent(runID, trace.EventContextCompact, source, raw, trace.Payload{
			"phase": "post",
			"mode":  compactMode(rawMap),
		})}
	case eventSubagentStart:
		return []trace.Event{normalEvent(runID, trace.EventSubagentStart, source, raw, subagentPayload(rawMap, input))}
	case eventSubagentStop:
		return []trace.Event{normalEvent(runID, trace.EventSubagentStop, source, raw, subagentPayload(rawMap, input))}
	case eventStop:
		payload := trace.Payload{
			"turn_id":                       input.TurnID,
			"last_assistant_message_sha256": sha256String(input.LastAssistantMessage),
		}
		if input.LastAssistantMessage != "" {
			payload["last_assistant_message"] = "[REDACTED:assistant_message]"
		}
		return []trace.Event{normalEvent(runID, trace.EventRunStop, source, raw, payload)}
	default:
		return []trace.Event{rawEvent(runID, source, raw, trace.Payload{
			"reason": "unknown_event",
		})}
	}
}

func normalizePreToolUse(runID string, source trace.Source, raw trace.RawEvent, input HookInput) []trace.Event {
	payload := trace.Payload{
		"turn_id":     input.TurnID,
		"tool_use_id": input.ToolUseID,
		"tool_name":   input.ToolName,
		"input":       summarizeToolInput(input.ToolName, input.ToolInput),
	}
	addMCPFields(payload, input.ToolName)

	events := []trace.Event{normalEvent(runID, trace.EventToolCall, source, raw, payload)}
	switch {
	case isBashTool(input.ToolName):
		command := toolInputString(input.ToolInput, "command", "cmd")
		events = append(events, normalEvent(runID, trace.EventShellCommand, source, raw, trace.Payload{
			"turn_id":        input.TurnID,
			"tool_use_id":    input.ToolUseID,
			"tool_name":      input.ToolName,
			"command":        redactString(command),
			"command_sha256": sha256String(command),
			"description":    redactString(toolInputString(input.ToolInput, "description")),
		}))
	case isApplyPatchTool(input.ToolName):
		patch := toolInputString(input.ToolInput, "patch", "input", "content")
		if patch != "" {
			events = append(events, normalEvent(runID, trace.EventFilePatch, source, raw, trace.Payload{
				"turn_id":          input.TurnID,
				"tool_use_id":      input.ToolUseID,
				"tool_name":        input.ToolName,
				"patch_sha256":     sha256String(patch),
				"patch_size_bytes": len([]byte(patch)),
			}))
		}
	}
	return events
}

func normalizePostToolUse(runID string, source trace.Source, raw trace.RawEvent, input HookInput) []trace.Event {
	summary := summarizeToolResponse(input.ToolResponse)
	payload := trace.Payload{
		"turn_id":     input.TurnID,
		"tool_use_id": input.ToolUseID,
		"tool_name":   input.ToolName,
		"result":      summary,
	}
	addMCPFields(payload, input.ToolName)

	events := []trace.Event{normalEvent(runID, trace.EventToolResult, source, raw, payload)}
	if isBashTool(input.ToolName) {
		shell := trace.Payload{
			"turn_id":     input.TurnID,
			"tool_use_id": input.ToolUseID,
			"tool_name":   input.ToolName,
		}
		if exitCode, ok := responseExitCode(input.ToolResponse); ok {
			shell["exit_code"] = exitCode
		}
		for _, field := range []string{"stdout", "stderr", "output"} {
			if value := responseString(input.ToolResponse, field); value != "" {
				shell[field+"_sha256"] = sha256String(value)
				shell[field+"_size_bytes"] = len([]byte(value))
			}
		}
		if isError, ok := responseBool(input.ToolResponse, "is_error", "error"); ok {
			shell["is_error"] = isError
		}
		events = append(events, normalEvent(runID, trace.EventShellResult, source, raw, shell))
	}
	return events
}

func hookInputFromRaw(raw trace.RawEvent) (HookInput, map[string]any, error) {
	if len(raw.Data) > 0 {
		return ParseHookInput(raw.Data)
	}
	if raw.Payload != nil {
		data, err := json.Marshal(raw.Payload)
		if err != nil {
			return HookInput{}, nil, err
		}
		return ParseHookInput(data)
	}
	return HookInput{}, nil, fmt.Errorf("raw hook input is empty")
}

func normalEvent(runID string, typ trace.EventType, source trace.Source, raw trace.RawEvent, payload trace.Payload) trace.Event {
	event := trace.NewEvent(runID, typ, source)
	event.Payload = dropEmpty(payload)
	if raw.RawRef != nil {
		event.RawRef = raw.RawRef
	}
	if !raw.ReceivedAt.IsZero() {
		event.Timestamp = raw.ReceivedAt.UTC()
	}
	return event
}

func rawEvent(runID string, source trace.Source, raw trace.RawEvent, payload trace.Payload) trace.Event {
	event := trace.NewEvent(runID, trace.EventRaw, source)
	event.Payload = payload
	if raw.RawRef != nil {
		event.RawRef = raw.RawRef
	}
	if !raw.ReceivedAt.IsZero() {
		event.Timestamp = raw.ReceivedAt.UTC()
	}
	return event
}

func dropEmpty(payload trace.Payload) trace.Payload {
	for key, value := range payload {
		switch v := value.(type) {
		case string:
			if v == "" {
				delete(payload, key)
			}
		case map[string]any:
			if len(v) == 0 {
				delete(payload, key)
			}
		}
	}
	return payload
}

func summarizeToolInput(toolName string, input map[string]any) map[string]any {
	data, _ := json.Marshal(input)
	summary := map[string]any{
		"sha256":     sha256Bytes(data),
		"size_bytes": len(data),
		"redacted":   true,
	}
	if isBashTool(toolName) {
		if command := toolInputString(input, "command", "cmd"); command != "" {
			summary["command_sha256"] = sha256String(command)
		}
	}
	if isApplyPatchTool(toolName) {
		if patch := toolInputString(input, "patch", "input", "content"); patch != "" {
			summary["patch_sha256"] = sha256String(patch)
			summary["patch_size_bytes"] = len([]byte(patch))
		}
	}
	addMCPFieldsToMap(summary, toolName)
	return summary
}

func summarizeToolResponse(response any) map[string]any {
	data, _ := json.Marshal(response)
	summary := map[string]any{
		"sha256":     sha256Bytes(data),
		"size_bytes": len(data),
		"mode":       "hash",
	}
	if exitCode, ok := responseExitCode(response); ok {
		summary["exit_code"] = exitCode
	}
	if isError, ok := responseBool(response, "is_error", "error"); ok {
		summary["is_error"] = isError
	}
	return summary
}

func toolInputString(input map[string]any, keys ...string) string {
	if input == nil {
		return ""
	}
	for _, key := range keys {
		if value := hookStringValue(input[key]); value != "" {
			return value
		}
	}
	return ""
}

func responseString(response any, keys ...string) string {
	switch value := response.(type) {
	case string:
		if len(keys) == 0 || keys[0] == "output" {
			return value
		}
	case map[string]any:
		return hookFirstString(value, keys...)
	case map[string]string:
		for _, key := range keys {
			if value[key] != "" {
				return value[key]
			}
		}
	}
	return ""
}

func responseExitCode(response any) (int, bool) {
	switch value := response.(type) {
	case map[string]any:
		for _, key := range []string{"exit_code", "exitCode", "code"} {
			switch v := value[key].(type) {
			case float64:
				return int(v), true
			case int:
				return v, true
			case string:
				parsed, err := strconv.Atoi(strings.TrimSpace(v))
				if err == nil {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}

func responseBool(response any, keys ...string) (bool, bool) {
	value, ok := response.(map[string]any)
	if !ok {
		return false, false
	}
	for _, key := range keys {
		switch v := value[key].(type) {
		case bool:
			return v, true
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			if err == nil {
				return parsed, true
			}
		}
	}
	return false, false
}

func compactMode(rawMap map[string]any) string {
	mode := hookFirstString(rawMap, "mode", "compact_mode", "trigger", "reason")
	switch mode {
	case "manual", "auto":
		return mode
	default:
		return mode
	}
}

func subagentPayload(rawMap map[string]any, input HookInput) trace.Payload {
	payload := trace.Payload{
		"turn_id": input.TurnID,
	}
	for _, key := range []string{"subagent_id", "subagent_name", "name", "task", "status"} {
		if value := hookFirstString(rawMap, key); value != "" {
			payload[key] = redactString(value)
		}
	}
	return payload
}

func addMCPFields(payload trace.Payload, toolName string) {
	for key, value := range parseMCPTool(toolName) {
		payload[key] = value
	}
}

func addMCPFieldsToMap(payload map[string]any, toolName string) {
	for key, value := range parseMCPTool(toolName) {
		payload[key] = value
	}
}

func parseMCPTool(toolName string) map[string]string {
	if !strings.HasPrefix(toolName, "mcp__") {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(toolName, "mcp__"), "__")
	fields := map[string]string{}
	if len(parts) > 0 && parts[0] != "" {
		fields["mcp_server"] = parts[0]
	}
	if len(parts) > 1 {
		fields["mcp_tool"] = strings.Join(parts[1:], "__")
	}
	return fields
}

func isBashTool(toolName string) bool {
	return strings.EqualFold(toolName, "Bash") || strings.EqualFold(toolName, "bash")
}

func isApplyPatchTool(toolName string) bool {
	return strings.EqualFold(toolName, "apply_patch")
}

func capturePrompt(prompt string) any {
	if prompt == "" {
		return ""
	}
	return "[REDACTED:prompt]"
}

func redactString(value string) string {
	if value == "" {
		return ""
	}
	masked, err := redact.MaskString(value, config.Default().Redaction)
	if err != nil {
		return value
	}
	return masked
}

func sha256String(value string) string {
	return sha256Bytes([]byte(value))
}

func sha256Bytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
