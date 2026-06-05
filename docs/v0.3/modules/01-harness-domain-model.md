# 01 - Harness Domain Model

## 目标

建立 v0.3 的公共模型层，先锁定 HarnessMetadata / HarnessDiff / Fingerprint 类型，供后续模块并行开发。

## 允许修改

```text
internal/harness/**
testdata/harness/model/**
```

## 禁止修改

```text
internal/adapters/**
internal/behavior/**
internal/trace/** 核心语义
internal/analysis/**
internal/check/**
internal/report/**
```

## 新增类型

建议创建：

```text
internal/harness/model.go
internal/harness/fingerprint.go
internal/harness/errors.go
```

核心类型：

```go
type HarnessMetadata struct
type RepoFingerprint struct
type ModelFingerprint struct
type AdapterFingerprint struct
type PromptFingerprint struct
type RulesFingerprint struct
type ToolsFingerprint struct
type MCPFingerprint struct
type PermissionFingerprint struct
type SandboxFingerprint struct
type ContextFingerprint struct
type ConfigFingerprint struct
type HashRef struct
type FileFingerprint struct
type HarnessWarning struct
type HarnessDiff struct
type HarnessField struct
type HarnessFieldDiff struct
type HarnessMissing struct
type BehaviorImpact struct
```

## 设计原则

- 所有结构必须 JSON serializable。
- 字段命名稳定。
- 默认存 hash，不存原文。
- 支持 missing / warning。
- 不包含 agent-specific 字段。
- 不依赖 adapters。

## HashRef

```go
type HashRef struct {
    Algorithm string `json:"algorithm"`
    Value     string `json:"value"`
    Source    string `json:"source,omitempty"`
}
```

## FileFingerprint

```go
type FileFingerprint struct {
    Path      string  `json:"path"`
    Exists    bool    `json:"exists"`
    Hash      HashRef `json:"hash,omitempty"`
    SizeBytes int64   `json:"size_bytes,omitempty"`
}
```

路径必须是 repo-relative，不能保存用户绝对路径。

## 测试要求

- HarnessMetadata JSON roundtrip。
- HarnessDiff JSON roundtrip。
- HashRef empty/non-empty 测试。
- FileFingerprint repo-relative path 测试。
- agent-specific 字段/分类 guard 测试。
- architecture test：`internal/harness` 不 import `internal/adapters`。

## 验证命令

```powershell
gofmt -w .
go test ./internal/harness/...
go test ./...
go vet ./...
```

## 输出

完成后输出：

```text
## 01-harness-domain-model 完成情况

### 修改文件
### 公共接口
### 测试结果
### NEEDS_INTEGRATION
```
