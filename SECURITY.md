# Security Policy

agent-vcr is local-first. It should not upload trace data, call extra LLM APIs,
or require cloud credentials for core recording, replay, diff, check, export,
redact, or doctor workflows.

## Reporting Vulnerabilities

Please do not post secrets, private traces, private prompts, tokens, or local
path-sensitive artifacts in public issues.

Use GitHub private vulnerability reporting if it is available for this
repository. If it is not available, open a minimal public issue that describes
the affected command or component without including sensitive data.

## Sensitive Data

Trace and report artifacts may contain prompts, commands, file paths, patches,
tool summaries, stdout or stderr snippets, and local project metadata.

Before sharing artifacts, prefer:

```bash
agent-vcr export latest --html --redacted
```

The repository must keep `.agent-vcr/`, `.env`, API keys, private keys, JWTs,
and generated local reports out of Git history.

## Maintainer Expectations

Security-sensitive changes should be reviewed by a maintainer before merging.
Changes that add network upload, cloud storage, extra LLM calls, credential
handling, secret redaction, or trace/report output behavior need explicit
maintainer approval and tests.
