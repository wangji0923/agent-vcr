## Summary

-

## Scope

- [ ] This PR is limited to one module or one clearly bounded maintenance task.
- [ ] I read the relevant module document or explained why no module document applies.
- [ ] I did not add cloud upload, extra LLM API calls, database usage, or a frontend framework.

## Architecture

- [ ] Codex-specific logic stays in `internal/adapters/codex`.
- [ ] Shared `analysis`, `report`, `check`, and `trace` code does not import adapter packages.
- [ ] Adapter output uses normalized `trace.Event` fields instead of agent-specific schema fields.
- [ ] Normalize failures preserve data through `raw_event` / `raw_ref` behavior.

## Privacy

- [ ] This PR does not commit `.agent-vcr/`, `.env`, secrets, private traces, or generated local reports.
- [ ] Trace/report changes preserve or improve redaction behavior.

## Verification

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go run ./cmd/agent-vcr --help`

## Notes

-
