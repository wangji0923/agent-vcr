package gitutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitutilRepoSnapshot(t *testing.T) {
	requireGit(t)
	repo := initRepo(t)

	root, err := FindRepoRoot(repo)
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	if !samePath(root, repo) {
		t.Fatalf("root = %q, want %q", root, repo)
	}
	head, err := CurrentHead(repo)
	if err != nil {
		t.Fatalf("CurrentHead: %v", err)
	}
	if len(head) < 7 {
		t.Fatalf("head looks invalid: %q", head)
	}
	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Fatal("branch should not be empty")
	}

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Status(repo)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(status, "tracked.txt") || !strings.Contains(status, "new.txt") {
		t.Fatalf("status did not include changed files: %q", status)
	}
	files, err := ChangedFiles(repo)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if !contains(files, "tracked.txt") || !contains(files, "new.txt") {
		t.Fatalf("changed files = %#v", files)
	}

	diff, err := Diff(repo)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(string(diff), "tracked.txt") {
		t.Fatalf("diff did not include tracked file: %s", string(diff))
	}
	summary := DiffSummary(diff)
	if summary["size_bytes"].(int) == 0 || summary["sha256"].(string) == "" {
		t.Fatalf("summary = %#v", summary)
	}
	snapshot, diffBytes, err := CaptureSnapshot(repo)
	if err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}
	if snapshot.Head != head || snapshot.Branch == "" || snapshot.DiffSHA256 == "" || len(diffBytes) == 0 {
		t.Fatalf("snapshot = %#v, diff len = %d", snapshot, len(diffBytes))
	}
}

func TestGitutilNonRepoGraceful(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Fatal("temp dir should not be a git repo")
	}
	if _, err := FindRepoRoot(dir); !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("FindRepoRoot err = %v, want ErrNotGitRepo", err)
	}
	if _, _, err := CaptureSnapshot(dir); !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("CaptureSnapshot err = %v, want ErrNotGitRepo", err)
	}
}

func TestChangedFilesDelta(t *testing.T) {
	before := Snapshot{Files: []string{"dirty.txt", "z.txt"}}
	after := Snapshot{Files: []string{"new.txt", "dirty.txt", "deleted.txt", "new.txt"}}

	got := ChangedFilesDelta(before, after)
	want := []string{"deleted.txt", "new.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("delta = %#v, want %#v", got, want)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "config", "user.email", "agent-vcr@example.local")
	run(t, dir, "config", "user.name", "Agent VCR Test")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "tracked.txt")
	run(t, dir, "commit", "-m", "initial")
	return dir
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	aInfo, aErr := os.Stat(filepath.Clean(a))
	bInfo, bErr := os.Stat(filepath.Clean(b))
	if aErr == nil && bErr == nil {
		return os.SameFile(aInfo, bInfo)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
