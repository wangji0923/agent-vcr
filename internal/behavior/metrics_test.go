package behavior

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMetricsJSONRoundTrip(t *testing.T) {
	metrics := Metrics{
		ContextDiscipline: ContextDisciplineMetrics{
			ReadTestsBeforeEdit: true,
			LegacyPathTouched:   true,
			UniqueFilesRead:     4,
			RepeatedReads:       1,
		},
		Validation: ValidationMetrics{
			RanTestsAfterEdit:    true,
			RanAnyTests:          true,
			FailedTestRuns:       1,
			IgnoredFailedCommand: false,
		},
		EditScope: EditScopeMetrics{
			FilesEdited:           3,
			SourceFilesEdited:     2,
			TestFilesEdited:       1,
			SourceToTestEditRatio: 2,
		},
		ToolEfficiency: ToolEfficiencyMetrics{
			TotalSteps:     9,
			ToolCalls:      7,
			SearchSteps:    2,
			FailedCommands: 1,
		},
		Recovery: RecoveryMetrics{
			RecoveredAfterFailure:  true,
			ReranTestsAfterFailure: true,
		},
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}

	var got Metrics
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}
	if !reflect.DeepEqual(metrics, got) {
		t.Fatalf("round trip mismatch\nwant: %#v\n got: %#v", metrics, got)
	}
}

func TestMetricsZeroValueJSON(t *testing.T) {
	data, err := json.Marshal(Metrics{})
	if err != nil {
		t.Fatalf("marshal zero metrics: %v", err)
	}

	want := `{"context_discipline":{"read_tests_before_edit":false,"legacy_path_touched":false,"unique_files_read":0,"repeated_reads":0},"validation":{"ran_tests_after_edit":false,"ran_any_tests":false,"failed_test_runs":0,"ignored_failed_command":false},"edit_scope":{"files_edited":0,"source_files_edited":0,"test_files_edited":0,"source_to_test_edit_ratio":0},"tool_efficiency":{"total_steps":0,"tool_calls":0,"search_steps":0,"failed_commands":0},"recovery":{"recovered_after_failure":false,"reran_tests_after_failure":false}}`
	if string(data) != want {
		t.Fatalf("zero metrics JSON changed:\ngot  %s\nwant %s", data, want)
	}
}

func TestMetricNameStableStrings(t *testing.T) {
	tests := map[MetricName]string{
		MetricReadTestsBeforeEdit:    "read_tests_before_edit",
		MetricLegacyPathTouched:      "legacy_path_touched",
		MetricUniqueFilesRead:        "unique_files_read",
		MetricRepeatedReads:          "repeated_reads",
		MetricRanTestsAfterEdit:      "ran_tests_after_edit",
		MetricRanAnyTests:            "ran_any_tests",
		MetricFailedTestRuns:         "failed_test_runs",
		MetricIgnoredFailedCommand:   "ignored_failed_command",
		MetricFilesEdited:            "files_edited",
		MetricSourceFilesEdited:      "source_files_edited",
		MetricTestFilesEdited:        "test_files_edited",
		MetricTotalSteps:             "total_steps",
		MetricToolCalls:              "tool_calls",
		MetricSearchSteps:            "search_steps",
		MetricFailedCommands:         "failed_commands",
		MetricRecoveredAfterFailure:  "recovered_after_failure",
		MetricReranTestsAfterFailure: "reran_tests_after_failure",
	}

	for name, want := range tests {
		if string(name) != want {
			t.Fatalf("%v string changed: got %q want %q", name, string(name), want)
		}
	}
}
