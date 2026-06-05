package trace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const defaultLockTimeout = 10 * time.Second

func WithRunLock(projectDir, runID string, fn func() error) error {
	if projectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectDir = cwd
	}
	lockDir := filepath.Join(projectDir, ".agent-vcr", "state", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(lockDir, filepath.Base(runID)+".lock")
	deadline := time.Now().Add(defaultLockTimeout)

	for {
		file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
			_ = file.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !os.IsExist(err) && !os.IsPermission(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for run lock: %s", lockPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
