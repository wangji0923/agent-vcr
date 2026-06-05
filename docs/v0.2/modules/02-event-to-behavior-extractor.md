# 02 - Event to Behavior Extractor

## 目标

把 v0.1 的 normalized `trace.Event` 转换成 v0.2 的 `behavior.Step`。本模块只做事件抽取和基础转换，不做复杂命令分类细节；命令/路径分类依赖模块 03 的 classifier。

## 允许修改

```text
internal/behavior/extractor.go
internal/behavior/extractor_test.go
internal/behavior/load.go
internal/behavior/load_test.go
testdata/behavior/runs/**
```

## 依赖

依赖模块：

```text
01-behavior-domain-model
03-command-and-path-classifiers
```

## 禁止修改

```text
internal/adapters/**
internal/trace/event.go 既有语义
cmd/**
```

## 抽取输入

输入：

```go
type ExtractInput struct {
    RunID  string
    Events []trace.Event
}
```

输出：

```go
type ExtractResult struct {
    RunID string
    Steps []Step
    Warnings []string
}
```

## 抽取规则

### run_start / run_stop

可以转为：

```text
process_start / process_result
```

但 v0.2 的行为 diff 可以默认过滤这类噪声。

### user_prompt

默认不作为行为 step，避免 prompt 文本影响 first divergence。可以作为 metadata 或 attributes 留给 future。v0.2 不把 user_prompt 放入 BehaviorSignature 的关键路径。

### shell_command / shell_result

按 command 分类：

```text
rg/grep/find/fd       → search
cat/sed/head/tail     → read_file if path parseable
npm test/pytest/...   → run_test
npm run build/...     → run_build
npm install/...       → install_dependency
unknown shell         → call_tool
```

如果 `shell_command` 和 `shell_result` 可通过 event id、parent id、tool_use_id 合并，则结果写入同一个 step。

### tool_call / tool_result

根据 tool_name 和 payload 分类：

```text
mcp__*                 → call_mcp_tool
Bash/shell.exec        → 根据 command 分类
Read/Open/View         → read_file
Edit/Write/apply_patch → edit_file
unknown                → call_tool
```

### file_patch

转为：

```text
edit_file
```

### raw_event

转为：

```text
raw_behavior
```

或忽略。建议保守转 `raw_behavior`，但它默认不参与 first divergence。

## 合并策略

同一 tool call 的 pre/post 事件应尽量合并到一个 step。

优先关联字段：

```text
tool_use_id
call_id
span_id / parent_id
event parent-child
相邻 shell_command / shell_result
```

## 降级原则

```text
缺字段不 panic
无法分类 → call_tool 或 raw_behavior
无法解析路径 → 保留 command，但不填 Files
无法关联结果 → ResultUnknown
```

## 测试要求

```text
TestExtractShellSearch
TestExtractShellRunTestWithResult
TestExtractToolReadFile
TestExtractToolEditFile
TestExtractFilePatch
TestExtractMCPTool
TestExtractUnknownRawEvent
TestExtractorDoesNotPanicOnMissingFields
TestExtractorMergesToolCallAndResult
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

## Codex 执行提示词

```text
请只实现 02-event-to-behavior-extractor.md。使用已有 behavior.Step 类型和 command/path classifier，把 normalized trace.Event 转换成 behavior.Step。不要实现 CLI，不要实现 harness metadata，不要修改 adapters。补齐抽取测试后运行 go test ./internal/behavior/... 和 go test ./...。
```
