package behavior

import (
	"path/filepath"
	"strings"
)

type DefaultCommandClassifier struct{}

func NewDefaultCommandClassifier() DefaultCommandClassifier {
	return DefaultCommandClassifier{}
}

func ClassifyCommand(command string) CommandClassification {
	return DefaultCommandClassifier{}.ClassifyCommand(command)
}

func (DefaultCommandClassifier) ClassifyCommand(command string) CommandClassification {
	normalized := normalizeCommand(command)
	attrs := map[string]string{
		"classifier": "default",
	}
	result := CommandClassification{
		Kind:       CommandUnknown,
		Command:    normalized,
		Confidence: 0.2,
		Attributes: attrs,
	}
	if normalized == "" {
		result.Confidence = 0
		return result
	}

	tokens := splitCommandFields(normalized)
	if len(tokens) == 0 {
		return result
	}
	base := commandExecutableBase(tokens[0])
	switch {
	case isGitGrep(tokens):
		result.Kind = CommandSearch
		result.Query, result.Files = searchQueryAndFiles(tokens[2:])
		attrs["tool"] = "git grep"
		result.Confidence = 0.95
	case isSearchCommand(base):
		result.Kind = CommandSearch
		result.Query, result.Files = searchQueryAndFiles(tokens[1:])
		attrs["tool"] = base
		result.Confidence = 0.95
	case isTestCommand(base, tokens):
		result.Kind = CommandRunTest
		attrs["tool"] = base
		result.Files = commandPathArgs(commandOperands(base, tokens))
		result.Confidence = 0.95
	case isBuildOrLintCommand(base, tokens):
		result.Kind = CommandRunBuild
		attrs["tool"] = base
		if commandLooksLikeLint(base, tokens) {
			attrs["category"] = "lint"
		} else {
			attrs["category"] = "build"
		}
		result.Files = commandPathArgs(commandOperands(base, tokens))
		result.Confidence = 0.9
	case isInstallDependencyCommand(base, tokens):
		result.Kind = CommandInstallDependency
		attrs["tool"] = base
		result.Confidence = 0.9
	case isReadFileCommand(base):
		result.Kind = CommandReadFile
		attrs["tool"] = base
		result.Files = readFileArgs(base, tokens[1:])
		result.Confidence = 0.85
	}
	result.Files = SortFiles(result.Files)
	return result
}

func normalizeCommand(command string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
}

func splitCommandFields(command string) []string {
	var fields []string
	var b strings.Builder
	var quote rune
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"' || r == '`':
			quote = r
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if b.Len() > 0 {
				fields = append(fields, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		fields = append(fields, b.String())
	}
	return fields
}

func commandExecutableBase(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, `"'`)
	token = strings.ReplaceAll(token, "\\", "/")
	base := filepath.Base(token)
	base = strings.ToLower(base)
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".cmd")
	base = strings.TrimSuffix(base, ".bat")
	if base == "gradlew" || base == "./gradlew" {
		return "gradle"
	}
	return base
}

func isGitGrep(tokens []string) bool {
	return len(tokens) >= 2 && commandExecutableBase(tokens[0]) == "git" && strings.ToLower(tokens[1]) == "grep"
}

func isSearchCommand(base string) bool {
	switch base {
	case "rg", "grep", "findstr", "fd", "find":
		return true
	default:
		return false
	}
}

func searchQueryAndFiles(args []string) (string, []string) {
	var query string
	var files []string
	expectQuery := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if expectQuery {
			query = arg
			expectQuery = false
			continue
		}
		if arg == "-e" || arg == "--regexp" || strings.HasPrefix(arg, "--regexp=") {
			if strings.Contains(arg, "=") {
				query = strings.TrimPrefix(arg, "--regexp=")
			} else {
				expectQuery = true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") || isSearchSlashFlag(arg) {
			continue
		}
		if query == "" {
			query = arg
			continue
		}
		files = append(files, arg)
	}
	return query, files
}

func isSearchSlashFlag(arg string) bool {
	if !strings.HasPrefix(arg, "/") || strings.Count(arg, "/") != 1 {
		return false
	}
	if len(arg) < 2 || len(arg) > 4 {
		return false
	}
	for _, r := range arg[1:] {
		if r < 'A' || (r > 'Z' && r < 'a') || r > 'z' {
			return false
		}
	}
	return true
}

func commandPathArgs(args []string) []string {
	var files []string
	for _, arg := range args {
		if arg == "--" || strings.HasPrefix(arg, "-") || strings.Contains(arg, "=") {
			continue
		}
		if looksLikeCommandOperand(arg) {
			files = append(files, arg)
		}
	}
	return files
}

