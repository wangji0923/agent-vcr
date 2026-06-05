package behavior

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSignatureCacheReadWrite(t *testing.T) {
	runDir := t.TempDir()
	signature := buildSignatureFromTimelineAt(Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_cache",
		Steps:         []Step{{Kind: StepSearch, Query: "cache"}},
	}, SignatureOptions{}, fixedSignatureTime(), "sha256:cache")

	if err := WriteSignatureCache(runDir, signature); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	if _, err := os.Stat(SignatureCachePath(runDir)); err != nil {
		t.Fatalf("cache file was not written: %v", err)
	}
	got, err := ReadSignatureCache(runDir)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if got.RunID != signature.RunID || got.SourceTraceHash != signature.SourceTraceHash || len(got.Steps) != 1 {
		t.Fatalf("unexpected cached signature: %#v", got)
	}
}

func TestSignatureCacheMissBuildsFromTimeline(t *testing.T) {
	runDir := t.TempDir()
	if _, err := ReadSignatureCache(runDir); !errors.Is(err, ErrSignatureCacheMiss) {
		t.Fatalf("missing cache should return ErrSignatureCacheMiss, got %v", err)
	}

	result, err := LoadOrBuildSignatureCache(runDir, Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_cache",
		Steps:         []Step{{Kind: StepRunBuild, Command: "go build ./cmd/agent-vcr"}},
	}, SignatureOptions{})
	if err != nil {
		t.Fatalf("load or build cache: %v", err)
	}
	if result.CacheHit {
		t.Fatalf("first load should rebuild on cache miss")
	}
	if result.Signature.RunID != "run_cache" || len(result.Signature.Steps) != 1 {
		t.Fatalf("unexpected rebuilt signature: %#v", result.Signature)
	}
	if _, err := os.Stat(filepath.Join(runDir, "behavior", "signature.json")); err != nil {
		t.Fatalf("cache file should exist after rebuild: %v", err)
	}
}

func TestSignatureCacheInvalidatesOnTraceHashChange(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, "trace.ndjson"), []byte(`{"event_id":"evt_1"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	first, err := LoadOrBuildSignatureCache(runDir, Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_cache",
		Steps:         []Step{{Kind: StepSearch, Query: "first"}},
	}, SignatureOptions{})
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	if first.CacheHit || first.TraceHash == "" || first.Signature.SourceTraceHash == "" {
		t.Fatalf("expected trace-backed cache miss rebuild: %#v", first)
	}

	second, err := LoadOrBuildSignatureCache(runDir, Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_cache",
		Steps:         []Step{{Kind: StepSearch, Query: "should not replace cached"}},
	}, SignatureOptions{})
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if !second.CacheHit {
		t.Fatalf("same trace hash should hit cache")
	}
	if second.Signature.Steps[0].Query != "first" {
		t.Fatalf("cache hit should preserve cached signature: %#v", second.Signature.Steps[0])
	}

	if err := os.WriteFile(filepath.Join(runDir, "trace.ndjson"), []byte(`{"event_id":"evt_2"}`+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite trace: %v", err)
	}
	third, err := LoadOrBuildSignatureCache(runDir, Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_cache",
		Steps:         []Step{{Kind: StepSearch, Query: "rebuilt"}},
	}, SignatureOptions{})
	if err != nil {
		t.Fatalf("third load: %v", err)
	}
	if third.CacheHit {
		t.Fatalf("changed trace hash should invalidate cache")
	}
	if third.TraceHash == first.TraceHash {
		t.Fatalf("trace hash should change after trace rewrite")
	}
	if third.Signature.Steps[0].Query != "rebuilt" {
		t.Fatalf("stale cache should be rebuilt: %#v", third.Signature.Steps[0])
	}
}
