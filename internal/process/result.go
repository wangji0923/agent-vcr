package process

import (
	"fmt"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const (
	OutputModeBlob    = "blob"
	OutputModeDiscard = "discard"
)

type RunOptions struct {
	Command    string
	Args       []string
	Cwd        string
	Env        []string
	StdoutMode string
	StderrMode string

	MaxBlobBytes int64
	StdoutName   string
	StderrName   string
	StdoutMIME   string
	StderrMIME   string

	OnStdoutLine func([]byte) error
}

type RunResult struct {
	ExitCode  int
	StartedAt time.Time
	EndedAt   time.Time

	StdoutRef *trace.ArtifactRef
	StderrRef *trace.ArtifactRef

	StdoutTruncated bool
	StderrTruncated bool
	StdoutBytes     int64
	StderrBytes     int64
	StartError      string
}

type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("child process exited with code %d", e.Code)
}

func (e ExitCodeError) ExitCode() int {
	return e.Code
}
