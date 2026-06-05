package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type EventSignature struct {
	Type       trace.EventType `json:"type"`
	ToolName   string          `json:"tool_name,omitempty"`
	Command    string          `json:"command,omitempty"`
	ArgsHash   string          `json:"args_hash,omitempty"`
	ResultHash string          `json:"result_hash,omitempty"`
	FilesHash  string          `json:"files_hash,omitempty"`
	ExitCode   string          `json:"exit_code,omitempty"`
}

func Signature(event trace.Event) EventSignature {
	sig := EventSignature{
		Type:     event.Type,
		ToolName: signaturePayloadString(event.Payload, "tool_name", "name"),
		Command:  CommandFromEvent(event),
		ExitCode: exitCodeString(event.Payload),
	}

	switch event.Type {
	case trace.EventUserPrompt:
		if value := signaturePayloadString(event.Payload, "prompt_sha256", "content_sha256", "message_sha256"); value != "" {
			sig.ArgsHash = value
		} else {
			sig.ArgsHash = hashAny(normalizePayloadForSignature(event.Payload))
		}
	case trace.EventToolCall, trace.EventShellCommand, trace.EventProcessStart:
		sig.ArgsHash = hashAny(normalizePayloadForSignature(event.Payload))
	case trace.EventToolResult, trace.EventToolError, trace.EventShellResult, trace.EventModelCall, trace.EventModelResult:
		sig.ResultHash = hashAny(normalizePayloadForSignature(event.Payload))
	case trace.EventProcessResult:
		sig.FilesHash = hashStrings(NormalizedChangedFiles(event.Payload))
		sig.ResultHash = hashAny(map[string]any{
			"command":             sig.Command,
			"exit_code":           sig.ExitCode,
			"process_start_error": signaturePayloadString(event.Payload, "process_start_error"),
		})
	case trace.EventRunStart:
		sig.ArgsHash = hashAny(map[string]any{
			"command": sig.Command,
			"args":    normalizeValueForSignature(event.Payload["args"], "args"),
			"status":  signaturePayloadString(event.Payload, "status"),
		})
	case trace.EventRunStop:
		sig.ResultHash = hashAny(map[string]any{
			"status":    signaturePayloadString(event.Payload, "status"),
			"exit_code": sig.ExitCode,
		})
	case trace.EventRaw:
		if event.RawRef != nil && event.RawRef.SHA256 != "" {
			sig.ResultHash = event.RawRef.SHA256
		} else {
			sig.ResultHash = hashAny(normalizePayloadForSignature(event.Payload))
		}
	default:
		sig.ArgsHash = hashAny(normalizePayloadForSignature(event.Payload))
	}

	return sig
}

func CommandFromEvent(event trace.Event) string {
	command := signaturePayloadString(event.Payload, "command", "cmd")
	if command == "" {
		if input, ok := event.Payload["input"].(map[string]any); ok {
			command = firstMapString(input, "command", "cmd")
		}
	}
	if command == "" {
		return ""
	}
	args := stringSlice(event.Payload["args"])
	if len(args) == 0 {
		return normalizeCommand(command)
	}
	return normalizeCommand(strings.TrimSpace(command + " " + strings.Join(args, " ")))
}

func NormalizedChangedFiles(payload trace.Payload) []string {
	files := stringSlice(payload["changed_files"])
	if len(files) == 0 {
		files = append(files, signaturePayloadString(payload, "changed_file", "path", "file", "filename"))
	}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(files))
	for _, file := range files {
		file = normalizeComparablePath(file)
		if file == "" || seen[file] {
			continue
		}
		seen[file] = true
		normalized = append(normalized, file)
	}
	sort.Strings(normalized)
	return normalized
}

func normalizePayloadForSignature(payload trace.Payload) any {
	return normalizeValueForSignature(map[string]any(payload), "")
}

