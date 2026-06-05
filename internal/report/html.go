package report

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/redact"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const maxInlinePatchBytes = 256 * 1024

//go:embed templates/report.html.tmpl
var templateFS embed.FS

type ReportData struct {
	Metadata      trace.Metadata
	Summary       ReportSummary
	InputOutput   ReportInputOutput
	Timeline      []analysis.TimelineItem
	CheckResult   *analysis.CheckResult
	FinalDiff     string
	Events        []EventDetail
	Artifacts     []ArtifactSummary
	Capabilities  []Capability
	RawEventCount int
	Duration      string
	GeneratedAt   time.Time
}

type ReportSummary struct {
	RunID         string
	Source        string
	SourceAdapter string
	ToolCalls     int
	ChangedFiles  []string
	Commands      []CommandSummary
	RiskScore     int
	Status        string
	StartedAt     string
	EndedAt       string
}

type CommandSummary struct {
	EventID  string
	Command  string
	ToolName string
	ExitCode string
}

type ReportInputOutput struct {
	TurnID        string
	Input         string
	InputSHA256   string
	InputEventID  string
	Output        string
	OutputSHA256  string
	OutputEventID string
}

type EventDetail struct {
	Index       int64
	EventID     string
	Type        string
	Source      string
	Timestamp   string
	PayloadJSON string
	RawRef      string
	Artifacts   []ArtifactSummary
}

type ArtifactSummary struct {
	Kind      string
	Path      string
	SizeBytes int64
	MimeType  string
	Redacted  bool
}

type Capability struct {
	Name    string
	Enabled bool
}

func Load(projectDir, ref string, cfg config.Config) (ReportData, error) {
	runID, err := trace.ResolveRunID(projectDir, ref)
	if err != nil {
		return ReportData{}, err
	}
	store, err := trace.OpenRun(projectDir, runID)
	if err != nil {
		return ReportData{}, err
	}
	meta, err := store.ReadMetadata()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return ReportData{}, err
		}
		meta = trace.Metadata{
			SchemaVersion: trace.SchemaVersion,
			RunID:         runID,
			Source:        "unknown",
			Status:        trace.RunStatusUnknown,
			Cwd:           store.ProjectDir,
			StartedAt:     time.Time{},
		}
	}
	events, err := analysis.ReadTraceFile(store.Path("trace.ndjson"))
	if err != nil {
		return ReportData{}, err
	}
	return Build(store.RunDir, meta, events, cfg)
}

func Build(runDir string, meta trace.Metadata, events []trace.Event, cfg config.Config) (ReportData, error) {
	redactedMeta := redactMetadata(meta, cfg.Redaction)
	redactedEvents := make([]trace.Event, 0, len(events))
	for _, event := range events {
		redactedEvents = append(redactedEvents, redact.ApplyToEvent(event, cfg.Redaction))
	}
	run := analysis.RunData{
		RunID:    fallback(redactedMeta.RunID, filepath.Base(runDir)),
		RunDir:   runDir,
		Metadata: redactedMeta,
		Events:   redactedEvents,
	}
	check := analysis.CheckRun(run, cfg)
	timeline := analysis.BuildTimeline(redactedEvents)
	finalDiff, err := readFinalDiff(runDir, cfg.Redaction)
	if err != nil {
		return ReportData{}, err
	}
	summary := buildSummary(run, timeline, check)
	artifacts := collectArtifacts(redactedEvents)
	return ReportData{
		Metadata:      redactedMeta,
		Summary:       summary,
		InputOutput:   buildInputOutput(redactedEvents),
		Timeline:      timeline,
		CheckResult:   &check,
		FinalDiff:     finalDiff,
		Events:        eventDetails(redactedEvents),
		Artifacts:     artifacts,
		Capabilities:  capabilities(redactedMeta.Capabilities),
		RawEventCount: countRawEvents(redactedEvents),
		Duration:      durationString(redactedMeta),
		GeneratedAt:   time.Now().UTC(),
	}, nil
}

func WriteHTML(data ReportData, writer io.Writer) error {
	tmpl, err := template.ParseFS(templateFS, "templates/report.html.tmpl")
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(writer, "report.html.tmpl", data)
}

func WriteHTMLFile(data ReportData, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return WriteHTML(data, file)
}

func redactMetadata(meta trace.Metadata, cfg config.RedactConfig) trace.Metadata {
	data, err := json.Marshal(meta)
	if err != nil {
		return meta
	}
	data = redact.ApplyToBytes(data, cfg)
	var out trace.Metadata
	if err := json.Unmarshal(data, &out); err != nil {
		return meta
	}
	return out
}

func buildInputOutput(events []trace.Event) ReportInputOutput {
	var out ReportInputOutput
	for _, event := range events {
		switch event.Type {
		case trace.EventUserPrompt:
			if value := payloadString(event.Payload, "turn_id"); value != "" {
				out.TurnID = value
			}
			out.Input = payloadString(event.Payload, "prompt", "content", "message", "text", "input")
			out.InputSHA256 = payloadString(event.Payload, "prompt_sha256", "content_sha256", "message_sha256", "input_sha256")
			out.InputEventID = event.EventID
		case trace.EventRunStop, trace.EventModelResult:
			if value := payloadString(event.Payload, "turn_id"); value != "" {
				out.TurnID = value
			}
			out.Output = payloadString(event.Payload, "last_assistant_message", "assistant_message", "output", "message", "text", "content")
			out.OutputSHA256 = payloadString(event.Payload, "last_assistant_message_sha256", "assistant_message_sha256", "output_sha256", "message_sha256", "content_sha256")
			out.OutputEventID = event.EventID
		}
	}
	return out
}

