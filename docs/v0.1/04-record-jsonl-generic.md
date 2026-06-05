# 04-record-jsonl-generic.md

> 模块目标：实现 `agent-vcr record`，支持 Codex `exec --json` JSONL 捕获与任意 CLI agent 的 Generic Wrapper。
> 本模块禁止实现 replay / diff / check / HTML report。

---

## 1. 本模块交付物

新增或修改：

```text
internal/cli/record.go
internal/adapters/codex/jsonl.go
internal/adapters/generic/record.go
internal/adapters/generic/normalize.go
internal/process/runner.go
internal/process/stream.go
internal/process/result.go
```

必须支持：

```bash
agent-vcr record -- codex exec --json "fix bug"
agent-vcr record --name auth-fix -- codex exec --json "fix bug"
agent-vcr record -- some-agent "fix bug"
```

---

## 2. record 命令参数设计

```bash
agent-vcr record [flags] -- <command> [args...]
```

flags：

```bash
--name string          自定义 run 名称后缀
--adapter string       auto|codex-jsonl|generic-cli，默认 auto
--cwd string           子进程工作目录，默认当前目录
--capture-stdout bool  默认 true
--capture-stderr bool  默认 true
```

auto 识别规则：

```text
如果 command 是 codex 且 args 包含 exec 和 --json：使用 codex-jsonl
否则：使用 generic-cli
```

如果 command 是 codex exec 但没有 --json：

```text
v0.1 不自动插入 --json，给出提示：建议使用 codex exec --json。
仍然按 generic-cli 记录。
```

---

## 3. Process Runner

文件：`internal/process/runner.go`

```go
type RunOptions struct {
    Command    string
    Args       []string
    Cwd        string
    Env        []string
    StdoutMode string
    StderrMode string
}

type RunResult struct {
    ExitCode int
    StartedAt time.Time
    EndedAt time.Time
    StdoutRef *trace.ArtifactRef
    StderrRef *trace.ArtifactRef
}
```

要求：

- 使用 `exec.CommandContext`。
- stdout/stderr 分别 stream。
- 支持 JSONL line callback。
- 子进程退出码必须记录。
- SIGINT / SIGTERM 要转发给子进程。

---

## 4. Codex JSONL Adapter

### 4.1 输入

```bash
agent-vcr record -- codex exec --json "fix bug"
```

流程：

```text
1. CreateRun(source=codex-jsonl)
2. 写 process_start
3. spawn codex exec --json ...
4. 逐行读取 stdout
5. 每行 parse JSON
6. NormalizeCodexJSONL
7. append events
8. stderr 写 blob
9. 子进程结束
10. 采集 final git diff
11. 写 process_result + run_stop
```

### 4.2 去重环境变量

为了避免用户同时安装了 hooks 导致双重采集，启动子进程时设置：

```bash
AGENT_VCR_CAPTURE_MODE=jsonl
AGENT_VCR_RUN_ID=<run-id>
```

Codex hook handler 检测到：

```text
AGENT_VCR_CAPTURE_MODE=jsonl
```

应该 no-op。

### 4.3 JSONL Normalize

文件：`internal/adapters/codex/jsonl.go`

函数：

```go
func NormalizeJSONL(raw map[string]any) []trace.Event
```

规则：

```text
thread.started     → run_start 或 raw_event
turn.started       → user_prompt 或 raw_event
turn.completed     → run_stop 或 turn_stop
turn.failed        → tool_error / run_stop failed
item.*             → 根据 item type 转 model/tool/file/process 事件
error              → tool_error 或 raw_event
未知事件           → raw_event
```

因为 JSONL item 类型可能变化，v0.1 必须保留 raw_event。

### 4.4 测试 fixture

```text
internal/adapters/codex/testdata/jsonl_thread_started.jsonl
internal/adapters/codex/testdata/jsonl_tool_call.jsonl
internal/adapters/codex/testdata/jsonl_file_change.jsonl
internal/adapters/codex/testdata/jsonl_error.jsonl
```

---

## 5. Generic CLI Adapter

### 5.1 输入

```bash
agent-vcr record -- some-agent "fix bug"
```

流程：

```text
1. CreateRun(source=generic-cli)
2. 采集 before git snapshot
3. 写 process_start
4. spawn 子进程
5. stdout 写 blobs/process_stdout.txt
6. stderr 写 blobs/process_stderr.txt
7. 子进程结束
8. 采集 after git snapshot
9. 写 final.diff
10. 写 process_result
11. 写 run_stop
```

### 5.2 Generic 能力声明

```go
Capabilities{
    PromptCapture:      false,
    ToolCallCapture:    false,
    ToolResultCapture:  false,
    ShellCapture:       false,
    FileDiffCapture:    true,
    CanRunAsWrapper:    true,
}
```

### 5.3 Generic trace 事件

至少产生：

```text
run_start
process_start
process_result
run_stop
```

process_start payload：

```json
{
  "command": "some-agent",
  "args": ["fix bug"],
  "cwd": "/repo"
}
```

process_result payload：

```json
{
  "exit_code": 0,
  "duration_ms": 12345,
  "changed_files": ["src/a.ts"],
  "stdout_blob": "blobs/process_stdout.txt",
  "stderr_blob": "blobs/process_stderr.txt",
  "final_diff_blob": "patches/final.diff"
}
```

---

## 6. Git before/after snapshot

Generic CLI 必须采集：

```text
before snapshot
after snapshot
final diff
changed files
```

如果不是 git repo：

```text
继续记录 process_start/process_result
metadata 中 repo_root 为空
report 中显示 no git repo
```

---

## 7. 错误处理

### 7.1 子进程不存在

返回非 0，写 run_stop failed。

### 7.2 JSONL 某行解析失败

写 raw_event，不中断进程。

### 7.3 blob 超过上限

根据 config：

```text
超过 max_blob_bytes 后截断，并设置 truncated=true
```

### 7.4 用户 Ctrl+C

转发信号给子进程，并记录 process_result。

---

## 8. 测试要求

测试：

1. `record -- echo hello` 生成 run。
2. stdout 写 blob。
3. 子进程 exit code 记录。
4. 非 0 退出码记录为 failed。
5. codex JSONL fixture normalize 成事件。
6. JSONL 非法行变 raw_event。
7. Generic CLI 在非 git 目录不失败。
8. `AGENT_VCR_CAPTURE_MODE=jsonl` 被设置。

命令：

```bash
go test ./...
```

---

## 9. Codex 执行提示词

```text
请只实现 docs/04-record-jsonl-generic.md。
不要实现 replay、list、diff、check、HTML report。

硬性约束：
- codex JSONL 逻辑只能放在 internal/adapters/codex/jsonl.go。
- generic CLI 逻辑只能放在 internal/adapters/generic。
- record 命令通过 adapter registry 选择 adapter，不要在核心分析层写 Codex 逻辑。
- JSONL 未知事件必须落 raw_event。
- 子进程 stdout/stderr 必须写 blob，不要全部塞进 trace.ndjson。
- 最后运行 go test ./...。
```

---

## 10. 完成标准

- `agent-vcr record -- echo hello` 能生成 run。
- `agent-vcr record -- codex exec --json "..."` 能读取 JSONL。
- 运行结果有 metadata、trace.ndjson、blobs、patches。
- Generic CLI 不依赖 Codex。
- `go test ./...` 通过。
