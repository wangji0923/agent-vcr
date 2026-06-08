package visualize

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRenderHTMLSingleRunIncludesTimelineFileAccessAndMetrics(t *testing.T) {
	report := singleHTMLReport()

	data, err := RenderHTML(report, HTMLOptions{Title: "Single Run Visual Report"})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	assertContainsAll(t, html,
		"Single Run Visual Report",
		"Behavior Timeline",
		"File Access Compare",
		"Metrics Cards",
		"read src/app.go",
		"go test ./...",
	)
	if strings.Contains(html, "<h2>Swimlane Timeline</h2>") {
		t.Fatalf("single run without explicit alignment should render behavior timeline, got swimlane")
	}
}

func TestRenderHTMLTwoRunIncludesSwimlaneAndFirstDivergence(t *testing.T) {
	report := twoRunHTMLReport()

	data, err := RenderHTML(report, HTMLOptions{Title: "Compare Report"})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	assertContainsAll(t, html,
		"Compare Report",
		"Swimlane Timeline",
		"First Divergence",
		"Auxiliary Path Graph",
		"Run A inspected tests while Run B entered legacy code",
		"row-divergent",
		"src/legacy_cookie.go",
	)
}

func TestRenderHTMLIncludesSearchScopeCompare(t *testing.T) {
	report := twoRunHTMLReport()
	report.SearchScopes = SearchScopeCompare{Rows: []SearchScopeRow{
		{Scope: "src", Runs: map[string]SearchScopeUse{
			"run-a": {SearchCount: 1, FirstStep: 1, LastStep: 1, Queries: []string{`rg "session" src tests`}},
			"run-b": {SearchCount: 1, FirstStep: 1, LastStep: 1, Queries: []string{`rg "cookie" src`}},
		}},
		{Scope: "tests", Runs: map[string]SearchScopeUse{
			"run-a": {SearchCount: 1, FirstStep: 1, LastStep: 1, Queries: []string{`rg "session" src tests`}},
			"run-b": {},
		}},
	}}

	data, err := RenderHTML(report, HTMLOptions{})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	assertContainsAll(t, html,
		"Search Scope Compare",
		`rg &#34;session&#34; src tests`,
		`rg &#34;cookie&#34; src`,
	)
}

func TestRenderHTMLMultiRunIncludesAllLanes(t *testing.T) {
	report := sampleVisualReport()

	data, err := RenderHTML(report, HTMLOptions{})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	assertContainsAll(t, html,
		"test-first",
		"legacy",
		"source-first",
		"First Divergence Per Compared Run",
		"run-a",
		"run-b",
		"run-c",
	)
}

func TestRenderHTMLEscapesUserContent(t *testing.T) {
	report := singleHTMLReport()
	report.Lanes[0].Steps[0].Summary = `<script>alert("x")</script>`
	report.Lanes[0].Steps[0].Command = `echo "<script>"`

	data, err := RenderHTML(report, HTMLOptions{})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	if strings.Contains(html, `<script>alert("x")</script>`) || strings.Contains(html, `echo "<script>"`) {
		t.Fatalf("HTML leaked unescaped script content:\n%s", html)
	}
	assertContainsAll(t, html, "&lt;script&gt;alert", "echo &#34;&lt;script&gt;&#34;")
}

func TestRenderHTMLAppliesRedaction(t *testing.T) {
	report := singleHTMLReport()
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	report.Lanes[0].Steps[0].Command = "OPENAI_API_KEY=" + secret
	report.Lanes[0].Steps[0].Files = []string{"config/.env.local"}
	report.FileAccess = BuildFileAccessCompare(report.Lanes)

	data, err := RenderHTML(report, HTMLOptions{Redacted: true})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	if strings.Contains(html, secret) || strings.Contains(html, ".env.local") {
		t.Fatalf("redacted HTML leaked secret content:\n%s", html)
	}
	assertContainsAll(t, html, "[REDACTED:openai_api_key]", "[REDACTED:env_path]")
}

func TestRenderHTMLMissingDivergenceDoesNotPanic(t *testing.T) {
	report := twoRunHTMLReport()
	report.Divergences = nil
	report.Summary.FirstDivergence = nil
	report.Summary.DivergenceCount = 0
	for i := range report.Alignment {
		report.Alignment[i].IsDivergent = false
		report.Alignment[i].Reason = ""
	}

	data, err := RenderHTML(report, HTMLOptions{})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := string(data)
	if strings.Contains(html, "First Divergence") {
		t.Fatalf("unexpected first divergence section:\n%s", html)
	}
	assertContainsAll(t, html, "Swimlane Timeline", "File Access Compare")
}

