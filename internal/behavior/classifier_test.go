package behavior

import (
	"reflect"
	"testing"
)

func TestClassifyCommandSearch(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantQuery string
		wantFiles []string
		wantTool  string
	}{
		{
			name:      "rg",
			command:   `rg "BehaviorSignature" internal docs`,
			wantQuery: "BehaviorSignature",
			wantFiles: []string{"docs", "internal"},
			wantTool:  "rg",
		},
		{
			name:      "grep",
			command:   `grep -R "TODO" src tests`,
			wantQuery: "TODO",
			wantFiles: []string{"src", "tests"},
			wantTool:  "grep",
		},
		{
			name:      "findstr",
			command:   `findstr /S /I "TODO" src\*.go`,
			wantQuery: "TODO",
			wantFiles: []string{"src/*.go"},
			wantTool:  "findstr",
		},
		{
			name:      "git grep",
			command:   `git grep "trace.Event" -- internal docs`,
			wantQuery: "trace.Event",
			wantFiles: []string{"docs", "internal"},
			wantTool:  "git grep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyCommand(tt.command)
			if got.Kind != CommandSearch {
				t.Fatalf("Kind = %q, want %q", got.Kind, CommandSearch)
			}
			if got.Query != tt.wantQuery {
				t.Fatalf("Query = %q, want %q", got.Query, tt.wantQuery)
			}
			if !reflect.DeepEqual(got.Files, tt.wantFiles) {
				t.Fatalf("Files = %#v, want %#v", got.Files, tt.wantFiles)
			}
			if got.Attributes["tool"] != tt.wantTool {
				t.Fatalf("tool attr = %q, want %q", got.Attributes["tool"], tt.wantTool)
			}
			if got.Confidence <= 0 {
				t.Fatalf("Confidence should be positive")
			}
		})
	}
}

func TestClassifyCommandRunTest(t *testing.T) {
	commands := []string{
		"go test ./...",
		"npm test",
		"npm run test -- --watch=false",
		"pnpm test",
		"yarn test",
		"pytest tests",
		"python -m pytest tests",
		"cargo test",
		"mvn test",
		"gradle test",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			got := ClassifyCommand(command)
			if got.Kind != CommandRunTest {
				t.Fatalf("Kind = %q, want %q for %q", got.Kind, CommandRunTest, command)
			}
			if got.Command == "" {
				t.Fatalf("normalized command should be retained")
			}
		})
	}
}

func TestClassifyCommandBuildAndLint(t *testing.T) {
	tests := []struct {
		command      string
		wantCategory string
	}{
		{command: "go build ./cmd/agent-vcr", wantCategory: "build"},
		{command: "go vet ./...", wantCategory: "lint"},
		{command: "npm run build", wantCategory: "build"},
		{command: "npm run lint", wantCategory: "lint"},
		{command: "pnpm build", wantCategory: "build"},
		{command: "yarn lint", wantCategory: "lint"},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := ClassifyCommand(tt.command)
			if got.Kind != CommandRunBuild {
				t.Fatalf("Kind = %q, want %q", got.Kind, CommandRunBuild)
			}
			if got.Attributes["category"] != tt.wantCategory {
				t.Fatalf("category = %q, want %q", got.Attributes["category"], tt.wantCategory)
			}
		})
	}
}

func TestClassifyCommandInstallDependency(t *testing.T) {
	commands := []string{
		"npm install",
		"npm i cobra",
		"pnpm add vite",
		"yarn add react",
		"go get golang.org/x/tools",
		"cargo add serde",
		"pip install pytest",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			got := ClassifyCommand(command)
			if got.Kind != CommandInstallDependency {
				t.Fatalf("Kind = %q, want %q", got.Kind, CommandInstallDependency)
			}
		})
	}
}

