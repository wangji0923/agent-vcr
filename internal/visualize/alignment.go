package visualize

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

const defaultAlignmentMinSimilarity = 4

type AlignOptions struct {
	BaselineRunID       string
	MinSimilarity       int
	Divergences         []VisualDivergence
	MarkFirstDivergence bool
}

func StepKey(step VisualStep) string {
	if step.Attributes != nil {
		for _, key := range []string{"fingerprint", "step_key", "stable_key"} {
			if value := normalizeStepText(step.Attributes[key]); value != "" {
				return value
			}
		}
	}
	parts := []string{
		string(step.Kind),
		normalizeStepText(step.Target),
		normalizeStepText(step.Query),
		normalizeStepText(step.Command),
		strings.Join(stepFilesForKey(step), ","),
	}
	return strings.Join(parts, "|")
}

func StepSimilarity(a, b VisualStep) int {
	if StepKey(a) == StepKey(b) {
		return 12
	}

	score := 0
	if a.Kind == b.Kind {
		score += 3
	} else if sameStepFamily(a.Kind, b.Kind) {
		score++
	}
	if sameNonEmpty(normalizePathOrText(a.Target), normalizePathOrText(b.Target)) {
		score += 3
	}
	if sameNonEmpty(normalizeStepText(a.Query), normalizeStepText(b.Query)) {
		score += 2
	}
	if sameNonEmpty(primaryStepFile(a), primaryStepFile(b)) {
		score += 2
	}
	if sameNonEmpty(commandKind(a.Command), commandKind(b.Command)) {
		score++
	}
	if stronglyDifferent(a.Kind, b.Kind) {
		score -= 3
	}
	return score
}

func AlignLanes(lanes []BehaviorLane, opts AlignOptions) []AlignmentRow {
	ordered := orderLanes(lanes, opts.BaselineRunID)
	switch len(ordered) {
	case 0:
		return nil
	case 1:
		rows := alignSingle(ordered[0])
		return markRows(rows, opts, ordered[0].RunID)
	case 2:
		rows := AlignPair(ordered[0], ordered[1], opts)
		return markRows(rows, opts, ordered[0].RunID)
	default:
		rows := alignMultiple(ordered, opts)
		return markRows(rows, opts, ordered[0].RunID)
	}
}

func AlignPair(left, right BehaviorLane, opts AlignOptions) []AlignmentRow {
	matches := lcsMatches(left.Steps, right.Steps, minSimilarity(opts))
	rows := make([]AlignmentRow, 0, len(left.Steps)+len(right.Steps))
	leftPos, rightPos := 0, 0
	for _, match := range matches {
		rows = appendUnmatchedRows(rows, left, right, leftPos, match.left, rightPos, match.right)
		rows = append(rows, rowForPair(left.RunID, right.RunID, &left.Steps[match.left], &right.Steps[match.right]))
		leftPos = match.left + 1
		rightPos = match.right + 1
	}
	rows = appendUnmatchedRows(rows, left, right, leftPos, len(left.Steps), rightPos, len(right.Steps))
	return renumberRows(rows)
}

func MarkDivergence(rows []AlignmentRow, divergences []VisualDivergence) []AlignmentRow {
	if len(rows) == 0 || len(divergences) == 0 {
		return rows
	}
	out := copyRows(rows)
	for _, divergence := range divergences {
		for i := range out {
			if !divergenceMatchesRow(divergence, out[i]) {
				continue
			}
			out[i].IsDivergent = true
			out[i].Reason = divergenceReason(divergence)
			markDivergentStep(out[i].Cells, divergence.BaselineRunID)
			markDivergentStep(out[i].Cells, divergence.CompareRunID)
			break
		}
	}
	return out
}

