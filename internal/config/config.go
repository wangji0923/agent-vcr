package config

type Config struct {
	Version   string        `yaml:"version" json:"version"`
	Capture   CaptureConfig `yaml:"capture" json:"capture"`
	Storage   StorageConfig `yaml:"storage" json:"storage"`
	Redaction RedactConfig  `yaml:"redaction" json:"redaction"`
	Rules     RulesConfig   `yaml:"rules" json:"rules"`
	Report    ReportConfig  `yaml:"report" json:"report"`
}

type CaptureConfig struct {
	Prompt         string `yaml:"prompt" json:"prompt"`
	ToolInput      string `yaml:"tool_input" json:"tool_input"`
	ToolOutput     string `yaml:"tool_output" json:"tool_output"`
	GitDiff        bool   `yaml:"git_diff" json:"git_diff"`
	FinalDiff      bool   `yaml:"final_diff" json:"final_diff"`
	MaxInlineBytes int64  `yaml:"max_inline_bytes" json:"max_inline_bytes"`
}

type StorageConfig struct {
	Dir           string `yaml:"dir" json:"dir"`
	RetentionDays int    `yaml:"retention_days" json:"retention_days"`
	MaxBlobBytes  int64  `yaml:"max_blob_bytes" json:"max_blob_bytes"`
}

type RedactConfig struct {
	Enabled        bool            `yaml:"enabled" json:"enabled"`
	RedactEnvFiles bool            `yaml:"redact_env_files" json:"redact_env_files"`
	Patterns       []PatternConfig `yaml:"patterns" json:"patterns"`
	Paths          []string        `yaml:"paths" json:"paths"`
}

type PatternConfig struct {
	Name  string `yaml:"name" json:"name"`
	Regex string `yaml:"regex" json:"regex"`
}

type RulesConfig struct {
	MaxChangedFiles               int      `yaml:"max_changed_files" json:"max_changed_files"`
	ForbiddenPaths                []string `yaml:"forbidden_paths" json:"forbidden_paths"`
	RequiredCommands              []string `yaml:"required_commands" json:"required_commands"`
	RequireTestsAfterSourceChange bool     `yaml:"require_tests_after_source_change" json:"require_tests_after_source_change"`
	SourceGlobs                   []string `yaml:"source_globs" json:"source_globs"`
	TestGlobs                     []string `yaml:"test_globs" json:"test_globs"`
}

type ReportConfig struct {
	HTML     bool `yaml:"html" json:"html"`
	Markdown bool `yaml:"markdown" json:"markdown"`
}
