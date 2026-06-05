# v0.1 Release Checklist

Use this before tagging a v0.1 release.

## Local Verification

```bash
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
powershell -ExecutionPolicy Bypass -File ./scripts/e2e.ps1
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
- CI passes on the supported runner matrix.

## Documentation

- README first screen positions the project as a Behavior Diff / Harness Diff
  foundation.
- Roadmap no longer prioritizes Matrix Compare or Behavior Regression.
- `docs/behavior-diff.md` exists and reflects the pairwise comparison roadmap.
- v0.2 roadmap is BehaviorSignature.
- v0.3 roadmap is HarnessMetadata / HarnessDiff.
- v0.4 roadmap is Pairwise Compare Report.
- v0.5 roadmap is Capture Completeness + selected adapters / SDK.
- README install and quick start commands match the CLI.
- Codex hook setup is documented.
- Generic CLI wrapper usage is documented.
- Replay, diff, check, and export examples are documented with current support
  boundaries.
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
git tag v0.1.0
git push origin v0.1.0
```
