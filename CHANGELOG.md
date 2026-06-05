# Changelog

All notable changes are tracked here.

## Unreleased

## 0.2.0

- Add `internal/behavior` domain model, event-to-behavior extraction, command
  classification, and path classification.
- Add `BehaviorSignature` generation and run-local signature cache support.
- Add first behavior divergence over behavior signatures.
- Add behavior metrics for context discipline, validation behavior, edit scope,
  tool efficiency, and recovery behavior.
- Add `agent-vcr behavior`, `agent-vcr behavior diff`, and
  `agent-vcr behavior metrics` commands with JSON output.
- Extend E2E coverage and docs for v0.2 behavior diff.
- Keep HarnessMetadata, HarnessDiff, Matrix Compare, Regression/baseline, LLM
  Explain, deterministic replay, and new adapters out of v0.2 scope.

## 0.1.0

- Reposition docs around Behavior Diff / Harness Diff.
- Add `docs/behavior-diff.md` and pairwise roadmap through v1.0.
- Clarify that v0.1 is trace foundation plus event-level behavior diff.
- Add release/docs/CI infrastructure for v0.1.
- Add GitHub Actions CI for formatting, tests, vet, build, help smoke, and E2E
  fixtures.
- Add local E2E fixture script for Codex hooks and generic CLI wrapper flows.
- Add release build script and release checklist.
- Expand README, adapter guide, and trace schema documentation.

- Codex-first MVP.
- Generic CLI wrapper recording.
- Normalized trace store under `.agent-vcr/runs/<run-id>/trace.ndjson`.
- List, replay, diff, and check commands.
