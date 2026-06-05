# 07-html-report-doctor.md

> 模块目标：实现静态 HTML 报告导出、redacted export、doctor 诊断。
> 本模块只能基于 normalized trace、metadata、config、filesystem 状态，不得依赖具体 adapter 内部实现。

---

## 1. 本模块交付物

新增或修改：

```text
internal/cli/export.go
internal/cli/redact.go
internal/cli/doctor.go
internal/report/html.go
internal/report/templates/report.html.tmpl
internal/doctor/doctor.go
internal/redact/redact.go
internal/redact/patterns.go
```

必须支持：

```bash
agent-vcr export latest --html
agent-vcr export latest --html --redacted
agent-vcr redact latest
agent-vcr doctor
agent-vcr doctor --json
```

---

## 2. HTML Report 目标

生成一个可离线打开的单文件 HTML：

```text
.agent-vcr/runs/<run-id>/report.html
```

要求：

```text
不依赖外部 CDN
不上传任何数据
不需要启动服务端
默认对敏感信息脱敏
```

---

## 3. Report 数据模型

文件：`internal/report/html.go`

```go
type ReportData struct {
    Metadata    trace.Metadata
    Summary     ReportSummary
    Timeline    []analysis.TimelineItem
    CheckResult *analysis.CheckResult
    DiffContext any
    Events      []trace.Event
}

type ReportSummary struct {
    ToolCalls    int
    ChangedFiles []string
    Commands     []CommandSummary
    RiskScore    int
    Status       string
}
```

注意：ReportData 只能由 trace / analysis 生成，不能依赖 adapter。

---

## 4. HTML 页面结构

页面必须包含：

```text
1. Header：run_id / source / status / started_at / duration
2. Capability Panel：本次 run 的可观测能力
3. Risk Panel：check 结果和 risk score
4. Changed Files：最终变更文件
5. Commands：执行过的 shell/process 命令
6. Timeline：事件时间线
7. Event Detail：点击/展开查看 payload
8. Artifacts：stdout/stderr/blob/patch 链接
9. Raw Events：可选折叠区
```

MVP 可以不做复杂 JS，只用 `<details>`。

---

## 5. HTML 安全要求

必须：

```text
所有字符串用 html/template 自动 escape
不要把 blob 原文直接全部内联
不要执行 trace 里的任何 HTML/JS
默认 redacted export
```

如果 payload 里包含：

```text
<script>alert(1)</script>
```

报告里必须以文本展示，不能执行。

---

## 6. Redaction

文件：`internal/redact/redact.go`

函数：

```go
func ApplyToEvent(event trace.Event, cfg config.RedactConfig) trace.Event
func ApplyToBytes(data []byte, cfg config.RedactConfig) []byte
func RedactRun(projectDir, runID string, outputDir string) error
```

默认规则：

```text
API key
JWT
private key
.env path
id_rsa
cookie/token/password 字段
```

字段级处理：

```text
payload 中 key 包含 token/password/secret/key → mask
路径命中 redaction.paths → mask path 或只保留 basename hash
blob 扫描后写 redacted copy
```

---

## 7. redact 命令

```bash
agent-vcr redact latest
```

输出：

```text
.agent-vcr/runs/<run-id>-redacted/
```

要求：

- 不修改原 run。
- 复制 metadata/trace/blob/patch 时脱敏。
- 保留 sha256 时使用 redacted 后的 hash。
- report.html 默认使用 redacted copy。

---

## 8. export --html

```bash
agent-vcr export latest --html
agent-vcr export latest --html --redacted
```

行为：

- 默认输出到原 run dir 的 `report.html`。
- `--redacted` 时先创建 redacted copy，再生成 report。
- 输出路径打印到 stdout。

---

## 9. Doctor 目标

`agent-vcr doctor` 检查环境和配置。

必须检查：

```text
agent-vcr version
当前 cwd
是否 git repo
.agent-vcr/config.yml 是否存在
.agent-vcr/runs 是否可写
.codex/hooks.json 是否存在
agent-vcr hook 是否在 PATH
codex 是否在 PATH
Codex hook 是否包含 agent-vcr command
AGENTS.md 是否存在
analysis/report/check 是否 import adapters 违规
```

注意：Doctor 可以检查 Codex，但不能在 core 模块依赖 Codex。Codex 检查逻辑可以通过 adapter registry 的 Probe 实现。

---

## 10. doctor 输出

普通输出：

```text
Agent VCR Doctor

Core:
  version: 0.1.0-dev
  cwd: /repo
  git repo: yes
  config: .agent-vcr/config.yml
  runs dir writable: yes

Adapters:
  codex-hooks: found
  generic-cli: available

Codex:
  codex binary: /usr/local/bin/codex
  hooks.json: .codex/hooks.json
  agent-vcr hook installed: yes

Architecture:
  analysis imports adapters: no
  report imports adapters: no
  check imports adapters: no
```

JSON 输出：

```json
{
  "core": {...},
  "adapters": [...],
  "architecture": {...}
}
```

---

## 11. 架构违规检查

Doctor 要扫描：

```text
internal/analysis
internal/report
internal/check
internal/trace
```

如果出现：

```go
import ".../internal/adapters/..."
```

输出 error。

这条检查非常重要，用来保证后续扩展不大改框架。

---

## 12. 测试要求

fixtures：

```text
testdata/traces/report_basic/
testdata/traces/report_with_secret/
testdata/traces/report_with_xss_payload/
```

测试：

1. export 生成 report.html。
2. report 不包含未转义 `<script>`。
3. redacted export 不包含 secret。
4. redact 不修改原 run。
5. doctor --json 可解析。
6. doctor 能发现 architecture import 违规。
7. HTML golden 测试可稳定。

---

## 13. Codex 执行提示词

```text
请只实现 docs/07-html-report-doctor.md。
不要实现 release 自动化。

硬性约束：
- report 只能读取 normalized trace，不得依赖具体 adapter。
- HTML 必须离线可打开，不引入 CDN。
- 所有字符串必须 escape。
- redacted export 不得覆盖原 run。
- doctor 必须检查 analysis/report/check/trace 是否 import adapters。
- 添加 XSS 和 secret redaction 测试。
- 最后运行 go test ./...。
```

---

## 14. 完成标准

- `agent-vcr export latest --html` 生成 report。
- `agent-vcr export latest --html --redacted` 生成脱敏报告。
- `agent-vcr redact latest` 可生成 redacted run。
- `agent-vcr doctor` 可检查环境。
- HTML 不执行 trace 中的脚本。
- `go test ./...` 通过。
