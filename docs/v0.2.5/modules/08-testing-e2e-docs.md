# 08 - Testing / E2E / Docs / Release

## 目标

补齐 v0.2.5 的 E2E、文档和发布验收。

## 允许修改

```text
scripts/e2e-v0.2.5.ps1
scripts/e2e.ps1 可小幅接入
README.md
docs/behavior-diff.md
docs/release-checklist.md
CHANGELOG.md
testdata/e2e/v0.2.5/**
.github/workflows/** 如果只是补 E2E 命令
```

## 禁止修改

```text
internal/** 大范围实现
cmd/** 大范围实现
```

除非 E2E 暴露了明显小 bug，必须说明。

## E2E 场景

构造三个 run：

### Run A：test-first

```text
search "session"
read tests/auth/session.test.ts
read src/auth/session.ts
edit src/auth/session.ts
run npm test
```

### Run B：implementation-first / legacy

```text
search "cookie"
read src/auth/legacy-cookie.ts
edit src/auth/legacy-cookie.ts
finish without test
```

### Run C：source-first but validated

```text
search "session"
read src/auth/session.ts
edit src/auth/session.ts
run npm test
```

## E2E 命令

必须验证：

```powershell
agent-vcr visualize <run-a> --json
agent-vcr visualize <run-a> --html --output single.html
agent-vcr visualize <run-a> <run-b> --json
agent-vcr visualize <run-a> <run-b> --html --output compare.html
agent-vcr visualize <run-a> <run-b> <run-c> --html --output multi.html
```

## E2E 验收点

- JSON 可 `ConvertFrom-Json`。
- HTML 文件存在且非空。
- HTML 包含 `First divergence` 或等价文案。
- HTML 包含 `Swimlane` 或 lanes 区域。
- HTML 包含 file access compare 区域。
- HTML 包含 metrics cards 区域。
- 不依赖真实 Codex。

## README 更新

新增一节：

```md
## Visualizing behavior paths
```

包含：

- 单 run 可视化。
- 双 run 对比。
- 多 run 对比。
- first divergence 高亮。
- 文件访问对比。
- metrics cards。

明确：

```text
v0.2.5 visualizes behavior data produced by v0.2.
v0.2.5 does not add HarnessMetadata or Matrix Compare.
```

## docs/behavior-diff.md 更新

补充：

- v0.2.5 是 Behavior Visualization。
- 它把 v0.2 behavior diff 可视化。
- v0.3 才解释 harness 差异。

## release checklist

增加：

```text
[ ] agent-vcr visualize latest --json works
[ ] agent-vcr visualize latest --html works
[ ] two-run visualize highlights first divergence
[ ] multi-run visualize supports at least 3 lanes
[ ] file access compare is present
[ ] metrics cards are present
[ ] HTML escape / redaction tests pass
[ ] no HarnessMetadata / Matrix / Regression implemented
```

## 测试命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr visualize --help
powershell -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\e2e-v0.2.5.ps1
```

## 最终输出要求

开发完成后输出：

```text
## v0.2.5 验收结果

### 新增功能
- single-run visualize
- two-run swimlane compare
- multi-run compare
- file access compare
- metrics cards

### 测试命令
...

### 阻塞问题
none

### 是否可以发布 v0.2.5
yes/no
```
