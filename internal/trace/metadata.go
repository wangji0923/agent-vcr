package trace

import "time"

const (
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
	RunStatusUnknown   = "unknown"
)

type Metadata struct {
	SchemaVersion string          `json:"schema_version"`
	RunID         string          `json:"run_id"`
	Source        string          `json:"source"`
	Status        string          `json:"status"`
	Cwd           string          `json:"cwd"`
	RepoRoot      string          `json:"repo_root,omitempty"`
	GitSHA        string          `json:"git_sha,omitempty"`
	Branch        string          `json:"branch,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	EndedAt       *time.Time      `json:"ended_at,omitempty"`
	Capabilities  map[string]bool `json:"capabilities,omitempty"`
	Summary       Payload         `json:"summary,omitempty"`
}
