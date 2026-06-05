package behavior

import "time"

const SchemaVersion = "0.1"

type SignatureOptions struct {
	IncludeRawBehavior  bool `json:"include_raw_behavior,omitempty"`
	IncludeProcessNoise bool `json:"include_process_noise,omitempty"`
	IncludeSourceRefs   bool `json:"include_source_refs,omitempty"`
	NormalizeUserPaths  bool `json:"normalize_user_paths,omitempty"`
}

type Signature struct {
	SchemaVersion   string           `json:"schema_version"`
	RunID           string           `json:"run_id"`
	SourceTraceHash string           `json:"source_trace_hash,omitempty"`
	GeneratedAt     time.Time        `json:"generated_at"`
	Steps           []Step           `json:"steps"`
	Metrics         Metrics          `json:"metrics"`
	Options         SignatureOptions `json:"options,omitempty"`
}
