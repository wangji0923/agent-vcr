package codex

import (
	"encoding/json"
	"strings"
)

type HookInput struct {
	SessionID            string         `json:"session_id"`
	TranscriptPath       string         `json:"transcript_path"`
	Cwd                  string         `json:"cwd"`
	HookEventName        string         `json:"hook_event_name"`
	Model                string         `json:"model"`
	PermissionMode       string         `json:"permission_mode,omitempty"`
	TurnID               string         `json:"turn_id,omitempty"`
	ToolName             string         `json:"tool_name,omitempty"`
	ToolUseID            string         `json:"tool_use_id,omitempty"`
	ToolInput            map[string]any `json:"tool_input,omitempty"`
	ToolResponse         any            `json:"tool_response,omitempty"`
	Prompt               string         `json:"prompt,omitempty"`
	LastAssistantMessage string         `json:"last_assistant_message,omitempty"`
	Raw                  map[string]any `json:"-"`
}

func ParseHookInput(raw []byte) (HookInput, map[string]any, error) {
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return HookInput{}, nil, err
	}

	var input HookInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return HookInput{}, nil, err
	}
	if input.HookEventName == "" {
		input.HookEventName = hookFirstString(rawMap, "event", "event_name", "hook_event", "hookEventName")
	}
	input.Raw = rawMap
	return input, rawMap, nil
}

func hookFirstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := hookStringValue(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func hookStringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}
