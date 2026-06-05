# 03-codex-hook-adapter.md

> 模块目标：实现 Codex Hook Adapter，包括 `agent-vcr init codex` 与 `agent-vcr hook --adapter codex`。
> 本模块只能写 Codex 适配层，不得修改 replay / diff / check / report 逻辑。

---

## 1. 本模块交付物

新增或修改：

```text
internal/adapters/codex/install.go
internal/adapters/codex/hook.go
internal/adapters/codex/normalize.go
internal/adapters/codex/types.go
internal/adapters/codex/testdata/
internal/cli/init.go
internal/cli/hook.go
```

必须支持命令：

```bash
agent-vcr init codex
agent-vcr hook --adapter codex
```

---

## 2. Codex Hook Adapter 能力声明

Capabilities：

```go
Capabilities{
    PromptCapture:      true,
    ToolCallCapture:    true,
    ToolResultCapture:  true,
    ShellCapture:       true,
    FileDiffCapture:    true,
    PermissionCapture:  true,
    SubagentCapture:    true,
    CanInstallHooks:    true,
}
```

注意：

```text
ModelCallCapture = false
ModelResultCapture = false
```

因为 hook 看到的是 agent 可观测行为，不记录模型内部私有推理。

---

## 3. Codex Hook 配置生成

`agent-vcr init codex` 默认写项目级：

```text
.codex/hooks.json
.agent-vcr/config.yml
.gitignore 追加 .agent-vcr/
```

支持 flags：

```bash
agent-vcr init codex --scope project
agent-vcr init codex --scope user
agent-vcr init codex --force
```

v0.1 默认：

```text
--scope project
```

User scope 路径：

```text
~/.codex/hooks.json
```

Project scope 路径：

```text
<repo>/.codex/hooks.json
```

---

## 4. hooks.json 内容

