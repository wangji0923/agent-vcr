package visualize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	_ "embed"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/redact"
)

//go:embed templates/behavior_visual_report.html.tmpl
var behaviorVisualReportTemplate string

type HTMLOptions struct {
	Redacted bool
	Title    string
}

func RenderHTML(report *VisualReport, opts HTMLOptions) ([]byte, error) {
	if report == nil {
		return nil, fmt.Errorf("visual report is nil")
	}
	safeReport, err := redactVisualReport(*report)
	if err != nil {
		return nil, err
	}
	safeReport.Options.Redacted = safeReport.Options.Redacted || opts.Redacted
	if err := ValidateReport(&safeReport); err != nil {
		return nil, fmt.Errorf("invalid visual report: %w", err)
	}

	tmpl, err := template.New("behavior_visual_report.html.tmpl").Parse(behaviorVisualReportTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse visual report template: %w", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, buildHTMLData(&safeReport, opts)); err != nil {
		return nil, fmt.Errorf("render visual report html: %w", err)
	}
	return out.Bytes(), nil
}

func redactVisualReport(report VisualReport) (VisualReport, error) {
	data, err := json.Marshal(report)
	if err != nil {
		return VisualReport{}, fmt.Errorf("marshal visual report for redaction: %w", err)
	}
	data = redact.ApplyToBytes(data, config.Default().Redaction)
	var out VisualReport
	if err := json.Unmarshal(data, &out); err != nil {
		return VisualReport{}, fmt.Errorf("unmarshal redacted visual report: %w", err)
	}
	out.Options.Redacted = true
	return out, nil
}

type htmlReportData struct {
	Title            string
	SchemaVersion    string
	GeneratedAt      string
	Mode             string
	Redacted         bool
	RunIDs           []string
	Runs             []htmlRun
	Summary          htmlSummary
	Warnings         []string
	Lanes            []htmlLane
	Alignment        []htmlAlignmentRow
	HasAlignment     bool
	FileAccess       []htmlFileAccessRow
	HasFileAccess    bool
	SearchScopes     []htmlSearchScopeRow
	HasSearchScopes  bool
	MetricGroups     []htmlMetricGroup
	HasMetrics       bool
	PathGraph        htmlPathGraph
	HasPathGraph     bool
	FirstDivergence  *htmlDivergence
	DivergencesByRun []htmlDivergence
	HasDivergence    bool
	RawEventIDs      []string
	HasRawReferences bool
}

type htmlSummary struct {
	RunCount             int
	StepCount            int
	SignificantStepCount int
	DivergenceCount      int
	FileCount            int
	MetricsCardCount     int
	Mode                 string
}

type htmlRun struct {
	RunID        string
	Label        string
	Source       string
	Status       string
	StartedAt    string
	EndedAt      string
	StepCount    int
	SummaryPairs []htmlPair
}

type htmlPair struct {
	Key   string
	Value string
}

type htmlLane struct {
	RunID string
	Label string
	Steps []htmlStep
}

type htmlStep struct {
	RunID          string
	StepID         string
	Index          int
	Kind           string
	KindClass      string
	Phase          string
	Summary        string
	Query          string
	Command        string
	Files          []string
	Target         string
	EventIDs       []string
	Significant    bool
	Divergent      bool
	Attributes     []htmlPair
	HasDetails     bool
	FileList       string
	EventIDList    string
	AttributeCount int
}

type htmlAlignmentRow struct {
	RowIndex    int
	IsDivergent bool
	Reason      string
	Cells       []htmlStepCell
}

type htmlStepCell struct {
	RunID string
	Label string
	Gap   bool
	Step  htmlStep
}

type htmlDivergence struct {
	BaselineRunID  string
	CompareRunID   string
	StepIndex      int
	AlignmentIndex int
	Kind           string
	Summary        string
	First          bool
	EventIDs       []string
	Left           *htmlStep
	Right          *htmlStep
}

