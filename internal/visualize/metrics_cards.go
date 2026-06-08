package visualize

import (
	"fmt"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

const (
	metricsGroupContextDiscipline = "Context discipline"
	metricsGroupValidation        = "Validation behavior"
	metricsGroupEditScope         = "Edit scope"
	metricsGroupToolEfficiency    = "Tool efficiency"
	metricsGroupRecovery          = "Recovery behavior"

	metricsLevelGood = "good"
	metricsLevelWarn = "warn"
	metricsLevelBad  = "bad"
	metricsLevelInfo = "info"
)

type metricCardDefinition struct {
	group string
	name  string
	value func(behavior.Metrics) string
	level func(behavior.Metrics) string
}

var metricCardDefinitions = []metricCardDefinition{
	{
		group: metricsGroupContextDiscipline,
		name:  "Read tests before edit",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.ContextDiscipline.ReadTestsBeforeEdit)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.ContextDiscipline.ReadTestsBeforeEdit {
				return metricsLevelGood
			}
			if metrics.EditScope.SourceFilesEdited > 0 {
				return metricsLevelWarn
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupContextDiscipline,
		name:  "Legacy path touched",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.ContextDiscipline.LegacyPathTouched)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.ContextDiscipline.LegacyPathTouched {
				return metricsLevelWarn
			}
			return metricsLevelGood
		},
	},
	{
		group: metricsGroupContextDiscipline,
		name:  "Files read",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.ContextDiscipline.UniqueFilesRead)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.ContextDiscipline.UniqueFilesRead == 0 && metrics.EditScope.SourceFilesEdited > 0 {
				return metricsLevelWarn
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupContextDiscipline,
		name:  "Repeated reads",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.ContextDiscipline.RepeatedReads)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.ContextDiscipline.RepeatedReads > 0 {
				return metricsLevelWarn
			}
			return metricsLevelGood
		},
	},
	{
		group: metricsGroupValidation,
		name:  "Ran tests after edit",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.Validation.RanTestsAfterEdit)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.Validation.RanTestsAfterEdit {
				return metricsLevelGood
			}
			if metrics.EditScope.SourceFilesEdited > 0 {
				return metricsLevelBad
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupValidation,
		name:  "Ran any tests",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.Validation.RanAnyTests)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.Validation.RanAnyTests {
				return metricsLevelGood
			}
			if metrics.EditScope.SourceFilesEdited > 0 {
				return metricsLevelBad
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupValidation,
		name:  "Failed test runs",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.Validation.FailedTestRuns)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.Validation.FailedTestRuns > 0 {
				return metricsLevelWarn
			}
			return metricsLevelGood
		},
	},
	{
		group: metricsGroupValidation,
		name:  "Skipped validation",
		value: func(metrics behavior.Metrics) string {
			return boolValue(skippedValidation(metrics))
		},
		level: func(metrics behavior.Metrics) string {
			if skippedValidation(metrics) {
				return metricsLevelBad
			}
			return metricsLevelGood
		},
	},
	{
		group: metricsGroupValidation,
		name:  "Ignored failed command",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.Validation.IgnoredFailedCommand)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.Validation.IgnoredFailedCommand {
				return metricsLevelBad
			}
			return metricsLevelGood
		},
	},
	{
		group: metricsGroupEditScope,
		name:  "Files edited",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.EditScope.FilesEdited)
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupEditScope,
		name:  "Source files edited",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.EditScope.SourceFilesEdited)
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupEditScope,
		name:  "Test files edited",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.EditScope.TestFilesEdited)
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupEditScope,
		name:  "Source/test edit ratio",
		value: func(metrics behavior.Metrics) string {
			return ratioValue(metrics.EditScope.SourceToTestEditRatio)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.EditScope.SourceFilesEdited > 0 && metrics.EditScope.TestFilesEdited == 0 {
				return metricsLevelWarn
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupToolEfficiency,
		name:  "Tool calls",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.ToolEfficiency.ToolCalls)
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupToolEfficiency,
		name:  "Shell commands",
		value: func(behavior.Metrics) string {
			return "unavailable"
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupToolEfficiency,
		name:  "Search steps",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.ToolEfficiency.SearchSteps)
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupToolEfficiency,
		name:  "Total steps",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.ToolEfficiency.TotalSteps)
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupToolEfficiency,
		name:  "Failed commands",
		value: func(metrics behavior.Metrics) string {
			return intValue(metrics.ToolEfficiency.FailedCommands)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.ToolEfficiency.FailedCommands > 0 {
				return metricsLevelWarn
			}
			return metricsLevelGood
		},
	},
	{
		group: metricsGroupToolEfficiency,
		name:  "Repeated commands",
		value: func(behavior.Metrics) string {
			return "unavailable"
		},
		level: func(behavior.Metrics) string {
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupRecovery,
		name:  "Recovered after failure",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.Recovery.RecoveredAfterFailure)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.Recovery.RecoveredAfterFailure {
				return metricsLevelGood
			}
			if hasFailure(metrics) {
				return metricsLevelWarn
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupRecovery,
		name:  "Reran tests after failure",
		value: func(metrics behavior.Metrics) string {
			return boolValue(metrics.Recovery.ReranTestsAfterFailure)
		},
		level: func(metrics behavior.Metrics) string {
			if metrics.Recovery.ReranTestsAfterFailure {
				return metricsLevelGood
			}
			if metrics.Validation.FailedTestRuns > 0 {
				return metricsLevelWarn
			}
			return metricsLevelInfo
		},
	},
	{
		group: metricsGroupRecovery,
		name:  "Repeated failures",
		value: func(metrics behavior.Metrics) string {
			return boolValue(repeatedFailures(metrics))
		},
		level: func(metrics behavior.Metrics) string {
			if repeatedFailures(metrics) {
				return metricsLevelWarn
			}
			if hasFailure(metrics) {
				return metricsLevelInfo
			}
			return metricsLevelGood
		},
	},
}

