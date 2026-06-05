# 01-bootstrap.md

> 模块目标：初始化 agent-vcr 项目骨架、CLI 根命令、目录结构、基础测试和 Codex 长期约束文件。
> 本模块禁止实现 Codex Adapter、Trace Store、Replay、Diff、Check、HTML Report。

---

## 1. 输入前置条件

执行本模块前，仓库可以是空仓库，也可以只有 README。

要求：

```text
Go 1.23+
git 可用
当前目录是 git repo
```

检查命令：

```bash
go version
git status
```

---

## 2. 本模块交付物

必须创建：

```text
go.mod
cmd/agent-vcr/main.go
internal/cli/root.go
internal/cli/version.go
internal/adapters/adapter.go
internal/adapters/capabilities.go
internal/adapters/registry.go
internal/version/version.go
AGENTS.md
README.md 最小版
```

必须支持命令：

```bash
agent-vcr --help
agent-vcr version
agent-vcr doctor
```

注意：`doctor` 本模块只做占位，输出基础环境，不做 Codex hooks 检查。

---

## 3. 初始化 Go 项目

执行：

```bash
go mod init github.com/<your-org>/agent-vcr
```

如果暂时没有 org，可以用：

```bash
go mod init github.com/agent-vcr/agent-vcr
```

安装依赖：

```bash
go get github.com/spf13/cobra@latest
go get gopkg.in/yaml.v3@latest
go mod tidy
```

v0.1 推荐先只引入：

```text
cobra：CLI 命令
yaml.v3：后续配置文件
```

文件锁、glob、HTML report 相关依赖后续模块再引入。

---

## 4. 创建目录结构

执行：

```bash
mkdir -p cmd/agent-vcr
mkdir -p internal/cli
mkdir -p internal/version
mkdir -p internal/adapters
mkdir -p docs
mkdir -p testdata
```

---

## 5. 实现版本模块

文件：`internal/version/version.go`

```go
package version

var (
    Version = "0.1.0-dev"
    Commit  = "unknown"
    Date    = "unknown"
)
```

验收：后续 `agent-vcr version` 能读取这些变量。

---

## 6. 实现 Adapter 基础接口

文件：`internal/adapters/adapter.go`

```go
package adapters

import "context"

type RawEvent struct {
    Adapter string
    Source  string
    Data    []byte
}

type ProbeResult struct {
    Found   bool
    Version string
    Details map[string]string
}

type InstallOptions struct {
    Scope      string
    ProjectDir string
    Force      bool
}

type Adapter interface {
    Name() string
    DisplayName() string
    Probe(ctx context.Context) (*ProbeResult, error)
    Install(ctx context.Context, opts InstallOptions) error
    Uninstall(ctx context.Context, opts InstallOptions) error
    Capabilities() Capabilities
}
```

注意：本模块还没有 trace.Event，所以 `Normalize` 先不要放进接口，等 `02-config-trace-store.md` 实现 trace 包后再补。

---

## 7. 实现 Capabilities

文件：`internal/adapters/capabilities.go`

```go
package adapters

type Capabilities struct {
    PromptCapture      bool `json:"prompt_capture"`
    ModelCallCapture   bool `json:"model_call_capture"`
    ModelResultCapture bool `json:"model_result_capture"`
    ToolCallCapture    bool `json:"tool_call_capture"`
    ToolResultCapture  bool `json:"tool_result_capture"`
    ShellCapture       bool `json:"shell_capture"`
    FileDiffCapture    bool `json:"file_diff_capture"`
    PermissionCapture  bool `json:"permission_capture"`
    SubagentCapture    bool `json:"subagent_capture"`
    MCPToolCapture     bool `json:"mcp_tool_capture"`
    CanInstallHooks    bool `json:"can_install_hooks"`
    CanRunAsWrapper    bool `json:"can_run_as_wrapper"`
    CanImportTrace     bool `json:"can_import_trace"`
    CanIngestHTTP      bool `json:"can_ingest_http"`
}
```

