# 07 - Testing, Docs, and E2E

## 目标

补齐 v0.3 文档、E2E 和 release checklist。不要实现核心功能。

## 允许修改

```text
README.md
docs/**
scripts/**
testdata/e2e/**
testdata/harness/**
.github/workflows/**
CHANGELOG.md
```

## 禁止修改

```text
internal/** 除非修文档测试暴露的明显小问题
cmd/** 除非 CLI 文档命令和实际命令不一致
```

## README 更新

README 应新增 v0.3 段落：

```text
v0.3 adds pairwise harness diff:
- inspect harness metadata
- compare AGENTS.md / tool set / permission / MCP config
- correlate harness changes with first behavior divergence
```

必须强调：

- v0.3 只做 pairwise。
- 不做 Matrix。
- 不做 Regression。
- 不做 LLM explain。

## docs/behavior-diff.md 更新

补充：

```text
v0.3: HarnessMetadata / Pairwise HarnessDiff
```

并解释：

```text
v0.2 tells where behavior diverged.
v0.3 tells which harness inputs differed.
```

## 新增 docs/harness-diff.md

内容：

- Harness 定义。
- HarnessMetadata 字段。
- HarnessDiff 示例。
- 与 BehaviorDiff 的关系。
- 隐私策略。
- 非因果声明。

## E2E 脚本

可新增：

```text
scripts/e2e-v0.3.ps1
```

构造两个 run：

Run A:

```text
AGENTS.md hash A
tool set: Bash, apply_patch
permission: workspace-write
behavior: read tests before edit
```

Run B:

```text
AGENTS.md hash B
tool set: Bash, apply_patch, mcp__filesystem
permission: unrestricted 或不同值
behavior: read legacy before tests
```

验证：

```powershell
agent-vcr harness inspect <run-a>
agent-vcr harness diff <run-a> <run-b>
agent-vcr harness diff <run-a> <run-b> --json
```

## Release Checklist

新增 v0.3 项：

```text
[ ] harness inspect works
[ ] harness diff works
[ ] harness diff JSON output parses
[ ] AGENTS.md hash difference detected
[ ] tool set difference detected
[ ] permission mode difference detected
[ ] behavior impact includes no causality claim
[ ] v0.2 behavior commands still pass
[ ] docs do not mention Matrix as near-term roadmap
```

## 验证命令

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/agent-vcr
powershell -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\e2e-v0.3.ps1
```
