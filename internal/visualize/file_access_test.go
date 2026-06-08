package visualize

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestExtractFileUseMapsKindsAndClassifiesPaths(t *testing.T) {
	cases := []struct {
		name         string
		step         VisualStep
		wantAction   string
		wantPath     string
		wantPathKind string
	}{
		{
			name: "read file",
			step: fileAccessStep("run-a", 1, VisualStepReadFile, []string{`C:\repo\src\session.go`}, map[string]string{
				"repo_root": `C:\repo`,
			}),
			wantAction:   FileActionRead,
			wantPath:     "src/session.go",
			wantPathKind: "source",
		},
		{
			name:         "inspect test",
			step:         fileAccessStep("run-a", 2, VisualStepInspectTest, []string{"tests/session_test.go"}, nil),
			wantAction:   FileActionRead,
			wantPath:     "tests/session_test.go",
			wantPathKind: "test",
		},
		{
			name:         "edit legacy",
			step:         fileAccessStep("run-a", 3, VisualStepEditFile, []string{"internal/legacy/session.go"}, nil),
			wantAction:   FileActionEdit,
			wantPath:     "internal/legacy/session.go",
			wantPathKind: "legacy",
		},
		{
			name: "step attributes can mark deprecated path",
			step: fileAccessStep("run-a", 4, VisualStepCallTool, []string{"internal/session"}, map[string]string{
				"is_deprecated": "true",
			}),
			wantAction:   FileActionOther,
			wantPath:     "internal/session",
			wantPathKind: "legacy",
		},
		{
			name:         "validation file is other",
			step:         fileAccessStep("run-a", 5, VisualStepRunTest, []string{"tests/session_test.go"}, nil),
			wantAction:   FileActionOther,
			wantPath:     "tests/session_test.go",
			wantPathKind: "test",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actions := ExtractFileUse(tt.step)
			if len(actions) != 1 {
				t.Fatalf("actions = %#v, want one action", actions)
			}
			action := actions[0]
			if action.Action != tt.wantAction || action.Path != tt.wantPath || action.Step != tt.step.Index {
				t.Fatalf("action = %#v, want action=%q path=%q step=%d", action, tt.wantAction, tt.wantPath, tt.step.Index)
			}
			if action.PathKind != tt.wantPathKind {
				t.Fatalf("path kind = %q, want %q for %#v", action.PathKind, tt.wantPathKind, action)
			}
		})
	}
}

func TestExtractFileUseIgnoresMissingFilesAndGenericTarget(t *testing.T) {
	if actions := ExtractFileUse(VisualStep{RunID: "run-a", Index: 1, Kind: VisualStepReadFile, Target: "target"}); len(actions) != 0 {
		t.Fatalf("actions = %#v, want none", actions)
	}

	step := VisualStep{RunID: "run-a", Index: 2, Kind: VisualStepReadFile, Target: "src/session.go"}
	actions := ExtractFileUse(step)
	if len(actions) != 1 || actions[0].Path != "src/session.go" || actions[0].Action != FileActionRead {
		t.Fatalf("actions = %#v, want target file read", actions)
	}
}

func TestBuildFileAccessCompareCountsReadsEditsAndStableRows(t *testing.T) {
	lanes := sampleFileAccessLanes()

	compare := BuildFileAccessCompare(lanes)
	if len(compare.Rows) != 3 {
		t.Fatalf("rows = %#v, want 3 rows", compare.Rows)
	}
	gotPaths := []string{compare.Rows[0].Path, compare.Rows[1].Path, compare.Rows[2].Path}
	wantPaths := []string{"src/legacy/session.go", "src/session.go", "tests/session_test.go"}
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths = %#v, want %#v", gotPaths, wantPaths)
	}

	source := findFileAccessRow(t, compare, "src/session.go")
	runA := source.Runs["run-a"]
	if runA.ReadCount != 1 || runA.EditCount != 2 || runA.FirstStep != 1 || runA.LastStep != 3 {
		t.Fatalf("run-a source use = %#v", runA)
	}
	if runA.FirstAction != FileActionRead || runA.LastAction != FileActionEdit {
		t.Fatalf("run-a source actions = %#v", runA)
	}
	if source.Runs["run-b"] != (FileUse{}) {
		t.Fatalf("run-b source zero use = %#v", source.Runs["run-b"])
	}

	tests := findFileAccessRow(t, compare, "tests/session_test.go")
	if tests.Runs["run-a"].ReadCount != 1 || tests.Runs["run-a"].FirstAction != FileActionRead {
		t.Fatalf("run-a test use = %#v", tests.Runs["run-a"])
	}
	if tests.Runs["run-b"].ReadCount != 0 || tests.Runs["run-b"].EditCount != 0 || tests.Runs["run-b"].FirstAction != "" {
		t.Fatalf("run-b test use = %#v", tests.Runs["run-b"])
	}
}

