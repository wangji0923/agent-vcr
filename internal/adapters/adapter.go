package adapters

import (
	"context"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type RawEvent = trace.RawEvent

type ProbeResult struct {
	Found   bool              `json:"found"`
	Version string            `json:"version,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

type InstallOptions struct {
	Scope      string
	ProjectDir string
	Force      bool
}

type Adapter interface {
	Name() string
	DisplayName() string
	Probe(ctx context.Context) (*ProbeResult, error)
	Install(ctx context.Context, opts InstallOptions) error
	Uninstall(ctx context.Context, opts InstallOptions) error
	Normalize(ctx context.Context, raw trace.RawEvent) ([]trace.Event, error)
	Capabilities() Capabilities
}