func commandOperands(base string, tokens []string) []string {
	if len(tokens) <= 1 {
		return nil
	}
	lower := lowerTokens(tokens)
	switch base {
	case "go", "cargo":
		if len(tokens) > 2 {
			return tokens[2:]
		}
	case "npm", "pnpm", "yarn":
		if len(lower) >= 3 && lower[1] == "run" {
			return tokens[3:]
		}
		if len(tokens) > 2 {
			return tokens[2:]
		}
	case "python", "python3", "py":
		if len(lower) >= 3 && lower[1] == "-m" {
			return tokens[3:]
		}
	case "mvn", "mvnw", "gradle", "gradlew":
		if len(tokens) > 2 {
			return tokens[2:]
		}
	default:
		return tokens[1:]
	}
	return nil
}

func isTestCommand(base string, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	lower := lowerTokens(tokens)
	switch base {
	case "go":
		return len(lower) >= 2 && lower[1] == "test"
	case "npm", "pnpm", "yarn":
		return len(lower) >= 2 && (lower[1] == "test" || (len(lower) >= 3 && lower[1] == "run" && lower[2] == "test"))
	case "pytest":
		return true
	case "python", "python3", "py":
		return len(lower) >= 4 && lower[1] == "-m" && lower[2] == "pytest"
	case "cargo":
		return len(lower) >= 2 && lower[1] == "test"
	case "mvn", "mvnw":
		return containsToken(lower[1:], "test")
	case "gradle", "gradlew":
		return containsToken(lower[1:], "test")
	default:
		return false
	}
}

func isBuildOrLintCommand(base string, tokens []string) bool {
	lower := lowerTokens(tokens)
	switch base {
	case "go":
		return len(lower) >= 2 && (lower[1] == "build" || lower[1] == "vet")
	case "npm":
		return len(lower) >= 3 && lower[1] == "run" && (lower[2] == "build" || lower[2] == "lint")
	case "pnpm", "yarn":
		return len(lower) >= 2 && (lower[1] == "build" || lower[1] == "lint" || (len(lower) >= 3 && lower[1] == "run" && (lower[2] == "build" || lower[2] == "lint")))
	case "cargo":
		return len(lower) >= 2 && lower[1] == "build"
	case "mvn", "mvnw":
		return containsToken(lower[1:], "package") || containsToken(lower[1:], "compile")
	case "gradle", "gradlew":
		return containsToken(lower[1:], "build") || containsToken(lower[1:], "assemble")
	default:
		return false
	}
}

func commandLooksLikeLint(base string, tokens []string) bool {
	lower := lowerTokens(tokens)
	if base == "go" {
		return len(lower) >= 2 && lower[1] == "vet"
	}
	return containsToken(lower[1:], "lint")
}

func isInstallDependencyCommand(base string, tokens []string) bool {
	lower := lowerTokens(tokens)
	switch base {
	case "npm":
		return len(lower) >= 2 && (lower[1] == "install" || lower[1] == "i" || lower[1] == "add")
	case "pnpm", "yarn":
		return len(lower) >= 2 && (lower[1] == "add" || lower[1] == "install")
	case "go":
		return len(lower) >= 2 && lower[1] == "get"
	case "cargo":
		return len(lower) >= 2 && lower[1] == "add"
	case "pip", "pip3":
		return len(lower) >= 2 && lower[1] == "install"
	case "python", "python3", "py":
		return len(lower) >= 4 && lower[1] == "-m" && lower[2] == "pip" && lower[3] == "install"
	default:
		return false
	}
}

func isReadFileCommand(base string) bool {
	switch base {
	case "cat", "type", "sed", "head", "tail", "less", "more", "get-content", "gc":
		return true
	default:
		return false
	}
}

func readFileArgs(base string, args []string) []string {
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			if base == "sed" && arg == "-n" && i+1 < len(args) {
				i++
			}
			continue
		}
		if base == "sed" && looksLikeSedScript(arg) {
			continue
		}
		if looksLikeCommandOperand(arg) {
			files = append(files, arg)
		}
	}
	return files
}

func lowerTokens(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, token := range tokens {
		if i == 0 {
			out[i] = commandExecutableBase(token)
		} else {
			out[i] = strings.ToLower(token)
		}
	}
	return out
}

func containsToken(tokens []string, needle string) bool {
	for _, token := range tokens {
		if token == needle {
			return true
		}
	}
	return false
}

func looksLikeCommandOperand(arg string) bool {
	arg = strings.TrimSpace(arg)
	return arg != "" && arg != "|" && arg != "&&" && arg != "||" && arg != ">"
}

func looksLikeSedScript(arg string) bool {
	lower := strings.ToLower(strings.TrimSpace(arg))
	return strings.HasSuffix(lower, "p") || strings.HasSuffix(lower, "d") || strings.Contains(lower, "s/")
}
