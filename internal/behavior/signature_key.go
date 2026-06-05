package behavior

import (
	"strings"
)

const blobPathFingerprint = "<blob>"

func StepFingerprint(step Step) string {
	parts := []string{
		string(step.Kind),
		normalizeFingerprintText(step.Action),
		normalizeFingerprintPath(step.Target),
		normalizeFingerprintText(step.Query),
		normalizeFingerprintText(step.Command),
		normalizeFingerprintText(step.ToolName),
		strings.Join(normalizeFingerprintFiles(step.Files), ","),
		string(normalizedResult(step.Result)),
	}
	return strings.Join(parts, "|")
}

func normalizeFingerprintFiles(files []string) []string {
	normalized := make([]string, 0, len(files))
	for _, file := range files {
		if isBlobPath(file) {
			normalized = append(normalized, blobPathFingerprint)
			continue
		}
		normalized = append(normalized, NormalizePathForKey(file))
	}
	return SortFiles(normalized)
}

func normalizeFingerprintText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	fields := strings.Fields(value)
	for i, field := range fields {
		prefix, core, suffix := splitTokenPathBoundary(field)
		fields[i] = prefix + normalizeFingerprintPath(core) + suffix
	}
	return strings.Join(fields, " ")
}

func normalizeFingerprintPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if isBlobPath(value) {
		return blobPathFingerprint
	}
	normalized := NormalizePathForKey(value)
	if isBlobPath(normalized) {
		return blobPathFingerprint
	}
	return normalized
}

func isBlobPath(value string) bool {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	return strings.Contains(value, ".agent-vcr/runs/") && strings.Contains(value, "/blobs/")
}
