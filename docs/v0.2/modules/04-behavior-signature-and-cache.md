# 04 - Behavior Signature and Optional Cache

## 目标

基于抽取出的 `behavior.Step[]` 生成 `BehaviorSignature`。v0.2 初版默认按需计算，不强制落盘；可以预留 `--write-cache`，但不要让缓存成为必需。

## 允许修改

```text
internal/behavior/signature_builder.go
internal/behavior/signature_key.go
internal/behavior/signature_test.go
internal/behavior/cache.go       # 可选
internal/behavior/cache_test.go  # 可选
testdata/behavior/golden/signature/**
```

## 禁止修改

```text
internal/adapters/**
cmd/**，除非 07 模块接线
internal/trace 既有语义
```

## Signature Builder

建议接口：

```go
type BuildOptions struct {
    IncludeRawBehavior bool
    IncludeProcessNoise bool
}

func BuildSignature(runID string, events []trace.Event, opts BuildOptions) (Signature, error)
```

流程：

```text
trace.Event[]
  → ExtractSteps
  → FilterNoise
  → NormalizeSteps
  → ComputeMetrics
  → Signature
```

## Noise filtering

默认排除：

```text
process_start
process_result
raw_behavior
context_compact
```

但 JSON 输出可通过选项包含。

## Stable step key

每个 Step 生成稳定 key：

```text
kind + normalized action + normalized target + normalized files + normalized result
```

忽略：

```text
step_id
index
source_event_ids
timestamp
blob path
absolute user home path
```

## Source trace hash

可以基于 `trace.ndjson` 内容计算 SHA256，也可以基于 events JSON 计算。v0.2 初版允许为空，但如果实现，必须有测试。

## Optional cache

可选缓存文件：

```text
.agent-vcr/runs/<run-id>/behavior.json
```

注意：

```text
v0.2 不要求缓存
如果实现缓存，必须检查 trace hash，避免旧缓存误用
```

## 测试要求

```text
TestBuildSignatureFromEvents
TestSignatureFiltersProcessNoiseByDefault
TestSignatureCanIncludeRawBehavior
TestStableKeysIgnoreNonBehaviorNoise
TestSignatureJSONGolden
TestSignatureCacheInvalidatesOnTraceHashChange  # 如果实现缓存
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

## Codex 执行提示词

```text
请只实现 04-behavior-signature-and-cache.md。基于 ExtractSteps 生成 BehaviorSignature，默认不强制落盘。不要实现 CLI，不要实现 HarnessMetadata。补充 golden 测试后运行 go test ./internal/behavior/... 和 go test ./...。
```
