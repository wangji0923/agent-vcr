package config

func Default() Config {
	return Config{
		Version: "0.2",
		Capture: CaptureConfig{
			Prompt:         "redacted",
			ToolInput:      "redacted",
			ToolOutput:     "blob",
			GitDiff:        true,
			FinalDiff:      true,
			MaxInlineBytes: 4096,
		},
		Storage: StorageConfig{
			Dir:           ".agent-vcr/runs",
			RetentionDays: 30,
			MaxBlobBytes:  10485760,
		},
		Redaction: RedactConfig{
			Enabled:        true,
			RedactEnvFiles: true,
			Patterns: []PatternConfig{
				{Name: "openai_api_key", Regex: "sk-[A-Za-z0-9_-]{20,}"},
				{Name: "private_key", Regex: "-----BEGIN [A-Z ]*PRIVATE KEY-----"},
				{Name: "jwt", Regex: "eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+"},
			},
			Paths: []string{".env", ".env.*", "secrets/**", "**/id_rsa"},
		},
		Rules: RulesConfig{
			MaxChangedFiles:               8,
			ForbiddenPaths:                []string{".env", "secrets/**"},
			RequiredCommands:              []string{},
			RequireTestsAfterSourceChange: true,
			SourceGlobs:                   []string{"src/**"},
			TestGlobs:                     []string{"test/**", "tests/**", "**/*.test.*", "**/*.spec.*"},
		},
		Report: ReportConfig{
			HTML:     true,
			Markdown: false,
		},
	}
}
