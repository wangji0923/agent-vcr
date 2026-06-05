# agent-vcr Implementation Plan

This plan locks the shared interfaces before feature work starts. Subagents must not change the public shape or semantics of the locked interfaces unless the main controller explicitly approves a schema change.

## Locked Public Interfaces

- `internal/trace.Event`
- `internal/trace.EventType`
- `internal/trace.Source`
- `internal/trace.Payload`
- `internal/trace.RawEvent`
- `internal/trace.Metadata`
- `internal/trace.Store`
- `internal/adapters.Adapter`
- `internal/adapters.Capabilities`
- `internal/config.Config`

## Directory Ownership

| Subagent | Module | Owns | Depends on | Must not modify |
| --- | --- | --- | --- | --- |
| Bootstrap | `01-bootstrap` | `cmd/agent-vcr`, `internal/cli/root.go`, `internal/cli/version.go`, `internal/version`, README | locked interfaces | Codex adapter, trace schema fields, analysis/report/check |
| Config Trace Store | `02-config-trace-store` | `internal/config/load.go`, `internal/config/validate.go`, `internal/trace` store implementation, `internal/gitutil` | locked trace/config/adapters signatures | adapter registry semantics, Codex code, analysis/report/check |
| Codex Hook Adapter | `03-codex-hook-adapter` | `internal/adapters/codex`, `internal/cli/init.go`, `internal/cli/hook.go` | config, trace store, registry | analysis/report/check/trace schema fields |
| Record JSONL Generic | `04-record-jsonl-generic` | `internal/cli/record.go`, `internal/adapters/codex/jsonl.go`, `internal/adapters/generic`, `internal/process` | config, trace store, gitutil, registry | replay/diff/check/report, trace schema fields |
| Replay List | `05-replay-list` | `internal/cli/list.go`, `internal/cli/replay.go`, `internal/analysis/timeline.go`, `internal/analysis/replay.go` | trace store, metadata | any `internal/adapters/*`, Codex/generic adapter internals |
| Diff Check | `06-diff-check` | `internal/cli/diff.go`, `internal/cli/check.go`, `internal/analysis/diff.go`, `internal/analysis/check.go`, `internal/analysis/signature.go`, `internal/analysis/rules.go` | trace, config rules, analysis timeline | any `internal/adapters/*`, report |
| HTML Report Doctor | `07-html-report-doctor` | `internal/cli/export.go`, `internal/cli/redact.go`, `internal/cli/doctor.go`, `internal/report`, `internal/doctor`, `internal/redact` | trace, config, analysis, registry probe only | adapter internals, trace schema fields |
| Release | `08-release` | README, CHANGELOG, CONTRIBUTING, schema docs, CI/release config | all completed modules | core behavior, public interface signatures |

## Files With Locked Signatures

Feature subagents may add implementation code where their module owns it, but must not rename public types, remove fields, change JSON/YAML tags, or change method signatures in:

- `internal/trace/event.go`
- `internal/trace/raw_event.go`
- `internal/trace/artifact.go`
- `internal/trace/metadata.go`
- `internal/trace/store.go`
- `internal/adapters/adapter.go`
- `internal/adapters/capabilities.go`
- `internal/adapters/registry.go`
- `internal/config/config.go`

Only the main controller may approve changes to these files' public API. Module 02 may replace `trace.Store` stub bodies with real implementations without changing signatures.

## Dependency Rules

- `internal/trace` must not import `internal/adapters/*`.
- `internal/analysis` must not import `internal/adapters/*`.
- `internal/report` must not import `internal/adapters/*`.
- `internal/check` must not import `internal/adapters/*`.
- Codex-specific parsing, hook input types, and JSONL normalization must stay under `internal/adapters/codex`.
- Unknown or failed normalization must produce `trace.EventRaw` with `raw_ref`.
- Hook commands must write nothing to stdout by default and return exit code 0 on errors.
- `.agent-vcr/` must remain in `.gitignore`.

## Suggested Subagent Order

1. Bootstrap subagent for `01-bootstrap`.
2. Config Trace Store subagent for `02-config-trace-store`.
3. Codex Hook Adapter and Record JSONL Generic can start after module 02 is complete. They should not run concurrently if both need `internal/cli` command wiring.
4. Replay List can start after module 02 has stable read APIs.
5. Diff Check can start after Replay List has timeline helpers or can work directly from `trace.Event`.
6. HTML Report Doctor starts after Replay List and Diff Check.
7. Release starts last.

## Acceptance Commands

Each module must run at minimum:

```bash
go test ./...
go run ./cmd/agent-vcr --help
```

Final release verification must run:

```bash
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr doctor
```

## Required Subagent Report Format

```text
完成模块：<module>

修改文件：
- ...

实现内容：
- ...

测试：
- go test ./...：通过/失败
- 其他命令：...

未完成项：
- 无 / ...
```
