# 04 - File Access Compare

## 目标

从 behavior lanes 中提取每个 run 的文件访问与编辑行为，生成文件访问对比矩阵。

## 允许修改

```text
internal/visualize/file_access.go
internal/visualize/file_access_test.go
testdata/visualize/file_access/**
```

## 主要函数

```go
func BuildFileAccessCompare(lanes []BehaviorLane) FileAccessCompare
func ExtractFileUse(step VisualStep) []FileAction
```

## FileAction

```go
type FileAction struct {
    Path   string
    Action string // read | edit | other
    Step   int
}
```

## 行为映射

- `read_file`、`inspect_test` → read
- `edit_file` → edit
- `run_test` / `run_build` 若包含文件，一般不算 read/edit，可作为 other。
- unknown / call_tool 如果有 files，可标记 other。

## 路径标准化

要求：

- Windows `\` 转 `/`。
- 去掉本机绝对 repo root 前缀。
- 保留相对路径。
- 输出稳定排序。

## FileUse 统计

每个 run 每个文件统计：

- ReadCount
- EditCount
- FirstStep
- LastStep
- FirstAction
- LastAction

## 过滤策略

默认保留所有文件，但 HTML 层可折叠大列表。

后续可加：

```text
--file-filter all|changed|read|edited
```

v0.2.5 可先不实现该参数。

## 测试

必须覆盖：

- read_file 被统计为 read。
- inspect_test 被统计为 read。
- edit_file 被统计为 edit。
- 同一文件多次读写计数正确。
- Windows / Unix 路径标准化。
- 多 run 文件矩阵稳定排序。

## 验收命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
```