func FirstDivergence(rows []AlignmentRow, baselineRunID string) *VisualDivergence {
	if strings.TrimSpace(baselineRunID) == "" && len(rows) > 0 {
		for runID := range rows[0].Cells {
			baselineRunID = runID
			break
		}
	}
	for _, row := range rows {
		baseline := row.Cells[baselineRunID]
		for _, item := range sortedCells(row.Cells) {
			runID := item.runID
			compare := item.cell
			if runID == baselineRunID {
				continue
			}
			if !cellsDiverge(baseline, compare) {
				continue
			}
			marker := buildDivergenceMarker(row, baselineRunID, runID, baseline, compare)
			return &marker
		}
	}
	return nil
}

func markRows(rows []AlignmentRow, opts AlignOptions, baselineRunID string) []AlignmentRow {
	if len(opts.Divergences) > 0 {
		return MarkDivergence(rows, opts.Divergences)
	}
	if !opts.MarkFirstDivergence {
		return rows
	}
	divergence := FirstDivergence(rows, baselineRunID)
	if divergence == nil {
		return rows
	}
	return MarkDivergence(rows, []VisualDivergence{*divergence})
}

func alignSingle(lane BehaviorLane) []AlignmentRow {
	rows := make([]AlignmentRow, 0, len(lane.Steps))
	for i := range lane.Steps {
		rows = append(rows, AlignmentRow{
			Cells: map[string]StepCell{
				lane.RunID: stepCell(lane.RunID, &lane.Steps[i]),
			},
		})
	}
	return renumberRows(rows)
}

func alignMultiple(lanes []BehaviorLane, opts AlignOptions) []AlignmentRow {
	base := lanes[0]
	rows := AlignPair(base, lanes[1], opts)
	runIDs := laneRunIDs(lanes[:2])
	rows = fillMissingCells(rows, runIDs)
	for _, lane := range lanes[2:] {
		pairRows := AlignPair(base, lane, opts)
		rows = mergePairRows(rows, pairRows, base.RunID, lane.RunID)
		runIDs = append(runIDs, lane.RunID)
		rows = fillMissingCells(rows, runIDs)
	}
	return renumberRows(rows)
}

func mergePairRows(rows, pairRows []AlignmentRow, baselineRunID, runID string) []AlignmentRow {
	index := baselineRowIndex(rows, baselineRunID)
	lastPos := -1
	for _, pairRow := range pairRows {
		baseCell := pairRow.Cells[baselineRunID]
		runCell := pairRow.Cells[runID]
		if baseCell.Step != nil {
			key := rowStepIdentity(*baseCell.Step)
			pos, ok := index[key]
			if !ok {
				row := rowWithCells(map[string]StepCell{
					baselineRunID: baseCell,
					runID:         runCell,
				})
				rows = append(rows, row)
				pos = len(rows) - 1
				index[key] = pos
			}
			if rows[pos].Cells == nil {
				rows[pos].Cells = map[string]StepCell{}
			}
			rows[pos].Cells[runID] = runCell
			lastPos = pos
			continue
		}
		row := rowWithCells(map[string]StepCell{
			baselineRunID: gapCell(baselineRunID),
			runID:         runCell,
		})
		insertAt := lastPos + 1
		if insertAt < 0 || insertAt > len(rows) {
			insertAt = len(rows)
		}
		rows = insertRow(rows, insertAt, row)
		lastPos = insertAt
		index = baselineRowIndex(rows, baselineRunID)
	}
	return rows
}

func appendUnmatchedRows(rows []AlignmentRow, left, right BehaviorLane, leftStart, leftEnd, rightStart, rightEnd int) []AlignmentRow {
	leftCount := leftEnd - leftStart
	rightCount := rightEnd - rightStart
	paired := min(leftCount, rightCount)
	for i := 0; i < paired; i++ {
		rows = append(rows, rowForPair(left.RunID, right.RunID, &left.Steps[leftStart+i], &right.Steps[rightStart+i]))
	}
	for i := paired; i < leftCount; i++ {
		rows = append(rows, rowForPair(left.RunID, right.RunID, &left.Steps[leftStart+i], nil))
	}
	for i := paired; i < rightCount; i++ {
		rows = append(rows, rowForPair(left.RunID, right.RunID, nil, &right.Steps[rightStart+i]))
	}
	return rows
}

