package trace

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCreateRunWritesMetadataAndDirectories(t *testing.T) {
	project := t.TempDir()
	store, err := CreateRun(project, "test")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	for _, rel := range []string{"metadata.json", "blobs", "patches", "raw"} {
		if _, err := os.Stat(store.Path(rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
	meta, err := store.ReadMetadata()
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	if meta.RunID != store.RunID || meta.Status != RunStatusRunning || meta.Source != "test" {
		t.Fatalf("metadata = %#v", meta)
	}
}

func TestAppendWritesOneNDJSONEventPerLine(t *testing.T) {
	store, err := CreateRun(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(NewEvent(store.RunID, EventUserPrompt, Source{Adapter: "fixture"})); err != nil {
		t.Fatalf("Append first: %v", err)
	}
	if err := store.Append(NewEvent(store.RunID, EventToolCall, Source{Adapter: "fixture"})); err != nil {
		t.Fatalf("Append second: %v", err)
	}

	events := readTraceEvents(t, store.Path("trace.ndjson"))
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].EventIndex != 1 || events[1].EventIndex != 2 {
		t.Fatalf("event indexes = %d, %d", events[0].EventIndex, events[1].EventIndex)
	}
	if events[0].EventID == "" || events[1].EventID == "" {
		t.Fatal("Append should assign event IDs")
	}
}

func TestAppendRejectsAgentSpecificEventType(t *testing.T) {
	store, err := CreateRun(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	err = store.Append(NewEvent(store.RunID, EventType("codex_tool_call"), Source{Adapter: "fixture"}))
	if err == nil {
		t.Fatal("Append accepted agent-specific event type")
	}
}

func TestWriteBlobPatchAndRawEvent(t *testing.T) {
	store, err := CreateRun(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	blob, err := store.WriteBlob("../output.txt", []byte("hello"), "text/plain")
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	if blob.Kind != ArtifactBlob || blob.SHA256 == "" || blob.Path != "blobs/output.txt" {
		t.Fatalf("blob ref = %#v", blob)
	}
	patch, err := store.WritePatch("change.patch", []byte("diff --git a/a b/a\n"))
	if err != nil {
		t.Fatalf("WritePatch: %v", err)
	}
	if patch.Kind != ArtifactPatch || patch.Path != "patches/change.patch" {
		t.Fatalf("patch ref = %#v", patch)
	}
	rawEvent, err := store.SaveRawEvent(RawEvent{
		Source:     Source{Adapter: "fixture", RawEventType: "unknown"},
		Data:       []byte(`{"agent_specific":"kept"}`),
		Payload:    Payload{"normalize_error": "unknown event"},
		ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveRawEvent: %v", err)
	}
	if rawEvent.Type != EventRaw || rawEvent.RawRef == nil || rawEvent.RawRef.Kind != ArtifactRaw {
		t.Fatalf("raw event = %#v", rawEvent)
	}
	if _, err := os.Stat(store.Path(rawEvent.RawRef.Path)); err != nil {
		t.Fatalf("raw artifact missing: %v", err)
	}
}

func TestWriteMetadataUpdatesMetadata(t *testing.T) {
	store, err := CreateRun(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.ReadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	ended := time.Now().UTC()
	meta.Status = RunStatusCompleted
	meta.EndedAt = &ended
	meta.Summary = Payload{"events": 1}
	if err := store.WriteMetadata(meta); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}
	updated, err := store.ReadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != RunStatusCompleted || updated.EndedAt == nil {
		t.Fatalf("updated metadata = %#v", updated)
	}
}

func TestResolveLatestAndPrefix(t *testing.T) {
	project := t.TempDir()
	oldStore, err := CreateRun(project, "old")
	if err != nil {
		t.Fatal(err)
	}
	oldMeta, err := oldStore.ReadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	oldMeta.StartedAt = time.Now().UTC().Add(-time.Hour)
	if err := oldStore.WriteMetadata(oldMeta); err != nil {
		t.Fatal(err)
	}

	newStore, err := CreateRun(project, "new")
	if err != nil {
		t.Fatal(err)
	}
	newMeta, err := newStore.ReadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	newMeta.StartedAt = time.Now().UTC()
	if err := newStore.WriteMetadata(newMeta); err != nil {
		t.Fatal(err)
	}

	latest, err := ResolveRunID(project, "latest")
	if err != nil {
		t.Fatalf("ResolveRunID latest: %v", err)
	}
	if latest != newStore.RunID {
		t.Fatalf("latest = %q, want %q", latest, newStore.RunID)
	}
	prefixRef := newStore.RunID[:len(newStore.RunID)-1]
	prefix, err := ResolveRunID(project, prefixRef)
	if err != nil {
		t.Fatalf("ResolveRunID prefix: %v", err)
	}
	if prefix != newStore.RunID {
		t.Fatalf("prefix = %q, want %q", prefix, newStore.RunID)
	}
}

func TestConcurrentAppendDoesNotCorruptTrace(t *testing.T) {
	store, err := CreateRun(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	const count = 25
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.Append(NewEvent(store.RunID, EventToolResult, Source{Adapter: "fixture"}))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Append returned error: %v", err)
		}
	}
	events := readTraceEvents(t, store.Path("trace.ndjson"))
	if len(events) != count {
		t.Fatalf("len(events) = %d, want %d", len(events), count)
	}
	seen := map[int64]bool{}
	for _, event := range events {
		if seen[event.EventIndex] {
			t.Fatalf("duplicate event index %d", event.EventIndex)
		}
		seen[event.EventIndex] = true
	}
}

func TestListRunsSkipsMissingOrCorruptMetadata(t *testing.T) {
	project := t.TempDir()
	valid, err := CreateRun(project, "valid")
	if err != nil {
		t.Fatal(err)
	}
	missingDir := filepath.Join(project, ".agent-vcr", "runs", "missing-meta")
	if err := os.MkdirAll(missingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corruptDir := filepath.Join(project, ".agent-vcr", "runs", "corrupt-meta")
	if err := os.MkdirAll(corruptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corruptDir, "metadata.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	runs, err := ListRuns(project)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != valid.RunID {
		t.Fatalf("runs = %#v, want only %s", runs, valid.RunID)
	}
}

func readTraceEvents(t *testing.T, path string) []Event {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", scanner.Text(), err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}
