package codex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/gitutil"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type HookRunOptions struct {
	Stdin      io.Reader
	ProjectDir string
	ConfigPath string
}

func RunHook(ctx context.Context, opts HookRunOptions) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = nil
		}
	}()

	reader := opts.Stdin
	if reader == nil {
		reader = os.Stdin
	}
	if mode := os.Getenv("AGENT_VCR_CAPTURE_MODE"); mode == "jsonl" || mode == "generic" {
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}
	rawData, err := io.ReadAll(reader)
	if err != nil || len(strings.TrimSpace(string(rawData))) == 0 {
		return nil
	}

	input, rawMap, err := ParseHookInput(rawData)
	if err != nil {
		return nil
	}
	projectDir, err := resolveProjectDir(input.Cwd, opts.ProjectDir)
	if err != nil {
		return nil
	}
	cfg, _, err := config.Load(projectDir, opts.ConfigPath)
	if err != nil {
		cfg = config.Default()
	}

	store, err := openOrCreateSessionRun(projectDir, input)
	if err != nil {
		return nil
	}
	rawRef, err := writeRawArtifact(store, input.HookEventName, rawData)
	if err != nil {
		return nil
	}

	rawEvent := trace.RawEvent{
		Source: trace.Source{
			Adapter:      SourceAdapter,
			Agent:        SourceAgent,
			RawEventType: input.HookEventName,
		},
		Data:       rawData,
		Payload:    rawMap,
		ReceivedAt: nowUTC(),
		RawRef:     &rawRef,
	}

	events, err := New().Normalize(ctx, rawEvent)
	if err != nil {
		_, _ = store.SaveRawEvent(trace.RawEvent{
			Source:     rawEvent.Source,
			Data:       rawData,
			Payload:    trace.Payload{"reason": "normalize_error", "error": err.Error()},
			ReceivedAt: rawEvent.ReceivedAt,
			RawRef:     &rawRef,
		})
		return nil
	}

	for _, event := range events {
		if event.Type == trace.EventRaw {
			_, _ = store.SaveRawEvent(trace.RawEvent{
				Source:     event.Source,
				Data:       rawData,
				Payload:    event.Payload,
				ReceivedAt: event.Timestamp,
				RawRef:     event.RawRef,
			})
			continue
		}
		event.RunID = store.RunID
		if event.RawRef == nil {
			event.RawRef = &rawRef
		}
		annotateGitSnapshot(store, cfg, input, &event)
		_ = store.Append(event)
	}

	if input.HookEventName == eventStop {
		_ = markRunCompleted(store)
	}
	return nil
}

func resolveProjectDir(values ...string) (string, error) {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		return filepath.Abs(value)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(cwd)
}