func rowForPair(leftRunID, rightRunID string, left, right *VisualStep) AlignmentRow {
	return rowWithCells(map[string]StepCell{
		leftRunID:  cellOrGap(leftRunID, left),
		rightRunID: cellOrGap(rightRunID, right),
	})
}

func rowWithCells(cells map[string]StepCell) AlignmentRow {
	if cells == nil {
		cells = map[string]StepCell{}
	}
	return AlignmentRow{Cells: cells}
}

func cellOrGap(runID string, step *VisualStep) StepCell {
	if step == nil {
		return gapCell(runID)
	}
	return stepCell(runID, step)
}

func stepCell(runID string, step *VisualStep) StepCell {
	return StepCell{RunID: runID, Step: copyStepPtr(step)}
}

func gapCell(runID string) StepCell {
	return StepCell{RunID: runID, Gap: true}
}

func fillMissingCells(rows []AlignmentRow, runIDs []string) []AlignmentRow {
	for i := range rows {
		if rows[i].Cells == nil {
			rows[i].Cells = map[string]StepCell{}
		}
		for _, runID := range runIDs {
			if _, ok := rows[i].Cells[runID]; !ok {
				rows[i].Cells[runID] = gapCell(runID)
			}
		}
	}
	return rows
}

func copyRows(rows []AlignmentRow) []AlignmentRow {
	out := make([]AlignmentRow, len(rows))
	for i, row := range rows {
		out[i] = row
		out[i].Cells = make(map[string]StepCell, len(row.Cells))
		for runID, cell := range row.Cells {
			copied := cell
			copied.Step = copyStepPtr(cell.Step)
			out[i].Cells[runID] = copied
		}
	}
	return out
}

func renumberRows(rows []AlignmentRow) []AlignmentRow {
	for i := range rows {
		rows[i].RowIndex = i + 1
	}
	return rows
}

func orderLanes(lanes []BehaviorLane, baselineRunID string) []BehaviorLane {
	if len(lanes) == 0 {
		return nil
	}
	ordered := append([]BehaviorLane{}, lanes...)
	if strings.TrimSpace(baselineRunID) == "" || ordered[0].RunID == baselineRunID {
		return ordered
	}
	for i := range ordered {
		if ordered[i].RunID != baselineRunID {
			continue
		}
		baseline := ordered[i]
		copy(ordered[1:i+1], ordered[0:i])
		ordered[0] = baseline
		break
	}
	return ordered
}

type alignmentMatch struct {
	left  int
	right int
}

