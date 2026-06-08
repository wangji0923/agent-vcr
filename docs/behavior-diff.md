# Behavior Diff

agent-vcr is Behavior Diff / Harness Diff infrastructure for AI coding agents.
The v0.1 release is the event-level trace foundation: it records normalized
traces, stores them as run artifacts, and compares where two runs diverge at the
event level. The v0.2 release adds BehaviorSignature and first behavior
divergence so two runs can be compared at the behavior-step level.

## Definition

Behavior Diff compares how an agent got to an outcome.

```text
git diff:
  final code changes

agent-vcr diff:
  behavior path:
  prompt -> search/read/edit/tool-call/test/recovery -> final outcome
```

The goal is pairwise comparison: same model, same task, different harness.

## Harness

For agent-vcr, a harness is the execution setup around the model:

```text
harness = model
        + prompt scaffold
        + AGENTS.md
        + tools
        + MCP servers
        + context policy
        + permission mode
        + sandbox
        + retry strategy
        + eval runner
```

Different harnesses can produce meaningfully different behavior even when the
model and task are unchanged.

## Example

Same model, same task:

Run A:

- searched `session`
- read `tests/auth/session.test.ts`
- edited `src/auth/session.ts`
- ran `npm test`

Run B:

- searched `cookie`
- read `src/auth/legacy-cookie.ts`
- edited the legacy module
- skipped tests

agent-vcr should report:

```text
First behavior divergence:
Run A inspected tests before editing.
Run B entered legacy path before test discovery.
```

In v0.1 this is approximated with normalized event signatures. v0.2 implements
higher-level BehaviorSignature comparison.

## v0.1: Trace Foundation + Event-level Diff

Current capabilities:

- normalized trace
- event-level first divergence
- tool sequence diff
- changed files diff
- command exit code diff
- replay timeline
- check
- export
- doctor

Current diff is event-level behavior diff. It is not a full
`BehaviorSignature`.

## v0.2: BehaviorSignature + First Behavior Divergence

Implemented behavior abstractions:

- `behavior.Step`
- `BehaviorSignature`
- behavior timeline
- first behavior divergence
- behavior metrics

Behavior steps may include:

- search
- read_file
- edit_file
- run_test
- run_build
- call_tool
- call_mcp_tool
- recover_from_error
- skip_validation

Implemented metrics:

- context discipline
- validation behavior
- edit scope
- tool efficiency
- recovery behavior

Commands:

```bash
agent-vcr behavior latest
agent-vcr behavior diff run-a run-b
agent-vcr behavior metrics latest
```

`agent-vcr diff` remains the event-level diff. `agent-vcr behavior diff` is the
BehaviorSignature-level diff and reports the first behavior divergence.

## v0.2.5: Behavior Visualization

v0.2.5 is the visualization layer for v0.2 behavior data. It does not explain
why a harness changed behavior; it makes the observed behavior path easier to
inspect.

Supported visualization scope:

- single-run behavior timeline
- two-run swimlane comparison
- small multi-run lane comparison for user-selected runs
- first divergence highlight
- file access comparison
- search scope comparison
- metrics cards
- static JSON and offline HTML output

Example commands:

```bash
agent-vcr visualize latest --json
agent-vcr visualize latest --html --output behavior.html
agent-vcr visualize run-a run-b --html --output compare.html
agent-vcr visualize run-a run-b run-c --json
```

v0.2.5 remains agent-agnostic and consumes normalized trace and BehaviorSignature
data. It does not implement HarnessMetadata, HarnessDiff, Matrix Compare,
Regression/baseline, new adapters, LLM Explain, deterministic replay, or cloud
dashboard features. v0.3 is the planned point where HarnessMetadata and pairwise
HarnessDiff enter the product.

The `path_graph` JSON field and the HTML Path Graph section are auxiliary. The
Swimlane Timeline remains the source of truth for behavior-path comparison.
Search scopes such as `src` and `tests` are reported under `search_scopes`, not
as file read/edit rows in `file_access`.

## v0.3: HarnessMetadata + Pairwise HarnessDiff

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

Planned commands:

```bash
agent-vcr harness inspect latest
agent-vcr harness diff run-a run-b
```

The focus is pairwise harness diff, not batch matrix execution.

## v0.4: Pairwise Compare Report

Planned command:

```bash
agent-vcr compare run-a run-b
```

The report should summarize:

- harness changes
- first behavior divergence
- behavior metrics
- validation behavior
- edit scope
- outcome difference
- likely behavior impact

This is a two-run comparison report, not a benchmark dashboard.

## v0.5: Capture Completeness + Selected Adapters / SDK

The goal is higher quality comparable traces, not supporting every agent for its
own sake.

Possible work:

- more complete Codex tool classification
- Read/Edit/Search/Test behavior recognition
- MCP tool behavior classification
- Claude Code adapter
- Kimi Code adapter
- minimal Python SDK
- minimal TypeScript SDK
- HTTP ingest

New adapters are useful when they let more harnesses emit comparable behavior
traces.

## Not The Near-term Mainline

Not in v0.1, and not prioritized as the v0.2-v0.5 mainline:

- deterministic replay
- full security firewall
- LLM-powered explanations
- cloud dashboard
- batch matrix runner
- benchmark platform
- behavior regression platform
- PR risk scoring product
- all-agent deep support out of the box
