package visualize

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func LoadVisualReport(ctx context.Context, opts LoadOptions) (VisualReport, error) {
	runs, err := LoadRuns(ctx, opts)
	if err != nil {
		return VisualReport{}, err
	}
	return BuildReportSkeleton(runs, opts)
}

func BuildReportSkeleton(runs []LoadedRun, opts LoadOptions) (VisualReport, error) {
	if len(runs) == 0 {
		return VisualReport{}, fmt.Errorf("at least one loaded run is required")
	}
	mode := RenderModeSingle
	if len(runs) > 1 {
		mode = RenderModeCompare
	}
	maxRuns := opts.MaxRuns
	if maxRuns <= 0 {
		maxRuns = MaxRecommendedRuns
	}

	visualRuns := make([]VisualRun, 0, len(runs))
	lanes := make([]BehaviorLane, 0, len(runs))
	var warnings []string
	for _, run := range runs {
		label := firstNonEmpty(run.Label, labelForRun(opts.Labels, run.Ref, run.RunID))
		visualRuns = append(visualRuns, BuildVisualRun(run, label))
		lanes = append(lanes, BuildLane(run, label))
		warnings = append(warnings, run.Warnings...)
	}

	divergences := buildDivergenceMarkers(runs)
	markDivergentLaneSteps(lanes, divergences)
	firstDivergence := firstDivergenceMarker(divergences)
	report := VisualReport{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Mode:          mode,
		Options: VisualOptions{
			Mode:          mode,
			BaselineRunID: runs[0].RunID,
			Labels:        copyStringMap(opts.Labels),
			MaxRuns:       maxRuns,
			NoCache:       opts.NoCache,
		},
		Runs:        visualRuns,
		Lanes:       lanes,
		Divergences: divergences,
		Warnings:    uniqueStrings(warnings),
	}
	report.Summary = summarizeVisualReport(report, firstDivergence)
	return report, nil
}

func BuildVisualRun(run LoadedRun, label string) VisualRun {
	source := firstNonEmpty(run.Metadata.Source, "unknown")
	status := firstNonEmpty(run.Metadata.Status, trace.RunStatusUnknown)
	summary := map[string]any{
		"cache_hit": run.CacheHit,
	}
	if run.Signature.SourceTraceHash != "" {
		summary["source_trace_hash"] = run.Signature.SourceTraceHash
	}
	if len(run.Warnings) > 0 {
		summary["warnings"] = append([]string{}, run.Warnings...)
	}
	if run.Metrics.ToolEfficiency.TotalSteps > 0 {
		summary["total_steps"] = run.Metrics.ToolEfficiency.TotalSteps
	}

	var startedAt *time.Time
	if !run.Metadata.StartedAt.IsZero() {
		value := run.Metadata.StartedAt.UTC()
		startedAt = &value
	}
	var endedAt *time.Time
	if run.Metadata.EndedAt != nil && !run.Metadata.EndedAt.IsZero() {
		value := run.Metadata.EndedAt.UTC()
		endedAt = &value
	}
	return VisualRun{
		RunID:     run.RunID,
		Label:     firstNonEmpty(label, run.Label, run.RunID),
		Source:    source,
		Status:    status,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		StepCount: len(stepsForLoadedRun(run)),
		Summary:   summary,
	}
}

func BuildLane(run LoadedRun, label string) BehaviorLane {
	steps := stepsForLoadedRun(run)
	visualSteps := make([]VisualStep, 0, len(steps))
	for _, step := range steps {
		visualSteps = append(visualSteps, VisualStepFromBehaviorStep(step))
	}
	return BehaviorLane{
		RunID: run.RunID,
		Label: firstNonEmpty(label, run.Label, run.RunID),
		Steps: visualSteps,
	}
}

func VisualStepFromBehaviorStep(step behavior.Step) VisualStep {
	index := step.Index + 1
	if index < 1 {
		index = 1
	}
	attributes := copyStringMap(step.Attributes)
	if step.Result != "" && step.Result != behavior.ResultUnknown {
		if attributes == nil {
			attributes = map[string]string{}
		}
		attributes["result"] = string(step.Result)
	}
	if step.ToolName != "" {
		if attributes == nil {
			attributes = map[string]string{}
		}
		attributes["tool_name"] = step.ToolName
	}
	if step.Fingerprint != "" {
		if attributes == nil {
			attributes = map[string]string{}
		}
		attributes["fingerprint"] = step.Fingerprint
	}
	if len(attributes) == 0 {
		attributes = nil
	}

	stepID := step.StepID
	if strings.TrimSpace(stepID) == "" {
		stepID = fmt.Sprintf("step_%04d", index)
	}
	return VisualStep{
		RunID:       step.RunID,
		StepID:      stepID,
		Index:       index,
		Kind:        VisualStepKind(step.Kind),
		Phase:       phaseForStepKind(step.Kind),
		Summary:     firstNonEmpty(step.Summary, string(step.Kind)),
		Query:       step.Query,
		Command:     step.Command,
		Files:       behavior.SortFiles(step.Files),
		Target:      behavior.NormalizePathForKey(step.Target),
		EventIDs:    eventIDsFromBehaviorStep(step),
		Significant: step.Significant,
		Attributes:  attributes,
	}
}

