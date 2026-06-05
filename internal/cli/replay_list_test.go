package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	runlist "github.com/agent-vcr/agent-vcr/internal/list"
)

func TestListGolden(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	assertGolden(t, stdout, filepath.Join("replay", "list.txt"))
}

func TestListJSONParseable(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var runs []runlist.Summary
	if err := json.Unmarshal([]byte(stdout), &runs); err != nil {
		t.Fatalf("decode list JSON: %v\n%s", err, stdout)
	}
	if len(runs) != 2 || runs[0].RunID != "20260604T020000Z-simple" {
		t.Fatalf("runs = %#v", runs)
	}
}

func TestReplayGolden(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "replay", "latest")
	if err != nil {
		t.Fatalf("replay latest: %v", err)
	}
	assertGolden(t, stdout, filepath.Join("replay", "replay_simple.txt"))
}

func TestReplayJSONParseable(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "replay", "020000", "--json")
	if err != nil {
		t.Fatalf("replay --json: %v", err)
	}
	var replay analysis.Replay
	if err := json.Unmarshal([]byte(stdout), &replay); err != nil {
		t.Fatalf("decode replay JSON: %v\n%s", err, stdout)
	}
	if replay.RunID != "20260604T020000Z-simple" || len(replay.Timeline) == 0 {
		t.Fatalf("replay = %#v", replay)
	}
}

func TestReplayFilterRaw(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "replay", "latest", "--filter", "raw")
	if err != nil {
		t.Fatalf("replay --filter raw: %v", err)
	}
	want := "00:50 raw            raw event -> raw/unknown.json (32 bytes)\n"
	if !strings.HasSuffix(stdout, want) {
		t.Fatalf("raw replay output = %q, want suffix %q", stdout, want)
	}
}

func fixtureProject(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "testdata", "runs", "replay-list"))
}

func assertGolden(t *testing.T, got string, rel string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(data) {
		t.Fatalf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", rel, got, string(data))
	}
}
