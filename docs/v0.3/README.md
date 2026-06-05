# agent-vcr v0.3 文档包

本包用于 v0.3 开发，范围严格限定为：

```text
HarnessMetadata
Pairwise HarnessDiff
Harness inspect / diff CLI
HarnessDiff + v0.2 BehaviorDiff 的轻量关联摘要
```

不包含：

```text
Matrix Compare
Regression / baseline
LLM Explain
新 agent adapter
deterministic replay
cloud dashboard
```

## 文件

- `v0.3-技术方案.md`
- `v0.3-实施方案.md`
- `modules/01-harness-domain-model.md`
- `modules/02-harness-detection-and-fingerprints.md`
- `modules/03-harness-store-and-cache.md`
- `modules/04-harness-diff-engine.md`
- `modules/05-behavior-impact-correlation.md`
- `modules/06-cli-integration-and-output.md`
- `modules/07-testing-docs-e2e.md`
- `modules/08-integration-review-release.md`

## 推荐执行顺序

```text
Round 0:
  01 单独执行，锁定 public types

Round 1:
  02 / 03 / 04 并行

Round 2:
  05 / 06 / 07 并行

Round 3:
  08 最终审查
```
