# Contributing

agent-vcr is organized around strict adapter boundaries. Before changing code,
read `AGENTS.md`, `IMPLEMENTATION_PLAN.md`, and the module document for the
current task.

## Development Commands

```bash
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
powershell -ExecutionPolicy Bypass -File ./scripts/e2e.ps1
```

Format Go files before sending changes:

```bash
gofmt -w .
```

## Contribution And Merge Policy

Anyone may open a pull request. External contributors should use forks; write
access is reserved for trusted maintainers.

The `main` branch is protected:

- changes must go through pull requests
- CI must pass before merge
- direct pushes to `main` are not allowed for normal development
- force pushes and branch deletion are not allowed

For the early project phase, maintainers merge PRs manually with squash merge.
Merge commits and rebase merges are disabled to keep the public history easy to
read. When the project has multiple active maintainers, required approvals and
CODEOWNERS reviews can be tightened.

Only maintainers may merge pull requests. Maintainer access should be granted
slowly and only to contributors who repeatedly make well-scoped changes, follow
the adapter boundaries, write tests, and review other pull requests carefully.

## Architecture Rules

- Codex logic belongs in `internal/adapters/codex`.
- Adapters emit normalized `trace.Event` values.
- Shared analysis, report, check, and trace packages must not import adapter
  packages.
- Normalize failures must produce `raw_event` with `raw_ref`.
- Hook commands must not write to stdout by default and must exit 0 on errors.

## Tests

Prefer focused tests:

- Unit tests for core behavior.
- Fixture tests for parser and adapter input.
- Golden tests for normalized output.
- Architecture tests for import boundaries.
- E2E fixtures for CLI flows.

## Privacy

Do not add network upload, extra LLM calls, cloud behavior, deterministic
replay, batch benchmark runners, or matrix comparison features without an
explicit roadmap decision. The near-term product direction is pairwise Behavior
Diff / Harness Diff. Keep `.agent-vcr/` out of git. Treat trace files and report
output as sensitive unless exported with redaction.
