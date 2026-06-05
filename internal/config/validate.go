package config

import (
	"fmt"
	"regexp"
)

var (
	captureModes    = map[string]bool{"full": true, "redacted": true, "hash": true, "none": true}
	toolOutputModes = map[string]bool{"inline": true, "blob": true, "hash": true, "none": true}
)

func Validate(cfg Config) error {
	if !captureModes[cfg.Capture.Prompt] {
		return fmt.Errorf("capture.prompt must be one of full/redacted/hash/none: %q", cfg.Capture.Prompt)
	}
	if !captureModes[cfg.Capture.ToolInput] {
		return fmt.Errorf("capture.tool_input must be one of full/redacted/hash/none: %q", cfg.Capture.ToolInput)
	}
	if !toolOutputModes[cfg.Capture.ToolOutput] {
		return fmt.Errorf("capture.tool_output must be one of inline/blob/hash/none: %q", cfg.Capture.ToolOutput)
	}
	if cfg.Storage.Dir == "" {
		return fmt.Errorf("storage.dir must not be empty")
	}
	if cfg.Storage.MaxBlobBytes <= 0 {
		return fmt.Errorf("storage.max_blob_bytes must be > 0")
	}
	if cfg.Rules.MaxChangedFiles < 0 {
		return fmt.Errorf("rules.max_changed_files must be >= 0")
	}
	for _, pattern := range cfg.Redaction.Patterns {
		if pattern.Regex == "" {
			return fmt.Errorf("redaction pattern %q regex must not be empty", pattern.Name)
		}
		if _, err := regexp.Compile(pattern.Regex); err != nil {
			return fmt.Errorf("redaction pattern %q has invalid regex: %w", pattern.Name, err)
		}
	}
	return nil
}
