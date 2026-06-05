package check

import (
	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/config"
)

type Result = analysis.CheckResult
type Violation = analysis.Violation

func Run(data analysis.RunData, cfg config.Config) analysis.CheckResult {
	return analysis.CheckRun(data, cfg)
}