func normalizeValueForSignature(value any, key string) any {
	key = strings.ToLower(key)
	if shouldIgnoreSignatureKey(key) {
		return nil
	}
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if isArtifactPathKey(key) {
			if strings.TrimSpace(v) == "" {
				return ""
			}
			return "<artifact>"
		}
		if looksLikePathKey(key) || looksLikePath(v) {
			return normalizeComparablePath(v)
		}
		if key == "command" || key == "cmd" {
			return normalizeCommand(v)
		}
		return strings.TrimSpace(v)
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeValueForSignature(item, key))
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeValueForSignature(item, key))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			normalized := normalizeValueForSignature(item, k)
			if normalized == nil && shouldIgnoreSignatureKey(strings.ToLower(k)) {
				continue
			}
			out[k] = normalized
		}
		return out
	case trace.Payload:
		return normalizeValueForSignature(map[string]any(v), key)
	case trace.ArtifactRef:
		return normalizeArtifactRef(v)
	case *trace.ArtifactRef:
		if v == nil {
			return nil
		}
		return normalizeArtifactRef(*v)
	default:
		return v
	}
}

func normalizeArtifactRef(ref trace.ArtifactRef) map[string]any {
	return map[string]any{
		"kind":       ref.Kind,
		"sha256":     ref.SHA256,
		"size_bytes": ref.SizeBytes,
		"mime_type":  ref.MimeType,
		"redacted":   ref.Redacted,
	}
}

func shouldIgnoreSignatureKey(key string) bool {
	if key == "" {
		return false
	}
	switch key {
	case "event_id", "event_index", "run_id", "parent_id", "span_id",
		"timestamp", "received_at", "started_at", "ended_at",
		"duration", "duration_ms", "elapsed_ms",
		"process_started_at_unix", "process_ended_at_unix",
		"tool_use_id", "turn_id", "call_id":
		return true
	default:
		return strings.Contains(key, "duration") ||
			strings.HasSuffix(key, "_timestamp") ||
			strings.HasSuffix(key, "_started_at") ||
			strings.HasSuffix(key, "_ended_at")
	}
}

func isArtifactPathKey(key string) bool {
	return strings.Contains(key, "blob") ||
		strings.Contains(key, "artifact_path") ||
		strings.HasSuffix(key, "_ref") ||
		key == "raw_ref"
}

func looksLikePathKey(key string) bool {
	return key == "path" || key == "file" || key == "filename" ||
		key == "cwd" || strings.HasSuffix(key, "_path") || strings.HasSuffix(key, "_file")
}

func looksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\n") {
		return false
	}
	return strings.Contains(value, `:\`) ||
		strings.HasPrefix(value, "/Users/") ||
		strings.HasPrefix(value, "/home/") ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../")
}

var (
	windowsUserPath = regexp.MustCompile(`(?i)^([a-z]:)[/\\]users[/\\][^/\\]+[/\\]?`)
	macUserPath     = regexp.MustCompile(`^/Users/[^/]+/?`)
	linuxUserPath   = regexp.MustCompile(`^/home/[^/]+/?`)
)

func normalizeComparablePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(value)
	value = windowsUserPath.ReplaceAllString(value, "${1}/Users/<user>/")
	value = macUserPath.ReplaceAllString(value, "/Users/<user>/")
	value = linuxUserPath.ReplaceAllString(value, "/home/<user>/")
	value = strings.TrimPrefix(value, "./")
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func normalizeCommand(command string) string {
	return strings.TrimSpace(command)
}

func signaturePayloadString(payload trace.Payload, keys ...string) string {
	if payload == nil {
		return ""
	}
	for _, key := range keys {
		if value := stringValue(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstMapString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

func exitCodeString(payload trace.Payload) string {
	return signaturePayloadString(payload, "exit_code", "exitCode", "code")
}

func exitCodeInt(payload trace.Payload) (int, bool) {
	value := exitCodeString(payload)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str := stringValue(item); str != "" {
				out = append(out, str)
			}
		}
		return out
	default:
		if str := stringValue(value); str != "" {
			return []string{str}
		}
		return nil
	}
}

func hashAny(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte(fmt.Sprintf("%#v", value))
	}
	return hashBytes(data)
}

func hashStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return hashAny(values)
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