type htmlFileAccessRow struct {
	Path  string
	Cells []htmlFileUse
}

type htmlFileUse struct {
	RunID       string
	Label       string
	ReadCount   int
	EditCount   int
	FirstStep   string
	LastStep    string
	FirstAction string
	LastAction  string
	Summary     string
	Level       string
}

type htmlSearchScopeRow struct {
	Scope string
	Cells []htmlSearchScopeUse
}

type htmlSearchScopeUse struct {
	RunID       string
	Label       string
	SearchCount int
	FirstStep   string
	LastStep    string
	Queries     string
	Level       string
}

type htmlMetricGroup struct {
	Group string
	Rows  []htmlMetricRow
}

type htmlMetricRow struct {
	Name  string
	Level string
	Cells []htmlMetricCell
}

type htmlMetricCell struct {
	RunID string
	Label string
	Value string
	Level string
}

type htmlPathGraph struct {
	Width      int
	Height     int
	NodeWidth  int
	NodeHeight int
	Nodes      []htmlPathNode
	Edges      []htmlPathEdge
}

type htmlPathNode struct {
	ID          string
	Label       string
	ShortLabel  string
	Kind        string
	ShortKind   string
	RunIDs      string
	ShortRunIDs string
	Title       string
	X           int
	Y           int
	KindCls     string
	RunCount    int
}

type htmlPathEdge struct {
	From   string
	To     string
	RunIDs string
	X1     int
	Y1     int
	X2     int
	Y2     int
}

func buildHTMLData(report *VisualReport, opts HTMLOptions) htmlReportData {
	runIDs := orderedRunIDs(report.Runs)
	runLabels := runLabelMap(report.Runs)
	alignment := report.Alignment
	if len(alignment) == 0 && len(report.Lanes) > 1 {
		alignment = AlignLanes(report.Lanes, AlignOptions{
			BaselineRunID:       report.Options.BaselineRunID,
			Divergences:         report.Divergences,
			MarkFirstDivergence: len(report.Divergences) == 0 && len(report.Lanes) > 1,
		})
	}

	graph := report.PathGraph
	if graph == nil || len(graph.Nodes) == 0 {
		graph = BuildPathGraph(report.Lanes)
	}

	first := firstHTMLDivergence(report)
	divergencesByRun := []htmlDivergence(nil)
	if first != nil || report.Summary.DivergenceCount > 0 || len(report.Divergences) > 0 {
		divergencesByRun = buildHTMLDivergencesByRun(alignment, report.Options.BaselineRunID, runIDs)
	}
	eventIDs := collectReportEventIDs(report)
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "agent-vcr Behavior Visualization"
	}

	return htmlReportData{
		Title:         title,
		SchemaVersion: report.SchemaVersion,
		GeneratedAt:   formatTime(report.GeneratedAt),
		Mode:          string(report.Mode),
		Redacted:      report.Options.Redacted,
		RunIDs:        runIDs,
		Runs:          buildHTMLRuns(report.Runs),
		Summary: htmlSummary{
			RunCount:             report.Summary.RunCount,
			StepCount:            report.Summary.StepCount,
			SignificantStepCount: report.Summary.SignificantStepCount,
			DivergenceCount:      report.Summary.DivergenceCount,
			FileCount:            len(report.FileAccess.Rows),
			MetricsCardCount:     countMetricCards(report.Metrics),
			Mode:                 string(report.Summary.Mode),
		},
		Warnings:         append([]string{}, report.Warnings...),
		Lanes:            buildHTMLLanes(report.Lanes),
		Alignment:        buildHTMLAlignment(alignment, runIDs, runLabels),
		HasAlignment:     len(alignment) > 0,
		FileAccess:       buildHTMLFileAccess(report.FileAccess, runIDs, runLabels),
		HasFileAccess:    len(report.FileAccess.Rows) > 0,
		SearchScopes:     buildHTMLSearchScopes(report.SearchScopes, runIDs, runLabels),
		HasSearchScopes:  len(report.SearchScopes.Rows) > 0,
		MetricGroups:     buildHTMLMetricGroups(report.Metrics, runIDs, runLabels),
		HasMetrics:       countMetricCards(report.Metrics) > 0,
		PathGraph:        buildHTMLPathGraph(graph),
		HasPathGraph:     graph != nil && len(graph.Nodes) > 0,
		FirstDivergence:  first,
		DivergencesByRun: divergencesByRun,
		HasDivergence:    first != nil,
		RawEventIDs:      eventIDs,
		HasRawReferences: len(eventIDs) > 0,
	}
}

