package behavior

import (
	"context"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type ExtractInput struct {
	RunID    string          `json:"run_id"`
	Events   []trace.Event   `json:"events"`
	Metadata *trace.Metadata `json:"metadata,omitempty"`
}

type ExtractResult struct {
	RunID    string   `json:"run_id"`
	Timeline Timeline `json:"timeline"`
	Warnings []string `json:"warnings,omitempty"`
}

type Extractor interface {
	Extract(ctx context.Context, input ExtractInput) (ExtractResult, error)
}