func openOrCreateSessionRun(projectDir string, input HookInput) (*trace.Store, error) {
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	var store *trace.Store
	err = withSessionLock(projectDir, func() error {
		sessions, err := readSessionMap(projectDir)
		if err != nil {
			return err
		}
		key := sessionKey(input)
		runID := sessions[key]
		if runID == "" {
			runID = newCodexRunID(input)
			sessions[key] = runID
			if err := writeSessionMap(projectDir, sessions); err != nil {
				return err
			}
		}
		store = storeForRun(projectDir, runID)
		return ensureRun(store, input)
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func storeForRun(projectDir, runID string) *trace.Store {
	runsDir := filepath.Join(projectDir, ".agent-vcr", "runs")
	return &trace.Store{
		ProjectDir: projectDir,
		RunsDir:    runsDir,
		RunID:      runID,
		RunDir:     filepath.Join(runsDir, runID),
	}
}

func ensureRun(store *trace.Store, input HookInput) error {
	for _, rel := range []string{"blobs", "patches", "raw"} {
		if err := os.MkdirAll(store.Path(rel), 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(store.Path("metadata.json")); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	meta := trace.Metadata{
		SchemaVersion: trace.SchemaVersion,
		RunID:         store.RunID,
		Source:        SourceAdapter,
		Status:        trace.RunStatusRunning,
		Cwd:           store.ProjectDir,
		StartedAt:     nowUTC(),
		Capabilities:  capabilityMap(New().Capabilities()),
		Summary: trace.Payload{
			"session_id": input.SessionID,
			"adapter":    AdapterName,
		},
	}
	if root, err := gitutil.FindRepoRoot(store.ProjectDir); err == nil {
		meta.RepoRoot = root
	}
	if head, err := gitutil.CurrentHead(store.ProjectDir); err == nil {
		meta.GitSHA = head
	}
	if branch, err := gitutil.CurrentBranch(store.ProjectDir); err == nil {
		meta.Branch = branch
	}
	return store.WriteMetadata(meta)
}

func capabilityMap(caps any) map[string]bool {
	data, err := json.Marshal(caps)
	if err != nil {
		return nil
	}
	values := map[string]bool{}
	_ = json.Unmarshal(data, &values)
	return values
}

func markRunCompleted(store *trace.Store) error {
	meta, err := store.ReadMetadata()
	if err != nil {
		return err
	}
	now := nowUTC()
	meta.Status = trace.RunStatusCompleted
	meta.EndedAt = &now
	return store.WriteMetadata(meta)
}

func sessionKey(input HookInput) string {
	if input.SessionID != "" {
		return "codex:" + input.SessionID
	}
	if input.TranscriptPath != "" {
		return "codex-transcript:" + sha256String(input.TranscriptPath)
	}
	return "codex:default"
}

func newCodexRunID(input HookInput) string {
	short := shortSessionID(input.SessionID)
	if short == "" {
		short = randomSuffix()
	}
	return time.Now().UTC().Format("2006-01-02-150405") + "-codex-" + short
}

func shortSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	safe := regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(sessionID, "-")
	safe = strings.Trim(safe, "-")
	if len(safe) <= 12 {
		return safe
	}
	return safe[:12]
}

func randomSuffix() string {
	var data [4]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(data[:])
}

func readSessionMap(projectDir string) (map[string]string, error) {
	path := sessionMapPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]string{}, nil
	}
	var sessions map[string]string
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func writeSessionMap(projectDir string, sessions map[string]string) error {
	path := sessionMapPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func sessionMapPath(projectDir string) string {
	return filepath.Join(projectDir, ".agent-vcr", "state", "sessions.json")
}

func withSessionLock(projectDir string, fn func() error) error {
	lockDir := filepath.Join(projectDir, ".agent-vcr", "state", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(lockDir, "sessions.lock")
	deadline := time.Now().UTC().Add(10 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
			_ = file.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !os.IsExist(err) && !os.IsPermission(err) {
			return err
		}
		if time.Now().UTC().After(deadline) {
			return fmt.Errorf("timed out waiting for session lock: %s", lockPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func writeRawArtifact(store *trace.Store, eventName string, data []byte) (trace.ArtifactRef, error) {
	if err := os.MkdirAll(store.Path("raw"), 0o755); err != nil {
		return trace.ArtifactRef{}, err
	}
	name := safeRawName(eventName) + "-" + time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + randomSuffix() + ".json"
	path := store.Path("raw", name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return trace.ArtifactRef{}, err
	}
	sum := sha256.Sum256(data)
	return trace.ArtifactRef{
		Kind:      trace.ArtifactRaw,
		Path:      filepath.ToSlash(filepath.Join("raw", name)),
		SHA256:    hex.EncodeToString(sum[:]),
		SizeBytes: int64(len(data)),
		MimeType:  "application/json",
	}, nil
}

func safeRawName(eventName string) string {
	eventName = strings.TrimSpace(eventName)
	if eventName == "" {
		eventName = "hook"
	}
	eventName = regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(eventName, "-")
	return strings.Trim(eventName, "-")
}

func annotateGitSnapshot(store *trace.Store, cfg config.Config, input HookInput, event *trace.Event) {
	if !cfg.Capture.GitDiff {
		return
	}
	if input.HookEventName != eventPreToolUse && input.HookEventName != eventPostToolUse {
		return
	}
	if event.Type != trace.EventToolCall && event.Type != trace.EventToolResult {
		return
	}
	snapshot, diff, err := gitutil.CaptureSnapshot(store.ProjectDir)
	if err != nil {
		return
	}
	if event.Payload == nil {
		event.Payload = trace.Payload{}
	}
	key := "git_before"
	if input.HookEventName == eventPostToolUse {
		key = "git_after"
		event.Payload["changed_files"] = snapshot.Files
	}
	event.Payload[key] = snapshot
	if len(diff) == 0 {
		return
	}
	if int64(len(diff)) > cfg.Storage.MaxBlobBytes {
		event.Payload["patch_truncated"] = true
		return
	}
	ref, err := store.WritePatch(string(event.Type)+"-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".patch", diff)
	if err != nil {
		return
	}
	event.Artifacts = append(event.Artifacts, ref)
}
