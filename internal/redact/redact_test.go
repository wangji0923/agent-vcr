package redact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestRedactsCommonSecretsAndEnvPaths(t *testing.T) {
	cfg := config.Default().Redaction
	redactor, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	input := strings.Join([]string{
		"openai=sk-abcdefghijklmnopqrstuvwxyz123456",
		"anthropic=sk-ant-abcdefghijklmnopqrstuvwxyz123456",
		"jwt=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature",
		"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
		"load C:\\work\\.env.local before running",
		"api_key=abcdefghijklmnop1234567890",
	}, "\n")

	output := redactor.String(input)
	for _, secret := range []string{
		"sk-abcdefghijklmnopqrstuvwxyz123456",
		"sk-ant-abcdefghijklmnopqrstuvwxyz123456",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature",
		"-----BEGIN PRIVATE KEY-----",
		"C:\\work\\.env.local",
		"abcdefghijklmnop1234567890",
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("secret %q was not redacted in %q", secret, output)
		}
	}
}

func TestCustomRegexAndDisabledRedaction(t *testing.T) {
	cfg := config.Default().Redaction
	cfg.Patterns = append(cfg.Patterns, config.PatternConfig{Name: "ticket", Regex: `TICKET-[0-9]+`})
	output, err := MaskString("id TICKET-1234", cfg)
	if err != nil {
		t.Fatalf("MaskString: %v", err)
	}
	if strings.Contains(output, "TICKET-1234") {
		t.Fatalf("custom regex did not redact: %q", output)
	}

	cfg.Enabled = false
	plain, err := MaskString("sk-abcdefghijklmnopqrstuvwxyz123456", cfg)
	if err != nil {
		t.Fatalf("MaskString disabled: %v", err)
	}
	if plain != "sk-abcdefghijklmnopqrstuvwxyz123456" {
		t.Fatalf("disabled redaction changed input: %q", plain)
	}
}

func TestPlainTextNotOverRedacted(t *testing.T) {
	cfg := config.Default().Redaction
	redactor, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	input := "normal discussion about token budgets and environment setup"
	output := redactor.String(input)
	if output != input {
		t.Fatalf("plain text was over-redacted: %q", output)
	}
}

func TestRedactRunCopiesRunWithoutModifyingOriginal(t *testing.T) {
	project := t.TempDir()
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	store, err := trace.CreateRun(project, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	patch, err := store.WritePatch("secret.diff", []byte("+OPENAI_API_KEY="+secret+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	event := trace.NewEvent(store.RunID, trace.EventToolCall, trace.Source{Adapter: "fixture"})
	event.Payload = trace.Payload{"tool_name": "shell", "command": "echo " + secret, "api_key": secret}
	event.Artifacts = []trace.ArtifactRef{patch}
	if err := store.Append(event); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(project, ".agent-vcr", "runs", store.RunID+"-redacted")
	if err := RedactRun(project, store.RunID, outputDir); err != nil {
		t.Fatalf("RedactRun: %v", err)
	}

	originalTrace, err := os.ReadFile(store.Path("trace.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(originalTrace), secret) {
		t.Fatalf("original trace was unexpectedly modified:\n%s", string(originalTrace))
	}
	redactedTrace, err := os.ReadFile(filepath.Join(outputDir, "trace.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(redactedTrace), secret) {
		t.Fatalf("redacted trace still contains secret:\n%s", string(redactedTrace))
	}
	if !strings.Contains(string(redactedTrace), store.RunID+"-redacted") {
		t.Fatalf("redacted trace does not use redacted run id:\n%s", string(redactedTrace))
	}
	redactedPatch, err := os.ReadFile(filepath.Join(outputDir, "patches", "secret.diff"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(redactedPatch), secret) {
		t.Fatalf("redacted patch still contains secret:\n%s", string(redactedPatch))
	}
}
