# 08 - Testing, E2E, Docs and Release Checklist

## 目标

补齐 v0.2 的测试、E2E、文档和 release checklist，确保 BehaviorSignature 作为 v0.2 主功能可以验收。

## 允许修改

```text
scripts/e2e.ps1
README.md
docs/behavior-diff.md
docs/trace-schema.md
docs/release-checklist.md
docs/v0.2-behavior-signature.md   # 可新增
.github/workflows/ci.yml
CHANGELOG.md
```

## 谨慎修改

```text
internal/** 仅允许修测试暴露的小 bug
cmd/** 仅允许修 CLI 文档不一致的小 bug
```

## E2E 要覆盖

新增或扩展 `scripts/e2e.ps1`，至少覆盖：

```text
1. 创建临时 git repo
2. 模拟两个 run：Run A 和 Run B
3. Run A 行为：search session → inspect test → edit source → run test success
4. Run B 行为：search cookie → read legacy → edit legacy → skip tests
5. 执行 agent-vcr behavior latest
6. 执行 agent-vcr behavior latest --json
7. 执行 agent-vcr behavior diff run-a run-b
8. 执行 agent-vcr behavior diff run-a run-b --json
9. 执行 agent-vcr behavior metrics latest
```

## 文档更新

### README

增加 v0.2 区块：

```md
## v0.2: BehaviorSignature

v0.2 turns normalized trace events into behavior steps and finds first behavior divergence between two runs.
```

### docs/behavior-diff.md

更新：

```text
v0.1 event-level diff
v0.2 BehaviorSignature
v0.3 HarnessMetadata / HarnessDiff
```

### docs/v0.2-behavior-signature.md

建议新增，包含：

```text
BehaviorStep
BehaviorSignature
BehaviorDiff
BehaviorMetrics
CLI examples
Limitations
```

## CI 更新

CI 增加：

```text
go test ./...
go vet ./...
go build ./cmd/agent-vcr
agent-vcr --help
scripts/e2e.ps1
behavior command smoke tests
```

## Release checklist 更新

增加：

```text
[ ] behavior command exists
[ ] behavior latest works
[ ] behavior latest --json parses
[ ] behavior diff finds first behavior divergence
[ ] behavior diff --json parses
[ ] behavior metrics works
[ ] v0.2 docs do not claim HarnessDiff implemented
[ ] v0.2 docs do not claim Matrix implemented
```

## 验收命令

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr behavior --help
powershell -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
```

## Codex 执行提示词

```text
请只实现 08-testing-e2e-release.md。补齐 v0.2 behavior 的 E2E、README 和 docs，不新增功能。确认文档没有把 v0.3 HarnessMetadata 写成已实现。运行 go test ./...、go vet、go build 和 scripts/e2e.ps1。
```
