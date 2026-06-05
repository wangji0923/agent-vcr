package codex

import (
	"context"
	"os/exec"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/adapters"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const (
	AdapterName   = "codex"
	SourceAdapter = "codex-hooks"
	SourceAgent   = "codex"
)

type CodexHookAdapter struct{}

func init() {
	adapters.Register(New())
}

func New() *CodexHookAdapter {
	return &CodexHookAdapter{}
}

func (a *CodexHookAdapter) Name() string {
	return AdapterName
}

func (a *CodexHookAdapter) DisplayName() string {
	return "Codex Hooks"
}

func (a *CodexHookAdapter) Probe(ctx context.Context) (*adapters.ProbeResult, error) {
	cmd := exec.CommandContext(ctx, "codex", "--version")
	output, err := cmd.Output()
	if err != nil {
		return &adapters.ProbeResult{
			Found:   false,
			Details: map[string]string{"error": err.Error()},
		}, nil
	}
	return &adapters.ProbeResult{
		Found:   true,
		Version: strings.TrimSpace(string(output)),
	}, nil
}

func (a *CodexHookAdapter) Install(ctx context.Context, opts adapters.InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return InstallHooks(opts)
}

func (a *CodexHookAdapter) Uninstall(ctx context.Context, opts adapters.InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (a *CodexHookAdapter) Normalize(ctx context.Context, raw trace.RawEvent) ([]trace.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return NormalizeHook(raw), nil
}

func (a *CodexHookAdapter) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{
		PromptCapture:      true,
		ModelCallCapture:   false,
		ModelResultCapture: false,
		ToolCallCapture:    true,
		ToolResultCapture:  true,
		ShellCapture:       true,
		FileDiffCapture:    true,
		PermissionCapture:  true,
		SubagentCapture:    true,
		MCPToolCapture:     true,
		CanInstallHooks:    true,
	}
}
