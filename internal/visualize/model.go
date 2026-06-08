package visualize

import (
	"time"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

const (
	SchemaVersion      = "0.2.5"
	MaxRecommendedRuns = 5
)

type RenderMode string

const (
	RenderModeSingle  RenderMode = "single"
	RenderModeCompare RenderMode = "compare"
)

type VisualStepKind string

const (
	VisualStepSearch            VisualStepKind = VisualStepKind(behavior.StepSearch)
	VisualStepReadFile          VisualStepKind = VisualStepKind(behavior.StepReadFile)
	VisualStepInspectTest       VisualStepKind = VisualStepKind(behavior.StepInspectTest)
	VisualStepEditFile          VisualStepKind = VisualStepKind(behavior.StepEditFile)
	VisualStepRunTest           VisualStepKind = VisualStepKind(behavior.StepRunTest)
	VisualStepRunBuild          VisualStepKind = VisualStepKind(behavior.StepRunBuild)
	VisualStepInstallDependency VisualStepKind = VisualStepKind(behavior.StepInstallDependency)
	VisualStepCallTool          VisualStepKind = VisualStepKind(behavior.StepCallTool)
	VisualStepCallMCPTool       VisualStepKind = VisualStepKind(behavior.StepCallMCPTool)
	VisualStepPermissionRequest VisualStepKind = VisualStepKind(behavior.StepPermissionRequest)
	VisualStepRecoverFromError  VisualStepKind = VisualStepKind(behavior.StepRecoverFromError)
	VisualStepSkipValidation    VisualStepKind = VisualStepKind(behavior.StepSkipValidation)
	VisualStepContextCompact    VisualStepKind = VisualStepKind(behavior.StepContextCompact)
	VisualStepProcessStart      VisualStepKind = VisualStepKind(behavior.StepProcessStart)
	VisualStepProcessResult     VisualStepKind = VisualStepKind(behavior.StepProcessResult)
	VisualStepRawBehavior       VisualStepKind = VisualStepKind(behavior.StepRawBehavior)
	VisualStepUnknown           VisualStepKind = VisualStepKind(behavior.StepUnknown)
)

type VisualPhase string

const (
	VisualPhaseDiscovery  VisualPhase = "discovery"
	VisualPhaseInspection VisualPhase = "inspection"
	VisualPhaseEditing    VisualPhase = "editing"
	VisualPhaseValidation VisualPhase = "validation"
	VisualPhaseRecovery   VisualPhase = "recovery"
	VisualPhaseFinish     VisualPhase = "finish"
)

type VisualReport struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   time.Time          `json:"generated_at"`
	Mode          RenderMode         `json:"mode"`
	Options       VisualOptions      `json:"options,omitempty"`
	Summary       VisualSummary      `json:"summary"`
	Runs          []VisualRun        `json:"runs"`
	Lanes         []BehaviorLane     `json:"lanes"`
	Alignment     []AlignmentRow     `json:"alignment,omitempty"`
	Divergences   []DivergenceMarker `json:"divergences,omitempty"`
	FileAccess    FileAccessCompare  `json:"file_access,omitempty"`
	SearchScopes  SearchScopeCompare `json:"search_scopes,omitempty"`
	Metrics       []MetricsCardGroup `json:"metrics,omitempty"`
	PathGraph     *PathGraph         `json:"path_graph,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

type VisualRun struct {
	RunID     string         `json:"run_id"`
	Label     string         `json:"label"`
	Source    string         `json:"source"`
	Status    string         `json:"status"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
	StepCount int            `json:"step_count"`
	Summary   map[string]any `json:"summary,omitempty"`
}

type BehaviorLane struct {
	RunID string       `json:"run_id"`
	Label string       `json:"label"`
	Steps []VisualStep `json:"steps"`
}

