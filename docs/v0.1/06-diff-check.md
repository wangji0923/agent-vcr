# 06-diff-check.md

> 模块目标：实现 `agent-vcr diff` 与 `agent-vcr check`。
> 本模块只能基于 normalized trace.Event，不得依赖任何具体 adapter。

---

## 1. 本模块交付物

新增或修改：

```text
internal/cli/diff.go
internal/cli/check.go
internal/analysis/diff.go
internal/analysis/check.go
internal/analysis/signature.go
internal/analysis/rules.go
```

必须支持：

```bash
agent-vcr diff run-a run-b
agent-vcr diff run-a run-b --json
agent-vcr check latest
agent-vcr check latest --ci
agent-vcr check latest --json
```

---

## 2. Diff 目标

Diff 不做普通文本 diff，而是回答：

```text
两次 agent run 第一次从哪里开始不同？
```

核心输出：

```text
First divergence at event #7

Run A:
  Bash: rg "session" src tests

Run B:
  Bash: rg "cookie" src

Impact:
  Run B never inspected tests before editing auth logic.
```

MVP 不需要 LLM，总结基于规则模板。

---

## 3. Event Signature

文件：`internal/analysis/signature.go`

```go
type EventSignature struct {
    Type       trace.EventType
    ToolName   string
    Command    string
    ArgsHash   string
    ResultHash string
    FilesHash  string
    ExitCode   string
}

func Signature(event trace.Event) EventSignature
```

归一化规则：

```text
忽略 timestamp
忽略 duration
忽略 event_id
忽略 run_id
路径去除用户 home 前缀
大输出用 sha256
Bash command trim space
```

不同事件：

```text
user_prompt: 比 prompt_sha256
tool_call: 比 tool_name + args_hash
tool_result: 比 tool_name + result_hash + exit_code
process_result: 比 exit_code + changed_files hash
run_start/run_stop: 通常弱比较
raw_event: 比 raw_ref.sha256
```

---

## 4. Diff 算法

v0.1 实现简单版本：

1. 读取两个 run 的 trace。
2. 过滤低价值事件：metadata update、snapshot-only raw 等。
3. 转成 signatures。
4. 从头扫描，找到第一个 signature 不相等的位置。
5. 如果一方结束，报告 length divergence。
6. 输出前后上下文。

后续可升级 LCS，但 v0.1 不需要。

函数：

```go
type DiffResult struct {
    RunA string
    RunB string
    FirstDivergence *Divergence
    Summary DiffSummary
}

type Divergence struct {
    Index int
    EventA *trace.Event
    EventB *trace.Event
    Reason string
}

func DiffRuns(a, b RunData) DiffResult
```

---

## 5. Diff 输出

普通输出：

```text
Run A: 2026-06-04-good
Run B: 2026-06-04-bad

First divergence at normalized event #8

A:
  tool_call Bash: rg "session" src tests

B:
  tool_call Bash: rg "cookie" src

Summary:
  A tool calls: 12
  B tool calls: 18
  A changed files: 2
  B changed files: 7
```

JSON 输出：

```json
{
  "run_a": "...",
  "run_b": "...",
  "first_divergence": {
    "index": 8,
    "event_a": {...},
    "event_b": {...},
    "reason": "tool_call_signature_mismatch"
  },
  "summary": {...}
}
```

---

## 6. Check 目标

`check` 用规则检查一次 run 是否符合项目要求。

输入：

```text
trace.ndjson
metadata.json
.agent-vcr/config.yml rules
```

输出：

```text
pass/fail
violations
warnings
risk score
```

---

## 7. Rule 数据结构

文件：`internal/analysis/rules.go`

```go
type CheckResult struct {
    Passed     bool
    RiskScore  int
    Violations []Violation
    Warnings   []Violation
}

type Violation struct {
    RuleID   string
    Severity string
    Message  string
    EventID  string
    Path     string
}
```

Severity：

```text
info
warning
error
critical
```

---

## 8. v0.1 检查规则

### 8.1 forbidden_paths

如果 changed_files 命中：

```yaml
forbidden_paths:
  - ".env"
  - "secrets/**"
```

输出 critical。

### 8.2 required_commands

如果 run 修改了源代码，但没有运行 required command，输出 error。

```yaml
required_commands:
  - "npm test"
```

匹配策略：

```text
Bash command 包含 required command 字符串即可。
```

### 8.3 max_changed_files

如果最终 changed files 超过上限，输出 warning 或 error。

### 8.4 require_tests_after_source_change

如果 source_globs 命中但 test_globs 没有任何改动，也没有运行测试，输出 warning/error。

### 8.5 secret pattern

扫描：

```text
prompt payload
工具 input/output inline 内容
blob 中小于 max scan size 的文本
patch
```

发现 secret 输出 critical。

### 8.6 dangerous shell commands

默认规则：

```text
rm -rf /
sudo
curl ... | sh
chmod 777
mkfs
```

v0.1 只检查，不阻断。

---

## 9. Risk Score

MVP 简单计算：

```text
critical +40
error +25
warning +10
info +2
最高 100
```

Passed：

```text
无 critical 且无 error
```

---

## 10. check --ci

行为：

```text
Passed=true  → exit 0
Passed=false → exit 1
```

输出尽量简洁：

```text
Agent VCR check failed:
- [critical] Touched forbidden file .env
- [error] Modified source but did not run npm test
```

---

## 11. 测试要求

fixtures：

```text
testdata/traces/check_pass/
testdata/traces/check_forbidden_path/
testdata/traces/check_missing_test/
testdata/traces/check_dangerous_command/
testdata/traces/diff_good/
testdata/traces/diff_bad/
```

测试：

1. diff 能找到 first divergence。
2. diff 完全相同返回 no divergence。
3. diff 一方更长能报告 length divergence。
4. forbidden_paths 命中。
5. required_commands 缺失。
6. dangerous command 命中。
7. check --ci 失败 exit 1。
8. analysis 不 import adapters。

---

## 12. Codex 执行提示词

```text
请只实现 docs/06-diff-check.md。
不要实现 HTML report、doctor、release。

硬性约束：
- diff/check 只能读取 normalized trace.Event、metadata 和 config rules。
- internal/analysis 不得 import internal/adapters/*。
- 不要写 Codex 特例。
- diff 输出 first divergence。
- check 支持 --ci，并在失败时 exit 1。
- 添加 fixtures 和单元测试。
- 最后运行 go test ./...。
```

---

## 13. 完成标准

- `agent-vcr diff run-a run-b` 可输出 first divergence。
- `agent-vcr diff --json` 可解析。
- `agent-vcr check latest` 可输出 violations。
- `agent-vcr check latest --ci` 可用于 CI。
- 所有规则有测试。
- `go test ./...` 通过。