生成内容：

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10,
            "statusMessage": "agent-vcr: recording session"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10,
            "statusMessage": "agent-vcr: recording prompt"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash|apply_patch|mcp__.*",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10,
            "statusMessage": "agent-vcr: recording tool call"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Bash|apply_patch|mcp__.*",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 30,
            "statusMessage": "agent-vcr: recording tool result"
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "matcher": "Bash|apply_patch|mcp__.*",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10,
            "statusMessage": "agent-vcr: recording permission request"
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": "manual|auto",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10
          }
        ]
      }
    ],
    "PostCompact": [
      {
        "matcher": "manual|auto",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "agent-vcr hook --adapter codex",
            "timeout": 10,
            "statusMessage": "agent-vcr: finalizing run"
          }
        ]
      }
    ]
  }
}
```

---

## 5. 合并已有 hooks.json

不要粗暴覆盖用户已有配置。

策略：

1. 如果 `.codex/hooks.json` 不存在，创建。
2. 如果存在，解析 JSON。
3. 对每个 event 追加 agent-vcr hook。
4. 如果同一个 command 已存在，不重复追加。
5. 写入前备份：`.codex/hooks.json.bak.<timestamp>`。
6. `--force` 允许覆盖 agent-vcr 自己的旧配置，但仍不删除用户其他 hook。

测试：

```text
空文件生成成功
已有 hooks 保留
重复 init 不重复添加
force 更新 timeout/statusMessage
JSON 损坏时报错且不覆盖
```

---

## 6. Codex Hook 输入类型

文件：`internal/adapters/codex/types.go`

```go
type HookInput struct {
    SessionID       string         `json:"session_id"`
    TranscriptPath  string         `json:"transcript_path"`
    Cwd             string         `json:"cwd"`
    HookEventName   string         `json:"hook_event_name"`
    Model           string         `json:"model"`
    PermissionMode  string         `json:"permission_mode,omitempty"`
    TurnID          string         `json:"turn_id,omitempty"`
    ToolName        string         `json:"tool_name,omitempty"`
    ToolUseID       string         `json:"tool_use_id,omitempty"`
    ToolInput       map[string]any `json:"tool_input,omitempty"`
    ToolResponse    any            `json:"tool_response,omitempty"`
    Prompt          string         `json:"prompt,omitempty"`
    LastAssistantMessage string    `json:"last_assistant_message,omitempty"`
    Raw             map[string]any `json:"-"`
}
```

解析时保留 raw：

```go
func ParseHookInput(raw []byte) (HookInput, map[string]any, error)
```

---

## 7. Hook 命令行为

`agent-vcr hook --adapter codex` 必须遵守：

```text
读取 stdin JSON
不向 stdout 输出任何内容
出错时 exit 0
只做记录，不做复杂分析
不阻止 Codex 行为
```

伪代码：

```go
func RunCodexHook(ctx context.Context) int {
    raw, err := io.ReadAll(os.Stdin)
    if err != nil { return 0 }

    input, rawMap, err := codex.ParseHookInput(raw)
    if err != nil { return 0 }

    projectDir := input.Cwd
    cfg, _, _ := config.Load(projectDir, "")

    runID := ResolveCodexRunID(input)
    store := trace.OpenOrCreateRun(projectDir, runID, "codex-hooks")

    rawRef := store.WriteBlob("raw/...json", raw, "application/json")
    events, err := adapter.Normalize(ctx, adapters.RawEvent{...})
    if err != nil {
        events = []trace.Event{trace.NewRawEvent(runID, source, rawRef)}
    }

    for _, event := range events {
        event.RawRef = &rawRef
        store.Append(event)
    }

    return 0
}
```

---

## 8. Run ID 规则

Codex hook 输入有 session_id。

推荐 run_id：

```text
<YYYY-MM-DD-HHMMSS>-codex-<short-session-id>
```

映射文件：

```text
.agent-vcr/state/sessions.json
```

内容：

```json
{
  "codex:s_123": "2026-06-04-183000-codex-s_123"
}
```

要求：

- 同一个 session_id 始终映射到同一个 run_id。
- 如果首次收到的不是 SessionStart，也能懒创建 run。
- 并发访问要加锁。

---

## 9. Normalize 映射规则

### 9.1 SessionStart → run_start

Payload：

```json
{
  "session_id": "...",
  "model": "...",
  "permission_mode": "...",
  "transcript_path": "path_only"
}
```

注意：只保存 transcript_path 路径，不解析 transcript 内容。

### 9.2 UserPromptSubmit → user_prompt

Payload：

```json
{
  "turn_id": "...",
  "prompt": "按配置 full/redacted/hash/none 处理",
  "prompt_sha256": "..."
}
```

### 9.3 PreToolUse → tool_call

Payload：

```json
{
  "turn_id": "...",
  "tool_use_id": "...",
  "tool_name": "Bash",
  "input": {...}
}
```

如果 tool 是 Bash，可以额外产生 shell_command 事件。

### 9.4 PostToolUse → tool_result

Payload：

```json
{
  "turn_id": "...",
  "tool_use_id": "...",
  "tool_name": "Bash",
  "result": "按配置 blob/hash/inline 处理"
}
```

如果 tool 是 Bash，可以额外产生 shell_result 事件。

### 9.5 PermissionRequest → permission_request

Payload：

```json
{
  "tool_name": "...",
  "tool_input": {...},
  "permission_mode": "..."
}
```

### 9.6 PreCompact / PostCompact → context_compact

Payload：

```json
{
  "phase": "pre|post",
  "mode": "manual|auto"
}
```

### 9.7 SubagentStart / SubagentStop

转换成：

```text
subagent_start
subagent_stop
```

### 9.8 Stop → run_stop

Payload：

```json
{
  "turn_id": "...",
  "last_assistant_message": "按配置处理"
}
```

---

## 10. Git Snapshot 集成

在 PreToolUse：

```text
采集 before snapshot
```

在 PostToolUse：

```text
采集 after snapshot
写 patch artifact
payload.changed_files
```

注意：

- Git 采集失败不能导致 hook 失败。
- 非 git 目录可以继续记录事件。
- patch 大于 max_blob_bytes 时只保存 hash 和 truncated 标记。

---

## 11. CLI init 命令

文件：`internal/cli/init.go`

行为：

```bash
agent-vcr init codex
```

流程：

1. 通过 registry 获取 codex adapter。
2. 调用 adapter.Probe。
3. 调用 adapter.Install。
4. 创建 `.agent-vcr/config.yml`。
5. 更新 `.gitignore`。
6. 输出 next steps。

输出示例：

```text
Installed Codex hooks.
Next:
  1. Run `codex` in this repo.
  2. Open `/hooks` and trust the agent-vcr hook.
  3. Use Codex normally.
```

---

## 12. 测试要求

fixtures：

```text
internal/adapters/codex/testdata/session_start.json
internal/adapters/codex/testdata/user_prompt.json
internal/adapters/codex/testdata/pre_tool_bash.json
internal/adapters/codex/testdata/post_tool_bash.json
internal/adapters/codex/testdata/permission_request.json
internal/adapters/codex/testdata/stop.json
```

测试：

1. 每个 fixture normalize 成预期事件类型。
2. 未知事件 normalize 成 raw_event。
3. hook handler 不向 stdout 输出。
4. hook handler 遇到非法 JSON exit 0。
5. init codex 不覆盖用户已有 hook。
6. 重复 init 不重复添加 hook。

命令：

```bash
go test ./...
```

---

## 13. Codex 执行提示词

```text
请只实现 docs/03-codex-hook-adapter.md。
不要实现 replay、diff、check、HTML report、codex exec jsonl record。

硬性约束：
- Codex 代码只能放在 internal/adapters/codex。
- hook 命令默认 stdout 为空。
- hook 出错 exit 0。
- Normalize 失败落 raw_event。
- 不解析 transcript_path 指向的 transcript 文件。
- init codex 必须合并已有 hooks.json，不能粗暴覆盖。
- 增加 testdata fixture 和单元测试。
- 最后运行 go test ./...。
```

---

## 14. 完成标准

- `agent-vcr init codex` 能生成 `.codex/hooks.json`。
- `agent-vcr hook --adapter codex` 能从 stdin 读取 fixture 并写 trace。
- hook 出错不影响 Codex。
- 所有 Codex 原始事件都转成 normalized trace 或 raw_event。
- `go test ./...` 通过。
