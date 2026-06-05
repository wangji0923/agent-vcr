package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/version"
)

const (
	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusError   = "error"
)

type Options struct {
	ProjectDir string
	ConfigPath string
	EnvPath    string
}

type Result struct {
	Version      version.Info       `json:"version"`
	Cwd          string             `json:"cwd"`
	Core         CoreResult         `json:"core"`
	Codex        CodexResult        `json:"codex"`
	Architecture ArchitectureResult `json:"architecture"`
	Checks       []Check            `json:"checks"`
}

type CoreResult struct {
	GoBinary           string `json:"go_binary,omitempty"`
	GoVersion          string `json:"go_version,omitempty"`
	GitBinary          string `json:"git_binary,omitempty"`
	GitRepo            bool   `json:"git_repo"`
	ConfigPath         string `json:"config_path,omitempty"`
	ConfigExists       bool   `json:"config_exists"`
	AgentVCRGitignored bool   `json:"agent_vcr_gitignored"`
	RunsDir            string `json:"runs_dir,omitempty"`
	RunsDirWritable    bool   `json:"runs_dir_writable"`
	AgentsFileExists   bool   `json:"agents_file_exists"`
}

type CodexResult struct {
	CodexBinary           string `json:"codex_binary,omitempty"`
	HooksPath             string `json:"hooks_path,omitempty"`
	HooksExists           bool   `json:"hooks_exists"`
	AgentVCRBinary        string `json:"agent_vcr_binary,omitempty"`
	AgentVCRHookOnPath    bool   `json:"agent_vcr_hook_on_path"`
	AgentVCRHookInstalled bool   `json:"agent_vcr_hook_installed"`
}

type ArchitectureResult struct {
	AnalysisImportsAdapters bool     `json:"analysis_imports_adapters"`
	ReportImportsAdapters   bool     `json:"report_imports_adapters"`
	CheckImportsAdapters    bool     `json:"check_imports_adapters"`
	TraceImportsAdapters    bool     `json:"trace_imports_adapters"`
	Violations              []string `json:"violations,omitempty"`
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
}

