package generic

import (
	"context"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func (Adapter) Normalize(ctx context.Context, raw trace.RawEvent) ([]trace.Event, error) {
	if raw.Payload == nil {
		return nil, nil
	}
	kind, _ := raw.Payload["kind"].(string)
	switch kind {
	case "run_start":
		return []trace.Event{trace.NewEvent("", trace.EventRunStart, Source())}, nil
	case "process_start":
		return []trace.Event{trace.NewEvent("", trace.EventProcessStart, Source())}, nil
	case "process_result":
		return []trace.Event{trace.NewEvent("", trace.EventProcessResult, Source())}, nil
	case "run_stop":
		return []trace.Event{trace.NewEvent("", trace.EventRunStop, Source())}, nil
	default:
		return nil, nil
	}
}
