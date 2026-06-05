package check

import (
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestRunDelegatesToAnalysisCheck(t *testing.T) {
	cfg := config.Default()
	cfg.Rules.RequireTestsAfterSourceChange = false
	result := Run(analysis.RunData{RunID: "run", Events: []trace.Event{
		{
			SchemaVersion: trace.SchemaVersion,
			EventID:       "evt",
			Type:          trace.EventProcessResult,
			Source:        trace.Source{Adapter: "fixture"},
			Payload:       trace.Payload{"command": "agent", "exit_code": 0, "changed_files": []string{".env"}},
		},
	}}, cfg)
	if result.Passed {
		t.Fatal("Run returned passing result for forbidden path")
	}
}
