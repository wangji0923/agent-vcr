# 05 - Behavior Impact Correlation

## 目标

把 v0.3 HarnessDiff 和 v0.2 BehaviorDiff 轻量关联，输出 behavior impact 摘要。

注意：不能做因果推断，只能输出“可能相关”。

## 允许修改

```text
internal/harness/impact.go
internal/harness/*_test.go
testdata/harness/impact/**
```

## 依赖

允许依赖：

```text
internal/behavior
internal/trace
```

禁止依赖：

```text
internal/adapters
```

## 设计

输入：

```go
HarnessDiff
behavior.Divergence 或 behavior.DiffResult
```

输出：

```go
BehaviorImpact
```

示例字段：

```go
type BehaviorImpact struct {
    FirstBehaviorDivergence string   `json:"first_behavior_divergence,omitempty"`
    RelatedHarnessChanges   []string `json:"related_harness_changes,omitempty"`
    Summary                 string   `json:"summary,omitempty"`
    CausalityDisclaimer     string   `json:"causality_disclaimer"`
}
```

## 关联规则

示例启发式：

- AGENTS.md changed + divergence at file discovery。
- tool set changed + divergence at tool call。
- permission mode changed + permission/request/tool block divergence。
- MCP set changed + mcp tool divergence。
- prompt hash changed + early search/query divergence。
- context policy changed + read_file / search divergence。

输出必须包含 disclaimer：

```text
agent-vcr reports correlation only. It does not infer causality.
```

## 测试要求

- AGENTS.md change + file discovery divergence。
- tool set change + tool divergence。
- no behavior diff fallback。
- no harness changes fallback。
- disclaimer 始终存在。
- JSON roundtrip。

## 验证命令

```powershell
gofmt -w .
go test ./internal/harness/...
go test ./...
go vet ./...
```
