# agent.md

> 说明：Codex 约定文件名通常是 `AGENTS.md`。本文件内容与 `AGENTS.md` 相同，方便你按需求下载或改名使用。

---

# AGENTS.md

> 本文件是给 Codex / AI coding agent 的长期项目指令。
> 每次开始开发前必须阅读并遵守。

---

## 项目目标

本项目是 `agent-vcr`：AI Coding Agent 的 Behavior Diff / Harness Diff 工具。

第一版目标：

```text
Codex CLI / Codex 客户端优先；v0.1 是 normalized trace + event-level diff
底座，后续路线优先做 pairwise BehaviorSignature / HarnessDiff，而不是
batch matrix 或 benchmark 平台。
```

核心产物：

```text
.agent-vcr/runs/<run-id>/trace.ndjson
```

核心能力：

```text
record
replay
diff
check
export
redact
doctor
```

---

## 最高优先级架构约束

必须遵守：

```text
1. Codex 相关逻辑只能存在于 internal/adapters/codex。
2. 所有 adapter 必须输出 normalized trace.Event。
3. internal/analysis 不得 import internal/adapters/*。
4. internal/report 不得 import internal/adapters/*。
5. internal/check 不得 import internal/adapters/*。
6. internal/trace 不得 import internal/adapters/*。
7. 新增 adapter 不允许修改 replay/diff/check 的核心逻辑，除非是新增通用事件类型。
8. Normalize 失败必须落 raw_event，不能 panic，不能丢数据。
9. hook 命令默认不得向 stdout 输出任何内容。
10. hook 出错必须 exit 0，不能阻塞 Codex。
```

---

## 开发方式

每次只实现一个模块。

不要一次性执行：

```text
实现整个项目。
```

必须按以下顺序开发：

```text
01-bootstrap.md
02-config-trace-store.md
03-codex-hook-adapter.md
04-record-jsonl-generic.md
05-replay-list.md
06-diff-check.md
07-html-report-doctor.md
08-release.md
```

每个模块完成后必须总结：

```text
修改了哪些文件
实现了哪些功能
运行了哪些测试
是否有未完成项
```

---

## 技术栈

```text
Go 1.23+
CLI: cobra
Config: gopkg.in/yaml.v3
File lock: github.com/gofrs/flock
HTML report: Go html/template
```

第一版不要引入前端框架。
第一版不要引入数据库。
第一版不要调用额外大模型 API。

---

## 常用命令

```bash
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr version
go run ./cmd/agent-vcr doctor
```

模块开发完成后至少运行：

```bash
go test ./...
go run ./cmd/agent-vcr --help
```

---

## Trace Schema 规则

统一事件必须是 agent 无关的。

允许：

```json
{
  "type": "tool_call",
  "source": {
    "adapter": "codex-hooks",
    "raw_event_type": "PreToolUse"
  },
  "payload": {
    "tool_name": "Bash"
  }
}
```

禁止：

```json
{
  "codex_tool_use_id": "...",
  "codex_hook_event_name": "PreToolUse"
}
```

Codex 原始字段必须放在：

```text
source.raw_event_type
raw_ref
payload 中的通用字段
```

---

## Adapter 规则

新增 adapter 时：

```text
1. 新建 internal/adapters/<name>/。
2. 实现 Adapter 接口。
3. 声明 Capabilities。
4. 实现 Probe。
5. 实现 Normalize。
6. Normalize 失败输出 raw_event。
7. 添加 testdata fixture。
8. 添加 golden test。
9. 在 registry 注册。
```

Adapter 不允许：

```text
直接生成 report
直接实现 replay/diff/check
绕过 Trace Store 写 trace.ndjson
把具体 agent 字段写进通用 schema
```

---

## Hook 安全规则

hook 命令用于 Codex/Kimi/Claude 等 agent 生命周期回调。

必须：

```text
stdin 读取 JSON
stderr 可写调试日志，但默认尽量安静
stdout 默认为空
任何错误 exit 0
不阻止 agent 正常执行
不上传任何数据
```

禁止：

```text
hook 中运行复杂分析
hook 中调用外部 LLM
hook 中打印普通日志到 stdout
hook 中 panic
```

---

## 隐私规则

默认：

```text
.agent-vcr/ 必须加入 .gitignore
redaction.enabled = true
.env、API key、private key、JWT 默认脱敏
report.html 默认不得包含未脱敏 secret
```

分享报告前必须使用：

```bash
agent-vcr export latest --html --redacted
```

---

## 测试规则

每个模块必须有测试。

优先使用：

```text
fixture tests
golden tests
unit tests
```

必须覆盖：

```text
正常路径
错误路径
unknown/raw event
非 git repo
非法 JSON
secret redaction
architecture import violation
```

---

## 禁止事项

不要：

```text
不要把 Codex 逻辑写进 analysis/report/check/trace。
不要解析 Codex transcript 内部格式作为核心数据源。
不要把 stdout/stderr 大内容直接塞进 trace.ndjson。
不要默认上传任何文件到远端。
不要默认要求用户配置额外 OPENAI_API_KEY。
不要一口气实现多个模块。
不要跳过 go test ./...。
```

---

## 当前开发顺序

当前任务应只读取对应模块文档。

例如开发 Bootstrap 时，只执行：

```text
docs/01-bootstrap.md
```

不要提前实现：

```text
Codex hook
record
replay
diff
check
report
```

---

## 任务完成输出格式

每次完成后输出：

```text
完成模块：<module>

修改文件：
- ...

实现内容：
- ...

测试：
- go test ./...：通过/失败
- 其他命令：...

未完成项：
- 无 / ...
```