func orderedRunIDs(runs []VisualRun) []string {
	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		runIDs = append(runIDs, run.RunID)
	}
	return runIDs
}

func runLabelMap(runs []VisualRun) map[string]string {
	out := make(map[string]string, len(runs))
	for _, run := range runs {
		label := strings.TrimSpace(run.Label)
		if label == "" {
			label = run.RunID
		}
		out[run.RunID] = label
	}
	return out
}

func buildHTMLRuns(runs []VisualRun) []htmlRun {
	out := make([]htmlRun, 0, len(runs))
	for _, run := range runs {
		label := strings.TrimSpace(run.Label)
		if label == "" {
			label = run.RunID
		}
		out = append(out, htmlRun{
			RunID:        run.RunID,
			Label:        label,
			Source:       run.Source,
			Status:       run.Status,
			StartedAt:    formatTimePtr(run.StartedAt),
			EndedAt:      formatTimePtr(run.EndedAt),
			StepCount:    run.StepCount,
			SummaryPairs: summaryPairs(run.Summary),
		})
	}
	return out
}

func buildHTMLLanes(lanes []BehaviorLane) []htmlLane {
	out := make([]htmlLane, 0, len(lanes))
	for _, lane := range lanes {
		steps := make([]htmlStep, 0, len(lane.Steps))
		for i := range lane.Steps {
			steps = append(steps, buildHTMLStep(lane.Steps[i]))
		}
		out = append(out, htmlLane{RunID: lane.RunID, Label: firstHTML(lane.Label, lane.RunID), Steps: steps})
	}
	return out
}

func buildHTMLAlignment(rows []AlignmentRow, runIDs []string, labels map[string]string) []htmlAlignmentRow {
	out := make([]htmlAlignmentRow, 0, len(rows))
	for _, row := range rows {
		cells := make([]htmlStepCell, 0, len(runIDs))
		for _, runID := range runIDs {
			cell, ok := row.Cells[runID]
			if !ok {
				cell = StepCell{RunID: runID, Gap: true}
			}
			view := htmlStepCell{
				RunID: runID,
				Label: firstHTML(labels[runID], runID),
				Gap:   cell.Gap || cell.Step == nil,
			}
			if cell.Step != nil {
				view.Step = buildHTMLStep(*cell.Step)
			}
			cells = append(cells, view)
		}
		out = append(out, htmlAlignmentRow{
			RowIndex:    row.RowIndex,
			IsDivergent: row.IsDivergent,
			Reason:      row.Reason,
			Cells:       cells,
		})
	}
	return out
}

func buildHTMLStep(step VisualStep) htmlStep {
	attrs := attributesPairs(step.Attributes)
	files := append([]string{}, step.Files...)
	eventIDs := append([]string{}, step.EventIDs...)
	view := htmlStep{
		RunID:          step.RunID,
		StepID:         step.StepID,
		Index:          step.Index,
		Kind:           string(step.Kind),
		KindClass:      classToken(string(step.Kind)),
		Phase:          string(step.Phase),
		Summary:        firstHTML(step.Summary, string(step.Kind)),
		Query:          step.Query,
		Command:        step.Command,
		Files:          files,
		Target:         step.Target,
		EventIDs:       eventIDs,
		Significant:    step.Significant,
		Divergent:      step.Divergent,
		Attributes:     attrs,
		FileList:       strings.Join(files, ", "),
		EventIDList:    strings.Join(eventIDs, ", "),
		AttributeCount: len(attrs),
	}
	view.HasDetails = view.Command != "" || view.Query != "" || view.Target != "" || len(view.Files) > 0 || len(view.EventIDs) > 0 || len(view.Attributes) > 0
	return view
}