func lcsMatches(left, right []VisualStep, minScore int) []alignmentMatch {
	dp := make([][]int, len(left)+1)
	for i := range dp {
		dp[i] = make([]int, len(right)+1)
	}
	for i := range left {
		for j := range right {
			best := max(dp[i][j+1], dp[i+1][j])
			if score := StepSimilarity(left[i], right[j]); score >= minScore && dp[i][j]+score > best {
				best = dp[i][j] + score
			}
			dp[i+1][j+1] = best
		}
	}

	var reversed []alignmentMatch
	for i, j := len(left), len(right); i > 0 && j > 0; {
		score := StepSimilarity(left[i-1], right[j-1])
		if score >= minScore && dp[i][j] == dp[i-1][j-1]+score {
			reversed = append(reversed, alignmentMatch{left: i - 1, right: j - 1})
			i--
			j--
			continue
		}
		if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	matches := make([]alignmentMatch, len(reversed))
	for i := range reversed {
		matches[len(reversed)-1-i] = reversed[i]
	}
	return matches
}

func minSimilarity(opts AlignOptions) int {
	if opts.MinSimilarity > 0 {
		return opts.MinSimilarity
	}
	return defaultAlignmentMinSimilarity
}

func sameStepFamily(a, b VisualStepKind) bool {
	return stepFamily(a) != "" && stepFamily(a) == stepFamily(b)
}

func stepFamily(kind VisualStepKind) string {
	switch kind {
	case VisualStepSearch, VisualStepReadFile, VisualStepInspectTest:
		return "discovery"
	case VisualStepEditFile:
		return "editing"
	case VisualStepRunTest, VisualStepRunBuild:
		return "validation"
	case VisualStepRecoverFromError:
		return "recovery"
	case VisualStepCallTool, VisualStepCallMCPTool, VisualStepProcessStart, VisualStepProcessResult:
		return "tool"
	default:
		return ""
	}
}

func stronglyDifferent(a, b VisualStepKind) bool {
	if a == b {
		return false
	}
	fa, fb := stepFamily(a), stepFamily(b)
	return (fa == "editing" && (fb == "discovery" || fb == "validation")) ||
		(fb == "editing" && (fa == "discovery" || fa == "validation")) ||
		(fa == "validation" && fb == "discovery") ||
		(fb == "validation" && fa == "discovery")
}

func stepFilesForKey(step VisualStep) []string {
	files := append([]string{}, step.Files...)
	if strings.TrimSpace(step.Target) != "" {
		files = append(files, step.Target)
	}
	return behavior.SortFiles(files)
}

func primaryStepFile(step VisualStep) string {
	files := stepFilesForKey(step)
	if len(files) == 0 {
		return ""
	}
	return files[0]
}

func normalizePathOrText(value string) string {
	normalizedPath := behavior.NormalizePathForKey(value)
	if normalizedPath != "" {
		return normalizedPath
	}
	return normalizeStepText(value)
}

func normalizeStepText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.ToLower(value)
}

func sameNonEmpty(a, b string) bool {
	return a != "" && b != "" && a == b
}

func commandKind(command string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	if len(fields) == 0 {
		return ""
	}
	if len(fields) >= 2 {
		switch fields[0] {
		case "go", "npm", "pnpm", "yarn", "cargo", "mvn", "gradle", "python", "pytest":
			return fields[0] + " " + fields[1]
		}
	}
	return fields[0]
}

func laneRunIDs(lanes []BehaviorLane) []string {
	out := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		out = append(out, lane.RunID)
	}
	return out
}

func baselineRowIndex(rows []AlignmentRow, baselineRunID string) map[string]int {
	index := map[string]int{}
	for i, row := range rows {
		cell := row.Cells[baselineRunID]
		if cell.Step == nil {
			continue
		}
		index[rowStepIdentity(*cell.Step)] = i
	}
	return index
}

func rowStepIdentity(step VisualStep) string {
	if strings.TrimSpace(step.StepID) != "" {
		return step.RunID + ":" + step.StepID
	}
	return fmt.Sprintf("%s:%d:%s", step.RunID, step.Index, StepKey(step))
}

func insertRow(rows []AlignmentRow, index int, row AlignmentRow) []AlignmentRow {
	rows = append(rows, AlignmentRow{})
	copy(rows[index+1:], rows[index:])
	rows[index] = row
	return rows
}

func sortedCells(cells map[string]StepCell) []struct {
	runID string
	cell  StepCell
} {
	runIDs := make([]string, 0, len(cells))
	for runID := range cells {
		runIDs = append(runIDs, runID)
	}
	sort.Strings(runIDs)
	out := make([]struct {
		runID string
		cell  StepCell
	}, 0, len(runIDs))
	for _, runID := range runIDs {
		out = append(out, struct {
			runID string
			cell  StepCell
		}{runID: runID, cell: cells[runID]})
	}
	return out
}

func cellsDiverge(baseline, compare StepCell) bool {
	switch {
	case baseline.Step == nil && compare.Step == nil:
		return false
	case baseline.Step == nil || compare.Step == nil:
		return true
	default:
		return StepKey(*baseline.Step) != StepKey(*compare.Step)
	}
}

