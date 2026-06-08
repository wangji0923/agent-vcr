package visualize

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

func TestBuildMetricsCardsCoversGroupsAndLevels(t *testing.T) {
	cards := BuildMetricsCards("run-a", riskyMetrics())

	if cards.RunID != "run-a" {
		t.Fatalf("run id = %q", cards.RunID)
	}
	if len(cards.Cards) != 22 {
		t.Fatalf("cards = %d, want 22", len(cards.Cards))
	}

	for _, group := range []string{
		metricsGroupContextDiscipline,
		metricsGroupValidation,
		metricsGroupEditScope,
		metricsGroupToolEfficiency,
		metricsGroupRecovery,
	} {
		if !hasMetricsGroup(cards, group) {
			t.Fatalf("missing group %q in %#v", group, cards.Cards)
		}
	}

	assertMetricCard(t, cards, metricsGroupContextDiscipline, "Legacy path touched", "yes", metricsLevelWarn)
	assertMetricCard(t, cards, metricsGroupValidation, "Ran tests after edit", "no", metricsLevelBad)
	assertMetricCard(t, cards, metricsGroupValidation, "Skipped validation", "yes", metricsLevelBad)
	assertMetricCard(t, cards, metricsGroupToolEfficiency, "Shell commands", "unavailable", metricsLevelInfo)
	assertMetricCard(t, cards, metricsGroupToolEfficiency, "Repeated commands", "unavailable", metricsLevelInfo)
	assertMetricCard(t, cards, metricsGroupRecovery, "Repeated failures", "yes", metricsLevelWarn)
}

func TestBuildMetricsCardsGoodPathLevels(t *testing.T) {
	cards := BuildMetricsCards("run-a", behavior.Metrics{
		ContextDiscipline: behavior.ContextDisciplineMetrics{
			ReadTestsBeforeEdit: true,
			UniqueFilesRead:     4,
		},
		Validation: behavior.ValidationMetrics{
			RanTestsAfterEdit: true,
			RanAnyTests:       true,
		},
		EditScope: behavior.EditScopeMetrics{
			FilesEdited:           2,
			SourceFilesEdited:     1,
			TestFilesEdited:       1,
			SourceToTestEditRatio: 1,
		},
		ToolEfficiency: behavior.ToolEfficiencyMetrics{
			TotalSteps:  8,
			ToolCalls:   5,
			SearchSteps: 2,
		},
		Recovery: behavior.RecoveryMetrics{
			RecoveredAfterFailure: false,
		},
	})

	assertMetricCard(t, cards, metricsGroupContextDiscipline, "Read tests before edit", "yes", metricsLevelGood)
	assertMetricCard(t, cards, metricsGroupValidation, "Ran tests after edit", "yes", metricsLevelGood)
	assertMetricCard(t, cards, metricsGroupValidation, "Skipped validation", "no", metricsLevelGood)
	assertMetricCard(t, cards, metricsGroupToolEfficiency, "Failed commands", "0", metricsLevelGood)
	assertMetricCard(t, cards, metricsGroupRecovery, "Recovered after failure", "no", metricsLevelInfo)
}

func TestBuildMetricsCardGroupsUnavailable(t *testing.T) {
	groups := BuildMetricsCardGroups([]VisualRun{{RunID: "run-a", Label: "baseline"}}, nil)

	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	group := groups[0]
	if group.RunID != "run-a" || group.Label != "baseline" {
		t.Fatalf("group identity = %#v", group)
	}
	if len(group.Cards) != 5 {
		t.Fatalf("unavailable cards = %d, want 5", len(group.Cards))
	}
	for _, card := range group.Cards {
		if card.Name != "Metrics unavailable" || card.Value != "unavailable" || card.Level != metricsLevelWarn {
			t.Fatalf("unavailable card = %#v", card)
		}
	}
}

