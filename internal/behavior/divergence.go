package behavior

type DivergenceKind string

const (
	DivergenceNone          DivergenceKind = "no_divergence"
	DivergenceStepChanged   DivergenceKind = "step_changed"
	DivergenceResultChanged DivergenceKind = "result_changed"
	DivergenceMissingInA    DivergenceKind = "step_missing_in_a"
	DivergenceMissingInB    DivergenceKind = "step_missing_in_b"
	DivergenceUnknown       DivergenceKind = "unknown"
)

type Divergence struct {
	Index           int            `json:"index"`
	Kind            DivergenceKind `json:"kind"`
	RunAStep        *Step          `json:"run_a_step,omitempty"`
	RunBStep        *Step          `json:"run_b_step,omitempty"`
	Summary         string         `json:"summary"`
	Explanation     string         `json:"explanation,omitempty"`
	RelatedEventIDs []string       `json:"related_event_ids,omitempty"`
}