type VisualStep struct {
	RunID       string            `json:"run_id"`
	StepID      string            `json:"step_id"`
	Index       int               `json:"index"`
	Kind        VisualStepKind    `json:"kind"`
	Phase       VisualPhase       `json:"phase,omitempty"`
	Summary     string            `json:"summary"`
	Query       string            `json:"query,omitempty"`
	Command     string            `json:"command,omitempty"`
	Files       []string          `json:"files,omitempty"`
	Target      string            `json:"target,omitempty"`
	EventIDs    []string          `json:"event_ids,omitempty"`
	Significant bool              `json:"significant"`
	Divergent   bool              `json:"divergent"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type AlignmentRow struct {
	RowIndex    int                 `json:"row_index"`
	Cells       map[string]StepCell `json:"cells"`
	IsDivergent bool                `json:"is_divergent"`
	Reason      string              `json:"reason,omitempty"`
}

type StepCell struct {
	RunID string      `json:"run_id"`
	Step  *VisualStep `json:"step,omitempty"`
	Gap   bool        `json:"gap"`
}

type DivergenceMarker struct {
	BaselineRunID  string      `json:"baseline_run_id"`
	CompareRunID   string      `json:"compare_run_id"`
	StepIndex      int         `json:"step_index"`
	AlignmentIndex int         `json:"alignment_index,omitempty"`
	Kind           string      `json:"kind"`
	Summary        string      `json:"summary"`
	First          bool        `json:"first"`
	Left           *VisualStep `json:"left,omitempty"`
	Right          *VisualStep `json:"right,omitempty"`
	EventIDs       []string    `json:"event_ids,omitempty"`
}

type VisualDivergence = DivergenceMarker

type FileAccessCompare struct {
	Rows []FileAccessRow `json:"rows"`
}

type FileAccessRow struct {
	Path string             `json:"path"`
	Runs map[string]FileUse `json:"runs"`
}

type FileUse struct {
	ReadCount   int    `json:"read_count"`
	EditCount   int    `json:"edit_count"`
	FirstStep   int    `json:"first_step"`
	LastStep    int    `json:"last_step"`
	FirstAction string `json:"first_action,omitempty"`
	LastAction  string `json:"last_action,omitempty"`
}

type SearchScopeCompare struct {
	Rows []SearchScopeRow `json:"rows"`
}

type SearchScopeRow struct {
	Scope string                    `json:"scope"`
	Runs  map[string]SearchScopeUse `json:"runs"`
}

type SearchScopeUse struct {
	SearchCount int      `json:"search_count"`
	FirstStep   int      `json:"first_step"`
	LastStep    int      `json:"last_step"`
	Queries     []string `json:"queries,omitempty"`
}

type MetricsCardGroup struct {
	RunID string        `json:"run_id"`
	Label string        `json:"label,omitempty"`
	Cards []MetricsCard `json:"cards"`
}

type RunMetricsCards = MetricsCardGroup

type MetricsCard struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Level string `json:"level,omitempty"`
}

type PathGraph struct {
	Nodes []PathNode `json:"nodes"`
	Edges []PathEdge `json:"edges"`
}

type PathNode struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Kind   string   `json:"kind"`
	RunIDs []string `json:"run_ids"`
}

type PathEdge struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	RunIDs []string `json:"run_ids"`
}

type VisualOptions struct {
	Mode          RenderMode        `json:"mode,omitempty"`
	BaselineRunID string            `json:"baseline_run_id,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	MaxRuns       int               `json:"max_runs,omitempty"`
	NoCache       bool              `json:"no_cache,omitempty"`
	Redacted      bool              `json:"redacted,omitempty"`
	IncludeGraph  bool              `json:"include_graph,omitempty"`
}

type VisualSummary struct {
	RunCount             int               `json:"run_count"`
	StepCount            int               `json:"step_count"`
	SignificantStepCount int               `json:"significant_step_count"`
	DivergenceCount      int               `json:"divergence_count"`
	FirstDivergence      *DivergenceMarker `json:"first_divergence,omitempty"`
	FileCount            int               `json:"file_count"`
	MetricsCardCount     int               `json:"metrics_card_count"`
	Mode                 RenderMode        `json:"mode,omitempty"`
}
