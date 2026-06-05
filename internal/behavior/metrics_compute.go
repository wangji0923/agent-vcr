package behavior

import (
	"sort"
	"strings"
)

type MetricsOptions struct {
	PathClassifier PathClassifier `json:"-"`
}

type MetricsReport struct {
	Metrics Metrics      `json:"metrics"`
	Facts   MetricsFacts `json:"facts,omitempty"`
}

type MetricsFacts struct {
	RepeatedSearches          int      `json:"repeated_searches,omitempty"`
	FileReadSteps             int      `json:"file_read_steps,omitempty"`
	ShellCommands             int      `json:"shell_commands,omitempty"`
	RepeatedCommands          int      `json:"repeated_commands,omitempty"`
	SkipValidation            bool     `json:"skip_validation,omitempty"`
	ContinuedAfterFailure     bool     `json:"continued_after_failure,omitempty"`
	RepeatedFailure           bool     `json:"repeated_failure,omitempty"`
	CrossUnrelatedDirectories bool     `json:"cross_unrelated_directories,omitempty"`
	EditedDirectories         []string `json:"edited_directories,omitempty"`
}

func ComputeMetrics(timeline Timeline) Metrics {
	return ComputeMetricsWithOptions(timeline, MetricsOptions{}).Metrics
}

func ComputeMetricsWithOptions(timeline Timeline, options MetricsOptions) MetricsReport {
	calculator := metricsCalculator{
		options:          options,
		readFiles:        map[string]int{},
		editedFiles:      map[string]bool{},
		sourceEdits:      map[string]bool{},
		testEdits:        map[string]bool{},
		searches:         map[string]int{},
		commands:         map[string]int{},
		failedCommands:   map[string]int{},
		editedDirs:       map[string]bool{},
		firstEditIndex:   -1,
		firstTestRead:    -1,
		lastFailureIndex: -1,
	}

	for index, step := range timeline.Steps {
		calculator.observeStep(index, step)
	}
	calculator.finalize()
	return MetricsReport{
		Metrics: calculator.metrics,
		Facts:   calculator.facts,
	}
}

type metricsCalculator struct {
	options MetricsOptions

	metrics Metrics
	facts   MetricsFacts

	readFiles      map[string]int
	editedFiles    map[string]bool
	sourceEdits    map[string]bool
	testEdits      map[string]bool
	searches       map[string]int
	commands       map[string]int
	failedCommands map[string]int
	editedDirs     map[string]bool

	firstEditIndex   int
	firstTestRead    int
	lastFailureIndex int
}

func (c *metricsCalculator) observeStep(index int, step Step) {
	c.metrics.ToolEfficiency.TotalSteps++

	if step.Kind == StepSkipValidation {
		c.facts.SkipValidation = true
	}
	if step.Kind == StepSearch {
		c.metrics.ToolEfficiency.SearchSteps++
		c.searches[metricsSearchKey(step)]++
	}
	if isMetricsToolCall(step) {
		c.metrics.ToolEfficiency.ToolCalls++
	}
	if isMetricsShellCommand(step) {
		c.facts.ShellCommands++
		command := metricsCommandKey(step)
		if command != "" {
			c.commands[command]++
		}
	}

	if isMetricsReadStep(step) {
		c.observeRead(index, step)
	}
	if step.Kind == StepEditFile {
		c.observeEdit(index, step)
	}
	if step.Kind == StepRunTest {
		c.observeTest(index, step)
	}
	if isMetricsFailedCommand(step) {
		c.observeFailure(index, step)
	}
	if c.lastFailureIndex >= 0 && index > c.lastFailureIndex && isMetricsRecoveryStep(step) {
		c.metrics.Recovery.RecoveredAfterFailure = true
		c.facts.ContinuedAfterFailure = true
	}
	if c.lastFailureIndex >= 0 && index > c.lastFailureIndex && step.Kind == StepRunTest {
		c.metrics.Recovery.ReranTestsAfterFailure = true
	}

	if c.pathsHaveKind(stepPaths(step), PathLegacy) {
		c.metrics.ContextDiscipline.LegacyPathTouched = true
	}
}

