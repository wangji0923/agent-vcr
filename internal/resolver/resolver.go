package resolver

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	runlist "github.com/agent-vcr/agent-vcr/internal/list"
)

var (
	ErrRunNotFound  = errors.New("run not found")
	ErrRunAmbiguous = errors.New("run reference is ambiguous")
)

type AmbiguousError struct {
	Ref        string
	Candidates []string
}

func (err AmbiguousError) Error() string {
	return fmt.Sprintf("%v: %s", ErrRunAmbiguous, strings.Join(err.Candidates, ", "))
}

func (err AmbiguousError) Unwrap() error {
	return ErrRunAmbiguous
}

func Resolve(projectDir string, ref string) (string, error) {
	runs, err := runlist.Runs(projectDir)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", ErrRunNotFound
	}

	ref = strings.TrimSpace(ref)
	if ref == "" || ref == "latest" {
		return runs[0].RunID, nil
	}

	for _, run := range runs {
		if run.RunID == ref {
			return run.RunID, nil
		}
	}
	matches := matchingRuns(runs, ref, true)
	if len(matches) == 0 {
		matches = matchingRuns(runs, ref, false)
	}
	switch len(matches) {
	case 0:
		return "", ErrRunNotFound
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", AmbiguousError{Ref: ref, Candidates: matches}
	}
}

func matchingRuns(runs []runlist.Summary, ref string, prefixOnly bool) []string {
	var matches []string
	for _, run := range runs {
		if prefixOnly {
			if strings.HasPrefix(run.RunID, ref) {
				matches = append(matches, run.RunID)
			}
			continue
		}
		if strings.Contains(run.RunID, ref) {
			matches = append(matches, run.RunID)
		}
	}
	return matches
}
