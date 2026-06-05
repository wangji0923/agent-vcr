package behavior

import (
	"fmt"
	"strings"
)

type DiffOptions struct {
	IgnoreRawBehavior  bool `json:"ignore_raw_behavior,omitempty"`
	IgnoreProcessNoise bool `json:"ignore_process_noise,omitempty"`
}

type DiffSummary struct {
	Diverged          bool           `json:"diverged"`
	DivergenceKind    DivergenceKind `json:"divergence_kind"`
	CommonPrefixSteps int            `json:"common_prefix_steps"`
	StepCountA        int            `json:"step_count_a"`
	StepCountB        int            `json:"step_count_b"`
	Message           string         `json:"message,omitempty"`
}

type DiffResult struct {
	RunA            string       `json:"run_a"`
	RunB            string       `json:"run_b"`
	FirstDivergence *Divergence  `json:"first_divergence,omitempty"`
	Summary         DiffSummary  `json:"summary"`
	MetricsDelta    MetricsDelta `json:"metrics_delta"`
}

func DiffSignatures(a, b Signature, opts DiffOptions) DiffResult {
	leftSteps := normalizeDiffSteps(a.RunID, a.Steps, opts)
	rightSteps := normalizeDiffSteps(b.RunID, b.Steps, opts)
	divergence := FirstDivergenceSteps(leftSteps, rightSteps, opts)

	return DiffResult{
		RunA:            a.RunID,
		RunB:            b.RunID,
		FirstDivergence: divergence,
		Summary:         buildDiffSummary(divergence, len(leftSteps), len(rightSteps)),
		MetricsDelta:    DiffMetrics(a.Metrics, b.Metrics),
	}
}

func DiffTimelines(a, b Timeline, opts DiffOptions) DiffResult {
	leftSteps := normalizeDiffSteps(a.RunID, a.Steps, opts)
	rightSteps := normalizeDiffSteps(b.RunID, b.Steps, opts)
	leftTimeline := Timeline{SchemaVersion: SchemaVersion, RunID: a.RunID, Steps: leftSteps}
	rightTimeline := Timeline{SchemaVersion: SchemaVersion, RunID: b.RunID, Steps: rightSteps}

	return DiffSignatures(
		Signature{SchemaVersion: SchemaVersion, RunID: a.RunID, Steps: leftSteps, Metrics: ComputeMetrics(leftTimeline)},
		Signature{SchemaVersion: SchemaVersion, RunID: b.RunID, Steps: rightSteps, Metrics: ComputeMetrics(rightTimeline)},
		opts,
	)
}

func FirstDivergenceSteps(a, b []Step, opts DiffOptions) *Divergence {
	leftSteps := normalizeDiffSteps("", a, opts)
	rightSteps := normalizeDiffSteps("", b, opts)
	maxLen := max(len(leftSteps), len(rightSteps))
	for i := 0; i < maxLen; i++ {
		var left *Step
		var right *Step
		if i < len(leftSteps) {
			left = &leftSteps[i]
		}
		if i < len(rightSteps) {
			right = &rightSteps[i]
		}
		if left == nil || right == nil {
			return buildDivergence(i, left, right)
		}
		if diffStepKey(*left) != diffStepKey(*right) {
			return buildDivergence(i, left, right)
		}
	}
	return nil
}

func normalizeDiffSteps(runID string, steps []Step, opts DiffOptions) []Step {
	out := make([]Step, 0, len(steps))
	for _, step := range steps {
		if ignoreDiffStep(step.Kind, opts) {
			continue
		}
		step.SourceEventIDs = uniqueStrings(append(step.SourceEventIDs, eventIDsFromRefs(step.SourceRefs)...))
		normalized := normalizeSignatureStep(runID, step, len(out), SignatureOptions{})
		normalized.SourceRefs = nil
		normalized.Fingerprint = diffStepKey(normalized)
		out = append(out, normalized)
	}
	return out
}

func ignoreDiffStep(kind StepKind, opts DiffOptions) bool {
	if opts.IgnoreRawBehavior && kind == StepRawBehavior {
		return true
	}
	if opts.IgnoreProcessNoise {
		return kind == StepProcessStart || kind == StepProcessResult || kind == StepContextCompact
	}
	return false
}

