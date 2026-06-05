package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var ErrNotGitRepo = errors.New("not a git repository")

func FindRepoRoot(cwd string) (string, error) {
	output, err := runGit(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotGitRepo
	}
	return strings.TrimSpace(output), nil
}

func CurrentHead(cwd string) (string, error) {
	output, err := runGit(cwd, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func CurrentSHA(cwd string) (string, error) {
	return CurrentHead(cwd)
}

func CurrentBranch(cwd string) (string, error) {
	output, err := runGit(cwd, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(output)
	if branch == "" {
		return "detached", nil
	}
	return branch, nil
}

func IsGitRepo(cwd string) bool {
	_, err := FindRepoRoot(cwd)
	return err == nil
}

func runGit(cwd string, args ...string) (string, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
