# 08 - Integration Review and Release Gate

## 目标

最终验收 v0.3。只做审查和必要小修复，不新增功能。

## 检查项

## 架构

- `internal/harness` 不 import `internal/adapters`。
- harness 核心类型不含 Codex/Kimi/Claude 专用字段。
- v0.2 behavior CLI 未破坏。
- v0.1 基础命令未破坏。
- harness diff 不做因果断言。

## 功能

- `agent-vcr harness inspect latest` 可用。
- `agent-vcr harness inspect latest --json` 可解析。
- `agent-vcr harness diff a b` 可用。
- `agent-vcr harness diff a b --json` 可解析。
- AGENTS.md hash 差异能识别。
- tool set / MCP 差异能识别。
- permission mode 差异能识别。
- behavior impact 能优雅降级。

## 文档

- README v0.3 定位准确。
- docs/harness-diff.md 存在。
- docs/behavior-diff.md 更新。
- release checklist 更新。
- 不恢复 Matrix/Regression/LLM Explain 主线。

## 必须运行

```powershell
gofmt -w .
go test ./internal/harness/...
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr harness --help
go run ./cmd/agent-vcr behavior --help
powershell -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\e2e-v0.3.ps1
```

## 输出格式

```text
## v0.3 最终验收结果

### 总体结论
通过 / 不通过 / 通过但有 warning

### 新增能力
- HarnessMetadata
- harness inspect
- harness diff
- behavior impact correlation

### 架构检查
...

### 测试命令
...

### 阻塞问题
none

### 非阻塞 warning
none

### 是否可以发布 v0.3
yes / no
```