func firstHTMLDivergence(report *VisualReport) *htmlDivergence {
	var marker *DivergenceMarker
	if report.Summary.FirstDivergence != nil {
		marker = report.Summary.FirstDivergence
	} else {
		for i := range report.Divergences {
			if report.Divergences[i].First {
				marker = &report.Divergences[i]
				break
			}
		}
		if marker == nil && len(report.Divergences) > 0 {
			marker = &report.Divergences[0]
		}
	}
	if marker == nil {
		return nil
	}
	view := &htmlDivergence{
		BaselineRunID:  marker.BaselineRunID,
		CompareRunID:   marker.CompareRunID,
		StepIndex:      marker.StepIndex,
		AlignmentIndex: marker.AlignmentIndex,
		Kind:           marker.Kind,
		Summary:        marker.Summary,
		First:          marker.First,
		EventIDs:       append([]string{}, marker.EventIDs...),
	}
	if marker.Left != nil {
		left := buildHTMLStep(*marker.Left)
		view.Left = &left
	}
	if marker.Right != nil {
		right := buildHTMLStep(*marker.Right)
		view.Right = &right
	}
	return view
}

func buildHTMLDivergencesByRun(rows []AlignmentRow, baselineRunID string, runIDs []string) []htmlDivergence {
	if len(rows) == 0 || len(runIDs) < 2 {
		return nil
	}
	if strings.TrimSpace(baselineRunID) == "" {
		baselineRunID = runIDs[0]
	}
	var out []htmlDivergence
	for _, runID := range runIDs {
		if runID == baselineRunID {
			continue
		}
		for _, row := range rows {
			baseline := row.Cells[baselineRunID]
			compare := row.Cells[runID]
			if !cellsDiverge(baseline, compare) {
				continue
			}
			marker := buildDivergenceMarker(row, baselineRunID, runID, baseline, compare)
			view := htmlDivergence{
				BaselineRunID:  marker.BaselineRunID,
				CompareRunID:   marker.CompareRunID,
				StepIndex:      marker.StepIndex,
				AlignmentIndex: marker.AlignmentIndex,
				Kind:           marker.Kind,
				Summary:        marker.Summary,
				First:          marker.First,
				EventIDs:       append([]string{}, marker.EventIDs...),
			}
			if marker.Left != nil {
				left := buildHTMLStep(*marker.Left)
				view.Left = &left
			}
			if marker.Right != nil {
				right := buildHTMLStep(*marker.Right)
				view.Right = &right
			}
			out = append(out, view)
			break
		}
	}
	return out
}

func buildHTMLFileAccess(compare FileAccessCompare, runIDs []string, labels map[string]string) []htmlFileAccessRow {
	rows := make([]htmlFileAccessRow, 0, len(compare.Rows))
	for _, row := range compare.Rows {
		cells := make([]htmlFileUse, 0, len(runIDs))
		for _, runID := range runIDs {
			use := row.Runs[runID]
			cells = append(cells, htmlFileUse{
				RunID:       runID,
				Label:       firstHTML(labels[runID], runID),
				ReadCount:   use.ReadCount,
				EditCount:   use.EditCount,
				FirstStep:   stepNumber(use.FirstStep),
				LastStep:    stepNumber(use.LastStep),
				FirstAction: use.FirstAction,
				LastAction:  use.LastAction,
				Summary:     fileUseSummary(use),
				Level:       fileUseLevel(use),
			})
		}
		rows = append(rows, htmlFileAccessRow{Path: row.Path, Cells: cells})
	}
	return rows
}

