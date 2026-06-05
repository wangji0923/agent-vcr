# 08-release.md

> 模块目标：完善测试矩阵、文档、GitHub Actions、构建、发布和后续扩展指南。
> 本模块不新增核心功能，只做工程化收尾。

---

## 1. 本模块交付物

新增或修改：

```text
README.md
CHANGELOG.md
LICENSE
CONTRIBUTING.md
docs/adapter-development.md
docs/trace-schema.md
.github/workflows/ci.yml
.github/workflows/release.yml
.goreleaser.yml
```

必须支持：

```bash
go test ./...
go vet ./...
go run ./cmd/agent-vcr doctor
go build ./cmd/agent-vcr
```

---

## 2. README 结构

README 必须包含：

```text
1. 项目一句话
2. 为什么不是普通日志
3. Quick Start
4. Codex 接入
5. Generic CLI 接入
6. Replay / Diff / Check / Export 示例
7. Adapter 架构
8. 自研 Agent 接入路线
9. 隐私与安全
10. 当前限制
11. Roadmap
```

第一屏文案建议：

```text
agent-vcr
Behavior diff for AI coding agents.

Same model. Same task. Different harness.
agent-vcr records normalized traces and shows where agent behavior diverged.
```

---

## 3. Quick Start

```bash
brew install agent-vcr
# 或下载二进制

cd your-repo
agent-vcr init codex
codex

agent-vcr list
agent-vcr replay latest
agent-vcr diff run-a run-b
agent-vcr check latest
agent-vcr export latest --html
```

Generic CLI 示例：

```bash
agent-vcr record -- some-agent "fix bug"
```

---

## 4. Adapter Development Guide

文件：`docs/adapter-development.md`

必须说明：

```text
如何新增 adapter
Adapter 接口
Capabilities
Normalize 规则
RawEvent 兜底
Fixture 格式
Golden test
禁止 import 规则
```

新增 adapter 示例：

```text
internal/adapters/kimi/
  install.go
  hook.go
  normalize.go
  testdata/
```

必须写清楚：

```text
新增 adapter 正常不需要修改 replay/diff/check/report。
如果需要修改，必须说明是新增通用事件类型，而不是 adapter 特例。
```

---

## 5. Trace Schema 文档

文件：`docs/trace-schema.md`

必须包含：

```text
schema_version
event_id
run_id
parent_id
span_id
event_index
type
source
payload
artifacts
raw_ref
```

每个事件类型给一个 JSON 示例：

```text
run_start
user_prompt
tool_call
tool_result
process_start
process_result
raw_event
run_stop
```

---

## 6. CI Workflow

文件：`.github/workflows/ci.yml`

要求：

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
        go: ['1.23.x']
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
      - run: go mod tidy
      - run: git diff --exit-code go.mod go.sum
      - run: go test ./...
      - run: go vet ./...
      - run: go build ./cmd/agent-vcr
      - run: go run ./cmd/agent-vcr doctor
```

如果 Windows 支持暂时不稳定，README 写明 v0.1 primary target 是 macOS/Linux。

---

## 7. Release Workflow

推荐使用 GoReleaser。

文件：`.goreleaser.yml`

构建目标：

```text
linux amd64/arm64
darwin amd64/arm64
windows amd64 可选
```

二进制名：

```text
agent-vcr
```

Release 产物：

```text
tar.gz
checksums.txt
```

---

## 8. 版本策略

采用 SemVer：

```text
0.1.x：Trace Foundation + Event-level Diff
0.2.x：BehaviorSignature + First Behavior Divergence
0.3.x：HarnessMetadata + Pairwise HarnessDiff
0.4.x：Pairwise Compare Report
0.5.x：Capture Completeness + selected adapters / SDK
1.0.0：Stable Protocol
```

Trace schema 版本独立：

```text
trace schema_version: "0.2"
```

注意：CLI 版本和 trace schema 版本不要混用。

---

## 9. 发布前检查清单

```text
[ ] go test ./...
[ ] go vet ./...
[ ] go build ./cmd/agent-vcr
[ ] agent-vcr doctor 通过
[ ] README Quick Start 可执行
[ ] README first screen positions project as Behavior Diff / Harness Diff foundation
[ ] docs/behavior-diff.md reflects pairwise comparison roadmap
[ ] docs/trace-schema.md 更新
[ ] docs/adapter-development.md 更新
[ ] CHANGELOG.md 更新
[ ] release tag 创建
[ ] GitHub Release 产物上传
```

---

## 10. 隐私与安全声明

README 必须明确：

```text
agent-vcr 默认本地运行。
核心功能不调用额外 LLM API。
.agent-vcr/ 默认应加入 .gitignore。
trace 可能包含 prompt、文件路径、命令输出、patch。
分享报告前请使用 --redacted。
agent-vcr check 不是完整安全沙箱。
```

---

## 11. Roadmap

```text
v0.1: Trace Foundation + Event-level Diff
- Codex hooks
- Codex exec --json
- Generic CLI wrapper
- normalized trace
- list / replay
- event-level diff
- check / export / doctor

v0.2: BehaviorSignature + First Behavior Divergence
- behavior.Step
- BehaviorSignature
- behavior timeline
- first behavior divergence
- behavior metrics

v0.3: HarnessMetadata + Pairwise HarnessDiff
- model / prompt / AGENTS.md / tool schema hashes
- MCP config hash
- permission mode
- sandbox mode
- context policy

v0.4: Pairwise Compare Report
- harness changes
- first behavior divergence
- behavior metrics
- validation behavior
- edit scope
- outcome difference

v0.5: Capture Completeness + selected adapters / SDK
- better Codex tool classification
- Read/Edit/Search/Test behavior recognition
- selected Claude/Kimi adapters
- minimal Python / TypeScript SDK
- HTTP ingest

v1.0: Stable Protocol
- Trace Schema v1
- BehaviorSignature v1
- HarnessMetadata v1
- Adapter API v1
```

---

## 12. Codex 执行提示词

```text
请只实现 docs/08-release.md。
不要新增核心功能。

硬性约束：
- 完善 README、CHANGELOG、CONTRIBUTING、docs/adapter-development.md、docs/trace-schema.md。
- 添加 GitHub Actions CI。
- 添加 GoReleaser 配置。
- README 要明确 Behavior Diff / Harness Diff 定位，Codex first，但架构支持 adapter 扩展。
- README 要明确隐私边界和当前限制。
- 最后运行 go test ./...、go vet ./...、go build ./cmd/agent-vcr。
```

---

## 13. 完成标准

- CI 文件存在。
- Release 配置存在。
- README 可指导用户完成 Codex 接入。
- Adapter 开发指南存在。
- Trace schema 文档存在。
- 隐私和限制写清楚。
- `go test ./...` 通过。
