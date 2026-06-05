package behavior

type MetricName string

const (
	MetricReadTestsBeforeEdit    MetricName = "read_tests_before_edit"
	MetricLegacyPathTouched      MetricName = "legacy_path_touched"
	MetricUniqueFilesRead        MetricName = "unique_files_read"
	MetricRepeatedReads          MetricName = "repeated_reads"
	MetricRanTestsAfterEdit      MetricName = "ran_tests_after_edit"
	MetricRanAnyTests            MetricName = "ran_any_tests"
	MetricFailedTestRuns         MetricName = "failed_test_runs"
	MetricIgnoredFailedCommand   MetricName = "ignored_failed_command"
	MetricFilesEdited            MetricName = "files_edited"
	MetricSourceFilesEdited      MetricName = "source_files_edited"
	MetricTestFilesEdited        MetricName = "test_files_edited"
	MetricTotalSteps             MetricName = "total_steps"
	MetricToolCalls              MetricName = "tool_calls"
	MetricSearchSteps            MetricName = "search_steps"
	MetricFailedCommands         MetricName = "failed_commands"
	MetricRecoveredAfterFailure  MetricName = "recovered_after_failure"
	MetricReranTestsAfterFailure MetricName = "reran_tests_after_failure"
)

type Metrics struct {
	ContextDiscipline ContextDisciplineMetrics `json:"context_discipline"`
	Validation        ValidationMetrics        `json:"validation"`
	EditScope         EditScopeMetrics         `json:"edit_scope"`
	ToolEfficiency    ToolEfficiencyMetrics    `json:"tool_efficiency"`
	Recovery          RecoveryMetrics          `json:"recovery"`
}

type ContextDisciplineMetrics struct {
	ReadTestsBeforeEdit bool `json:"read_tests_before_edit"`
	LegacyPathTouched   bool `json:"legacy_path_touched"`
	UniqueFilesRead     int  `json:"unique_files_read"`
	RepeatedReads       int  `json:"repeated_reads"`
}

type ValidationMetrics struct {
	RanTestsAfterEdit    bool `json:"ran_tests_after_edit"`
	RanAnyTests          bool `json:"ran_any_tests"`
	FailedTestRuns       int  `json:"failed_test_runs"`
	IgnoredFailedCommand bool `json:"ignored_failed_command"`
}

type EditScopeMetrics struct {
	FilesEdited           int     `json:"files_edited"`
	SourceFilesEdited     int     `json:"source_files_edited"`
	TestFilesEdited       int     `json:"test_files_edited"`
	SourceToTestEditRatio float64 `json:"source_to_test_edit_ratio"`
}

type ToolEfficiencyMetrics struct {
	TotalSteps     int `json:"total_steps"`
	ToolCalls      int `json:"tool_calls"`
	SearchSteps    int `json:"search_steps"`
	FailedCommands int `json:"failed_commands"`
}

type RecoveryMetrics struct {
	RecoveredAfterFailure  bool `json:"recovered_after_failure"`
	ReranTestsAfterFailure bool `json:"reran_tests_after_failure"`
}