func (c *metricsCalculator) observeRead(index int, step Step) {
	c.facts.FileReadSteps++
	files := stepPaths(step)
	if len(files) == 0 {
		files = []string{NormalizePathForKey(step.Target)}
	}
	for _, file := range metricsNormalizedNonEmpty(files) {
		c.readFiles[file]++
	}
	if c.firstTestRead == -1 && (step.Kind == StepInspectTest || c.pathsHaveKind(files, PathTest)) {
		c.firstTestRead = index
	}
}

func (c *metricsCalculator) observeEdit(index int, step Step) {
	if c.firstEditIndex == -1 {
		c.firstEditIndex = index
	}
	for _, file := range metricsNormalizedNonEmpty(stepPaths(step)) {
		c.editedFiles[file] = true
		pathKind := c.pathKind(file)
		switch pathKind {
		case PathTest:
			c.testEdits[file] = true
		case PathSource:
			c.sourceEdits[file] = true
		}
		dir := metricsTopLevelDirectory(file)
		if dir != "" {
			c.editedDirs[dir] = true
		}
	}
}

func (c *metricsCalculator) observeTest(index int, step Step) {
	c.metrics.Validation.RanAnyTests = true
	if c.firstEditIndex >= 0 && index > c.firstEditIndex {
		c.metrics.Validation.RanTestsAfterEdit = true
	}
	if step.Result == ResultFailure {
		c.metrics.Validation.FailedTestRuns++
	}
}

func (c *metricsCalculator) observeFailure(index int, step Step) {
	c.metrics.ToolEfficiency.FailedCommands++
	c.lastFailureIndex = index
	key := metricsCommandKey(step)
	if key != "" {
		c.failedCommands[key]++
	}
}

func (c *metricsCalculator) finalize() {
	c.metrics.ContextDiscipline.UniqueFilesRead = len(c.readFiles)
	for _, count := range c.readFiles {
		if count > 1 {
			c.metrics.ContextDiscipline.RepeatedReads += count - 1
		}
	}
	if c.firstTestRead >= 0 && c.firstEditIndex >= 0 && c.firstTestRead < c.firstEditIndex {
		c.metrics.ContextDiscipline.ReadTestsBeforeEdit = true
	}

	c.metrics.EditScope.FilesEdited = len(c.editedFiles)
	c.metrics.EditScope.SourceFilesEdited = len(c.sourceEdits)
	c.metrics.EditScope.TestFilesEdited = len(c.testEdits)
	c.metrics.EditScope.SourceToTestEditRatio = metricsSourceToTestRatio(len(c.sourceEdits), len(c.testEdits))

	for _, count := range c.searches {
		if count > 1 {
			c.facts.RepeatedSearches += count - 1
		}
	}
	for _, count := range c.commands {
		if count > 1 {
			c.facts.RepeatedCommands += count - 1
		}
	}
	for _, count := range c.failedCommands {
		if count > 1 {
			c.facts.RepeatedFailure = true
			break
		}
	}

	if c.lastFailureIndex >= 0 && !c.metrics.Recovery.RecoveredAfterFailure && !c.metrics.Recovery.ReranTestsAfterFailure {
		c.metrics.Validation.IgnoredFailedCommand = true
	}
	if len(c.editedFiles) > 0 && !c.metrics.Validation.RanAnyTests {
		c.facts.SkipValidation = true
	}

	c.facts.EditedDirectories = metricsSortedKeys(c.editedDirs)
	c.facts.CrossUnrelatedDirectories = len(c.facts.EditedDirectories) > 1
}

func (c *metricsCalculator) pathsHaveKind(paths []string, kind PathKind) bool {
	for _, path := range paths {
		if c.pathKind(path) == kind {
			return true
		}
	}
	return false
}

func (c *metricsCalculator) pathKind(path string) PathKind {
	normalized := NormalizePathForKey(path)
	if normalized == "" {
		return PathUnknown
	}
	if c.options.PathClassifier != nil {
		classification := c.options.PathClassifier.ClassifyPath(normalized)
		if classification.Kind != "" && classification.Kind != PathUnknown {
			return classification.Kind
		}
	}
	return metricsFallbackPathKind(normalized)
}

func isMetricsReadStep(step Step) bool {
	return step.Kind == StepReadFile || step.Kind == StepInspectTest
}

func isMetricsToolCall(step Step) bool {
	return step.Kind == StepCallTool ||
		step.Kind == StepCallMCPTool ||
		step.ToolName != ""
}

