package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type TimelineItem struct {
	Index        int64               `json:"index"`
	Time         time.Time           `json:"time,omitempty"`
	Type         string              `json:"type"`
	Title        string              `json:"title"`
	Detail       string              `json:"detail,omitempty"`
	ToolName     string              `json:"tool_name,omitempty"`
	ToolUseID    string              `json:"tool_use_id,omitempty"`
	ExitCode     *int                `json:"exit_code,omitempty"`
	ChangedFiles []string            `json:"changed_files,omitempty"`
	Artifacts    []trace.ArtifactRef `json:"artifacts,omitempty"`
	RawEventID   string              `json:"raw_event_id,omitempty"`
	EventTypes   []string            `json:"event_types,omitempty"`
}

func BuildTimeline(events []trace.Event) []TimelineItem {
	ordered := append([]trace.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].EventIndex != ordered[j].EventIndex {
			return ordered[i].EventIndex < ordered[j].EventIndex
		}
		if !ordered[i].Timestamp.Equal(ordered[j].Timestamp) {
			return ordered[i].Timestamp.Before(ordered[j].Timestamp)
		}
		return ordered[i].EventID < ordered[j].EventID
	})

	var items []TimelineItem
	toolByKey := map[string]int{}
	shellByKey := map[string]int{}

	for _, event := range ordered {
		switch event.Type {
		case trace.EventRunStart:
			items = append(items, baseItem(event, "run_start", runStartTitle(event)))
		case trace.EventUserPrompt:
			items = append(items, baseItem(event, "user_prompt", promptTitle(event)))
		case trace.EventToolCall:
			idx, ok := findItem(toolByKey, lookupKeys(event))
			if !ok {
				items = append(items, baseItem(event, "tool", toolTitle(event, "")))
				idx = len(items) - 1
			}
			mergeToolFields(&items[idx], event)
			registerItem(toolByKey, registerKeys(event), idx)
			registerItem(shellByKey, registerKeys(event), idx)
		case trace.EventShellCommand:
			idx, ok := findItem(toolByKey, lookupKeys(event))
			if !ok {
				idx, ok = findItem(shellByKey, lookupKeys(event))
			}
			if !ok {
				items = append(items, baseItem(event, "shell", shellTitle(event)))
				idx = len(items) - 1
			}
			mergeShellCommand(&items[idx], event)
			registerItem(shellByKey, registerKeys(event), idx)
			registerItem(toolByKey, registerKeys(event), idx)
		case trace.EventToolResult:
			idx, ok := findItem(toolByKey, lookupKeys(event))
			if !ok {
				items = append(items, baseItem(event, "tool_result", toolResultTitle(event)))
				idx = len(items) - 1
			}
			mergeToolFields(&items[idx], event)
			mergeResultFields(&items[idx], event)
			registerItem(toolByKey, registerKeys(event), idx)
		case trace.EventShellResult:
			idx, ok := findItem(shellByKey, lookupKeys(event))
			if !ok {
				idx, ok = findItem(toolByKey, lookupKeys(event))
			}
			if !ok {
				items = append(items, baseItem(event, "shell_result", "shell result"))
				idx = len(items) - 1
			}
			mergeResultFields(&items[idx], event)
			registerItem(shellByKey, registerKeys(event), idx)
		case trace.EventToolError:
			item := baseItem(event, "tool_error", firstPayloadString(event.Payload, "error", "message"))
			if item.Title == "" {
				item.Title = "tool error"
			}
			items = append(items, item)
		case trace.EventFilePatch:
			idx, ok := findItem(toolByKey, lookupKeys(event))
			if !ok {
				items = append(items, baseItem(event, "file_patch", filePatchTitle(event)))
				idx = len(items) - 1
			}
			mergeFilePatch(&items[idx], event)
			registerItem(toolByKey, registerKeys(event), idx)
		case trace.EventFileRead, trace.EventFileWrite:
			item := baseItem(event, string(event.Type), fileEventTitle(event))
			item.ChangedFiles = uniqueSorted(filesFromEvent(event))
			items = append(items, item)
		case trace.EventRaw:
			item := baseItem(event, "raw", "raw event")
			item.RawEventID = event.EventID
			if event.RawRef != nil {
				item.Detail = artifactDetail(*event.RawRef)
			} else if reason := firstPayloadString(event.Payload, "reason", "normalize_error", "error"); reason != "" {
				item.Detail = reason
			}
			items = append(items, item)
		case trace.EventProcessStart:
			items = append(items, baseItem(event, "process_start", commandTitle(event)))
		case trace.EventProcessResult:
			item := baseItem(event, "process_result", commandTitle(event))
			mergeResultFields(&item, event)
			item.ChangedFiles = uniqueSorted(filesFromEvent(event))
			refreshDetail(&item)
			items = append(items, item)
		case trace.EventRunStop:
			items = append(items, baseItem(event, "run_stop", runStopTitle(event)))
		default:
			items = append(items, baseItem(event, string(event.Type), string(event.Type)))
		}
	}

	for i := range items {
		items[i].Index = int64(i + 1)
		items[i].ChangedFiles = uniqueSorted(items[i].ChangedFiles)
		items[i].EventTypes = uniqueSorted(items[i].EventTypes)
		refreshDetail(&items[i])
	}
	return items
}

