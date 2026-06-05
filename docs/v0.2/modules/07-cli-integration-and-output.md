# 07 - CLI Integration and Output

## 目标

把 behavior 模块接入 CLI，提供 `agent-vcr behavior` 命令组，并支持 human-readable 和 JSON 输出。

## 允许修改

```text
cmd/agent-vcr/**
internal/cli/behavior.go
internal/behavior/render.go
internal/behavior/json.go
internal/resolver/**，如果需要复用 run resolver
internal/cli/*_test.go
testdata/behavior/golden/cli/**
```

## 禁止修改

```text
internal/adapters/**
internal/trace 既有语义
internal/check/**
internal/report/**
```

## 命令设计

新增：

```bash
agent-vcr behavior <run-id|latest>
agent-vcr behavior diff <run-a> <run-b>
agent-vcr behavior metrics <run-id|latest>
```

全部支持：

```bash
--json
```

### `agent-vcr behavior latest`

输出 behavior timeline：

```text
Behavior timeline: latest
1. search       rg "session" src tests
2. inspect_test tests/auth/session.test.ts
3. read_file    src/auth/session.ts
4. edit_file    src/auth/session.ts
5. run_test     npm test → success
```

### `agent-vcr behavior diff run-a run-b`

输出：

```text
First behavior divergence at step 3

Run A:
  inspect_test tests/auth/session.test.ts

Run B:
  read_file src/auth/legacy-cookie.ts

Summary:
  Run B entered legacy implementation path before inspecting tests.
```

### `agent-vcr behavior metrics latest`

输出：

```text
Context discipline:
  read tests before edit: yes
  legacy path touched: no

Validation:
  ran tests after edit: yes
```

## JSON 输出

必须可解析，且稳定。建议直接输出：

```text
behavior latest --json   → Signature
behavior diff --json     → DiffResult
behavior metrics --json  → Metrics
```

## Error handling

```text
run 不存在 → 非 0 exit code，清晰错误
trace 缺失 → 非 0 exit code 或 empty behavior + warning，二选一但要测试
非法 trace 行 → 降级 warning，不 panic
```

## 测试要求

```text
TestBehaviorCommandExists
TestBehaviorLatestHumanOutput
TestBehaviorLatestJSONOutput
TestBehaviorDiffHumanOutput
TestBehaviorDiffJSONOutput
TestBehaviorMetricsHumanOutput
TestBehaviorMetricsJSONOutput
TestBehaviorRunNotFound
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/cli/... ./internal/behavior/...
go test ./...
go run ./cmd/agent-vcr behavior latest --json
```

## Codex 执行提示词

```text
请只实现 07-cli-integration-and-output.md。把 internal/behavior 接入 CLI，新增 behavior 命令组和 JSON 输出。不要实现 HarnessDiff、Matrix 或新 adapter。补 CLI 测试，运行 go test ./... 和 go run ./cmd/agent-vcr --help。
```