func singleHTMLReport() *VisualReport {
	start := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	read := VisualStep{
		RunID:       "run-a",
		StepID:      "run-a-step-1",
		Index:       1,
		Kind:        VisualStepReadFile,
		Phase:       VisualPhaseInspection,
		Summary:     "read src/app.go",
		Files:       []string{"src/app.go"},
		EventIDs:    []string{"evt-read"},
		Significant: true,
	}
	edit := VisualStep{
		RunID:       "run-a",
		StepID:      "run-a-step-2",
		Index:       2,
		Kind:        VisualStepEditFile,
		Phase:       VisualPhaseEditing,
		Summary:     "edit src/app.go",
		Files:       []string{"src/app.go"},
		EventIDs:    []string{"evt-edit"},
		Significant: true,
	}
	test := VisualStep{
		RunID:       "run-a",
		StepID:      "run-a-step-3",
		Index:       3,
		Kind:        VisualStepRunTest,
		Phase:       VisualPhaseValidation,
		Summary:     "go test ./...",
		Command:     "go test ./...",
		EventIDs:    []string{"evt-test"},
		Significant: true,
	}
	lane := BehaviorLane{RunID: "run-a", Label: "baseline", Steps: []VisualStep{read, edit, test}}
	fileAccess := BuildFileAccessCompare([]BehaviorLane{lane})
	metrics := []MetricsCardGroup{{
		RunID: "run-a",
		Label: "baseline",
		Cards: []MetricsCard{
			{Group: "Validation behavior", Name: "Ran tests after edit", Value: "yes", Level: "good"},
			{Group: "Edit scope", Name: "Files edited", Value: "1", Level: "info"},
		},
	}}
	return &VisualReport{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   start,
		Mode:          RenderModeSingle,
		Options:       VisualOptions{Mode: RenderModeSingle, BaselineRunID: "run-a", MaxRuns: MaxRecommendedRuns},
		Summary: VisualSummary{
			RunCount:             1,
			StepCount:            3,
			SignificantStepCount: 3,
			FileCount:            len(fileAccess.Rows),
			MetricsCardCount:     2,
			Mode:                 RenderModeSingle,
		},
		Runs: []VisualRun{{
			RunID:     "run-a",
			Label:     "baseline",
			Source:    "codex-hooks",
			Status:    "completed",
			StartedAt: &start,
			StepCount: 3,
			Summary:   map[string]any{"cache_hit": true},
		}},
		Lanes:      []BehaviorLane{lane},
		FileAccess: fileAccess,
		Metrics:    metrics,
		PathGraph:  BuildPathGraph([]BehaviorLane{lane}),
	}
}

func twoRunHTMLReport() *VisualReport {
	report := cloneVisualReport(sampleVisualReport())
	report.Runs = report.Runs[:2]
	report.Lanes = report.Lanes[:2]
	report.Metrics = report.Metrics[:2]
	report.PathGraph = BuildPathGraph(report.Lanes)
	report.SearchScopes = SearchScopeCompare{}
	report.Summary.RunCount = 2
	report.Summary.StepCount = 4
	report.Summary.FileCount = len(report.FileAccess.Rows)
	report.Summary.MetricsCardCount = countMetricCards(report.Metrics)
	report.Options.Labels = map[string]string{"run-a": "test-first", "run-b": "legacy"}

	for i := range report.Alignment {
		report.Alignment[i].Cells = filterCells(report.Alignment[i].Cells, map[string]bool{"run-a": true, "run-b": true})
	}
	for i := range report.FileAccess.Rows {
		report.FileAccess.Rows[i].Runs = filterFileUses(report.FileAccess.Rows[i].Runs, map[string]bool{"run-a": true, "run-b": true})
	}
	return report
}

func cloneVisualReport(report *VisualReport) *VisualReport {
	data, err := json.Marshal(report)
	if err != nil {
		panic(err)
	}
	var cloned VisualReport
	if err := json.Unmarshal(data, &cloned); err != nil {
		panic(err)
	}
	return &cloned
}

func filterCells(input map[string]StepCell, keep map[string]bool) map[string]StepCell {
	out := map[string]StepCell{}
	for runID, cell := range input {
		if keep[runID] {
			out[runID] = cell
		}
	}
	return out
}

func filterFileUses(input map[string]FileUse, keep map[string]bool) map[string]FileUse {
	out := map[string]FileUse{}
	for runID, use := range input {
		if keep[runID] {
			out[runID] = use
		}
	}
	return out
}

func assertContainsAll(t *testing.T, value string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			t.Fatalf("expected HTML to contain %q:\n%s", needle, value)
		}
	}
}