func buildSummary(run analysis.RunData, timeline []analysis.TimelineItem, check analysis.CheckResult) ReportSummary {
	meta := run.Metadata
	sourceAdapter := firstSourceAdapter(run.Events)
	if sourceAdapter == "" {
		sourceAdapter = meta.Source
	}
	return ReportSummary{
		RunID:         fallback(meta.RunID, run.RunID),
		Source:        fallback(meta.Source, "unknown"),
		SourceAdapter: fallback(sourceAdapter, "unknown"),
		ToolCalls:     countType(run.Events, trace.EventToolCall),
		ChangedFiles:  analysis.ChangedFiles(run),
		Commands:      commandSummaries(run.Events, timeline),
		RiskScore:     check.RiskScore,
		Status:        fallback(meta.Status, trace.RunStatusUnknown),
		StartedAt:     formatTime(meta.StartedAt),
		EndedAt:       formatTimePtr(meta.EndedAt),
	}
}

func commandSummaries(events []trace.Event, timeline []analysis.TimelineItem) []CommandSummary {
	seen := map[string]bool{}
	var out []CommandSummary
	for _, event := range events {
		command := analysis.CommandFromEvent(event)
		if command == "" {
			continue
		}
		key := event.EventID + "\x00" + command
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, CommandSummary{
			EventID:  event.EventID,
			Command:  command,
			ToolName: payloadString(event.Payload, "tool_name", "name"),
			ExitCode: payloadString(event.Payload, "exit_code", "code"),
		})
	}
	if len(out) > 0 {
		return out
	}
	for _, item := range timeline {
		if item.Type != "shell" && item.Type != "process_start" && item.Type != "process_result" {
			continue
		}
		exitCode := ""
		if item.ExitCode != nil {
			exitCode = fmt.Sprintf("%d", *item.ExitCode)
		}
		out = append(out, CommandSummary{
			Command:  item.Title,
			ToolName: item.ToolName,
			ExitCode: exitCode,
		})
	}
	return out
}

func readFinalDiff(runDir string, cfg config.RedactConfig) (string, error) {
	root := filepath.Join(runDir, "patches")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	var out strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(runDir, path)
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		fmt.Fprintf(&out, "--- %s ---\n", filepath.ToSlash(rel))
		if len(data) > maxInlinePatchBytes {
			data = data[:maxInlinePatchBytes]
			data = append(data, []byte("\n[truncated]\n")...)
		}
		out.Write(redact.ApplyToBytes(data, cfg))
	}
	return out.String(), nil
}

func eventDetails(events []trace.Event) []EventDetail {
	out := make([]EventDetail, 0, len(events))
	for _, event := range events {
		payload := "{}"
		if event.Payload != nil {
			if data, err := json.MarshalIndent(event.Payload, "", "  "); err == nil {
				payload = string(data)
			}
		}
		rawRef := ""
		artifacts := artifactSummaries(event.Artifacts)
		if event.RawRef != nil {
			rawRef = event.RawRef.Path
			artifacts = append(artifacts, artifactSummary(*event.RawRef))
		}
		out = append(out, EventDetail{
			Index:       event.EventIndex,
			EventID:     event.EventID,
			Type:        string(event.Type),
			Source:      event.Source.Adapter,
			Timestamp:   formatTime(event.Timestamp),
			PayloadJSON: payload,
			RawRef:      rawRef,
			Artifacts:   artifacts,
		})
	}
	return out
}

func collectArtifacts(events []trace.Event) []ArtifactSummary {
	seen := map[string]bool{}
	var out []ArtifactSummary
	for _, event := range events {
		for _, ref := range event.Artifacts {
			if ref.Path == "" || seen[ref.Path] {
				continue
			}
			seen[ref.Path] = true
			out = append(out, artifactSummary(ref))
		}
		if event.RawRef != nil && event.RawRef.Path != "" && !seen[event.RawRef.Path] {
			seen[event.RawRef.Path] = true
			out = append(out, artifactSummary(*event.RawRef))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func artifactSummaries(refs []trace.ArtifactRef) []ArtifactSummary {
	out := make([]ArtifactSummary, 0, len(refs))
	for _, ref := range refs {
		out = append(out, artifactSummary(ref))
	}
	return out
}

func artifactSummary(ref trace.ArtifactRef) ArtifactSummary {
	return ArtifactSummary{
		Kind:      string(ref.Kind),
		Path:      ref.Path,
		SizeBytes: ref.SizeBytes,
		MimeType:  ref.MimeType,
		Redacted:  ref.Redacted,
	}
}

func capabilities(values map[string]bool) []Capability {
	out := make([]Capability, 0, len(values))
	for key, enabled := range values {
		out = append(out, Capability{Name: key, Enabled: enabled})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func firstSourceAdapter(events []trace.Event) string {
	for _, event := range events {
		if event.Source.Adapter != "" {
			return event.Source.Adapter
		}
	}
	return ""
}

func countType(events []trace.Event, typ trace.EventType) int {
	count := 0
	for _, event := range events {
		if event.Type == typ {
			count++
		}
	}
	return count
}

func countRawEvents(events []trace.Event) int {
	return countType(events, trace.EventRaw)
}

func payloadString(payload trace.Payload, keys ...string) string {
	for _, key := range keys {
		switch value := payload[key].(type) {
		case string:
			if value != "" {
				return value
			}
		case int:
			return fmt.Sprintf("%d", value)
		case int64:
			return fmt.Sprintf("%d", value)
		case float64:
			return fmt.Sprintf("%g", value)
		}
	}
	return ""
}

func durationString(meta trace.Metadata) string {
	if meta.StartedAt.IsZero() || meta.EndedAt == nil || meta.EndedAt.IsZero() {
		return "-"
	}
	return meta.EndedAt.Sub(meta.StartedAt).Round(time.Second).String()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return formatTime(*value)
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallbackValue
}
