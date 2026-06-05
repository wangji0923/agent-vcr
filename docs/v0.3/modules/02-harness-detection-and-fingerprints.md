# 02 - Harness Detection and Fingerprints

## 目标

从 run artifact、trace、repo 文件和 config 中生成 HarnessMetadata。

## 依赖

依赖模块 01 的 public types。

## 允许修改

```text
internal/harness/detect.go
internal/harness/detectors.go
internal/harness/fingerprint.go
internal/harness/*_test.go
testdata/harness/detect/**
```

## 禁止修改

```text
internal/adapters/**
internal/behavior/**
internal/cli/**
internal/trace/** 核心语义
```

## Detector 设计

建议接口：

```go
type DetectOptions struct {
    ProjectDir string
    RunID      string
    Refresh    bool
}

type Detector interface {
    Name() string
    Detect(ctx context.Context, input DetectInput) (*PartialMetadata, error)
}
```

也可以用简单函数组合，不必过度抽象。

## 检测来源

### 1. Run metadata

从 `.agent-vcr/runs/<run-id>/metadata.json` 获取：

- RunID
- Source
- GitSHA
- Branch
- Capabilities
- Summary
- Adapter name

### 2. Trace

从 `trace.ndjson` 获取：

- model
- prompt hash，如果存在
- permission_mode
- tool names
- MCP tool names
- context compact events
- run_start source

### 3. Repo files

检测并 hash：

```text
AGENTS.md
CLAUDE.md
.cursor/rules/**
.codex/hooks.json
.codex/config.toml
.agent-vcr/config.yml
package.json
go.mod
pyproject.toml
```

注意：

- 只保存 hash / size / exists。
- 不保存全文。
- 文件缺失不是错误。

### 4. Tools / MCP

从 trace 中抽取：

- tool_name 列表
- mcp tool name 列表
- tool set hash
- mcp set hash

## Fingerprint 要求

- hash 算法固定 sha256。
- 对 list 型数据先排序再 hash。
- 对路径做 normalize。
- Windows / Unix path 都要支持。

## 隐私要求

不得保存：

- prompt 原文
- API key
- .env 内容
- 用户 HOME 绝对路径
- tool schema 原文

## 测试要求

- AGENTS.md 存在/缺失测试。
- `.codex/hooks.json` hash 测试。
- tool names 排序稳定 hash 测试。
- MCP tools hash 测试。
- trace model/permission 提取测试。
- repo-relative path 测试。
- missing metadata 不 panic。
- 无 git repo 降级测试。

## 验证命令

```powershell
gofmt -w .
go test ./internal/harness/...
go test ./...
go vet ./...
```
