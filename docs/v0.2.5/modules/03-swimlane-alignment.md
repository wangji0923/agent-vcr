# 03 - Swimlane Alignment

## 目标

实现 behavior steps 的泳道对齐，用于单 run、双 run 和 3-5 个 run 的可视化对比。

## 允许修改

```text
internal/visualize/align.go
internal/visualize/path_graph.go
internal/visualize/align_test.go
testdata/visualize/align/**
```

## 禁止修改

```text
internal/adapters/**
internal/behavior/** public API
internal/trace/**
internal/cli/**
```

## 对齐策略

### 单 run

直接每个 step 一行。

### 双 run

使用 LCS / 动态规划，输入为 baseline steps 和 compare steps。

### 多 run

以第一个 run 为 baseline，其他 run 分别和 baseline 对齐，然后合并 alignment rows。

v0.2.5 不做全局多序列最优对齐。

## Step Key

实现：

```go
func StepKey(step VisualStep) string
func StepSimilarity(a, b VisualStep) int
```

建议评分：

```text
kind 相同: +3
target 相同: +3
query 相同: +2
主文件相同: +2
command kind 相同: +1
edit vs read / test vs edit 这类强差异: -3
```

## 主要函数

```go
func AlignLanes(lanes []BehaviorLane, opts AlignOptions) []AlignmentRow
func AlignPair(left, right BehaviorLane, opts AlignOptions) []AlignmentRow
func MarkDivergence(rows []AlignmentRow, divergences []VisualDivergence) []AlignmentRow
func BuildPathGraph(lanes []BehaviorLane) *PathGraph
```

## Divergence 高亮

- 使用 v0.2 `behavior.Divergence` 结果转换成 `VisualDivergence`。
- 对齐表中对应 row 设置 `IsDivergent=true`。
- 对左右 step 设置 `Divergent=true`。

## 测试场景

### 场景 1：完全相同

```text
A: search → read → edit → test
B: search → read → edit → test
```

预期：无 divergence，高度对齐。

### 场景 2：读测试 vs 读 legacy

```text
A: search session → read tests → read src → edit → test
B: search session → read legacy → edit → finish
```

预期：第 2 行 divergence。

### 场景 3：gap

```text
A: search → read test → read source → edit → test
B: search → read source → edit → test
```

预期：B 在 read test 行是 gap。

### 场景 4：三 run

A 作为 baseline，B/C 分别对齐。

## 验收命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
```
