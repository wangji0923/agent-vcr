# 07 - CLI Integration

## 目标

新增 `agent-vcr visualize` 命令，将可视化报告接入 CLI。

## 允许修改

```text
internal/cli/visualize.go
cmd/agent-vcr 相关命令注册
internal/visualize/json.go
internal/visualize/cli_test.go
README.md 命令示例小幅更新
```

## CLI 设计

```bash
agent-vcr visualize <run-id|latest> [run-id...] --json
agent-vcr visualize <run-id|latest> [run-id...] --html --output behavior.html
```

## 参数

```text
--json              输出 VisualReport JSON
--html              输出静态 HTML
--output <path>     输出文件路径，默认 stdout 或 auto path
--label <k=v>       指定 run label，可重复
--max-runs <n>      默认 5
--no-cache          从 trace 重建 behavior
--baseline <run-id> 指定 baseline，默认第一个 run
--redacted          输出脱敏 HTML/JSON
```

## 参数规则

- 至少一个 run id。
- run id 支持 `latest`、精确 id、前缀匹配，复用现有 resolver。
- 多 run 数量超过 max-runs 时返回错误。
- `--json` 和 `--html` 二选一；如果都不传，默认 human summary 或提示用户传 `--html/--json`。
- `--output` 仅对 `--html` 必需或推荐；若 `--json --output` 也可支持。

## 输出策略

### JSON

stdout 只能输出 JSON，不允许混入日志。

### HTML

若有 `--output`：

```text
write file and print path
```

若没有 `--output`：

```text
默认写到 .agent-vcr/runs/<first-run>/visual/compare-<timestamp>.html
```

或者返回错误要求用户指定。实现上推荐默认输出路径，用户体验更好。

## CLI 示例

```bash
agent-vcr visualize latest --html
agent-vcr visualize run-a run-b --html --output compare.html
agent-vcr visualize run-a run-b run-c --json
agent-vcr visualize run-a run-b --html --label run-a=default --label run-b=strict
```

## 测试

必须覆盖：

- `visualize --help`
- `visualize latest --json` 输出可解析
- `visualize run-a run-b --html --output file` 生成文件
- run id 不存在错误清晰
- 超过 max-runs 错误清晰
- `--json` 不混入非 JSON 文本
- `--label` 生效

## 验收命令

```powershell
gofmt -w .
go test ./internal/cli/... ./internal/visualize/...
go test ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr visualize --help
```