func baseItem(event trace.Event, typ string, title string) TimelineItem {
	item := TimelineItem{
		Time:       event.Timestamp,
		Type:       typ,
		Title:      fallback(title, typ),
		Artifacts:  append([]trace.ArtifactRef(nil), event.Artifacts...),
		EventTypes: []string{string(event.Type)},
	}
	if event.RawRef != nil {
		item.Artifacts = append(item.Artifacts, *event.RawRef)
	}
	return item
}

func mergeToolFields(item *TimelineItem, event trace.Event) {
	addEventType(item, event.Type)
	item.ToolName = fallback(item.ToolName, firstPayloadString(event.Payload, "tool_name", "name"))
	item.ToolUseID = fallback(item.ToolUseID, firstPayloadString(event.Payload, "tool_use_id", "call_id", "id"))
	command := commandFromEvent(event)
	if item.Type == "tool_result" {
		item.Type = "tool"
	}
	if command != "" || item.Title == "" || item.Type == "tool_result" {
		item.Title = toolTitle(event, command)
		if item.ToolName != "" && command != "" {
			item.Title = item.ToolName + " " + command
		}
	}
	item.Artifacts = append(item.Artifacts, event.Artifacts...)
}

func mergeShellCommand(item *TimelineItem, event trace.Event) {
	addEventType(item, event.Type)
	command := commandFromEvent(event)
	toolName := firstPayloadString(event.Payload, "tool_name", "name")
	if toolName != "" {
		item.ToolName = fallback(item.ToolName, toolName)
	}
	item.ToolUseID = fallback(item.ToolUseID, firstPayloadString(event.Payload, "tool_use_id", "call_id", "id"))
	if command != "" {
		if item.ToolName != "" {
			item.Title = item.ToolName + " " + command
		} else {
			item.Title = command
		}
	}
	item.Artifacts = append(item.Artifacts, event.Artifacts...)
}

func mergeResultFields(item *TimelineItem, event trace.Event) {
	addEventType(item, event.Type)
	item.ToolName = fallback(item.ToolName, firstPayloadString(event.Payload, "tool_name", "name"))
	item.ToolUseID = fallback(item.ToolUseID, firstPayloadString(event.Payload, "tool_use_id", "call_id", "id"))
	if code, ok := exitCodeFromPayload(event.Payload); ok {
		item.ExitCode = &code
	}
	item.Artifacts = append(item.Artifacts, event.Artifacts...)
	refreshDetail(item)
}

func mergeFilePatch(item *TimelineItem, event trace.Event) {
	addEventType(item, event.Type)
	item.ChangedFiles = append(item.ChangedFiles, filesFromEvent(event)...)
	item.ToolName = fallback(item.ToolName, firstPayloadString(event.Payload, "tool_name", "name"))
	item.ToolUseID = fallback(item.ToolUseID, firstPayloadString(event.Payload, "tool_use_id", "call_id", "id"))
	if item.Title == "" || item.Title == "file_patch" {
		item.Title = filePatchTitle(event)
	}
	item.Artifacts = append(item.Artifacts, event.Artifacts...)
	refreshDetail(item)
}

func refreshDetail(item *TimelineItem) {
	var parts []string
	if item.ExitCode != nil {
		parts = append(parts, fmt.Sprintf("exit %d", *item.ExitCode))
	}
	if len(item.ChangedFiles) > 0 {
		parts = append(parts, fmt.Sprintf("changed %d files", len(uniqueSorted(item.ChangedFiles))))
	}
	if len(parts) > 0 {
		item.Detail = strings.Join(parts, ", ")
	}
}

func runStartTitle(event trace.Event) string {
	if title := commandTitle(event); title != "" {
		return title
	}
	if status := firstPayloadString(event.Payload, "status"); status != "" {
		return "run " + status
	}
	return "session started"
}

func runStopTitle(event trace.Event) string {
	if status := firstPayloadString(event.Payload, "status"); status != "" {
		return status
	}
	if err := firstPayloadString(event.Payload, "error"); err != "" {
		return err
	}
	return "completed"
}

