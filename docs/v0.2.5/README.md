# agent-vcr v0.2.5 文档包

本包用于 v0.2.5 开发，范围严格限定为 Behavior Visualization：

```text
Single-run behavior visualization
Pairwise behavior path comparison
Small multi-run swimlane alignment
File access / edit comparison
Metrics cards
Offline HTML visual report
agent-vcr visualize CLI
```

不包含：

```text
新的分析理论
HarnessMetadata / HarnessDiff
Matrix Compare
Regression / baseline
LLM Explain
新 agent adapter
deterministic replay
cloud dashboard
```

## 文件

- `v0.2.5-技术方案.md`
- `v0.2.5-实施方案.md`
- `modules/01-visual-domain-model.md`
- `modules/02-visual-data-loader.md`
- `modules/03-swimlane-alignment.md`
- `modules/04-file-access-compare.md`
- `modules/05-metrics-cards.md`
- `modules/06-html-visual-report.md`
- `modules/07-cli-integration.md`
- `modules/08-testing-e2e-docs.md`

## 推荐执行顺序

```text
Round 0:
  01 单独执行，锁定 visual public types

Round 1:
  02 / 03 / 04 / 05 并行

Round 2:
  06 / 07 并行

Round 3:
  08 最终验收
```

## 版本定位

v0.2.5 不改变 v0.2 的 BehaviorSignature / Divergence / Metrics 语义，只把已有行为数据转成更容易检查和比较的离线可视化报告。
