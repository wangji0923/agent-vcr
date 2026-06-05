package behavior

import (
	"fmt"
	"strings"
	"time"
)

func BuildSignatureFromTimeline(timeline Timeline, opts SignatureOptions) Signature {
	return buildSignatureFromTimelineAt(timeline, opts, time.Now().UTC(), "")
}

func buildSignatureFromTimelineAt(timeline Timeline, opts SignatureOptions, generatedAt time.Time, sourceTraceHash string) Signature {
	steps := make([]Step, 0, len(timeline.Steps))
	for _, step := range timeline.Steps {
		if isSignatureNoise(step.Kind, opts) {
			continue
		}
		steps = append(steps, normalizeSignatureStep(timeline.RunID, step, len(steps), opts))
	}

	return Signature{
		SchemaVersion:   SchemaVersion,
		RunID:           timeline.RunID,
		SourceTraceHash: sourceTraceHash,
		GeneratedAt:     generatedAt.UTC(),
		Steps:           steps,
		Metrics:         summarizeSignatureSteps(steps),
		Options:         opts,
	}
}

func isSignatureNoise(kind StepKind, opts SignatureOptions) bool {
	switch kind {
	case StepRawBehavior:
		return !opts.IncludeRawBehavior
	case StepProcessStart, StepProcessResult, StepContextCompact:
		return !opts.IncludeProcessNoise
	default:
		return false
	}
}

func normalizeSignatureStep(runID string, step Step, index int, opts SignatureOptions) Step {
	step.RunID = firstSignatureNonEmpty(step.RunID, runID)
	step.Index = index
	step.StepID = fmt.Sprintf("behavior_step_%04d", index+1)
	step.Result = normalizedResult(step.Result)
	step.Files = SortFiles(step.Files)
	step.SourceEventIDs = uniqueStrings(step.SourceEventIDs)
	if !opts.IncludeSourceRefs {
		step.SourceRefs = nil
	}
	step.Fingerprint = StepFingerprint(step)
	step.Significant = true
	if strings.TrimSpace(step.Summary) == "" {
		step.Summary = summarizeStep(step)
	}
	return step
}

func summarizeStep(step Step) string {
	switch step.Kind {
	case StepSearch:
		return formatSummary("search", firstSignatureNonEmpty(step.Query, step.Command, step.Action, step.Target))
	case StepReadFile, StepInspectTest:
		return formatSummary("read", firstSignatureNonEmpty(step.Target, strings.Join(step.Files, ", ")))
	case StepEditFile:
		return formatSummary("edit", firstSignatureNonEmpty(strings.Join(step.Files, ", "), step.Target))
	case StepRunTest:
		return formatSummary("run tests", firstSignatureNonEmpty(step.Command, step.Action, step.Target))
	case StepRunBuild:
		return formatSummary("run build", firstSignatureNonEmpty(step.Command, step.Action, step.Target))
	case StepInstallDependency:
		return formatSummary("install dependency", firstSignatureNonEmpty(step.Command, step.Action, step.Target))
	case StepCallMCPTool:
		return formatSummary("call MCP tool", firstSignatureNonEmpty(step.ToolName, step.Action, step.Target))
	case StepCallTool:
		return formatSummary("call tool", firstSignatureNonEmpty(step.ToolName, step.Action, step.Target))
	case StepRecoverFromError:
		return formatSummary("recover from error", firstSignatureNonEmpty(step.Action, step.Command, step.Target))
	case StepSkipValidation:
		return formatSummary("skip validation", firstSignatureNonEmpty(step.Action, step.Command, step.Target))
	case StepPermissionRequest:
		return formatSummary("permission request", firstSignatureNonEmpty(step.Action, step.ToolName, step.Target))
	default:
		return formatSummary(string(step.Kind), firstSignatureNonEmpty(step.Action, step.Command, step.Target, step.ToolName))
	}
}

func formatSummary(prefix string, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return prefix
	}
	return prefix + ": " + detail
}

func summarizeSignatureSteps(steps []Step) Metrics {
	var metrics Metrics
	metrics.ToolEfficiency.TotalSteps = len(steps)
	for _, step := range steps {
		switch step.Kind {
		case StepSearch:
			metrics.ToolEfficiency.SearchSteps++
		case StepCallTool, StepCallMCPTool:
			metrics.ToolEfficiency.ToolCalls++
		}
		if step.Result == ResultFailure && (step.Command != "" || step.ToolName != "") {
			metrics.ToolEfficiency.FailedCommands++
		}
	}
	return metrics
}

func firstSignatureNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
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
