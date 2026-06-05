package behavior

import (
	"path"
	"strings"
)

type DefaultPathClassifier struct{}

func NewDefaultPathClassifier() DefaultPathClassifier {
	return DefaultPathClassifier{}
}

func ClassifyPath(pathValue string) PathClassification {
	return DefaultPathClassifier{}.ClassifyPath(pathValue)
}

func ClassifyPathKind(pathValue string) PathKind {
	return ClassifyPath(pathValue).Kind
}

func (DefaultPathClassifier) ClassifyPath(pathValue string) PathClassification {
	normalized := NormalizePath(pathValue)
	attrs := pathAttributes(normalized)
	result := PathClassification{
		Kind:       PathUnknown,
		Path:       normalized,
		Confidence: 0,
		Attributes: attrs,
	}
	if normalized == "" {
		return result
	}

	switch {
	case attrs["is_secret"] == "true":
		result.Kind = PathSecret
		result.Confidence = 1
	case attrs["is_legacy"] == "true":
		result.Kind = PathLegacy
		result.Confidence = 0.95
	case attrs["is_test"] == "true":
		result.Kind = PathTest
		result.Confidence = 0.95
	case attrs["is_docs"] == "true":
		result.Kind = PathDocs
		result.Confidence = 0.9
	case attrs["is_config"] == "true":
		result.Kind = PathConfig
		result.Confidence = 0.9
	case attrs["is_source"] == "true":
		result.Kind = PathSource
		result.Confidence = 0.75
	default:
		result.Confidence = 0.2
	}
	return result
}

func NormalizePath(pathValue string) string {
	pathValue = strings.TrimSpace(pathValue)
	pathValue = strings.Trim(pathValue, `"'`)
	if pathValue == "" {
		return ""
	}
	pathValue = strings.ReplaceAll(pathValue, "\\", "/")
	pathValue = stripWindowsDrive(pathValue)
	pathValue = strings.TrimPrefix(pathValue, "./")
	for strings.Contains(pathValue, "//") {
		pathValue = strings.ReplaceAll(pathValue, "//", "/")
	}
	pathValue = strings.Trim(pathValue, "/")
	if pathValue == "." {
		return ""
	}
	return path.Clean(pathValue)
}

func IsTestPath(pathValue string) bool {
	return pathAttributes(NormalizePath(pathValue))["is_test"] == "true"
}

func IsLegacyPath(pathValue string) bool {
	return pathAttributes(NormalizePath(pathValue))["is_legacy"] == "true"
}

func IsSecretPath(pathValue string) bool {
	return pathAttributes(NormalizePath(pathValue))["is_secret"] == "true"
}

func pathAttributes(normalized string) map[string]string {
	attrs := map[string]string{
		"classifier": "default",
	}
	if normalized == "" {
		return attrs
	}
	lower := strings.ToLower(normalized)
	components := strings.Split(lower, "/")
	base := components[len(components)-1]

	if isSecretEnvPath(lower, components, base) {
		attrs["is_secret"] = "true"
	}
	if isLegacyOrDeprecatedPath(lower, components, base) {
		attrs["is_legacy"] = "true"
	}
	if isTestPathNormalized(lower, components, base) {
		attrs["is_test"] = "true"
	}
	if isDocsPathNormalized(lower, components, base) {
		attrs["is_docs"] = "true"
	}
	if isConfigPathNormalized(lower, components, base) {
		attrs["is_config"] = "true"
	}
	if isSourcePathNormalized(base) {
		attrs["is_source"] = "true"
	}
	return attrs
}

func isSecretEnvPath(lower string, components []string, base string) bool {
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	if base == "id_rsa" || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") || strings.HasSuffix(base, ".p12") {
		return true
	}
	for _, component := range components {
		if component == "secret" || component == "secrets" || component == ".secrets" {
			return true
		}
	}
	return strings.Contains(lower, "/.env/")
}

func isLegacyOrDeprecatedPath(lower string, components []string, base string) bool {
	for _, component := range components {
		if component == "legacy" || component == "deprecated" {
			return true
		}
	}
	return strings.Contains(base, "legacy") || strings.Contains(base, "deprecated") || strings.Contains(lower, "/legacy-") || strings.Contains(lower, "/deprecated-")
}

func isTestPathNormalized(lower string, components []string, base string) bool {
	for _, component := range components {
		if component == "test" || component == "tests" || component == "__tests__" {
			return true
		}
	}
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.Contains(lower, "/spec/")
}

func isDocsPathNormalized(_ string, components []string, base string) bool {
	for _, component := range components {
		if component == "docs" || component == "doc" || component == "documentation" {
			return true
		}
	}
	switch base {
	case "readme", "readme.md", "readme.mdx", "changelog.md", "contributing.md", "license", "license.md":
		return true
	default:
		return strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".mdx") || strings.HasSuffix(base, ".rst")
	}
}

func isConfigPathNormalized(lower string, components []string, base string) bool {
	for _, component := range components {
		if component == ".github" || component == ".config" || component == "config" || component == "configs" {
			return true
		}
	}
	switch base {
	case "package.json", "go.mod", "go.sum", "cargo.toml", "cargo.lock", "pyproject.toml",
		"tsconfig.json", "makefile", "dockerfile", ".gitignore", ".gitattributes",
		"composer.json", "requirements.txt", "pom.xml", "build.gradle", "settings.gradle":
		return true
	default:
		return strings.Contains(base, ".config.") ||
			strings.HasSuffix(base, ".config.js") ||
			strings.HasSuffix(base, ".config.ts") ||
			strings.HasSuffix(base, ".yml") ||
			strings.HasSuffix(base, ".yaml") ||
			(strings.HasSuffix(base, ".json") && strings.Contains(lower, "/.github/"))
	}
}

func isSourcePathNormalized(base string) bool {
	extensions := []string{
		".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".java", ".c", ".cc", ".cpp",
		".h", ".hpp", ".rs", ".rb", ".php", ".cs", ".kt", ".swift", ".scala", ".sh",
	}
	for _, ext := range extensions {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	return false
}