func isMetricsShellCommand(step Step) bool {
	return strings.TrimSpace(step.Command) != ""
}

func isMetricsFailedCommand(step Step) bool {
	if step.Result != ResultFailure {
		return false
	}
	return isMetricsShellCommand(step) ||
		step.Kind == StepRunTest ||
		step.Kind == StepRunBuild ||
		step.Kind == StepCallTool ||
		step.Kind == StepCallMCPTool ||
		step.Kind == StepProcessResult
}

func isMetricsRecoveryStep(step Step) bool {
	switch step.Kind {
	case StepRecoverFromError, StepSearch, StepReadFile, StepInspectTest, StepEditFile:
		return true
	default:
		return false
	}
}

func metricsSearchKey(step Step) string {
	if strings.TrimSpace(step.Query) != "" {
		return normalizeMetricText(step.Query)
	}
	if strings.TrimSpace(step.Command) != "" {
		return normalizeMetricText(step.Command)
	}
	return normalizeMetricText(step.StableKey())
}

func metricsCommandKey(step Step) string {
	if strings.TrimSpace(step.Command) != "" {
		return normalizeMetricText(step.Command)
	}
	if strings.TrimSpace(step.Action) != "" {
		return normalizeMetricText(step.Action)
	}
	return ""
}

func normalizeMetricText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(normalizeUserPathsInText(value)), " "))
}

func stepPaths(step Step) []string {
	paths := append([]string{}, step.Files...)
	if step.Target != "" {
		paths = append(paths, step.Target)
	}
	return paths
}

func metricsNormalizedNonEmpty(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, path := range paths {
		normalized := NormalizePathForKey(path)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func metricsFallbackPathKind(path string) PathKind {
	normalized := strings.ToLower(NormalizePathForKey(path))
	if normalized == "" {
		return PathUnknown
	}
	if metricsIsSecretPath(normalized) {
		return PathSecret
	}
	if metricsIsLegacyPath(normalized) {
		return PathLegacy
	}
	if metricsIsDocsPath(normalized) {
		return PathDocs
	}
	if metricsIsConfigPath(normalized) {
		return PathConfig
	}
	if metricsIsTestPath(normalized) {
		return PathTest
	}
	return PathSource
}

func metricsIsSecretPath(path string) bool {
	base := metricsLastPathPart(path)
	return strings.Contains(path, ".env") ||
		strings.Contains(path, "secret") ||
		strings.Contains(path, "secrets/") ||
		base == "id_rsa" ||
		base == "id_dsa" ||
		base == "id_ecdsa" ||
		base == "id_ed25519"
}

func metricsIsLegacyPath(path string) bool {
	return strings.Contains(path, "legacy") ||
		strings.Contains(path, "deprecated") ||
		strings.Contains(path, "old/")
}

func metricsIsDocsPath(path string) bool {
	return strings.HasPrefix(path, "docs/") ||
		strings.HasSuffix(path, ".md") ||
		strings.HasSuffix(path, ".mdx") ||
		strings.HasSuffix(path, ".rst") ||
		strings.HasSuffix(path, ".txt")
}

func metricsIsConfigPath(path string) bool {
	base := metricsLastPathPart(path)
	return base == "go.mod" ||
		base == "go.sum" ||
		base == "package.json" ||
		base == "tsconfig.json" ||
		base == "makefile" ||
		strings.HasSuffix(base, ".yml") ||
		strings.HasSuffix(base, ".yaml") ||
		strings.HasSuffix(base, ".toml") ||
		strings.HasSuffix(base, ".json")
}

func metricsIsTestPath(path string) bool {
	base := metricsLastPathPart(path)
	return strings.Contains(path, "/test/") ||
		strings.Contains(path, "/tests/") ||
		strings.Contains(path, "/testdata/") ||
		strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasPrefix(base, "test_")
}

func metricsLastPathPart(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func metricsTopLevelDirectory(path string) string {
	normalized := NormalizePathForKey(path)
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "/")
	if len(parts) <= 1 {
		return "."
	}
	return parts[0]
}

func metricsSourceToTestRatio(sourceFiles, testFiles int) float64 {
	if testFiles == 0 {
		return float64(sourceFiles)
	}
	return float64(sourceFiles) / float64(testFiles)
}

func metricsSortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
