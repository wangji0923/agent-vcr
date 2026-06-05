# 04 - Harness Diff Engine

## 目标

实现 pairwise HarnessDiff，只比较两个 run 的 HarnessMetadata。

## 允许修改

```text
internal/harness/diff.go
internal/harness/output.go
internal/harness/*_test.go
testdata/harness/diff/**
```

## 输入输出

输入：

```go
left  *HarnessMetadata
right *HarnessMetadata
```

输出：

```go
HarnessDiff
```

## 比较维度

至少比较：

- model.name / model.hash
- adapter.name / adapter.version
- prompt.hash
- AGENTS.md hash
- rules files hash
- tool set hash
- MCP set hash
- permission mode
- sandbox mode
- context policy
- `.agent-vcr/config.yml` hash
- `.codex/hooks.json` hash
- repo git sha / branch

## 分类

输出分类：

```text
same
different
missing-left
missing-right
unknown
```

Severity 建议：

```text
info:
  branch changed
  adapter version changed

medium:
  AGENTS.md changed
  tool set changed
  permission mode changed

high:
  model changed
  prompt hash changed
  context policy changed
```

Severity 只是启发式，不要写成因果。

## 输出 Summary

Human summary 示例：

```text
Same:
- model
- task prompt hash

Different:
- AGENTS.md changed
- tool set changed
- permission mode changed
```

## 测试要求

- same metadata diff。
- AGENTS.md changed。
- tool set changed。
- permission changed。
- missing field。
- JSON output roundtrip。
- severity 分类测试。
- stable ordering 测试。

## 验证命令

```powershell
gofmt -w .
go test ./internal/harness/...
go test ./...
go vet ./...
```
