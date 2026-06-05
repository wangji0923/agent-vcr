# 03 - Harness Store and Cache

## 目标

为每个 run 缓存 HarnessMetadata，避免每次 inspect/diff 都重新检测。

## 允许修改

```text
internal/harness/store.go
internal/harness/cache.go
internal/harness/*_test.go
testdata/harness/store/**
```

## 存储路径

建议：

```text
.agent-vcr/runs/<run-id>/harness/metadata.json
.agent-vcr/runs/<run-id>/harness/detect.log 或 warnings.json
```

## API

建议：

```go
type Store struct {
    ProjectDir string
}

func (s *Store) Load(runID string) (*HarnessMetadata, error)
func (s *Store) Save(runID string, meta *HarnessMetadata) error
func (s *Store) Exists(runID string) bool
func (s *Store) Inspect(ctx context.Context, runID string, opts InspectOptions) (*HarnessMetadata, error)
```

`Inspect` 行为：

- cache exists 且未要求 refresh：读 cache。
- cache missing 或 `--refresh`：重新 detect 并保存。
- detect warning 不应导致失败。

## 并发

- 写 `metadata.json` 使用 temp file + atomic rename。
- 读到损坏 JSON 要返回明确错误。
- 不应破坏已有 run artifact。

## 测试要求

- Save / Load roundtrip。
- Inspect cache hit。
- Inspect refresh。
- 损坏 cache 错误。
- concurrent save 基础测试。
- missing run 错误清晰。
- 不存在 harness dir 时自动创建。

## 验证命令

```powershell
gofmt -w .
go test ./internal/harness/...
go test ./...
go vet ./...
```