func buildDivergenceMarker(row AlignmentRow, baselineRunID, compareRunID string, baseline, compare StepCell) VisualDivergence {
	kind := "step_changed"
	switch {
	case baseline.Step == nil:
		kind = "step_missing_in_baseline"
	case compare.Step == nil:
		kind = "step_missing_in_compare"
	}
	marker := VisualDivergence{
		BaselineRunID:  baselineRunID,
		CompareRunID:   compareRunID,
		StepIndex:      divergenceStepIndex(baseline, compare),
		AlignmentIndex: row.RowIndex,
		Kind:           kind,
		Summary:        summarizeVisualDivergence(kind, baseline, compare),
		First:          true,
		Left:           copyStepPtr(baseline.Step),
		Right:          copyStepPtr(compare.Step),
		EventIDs:       mergeEventIDs(baseline.Step, compare.Step),
	}
	return marker
}

func divergenceStepIndex(baseline, compare StepCell) int {
	if baseline.Step != nil {
		return baseline.Step.Index
	}
	if compare.Step != nil {
		return compare.Step.Index
	}
	return 0
}

func summarizeVisualDivergence(kind string, baseline, compare StepCell) string {
	switch kind {
	case "step_missing_in_baseline":
		return fmt.Sprintf("baseline has no step while compare has %s", describeVisualStep(compare.Step))
	case "step_missing_in_compare":
		return fmt.Sprintf("compare has no step while baseline has %s", describeVisualStep(baseline.Step))
	default:
		return fmt.Sprintf("baseline has %s while compare has %s", describeVisualStep(baseline.Step), describeVisualStep(compare.Step))
	}
}

func describeVisualStep(step *VisualStep) string {
	if step == nil {
		return "no step"
	}
	if strings.TrimSpace(step.Summary) != "" {
		return step.Summary
	}
	return string(step.Kind)
}

func mergeEventIDs(steps ...*VisualStep) []string {
	seen := map[string]bool{}
	var ids []string
	for _, step := range steps {
		if step == nil {
			continue
		}
		for _, id := range step.EventIDs {
			if strings.TrimSpace(id) == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func divergenceMatchesRow(divergence VisualDivergence, row AlignmentRow) bool {
	if divergence.AlignmentIndex > 0 && divergence.AlignmentIndex == row.RowIndex {
		return true
	}
	if divergence.Left != nil && rowHasStep(row, *divergence.Left) {
		return true
	}
	if divergence.Right != nil && rowHasStep(row, *divergence.Right) {
		return true
	}
	for _, runID := range []string{divergence.BaselineRunID, divergence.CompareRunID} {
		cell := row.Cells[runID]
		if cell.Step == nil {
			continue
		}
		if cell.Step.Index == divergence.StepIndex || row.RowIndex == divergence.StepIndex+1 {
			return true
		}
	}
	return false
}

func rowHasStep(row AlignmentRow, step VisualStep) bool {
	cell := row.Cells[step.RunID]
	if cell.Step == nil {
		return false
	}
	if strings.TrimSpace(step.StepID) != "" && cell.Step.StepID == step.StepID {
		return true
	}
	return cell.Step.Index == step.Index && StepKey(*cell.Step) == StepKey(step)
}

func divergenceReason(divergence VisualDivergence) string {
	if divergence.First {
		if strings.TrimSpace(divergence.Kind) != "" {
			return "first_divergence:" + divergence.Kind
		}
		return "first_divergence"
	}
	if strings.TrimSpace(divergence.Kind) != "" {
		return divergence.Kind
	}
	return "divergence"
}

func markDivergentStep(cells map[string]StepCell, runID string) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	cell, ok := cells[runID]
	if !ok || cell.Step == nil {
		return
	}
	cell.Step.Divergent = true
	cells[runID] = cell
}

func copyStepPtr(step *VisualStep) *VisualStep {
	if step == nil {
		return nil
	}
	copied := *step
	copied.Files = append([]string{}, step.Files...)
	copied.EventIDs = append([]string{}, step.EventIDs...)
	if step.Attributes != nil {
		copied.Attributes = make(map[string]string, len(step.Attributes))
		for key, value := range step.Attributes {
			copied.Attributes[key] = value
		}
	}
	return &copied
}
