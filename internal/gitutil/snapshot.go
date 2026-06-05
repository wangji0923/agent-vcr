package gitutil

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

type Snapshot struct {
	Head       string   `json:"head,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Status     string   `json:"status,omitempty"`
	DiffSHA256 string   `json:"diff_sha256,omitempty"`
	Files      []string `json:"files,omitempty"`
}

func CaptureSnapshot(cwd string) (Snapshot, []byte, error) {
	if !IsGitRepo(cwd) {
		return Snapshot{}, nil, ErrNotGitRepo
	}
	head, err := CurrentHead(cwd)
	if err != nil {
		return Snapshot{}, nil, err
	}
	branch, err := CurrentBranch(cwd)
	if err != nil {
		return Snapshot{}, nil, err
	}
	status, err := Status(cwd)
	if err != nil {
		return Snapshot{}, nil, err
	}
	diff, err := Diff(cwd)
	if err != nil {
		return Snapshot{}, nil, err
	}
	sum := sha256.Sum256(diff)
	return Snapshot{
		Head:       head,
		Branch:     branch,
		Status:     status,
		DiffSHA256: hex.EncodeToString(sum[:]),
		Files:      ChangedFilesFromStatus(status),
	}, diff, nil
}

func ChangedFilesDelta(before, after Snapshot) []string {
	beforeFiles := make(map[string]bool, len(before.Files))
	for _, file := range before.Files {
		if file == "" {
			continue
		}
		beforeFiles[file] = true
	}

	seen := map[string]bool{}
	var delta []string
	for _, file := range after.Files {
		if file == "" || beforeFiles[file] || seen[file] {
			continue
		}
		seen[file] = true
		delta = append(delta, file)
	}
	sort.Strings(delta)
	return delta
}