func BuildMetricsCards(runID string, metrics behavior.Metrics) RunMetricsCards {
	return BuildMetricsCardsWithLabel(runID, "", metrics)
}

func BuildMetricsCardsWithLabel(runID, label string, metrics behavior.Metrics) RunMetricsCards {
	cards := make([]MetricsCard, 0, len(metricCardDefinitions))
	for _, definition := range metricCardDefinitions {
		cards = append(cards, MetricsCard{
			Group: definition.group,
			Name:  definition.name,
			Value: definition.value(metrics),
			Level: definition.level(metrics),
		})
	}
	return RunMetricsCards{RunID: runID, Label: label, Cards: cards}
}

func BuildMetricsCardGroups(runs []VisualRun, metricsByRun map[string]*behavior.Metrics) []MetricsCardGroup {
	groups := make([]MetricsCardGroup, 0, len(runs))
	for _, run := range runs {
		metrics := metricsByRun[run.RunID]
		if metrics == nil {
			groups = append(groups, buildUnavailableMetricsCards(run.RunID, run.Label))
			continue
		}
		groups = append(groups, BuildMetricsCardsWithLabel(run.RunID, run.Label, *metrics))
	}
	return groups
}

func CompareMetricsCards(groups []RunMetricsCards) []MetricsCard {
	if len(groups) == 0 {
		return nil
	}

	cards := make([]MetricsCard, 0, len(metricCardDefinitions))
	for _, definition := range metricCardDefinitions {
		values := make([]string, 0, len(groups))
		level := metricsLevelInfo
		for _, group := range groups {
			card, ok := findMetricCard(group, definition.group, definition.name)
			value := "unavailable"
			if ok {
				value = card.Value
				level = strongestMetricLevel(level, card.Level)
			}
			values = append(values, fmt.Sprintf("%s=%s", metricRunLabel(group), value))
		}
		cards = append(cards, MetricsCard{
			Group: definition.group,
			Name:  definition.name,
			Value: strings.Join(values, " | "),
			Level: level,
		})
	}
	return cards
}

func buildUnavailableMetricsCards(runID, label string) MetricsCardGroup {
	groups := []string{
		metricsGroupContextDiscipline,
		metricsGroupValidation,
		metricsGroupEditScope,
		metricsGroupToolEfficiency,
		metricsGroupRecovery,
	}
	cards := make([]MetricsCard, 0, len(groups))
	for _, group := range groups {
		cards = append(cards, MetricsCard{
			Group: group,
			Name:  "Metrics unavailable",
			Value: "unavailable",
			Level: metricsLevelWarn,
		})
	}
	return MetricsCardGroup{RunID: runID, Label: label, Cards: cards}
}

func findMetricCard(group RunMetricsCards, cardGroup, name string) (MetricsCard, bool) {
	for _, card := range group.Cards {
		if card.Group == cardGroup && card.Name == name {
			return card, true
		}
	}
	return MetricsCard{}, false
}

func metricRunLabel(group RunMetricsCards) string {
	if strings.TrimSpace(group.Label) != "" {
		return group.Label
	}
	if strings.TrimSpace(group.RunID) != "" {
		return group.RunID
	}
	return "run"
}

func boolValue(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func intValue(value int) string {
	return fmt.Sprintf("%d", value)
}

func ratioValue(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func skippedValidation(metrics behavior.Metrics) bool {
	return metrics.EditScope.SourceFilesEdited > 0 && !metrics.Validation.RanAnyTests
}

func hasFailure(metrics behavior.Metrics) bool {
	return metrics.ToolEfficiency.FailedCommands > 0 || metrics.Validation.FailedTestRuns > 0
}

func repeatedFailures(metrics behavior.Metrics) bool {
	failures := metrics.ToolEfficiency.FailedCommands
	if metrics.Validation.FailedTestRuns > failures {
		failures = metrics.Validation.FailedTestRuns
	}
	return failures > 1
}

func strongestMetricLevel(left, right string) string {
	order := map[string]int{
		"":               0,
		metricsLevelInfo: 1,
		metricsLevelGood: 1,
		metricsLevelWarn: 2,
		metricsLevelBad:  3,
	}
	if order[right] > order[left] {
		return right
	}
	return left
}
