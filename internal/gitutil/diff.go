package gitutil

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Status(cwd string) (string, error) {
	output, err := runGit(cwd, "status", "--porcelain=v1")
	if err != nil {
		return "", err
	}
	return output, nil
}

func Diff(cwd string) ([]byte, error) {
	unstaged, err := runGit(cwd, "diff", "--binary")
	if err != nil {
		return nil, err
	}
	cached, err := runGit(cwd, "diff", "--cached", "--binary")
	if err != nil {
		return nil, err
	}
	return []byte(unstaged + cached), nil
}

func ChangedFiles(cwd string) ([]string, error) {
	status, err := Status(cwd)
	if err != nil {
		return nil, err
	}
	return ChangedFilesFromStatus(status), nil
}

func ChangedFilesFromStatus(status string) []string {
	seen := map[string]bool{}
	var files []string
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = strings.TrimSpace(parts[len(parts)-1])
		}
		path = strings.Trim(path, `"`)
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	return files
}

func DiffSummary(diff []byte) map[string]any {
	sum := sha256.Sum256(diff)
	files := map[string]bool{}
	for _, line := range strings.Split(string(diff), "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) >= 4 {
			path := strings.TrimPrefix(parts[3], "b/")
			files[path] = true
		}
	}
	return map[string]any{
		"size_bytes": len(diff),
		"sha256":     hex.EncodeToString(sum[:]),
		"file_count": len(files),
	}
}