func Run(opts Options) (Result, error) {
	projectDir, err := resolveProjectDir(opts.ProjectDir)
	if err != nil {
		return Result{}, err
	}
	envPath := opts.EnvPath
	if envPath == "" {
		envPath = os.Getenv("PATH")
	}

	result := Result{
		Version: version.Get(),
		Cwd:     projectDir,
	}
	result.Core.ConfigPath = configPath(projectDir, opts.ConfigPath)
	result.Core.RunsDir = filepath.Join(projectDir, ".agent-vcr", "runs")
	result.Codex.HooksPath = filepath.Join(projectDir, ".codex", "hooks.json")

	goPath := lookPath("go", envPath)
	result.Core.GoBinary = goPath
	if goPath == "" {
		result.add("go binary", StatusWarning, "go was not found on PATH", "")
	} else {
		result.Core.GoVersion = firstLine(commandOutput(goPath, "version"))
		result.add("go binary", StatusOK, result.Core.GoVersion, goPath)
	}

	gitPath := lookPath("git", envPath)
	result.Core.GitBinary = gitPath
	if gitPath == "" {
		result.add("git binary", StatusError, "git was not found on PATH", "")
	} else {
		result.add("git binary", StatusOK, "", gitPath)
		result.Core.GitRepo = isGitRepo(gitPath, projectDir)
		if result.Core.GitRepo {
			result.add("git repo", StatusOK, "inside work tree", projectDir)
		} else {
			result.add("git repo", StatusWarning, "not inside a git work tree", projectDir)
		}
	}

	if _, err := os.Stat(result.Core.ConfigPath); err == nil {
		result.Core.ConfigExists = true
		result.add("config", StatusOK, "", result.Core.ConfigPath)
	} else if os.IsNotExist(err) {
		result.add("config", StatusWarning, ".agent-vcr/config.yml was not found", result.Core.ConfigPath)
	} else {
		result.add("config", StatusError, err.Error(), result.Core.ConfigPath)
	}

	result.Core.AgentVCRGitignored = agentVCRGitignored(filepath.Join(projectDir, ".gitignore"))
	if result.Core.AgentVCRGitignored {
		result.add(".agent-vcr gitignored", StatusOK, "", filepath.Join(projectDir, ".gitignore"))
	} else {
		result.add(".agent-vcr gitignored", StatusError, ".agent-vcr/ is not ignored by git", filepath.Join(projectDir, ".gitignore"))
	}

	result.Core.RunsDirWritable = runsDirWritable(result.Core.RunsDir)
	if result.Core.RunsDirWritable {
		result.add("trace store writable", StatusOK, "", result.Core.RunsDir)
	} else {
		result.add("trace store writable", StatusError, "could not write to runs directory", result.Core.RunsDir)
	}

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err == nil {
		result.Core.AgentsFileExists = true
		result.add("AGENTS.md", StatusOK, "", agentsPath)
	} else {
		result.add("AGENTS.md", StatusWarning, "AGENTS.md was not found", agentsPath)
	}

	codexPath := lookPath("codex", envPath)
	result.Codex.CodexBinary = codexPath
	if codexPath == "" {
		result.add("codex binary", StatusWarning, "codex was not found on PATH", "")
	} else {
		result.add("codex binary", StatusOK, "", codexPath)
	}

	agentVCRPath := lookPath("agent-vcr", envPath)
	result.Codex.AgentVCRBinary = agentVCRPath
	result.Codex.AgentVCRHookOnPath = agentVCRPath != ""
	if agentVCRPath == "" {
		result.add("agent-vcr hook executable", StatusWarning, "agent-vcr was not found on PATH", "")
	} else {
		result.add("agent-vcr hook executable", StatusOK, "", agentVCRPath)
	}

	hooksData, hooksErr := os.ReadFile(result.Codex.HooksPath)
	if hooksErr == nil {
		result.Codex.HooksExists = true
		result.add("codex hooks.json", StatusOK, "", result.Codex.HooksPath)
		result.Codex.AgentVCRHookInstalled = bytes.Contains(hooksData, []byte("agent-vcr")) && bytes.Contains(hooksData, []byte("hook"))
		if result.Codex.AgentVCRHookInstalled {
			result.add("codex agent-vcr hook", StatusOK, "", result.Codex.HooksPath)
		} else {
			result.add("codex agent-vcr hook", StatusWarning, "hooks.json does not reference agent-vcr hook", result.Codex.HooksPath)
		}
	} else if os.IsNotExist(hooksErr) {
		result.add("codex hooks.json", StatusWarning, ".codex/hooks.json was not found", result.Codex.HooksPath)
	} else {
		result.add("codex hooks.json", StatusError, hooksErr.Error(), result.Codex.HooksPath)
	}

	architecture := ScanArchitecture(projectDir)
	result.Architecture = architecture
	if len(architecture.Violations) == 0 {
		result.add("architecture imports", StatusOK, "core packages do not import adapters", "")
	} else {
		for _, violation := range architecture.Violations {
			result.add("architecture imports", StatusError, violation, "")
		}
	}

	return result, nil
}

