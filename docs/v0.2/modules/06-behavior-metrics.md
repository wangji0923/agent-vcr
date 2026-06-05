# 06 - Behavior Metrics

## 目标

实现行为指标计算，为 behavior diff 提供可解释的量化维度。v0.2 的 metrics 不做评分系统，只输出事实指标。

## 允许修改

```text
internal/behavior/metrics.go
internal/behavior/metrics_compute.go
internal/behavior/metrics_test.go
testdata/behavior/golden/metrics/**
```

## 禁止修改

```text
internal/adapters/**
internal/check/**
internal/report/**
cmd/**，除非 07 模块接线
```

## 指标分类

### ContextDisciplineMetrics

```go
type ContextDisciplineMetrics struct {
    ReadTestsBeforeEdit bool `json:"read_tests_before_edit"`
    LegacyPathTouched   bool `json:"legacy_path_touched"`
    UniqueFilesRead     int  `json:"unique_files_read"`
    RepeatedReads       int  `json:"repeated_reads"`
}
```

### ValidationMetrics

```go
type ValidationMetrics struct {
    RanTestsAfterEdit    bool `json:"ran_tests_after_edit"`
    RanAnyTests          bool `json:"ran_any_tests"`
    FailedTestRuns       int  `json:"failed_test_runs"`
    IgnoredFailedCommand bool `json:"ignored_failed_command"`
}
```

### EditScopeMetrics

```go
type EditScopeMetrics struct {
    FilesEdited          int     `json:"files_edited"`
    SourceFilesEdited    int     `json:"source_files_edited"`
    TestFilesEdited      int     `json:"test_files_edited"`
    SourceToTestEditRatio float64 `json:"source_to_test_edit_ratio"`
}
```

### ToolEfficiencyMetrics

```go
type ToolEfficiencyMetrics struct {
    TotalSteps     int `json:"total_steps"`
    ToolCalls      int `json:"tool_calls"`
    SearchSteps    int `json:"search_steps"`
    FailedCommands int `json:"failed_commands"`
}
```

### RecoveryMetrics

```go
type RecoveryMetrics struct {
    RecoveredAfterFailure bool `json:"recovered_after_failure"`
    ReranTestsAfterFailure bool `json:"reran_tests_after_failure"`
}
```

## 计算规则

### ReadTestsBeforeEdit

如果第一次 `inspect_test` 出现在第一次 `edit_file` 之前，则 true。

### RanTestsAfterEdit

如果第一次 `run_test` 出现在任意 `edit_file` 之后，则 true。

### LegacyPathTouched

任意 step files/path 命中 legacy 规则，则 true。

### IgnoredFailedCommand

如果存在失败的 `run_test` 或 `run_build`，之后没有 recovery/edit/rerun 行为，且 run 结束，则 true。v0.2 可先 best effort。

## MetricsDelta

用于 diff：

```go
type MetricsDelta struct {
    ContextDiscipline map[string]DeltaValue `json:"context_discipline"`
    Validation        map[string]DeltaValue `json:"validation"`
    EditScope         map[string]DeltaValue `json:"edit_scope"`
    ToolEfficiency    map[string]DeltaValue `json:"tool_efficiency"`
    Recovery          map[string]DeltaValue `json:"recovery"`
}
```

也可以先实现简化版本，保证 JSON 可读。

## 测试要求

```text
TestMetricsReadTestsBeforeEdit
TestMetricsRanTestsAfterEdit
TestMetricsLegacyPathTouched
TestMetricsRepeatedReads
TestMetricsEditScope
TestMetricsToolEfficiency
TestMetricsRecoveryAfterFailure
TestMetricsDelta
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

## Codex 执行提示词

```text
请只实现 06-behavior-metrics.md。计算行为事实指标，不做 risk score，不接 CLI，不实现 HarnessDiff。补表驱动测试，运行 go test ./internal/behavior/... 和 go test ./...。
```
