package visualize

import (
	"fmt"
	"strings"
)

func ValidateReport(report *VisualReport) error {
	if report == nil {
		return fmt.Errorf("visual report is nil")
	}
	if strings.TrimSpace(report.SchemaVersion) == "" {
		return fmt.Errorf("schema_version is required")
	}
	runIDs, err := collectRunIDs(report)
	if err != nil {
		return err
	}
	for _, lane := range report.Lanes {
		if strings.TrimSpace(lane.RunID) == "" {
			return fmt.Errorf("lane run_id is required")
		}
		if !runIDs[lane.RunID] {
			return fmt.Errorf("lane references unknown run_id %q", lane.RunID)
		}
		for _, step := range lane.Steps {
			if strings.TrimSpace(step.RunID) == "" {
				return fmt.Errorf("step %q run_id is required", step.StepID)
			}
			if !runIDs[step.RunID] {
				return fmt.Errorf("step %q references unknown run_id %q", step.StepID, step.RunID)
			}
			if step.RunID != lane.RunID {
				return fmt.Errorf("step %q run_id %q does not match lane run_id %q", step.StepID, step.RunID, lane.RunID)
			}
		}
	}
	for _, row := range report.Alignment {
		for key, cell := range row.Cells {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("alignment row %d has empty cell key", row.RowIndex)
			}
			if !runIDs[key] {
				return fmt.Errorf("alignment row %d references unknown run_id %q", row.RowIndex, key)
			}
			if strings.TrimSpace(cell.RunID) == "" {
				return fmt.Errorf("alignment row %d cell %q run_id is required", row.RowIndex, key)
			}
			if !runIDs[cell.RunID] {
				return fmt.Errorf("alignment row %d cell references unknown run_id %q", row.RowIndex, cell.RunID)
			}
			if cell.RunID != key {
				return fmt.Errorf("alignment row %d cell key %q does not match run_id %q", row.RowIndex, key, cell.RunID)
			}
			if cell.Gap && cell.Step != nil {
				return fmt.Errorf("alignment row %d cell %q cannot be both gap and step", row.RowIndex, key)
			}
			if !cell.Gap && cell.Step == nil {
				return fmt.Errorf("alignment row %d cell %q requires step or gap", row.RowIndex, key)
			}
			if cell.Step != nil && cell.Step.RunID != cell.RunID {
				return fmt.Errorf("alignment row %d cell %q step run_id %q does not match cell run_id %q", row.RowIndex, key, cell.Step.RunID, cell.RunID)
			}
		}
	}
	for _, divergence := range report.Divergences {
		if strings.TrimSpace(divergence.BaselineRunID) != "" && !runIDs[divergence.BaselineRunID] {
			return fmt.Errorf("divergence references unknown baseline_run_id %q", divergence.BaselineRunID)
		}
		if strings.TrimSpace(divergence.CompareRunID) != "" && !runIDs[divergence.CompareRunID] {
			return fmt.Errorf("divergence references unknown compare_run_id %q", divergence.CompareRunID)
		}
		if divergence.Left != nil && !runIDs[divergence.Left.RunID] {
			return fmt.Errorf("divergence left step references unknown run_id %q", divergence.Left.RunID)
		}
		if divergence.Right != nil && !runIDs[divergence.Right.RunID] {
			return fmt.Errorf("divergence right step references unknown run_id %q", divergence.Right.RunID)
		}
	}
	for _, row := range report.FileAccess.Rows {
		if strings.TrimSpace(row.Path) == "" {
			return fmt.Errorf("file access row path is required")
		}
		for runID := range row.Runs {
			if !runIDs[runID] {
				return fmt.Errorf("file access row %q references unknown run_id %q", row.Path, runID)
			}
		}
	}
	for _, row := range report.SearchScopes.Rows {
		if strings.TrimSpace(row.Scope) == "" {
			return fmt.Errorf("search scope row scope is required")
		}
		for runID := range row.Runs {
			if !runIDs[runID] {
				return fmt.Errorf("search scope row %q references unknown run_id %q", row.Scope, runID)
			}
		}
	}
	for _, group := range report.Metrics {
		if strings.TrimSpace(group.RunID) == "" {
			return fmt.Errorf("metrics group run_id is required")
		}
		if !runIDs[group.RunID] {
			return fmt.Errorf("metrics group references unknown run_id %q", group.RunID)
		}
	}
	return nil
}

func ValidateRunIDs(report *VisualReport) error {
	_, err := collectRunIDs(report)
	return err
}

func collectRunIDs(report *VisualReport) (map[string]bool, error) {
	if report == nil {
		return nil, fmt.Errorf("visual report is nil")
	}
	if len(report.Runs) == 0 {
		return nil, fmt.Errorf("at least one run is required")
	}
	runIDs := make(map[string]bool, len(report.Runs))
	for i, run := range report.Runs {
		runID := strings.TrimSpace(run.RunID)
		if runID == "" {
			return nil, fmt.Errorf("runs[%d].run_id is required", i)
		}
		if runIDs[runID] {
			return nil, fmt.Errorf("duplicate run_id %q", runID)
		}
		runIDs[runID] = true
	}
	return runIDs, nil
}