func RenderHuman(result Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Agent VCR Doctor\n\n")
	fmt.Fprintf(&b, "Core:\n")
	fmt.Fprintf(&b, "  version: %s\n", version.String())
	fmt.Fprintf(&b, "  cwd: %s\n", result.Cwd)
	fmt.Fprintf(&b, "  go: %s\n", printablePath(result.Core.GoBinary))
	fmt.Fprintf(&b, "  git: %s\n", printablePath(result.Core.GitBinary))
	fmt.Fprintf(&b, "  git repo: %s\n", yesNo(result.Core.GitRepo))
	fmt.Fprintf(&b, "  config: %s\n", printablePath(result.Core.ConfigPath))
	fmt.Fprintf(&b, "  .agent-vcr gitignored: %s\n", yesNo(result.Core.AgentVCRGitignored))
	fmt.Fprintf(&b, "  runs dir writable: %s\n", yesNo(result.Core.RunsDirWritable))
	fmt.Fprintf(&b, "\nCodex:\n")
	fmt.Fprintf(&b, "  codex binary: %s\n", printablePath(result.Codex.CodexBinary))
	fmt.Fprintf(&b, "  hooks.json: %s\n", printablePath(result.Codex.HooksPath))
	fmt.Fprintf(&b, "  agent-vcr hook executable: %s\n", yesNo(result.Codex.AgentVCRHookOnPath))
	fmt.Fprintf(&b, "  agent-vcr hook installed: %s\n", yesNo(result.Codex.AgentVCRHookInstalled))
	fmt.Fprintf(&b, "\nArchitecture:\n")
	fmt.Fprintf(&b, "  analysis imports adapters: %s\n", yesNo(result.Architecture.AnalysisImportsAdapters))
	fmt.Fprintf(&b, "  report imports adapters: %s\n", yesNo(result.Architecture.ReportImportsAdapters))
	fmt.Fprintf(&b, "  check imports adapters: %s\n", yesNo(result.Architecture.CheckImportsAdapters))
	fmt.Fprintf(&b, "  trace imports adapters: %s\n", yesNo(result.Architecture.TraceImportsAdapters))
	fmt.Fprintf(&b, "\nChecks:\n")
	for _, check := range result.Checks {
		fmt.Fprintf(&b, "  [%s] %s", check.Status, check.Name)
		if check.Message != "" {
			fmt.Fprintf(&b, ": %s", check.Message)
		}
		if check.Path != "" {
			fmt.Fprintf(&b, " (%s)", check.Path)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func RenderJSON(result Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

func ScanArchitecture(projectDir string) ArchitectureResult {
	var result ArchitectureResult
	checks := []struct {
		name string
		dir  string
		set  func(bool)
	}{
		{name: "analysis", dir: filepath.Join(projectDir, "internal", "analysis"), set: func(v bool) { result.AnalysisImportsAdapters = v }},
		{name: "report", dir: filepath.Join(projectDir, "internal", "report"), set: func(v bool) { result.ReportImportsAdapters = v }},
		{name: "check", dir: filepath.Join(projectDir, "internal", "check"), set: func(v bool) { result.CheckImportsAdapters = v }},
		{name: "trace", dir: filepath.Join(projectDir, "internal", "trace"), set: func(v bool) { result.TraceImportsAdapters = v }},
	}
	for _, check := range checks {
		violations := scanImportViolations(check.dir)
		if len(violations) > 0 {
			check.set(true)
			for _, violation := range violations {
				result.Violations = append(result.Violations, check.name+": "+violation)
			}
		}
	}
	sort.Strings(result.Violations)
	return result
}

func scanImportViolations(dir string) []string {
	var violations []string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s parse error: %v", path, err))
			return nil
		}
		for _, imp := range file.Imports {
			value := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(value, "/internal/adapters") {
				violations = append(violations, fmt.Sprintf("%s imports %s", path, value))
			}
		}
		return nil
	})
	return violations
}

func (r *Result) add(name, status, message, path string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Message: message, Path: path})
}

func resolveProjectDir(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		value = cwd
	}
	return filepath.Abs(value)
}

func configPath(projectDir, explicit string) string {
	if strings.TrimSpace(explicit) == "" {
		return filepath.Join(projectDir, ".agent-vcr", "config.yml")
	}
	if filepath.IsAbs(explicit) {
		return explicit
	}
	return filepath.Join(projectDir, explicit)
}

func lookPath(name, envPath string) string {
	extensions := []string{""}
	if runtime.GOOS == "windows" {
		extensions = strings.Split(strings.ToLower(os.Getenv("PATHEXT")), ";")
		if len(extensions) == 0 || extensions[0] == "" {
			extensions = []string{".exe", ".bat", ".cmd", ""}
		} else {
			extensions = append(extensions, "")
		}
	}
	for _, dir := range filepath.SplitList(envPath) {
		if dir == "" {
			continue
		}
		for _, ext := range extensions {
			candidate := filepath.Join(dir, name)
			if ext != "" && !strings.HasSuffix(strings.ToLower(candidate), ext) {
				candidate += ext
			}
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func commandOutput(path string, args ...string) string {
	cmd := exec.Command(path, args...)
	if runtime.GOOS == "windows" && (strings.HasSuffix(strings.ToLower(path), ".bat") || strings.HasSuffix(strings.ToLower(path), ".cmd")) {
		all := append([]string{"/c", path}, args...)
		cmd = exec.Command("cmd", all...)
	}
	data, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func isGitRepo(gitPath, projectDir string) bool {
	cmd := exec.Command(gitPath, "-C", projectDir, "rev-parse", "--is-inside-work-tree")
	if runtime.GOOS == "windows" && (strings.HasSuffix(strings.ToLower(gitPath), ".bat") || strings.HasSuffix(strings.ToLower(gitPath), ".cmd")) {
		cmd = exec.Command("cmd", "/c", gitPath, "-C", projectDir, "rev-parse", "--is-inside-work-tree")
	}
	data, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(data)) == "true"
}

func agentVCRGitignored(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == ".agent-vcr/" || line == ".agent-vcr" || line == "/.agent-vcr/" || line == "/.agent-vcr" {
			return true
		}
	}
	return false
}

func runsDirWritable(path string) bool {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return false
	}
	file, err := os.CreateTemp(path, ".doctor-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func printablePath(value string) string {
	if value == "" {
		return "not found"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
