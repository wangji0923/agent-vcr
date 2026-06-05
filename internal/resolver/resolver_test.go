package resolver

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveLatestAndPartial(t *testing.T) {
	project := fixtureProject(t)
	latest, err := Resolve(project, "latest")
	if err != nil {
		t.Fatalf("Resolve latest: %v", err)
	}
	if latest != "20260604T020000Z-simple" {
		t.Fatalf("latest = %q", latest)
	}
	exact, err := Resolve(project, "20260604T010000Z-old")
	if err != nil || exact != "20260604T010000Z-old" {
		t.Fatalf("exact = %q err=%v", exact, err)
	}
	partial, err := Resolve(project, "020000")
	if err != nil || partial != "20260604T020000Z-simple" {
		t.Fatalf("partial = %q err=%v", partial, err)
	}
}

func TestResolveAmbiguous(t *testing.T) {
	_, err := Resolve(fixtureProject(t), "20260604T0")
	if !errors.Is(err, ErrRunAmbiguous) {
		t.Fatalf("err = %v, want ErrRunAmbiguous", err)
	}
	var ambiguous AmbiguousError
	if !errors.As(err, &ambiguous) || len(ambiguous.Candidates) != 2 {
		t.Fatalf("ambiguous = %#v err=%v", ambiguous, err)
	}
}

func fixtureProject(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "testdata", "runs", "replay-list"))
}