func TestCompareMetricsCardsSupportsThreeRuns(t *testing.T) {
	good := behavior.Metrics{
		ContextDiscipline: behavior.ContextDisciplineMetrics{ReadTestsBeforeEdit: true},
		Validation:        behavior.ValidationMetrics{RanTestsAfterEdit: true, RanAnyTests: true},
		EditScope:         behavior.EditScopeMetrics{FilesEdited: 1, SourceFilesEdited: 1, TestFilesEdited: 1, SourceToTestEditRatio: 1},
	}
	bad := riskyMetrics()
	groups := BuildMetricsCardGroups(
		[]VisualRun{
			{RunID: "run-a", Label: "baseline"},
			{RunID: "run-b", Label: "missing"},
			{RunID: "run-c", Label: "variant"},
		},
		map[string]*behavior.Metrics{
			"run-a": &good,
			"run-c": &bad,
		},
	)

	cards := CompareMetricsCards(groups)
	if len(cards) != 22 {
		t.Fatalf("comparison cards = %d, want 22", len(cards))
	}

	card := findRequiredComparisonCard(t, cards, metricsGroupValidation, "Ran tests after edit")
	wantValue := "baseline=yes | missing=unavailable | variant=no"
	if card.Value != wantValue {
		t.Fatalf("comparison value = %q, want %q", card.Value, wantValue)
	}
	if card.Level != metricsLevelBad {
		t.Fatalf("comparison level = %q, want %q", card.Level, metricsLevelBad)
	}
}

func TestMetricsCardsJSONRoundTrip(t *testing.T) {
	group := BuildMetricsCardsWithLabel("run-a", "baseline", riskyMetrics())

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded MetricsCardGroup
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.RunID != "run-a" || decoded.Label != "baseline" {
		t.Fatalf("decoded identity = %#v", decoded)
	}
	assertMetricCard(t, decoded, metricsGroupEditScope, "Source/test edit ratio", "2.50", metricsLevelWarn)
}

func TestCompareMetricsCardsEmptyInput(t *testing.T) {
	if cards := CompareMetricsCards(nil); cards != nil {
		t.Fatalf("cards = %#v, want nil", cards)
	}
}

func riskyMetrics() behavior.Metrics {
	return behavior.Metrics{
		ContextDiscipline: behavior.ContextDisciplineMetrics{
			ReadTestsBeforeEdit: false,
			LegacyPathTouched:   true,
			UniqueFilesRead:     0,
			RepeatedReads:       2,
		},
		Validation: behavior.ValidationMetrics{
			RanTestsAfterEdit:    false,
			RanAnyTests:          false,
			FailedTestRuns:       1,
			IgnoredFailedCommand: true,
		},
		EditScope: behavior.EditScopeMetrics{
			FilesEdited:           3,
			SourceFilesEdited:     2,
			TestFilesEdited:       0,
			SourceToTestEditRatio: 2.5,
		},
		ToolEfficiency: behavior.ToolEfficiencyMetrics{
			TotalSteps:     12,
			ToolCalls:      7,
			SearchSteps:    1,
			FailedCommands: 2,
		},
		Recovery: behavior.RecoveryMetrics{
			RecoveredAfterFailure:  false,
			ReranTestsAfterFailure: false,
		},
	}
}

func hasMetricsGroup(group MetricsCardGroup, name string) bool {
	for _, card := range group.Cards {
		if card.Group == name {
			return true
		}
	}
	return false
}

func assertMetricCard(t *testing.T, group MetricsCardGroup, cardGroup, name, value, level string) {
	t.Helper()
	card, ok := findMetricCard(group, cardGroup, name)
	if !ok {
		t.Fatalf("missing card %q/%q", cardGroup, name)
	}
	if card.Value != value || card.Level != level {
		t.Fatalf("card %q/%q = value %q level %q, want value %q level %q", cardGroup, name, card.Value, card.Level, value, level)
	}
}

func findRequiredComparisonCard(t *testing.T, cards []MetricsCard, cardGroup, name string) MetricsCard {
	t.Helper()
	for _, card := range cards {
		if card.Group == cardGroup && card.Name == name {
			return card
		}
	}
	var names []string
	for _, card := range cards {
		names = append(names, card.Group+"/"+card.Name)
	}
	t.Fatalf("missing comparison card %q/%q in %s", cardGroup, name, strings.Join(names, ", "))
	return MetricsCard{}
}
