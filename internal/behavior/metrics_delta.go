package behavior

type DeltaValue struct {
	Before  any     `json:"before"`
	After   any     `json:"after"`
	Changed bool    `json:"changed"`
	Delta   float64 `json:"delta,omitempty"`
}

type MetricsDelta struct {
	ContextDiscipline map[string]DeltaValue `json:"context_discipline"`
	Validation        map[string]DeltaValue `json:"validation"`
	EditScope         map[string]DeltaValue `json:"edit_scope"`
	ToolEfficiency    map[string]DeltaValue `json:"tool_efficiency"`
	Recovery          map[string]DeltaValue `json:"recovery"`
}

func DiffMetrics(before, after Metrics) MetricsDelta {
	return MetricsDelta{
		ContextDiscipline: map[string]DeltaValue{
			string(MetricReadTestsBeforeEdit): boolDelta(before.ContextDiscipline.ReadTestsBeforeEdit, after.ContextDiscipline.ReadTestsBeforeEdit),
			string(MetricLegacyPathTouched):   boolDelta(before.ContextDiscipline.LegacyPathTouched, after.ContextDiscipline.LegacyPathTouched),
			string(MetricUniqueFilesRead):     intDelta(before.ContextDiscipline.UniqueFilesRead, after.ContextDiscipline.UniqueFilesRead),
			string(MetricRepeatedReads):       intDelta(before.ContextDiscipline.RepeatedReads, after.ContextDiscipline.RepeatedReads),
		},
		Validation: map[string]DeltaValue{
			string(MetricRanTestsAfterEdit):    boolDelta(before.Validation.RanTestsAfterEdit, after.Validation.RanTestsAfterEdit),
			string(MetricRanAnyTests):          boolDelta(before.Validation.RanAnyTests, after.Validation.RanAnyTests),
			string(MetricFailedTestRuns):       intDelta(before.Validation.FailedTestRuns, after.Validation.FailedTestRuns),
			string(MetricIgnoredFailedCommand): boolDelta(before.Validation.IgnoredFailedCommand, after.Validation.IgnoredFailedCommand),
		},
		EditScope: map[string]DeltaValue{
			string(MetricFilesEdited):       intDelta(before.EditScope.FilesEdited, after.EditScope.FilesEdited),
			string(MetricSourceFilesEdited): intDelta(before.EditScope.SourceFilesEdited, after.EditScope.SourceFilesEdited),
			string(MetricTestFilesEdited):   intDelta(before.EditScope.TestFilesEdited, after.EditScope.TestFilesEdited),
			"source_to_test_edit_ratio":     floatDelta(before.EditScope.SourceToTestEditRatio, after.EditScope.SourceToTestEditRatio),
		},
		ToolEfficiency: map[string]DeltaValue{
			string(MetricTotalSteps):     intDelta(before.ToolEfficiency.TotalSteps, after.ToolEfficiency.TotalSteps),
			string(MetricToolCalls):      intDelta(before.ToolEfficiency.ToolCalls, after.ToolEfficiency.ToolCalls),
			string(MetricSearchSteps):    intDelta(before.ToolEfficiency.SearchSteps, after.ToolEfficiency.SearchSteps),
			string(MetricFailedCommands): intDelta(before.ToolEfficiency.FailedCommands, after.ToolEfficiency.FailedCommands),
		},
		Recovery: map[string]DeltaValue{
			string(MetricRecoveredAfterFailure):  boolDelta(before.Recovery.RecoveredAfterFailure, after.Recovery.RecoveredAfterFailure),
			string(MetricReranTestsAfterFailure): boolDelta(before.Recovery.ReranTestsAfterFailure, after.Recovery.ReranTestsAfterFailure),
		},
	}
}

func boolDelta(before, after bool) DeltaValue {
	return DeltaValue{Before: before, After: after, Changed: before != after}
}

func intDelta(before, after int) DeltaValue {
	return DeltaValue{
		Before:  before,
		After:   after,
		Changed: before != after,
		Delta:   float64(after - before),
	}
}

func floatDelta(before, after float64) DeltaValue {
	return DeltaValue{
		Before:  before,
		After:   after,
		Changed: before != after,
		Delta:   after - before,
	}
}
