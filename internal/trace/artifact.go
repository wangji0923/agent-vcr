package trace

type ArtifactKind string

const (
	ArtifactBlob     ArtifactKind = "blob"
	ArtifactPatch    ArtifactKind = "patch"
	ArtifactSnapshot ArtifactKind = "snapshot"
	ArtifactReport   ArtifactKind = "report"
	ArtifactRaw      ArtifactKind = "raw"
)

type ArtifactRef struct {
	Kind      ArtifactKind `json:"kind"`
	Path      string       `json:"path"`
	SHA256    string       `json:"sha256,omitempty"`
	SizeBytes int64        `json:"size_bytes,omitempty"`
	MimeType  string       `json:"mime_type,omitempty"`
	Redacted  bool         `json:"redacted,omitempty"`
}
