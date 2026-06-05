# Adapter Development

Adapters are the only place for agent-specific ingestion. The rest of the
project works with normalized trace events.

The product direction is Behavior Diff / Harness Diff. Add an adapter when it
helps another harness produce comparable behavior traces; broad agent support is
not the goal by itself.

## Directory

Create a package under:

```text
internal/adapters/<name>/
```

Example:

```text
internal/adapters/kimi/
  adapter.go
  install.go
  normalize.go
  testdata/
```

## Interface

An adapter must implement the shared adapter interface:

```text
Name()
DisplayName()
Probe(context.Context)
Install(context.Context, adapters.InstallOptions)
Uninstall(context.Context, adapters.InstallOptions)
Normalize(context.Context, trace.RawEvent)
Capabilities()
```

Register it from the adapter package with the registry.

## Capabilities

Declare only what the adapter can actually observe, such as:

```text
prompt_capture
tool_call_capture
tool_result_capture
shell_capture
file_diff_capture
permission_capture
subagent_capture
can_install_hooks
can_run_as_wrapper
```

Capabilities are documentation for users and downstream report code. They are
not permission grants.

## Normalize Rules

All normalized events must be agent-independent `trace.Event` values. Use common
payload keys such as:

```text
tool_name
tool_use_id
command
exit_code
changed_files
duration_ms
stdout_blob
stderr_blob
```

Agent-specific raw field names belong in one of these places:

```text
source.raw_event_type
raw_ref
payload values after conversion to common names
```

Do not emit fields like `codex_tool_use_id` or `kimi_hook_event_name` in the
common schema.

## RawEvent Fallback

Normalize failures and unknown event types must produce `raw_event` with a
`raw_ref`. The adapter must not panic and must not drop the original data.

Recommended payload:

```json
{
  "reason": "unknown_event"
}
```

or:

```json
{
  "reason": "normalize_error",
  "error": "short diagnostic"
}
```

## Tests

Each adapter should include:

- Fixture input files under `internal/adapters/<name>/testdata/`.
- Golden tests for normalized events.
- Unknown/raw event coverage.
- Invalid JSON or malformed payload coverage when the adapter parses JSON.
- Probe behavior that does not fail when the target agent is absent.

## Import Boundaries

Adapter packages may import shared packages such as `internal/trace` and
`internal/config`. Shared analysis/report/check/trace packages must not import
`internal/adapters/*`.

Adding a new adapter should normally not modify replay, diff, check, or report.
If it does, the change must be a new generic event type or a generic trace
behavior, not an adapter special case.

## Hook Safety

Hook commands must:

- Read JSON from stdin.
- Write nothing to stdout by default.
- Return exit code 0 on errors.
- Avoid complex analysis and external LLM calls.
- Avoid uploads.
