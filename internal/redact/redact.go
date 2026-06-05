package redact

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type Redactor struct {
	enabled        bool
	redactEnvFiles bool
	patterns       []compiledPattern
	paths          []string
}

type compiledPattern struct {
	name  string
	regex *regexp.Regexp
}

var builtInPatterns = []config.PatternConfig{
	{Name: "anthropic_api_key", Regex: `sk-ant-[A-Za-z0-9_-]{20,}`},
	{Name: "openai_api_key", Regex: `sk-[A-Za-z0-9_-]{20,}`},
	{Name: "jwt", Regex: `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`},
	{Name: "private_key_block", Regex: `(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`},
	{Name: "private_key_header", Regex: `-----BEGIN [A-Z ]*PRIVATE KEY-----`},
	{Name: "common_secret_assignment", Regex: `(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*["']?[A-Za-z0-9_\-./+=]{16,}["']?`},
}

var envPathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)([A-Za-z]:\\[^\s"'<>]*\\\.env(?:\.[^\s"'<>]*)?|[./~A-Za-z0-9_-][^\s"'<>]*/\.env(?:\.[^\s"'<>]*)?|\.env(?:\.[^\s"'<>]*)?)`),
	regexp.MustCompile(`(?i)([A-Za-z]:\\[^\s"'<>]*\\secrets\\[^\s"'<>]+|[./~A-Za-z0-9_-][^\s"'<>]*/secrets/[^\s"'<>]+|secrets[/\\][^\s"'<>]+)`),
	regexp.MustCompile(`(?i)([A-Za-z]:\\[^\s"'<>]*\\id_rsa|[./~A-Za-z0-9_-][^\s"'<>]*/id_rsa|id_rsa)`),
}

func New(cfg config.RedactConfig) (*Redactor, error) {
	patterns := append([]config.PatternConfig{}, builtInPatterns...)
	patterns = append(patterns, cfg.Patterns...)

	compiled := make([]compiledPattern, 0, len(patterns))
	for _, pattern := range patterns {
		regex, err := regexp.Compile(pattern.Regex)
		if err != nil {
			return nil, fmt.Errorf("compile redaction pattern %q: %w", pattern.Name, err)
		}
		compiled = append(compiled, compiledPattern{name: pattern.Name, regex: regex})
	}

	return &Redactor{
		enabled:        cfg.Enabled,
		redactEnvFiles: cfg.RedactEnvFiles,
		patterns:       compiled,
		paths:          append([]string{}, cfg.Paths...),
	}, nil
}

func MaskString(input string, cfg config.RedactConfig) (string, error) {
	redactor, err := New(cfg)
	if err != nil {
		return "", err
	}
	return redactor.String(input), nil
}

func ApplyToEvent(event trace.Event, cfg config.RedactConfig) trace.Event {
	redactor, err := New(cfg)
	if err != nil {
		return event
	}
	return redactor.Event(event)
}

func ApplyToBytes(data []byte, cfg config.RedactConfig) []byte {
	redactor, err := New(cfg)
	if err != nil {
		return data
	}
	return redactor.Bytes(data)
}

func RedactRun(projectDir, runID string, outputDir string) error {
	if strings.TrimSpace(projectDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectDir = cwd
	}
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	runID, err = trace.ResolveRunID(projectDir, runID)
	if err != nil {
		return err
	}
	source, err := trace.OpenRun(projectDir, runID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Join(source.RunsDir, runID+"-redacted")
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	if samePath(source.RunDir, outputDir) {
		return fmt.Errorf("redacted output must not overwrite source run: %s", outputDir)
	}
	if err := prepareRedactedOutput(source.RunsDir, outputDir); err != nil {
		return err
	}

	cfg := config.Default().Redaction
	artifactRefs, err := copyArtifactDirs(source.RunDir, outputDir, cfg)
	if err != nil {
		return err
	}
	if err := copyRedactedMetadata(source.Path("metadata.json"), filepath.Join(outputDir, "metadata.json"), filepath.Base(outputDir), cfg); err != nil {
		return err
	}
	if err := copyRedactedTrace(source.Path("trace.ndjson"), filepath.Join(outputDir, "trace.ndjson"), filepath.Base(outputDir), artifactRefs, cfg); err != nil {
		return err
	}
	return nil
}

func (r *Redactor) Event(event trace.Event) trace.Event {
	if r == nil || !r.enabled {
		return event
	}
	event.RunID = r.String(event.RunID)
	event.ParentID = r.String(event.ParentID)
	event.SpanID = r.String(event.SpanID)
	event.Source.Adapter = r.String(event.Source.Adapter)
	event.Source.Agent = r.String(event.Source.Agent)
	event.Source.RawEventType = r.String(event.Source.RawEventType)
	event.Source.Version = r.String(event.Source.Version)
	event.Payload = redactPayload(event.Payload, r)
	for i := range event.Artifacts {
		event.Artifacts[i] = r.ArtifactRef(event.Artifacts[i])
	}
	if event.RawRef != nil {
		ref := r.ArtifactRef(*event.RawRef)
		event.RawRef = &ref
	}
	return event
}

func (r *Redactor) ArtifactRef(ref trace.ArtifactRef) trace.ArtifactRef {
	ref.Path = r.String(ref.Path)
	ref.MimeType = r.String(ref.MimeType)
	if ref.SHA256 != "" {
		ref.Redacted = true
	}
	return ref
}

func (r *Redactor) Bytes(input []byte) []byte {
	if input == nil {
		return nil
	}
	return []byte(r.String(string(input)))
}

func (r *Redactor) String(input string) string {
	if r == nil || !r.enabled || input == "" {
		return input
	}
	output := input
	for _, pattern := range r.patterns {
		output = pattern.regex.ReplaceAllString(output, replacement(pattern.name))
	}
	if r.redactEnvFiles {
		output = r.maskEnvPaths(output)
	}
	return output
}

func (r *Redactor) maskEnvPaths(input string) string {
	output := input
	for _, pattern := range envPathPatterns {
		output = pattern.ReplaceAllString(output, "[REDACTED:env_path]")
	}
	for _, path := range r.paths {
		if path == "" || path == ".env" || path == ".env.*" || path == "secrets/**" || path == "**/id_rsa" {
			continue
		}
		output = regexp.MustCompile(regexp.QuoteMeta(path)).ReplaceAllString(output, "[REDACTED:path]")
	}
	return output
}

func replacement(name string) string {
	if name == "" {
		return "[REDACTED]"
	}
	return "[REDACTED:" + name + "]"
}

func redactPayload(payload trace.Payload, redactor *Redactor) trace.Payload {
	if payload == nil {
		return nil
	}
	out := trace.Payload{}
	for key, value := range payload {
		if sensitivePayloadKey(key) {
			out[key] = replacement(key)
			continue
		}
		out[key] = redactValue(key, value, redactor)
	}
	return out
}

func redactValue(key string, value any, redactor *Redactor) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return redactor.String(typed)
	case []string:
		out := make([]string, len(typed))
		for i, item := range typed {
			out[i] = redactor.String(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactValue(key, item, redactor)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for childKey, childValue := range typed {
			if sensitivePayloadKey(childKey) {
				out[childKey] = replacement(childKey)
				continue
			}
			out[childKey] = redactValue(childKey, childValue, redactor)
		}
		return out
	case trace.Payload:
		return redactPayload(typed, redactor)
	case trace.ArtifactRef:
		return redactor.ArtifactRef(typed)
	case *trace.ArtifactRef:
		if typed == nil {
			return nil
		}
		ref := redactor.ArtifactRef(*typed)
		return &ref
	default:
		return typed
	}
}

func sensitivePayloadKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "apikey") ||
		lower == "key" ||
		strings.HasSuffix(lower, "_key")
}

func prepareRedactedOutput(runsDir, outputDir string) error {
	if info, err := os.Stat(outputDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("redacted output exists and is not a directory: %s", outputDir)
		}
		if !isDefaultRedactedOutput(runsDir, outputDir) {
			return fmt.Errorf("redacted output already exists: %s", outputDir)
		}
		if err := os.RemoveAll(outputDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(outputDir, 0o755)
}

func isDefaultRedactedOutput(runsDir, outputDir string) bool {
	rel, err := filepath.Rel(runsDir, outputDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false
	}
	return strings.HasSuffix(filepath.Base(outputDir), "-redacted")
}

func copyArtifactDirs(sourceDir, outputDir string, cfg config.RedactConfig) (map[string]trace.ArtifactRef, error) {
	refs := map[string]trace.ArtifactRef{}
	for _, item := range []struct {
		dir  string
		kind trace.ArtifactKind
	}{
		{dir: "blobs", kind: trace.ArtifactBlob},
		{dir: "patches", kind: trace.ArtifactPatch},
		{dir: "raw", kind: trace.ArtifactRaw},
	} {
		sourceRoot := filepath.Join(sourceDir, item.dir)
		if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry == nil {
				return err
			}
			rel, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}
			target := filepath.Join(outputDir, rel)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			data = ApplyToBytes(data, cfg)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return err
			}
			refs[filepath.ToSlash(rel)] = artifactRef(item.kind, rel, data)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func copyRedactedMetadata(sourcePath, targetPath, outputRunID string, cfg config.RedactConfig) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data = ApplyToBytes(data, cfg)
	var meta trace.Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	}
	meta.RunID = outputRunID
	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, append(encoded, '\n'), 0o644)
}

