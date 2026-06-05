# 02-config-trace-store.md

> 模块目标：实现配置加载、统一 Trace Schema、Trace Store、Blob Store、Run Resolver、文件锁与基础 Git Snapshot。
> 本模块禁止实现 Codex Adapter、JSONL Record、Replay、Diff、Check、HTML Report。

---

## 1. 本模块交付物

新增或修改：

```text
internal/config/config.go
internal/config/default.go
internal/config/load.go
internal/config/validate.go
internal/trace/event.go
internal/trace/artifact.go
internal/trace/metadata.go
internal/trace/store.go
internal/trace/run_resolver.go
internal/trace/lock.go
internal/gitutil/repo.go
internal/gitutil/snapshot.go
internal/gitutil/diff.go
internal/adapters/adapter.go  # 补 Normalize 方法
```

必须支持能力：

```text
加载 .agent-vcr/config.yml
创建 run 目录
写 metadata.json
append trace.ndjson
写 blob
写 patch
git repo 探测
git snapshot
latest run resolver
```

---

## 2. 配置结构设计

文件：`internal/config/config.go`

v0.1 说明：

```text
配置结构先保留 capture/storage/redaction/rules/report 的统一表面。
当前实现采用 privacy-first 默认值：
- run storage 是 `.agent-vcr/runs`
- prompt/tool content 在 adapter 中按可观测能力做 redacted/hash/blob 处理
- capture.prompt / capture.tool_input / capture.tool_output 的完整策略控制是后续版本能力
```

```go
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
    Dir          string `yaml:"dir" json:"dir"`
    RetentionDays int  `yaml:"retention_days" json:"retention_days"`
    MaxBlobBytes int64 `yaml:"max_blob_bytes" json:"max_blob_bytes"`
}

type RedactConfig struct {
    Enabled        bool              `yaml:"enabled" json:"enabled"`
    RedactEnvFiles bool              `yaml:"redact_env_files" json:"redact_env_files"`
    Patterns       []PatternConfig   `yaml:"patterns" json:"patterns"`
    Paths          []string          `yaml:"paths" json:"paths"`
}

type PatternConfig struct {
    Name  string `yaml:"name" json:"name"`
    Regex string `yaml:"regex" json:"regex"`
}

type RulesConfig struct {
    MaxChangedFiles                 int      `yaml:"max_changed_files" json:"max_changed_files"`
    ForbiddenPaths                  []string `yaml:"forbidden_paths" json:"forbidden_paths"`
    RequiredCommands                []string `yaml:"required_commands" json:"required_commands"`
    RequireTestsAfterSourceChange   bool     `yaml:"require_tests_after_source_change" json:"require_tests_after_source_change"`
    SourceGlobs                     []string `yaml:"source_globs" json:"source_globs"`
    TestGlobs                       []string `yaml:"test_globs" json:"test_globs"`
}

type ReportConfig struct {
    HTML     bool `yaml:"html" json:"html"`
    Markdown bool `yaml:"markdown" json:"markdown"`
}
```

---

## 3. 默认配置

文件：`internal/config/default.go`

```go
func Default() Config
```

默认值：

```yaml
version: "0.2"
capture:
  prompt: "redacted"
  tool_input: "redacted"
  tool_output: "blob"
  git_diff: true
  final_diff: true
  max_inline_bytes: 4096
storage:
  dir: ".agent-vcr/runs"
  retention_days: 30
  max_blob_bytes: 10485760
redaction:
  enabled: true
  redact_env_files: true
  patterns:
    - name: "openai_api_key"
      regex: "sk-[A-Za-z0-9_-]{20,}"
    - name: "private_key"
      regex: "-----BEGIN [A-Z ]*PRIVATE KEY-----"
    - name: "jwt"
      regex: "eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+"
  paths:
    - ".env"
    - ".env.*"
    - "secrets/**"
    - "**/id_rsa"
rules:
  max_changed_files: 8
  forbidden_paths:
    - ".env"
    - "secrets/**"
  required_commands: []
  require_tests_after_source_change: true
  source_globs:
    - "src/**"
  test_globs:
    - "test/**"
    - "tests/**"
    - "**/*.test.*"
    - "**/*.spec.*"
report:
  html: true
  markdown: false
```

---

## 4. 配置加载

文件：`internal/config/load.go`

函数：

```go
func Load(projectDir string, explicitPath string) (Config, string, error)
```

加载顺序：

