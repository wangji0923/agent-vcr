package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/adapters"
)

const hookCommand = "agent-vcr hook --adapter codex"

func InstallHooks(opts adapters.InstallOptions) error {
	scope := strings.TrimSpace(opts.Scope)
	if scope == "" {
		scope = "project"
	}

	hooksPath, err := hooksPath(scope, opts.ProjectDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return err
	}

	root := map[string]any{}
	exists := false
	if data, err := os.ReadFile(hooksPath); err == nil {
		exists = true
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &root); err != nil {
				return fmt.Errorf("parse %s: %w", hooksPath, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	changed, err := mergeAgentVCRHooks(root, opts.Force)
	if err != nil {
		return err
	}
	if !changed && exists {
		return nil
	}

	if exists {
		if err := backupFile(hooksPath); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(hooksPath, append(data, '\n'), 0o644)
}

func hooksPath(scope, projectDir string) (string, error) {
	switch scope {
	case "project":
		if projectDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			projectDir = cwd
		}
		abs, err := filepath.Abs(projectDir)
		if err != nil {
			return "", err
		}
		return filepath.Join(abs, ".codex", "hooks.json"), nil
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".codex", "hooks.json"), nil
	default:
		return "", fmt.Errorf("scope must be project or user: %q", scope)
	}
}

func mergeAgentVCRHooks(root map[string]any, force bool) (bool, error) {
	hooks, err := hooksObject(root)
	if err != nil {
		return false, err
	}

	changed := false
	for eventName, desired := range desiredHookEntries() {
		entries, err := hookEntries(hooks[eventName])
		if err != nil {
			return false, fmt.Errorf("hooks.%s: %w", eventName, err)
		}
		found, updated := findOrUpdateAgentVCRHook(entries, desired, force)
		if updated {
			changed = true
		}
		if !found {
			entries = append(entries, desired)
			changed = true
		}
		hooks[eventName] = entries
	}
	root["hooks"] = hooks
	return changed, nil
}

func hooksObject(root map[string]any) (map[string]any, error) {
	if root == nil {
		root = map[string]any{}
	}
	value, ok := root["hooks"]
	if !ok || value == nil {
		hooks := map[string]any{}
		root["hooks"] = hooks
		return hooks, nil
	}
	hooks, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("hooks must be an object")
	}
	return hooks, nil
}

func hookEntries(value any) ([]any, error) {
	if value == nil {
		return []any{}, nil
	}
	entries, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("event hooks must be an array")
	}
	return entries, nil
}

func findOrUpdateAgentVCRHook(entries []any, desired map[string]any, force bool) (bool, bool) {
	updated := false
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if !ok || hookStringValue(hookMap["command"]) != hookCommand {
				continue
			}
			if force {
				desiredHook := desiredCommand(desired)
				for key, value := range desiredHook {
					if hookMap[key] != value {
						hookMap[key] = value
						updated = true
					}
				}
			}
			return true, updated
		}
	}
	return false, updated
}

func desiredCommand(entry map[string]any) map[string]any {
	hooks, _ := entry["hooks"].([]any)
	if len(hooks) == 0 {
		return map[string]any{}
	}
	hook, _ := hooks[0].(map[string]any)
	return hook
}

func desiredHookEntries() map[string]map[string]any {
	return map[string]map[string]any{
		eventSessionStart:     hookEntry("startup|resume|clear|compact", 10, "agent-vcr: recording session"),
		eventUserPromptSubmit: hookEntry("", 10, "agent-vcr: recording prompt"),
		eventPreToolUse:       hookEntry("Bash|apply_patch|mcp__.*", 10, "agent-vcr: recording tool call"),
		eventPostToolUse:      hookEntry("Bash|apply_patch|mcp__.*", 30, "agent-vcr: recording tool result"),
		eventPermission:       hookEntry("Bash|apply_patch|mcp__.*", 10, "agent-vcr: recording permission request"),
		eventPreCompact:       hookEntry("manual|auto", 10, ""),
		eventPostCompact:      hookEntry("manual|auto", 10, ""),
		eventSubagentStart:    hookEntry(".*", 10, ""),
		eventSubagentStop:     hookEntry(".*", 10, ""),
		eventStop:             hookEntry("", 10, "agent-vcr: finalizing run"),
	}
}

func hookEntry(matcher string, timeout int, statusMessage string) map[string]any {
	command := map[string]any{
		"type":    "command",
		"command": hookCommand,
		"timeout": float64(timeout),
	}
	if statusMessage != "" {
		command["statusMessage"] = statusMessage
	}
	entry := map[string]any{
		"hooks": []any{command},
	}
	if matcher != "" {
		entry["matcher"] = matcher
	}
	return entry
}

func backupFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	backupPath := path + ".bak." + time.Now().UTC().Format("20060102T150405.000000000Z")
	return os.WriteFile(backupPath, data, 0o644)
}
