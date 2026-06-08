# v0.2 / v0.2.5 Release Checklist

Use this before tagging a v0.2 or v0.2.5 release.

## Local Verification

```bash
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr behavior --help
go run ./cmd/agent-vcr visualize --help
powershell -ExecutionPolicy Bypass -File ./scripts/e2e.ps1
powershell -ExecutionPolicy Bypass -File ./scripts/e2e-v0.2.5.ps1
```

Also run the report/export path when the report module is present:

```bash
go run ./cmd/agent-vcr export latest --html --redacted
go run ./cmd/agent-vcr doctor
```

## CI

- CI workflow exists under `.github/workflows/ci.yml`.
- CI checks `gofmt`, `go test ./...`, `go vet ./...`, build, help smoke, and
  E2E fixtures.
- CI includes a behavior command smoke test.
- CI passes on the supported runner matrix.

## BehaviorSignature v0.2

- `agent-vcr behavior --help` works.
- `agent-vcr behavior latest` works on a run fixture.
- `agent-vcr behavior latest --json` parses as JSON.
- `agent-vcr behavior diff run-a run-b` reports first behavior divergence.
- `agent-vcr behavior diff run-a run-b --json` parses as JSON.
- `agent-vcr behavior metrics latest` works.
- Behavior metrics cover context discipline, validation behavior, edit scope,
  tool efficiency, and recovery behavior.
- First behavior divergence has a golden test.
- E2E fixtures create two behavior runs and verify behavior latest, diff, and
  metrics.

## Behavior Visualization v0.2.5

- `agent-vcr visualize latest --json` works.
- `agent-vcr visualize latest --html` works.
- Single-run visualize includes a behavior lane, file access data, and metrics
  cards.
- Two-run visualize shows swimlane comparison.
- Two-run visualize highlights first divergence.
- Multi-run visualize supports at least 3 lanes.
- Multi-run visualize remains a lane comparison, not Matrix Compare.
- File access compare is present in JSON and HTML output.
- Metrics cards are present in JSON and HTML output.
- HTML escaping tests pass.
- Redacted output does not expose prompts, secrets, or raw tool output.
- E2E fixtures do not depend on a real Codex session.
- No HarnessMetadata, HarnessDiff, Matrix Compare, Regression/baseline, new
  adapter, LLM Explain, deterministic replay, or cloud dashboard is implemented
  as part of v0.2.5.
- Documentation states that v0.3 is where HarnessMetadata and pairwise
  HarnessDiff are planned.

## Documentation

- README first screen positions the project as a Behavior Diff / Harness Diff
  foundation.
- Roadmap no longer prioritizes Matrix Compare or Behavior Regression.
- `docs/behavior-diff.md` exists and reflects the pairwise comparison roadmap.
- v0.2 is BehaviorSignature and first behavior divergence.
- v0.3 roadmap is HarnessMetadata / HarnessDiff, and docs do not claim it is
  implemented.
- v0.4 roadmap is Pairwise Compare Report.
- v0.5 roadmap is Capture Completeness + selected adapters / SDK.
- Docs do not claim Matrix Compare is implemented.
- Docs do not claim Regression/baseline is implemented.
- Docs explain v0.2.5 Behavior Visualization and its scope boundary.
- Docs do not claim v0.2.5 implements HarnessMetadata, HarnessDiff, Matrix
  Compare, Regression/baseline, LLM Explain, deterministic replay, or a cloud
  dashboard.
- README install and quick start commands match the CLI.
- Codex hook setup is documented.
- Generic CLI wrapper usage is documented.
- Replay, event-level diff, behavior diff, metrics, check, and export examples
  are documented with current support boundaries.
- Privacy and redaction limitations are explicit.
- Adapter development guide is current.
- Trace schema documentation is current.
- Changelog has the release entry.

## Privacy

- `.agent-vcr/` remains gitignored.
- Redaction is enabled by default in config.
- Shared reports should be generated with `--redacted` when export supports it.
- Documentation does not claim a complete security sandbox.

## Scope Guard

- No deterministic replay claim.
- No full security firewall claim.
- No cloud dashboard claim.
- No LLM explain feature.
- No HarnessDiff implementation claim.
- No Matrix Compare implementation claim.
- No Regression/baseline implementation claim.
- No v0.2.5 HarnessMetadata claim.
- No v0.2.5 Matrix or Regression claim.
- No claim that every agent has deep first-party support.
- No claim that hidden/private model reasoning is recorded.
- Docs do not present LLM explain as core.

## Artifacts

- Run `scripts/build-release.ps1 -Version <version> -Clean`.
- Confirm archives exist for linux amd64/arm64, darwin amd64/arm64, and windows
  amd64.
- Confirm `dist/checksums.txt` exists.
- Upload artifacts to the GitHub release for tag `v<version>`.

## Tagging

```bash
git tag v0.2.0
git push origin v0.2.0
```
