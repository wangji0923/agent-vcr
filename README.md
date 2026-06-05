# agent-vcr

Behavior diff for AI coding agents.

Same model. Same task. Different harness.
agent-vcr records normalized traces and shows where agent behavior diverged.

Git diff shows what changed in code.
agent-vcr diff shows how the agent got there.

Use it to compare:

- different prompt scaffolds
- different AGENTS.md files
- different tool sets
- different MCP configs
- different permission modes
- different agent harnesses

The v0.1 release is the trace foundation: Codex hooks, Codex JSONL, Generic CLI
recording, normalized traces, list/replay, event-level diff, check, export, and
doctor. The core value is not "watching the process" by itself; it is preserving
enough behavior evidence to compare two agent runs.

The v0.2 line adds `BehaviorSignature`: normalized trace events are summarized
into behavior steps, behavior metrics, and first behavior divergence between two
runs. v0.2 does not implement HarnessDiff, Matrix Compare, Regression/baseline,
LLM explanations, or deterministic replay.

## Install

From a checkout:

```bash
go build -o ./bin/agent-vcr ./cmd/agent-vcr
./bin/agent-vcr --help
```

When release binaries are available, download the archive for your platform,
put `agent-vcr` on `PATH`, and verify it:

```bash
agent-vcr version
agent-vcr doctor
```

## Quick Start

```bash
cd your-repo
agent-vcr init codex
codex

agent-vcr list
agent-vcr replay latest
agent-vcr diff <run-a> <run-b>
agent-vcr behavior latest
agent-vcr behavior diff <run-a> <run-b>
agent-vcr check latest
```

Export a portable offline HTML artifact when you need to share or inspect a run:

```bash
agent-vcr export latest --html
agent-vcr export latest --html --redacted
```

## Codex Usage

Install project-local Codex hooks:

```bash
cd your-repo
agent-vcr init codex
```

Then run Codex in that repo. The hook command reads Codex hook JSON from stdin,
writes normalized events under `.agent-vcr/runs/`, and must stay quiet on stdout
so it does not block the agent workflow.

Useful commands after a Codex run:

```bash
agent-vcr list
agent-vcr replay latest --filter tool
agent-vcr diff <run-a> <run-b>
```

## Generic CLI Usage

Wrap any local CLI command:

```bash
agent-vcr record -- some-agent "fix the failing test"
agent-vcr record --adapter generic-cli --name smoke -- go test ./...
```

The generic adapter captures process start/result, stdout/stderr blobs when
enabled, exit code, and git working tree changes. It does not understand that
agent's private internal transcript, but it can still produce behavior trace
artifacts for pairwise comparison.

## Behavior Diff Commands

```bash
agent-vcr list
agent-vcr replay latest
agent-vcr replay latest --json
agent-vcr replay latest --filter shell

agent-vcr diff <run-a> <run-b>
agent-vcr diff <run-a> <run-b> --json

agent-vcr behavior latest
agent-vcr behavior latest --json
agent-vcr behavior diff <run-a> <run-b>
agent-vcr behavior diff <run-a> <run-b> --json
agent-vcr behavior metrics latest

agent-vcr check latest
agent-vcr check latest --ci
agent-vcr check latest --json

agent-vcr export latest --html
agent-vcr export latest --html --redacted
```

Replay is an inspection timeline, not deterministic re-execution. Diff compares
normalized event signatures, changed files, tool sequences, and command exit
codes. Check applies local heuristic rules; it is not a complete security
firewall.

## v0.2 BehaviorSignature

BehaviorSignature is the v0.2 behavior layer above normalized traces. It turns
events into stable behavior steps such as search, read_file, edit_file,
run_test, run_build, call_tool, call_mcp_tool, recover_from_error, and unknown.

Use it when you want to compare how two runs behaved before looking at the final
code diff:

```bash
agent-vcr behavior latest
agent-vcr behavior latest --json
agent-vcr behavior metrics latest
agent-vcr behavior diff <run-a> <run-b>
agent-vcr behavior diff <run-a> <run-b> --json
```

`agent-vcr diff` remains the v0.1 event-level diff. `agent-vcr behavior diff`
is the v0.2 behavior-level diff based on BehaviorSignature and first behavior
divergence.

## Adapter Architecture

Hard boundaries:

- Codex-specific logic lives only in `internal/adapters/codex`.
- Every adapter emits normalized `trace.Event` values.
- `internal/analysis`, `internal/report`, `internal/check`, and
  `internal/trace` do not import `internal/adapters/*`.
