# 06 - HTML Visual Report

## 目标

生成离线可打开的行为可视化 HTML 报告。

## 允许修改

```text
internal/visualize/html.go
internal/visualize/templates/behavior_visual_report.html.tmpl
internal/visualize/html_test.go
testdata/golden/visualize/html/**
```

## 禁止修改

```text
internal/adapters/**
internal/report/** 既有 report 逻辑，除非抽取共享 helper
```

## 技术要求

- 使用 `html/template`。
- 静态 HTML，离线可打开。
- 不引入 npm / React / Vue / build pipeline。
- 可使用少量内联 JS 进行展开/折叠、过滤、高亮。
- 所有用户内容自动 escape。
- `--redacted` 或默认 privacy 模式下不得泄露 secret。

## 页面结构

### Header

- title
- generated_at
- run ids
- source / status

### Summary

- first divergence summary
- run count
- key metrics cards
- warnings

### Swimlane Timeline

- lanes 横向展示
- alignment rows 纵向展示
- divergence row 高亮
- gap 显示为 `—`
- step 可展开查看 details

### Path Graph

- 简单 SVG
- 节点为行为 step kind + target
- 边为顺序
- 多 run 用不同 class 或 data-run 标记

### File Access Compare

- 文件 x run 矩阵
- read/edit count
- first step
- last step

### Metrics Cards

- 每个 run 一个卡片组
- good/warn/bad/info 样式

### Raw References

- event ids
- artifact refs
- 不直接嵌入大 blob

## HTML 可用性

最小 CSS：

- 支持宽屏表格横向滚动。
- lane header sticky 可选。
- divergence 用醒目边框/背景。
- 文件路径可换行。

## 主要函数

```go
type HTMLOptions struct {
    Redacted bool
    Title    string
}

func RenderHTML(report *VisualReport, opts HTMLOptions) ([]byte, error)
```

## 测试

必须覆盖：

- HTML 生成。
- HTML 包含 lanes / divergence / file access / metrics。
- 用户内容 `<script>` 被 escape。
- redaction 后 secret 不出现。
- 缺 divergence 也能渲染。
- 单 run / 双 run / 三 run 都能渲染。

## 验收命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
```