1. 从 `Default()` 开始。
2. 如果 `explicitPath` 不为空，加载该文件。
3. 否则查找 `<projectDir>/.agent-vcr/config.yml`。
4. 如果不存在，返回默认配置。
5. YAML 覆盖默认配置。
6. 调用 Validate。

注意：

- 不要因为配置不存在报错。
- 配置解析失败要返回 error。
- Validate 失败要返回 error。

---

## 5. 配置校验

文件：`internal/config/validate.go`

校验项：

```text
capture.prompt 必须是 full/redacted/hash/none
capture.tool_input 必须是 full/redacted/hash/none
capture.tool_output 必须是 inline/blob/hash/none
storage.dir 不可为空
storage.max_blob_bytes 必须 > 0
regex 必须能编译
max_changed_files 必须 >= 0
```

测试：

```text
默认配置合法
非法枚举报错
非法 regex 报错
不存在配置文件不报错
显式配置文件加载成功
```

---

## 6. Trace Event Schema

文件：`internal/trace/event.go`

实现：

```go
type EventType string

const (
    EventRunStart          EventType = "run_start"
    EventRunStop           EventType = "run_stop"
    EventUserPrompt        EventType = "user_prompt"
    EventModelCall         EventType = "model_call"
    EventModelResult       EventType = "model_result"
    EventToolCall          EventType = "tool_call"
    EventToolResult        EventType = "tool_result"
    EventToolError         EventType = "tool_error"
    EventFileRead          EventType = "file_read"
    EventFileWrite         EventType = "file_write"
    EventFilePatch         EventType = "file_patch"
    EventShellCommand      EventType = "shell_command"
    EventShellResult       EventType = "shell_result"
    EventPermissionRequest EventType = "permission_request"
    EventSubagentStart     EventType = "subagent_start"
    EventSubagentStop      EventType = "subagent_stop"
    EventContextCompact    EventType = "context_compact"
    EventProcessStart      EventType = "process_start"
    EventProcessResult     EventType = "process_result"
    EventRaw               EventType = "raw_event"
)

type Event struct {
    SchemaVersion string         `json:"schema_version"`
    EventID       string         `json:"event_id"`
    RunID         string         `json:"run_id"`
    ParentID      string         `json:"parent_id,omitempty"`
    SpanID        string         `json:"span_id,omitempty"`
    EventIndex    int64          `json:"event_index"`
    Type          EventType      `json:"type"`
    Source        EventSource    `json:"source"`
    Timestamp     time.Time      `json:"timestamp"`
    Payload       map[string]any `json:"payload,omitempty"`
    Artifacts     []ArtifactRef  `json:"artifacts,omitempty"`
    RawRef        *ArtifactRef   `json:"raw_ref,omitempty"`
}

type EventSource struct {
    Adapter      string `json:"adapter"`
    Agent        string `json:"agent,omitempty"`
    RawEventType string `json:"raw_event_type,omitempty"`
    Version      string `json:"version,omitempty"`
}
```

辅助函数：

```go
func NewEvent(runID string, typ EventType, source EventSource) Event
func NewRawEvent(runID string, source EventSource, raw ArtifactRef) Event
```

---

## 7. ArtifactRef

文件：`internal/trace/artifact.go`

```go
type ArtifactRef struct {
    Kind      string `json:"kind"`
    Path      string `json:"path"`
    Sha256    string `json:"sha256,omitempty"`
    SizeBytes int64  `json:"size_bytes,omitempty"`
    MimeType  string `json:"mime_type,omitempty"`
    Redacted  bool   `json:"redacted,omitempty"`
}
```

Kind 枚举：

```text
blob
patch
snapshot
report
raw
```

---

## 8. Metadata

文件：`internal/trace/metadata.go`

```go
type Metadata struct {
    SchemaVersion string                 `json:"schema_version"`
    RunID         string                 `json:"run_id"`
    Source        string                 `json:"source"`
    Status        string                 `json:"status"`
    Cwd           string                 `json:"cwd"`
    RepoRoot      string                 `json:"repo_root,omitempty"`
    GitSHA        string                 `json:"git_sha,omitempty"`
    Branch        string                 `json:"branch,omitempty"`
    StartedAt     time.Time              `json:"started_at"`
    EndedAt       *time.Time             `json:"ended_at,omitempty"`
    Capabilities  map[string]bool        `json:"capabilities,omitempty"`
    Summary       map[string]any         `json:"summary,omitempty"`
}
```

Status：

```text
running
completed
failed
unknown
```

---

## 9. Trace Store

文件：`internal/trace/store.go`

结构：

```go
type Store struct {
    ProjectDir string
    RunsDir    string
    RunID      string
    RunDir     string
}
```

