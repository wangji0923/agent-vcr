package trace

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrStoreNotImplemented = errors.New("trace store implementation is owned by module 02-config-trace-store")

type Store struct {
	ProjectDir string
	RunsDir    string
	RunID      string
	RunDir     string
}

func CreateRun(projectDir string, source string) (*Store, error) {
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

	runID := newRunID()
	store := storeFor(projectDir, runID)
	if err := os.MkdirAll(filepath.Join(store.RunDir, "blobs"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(store.RunDir, "patches"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(store.RunDir, "raw"), 0o755); err != nil {
		return nil, err
	}

	meta := Metadata{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		Source:        source,
		Status:        RunStatusRunning,
		Cwd:           projectDir,
		StartedAt:     time.Now().UTC(),
	}
	if err := store.WriteMetadata(meta); err != nil {
		return nil, err
	}
	return store, nil
}

func OpenRun(projectDir string, runID string) (*Store, error) {
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
	store := storeFor(projectDir, runID)
	info, err := os.Stat(store.RunDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("run path is not a directory: %s", store.RunDir)
	}
	return store, nil
}

func (s *Store) Append(event Event) error {
	_, err := s.appendEvent(event)
	return err
}

func (s *Store) appendEvent(event Event) (Event, error) {
	var appended Event
	err := WithRunLock(s.ProjectDir, s.RunID, func() error {
		if IsAgentSpecificEventType(event.Type) {
			return fmt.Errorf("agent-specific event type is not allowed: %s", event.Type)
		}
		if event.SchemaVersion == "" {
			event.SchemaVersion = SchemaVersion
		}
		if event.EventID == "" {
			event.EventID = newID("evt")
		}
		if event.RunID == "" {
			event.RunID = s.RunID
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now().UTC()
		}
		if event.Type == "" {
			return errors.New("event type must not be empty")
		}

		lastIndex, err := s.lastEventIndex()
		if err != nil {
			return err
		}
		event.EventIndex = lastIndex + 1

		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(s.RunDir, 0o755); err != nil {
			return err
		}
		file, err := os.OpenFile(s.Path("trace.ndjson"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		appended = event
		return nil
	})
	return appended, err
}

func (s *Store) WriteMetadata(meta Metadata) error {
	if meta.SchemaVersion == "" {
		meta.SchemaVersion = SchemaVersion
	}
	if meta.RunID == "" {
		meta.RunID = s.RunID
	}
	if meta.Cwd == "" {
		meta.Cwd = s.ProjectDir
	}
	if meta.Status == "" {
		meta.Status = RunStatusUnknown
	}
	if meta.StartedAt.IsZero() {
		meta.StartedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(s.RunDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path("metadata.json"), append(data, '\n'), 0o644)
}

func (s *Store) ReadMetadata() (Metadata, error) {
	data, err := os.ReadFile(s.Path("metadata.json"))
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func (s *Store) WriteBlob(name string, data []byte, mime string) (ArtifactRef, error) {
	return s.writeArtifact(ArtifactBlob, "blobs", name, data, mime)
}

func (s *Store) WritePatch(name string, data []byte) (ArtifactRef, error) {
	return s.writeArtifact(ArtifactPatch, "patches", name, data, "text/x-diff")
}

func (s *Store) Path(parts ...string) string {
	all := append([]string{s.RunDir}, parts...)
	return filepath.Join(all...)
}

func (s *Store) SaveRawEvent(raw RawEvent) (Event, error) {
	rawRef := raw.RawRef
	if rawRef == nil {
		data := raw.Data
		if len(data) == 0 && raw.Payload != nil {
			encoded, err := json.Marshal(raw.Payload)
			if err != nil {
				return Event{}, err
			}
			data = encoded
		}
		ref, err := s.writeArtifact(ArtifactRaw, "raw", newID("raw")+".json", data, "application/json")
		if err != nil {
			return Event{}, err
		}
		rawRef = &ref
	}

	event := NewRawEvent(s.RunID, raw.Source, *rawRef)
	event.Payload = raw.Payload
	if !raw.ReceivedAt.IsZero() {
		event.Timestamp = raw.ReceivedAt.UTC()
	}
	return s.appendEvent(event)
}

func storeFor(projectDir, runID string) *Store {
	runsDir := filepath.Join(projectDir, ".agent-vcr", "runs")
	return &Store{
		ProjectDir: projectDir,
		RunsDir:    runsDir,
		RunID:      runID,
		RunDir:     filepath.Join(runsDir, runID),
	}
}

func newRunID() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + strings.TrimPrefix(newID("run"), "run_")
}

func (s *Store) lastEventIndex() (int64, error) {
	file, err := os.Open(s.Path("trace.ndjson"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	var maxIndex int64
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item struct {
			EventIndex int64 `json:"event_index"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if item.EventIndex > maxIndex {
			maxIndex = item.EventIndex
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return maxIndex, nil
}

func (s *Store) writeArtifact(kind ArtifactKind, dirName string, name string, data []byte, mime string) (ArtifactRef, error) {
	if err := os.MkdirAll(s.Path(dirName), 0o755); err != nil {
		return ArtifactRef{}, err
	}
	name = safeArtifactName(name)
	path := s.Path(dirName, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ArtifactRef{}, err
	}
	sum := sha256.Sum256(data)
	relative := filepath.ToSlash(filepath.Join(dirName, name))
	return ArtifactRef{
		Kind:      kind,
		Path:      relative,
		SHA256:    hex.EncodeToString(sum[:]),
		SizeBytes: int64(len(data)),
		MimeType:  mime,
	}, nil
}

func safeArtifactName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return newID("artifact")
	}
	name = strings.ReplaceAll(name, "\x00", "")
	return name
}
