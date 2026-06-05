package analysis

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type Replay struct {
	RunID           string         `json:"run_id"`
	RunDir          string         `json:"run_dir"`
	Metadata        trace.Metadata `json:"metadata"`
	MetadataMissing bool           `json:"metadata_missing,omitempty"`
	MetadataError   string         `json:"metadata_error,omitempty"`
	Timeline        []TimelineItem `json:"timeline"`
}

func LoadReplay(projectDir string, runID string) (Replay, error) {
	store, err := trace.OpenRun(projectDir, runID)
	if err != nil {
		return Replay{}, err
	}

	events, err := ReadEvents(store.Path("trace.ndjson"))
	if err != nil {
		return Replay{}, err
	}

	replay := Replay{
		RunID:    runID,
		RunDir:   store.RunDir,
		Timeline: BuildTimeline(events),
	}

	meta, metaErr := store.ReadMetadata()
	if metaErr != nil {
		replay.MetadataMissing = errors.Is(metaErr, os.ErrNotExist)
		replay.MetadataError = metaErr.Error()
		meta = metadataFromEvents(runID, events)
	} else {
		fillMetadataFromEvents(&meta, events)
	}
	if meta.RunID == "" {
		meta.RunID = runID
	}
	replay.Metadata = meta
	return replay, nil
}

func ReadEvents(path string) ([]trace.Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []trace.Event{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var events []trace.Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := int64(0)
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event trace.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			events = append(events, invalidTraceLineEvent(lineNo, err))
			continue
		}
		if event.EventIndex == 0 {
			event.EventIndex = lineNo
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func FilterTimeline(items []TimelineItem, filter string) []TimelineItem {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" || filter == "all" {
		return append([]TimelineItem(nil), items...)
	}
	var filtered []TimelineItem
	for _, item := range items {
		if timelineItemMatches(item, filter) {
			filtered = append(filtered, item)
		}
	}
	for i := range filtered {
		filtered[i].Index = int64(i + 1)
	}
	return filtered
}

func timelineItemMatches(item TimelineItem, filter string) bool {
	switch filter {
	case "tool":
		return item.Type == "tool" || item.Type == "tool_result" || item.Type == "tool_error" ||
			hasEventType(item, trace.EventToolCall, trace.EventToolResult, trace.EventToolError)
	case "shell":
		return item.Type == "shell" || item.Type == "shell_result" ||
			hasEventType(item, trace.EventShellCommand, trace.EventShellResult)
	case "file":
		return item.Type == "file_patch" || item.Type == "file_read" || item.Type == "file_write" ||
			hasEventType(item, trace.EventFilePatch, trace.EventFileRead, trace.EventFileWrite) ||
			len(item.ChangedFiles) > 0
	case "raw":
		return item.Type == "raw" || hasEventType(item, trace.EventRaw)
	default:
		return item.Type == filter || hasEventTypeString(item, filter)
	}
}

func hasEventType(item TimelineItem, types ...trace.EventType) bool {
	for _, typ := range types {
		if hasEventTypeString(item, string(typ)) {
			return true
		}
	}
	return false
}

func hasEventTypeString(item TimelineItem, typ string) bool {
	for _, existing := range item.EventTypes {
		if existing == typ {
			return true
		}
	}
	return false
}

func metadataFromEvents(runID string, events []trace.Event) trace.Metadata {
	meta := trace.Metadata{
		SchemaVersion: trace.SchemaVersion,
		RunID:         runID,
		Status:        trace.RunStatusUnknown,
	}
	fillMetadataFromEvents(&meta, events)
	return meta
}

func fillMetadataFromEvents(meta *trace.Metadata, events []trace.Event) {
	if meta.Source == "" {
		for _, event := range events {
			if event.Source.Adapter != "" {
				meta.Source = event.Source.Adapter
				break
			}
		}
	}
	if meta.Status == "" || meta.Status == trace.RunStatusUnknown {
		if status := lastRunStatus(events); status != "" {
			meta.Status = status
		} else if hasRunStart(events) {
			meta.Status = trace.RunStatusRunning
		} else if meta.Status == "" {
			meta.Status = trace.RunStatusUnknown
		}
	}
	if meta.StartedAt.IsZero() {
		for _, event := range events {
			if !event.Timestamp.IsZero() {
				meta.StartedAt = event.Timestamp
				break
			}
		}
	}
	if meta.EndedAt == nil {
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].Type == trace.EventRunStop && !events[i].Timestamp.IsZero() {
				ended := events[i].Timestamp
				meta.EndedAt = &ended
				break
			}
		}
	}
}

func lastRunStatus(events []trace.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Type != trace.EventRunStop {
			continue
		}
		if status := firstPayloadString(event.Payload, "status"); status != "" {
			return status
		}
		if code, ok := exitCodeFromPayload(event.Payload); ok {
			if code == 0 {
				return trace.RunStatusCompleted
			}
			return trace.RunStatusFailed
		}
		return trace.RunStatusCompleted
	}
	return ""
}

func hasRunStart(events []trace.Event) bool {
	for _, event := range events {
		if event.Type == trace.EventRunStart {
			return true
		}
	}
	return false
}

func invalidTraceLineEvent(lineNo int64, err error) trace.Event {
	return trace.Event{
		SchemaVersion: trace.SchemaVersion,
		EventID:       fmt.Sprintf("invalid-line-%d", lineNo),
		EventIndex:    lineNo,
		Type:          trace.EventRaw,
		Source: trace.Source{
			Adapter:      "unknown",
			RawEventType: "invalid_trace_json",
		},
		Payload: trace.Payload{
			"reason": "invalid_trace_json",
			"line":   lineNo,
			"error":  err.Error(),
		},
	}
}

func ChangedFilesFromPatch(projectRunDir string, relPath string) []string {
	if relPath == "" {
		return nil
	}
	path := filepath.Join(projectRunDir, filepath.FromSlash(relPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parsePatchChangedFiles(string(data))
}

func parsePatchChangedFiles(data string) []string {
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				files = append(files, cleanPatchPath(parts[3]))
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			files = append(files, cleanPatchPath(path))
		}
	}
	files = uniqueSorted(files)
	sort.Strings(files)
	return files
}

func cleanPatchPath(path string) string {
	path = strings.Trim(path, `"`)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}