func copyRedactedTrace(sourcePath, targetPath, outputRunID string, artifactRefs map[string]trace.ArtifactRef, cfg config.RedactConfig) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	writer := bufio.NewWriter(out)
	defer writer.Flush()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event trace.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			_, writeErr := writer.Write(append(ApplyToBytes([]byte(line), cfg), '\n'))
			return writeErr
		}
		event = ApplyToEvent(event, cfg)
		event.RunID = outputRunID
		for i := range event.Artifacts {
			event.Artifacts[i] = refreshedArtifactRef(event.Artifacts[i], artifactRefs)
		}
		if event.RawRef != nil {
			ref := refreshedArtifactRef(*event.RawRef, artifactRefs)
			event.RawRef = &ref
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func refreshedArtifactRef(ref trace.ArtifactRef, refs map[string]trace.ArtifactRef) trace.ArtifactRef {
	if updated, ok := refs[filepath.ToSlash(ref.Path)]; ok {
		updated.MimeType = ref.MimeType
		if updated.MimeType == "" {
			updated.MimeType = ref.MimeType
		}
		updated.Redacted = true
		return updated
	}
	ref.Redacted = true
	return ref
}

func artifactRef(kind trace.ArtifactKind, rel string, data []byte) trace.ArtifactRef {
	sum := sha256.Sum256(data)
	return trace.ArtifactRef{
		Kind:      kind,
		Path:      filepath.ToSlash(rel),
		SHA256:    hex.EncodeToString(sum[:]),
		SizeBytes: int64(len(data)),
		Redacted:  true,
	}
}

func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