func promptTitle(event trace.Event) string {
	for _, key := range []string{"prompt", "content", "message", "text"} {
		if value := firstPayloadString(event.Payload, key); value != "" {
			return singleLine(value)
		}
	}
	if sha := firstPayloadString(event.Payload, "prompt_sha256"); sha != "" {
		return "prompt sha256 " + shortHash(sha)
	}
	return "user prompt"
}

func toolTitle(event trace.Event, command string) string {
	name := firstPayloadString(event.Payload, "tool_name", "name")
	if command == "" {
		command = commandFromEvent(event)
	}
	switch {
	case name != "" && command != "":
		return name + " " + command
	case name != "":
		return name
	case command != "":
		return command
	default:
		return "tool call"
	}
}

func toolResultTitle(event trace.Event) string {
	if name := firstPayloadString(event.Payload, "tool_name", "name"); name != "" {
		return name
	}
	return "tool result"
}

func shellTitle(event trace.Event) string {
	if command := commandFromEvent(event); command != "" {
		return command
	}
	return "shell command"
}

func filePatchTitle(event trace.Event) string {
	if path := firstPayloadString(event.Payload, "path", "file", "filename"); path != "" {
		return path
	}
	if name := firstPayloadString(event.Payload, "tool_name", "name"); name != "" {
		return name
	}
	return "file patch"
}

func fileEventTitle(event trace.Event) string {
	if path := firstPayloadString(event.Payload, "path", "file", "filename"); path != "" {
		return path
	}
	return string(event.Type)
}

func commandTitle(event trace.Event) string {
	command := commandFromEvent(event)
	if command == "" {
		return ""
	}
	args := strings.Join(stringSliceFromAny(event.Payload["args"]), " ")
	if args == "" {
		return command
	}
	return strings.TrimSpace(command + " " + args)
}

func commandFromEvent(event trace.Event) string {
	if command := firstPayloadString(event.Payload, "command", "cmd"); command != "" {
		return singleLine(command)
	}
	for _, key := range []string{"arguments", "input", "tool_input"} {
		if nested, ok := event.Payload[key].(map[string]any); ok {
			if command := firstPayloadString(nested, "command", "cmd"); command != "" {
				return singleLine(command)
			}
		}
	}
	return ""
}

func filesFromEvent(event trace.Event) []string {
	var files []string
	for _, key := range []string{"changed_files", "files", "paths"} {
		files = append(files, stringSliceFromAny(event.Payload[key])...)
	}
	for _, key := range []string{"path", "file", "filename"} {
		if value := firstPayloadString(event.Payload, key); value != "" {
			files = append(files, value)
		}
	}
	return files
}

func lookupKeys(event trace.Event) []string {
	var keys []string
	for _, key := range []string{"tool_use_id", "call_id", "id"} {
		if value := firstPayloadString(event.Payload, key); value != "" {
			keys = append(keys, key+":"+value)
		}
	}
	if event.ParentID != "" {
		keys = append(keys, "span:"+event.ParentID)
	}
	if event.SpanID != "" {
		keys = append(keys, "span:"+event.SpanID)
	}
	return keys
}

func registerKeys(event trace.Event) []string {
	var keys []string
	for _, key := range []string{"tool_use_id", "call_id", "id"} {
		if value := firstPayloadString(event.Payload, key); value != "" {
			keys = append(keys, key+":"+value)
		}
	}
	if event.SpanID != "" {
		keys = append(keys, "span:"+event.SpanID)
	}
	return keys
}

func findItem(itemsByKey map[string]int, keys []string) (int, bool) {
	for _, key := range keys {
		if idx, ok := itemsByKey[key]; ok {
			return idx, true
		}
	}
	return 0, false
}

func registerItem(itemsByKey map[string]int, keys []string, idx int) {
	for _, key := range keys {
		itemsByKey[key] = idx
	}
}

func addEventType(item *TimelineItem, typ trace.EventType) {
	value := string(typ)
	for _, existing := range item.EventTypes {
		if existing == value {
			return
		}
	}
	item.EventTypes = append(item.EventTypes, value)
}

func exitCodeFromPayload(payload trace.Payload) (int, bool) {
	if code, ok := intFromAny(payload["exit_code"]); ok {
		return code, true
	}
	if nested, ok := payload["result"].(map[string]any); ok {
		return intFromAny(nested["exit_code"])
	}
	return 0, false
}

func firstPayloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromAny(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	}
	return 0, false
}

func stringSliceFromAny(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := stringFromAny(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func artifactDetail(ref trace.ArtifactRef) string {
	detail := ref.Path
	if ref.SizeBytes > 0 {
		detail = fmt.Sprintf("%s (%d bytes)", detail, ref.SizeBytes)
	}
	return detail
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallbackValue
}

func shortHash(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func singleLine(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 160 {
		return value[:157] + "..."
	}
	return value
}