func diffStepKey(step Step) string {
	return StepFingerprint(step)
}

func diffStepKeyWithoutResult(step Step) string {
	step.Result = ResultUnknown
	return StepFingerprint(step)
}

func buildDivergence(index int, left, right *Step) *Divergence {
	kind := classifyDivergenceKind(left, right)
	divergence := &Divergence{
		Index:           index,
		Kind:            kind,
		RunAStep:        copyStepPtr(left),
		RunBStep:        copyStepPtr(right),
		Summary:         summarizeDivergence(index, kind, left, right),
		Explanation:     explainDivergence(kind, left, right),
		RelatedEventIDs: relatedEventIDs(left, right),
	}
	return divergence
}

func classifyDivergenceKind(left, right *Step) DivergenceKind {
	switch {
	case left == nil:
		return DivergenceMissingInA
	case right == nil:
		return DivergenceMissingInB
	case diffStepKeyWithoutResult(*left) == diffStepKeyWithoutResult(*right) && normalizedResult(left.Result) != normalizedResult(right.Result):
		return DivergenceResultChanged
	default:
		return DivergenceStepChanged
	}
}

func summarizeDivergence(index int, kind DivergenceKind, left, right *Step) string {
	stepNumber := index + 1
	switch kind {
	case DivergenceMissingInA:
		return fmt.Sprintf("missing behavior at step %d: Run B has %s", stepNumber, describeDiffStep(right))
	case DivergenceMissingInB:
		return fmt.Sprintf("missing behavior at step %d: Run A has %s", stepNumber, describeDiffStep(left))
	case DivergenceResultChanged:
		return fmt.Sprintf("outcome divergence at step %d: Run A %s, Run B %s", stepNumber, describeResult(left), describeResult(right))
	}

	switch {
	case isSearchQueryDivergence(left, right):
		return fmt.Sprintf("search query divergence at step %d", stepNumber)
	case isValidationDivergence(left, right):
		return fmt.Sprintf("validation behavior divergence at step %d", stepNumber)
	case isEditTargetDivergence(left, right):
		return fmt.Sprintf("edit target divergence at step %d", stepNumber)
	case isToolCallDivergence(left, right):
		return fmt.Sprintf("tool call divergence at step %d", stepNumber)
	case isFileDiscoveryDivergence(left, right):
		return fmt.Sprintf("file discovery divergence at step %d", stepNumber)
	default:
		return fmt.Sprintf("behavior step changed at step %d: Run A %s; Run B %s", stepNumber, describeDiffStep(left), describeDiffStep(right))
	}
}

func explainDivergence(kind DivergenceKind, left, right *Step) string {
	switch kind {
	case DivergenceMissingInA:
		return "Run B performed an additional behavior before Run A had a matching step."
	case DivergenceMissingInB:
		return "Run A performed an additional behavior before Run B had a matching step."
	case DivergenceResultChanged:
		return "Equivalent behavior produced different outcomes."
	}

	switch {
	case isTestInspectionVsLegacyRead(left, right):
		return "Run B entered a legacy path before Run A inspected tests."
	case isTestInspectionVsLegacyRead(right, left):
		return "Run A entered a legacy path before Run B inspected tests."
	case isSearchQueryDivergence(left, right):
		return "Runs diverged during search/query selection."
	case isValidationDivergence(left, right):
		return "Runs diverged in validation behavior."
	case isEditTargetDivergence(left, right):
		return "Runs changed different edit targets."
	case isToolCallDivergence(left, right):
		return "Runs called different tools."
	case isFileDiscoveryDivergence(left, right):
		return "Runs inspected different files or discovery paths."
	default:
		return "Runs diverged at the first non-matching behavior step."
	}
}

func isSearchQueryDivergence(left, right *Step) bool {
	if left == nil || right == nil || left.Kind != StepSearch || right.Kind != StepSearch {
		return false
	}
	return normalizeDiffText(firstSignatureNonEmpty(left.Query, left.Command, left.Action, left.Target)) !=
		normalizeDiffText(firstSignatureNonEmpty(right.Query, right.Command, right.Action, right.Target))
}