func TestBuildSearchScopeCompareSeparatesSearchScopesFromFileAccess(t *testing.T) {
	lanes := sampleFileAccessLanes()

	fileAccess := BuildFileAccessCompare(lanes)
	for _, row := range fileAccess.Rows {
		if row.Path == "src" || row.Path == "tests" {
			t.Fatalf("search scope leaked into file access rows: %#v", fileAccess.Rows)
		}
	}

	scopes := BuildSearchScopeCompare(lanes)
	src := findSearchScopeRow(t, scopes, "src")
	if src.Runs["run-a"].SearchCount != 1 || src.Runs["run-b"].SearchCount != 1 {
		t.Fatalf("src scope uses = %#v", src.Runs)
	}
	tests := findSearchScopeRow(t, scopes, "tests")
	if tests.Runs["run-a"].SearchCount != 1 || tests.Runs["run-b"].SearchCount != 0 {
		t.Fatalf("tests scope uses = %#v", tests.Runs)
	}
	if len(src.Runs["run-a"].Queries) != 1 || !strings.Contains(src.Runs["run-a"].Queries[0], "session") {
		t.Fatalf("src run-a queries = %#v", src.Runs["run-a"].Queries)
	}
}

func TestNormalizeFileAccessPathWithRootHandlesWindowsAndUnixPaths(t *testing.T) {
	cases := []struct {
		name string
		path string
		root string
		want string
	}{
		{
			name: "windows drive root",
			path: `D:\agent-vcr\internal\visualize\file_access.go`,
			root: `D:\agent-vcr`,
			want: "internal/visualize/file_access.go",
		},
		{
			name: "windows user root",
			path: `C:\Users\wangji\Desktop\repo\src\session.go`,
			root: `C:\Users\wangji\Desktop\repo`,
			want: "src/session.go",
		},
		{
			name: "unix user root",
			path: "/home/wangji/repo/tests/session_test.go",
			root: "/home/wangji/repo",
			want: "tests/session_test.go",
		},
		{
			name: "relative path",
			path: `.\src\session.go`,
			root: "",
			want: "src/session.go",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeFileAccessPathWithRoot(tt.path, tt.root); got != tt.want {
				t.Fatalf("NormalizeFileAccessPathWithRoot(%q, %q) = %q, want %q", tt.path, tt.root, got, tt.want)
			}
		})
	}
}

func TestBuildFileAccessCompareGolden(t *testing.T) {
	compare := BuildFileAccessCompare(sampleFileAccessLanes())
	data, err := json.MarshalIndent(compare, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	got := string(data) + "\n"
	want, err := os.ReadFile("../../testdata/visualize/file-access/compare.golden.json")
	if err != nil {
		t.Fatalf("ReadFile golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("file access compare golden mismatch\n got:\n%s\nwant:\n%s", got, string(want))
	}
}

func sampleFileAccessLanes() []BehaviorLane {
	return []BehaviorLane{
		{
			RunID: "run-a",
			Label: "source-first",
			Steps: []VisualStep{
				fileAccessSearchStep("run-a", 0, []string{"src", "tests"}, `rg "session" src tests`),
				fileAccessStep("run-a", 1, VisualStepReadFile, []string{`D:\repo\src\session.go`}, map[string]string{"repo_root": `D:\repo`}),
				fileAccessStep("run-a", 2, VisualStepEditFile, []string{"src/session.go"}, nil),
				fileAccessStep("run-a", 3, VisualStepEditFile, []string{"src/session.go"}, nil),
				fileAccessStep("run-a", 4, VisualStepInspectTest, []string{"tests/session_test.go"}, nil),
			},
		},
		{
			RunID: "run-b",
			Label: "legacy",
			Steps: []VisualStep{
				fileAccessSearchStep("run-b", 0, []string{"src"}, `rg "cookie" src`),
				fileAccessStep("run-b", 1, VisualStepReadFile, []string{"src/legacy/session.go"}, nil),
				fileAccessStep("run-b", 2, VisualStepRunTest, []string{"tests/session_test.go"}, nil),
			},
		},
	}
}

func fileAccessSearchStep(runID string, index int, scopes []string, query string) VisualStep {
	return VisualStep{
		RunID:       runID,
		StepID:      runID + "-search-step-" + strconv.Itoa(index),
		Index:       index,
		Kind:        VisualStepSearch,
		Summary:     "search " + query,
		Query:       query,
		Files:       scopes,
		Significant: true,
	}
}

func fileAccessStep(runID string, index int, kind VisualStepKind, files []string, attrs map[string]string) VisualStep {
	return VisualStep{
		RunID:       runID,
		StepID:      runID + "-file-step-" + strconv.Itoa(index),
		Index:       index,
		Kind:        kind,
		Summary:     string(kind),
		Files:       files,
		Significant: true,
		Attributes:  attrs,
	}
}

func findSearchScopeRow(t *testing.T, compare SearchScopeCompare, scope string) SearchScopeRow {
	t.Helper()
	for _, row := range compare.Rows {
		if row.Scope == scope {
			return row
		}
	}
	t.Fatalf("scope %q not found in %#v", scope, compare.Rows)
	return SearchScopeRow{}
}

func findFileAccessRow(t *testing.T, compare FileAccessCompare, path string) FileAccessRow {
	t.Helper()
	for _, row := range compare.Rows {
		if row.Path == path {
			return row
		}
	}
	t.Fatalf("row %q not found in %#v", path, compare.Rows)
	return FileAccessRow{}
}
