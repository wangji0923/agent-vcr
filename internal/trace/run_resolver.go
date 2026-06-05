package trace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrRunNotFound  = errors.New("run not found")
	ErrRunAmbiguous = errors.New("run reference is ambiguous")
)

func ListRuns(projectDir string) ([]Metadata, error) {
	if projectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		projectDir = cwd
	}
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	runsDir := filepath.Join(projectDir, ".agent-vcr", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Metadata{}, nil
		}
		return nil, err
	}

	runs := make([]Metadata, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		store := storeFor(projectDir, entry.Name())
		meta, err := store.ReadMetadata()
		if err != nil {
			continue
		}
		runs = append(runs, meta)
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs, nil
}

func ResolveRunID(projectDir string, ref string) (string, error) {
	if projectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		projectDir = cwd
	}
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", err
	}
	runsDir := filepath.Join(projectDir, ".agent-vcr", "runs")
	ref = strings.TrimSpace(ref)
	if ref == "" || ref == "latest" {
		runs, err := ListRuns(projectDir)
		if err != nil {
			return "", err
		}
		if len(runs) == 0 {
			return "", ErrRunNotFound
		}
		return runs[0].RunID, nil
	}

	exact := filepath.Join(runsDir, ref)
	if info, err := os.Stat(exact); err == nil && info.IsDir() {
		return ref, nil
	}

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrRunNotFound
		}
		return "", err
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ref) {
			matches = append(matches, entry.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", ErrRunNotFound
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("%w: %s", ErrRunAmbiguous, strings.Join(matches, ", "))
	}
}