func isFileDiscoveryDivergence(left, right *Step) bool {
	if left == nil || right == nil {
		return false
	}
	if !isFileDiscoveryStep(*left) && !isFileDiscoveryStep(*right) {
		return false
	}
	return strings.Join(diffStepPaths(*left), ",") != strings.Join(diffStepPaths(*right), ",") ||
		left.Kind != right.Kind
}

func isEditTargetDivergence(left, right *Step) bool {
	if left == nil || right == nil {
		return false
	}
	if left.Kind != StepEditFile && right.Kind != StepEditFile {
		return false
	}
	return strings.Join(diffStepPaths(*left), ",") != strings.Join(diffStepPaths(*right), ",") ||
		left.Kind != right.Kind
}

func isValidationDivergence(left, right *Step) bool {
	if left == nil || right == nil {
		return false
	}
	return left.IsValidation() || right.IsValidation()
}

func isToolCallDivergence(left, right *Step) bool {
	if left == nil || right == nil {
		return false
	}
	return isToolCallStep(*left) || isToolCallStep(*right)
}

func isTestInspectionVsLegacyRead(left, right *Step) bool {
	if left == nil || right == nil {
		return false
	}
	return left.Kind == StepInspectTest && right.Kind == StepReadFile && stepTouchesLegacyPath(*right)
}

func isFileDiscoveryStep(step Step) bool {
	return step.Kind == StepSearch || step.Kind == StepReadFile || step.Kind == StepInspectTest
}

func isToolCallStep(step Step) bool {
	return step.Kind == StepCallTool || step.Kind == StepCallMCPTool || strings.TrimSpace(step.ToolName) != ""
}

func stepTouchesLegacyPath(step Step) bool {
	for _, file := range diffStepPaths(step) {
		if IsLegacyPath(file) {
			return true
		}
	}
	return false
}

func diffStepPaths(step Step) []string {
	paths := append([]string{}, step.Files...)
	if strings.TrimSpace(step.Target) != "" {
		paths = append(paths, step.Target)
	}
	return SortFiles(paths)
}

func describeDiffStep(step *Step) string {
	if step == nil {
		return "no step"
	}
	if strings.TrimSpace(step.Summary) != "" {
		return step.Summary
	}
	return summarizeStep(*step)
}

func describeResult(step *Step) string {
	if step == nil {
		return string(ResultUnknown)
	}
	result := normalizedResult(step.Result)
	if result == ResultUnknown {
		return string(result)
	}
	return fmt.Sprintf("result=%s", result)
}

func normalizeDiffText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(normalizeFingerprintText(value)), " "))
}

func copyStepPtr(step *Step) *Step {
	if step == nil {
		return nil
	}
	copied := *step
	copied.Files = append([]string{}, step.Files...)
	copied.SourceRefs = append([]StepRef{}, step.SourceRefs...)
	copied.SourceEventIDs = append([]string{}, step.SourceEventIDs...)
	if step.Attributes != nil {
		copied.Attributes = make(map[string]string, len(step.Attributes))
		for key, value := range step.Attributes {
			copied.Attributes[key] = value
		}
	}
	return &copied
}

func relatedEventIDs(steps ...*Step) []string {
	var ids []string
	for _, step := range steps {
		if step == nil {
			continue
		}
		ids = append(ids, step.SourceEventIDs...)
		ids = append(ids, eventIDsFromRefs(step.SourceRefs)...)
	}
	return uniqueStrings(ids)
}

func eventIDsFromRefs(refs []StepRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.EventID) != "" {
			ids = append(ids, ref.EventID)
		}
	}
	return ids
}

func buildDiffSummary(divergence *Divergence, leftCount, rightCount int) DiffSummary {
	if divergence == nil {
		return DiffSummary{
			Diverged:          false,
			DivergenceKind:    DivergenceNone,
			CommonPrefixSteps: min(leftCount, rightCount),
			StepCountA:        leftCount,
			StepCountB:        rightCount,
			Message:           "no behavior divergence",
		}
	}
	return DiffSummary{
		Diverged:          true,
		DivergenceKind:    divergence.Kind,
		CommonPrefixSteps: divergence.Index,
		StepCountA:        leftCount,
		StepCountB:        rightCount,
		Message:           divergence.Summary,
	}
}