func buildHTMLSearchScopes(compare SearchScopeCompare, runIDs []string, labels map[string]string) []htmlSearchScopeRow {
	rows := make([]htmlSearchScopeRow, 0, len(compare.Rows))
	for _, row := range compare.Rows {
		cells := make([]htmlSearchScopeUse, 0, len(runIDs))
		for _, runID := range runIDs {
			use := row.Runs[runID]
			cells = append(cells, htmlSearchScopeUse{
				RunID:       runID,
				Label:       firstHTML(labels[runID], runID),
				SearchCount: use.SearchCount,
				FirstStep:   stepNumber(use.FirstStep),
				LastStep:    stepNumber(use.LastStep),
				Queries:     strings.Join(use.Queries, " | "),
				Level:       searchScopeLevel(use),
			})
		}
		rows = append(rows, htmlSearchScopeRow{Scope: row.Scope, Cells: cells})
	}
	return rows
}

func buildHTMLMetricGroups(groups []MetricsCardGroup, runIDs []string, labels map[string]string) []htmlMetricGroup {
	type key struct {
		group string
		name  string
	}
	values := map[key]map[string]MetricsCard{}
	var order []key
	seen := map[key]bool{}
	for _, group := range groups {
		for _, card := range group.Cards {
			k := key{group: card.Group, name: card.Name}
			if !seen[k] {
				seen[k] = true
				order = append(order, k)
			}
			if values[k] == nil {
				values[k] = map[string]MetricsCard{}
			}
			values[k][group.RunID] = card
		}
	}

	byGroup := map[string][]htmlMetricRow{}
	var groupOrder []string
	seenGroup := map[string]bool{}
	for _, k := range order {
		if !seenGroup[k.group] {
			seenGroup[k.group] = true
			groupOrder = append(groupOrder, k.group)
		}
		row := htmlMetricRow{Name: k.name, Level: "neutral"}
		for _, runID := range runIDs {
			card, ok := values[k][runID]
			cell := htmlMetricCell{
				RunID: runID,
				Label: firstHTML(labels[runID], runID),
				Value: "unavailable",
				Level: "warn",
			}
			if ok {
				cell.Value = card.Value
				cell.Level = normalizeLevel(card.Level)
			}
			row.Cells = append(row.Cells, cell)
			row.Level = strongestHTMLLevel(row.Level, cell.Level)
		}
		byGroup[k.group] = append(byGroup[k.group], row)
	}

	out := make([]htmlMetricGroup, 0, len(groupOrder))
	for _, group := range groupOrder {
		out = append(out, htmlMetricGroup{Group: group, Rows: byGroup[group]})
	}
	return out
}

func buildHTMLPathGraph(graph *PathGraph) htmlPathGraph {
	if graph == nil || len(graph.Nodes) == 0 {
		return htmlPathGraph{}
	}
	const (
		nodeWidth  = 240
		nodeHeight = 88
		gap        = 80
		top        = 52
	)
	width := 64 + len(graph.Nodes)*(nodeWidth+gap)
	if width < 640 {
		width = 640
	}
	nodePos := map[string]htmlPathNode{}
	nodes := make([]htmlPathNode, 0, len(graph.Nodes))
	for i, node := range graph.Nodes {
		runIDs := strings.Join(node.RunIDs, ", ")
		view := htmlPathNode{
			ID:          node.ID,
			Label:       node.Label,
			ShortLabel:  clipText(node.Label, 32),
			Kind:        node.Kind,
			ShortKind:   clipText(node.Kind, 24),
			RunIDs:      runIDs,
			ShortRunIDs: clipText(runIDs, 30),
			Title:       graphNodeTitle(node.Kind, node.Label, runIDs),
			X:           32 + i*(nodeWidth+gap),
			Y:           top,
			KindCls:     classToken(node.Kind),
			RunCount:    len(node.RunIDs),
		}
		nodes = append(nodes, view)
		nodePos[node.ID] = view
	}

	edges := make([]htmlPathEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		from, okFrom := nodePos[edge.From]
		to, okTo := nodePos[edge.To]
		if !okFrom || !okTo {
			continue
		}
		edges = append(edges, htmlPathEdge{
			From:   edge.From,
			To:     edge.To,
			RunIDs: strings.Join(edge.RunIDs, ", "),
			X1:     from.X + nodeWidth,
			Y1:     from.Y + nodeHeight/2,
			X2:     to.X,
			Y2:     to.Y + nodeHeight/2,
		})
	}
	return htmlPathGraph{Width: width, Height: 190, NodeWidth: nodeWidth, NodeHeight: nodeHeight, Nodes: nodes, Edges: edges}
}

