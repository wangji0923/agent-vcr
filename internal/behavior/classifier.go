package behavior

type CommandKind string

const (
	CommandUnknown           CommandKind = "unknown"
	CommandSearch            CommandKind = "search"
	CommandReadFile          CommandKind = "read_file"
	CommandRunTest           CommandKind = "run_test"
	CommandRunBuild          CommandKind = "run_build"
	CommandInstallDependency CommandKind = "install_dependency"
)

type CommandClassification struct {
	Kind       CommandKind       `json:"kind"`
	Query      string            `json:"query,omitempty"`
	Files      []string          `json:"files,omitempty"`
	Command    string            `json:"command,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type CommandClassifier interface {
	ClassifyCommand(command string) CommandClassification
}

type PathKind string

const (
	PathUnknown PathKind = "unknown"
	PathSource  PathKind = "source"
	PathTest    PathKind = "test"
	PathConfig  PathKind = "config"
	PathDocs    PathKind = "docs"
	PathLegacy  PathKind = "legacy"
	PathSecret  PathKind = "secret"
)

type PathClassification struct {
	Kind       PathKind          `json:"kind"`
	Path       string            `json:"path,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type PathClassifier interface {
	ClassifyPath(path string) PathClassification
}
