# 05 - Metrics Cards

## 目标

把 v0.2 `behavior.Metrics` 转换成可视化层的指标卡，用于单 run 和多 run 对比。

## 允许修改

```text
internal/visualize/metrics_cards.go
internal/visualize/metrics_cards_test.go
testdata/visualize/metrics/**
```

## 输出类型

```go
type RunMetricsCards struct {
    RunID string        `json:"run_id"`
    Cards []MetricsCard `json:"cards"`
}

type MetricsCard struct {
    Group string `json:"group"`
    Name  string `json:"name"`
    Value string `json:"value"`
    Level string `json:"level,omitempty"`
}
```

## 指标分组

### Context Discipline

- read tests before edit
- legacy path touched
- files read
- repeated reads

### Validation Behavior

- ran tests after edit
- failed test runs
- skipped validation

### Edit Scope

- files edited
- source files edited
- test files edited
- source/test edit ratio

### Tool Efficiency

- tool calls
- shell commands
- search steps
- failed commands
- repeated commands

### Recovery Behavior

- recovered after failure
- repeated failures

## Level 规则

`Level` 用于 UI 样式：

```text
good
warn
bad
info
```

示例：

- read tests before edit = true → good
- read tests before edit = false 且 edited source → warn/bad
- ran tests after edit = false 且 edited source → bad
- legacy path touched = true → warn

## 主要函数

```go
func BuildMetricsCards(runID string, metrics behavior.Metrics) RunMetricsCards
func CompareMetricsCards(cards []RunMetricsCards) []MetricsComparison
```

v0.2.5 可先不实现复杂 comparison，只要 HTML 能横向展示每个 run 的 cards。

## 测试

必须覆盖：

- 每组指标都有 card。
- Level 计算合理。
- 缺 metrics 时不 panic，输出 warning card。
- JSON round-trip。

## 验收命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
```