func stepsForLoadedRun(run LoadedRun) []behavior.Step {
	if len(run.Signature.Steps) > 0 {
		return run.Signature.Steps
	}
	return run.Timeline.Steps
}

func buildDivergenceMarkers(runs []LoadedRun) []DivergenceMarker {
	if len(runs) < 2 {
		return nil
	}
	baseline := runs[0]
	markers := make([]DivergenceMarker, 0, len(runs)-1)
	for _, compare := range runs[1:] {
		diff := behavior.DiffSignatures(baseline.Signature, compare.Signature, behavior.DiffOptions{
			IgnoreRawBehavior:  true,
			IgnoreProcessNoise: true,
		})
		if diff.FirstDivergence == nil {
			continue
		}
		markers = append(markers, divergenceMarkerFromBehavior(baseline.RunID, compare.RunID, diff.FirstDivergence))
	}
	sort.SliceStable(markers, func(i, j int) bool {
		if markers[i].StepIndex != markers[j].StepIndex {
			return markers[i].StepIndex < markers[j].StepIndex
		}
		return markers[i].CompareRunID < markers[j].CompareRunID
	})
	return markers
}

func divergenceMarkerFromBehavior(baselineRunID, compareRunID string, divergence *behavior.Divergence) DivergenceMarker {
	marker := DivergenceMarker{
		BaselineRunID: baselineRunID,
		CompareRunID:  compareRunID,
		StepIndex:     divergence.Index + 1,
		Kind:          string(divergence.Kind),
		Summary:       divergence.Summary,
		First:         true,
		EventIDs:      uniqueStrings(divergence.RelatedEventIDs),
	}
	if divergence.RunAStep != nil {
		left := VisualStepFromBehaviorStep(*divergence.RunAStep)
		left.RunID = baselineRunID
		left.Divergent = true
		marker.Left = &left
		marker.EventIDs = uniqueStrings(append(marker.EventIDs, left.EventIDs...))
	}
	if divergence.RunBStep != nil {
		right := VisualStepFromBehaviorStep(*divergence.RunBStep)
		right.RunID = compareRunID
		right.Divergent = true
		marker.Right = &right
		marker.EventIDs = uniqueStrings(append(marker.EventIDs, right.EventIDs...))
	}
	return marker
}

func markDivergentLaneSteps(lanes []BehaviorLane, divergences []DivergenceMarker) {
	divergent := map[string]map[int]bool{}
	add := func(step *VisualStep) {
		if step == nil {
			return
		}
		if divergent[step.RunID] == nil {
			divergent[step.RunID] = map[int]bool{}
		}
		divergent[step.RunID][step.Index] = true
	}
	for _, marker := range divergences {
		add(marker.Left)
		add(marker.Right)
	}
	for laneIndex := range lanes {
		for stepIndex := range lanes[laneIndex].Steps {
			step := &lanes[laneIndex].Steps[stepIndex]
			if divergent[step.RunID][step.Index] {
				step.Divergent = true
			}
		}
	}
}

func firstDivergenceMarker(markers []DivergenceMarker) *DivergenceMarker {
	if len(markers) == 0 {
		return nil
	}
	first := markers[0]
	return &first
}

func summarizeVisualReport(report VisualReport, firstDivergence *DivergenceMarker) VisualSummary {
	var stepCount int
	var significant int
	for _, lane := range report.Lanes {
		for _, step := range lane.Steps {
			stepCount++
			if step.Significant {
				significant++
			}
		}
	}
	return VisualSummary{
		RunCount:             len(report.Runs),
		StepCount:            stepCount,
		SignificantStepCount: significant,
		DivergenceCount:      len(report.Divergences),
		FirstDivergence:      firstDivergence,
		FileCount:            len(report.FileAccess.Rows),
		MetricsCardCount:     countMetricCards(report.Metrics),
		Mode:                 report.Mode,
	}
}

func phaseForStepKind(kind behavior.StepKind) VisualPhase {
	switch kind {
	case behavior.StepSearch:
		return VisualPhaseDiscovery
	case behavior.StepReadFile, behavior.StepInspectTest, behavior.StepCallTool, behavior.StepCallMCPTool,
		behavior.StepPermissionRequest:
		return VisualPhaseInspection
	case behavior.StepEditFile, behavior.StepInstallDependency:
		return VisualPhaseEditing
	case behavior.StepRunTest, behavior.StepRunBuild, behavior.StepSkipValidation:
		return VisualPhaseValidation
	case behavior.StepRecoverFromError, behavior.StepContextCompact:
		return VisualPhaseRecovery
	case behavior.StepProcessResult:
		return VisualPhaseFinish
	default:
		return ""
	}
}

func eventIDsFromBehaviorStep(step behavior.Step) []string {
	ids := append([]string{}, step.SourceEventIDs...)
	for _, ref := range step.SourceRefs {
		if strings.TrimSpace(ref.EventID) != "" {
			ids = append(ids, ref.EventID)
		}
	}
	return uniqueStrings(ids)
}

func countMetricCards(groups []MetricsCardGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Cards)
	}
	return count
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
