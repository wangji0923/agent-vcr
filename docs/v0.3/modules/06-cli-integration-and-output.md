# 06 - CLI Integration and Output

## 目标

实现 `agent-vcr harness` 命令组。

## 允许修改

```text
internal/cli/harness.go
internal/harness/output.go
cmd/agent-vcr 必要接线
testdata/golden/harness-cli/**
README.md 小幅命令说明
docs/behavior-diff.md 小幅更新
```

## 禁止修改

```text
internal/adapters/**
internal/behavior 核心语义
internal/trace 核心语义
```

## 命令设计

```bash
agent-vcr harness --help
agent-vcr harness inspect latest
agent-vcr harness inspect <run-id>
agent-vcr harness inspect latest --json
agent-vcr harness inspect latest --refresh

agent-vcr harness diff run-a run-b
agent-vcr harness diff run-a run-b --json
agent-vcr harness diff run-a run-b --with-behavior
```

## Human 输出

`inspect` 输出：

```text
Harness: run_001

Model:
  name: gpt-5-codex

Rules:
  AGENTS.md: present sha256:...

Tools:
  names: Bash, apply_patch

Permission:
  mode: workspace-write

Warnings:
  prompt hash unavailable
```

`diff` 输出：

```text
Harness diff: run-a vs run-b

Same:
- model

Different:
- AGENTS.md changed
- tool set changed
- permission mode changed

Behavior impact:
- First behavior divergence occurred at file discovery.
- Correlation only; no causality inferred.
```

## JSON 输出

必须可解析，不要混入 human text。

## 错误处理

- run 不存在：明确错误。
- metadata 缺失：自动 inspect/detect 或提示 refresh。
- behavior diff 不可用：harness diff 仍成功。
- damaged cache：明确错误并建议 `--refresh`。

## 测试要求

- `harness --help`。
- `harness inspect latest`。
- `harness inspect latest --json` 可解析。
- `harness diff a b`。
- `harness diff a b --json` 可解析。
- behavior impact unavailable fallback。
- golden output 稳定。

## 验证命令

```powershell
gofmt -w .
go test ./internal/harness/... ./internal/cli/...
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr harness --help
```
