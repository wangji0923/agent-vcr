package behavior

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDivergenceJSONRoundTrip(t *testing.T) {
	runAStep := Step{
		StepID:      "step_a",
		RunID:       "run_a",
		Index:       2,
		Kind:        StepInspectTest,
		Target:      "tests/session.test.ts",
		Files:       []string{"tests/session.test.ts"},
		Significant: true,
		Result:      ResultSuccess,
	}
	runBStep := Step{
		StepID:      "step_b",
		RunID:       "run_b",
		Index:       2,
		Kind:        StepReadFile,
		Target:      "src/legacy-cookie.ts",
		Files:       []string{"src/legacy-cookie.ts"},
		Significant: true,
		Result:      ResultSuccess,
	}
	divergence := Divergence{
		Index:       2,
		Kind:        DivergenceStepChanged,
		RunAStep:    &runAStep,
		RunBStep:    &runBStep,
		Summary:     "behavior changed at step 2",
		Explanation: "Run B entered legacy path before test inspection.",
	}

	data, err := json.Marshal(divergence)
	if err != nil {
		t.Fatalf("marshal divergence: %v", err)
	}

	var got Divergence
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal divergence: %v", err)
	}
	if !reflect.DeepEqual(divergence, got) {
		t.Fatalf("round trip mismatch\nwant: %#v\n got: %#v", divergence, got)
	}
}

func TestDivergenceKindStableStrings(t *testing.T) {
	tests := map[DivergenceKind]string{
		DivergenceNone:          "no_divergence",
		DivergenceStepChanged:   "step_changed",
		DivergenceResultChanged: "result_changed",
		DivergenceMissingInA:    "step_missing_in_a",
		DivergenceMissingInB:    "step_missing_in_b",
		DivergenceUnknown:       "unknown",
	}

	for kind, want := range tests {
		if string(kind) != want {
			t.Fatalf("%v string changed: got %q want %q", kind, string(kind), want)
		}
	}
}
