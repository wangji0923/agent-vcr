package trace

import "time"

type RawEvent struct {
	Source     Source       `json:"source"`
	Data       []byte       `json:"-"`
	Payload    Payload      `json:"payload,omitempty"`
	ReceivedAt time.Time    `json:"received_at,omitempty"`
	RawRef     *ArtifactRef `json:"raw_ref,omitempty"`
}
