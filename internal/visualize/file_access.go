package visualize

import (
	"path"
	"sort"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

const (
	FileActionRead  = "read"
	FileActionEdit  = "edit"
	FileActionOther = "other"
)

type FileAction struct {
	Path       string            `json:"path"`
	Action     string            `json:"action"`
	Step       int               `json:"step"`
	PathKind   string            `json:"path_kind,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type FileAccessOptions struct {
	RepoRoot string
}

func BuildFileAccessCompare(lanes []BehaviorLane) FileAccessCompare {
	return BuildFileAccessCompareWithOptions(lanes, FileAccessOptions{})
}

func BuildFileAccessCompareWithOptions(lanes []BehaviorLane, options FileAccessOptions) FileAccessCompare {
	runIDs := collectLaneRunIDs(lanes)
	uses := map[string]map[string]FileUse{}

	for _, lane := range lanes {
		runID := strings.TrimSpace(lane.RunID)
		for _, step := range lane.Steps {
			stepRunID := strings.TrimSpace(step.RunID)
			if runID == "" {
				runID = stepRunID
			}
			if runID == "" {
				continue
			}
			stepOptions := options
			if stepOptions.RepoRoot == "" {
				stepOptions.RepoRoot = stepRepoRoot(step)
			}
			for _, action := range ExtractFileUseWithOptions(step, stepOptions) {
				if action.Path == "" || action.Action == FileActionOther {
					continue
				}
				byRun := uses[action.Path]
				if byRun == nil {
					byRun = map[string]FileUse{}
					uses[action.Path] = byRun
				}
				current := byRun[runID]
				byRun[runID] = applyFileAction(current, action)
			}
		}
	}

	paths := make([]string, 0, len(uses))
	for filePath := range uses {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)

	rows := make([]FileAccessRow, 0, len(paths))
	for _, filePath := range paths {
		runs := map[string]FileUse{}
		for _, runID := range runIDs {
			runs[runID] = uses[filePath][runID]
		}
		for runID, use := range uses[filePath] {
			if strings.TrimSpace(runID) != "" {
				runs[runID] = use
			}
		}
		rows = append(rows, FileAccessRow{Path: filePath, Runs: runs})
	}
	return FileAccessCompare{Rows: rows}
}

func BuildSearchScopeCompare(lanes []BehaviorLane) SearchScopeCompare {
	runIDs := collectLaneRunIDs(lanes)
	uses := map[string]map[string]SearchScopeUse{}

	for _, lane := range lanes {
		runID := strings.TrimSpace(lane.RunID)
		for _, step := range lane.Steps {
			stepRunID := strings.TrimSpace(step.RunID)
			if runID == "" {
				runID = stepRunID
			}
			if runID == "" || step.Kind != VisualStepSearch {
				continue
			}
			for _, scope := range searchScopesForStep(step) {
				if scope == "" {
					continue
				}
				byRun := uses[scope]
				if byRun == nil {
					byRun = map[string]SearchScopeUse{}
					uses[scope] = byRun
				}
				current := byRun[runID]
				byRun[runID] = applySearchScopeUse(current, step)
			}
		}
	}

	scopes := make([]string, 0, len(uses))
	for scope := range uses {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	rows := make([]SearchScopeRow, 0, len(scopes))
	for _, scope := range scopes {
		runs := map[string]SearchScopeUse{}
		for _, runID := range runIDs {
			runs[runID] = uses[scope][runID]
		}
		for runID, use := range uses[scope] {
			if strings.TrimSpace(runID) != "" {
				runs[runID] = use
			}
		}
		rows = append(rows, SearchScopeRow{Scope: scope, Runs: runs})
	}
	return SearchScopeCompare{Rows: rows}
}

func ExtractFileUse(step VisualStep) []FileAction {
	return ExtractFileUseWithOptions(step, FileAccessOptions{})
}

func ExtractFileUseWithOptions(step VisualStep, options FileAccessOptions) []FileAction {
	action := actionForStepKind(step.Kind)
	files := stepFiles(step)
	if len(files) == 0 {
		return nil
	}
	if options.RepoRoot == "" {
		options.RepoRoot = stepRepoRoot(step)
	}

	seen := map[string]bool{}
	actions := make([]FileAction, 0, len(files))
	for _, file := range files {
		normalized := NormalizeFileAccessPathWithRoot(file, options.RepoRoot)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		pathKind, attrs := classifyFileActionPath(normalized, step.Attributes)
		actions = append(actions, FileAction{
			Path:       normalized,
			Action:     action,
			Step:       step.Index,
			PathKind:   pathKind,
			Attributes: attrs,
		})
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Path == actions[j].Path {
			return actions[i].Action < actions[j].Action
		}
		return actions[i].Path < actions[j].Path
	})
	return actions
}

func NormalizeFileAccessPath(filePath string) string {
	return NormalizeFileAccessPathWithRoot(filePath, "")
}

func NormalizeFileAccessPathWithRoot(filePath string, repoRoot string) string {
	normalized := behavior.NormalizePathForKey(filePath)
	if normalized == "" {
		return ""
	}
	root := behavior.NormalizePathForKey(repoRoot)
	if root != "" {
		normalized = stripPathPrefix(normalized, root)
	}
	normalized = path.Clean(normalized)
	if normalized == "." {
		return ""
	}
	return strings.Trim(normalized, "/")
}

func collectLaneRunIDs(lanes []BehaviorLane) []string {
	seen := map[string]bool{}
	var runIDs []string
	for _, lane := range lanes {
		runID := strings.TrimSpace(lane.RunID)
		if runID == "" {
			for _, step := range lane.Steps {
				runID = strings.TrimSpace(step.RunID)
				if runID != "" {
					break
				}
			}
		}
		if runID == "" || seen[runID] {
			continue
		}
		seen[runID] = true
		runIDs = append(runIDs, runID)
	}
	return runIDs
}

func actionForStepKind(kind VisualStepKind) string {
	switch kind {
	case VisualStepReadFile, VisualStepInspectTest:
		return FileActionRead
	case VisualStepEditFile:
		return FileActionEdit
	default:
		return FileActionOther
	}
}

func applyFileAction(use FileUse, action FileAction) FileUse {
	switch action.Action {
	case FileActionRead:
		use.ReadCount++
	case FileActionEdit:
		use.EditCount++
	}
	if use.FirstAction == "" || action.Step < use.FirstStep {
		use.FirstStep = action.Step
		use.FirstAction = action.Action
	}
	if use.LastAction == "" || action.Step >= use.LastStep {
		use.LastStep = action.Step
		use.LastAction = action.Action
	}
	return use
}

func applySearchScopeUse(use SearchScopeUse, step VisualStep) SearchScopeUse {
	use.SearchCount++
	if use.FirstStep == 0 || step.Index < use.FirstStep {
		use.FirstStep = step.Index
	}
	if step.Index >= use.LastStep {
		use.LastStep = step.Index
	}
	query := firstNonBlank(step.Query, step.Command, step.Summary)
	if query != "" && !stringInSlice(use.Queries, query) {
		use.Queries = append(use.Queries, query)
		sort.Strings(use.Queries)
	}
	return use
}

func searchScopesForStep(step VisualStep) []string {
	candidates := append([]string{}, step.Files...)
	for _, value := range []string{step.Target} {
		if strings.TrimSpace(value) != "" {
			candidates = append(candidates, value)
		}
	}
	seen := map[string]bool{}
	scopes := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		normalized := NormalizeFileAccessPath(candidate)
		if normalized == "" || isLikelyFilePath(normalized) || seen[normalized] {
			continue
		}
		seen[normalized] = true
		scopes = append(scopes, normalized)
	}
	sort.Strings(scopes)
	return scopes
}

func stepFiles(step VisualStep) []string {
	files := append([]string(nil), step.Files...)
	if len(files) == 0 && isLikelyFilePath(step.Target) {
		files = append(files, step.Target)
	}
	return files
}

func isLikelyFilePath(value string) bool {
	normalized := behavior.NormalizePathForKey(value)
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "/") {
		return true
	}
	base := strings.ToLower(path.Base(normalized))
	switch base {
	case "dockerfile", "makefile", "readme", "license":
		return true
	}
	return strings.Contains(base, ".")
}

func stepRepoRoot(step VisualStep) string {
	if step.Attributes == nil {
		return ""
	}
	keys := []string{"repo_root", "project_dir", "workspace_root", "cwd"}
	for _, key := range keys {
		if value := strings.TrimSpace(step.Attributes[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringInSlice(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func stripPathPrefix(filePath string, prefix string) string {
	filePath = strings.Trim(filePath, "/")
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return filePath
	}
	if strings.EqualFold(filePath, prefix) {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(filePath), strings.ToLower(prefix)+"/") {
		return strings.TrimPrefix(filePath[len(prefix):], "/")
	}
	return filePath
}

func classifyFileActionPath(filePath string, stepAttrs map[string]string) (string, map[string]string) {
	attrs := map[string]string{}
	for key, value := range behavior.ClassifyPath(filePath).Attributes {
		attrs[key] = value
	}
	mergeStepPathAttrs(attrs, stepAttrs)

	pathKind := strings.TrimSpace(stepPathKind(stepAttrs))
	if pathKind == "" {
		pathKind = string(behavior.ClassifyPath(filePath).Kind)
	}
	if pathKind == string(behavior.PathUnknown) {
		if attrs["is_legacy"] == "true" || attrs["is_deprecated"] == "true" {
			pathKind = string(behavior.PathLegacy)
		} else if attrs["is_test"] == "true" {
			pathKind = string(behavior.PathTest)
		} else if attrs["is_source"] == "true" {
			pathKind = string(behavior.PathSource)
		}
	}
	if pathKind != "" {
		attrs["path_kind"] = pathKind
	}
	if len(attrs) == 0 {
		return pathKind, nil
	}
	return pathKind, attrs
}

func mergeStepPathAttrs(dst map[string]string, src map[string]string) {
	for _, key := range []string{"is_test", "is_source", "is_legacy", "is_deprecated"} {
		if value := strings.TrimSpace(src[key]); value != "" {
			dst[key] = value
		}
	}
}

func stepPathKind(attrs map[string]string) string {
	if attrs == nil {
		return ""
	}
	for _, key := range []string{"path_kind", "file_kind", "file_type"} {
		value := strings.TrimSpace(attrs[key])
		if value != "" {
			return value
		}
	}
	if attrs["is_legacy"] == "true" || attrs["is_deprecated"] == "true" {
		return string(behavior.PathLegacy)
	}
	if attrs["is_test"] == "true" {
		return string(behavior.PathTest)
	}
	if attrs["is_source"] == "true" {
		return string(behavior.PathSource)
	}
	return ""
}