函数：

```go
func CreateRun(projectDir string, source string) (*Store, error)
func OpenRun(projectDir string, runID string) (*Store, error)
func (s *Store) Append(event Event) error
func (s *Store) WriteMetadata(meta Metadata) error
func (s *Store) ReadMetadata() (Metadata, error)
func (s *Store) WriteBlob(name string, data []byte, mime string) (ArtifactRef, error)
func (s *Store) WritePatch(name string, data []byte) (ArtifactRef, error)
func (s *Store) Path(parts ...string) string
```

Append 行为：

1. 获取 lock。
2. 读取当前 event_index。
3. event_index + 1。
4. 如果 EventID 为空，生成。
5. 如果 Timestamp 为空，填当前时间。
6. 一行 JSON append 到 `trace.ndjson`。
7. fsync 可选。
8. 释放 lock。

---

## 10. 文件锁

文件：`internal/trace/lock.go`

可以先使用简单 lock file：

```text
.agent-vcr/state/locks/<run-id>.lock
```

推荐依赖：

```bash
go get github.com/gofrs/flock@latest
```

函数：

```go
func WithRunLock(projectDir, runID string, fn func() error) error
```

要求：

- lock timeout 默认 10s。
- lock 获取失败返回 error。
- hook 调用处必须吞掉错误并 exit 0。
- CLI 分析命令可以正常报错。

---

## 11. Run Resolver

文件：`internal/trace/run_resolver.go`

函数：

```go
func ListRuns(projectDir string) ([]Metadata, error)
func ResolveRunID(projectDir string, ref string) (string, error)
```

支持：

```text
latest
完整 run_id
run_id 前缀
```

排序：

```text
started_at desc
```

错误：

```text
找不到 run
前缀匹配多个 run
metadata 损坏
```

---

## 12. Git 工具

文件：`internal/gitutil/repo.go`

函数：

```go
func FindRepoRoot(cwd string) (string, error)
func CurrentSHA(cwd string) (string, error)
func CurrentBranch(cwd string) (string, error)
func IsGitRepo(cwd string) bool
```

实现方式：调用 git 命令。

文件：`internal/gitutil/snapshot.go`

```go
type Snapshot struct {
    Head       string   `json:"head,omitempty"`
    Branch     string   `json:"branch,omitempty"`
    Status     string   `json:"status,omitempty"`
    DiffSHA256 string   `json:"diff_sha256,omitempty"`
    Files      []string `json:"files,omitempty"`
}

func CaptureSnapshot(cwd string) (Snapshot, []byte, error)
```

采集命令：

```bash
git rev-parse HEAD
git branch --show-current
git status --porcelain=v1
git diff --binary
git diff --cached --binary
```

文件：`internal/gitutil/diff.go`

```go
func ChangedFilesFromStatus(status string) []string
func DiffSummary(diff []byte) map[string]any
```

---

## 13. 补 Adapter Normalize 接口

修改 `internal/adapters/adapter.go`：

```go
Normalize(ctx context.Context, raw RawEvent) ([]trace.Event, error)
```

注意导入 `internal/trace` 后，adapter 包仍然不能导入具体 adapter。

---

## 14. 测试要求

新增测试：

```text
internal/config/*_test.go
internal/trace/*_test.go
internal/gitutil/*_test.go
```

测试命令：

```bash
go test ./...
```

重点测试：

1. Config default 合法。
2. Config YAML 覆盖默认值。
3. Store 创建 run 目录。
4. Append 写入 NDJSON。
5. Blob 写入并返回 sha256。
6. Run resolver latest 正确。
7. Git repo 探测在非 git 目录不 panic。
8. Adapter Registry 仍然通过。

---

## 15. Codex 执行提示词

```text
请只实现 docs/02-config-trace-store.md 的内容。
不要实现 Codex Adapter、record、replay、diff、check、report。

硬性约束：
- Trace schema 必须通用，不得出现 codex_ 前缀字段。
- internal/trace 不得 import internal/adapters/*。
- internal/adapters 只能依赖 trace，不得依赖 analysis/report/check。
- Normalize 失败兜底 RawEvent 的结构要预留。
- 所有新增模块必须有单元测试。
- 最后运行 go test ./...。
```

---

## 16. 完成标准

- `go test ./...` 通过。
- `.agent-vcr/runs/<run-id>/trace.ndjson` 可被创建和追加。
- metadata.json 可读写。
- blob / patch 可写入并有 sha256。
- latest run resolver 可用。
- `trace.Event` 没有 Codex 专用字段。
