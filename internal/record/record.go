package record

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/adapters"
	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/gitutil"
	"github.com/agent-vcr/agent-vcr/internal/process"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const (
	AdapterAuto = "auto"
)

type Options struct {
	ProjectDir    string
	ConfigPath    string
	Name          string
	Adapter       string
	Cwd           string
	CaptureStdout bool
	CaptureStderr bool
	Command       []string
}

type Result struct {
	RunID    string
	RunDir   string
	Adapter  string
	ExitCode int
	Warnings []string
}

type commandMatcher interface {
	MatchesCommand(command string, args []string) bool
}

type commandHinter interface {
	CommandHint(command string, args []string) string
}

type lineNormalizer interface {
	NormalizeLine(context.Context, []byte) ([]trace.Event, trace.RawEvent, bool, error)
}

type gitCapture struct {
	IsRepo   bool
	RepoRoot string
	Snapshot gitutil.Snapshot
	Diff     []byte
	Err      error
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if len(opts.Command) == 0 || opts.Command[0] == "" {
		return Result{}, errors.New("record requires a command after --")
	}
	if opts.Adapter == "" {
		opts.Adapter = AdapterAuto
	}
	if opts.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Result{}, err
		}
		opts.Cwd = cwd
	}
	cwd, err := filepath.Abs(opts.Cwd)
	if err != nil {
		return Result{}, err
	}
	opts.Cwd = cwd

	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = cwd
	}
	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return Result{}, err
	}

	cfg, _, err := config.Load(projectDir, opts.ConfigPath)
	if err != nil {
		return Result{}, err
	}

	command := opts.Command[0]
	args := append([]string(nil), opts.Command[1:]...)
	adapter, warnings, err := selectAdapter(opts.Adapter, command, args)
	if err != nil {
		return Result{}, err
	}

	store, err := trace.CreateRun(projectDir, adapter.Name())
	if err != nil {
		return Result{}, err
	}
	if opts.Name != "" {
		if err := applyRunNameSuffix(store, opts.Name); err != nil {
			return Result{}, err
		}
	}

	before := captureGit(cwd)
	beforeDiffRef, err := writePatchIfAvailable(store, "before.diff", before.Diff)
	if err != nil {
		return Result{}, err
	}

	if err := initializeMetadata(store, adapter, opts, before, beforeDiffRef); err != nil {
		return Result{}, err
	}

	if adapter.Name() == "generic-cli" {
		if err := store.Append(newRunStartEvent(adapter, command, args, cwd, opts.Name)); err != nil {
			return Result{}, err
		}
	}
	if err := store.Append(newProcessStartEvent(adapter, command, args, cwd)); err != nil {
		return Result{}, err
	}

	mode := captureModeForAdapter(adapter)
	runOpts := process.RunOptions{
		Command:      command,
		Args:         args,
		Cwd:          cwd,
		Env:          []string{"AGENT_VCR_CAPTURE_MODE=" + mode, "AGENT_VCR_RUN_ID=" + store.RunID},
		StdoutMode:   outputMode(opts.CaptureStdout),
		StderrMode:   outputMode(opts.CaptureStderr),
		MaxBlobBytes: cfg.Storage.MaxBlobBytes,
		StdoutName:   stdoutName(mode),
		StderrName:   "process_stderr.txt",
		StdoutMIME:   stdoutMIME(mode),
		StderrMIME:   "text/plain",
	}
	if normalizer, ok := adapter.(lineNormalizer); ok {
		runOpts.OnStdoutLine = func(line []byte) error {
			events, raw, saveRaw, err := normalizer.NormalizeLine(ctx, line)
			if err != nil {
				return err
			}
			if saveRaw {
				_, err := store.SaveRawEvent(raw)
				return err
			}
			for _, event := range events {
				if event.Type == trace.EventRaw {
					if event.RawRef != nil {
						if err := store.Append(event); err != nil {
							return err
						}
						continue
					}
					_, err := store.SaveRawEvent(raw)
					if err != nil {
						return err
					}
					continue
				}
				if err := store.Append(event); err != nil {
					return err
				}
			}
			return nil
		}
	}

	processResult, runErr := process.Run(ctx, store, runOpts)
	after := captureGit(cwd)
	finalDiffRef, err := writePatchIfAvailable(store, "final.diff", after.Diff)
	if err != nil {
		return Result{}, err
	}

	changedFiles := gitutil.ChangedFilesDelta(before.Snapshot, after.Snapshot)
	if !after.IsRepo {
		changedFiles = nil
	}
	processResultEvent := newProcessResultEvent(adapter, command, args, cwd, processResult, changedFiles, beforeDiffRef, finalDiffRef)
	if err := store.Append(processResultEvent); err != nil {
		return Result{}, err
	}
	if err := store.Append(newRunStopEvent(adapter, processResult.ExitCode, processResult.StartedAt, processResult.EndedAt)); err != nil {
		return Result{}, err
	}
	if err := finalizeMetadata(store, processResult, before, after, changedFiles, beforeDiffRef, finalDiffRef); err != nil {
		return Result{}, err
	}
	if runErr != nil {
		return Result{}, runErr
	}

	return Result{
		RunID:    store.RunID,
		RunDir:   store.RunDir,
		Adapter:  adapter.Name(),
		ExitCode: processResult.ExitCode,
		Warnings: warnings,
	}, nil
}

