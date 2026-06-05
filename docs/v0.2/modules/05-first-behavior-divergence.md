# 05 - First Behavior Divergence

## 目标

实现两个 BehaviorSignature 的 pairwise 行为差异分析，核心是 first behavior divergence。

## 允许修改

```text
internal/behavior/divergence.go
internal/behavior/diff.go
internal/behavior/diff_test.go
testdata/behavior/golden/diff/**
```

## 禁止修改

```text
internal/adapters/**
internal/check/**
internal/report/**
cmd/**，除非 07 模块接线
```

## Diff API

建议接口：

```go
type DiffOptions struct {
    IgnoreRawBehavior bool
    IgnoreProcessNoise bool
}

type DiffResult struct {
    RunA string `json:"run_a"`
    RunB string `json:"run_b"`
    FirstDivergence *Divergence `json:"first_divergence,omitempty"`
    Summary DiffSummary `json:"summary"`
    MetricsDelta MetricsDelta `json:"metrics_delta"`
}

func DiffSignatures(a, b Signature, opts DiffOptions) DiffResult
```

## First divergence 算法

v0.2 先使用线性比较，不做复杂 LCS：

```text
1. 过滤噪声 step
2. 生成 stable key
3. 从 index 0 往后比较
4. 第一个不同 key 即 first divergence
5. 若一方提前结束，则返回 missing_step / extra_step
```

## Divergence kind

建议值：

```text
step_changed
step_missing_in_a
step_missing_in_b
result_changed
no_divergence
```

## Explanation 模板

v0.2 可以生成规则模板解释，但不要使用 LLM。

示例规则：

```text
A = inspect_test, B = read_file legacy → "Run B entered legacy path before inspecting tests."
A = run_test, B = edit_file → "Run B continued editing where Run A validated behavior."
A = search, B = search but query different → "Runs diverged during search/query selection."
```

模板必须保守，不能假装知道模型意图。

## 测试要求

```text
TestFirstDivergenceStepChanged
TestFirstDivergenceMissingStep
TestFirstDivergenceNoDivergence
TestFirstDivergenceIgnoresStepIDs
TestFirstDivergenceSearchQueryChanged
TestFirstDivergenceTestInspectionVsLegacyRead
TestDiffResultJSONGolden
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

## Codex 执行提示词

```text
请只实现 05-first-behavior-divergence.md。基于 BehaviorSignature 做 pairwise behavior diff 和 first divergence。不要接 CLI，不要做 HarnessDiff，不要做 Matrix。补 golden tests，运行 go test ./internal/behavior/... 和 go test ./...。
```
