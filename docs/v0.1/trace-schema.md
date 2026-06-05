# Trace Schema

The trace schema is the raw data layer for Behavior Diff. It stores normalized
events from different agent harnesses so replay, event-level diff, check, export,
and future behavior analysis can consume one adapter-independent format.

Current schema version:

```text
0.2
```

The CLI version and trace schema version are separate. The current schema is a
normalized event schema. It is not the future `BehaviorSignature` schema and it
does not yet include `HarnessMetadata`.

## Event Shape

```json
{
  "schema_version": "0.2",
  "event_id": "evt_123",
  "run_id": "run_123",
  "parent_id": "",
  "span_id": "",
  "event_index": 1,
  "type": "tool_call",
  "source": {
    "adapter": "codex-hooks",
    "agent": "codex",
    "raw_event_type": "PreToolUse",
    "version": ""
  },
  "timestamp": "2026-06-04T00:00:00Z",
  "payload": {
    "tool_name": "Bash"
  },
  "artifacts": [],
  "raw_ref": {
    "kind": "raw",
    "path": "raw/PreToolUse.json",
    "sha256": "abc123",
    "size_bytes": 128,
    "mime_type": "application/json"
  }
}
```

Required fields:

```text
schema_version
event_id
run_id
event_index
type
source
timestamp
```

Optional fields:

```text
parent_id
span_id
payload
artifacts
raw_ref
```

## Source

`source.adapter` identifies the adapter output format, not a core subsystem.
`source.raw_event_type` may contain the original hook or input type. Raw
agent-specific fields should not become top-level trace fields.

## ArtifactRef

Artifacts point to files under the run directory:

```json
{
  "kind": "blob",
  "path": "blobs/process_stdout.txt",
  "sha256": "abc123",
  "size_bytes": 42,
  "mime_type": "text/plain"
}
```

Common kinds:

```text
blob
patch
raw
snapshot
```

## Event Types

Adapters do not need to emit every event type.

### run_start

```json
{
  "schema_version": "0.2",
  "event_id": "evt_run_start",
  "run_id": "run_1",
  "event_index": 1,
  "type": "run_start",
  "source": { "adapter": "generic-cli", "agent": "generic-cli" },
  "timestamp": "2026-06-04T00:00:00Z",
  "payload": {
    "command": "go",
    "args": ["test", "./..."],
    "cwd": "/repo"
  }
}
```

### user_prompt

```json
{
  "schema_version": "0.2",
  "event_id": "evt_prompt",
  "run_id": "run_1",
  "event_index": 2,
  "type": "user_prompt",
  "source": { "adapter": "codex-hooks", "agent": "codex", "raw_event_type": "UserPromptSubmit" },
  "timestamp": "2026-06-04T00:00:01Z",
  "payload": {
    "turn_id": "turn_1",
    "prompt": "[REDACTED:prompt]",
    "prompt_sha256": "abc123"
  }
}
```

### tool_call

```json
{
  "schema_version": "0.2",
  "event_id": "evt_tool_call",
  "run_id": "run_1",
  "event_index": 3,
  "type": "tool_call",
  "source": { "adapter": "codex-hooks", "agent": "codex", "raw_event_type": "PreToolUse" },
  "timestamp": "2026-06-04T00:00:02Z",
  "payload": {
    "turn_id": "turn_1",
    "tool_use_id": "toolu_1",
    "tool_name": "Bash",
    "input": {
      "redacted": true,
      "size_bytes": 64,
      "sha256": "abc123"
    }
  }
}
```

### tool_result

```json
{
  "schema_version": "0.2",
  "event_id": "evt_tool_result",
  "run_id": "run_1",
  "event_index": 4,
  "type": "tool_result",
  "source": { "adapter": "codex-hooks", "agent": "codex", "raw_event_type": "PostToolUse" },
  "timestamp": "2026-06-04T00:00:03Z",
  "payload": {
    "turn_id": "turn_1",
    "tool_use_id": "toolu_1",
    "tool_name": "Bash",
    "result": {
      "mode": "hash",
      "exit_code": 0,
      "size_bytes": 128,
      "sha256": "abc123"
    }
  }
}
```

### process_start

```json
{
  "schema_version": "0.2",
  "event_id": "evt_process_start",
  "run_id": "run_1",
  "event_index": 5,
  "type": "process_start",
  "source": { "adapter": "generic-cli", "agent": "generic-cli" },
  "timestamp": "2026-06-04T00:00:04Z",
  "payload": {
    "command": "go",
    "args": ["test", "./..."],
    "cwd": "/repo"
  }
}
```

### process_result

```json
{
  "schema_version": "0.2",
  "event_id": "evt_process_result",
  "run_id": "run_1",
  "event_index": 6,
  "type": "process_result",
  "source": { "adapter": "generic-cli", "agent": "generic-cli" },
  "timestamp": "2026-06-04T00:00:05Z",
  "payload": {
    "command": "go",
    "args": ["test", "./..."],
    "exit_code": 0,
    "duration_ms": 1200,
    "changed_files": ["internal/example.go"],
    "stdout_blob": "blobs/process_stdout.txt"
  }
}
```

### raw_event

```json
{
  "schema_version": "0.2",
  "event_id": "evt_raw",
  "run_id": "run_1",
  "event_index": 7,
  "type": "raw_event",
  "source": { "adapter": "codex-hooks", "agent": "codex", "raw_event_type": "UnknownHook" },
  "timestamp": "2026-06-04T00:00:06Z",
  "payload": {
    "reason": "unknown_event"
  },
  "raw_ref": {
    "kind": "raw",
    "path": "raw/UnknownHook.json",
    "sha256": "abc123",
    "size_bytes": 64,
    "mime_type": "application/json"
  }
}
```

### run_stop

```json
{
  "schema_version": "0.2",
  "event_id": "evt_run_stop",
  "run_id": "run_1",
  "event_index": 8,
  "type": "run_stop",
  "source": { "adapter": "generic-cli", "agent": "generic-cli" },
  "timestamp": "2026-06-04T00:00:07Z",
  "payload": {
    "status": "completed",
    "exit_code": 0,
    "duration_ms": 1300
  }
}
```

## Current Defined Types

```text
run_start
run_stop
user_prompt
model_call
model_result
tool_call
tool_result
tool_error
file_read
file_write
file_patch
shell_command
shell_result
permission_request
subagent_start
subagent_stop
context_compact
process_start
process_result
raw_event
```

Adapters must not create agent-prefixed event types such as `codex_tool_call`.

## Future: BehaviorSignature

`BehaviorSignature` is planned for v0.2. It will summarize low-level events into
behavior steps such as search, read_file, edit_file, run_test, run_build,
call_tool, call_mcp_tool, recover_from_error, and skip_validation.

This is not implemented in v0.1. v0.1 diff is event-level behavior diff over the
normalized trace schema above.

## Future: HarnessMetadata

`HarnessMetadata` is planned for v0.3. It will describe the harness around a run,
including values such as model, prompt hash, AGENTS.md hash, tool schema hash,
MCP config hash, permission mode, sandbox mode, context policy, and adapter
version.

This is not implemented in v0.1. v0.1 traces may contain adapter source and
capability metadata, but they are not a full harness diff model.