func selectAdapter(requested string, command string, args []string) (adapters.Adapter, []string, error) {
	var warnings []string
	if requested != AdapterAuto {
		adapter, ok := adapters.Get(requested)
		if !ok {
			return nil, nil, fmt.Errorf("unknown adapter: %s", requested)
		}
		return adapter, warnings, nil
	}

	for _, adapter := range adapters.List() {
		if hinter, ok := adapter.(commandHinter); ok {
			if hint := hinter.CommandHint(command, args); hint != "" {
				warnings = append(warnings, hint)
			}
		}
	}
	for _, adapter := range adapters.List() {
		matcher, ok := adapter.(commandMatcher)
		if ok && matcher.MatchesCommand(command, args) {
			return adapter, warnings, nil
		}
	}
	adapter, ok := adapters.Get("generic-cli")
	if !ok {
		return nil, nil, errors.New("generic-cli adapter is not registered")
	}
	return adapter, warnings, nil
}

func captureGit(cwd string) gitCapture {
	repoRoot, err := gitutil.FindRepoRoot(cwd)
	if err != nil {
		return gitCapture{}
	}
	snapshot, diff, err := gitutil.CaptureSnapshot(cwd)
	if err != nil {
		return gitCapture{IsRepo: true, RepoRoot: repoRoot, Err: err}
	}
	return gitCapture{IsRepo: true, RepoRoot: repoRoot, Snapshot: snapshot, Diff: diff}
}

