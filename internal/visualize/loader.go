package visualize

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type LoadOptions struct {
	ProjectDir string            `json:"project_dir,omitempty"`
	RunIDs     []string          `json:"run_ids,omitempty"`
	NoCache    bool              `json:"no_cache,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	MaxRuns    int               `json:"max_runs,omitempty"`
}

type LoadedRun struct {
	Ref           string                 `json:"ref,omitempty"`
	RunID         string                 `json:"run_id"`
	RunDir        string                 `json:"run_dir,omitempty"`
	Label         string                 `json:"label,omitempty"`
	Metadata      trace.Metadata         `json:"metadata"`
	Timeline      behavior.Timeline      `json:"timeline"`
	Signature     behavior.Signature     `json:"signature"`
	Metrics       behavior.Metrics       `json:"metrics"`
	MetricsReport behavior.MetricsReport `json:"metrics_report"`
	CacheHit      bool                   `json:"cache_hit"`
	Warnings      []string               `json:"warnings,omitempty"`
}

func LoadRuns(ctx context.Context, opts LoadOptions) ([]LoadedRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runRefs := normalizeRunRefs(opts.RunIDs)
	if len(runRefs) == 0 {
		return nil, fmt.Errorf("at least one run id is required")
	}
	maxRuns := opts.MaxRuns
	if maxRuns <= 0 {
		maxRuns = MaxRecommendedRuns
	}
	if len(runRefs) > maxRuns {
		return nil, fmt.Errorf("too many runs: got %d, max %d", len(runRefs), maxRuns)
	}

	projectDir, err := resolveProjectDir(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	loaded := make([]LoadedRun, 0, len(runRefs))
	for _, ref := range runRefs {
		run, err := loadRun(ctx, projectDir, ref, opts)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, run)
	}
	return loaded, nil
}

func loadRun(ctx context.Context, projectDir, ref string, opts LoadOptions) (LoadedRun, error) {
	runID, err := trace.ResolveRunID(projectDir, ref)
	if err != nil {
		return LoadedRun{}, fmt.Errorf("resolve visual run %q: %w", ref, err)
	}
	store, err := trace.OpenRun(projectDir, runID)
	if err != nil {
		return LoadedRun{}, fmt.Errorf("open visual run %q: %w", runID, err)
	}

	metadata, warnings, err := readRunMetadata(store)
	if err != nil {
		return LoadedRun{}, err
	}
	label := labelForRun(opts.Labels, ref, runID)
	pathClassifier := behavior.NewDefaultPathClassifier()

	if !opts.NoCache {
		signature, err := behavior.ReadSignatureCache(store.RunDir)
		if err == nil {
			stale, staleWarning := signatureCacheStale(store.RunDir, signature)
			if !stale {
				signature = normalizeLoadedSignature(runID, signature)
				timeline := timelineFromSignature(runID, signature)
				metricsReport := behavior.ComputeMetricsWithOptions(timeline, behavior.MetricsOptions{PathClassifier: pathClassifier})
				metrics := preferredMetrics(signature.Metrics, metricsReport.Metrics, len(signature.Steps))
				signature.Metrics = metrics
				return LoadedRun{
					Ref:           ref,
					RunID:         runID,
					RunDir:        store.RunDir,
					Label:         label,
					Metadata:      metadata,
					Timeline:      timeline,
					Signature:     signature,
					Metrics:       metrics,
					MetricsReport: behavior.MetricsReport{Metrics: metrics, Facts: metricsReport.Facts},
					CacheHit:      true,
					Warnings:      uniqueStrings(warnings),
				}, nil
			}
			warnings = append(warnings, staleWarning)
		}
		if err != nil && !errors.Is(err, behavior.ErrSignatureCacheMiss) {
			warnings = append(warnings, fmt.Sprintf("behavior cache unreadable for run %q: %v; rebuilding from trace", runID, err))
		}
	}

	events, err := readTraceFile(store.Path("trace.ndjson"))
	if err != nil {
		return LoadedRun{}, fmt.Errorf("read trace for run %q: %w", runID, err)
	}
	extractor := behavior.NewEventExtractor(behavior.NewDefaultCommandClassifier(), pathClassifier)
	extracted, err := extractor.Extract(ctx, behavior.ExtractInput{
		RunID:    runID,
		Events:   events,
		Metadata: &metadata,
	})
	if err != nil {
		return LoadedRun{}, fmt.Errorf("extract behavior for run %q: %w", runID, err)
	}
	timeline := extracted.Timeline
	if strings.TrimSpace(timeline.RunID) == "" {
		timeline.RunID = runID
	}
	if strings.TrimSpace(timeline.SchemaVersion) == "" {
		timeline.SchemaVersion = behavior.SchemaVersion
	}
	metricsReport := behavior.ComputeMetricsWithOptions(timeline, behavior.MetricsOptions{PathClassifier: pathClassifier})
	signature := behavior.BuildSignatureFromTimeline(timeline, behavior.SignatureOptions{NormalizeUserPaths: true})
	signature.RunID = runID
	signature.Metrics = metricsReport.Metrics
	if traceHash, err := behavior.ComputeRunTraceHash(store.RunDir); err == nil {
		signature.SourceTraceHash = traceHash
	} else {
		warnings = append(warnings, fmt.Sprintf("could not hash trace for run %q: %v", runID, err))
	}
	if !opts.NoCache {
		if err := behavior.WriteSignatureCache(store.RunDir, signature); err != nil {
			warnings = append(warnings, fmt.Sprintf("could not write behavior cache for run %q: %v", runID, err))
		}
	}

	warnings = append(warnings, extracted.Warnings...)
	warnings = append(warnings, timeline.Warnings...)
	return LoadedRun{
		Ref:           ref,
		RunID:         runID,
		RunDir:        store.RunDir,
		Label:         label,
		Metadata:      metadata,
		Timeline:      timeline,
		Signature:     signature,
		Metrics:       metricsReport.Metrics,
		MetricsReport: metricsReport,
		CacheHit:      false,
		Warnings:      uniqueStrings(warnings),
	}, nil
}

func readRunMetadata(store *trace.Store) (trace.Metadata, []string, error) {
	meta, err := store.ReadMetadata()
	if err == nil {
		return fillMetadataDefaults(store, meta), nil, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		meta := fillMetadataDefaults(store, trace.Metadata{
			RunID:  store.RunID,
			Status: trace.RunStatusUnknown,
			Cwd:    store.ProjectDir,
		})
		return meta, []string{fmt.Sprintf("metadata missing for run %q; using unknown status", store.RunID)}, nil
	}
	return trace.Metadata{}, nil, fmt.Errorf("read metadata for run %q: %w", store.RunID, err)
}

func fillMetadataDefaults(store *trace.Store, meta trace.Metadata) trace.Metadata {
	if strings.TrimSpace(meta.RunID) == "" {
		meta.RunID = store.RunID
	}
	if strings.TrimSpace(meta.Source) == "" {
		meta.Source = "unknown"
	}
	if strings.TrimSpace(meta.Status) == "" {
		meta.Status = trace.RunStatusUnknown
	}
	if strings.TrimSpace(meta.Cwd) == "" {
		meta.Cwd = store.ProjectDir
	}
	return meta
}

func readTraceFile(path string) ([]trace.Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("trace not found: %s", path)
		}
		return nil, err
	}
	defer file.Close()
	return readTraceEvents(file)
}

func readTraceEvents(reader io.Reader) ([]trace.Event, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var events []trace.Event
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event trace.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse trace line %d: %w", lineNo, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func normalizeLoadedSignature(runID string, signature behavior.Signature) behavior.Signature {
	if strings.TrimSpace(signature.SchemaVersion) == "" {
		signature.SchemaVersion = behavior.SchemaVersion
	}
	if strings.TrimSpace(signature.RunID) == "" {
		signature.RunID = runID
	}
	for i := range signature.Steps {
		if strings.TrimSpace(signature.Steps[i].RunID) == "" {
			signature.Steps[i].RunID = signature.RunID
		}
		if signature.Steps[i].Index < 0 {
			signature.Steps[i].Index = i
		}
		if strings.TrimSpace(signature.Steps[i].StepID) == "" {
			signature.Steps[i].StepID = fmt.Sprintf("behavior_step_%04d", i+1)
		}
		signature.Steps[i].Files = behavior.SortFiles(signature.Steps[i].Files)
	}
	return signature
}

func timelineFromSignature(runID string, signature behavior.Signature) behavior.Timeline {
	steps := make([]behavior.Step, len(signature.Steps))
	copy(steps, signature.Steps)
	for i := range steps {
		if strings.TrimSpace(steps[i].RunID) == "" {
			steps[i].RunID = firstNonEmpty(signature.RunID, runID)
		}
		if steps[i].Index < 0 {
			steps[i].Index = i
		}
	}
	return behavior.Timeline{
		SchemaVersion: behavior.SchemaVersion,
		RunID:         firstNonEmpty(signature.RunID, runID),
		Steps:         steps,
	}
}

func preferredMetrics(cached, computed behavior.Metrics, stepCount int) behavior.Metrics {
	if stepCount > 0 && cached.ToolEfficiency.TotalSteps == 0 {
		return computed
	}
	return cached
}

func signatureCacheStale(runDir string, signature behavior.Signature) (bool, string) {
	traceHash, err := behavior.ComputeRunTraceHash(runDir)
	if err != nil {
		return false, fmt.Sprintf("could not verify behavior cache for run %q: %v", filepath.Base(runDir), err)
	}
	if traceHash == "" || signature.SourceTraceHash == "" || signature.SourceTraceHash == traceHash {
		return false, ""
	}
	return true, fmt.Sprintf("behavior cache stale for run %q; rebuilding from trace", filepath.Base(runDir))
}

func resolveProjectDir(projectDir string) (string, error) {
	if strings.TrimSpace(projectDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		projectDir = cwd
	}
	return filepath.Abs(projectDir)
}

func normalizeRunRefs(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		out = append(out, ref)
	}
	return out
}

func labelForRun(labels map[string]string, ref, runID string) string {
	for _, key := range []string{runID, ref} {
		if labels == nil {
			continue
		}
		if label := strings.TrimSpace(labels[key]); label != "" {
			return label
		}
	}
	return runID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
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
	return out
}