func TestClassifyCommandReadFile(t *testing.T) {
	tests := []struct {
		command   string
		wantFiles []string
	}{
		{command: "cat internal/behavior/step.go", wantFiles: []string{"internal/behavior/step.go"}},
		{command: "sed -n '1,40p' internal/behavior/classifier.go", wantFiles: []string{"internal/behavior/classifier.go"}},
		{command: "head -20 README.md", wantFiles: []string{"README.md"}},
		{command: "Get-Content docs\\trace-schema.md", wantFiles: []string{"docs/trace-schema.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := ClassifyCommand(tt.command)
			if got.Kind != CommandReadFile {
				t.Fatalf("Kind = %q, want %q", got.Kind, CommandReadFile)
			}
			if !reflect.DeepEqual(got.Files, tt.wantFiles) {
				t.Fatalf("Files = %#v, want %#v", got.Files, tt.wantFiles)
			}
		})
	}
}

func TestClassifyCommandUnknown(t *testing.T) {
	got := ClassifyCommand("echo hello")
	if got.Kind != CommandUnknown {
		t.Fatalf("Kind = %q, want %q", got.Kind, CommandUnknown)
	}
	if got.Confidence <= 0 {
		t.Fatalf("unknown command should still have a low confidence classification")
	}
}

func TestDefaultClassifiersSatisfyInterfaces(t *testing.T) {
	var commandClassifier CommandClassifier = NewDefaultCommandClassifier()
	var pathClassifier PathClassifier = NewDefaultPathClassifier()
	if commandClassifier.ClassifyCommand("go test").Kind != CommandRunTest {
		t.Fatalf("default command classifier should classify go test")
	}
	if pathClassifier.ClassifyPath("internal/behavior/classifier_test.go").Kind != PathTest {
		t.Fatalf("default path classifier should classify test path")
	}
}

func TestNormalizePathWindowsAndUnix(t *testing.T) {
	tests := map[string]string{
		`C:\Users\alice\repo\internal\behavior\classifier_test.go`: "Users/alice/repo/internal/behavior/classifier_test.go",
		`.\internal\behavior\classifier.go`:                        "internal/behavior/classifier.go",
		"/home/alice/repo/src/main.go":                             "home/alice/repo/src/main.go",
		`"docs\behavior-diff.md"`:                                  "docs/behavior-diff.md",
	}
	for input, want := range tests {
		if got := NormalizePath(input); got != want {
			t.Fatalf("NormalizePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestClassifyPathKinds(t *testing.T) {
	tests := []struct {
		path string
		want PathKind
		attr string
	}{
		{path: "internal/behavior/classifier_test.go", want: PathTest, attr: "is_test"},
		{path: "tests/behavior/extractor.json", want: PathTest, attr: "is_test"},
		{path: "src/auth/session.ts", want: PathSource, attr: "is_source"},
		{path: "docs/behavior-diff.md", want: PathDocs, attr: "is_docs"},
		{path: "README.md", want: PathDocs, attr: "is_docs"},
		{path: "go.mod", want: PathConfig, attr: "is_config"},
		{path: ".github/workflows/ci.yml", want: PathConfig, attr: "is_config"},
		{path: "internal/legacy/session.go", want: PathLegacy, attr: "is_legacy"},
		{path: "src/deprecated-auth.ts", want: PathLegacy, attr: "is_legacy"},
		{path: ".env.local", want: PathSecret, attr: "is_secret"},
		{path: "secrets/prod.pem", want: PathSecret, attr: "is_secret"},
		{path: `C:\repo\tests\session.spec.ts`, want: PathTest, attr: "is_test"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ClassifyPath(tt.path)
			if got.Kind != tt.want {
				t.Fatalf("Kind = %q, want %q for %q", got.Kind, tt.want, tt.path)
			}
			if got.Attributes[tt.attr] != "true" {
				t.Fatalf("expected attr %q=true, got %#v", tt.attr, got.Attributes)
			}
			if got.Path == "" {
				t.Fatalf("normalized path should be retained")
			}
		})
	}
}

func TestPathHelpers(t *testing.T) {
	if !IsTestPath(`C:\repo\src\session.test.ts`) {
		t.Fatalf("expected Windows test path")
	}
	if !IsLegacyPath("src/legacy/session.go") {
		t.Fatalf("expected legacy path")
	}
	if !IsSecretPath("config/.env.production") {
		t.Fatalf("expected secret env path")
	}
	if ClassifyPathKind("cmd/agent-vcr/main.go") != PathSource {
		t.Fatalf("expected source path")
	}
}
