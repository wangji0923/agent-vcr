package generic

import (
	"context"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/adapters"
	"github.com/agent-vcr/agent-vcr/internal/process"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const AdapterName = "generic-cli"

type Adapter struct{}

func init() {
	adapters.Register(Adapter{})
}

func (Adapter) Name() string {
	return AdapterName
}

func (Adapter) DisplayName() string {
	return "Generic CLI"
}

func (Adapter) Probe(ctx context.Context) (*adapters.ProbeResult, error) {
	return &adapters.ProbeResult{Found: true, Details: map[string]string{"mode": "wrapper"}}, nil
}

func (Adapter) Install(ctx context.Context, opts adapters.InstallOptions) error {
	return nil
}

func (Adapter) Uninstall(ctx context.Context, opts adapters.InstallOptions) error {
	return nil
}

func (Adapter) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{
		PromptCapture:     false,
		ToolCallCapture:   false,
		ToolResultCapture: false,
		ShellCapture:      false,
		FileDiffCapture:   true,
		CanRunAsWrapper:   true,
	}
}

func (Adapter) MatchesCommand(command string, args []string) bool {
	return command != ""
}

func Source() trace.Source {
	return trace.Source{Adapter: AdapterName, Agent: "generic-cli"}
}

func RunStartEvent(command string, args []string, cwd string, name string) trace.Event {
	event := trace.NewEvent("", trace.EventRunStart, Source())
	event.Payload = trace.Payload{
		"command": command,
		"args":    args,
		"cwd":     cwd,
	}
	if name != "" {
		event.Payload["name"] = name
	}
	return event
}

func ProcessStartEvent(command string, args []string, cwd string) trace.Event {
	event := trace.NewEvent("", trace.EventProcessStart, Source())
	event.Payload = trace.Payload{
		"command": command,
		"args":    args,
		"cwd":     cwd,
	}
	return event
}

type ProcessResultInput struct {
	Command       string
	Args          []string
	Cwd           string
	Result        process.RunResult
	ChangedFiles  []string
	StdoutRef     *trace.ArtifactRef
	StderrRef     *trace.ArtifactRef
	FinalDiffRef  *trace.ArtifactRef
	BeforeDiffRef *trace.ArtifactRef
	StartError    string
}

func ProcessResultEvent(input ProcessResultInput) trace.Event {
	event := trace.NewEvent("", trace.EventProcessResult, Source())
	duration := input.Result.EndedAt.Sub(input.Result.StartedAt)
	if input.Result.StartedAt.IsZero() || input.Result.EndedAt.IsZero() {
		duration = 0
	}
	payload := trace.Payload{
		"command":                 input.Command,
		"args":                    input.Args,
		"cwd":                     input.Cwd,
		"exit_code":               input.Result.ExitCode,
		"duration_ms":             duration.Milliseconds(),
		"changed_files":           input.ChangedFiles,
		"stdout_bytes":            input.Result.StdoutBytes,
		"stderr_bytes":            input.Result.StderrBytes,
		"stdout_blob_truncated":   input.Result.StdoutTruncated,
		"stderr_blob_truncated":   input.Result.StderrTruncated,
		"stdout_blob":             artifactPath(input.StdoutRef),
		"stderr_blob":             artifactPath(input.StderrRef),
		"final_diff_blob":         artifactPath(input.FinalDiffRef),
		"before_diff_blob":        artifactPath(input.BeforeDiffRef),
		"process_start_error":     input.StartError,
		"process_started_at_unix": input.Result.StartedAt.UnixMilli(),
		"process_ended_at_unix":   input.Result.EndedAt.UnixMilli(),
	}
	if input.StartError == "" {
		delete(payload, "process_start_error")
	}
	if input.BeforeDiffRef == nil {
		delete(payload, "before_diff_blob")
	}
	if input.FinalDiffRef == nil {
		delete(payload, "final_diff_blob")
	}
	if input.StdoutRef == nil {
		delete(payload, "stdout_blob")
	}
	if input.StderrRef == nil {
		delete(payload, "stderr_blob")
	}
	event.Payload = payload
	return event
}

func RunStopEvent(exitCode int, startedAt time.Time, endedAt time.Time) trace.Event {
	event := trace.NewEvent("", trace.EventRunStop, Source())
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	event.Payload = trace.Payload{
		"status":      status,
		"exit_code":   exitCode,
		"duration_ms": endedAt.Sub(startedAt).Milliseconds(),
	}
	return event
}

func artifactPath(ref *trace.ArtifactRef) any {
	if ref == nil {
		return nil
	}
	return ref.Path
}
