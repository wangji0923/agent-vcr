# agent-vcr v0.2 文档包

本包包含 agent-vcr v0.2 的技术方案与分模块实施方案。v0.2 只聚焦一个目标：

> 从 v0.1 的 event-level diff 升级为 BehaviorSignature + First Behavior Divergence。

## 文件结构

```text
agent-vcr-v0.2-docs/
  v0.2-技术方案.md
  v0.2-实施方案.md
  modules/
    01-behavior-domain-model.md
    02-event-to-behavior-extractor.md
    03-command-and-path-classifiers.md
    04-behavior-signature-and-cache.md
    05-first-behavior-divergence.md
    06-behavior-metrics.md
    07-cli-integration-and-output.md
    08-testing-e2e-release.md
```

## v0.2 不做什么

v0.2 不实现 HarnessMetadata、HarnessDiff、Matrix Compare、Regression、PR Risk、LLM Explain、云端 Dashboard、deterministic replay，也不新增 Claude/Kimi/MCP adapter。v0.2 的任务是把已有 normalized trace 转成行为步骤，并做 pairwise 行为差异分析。