func graphNodeTitle(kind, label, runIDs string) string {
	parts := []string{}
	if strings.TrimSpace(kind) != "" {
		parts = append(parts, "kind: "+kind)
	}
	if strings.TrimSpace(label) != "" {
		parts = append(parts, "summary: "+label)
	}
	if strings.TrimSpace(runIDs) != "" {
		parts = append(parts, "runs: "+runIDs)
	}
	return strings.Join(parts, "\n")
}

func clipText(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func summaryPairs(summary map[string]any) []htmlPair {
	if len(summary) == 0 {
		return nil
	}
	keys := make([]string, 0, len(summary))
	for key := range summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]htmlPair, 0, len(keys))
	for _, key := range keys {
		out = append(out, htmlPair{Key: key, Value: formatAny(summary[key])})
	}
	return out
}

func attributesPairs(attrs map[string]string) []htmlPair {
	if len(attrs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]htmlPair, 0, len(keys))
	for _, key := range keys {
		out = append(out, htmlPair{Key: key, Value: attrs[key]})
	}
	return out
}

func collectReportEventIDs(report *VisualReport) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(values []string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			ids = append(ids, value)
		}
	}
	for _, lane := range report.Lanes {
		for _, step := range lane.Steps {
			add(step.EventIDs)
		}
	}
	for _, marker := range report.Divergences {
		add(marker.EventIDs)
	}
	sort.Strings(ids)
	return ids
}

func formatAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []string:
		return strings.Join(typed, ", ")
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return formatTime(*value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func stepNumber(step int) string {
	if step <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", step)
}

func fileUseSummary(use FileUse) string {
	parts := []string{}
	if use.ReadCount > 0 {
		parts = append(parts, fmt.Sprintf("read %d", use.ReadCount))
	}
	if use.EditCount > 0 {
		parts = append(parts, fmt.Sprintf("edit %d", use.EditCount))
	}
	if len(parts) == 0 && (use.FirstAction != "" || use.LastAction != "") {
		parts = append(parts, "other")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
}

func fileUseLevel(use FileUse) string {
	if use.EditCount > 0 {
		return "bad"
	}
	if use.ReadCount > 0 {
		return "good"
	}
	if use.FirstAction != "" || use.LastAction != "" {
		return "neutral"
	}
	return "muted"
}

func searchScopeLevel(use SearchScopeUse) string {
	if use.SearchCount == 0 {
		return "muted"
	}
	return "info"
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "good", "bad", "warn", "warning", "neutral", "info", "muted":
		if strings.EqualFold(level, "warning") {
			return "warn"
		}
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "neutral"
	}
}

func strongestHTMLLevel(left, right string) string {
	order := map[string]int{
		"muted":   0,
		"neutral": 1,
		"info":    1,
		"good":    1,
		"warn":    2,
		"bad":     3,
	}
	if order[normalizeLevel(right)] > order[normalizeLevel(left)] {
		return normalizeLevel(right)
	}
	return normalizeLevel(left)
}

func classToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func firstHTML(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