func writePatchIfAvailable(store *trace.Store, name string, diff []byte) (*trace.ArtifactRef, error) {
	if diff == nil {
		return nil, nil
	}
	ref, err := store.WritePatch(name, diff)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

func initializeMetadata(store *trace.Store, adapter adapters.Adapter, opts Options, before gitCapture, beforeDiffRef *trace.ArtifactRef) error {
	meta, err := store.ReadMetadata()
	if err != nil {
		return err
	}
	meta.Source = adapter.Name()
	meta.Cwd = opts.Cwd
	meta.Capabilities = capabilitiesMap(adapter.Capabilities())
	meta.Summary = trace.Payload{
		"name":         opts.Name,
		"command":      opts.Command,
		"capture_mode": captureModeForAdapter(adapter),
	}
	if before.IsRepo {
		meta.RepoRoot = before.RepoRoot
		meta.GitSHA = before.Snapshot.Head
		meta.Branch = before.Snapshot.Branch
		meta.Summary["before_snapshot"] = before.Snapshot
		if beforeDiffRef != nil {
			meta.Summary["before_diff_blob"] = beforeDiffRef.Path
		}
		if before.Err != nil {
			meta.Summary["git_before_error"] = before.Err.Error()
		}
	} else {
		meta.Summary["no_git_repo"] = true
	}
	if opts.Name == "" {
		delete(meta.Summary, "name")
	}
	return store.WriteMetadata(meta)
}

func finalizeMetadata(store *trace.Store, result process.RunResult, before, after gitCapture, changedFiles []string, beforeDiffRef, finalDiffRef *trace.ArtifactRef) error {
	meta, err := store.ReadMetadata()
	if err != nil {
		return err
	}
	ended := time.Now().UTC()
	if !result.EndedAt.IsZero() {
		ended = result.EndedAt
	}
	meta.EndedAt = &ended
	meta.Status = trace.RunStatusCompleted
	if result.ExitCode != 0 {
		meta.Status = trace.RunStatusFailed
	}
	if meta.Summary == nil {
		meta.Summary = trace.Payload{}
	}
	meta.Summary["exit_code"] = result.ExitCode
	meta.Summary["stdout_bytes"] = result.StdoutBytes
	meta.Summary["stderr_bytes"] = result.StderrBytes
	if result.StdoutRef != nil {
		meta.Summary["stdout_blob"] = result.StdoutRef.Path
	}
	if result.StderrRef != nil {
		meta.Summary["stderr_blob"] = result.StderrRef.Path
	}
	if beforeDiffRef != nil {
		meta.Summary["before_diff_blob"] = beforeDiffRef.Path
	}
	if finalDiffRef != nil {
		meta.Summary["final_diff_blob"] = finalDiffRef.Path
	}
	if after.IsRepo {
		meta.Summary["after_snapshot"] = after.Snapshot
		meta.Summary["changed_files"] = changedFiles
		if after.Err != nil {
			meta.Summary["git_after_error"] = after.Err.Error()
		}
	} else if !before.IsRepo {
		meta.Summary["no_git_repo"] = true
	}
	if result.StartError != "" {
		meta.Summary["process_start_error"] = result.StartError
	}
	return store.WriteMetadata(meta)
}

func newRunStartEvent(adapter adapters.Adapter, command string, args []string, cwd string, name string) trace.Event {
	event := trace.NewEvent("", trace.EventRunStart, sourceFor(adapter))
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

func newProcessStartEvent(adapter adapters.Adapter, command string, args []string, cwd string) trace.Event {
	event := trace.NewEvent("", trace.EventProcessStart, sourceFor(adapter))
	event.Payload = trace.Payload{
		"command": command,
		"args":    args,
		"cwd":     cwd,
	}
	return event
}

func newProcessResultEvent(adapter adapters.Adapter, command string, args []string, cwd string, result process.RunResult, changedFiles []string, beforeDiffRef, finalDiffRef *trace.ArtifactRef) trace.Event {
	event := trace.NewEvent("", trace.EventProcessResult, sourceFor(adapter))
	duration := result.EndedAt.Sub(result.StartedAt)
	if result.StartedAt.IsZero() || result.EndedAt.IsZero() {
		duration = 0
	}
	payload := trace.Payload{
		"command":               command,
		"args":                  args,
		"cwd":                   cwd,
		"exit_code":             result.ExitCode,
		"duration_ms":           duration.Milliseconds(),
		"changed_files":         changedFiles,
		"stdout_bytes":          result.StdoutBytes,
		"stderr_bytes":          result.StderrBytes,
		"stdout_blob_truncated": result.StdoutTruncated,
		"stderr_blob_truncated": result.StderrTruncated,
	}
	if result.StdoutRef != nil {
		payload["stdout_blob"] = result.StdoutRef.Path
	}
	if result.StderrRef != nil {
		payload["stderr_blob"] = result.StderrRef.Path
	}
	if beforeDiffRef != nil {
		payload["before_diff_blob"] = beforeDiffRef.Path
	}
	if finalDiffRef != nil {
		payload["final_diff_blob"] = finalDiffRef.Path
	}
	if result.StartError != "" {
		payload["process_start_error"] = result.StartError
	}
	event.Payload = payload
	return event
}

func newRunStopEvent(adapter adapters.Adapter, exitCode int, startedAt time.Time, endedAt time.Time) trace.Event {
	event := trace.NewEvent("", trace.EventRunStop, sourceFor(adapter))
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	duration := endedAt.Sub(startedAt)
	if startedAt.IsZero() || endedAt.IsZero() {
		duration = 0
	}
	event.Payload = trace.Payload{
		"status":      status,
		"exit_code":   exitCode,
		"duration_ms": duration.Milliseconds(),
	}
	return event
}

func sourceFor(adapter adapters.Adapter) trace.Source {
	return trace.Source{
		Adapter: adapter.Name(),
		Agent:   adapter.Name(),
	}
}

func captureModeForAdapter(adapter adapters.Adapter) string {
	if _, ok := adapter.(lineNormalizer); ok {
		return "jsonl"
	}
	return "generic"
}

func outputMode(capture bool) string {
	if capture {
		return process.OutputModeBlob
	}
	return process.OutputModeDiscard
}

func stdoutName(mode string) string {
	if mode == "jsonl" {
		return "codex_stdout.jsonl"
	}
	return "process_stdout.txt"
}

func stdoutMIME(mode string) string {
	if mode == "jsonl" {
		return "application/x-ndjson"
	}
	return "text/plain"
}

func capabilitiesMap(caps adapters.Capabilities) map[string]bool {
	return map[string]bool{
		"prompt_capture":       caps.PromptCapture,
		"model_call_capture":   caps.ModelCallCapture,
		"model_result_capture": caps.ModelResultCapture,
		"tool_call_capture":    caps.ToolCallCapture,
		"tool_result_capture":  caps.ToolResultCapture,
		"shell_capture":        caps.ShellCapture,
		"file_diff_capture":    caps.FileDiffCapture,
		"permission_capture":   caps.PermissionCapture,
		"subagent_capture":     caps.SubagentCapture,
		"mcp_tool_capture":     caps.MCPToolCapture,
		"can_install_hooks":    caps.CanInstallHooks,
		"can_run_as_wrapper":   caps.CanRunAsWrapper,
		"can_import_trace":     caps.CanImportTrace,
		"can_ingest_http":      caps.CanIngestHTTP,
	}
}

func applyRunNameSuffix(store *trace.Store, name string) error {
	suffix := slugify(name)
	if suffix == "" {
		return nil
	}
	oldDir := store.RunDir
	newRunID := store.RunID + "-" + suffix
	newDir := filepath.Join(store.RunsDir, newRunID)
	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}
	store.RunID = newRunID
	store.RunDir = newDir
	meta, err := store.ReadMetadata()
	if err != nil {
		return err
	}
	meta.RunID = newRunID
	return store.WriteMetadata(meta)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9._-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._")
	if len(value) > 48 {
		value = value[:48]
		value = strings.Trim(value, "-._")
	}
	return value
}
