# 05-replay-list.md

> 模块目标：实现 `agent-vcr list` 与 `agent-vcr replay`，让用户能查看 run 列表和回放 timeline。
> 本模块只能读取 normalized trace，不得依赖 Codex/Kimi/Claude 等具体 adapter。

---

## 1. 本模块交付物

新增或修改：

```text
internal/cli/list.go
internal/cli/replay.go
internal/analysis/timeline.go
internal/analysis/replay.go
internal/trace/run_resolver.go  # 如需补充
```

必须支持：

```bash
agent-vcr list
agent-vcr list --json
agent-vcr replay latest
agent-vcr replay <run-id>
agent-vcr replay <run-id> --json
agent-vcr replay <run-id> --step
agent-vcr replay <run-id> --filter tool
```

---

## 2. list 命令

### 2.1 普通输出

```text
ID                         Source        Status     Tools  Files  Started
2026-06-04-183000-codex    codex-hooks   completed  12     3      18:30:00
2026-06-04-190100-generic  generic-cli   failed     0      4      19:01:00
```

字段：

```text
ID
Source
Status
Tools
Files
Started
```

Tools 来源：统计 trace 中 `tool_call` 数量。

Files 来源：从事件 payload.changed_files、patch summary、metadata summary 聚合。

### 2.2 JSON 输出

```json
[
  {
    "run_id": "2026-06-04-183000-codex",
    "source": "codex-hooks",
    "status": "completed",
    "tool_calls": 12,
    "changed_files": 3,
    "started_at": "2026-06-04T18:30:00+08:00"
  }
]
```

---

## 3. Run Resolver

支持：

```bash
agent-vcr replay latest
agent-vcr replay 2026-06-04-183000-codex
agent-vcr replay 183000
```

规则：

1. `latest` → started_at 最新 run。
2. 精确 run_id → 直接打开。
3. 前缀匹配 → 唯一则打开。
4. 多个匹配 → 报错列出候选。
5. 无匹配 → 报错。

---

## 4. Timeline 模型

文件：`internal/analysis/timeline.go`

```go
type TimelineItem struct {
    Index        int64
    Time         time.Time
    Type         string
    Title        string
    Detail       string
    ToolName     string
    ToolUseID    string
    ExitCode     *int
    ChangedFiles []string
    Artifacts    []trace.ArtifactRef
    RawEventID   string
}
```

函数：

```go
func BuildTimeline(events []trace.Event) []TimelineItem
```

---

## 5. ToolCall / ToolResult 合并

如果事件序列为：

```text
tool_call(tool_use_id=u1)
tool_result(tool_use_id=u1)
```

Timeline 显示为一项：

```text
[00:12] Bash npm test → exit 1
```

合并规则：

```text
优先使用 payload.tool_use_id
其次使用 span_id/parent_id
找不到则各自显示
```

---

## 6. replay 输出

示例：

```text
Run: 2026-06-04-183000-codex
Source: codex-hooks
Status: completed
Git: abc1234

00:00 run_start       Codex session started
00:07 user_prompt     修复登录后 session 丢失问题
00:12 tool            Bash rg "session" src tests → exit 0
00:19 tool            apply_patch → changed 2 files
00:26 tool            Bash npm test → exit 0
00:43 run_stop        completed
```

要求：

- prompt 按 redaction 配置显示。
- blob 不默认展开，只显示路径和大小。
- patch 不默认展开，只显示 changed files。

---

## 7. --filter

支持：

```bash
agent-vcr replay latest --filter tool
agent-vcr replay latest --filter shell
agent-vcr replay latest --filter file
agent-vcr replay latest --filter raw
```

过滤逻辑只看 normalized event type，不看 adapter。

---

## 8. --step

MVP 可以实现简单交互：

```text
按 Enter 显示下一步，q 退出
```

如果 stdin 不是 TTY，则忽略 `--step` 并正常输出。

---

## 9. --json

输出：

```json
{
  "run_id": "...",
  "metadata": {...},
  "timeline": [...]
}
```

必须稳定，用于后续测试和机器消费。

---

## 10. 测试要求

fixtures：

```text
testdata/traces/simple_codex/metadata.json
testdata/traces/simple_codex/trace.ndjson
testdata/traces/generic_cli/metadata.json
testdata/traces/generic_cli/trace.ndjson
```

golden：

```text
testdata/golden/replay_simple.txt
testdata/golden/list.txt
```

测试：

1. list 输出稳定。
2. list --json 可解析。
3. replay latest 成功。
4. replay 前缀匹配成功。
5. 多前缀匹配报错。
6. tool_call/tool_result 合并。
7. raw_event 可显示。
8. 不 import adapters。

可用脚本检查：

```bash
! grep -R "internal/adapters" internal/analysis internal/cli/list.go internal/cli/replay.go
```

---

## 11. Codex 执行提示词

```text
请只实现 docs/05-replay-list.md。
不要实现 diff、check、HTML report、doctor。

硬性约束：
- replay/list 只能读取 normalized trace.Event 和 metadata。
- internal/analysis 不得 import internal/adapters/*。
- tool_call/tool_result 根据 tool_use_id 合并。
- 添加 testdata traces 和 golden tests。
- 最后运行 go test ./...。
```

---

## 12. 完成标准

- `agent-vcr list` 可列出 run。
- `agent-vcr replay latest` 可输出 timeline。
- `--json` 输出可解析。
- `--filter tool` 可用。
- golden tests 通过。
- `go test ./...` 通过。