---

## 8. 实现 Adapter Registry

文件：`internal/adapters/registry.go`

要求：

```go
func Register(adapter Adapter)
func Get(name string) (Adapter, bool)
func List() []Adapter
```

必须具备：

- 注册重复 name 时 panic，开发期及早发现。
- `List()` 输出按 name 排序，方便测试稳定。
- 不要在 registry 里 import 具体 adapter，具体 adapter 由后续模块在 init 或 CLI 注册。

测试文件：`internal/adapters/registry_test.go`

测试点：

1. 注册后能 Get。
2. List 顺序稳定。
3. 重复注册会 panic。

---

## 9. 实现 CLI Root

文件：`internal/cli/root.go`

要求：

```go
func NewRootCommand() *cobra.Command
```

支持：

```bash
agent-vcr --help
agent-vcr version
agent-vcr doctor
```

Root command 描述：

```text
Behavior diff for AI coding agents.
```

全局 flags 预留：

```bash
--project-dir string
--config string
--json
--verbose
```

本模块只解析，不需要完整使用。

---

## 10. 实现 main.go

文件：`cmd/agent-vcr/main.go`

```go
package main

import (
    "os"
    "github.com/agent-vcr/agent-vcr/internal/cli"
)

func main() {
    cmd := cli.NewRootCommand()
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

---

## 11. 实现 version 命令

文件：`internal/cli/version.go`

输出普通文本：

```text
agent-vcr 0.1.0-dev commit=unknown date=unknown
```

如果 `--json`：

```json
{"version":"0.1.0-dev","commit":"unknown","date":"unknown"}
```

---

## 12. 实现 doctor 占位

文件：`internal/cli/doctor.go`

本模块只输出：

```text
agent-vcr doctor
- go runtime: ...
- cwd: ...
- git repo: yes/no
```

不要检查 Codex。

---

## 13. 生成 AGENTS.md

文件：`AGENTS.md`

必须包含：

```text
所有 adapter 必须输出 normalized trace.Event。
analysis/report/check/store 模块不得 import internal/adapters/*。
Codex 相关逻辑只能存在于 internal/adapters/codex。
每次只实现一个模块。
go test ./... 必须通过。
hook 命令默认不得向 stdout 输出内容。
hook 出错必须 exit 0。
```

本文件也会单独提供给用户下载。

---

## 14. README 最小版

文件：`README.md`

必须包含：

```text
agent-vcr
Behavior diff for AI coding agents.

Same model. Same task. Different harness.
agent-vcr records normalized traces and shows where agent behavior diverged.
```

命令示例先只写：

```bash
agent-vcr --help
agent-vcr version
agent-vcr doctor
```

---

## 15. 测试要求

执行：

```bash
go test ./...
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr version
go run ./cmd/agent-vcr doctor
```

必须通过。

---

## 16. Codex 执行提示词

把下面 prompt 给 Codex：

```text
请只实现 docs/01-bootstrap.md 的内容。
不要实现 Codex hook、Trace Store、Replay、Diff、Check、HTML report。

硬性约束：
- 只创建本模块列出的文件。
- CLI 使用 cobra。
- 实现 Adapter Registry 和 Capabilities。
- 生成 AGENTS.md。
- 添加 registry 单元测试。
- 最后运行 go test ./...、go run ./cmd/agent-vcr --help、go run ./cmd/agent-vcr version、go run ./cmd/agent-vcr doctor。
- 输出修改了哪些文件、测试结果、未完成项。
```

---

## 17. 完成标准

本模块完成必须满足：

- `go test ./...` 通过。
- `agent-vcr --help` 可运行。
- `agent-vcr version` 可运行。
- `agent-vcr doctor` 可运行。
- `internal/adapters` 中没有具体 Codex 逻辑。
- `AGENTS.md` 已创建。