- Normalize failures produce `raw_event` with `raw_ref`; they must not panic or
  drop data.
- New adapters should not require replay/diff/check changes unless a new
  generic event type is added.

See [docs/behavior-diff.md](docs/behavior-diff.md),
[docs/v0.2-behavior-signature.md](docs/v0.2-behavior-signature.md),
[docs/adapter-development.md](docs/adapter-development.md), and
[docs/trace-schema.md](docs/trace-schema.md).

## Privacy And Redaction

agent-vcr is local-first. Core recording does not upload data and does not call
extra LLM APIs.

v0.1 uses a privacy-first default:

- run storage is `.agent-vcr/runs`
- `.agent-vcr/` should remain in `.gitignore`
- Codex adapter output stores prompt/tool content as redacted or hashed
  summaries where applicable
- full capture policy controls are planned for future versions

Traces can still contain prompts, file paths, command summaries, patches, and
stdout/stderr blobs. Default config enables best-effort redaction for common
secrets such as `.env` values, API keys, private keys, and JWT-like tokens.

Before sharing output, prefer:

```bash
agent-vcr export latest --html --redacted
```

agent-vcr records observable hooks, wrapper process data, and local artifacts. It
does not record hidden model reasoning or private model-side state that the
agent surface does not expose.

## Current v0.2 Boundary

- v0.1 is the Behavior Diff trace foundation.
- v0.2 adds BehaviorSignature, behavior timelines, first behavior divergence,
  and behavior metrics.
- Codex hooks are the first native adapter.
- Codex `exec --json` is supported as a structured stream adapter.
- Generic CLI wrapper support is process-level and agent-agnostic.
- Replay is a timeline view, not deterministic execution.
- `diff` is event-level diff.
- `behavior diff` is behavior-level diff over BehaviorSignature.
- Check is local heuristic policy evaluation, not a complete security firewall.
- HTML export is an artifact view, not the main product surface.
- No batch matrix runner, benchmark platform, cloud dashboard, remote upload, or
  LLM explanation layer.
- Additional agents need adapters; they are not all deeply supported out of the
  box.

## Roadmap

### v0.1: Trace Foundation + Event-level Diff

- Codex hook adapter
- Codex JSONL adapter
- Generic CLI adapter
- normalized trace
- list / replay
- event-level diff
- check
- export
- doctor

### v0.2: BehaviorSignature + First Behavior Divergence

Implemented in v0.2:

- `behavior.Step`
- `BehaviorSignature`
- behavior timeline
- first behavior divergence
- behavior metrics for context discipline, validation behavior, edit scope, tool
  efficiency, and recovery behavior

Commands:

```bash
agent-vcr behavior latest
agent-vcr behavior diff run-a run-b
agent-vcr behavior metrics latest
```

### v0.3: HarnessMetadata + Pairwise HarnessDiff

Planned for v0.3, not implemented in v0.2:

- model
- prompt hash
- AGENTS.md hash
- tool schema hash
- MCP config hash
- permission mode
- sandbox mode
- context policy
- adapter version

Example planned commands:

```bash
agent-vcr harness inspect latest
agent-vcr harness diff run-a run-b
```

### v0.4: Pairwise Compare Report

Planned after v0.3, not implemented in v0.2:

- harness changes
- first behavior divergence
- behavior metrics
- validation behavior
- edit scope
- outcome difference
- likely behavior impact

Example planned command:

```bash
agent-vcr compare run-a run-b
```

### v0.5: Capture Completeness + Selected Adapters / SDK

The goal is better comparable behavior capture, not broad adapter count:

- more complete Codex tool classification
- Read/Edit/Search/Test behavior recognition
- MCP tool behavior classification
- selected Claude Code and Kimi Code adapters
- minimal Python and TypeScript SDKs
- HTTP ingest

### v1.0: Stable Protocol

- Trace Schema v1
- BehaviorSignature v1
- HarnessMetadata v1
- Adapter API v1
- CLI command format
- SDK event protocol

## Release

Local verification:

```bash
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr --version
go run ./cmd/agent-vcr doctor
powershell -ExecutionPolicy Bypass -File ./scripts/e2e.ps1
```

Release checklist: [docs/release-checklist.md](docs/release-checklist.md).

Manual release builds:

```bash
powershell -ExecutionPolicy Bypass -File ./scripts/build-release.ps1 -Version 0.2.0 -Clean
```
