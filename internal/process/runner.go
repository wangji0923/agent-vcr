package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func Run(ctx context.Context, store *trace.Store, opts RunOptions) (RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Command == "" {
		return RunResult{}, errors.New("command must not be empty")
	}
	if opts.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return RunResult{}, err
		}
		opts.Cwd = cwd
	}
	opts.Cwd = filepath.Clean(opts.Cwd)

	result := RunResult{StartedAt: time.Now().UTC(), ExitCode: 0}
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = opts.Cwd
	cmd.Env = append(os.Environ(), opts.Env...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, err
	}

	stdoutCapture := newLimitedCapture(opts.MaxBlobBytes)
	stderrCapture := newLimitedCapture(opts.MaxBlobBytes)

	if err := cmd.Start(); err != nil {
		result.EndedAt = time.Now().UTC()
		result.ExitCode = 127
		result.StartError = err.Error()
		if writeErr := writeCapturedBlobs(store, opts, stdoutCapture, stderrCapture, &result); writeErr != nil {
			return result, writeErr
		}
		return result, nil
	}

	stopSignals := forwardSignals(cmd)
	defer stopSignals()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- streamOutput(stdout, stdoutCapture, opts.OnStdoutLine)
	}()
	go func() {
		defer wg.Done()
		errs <- streamOutput(stderr, stderrCapture, nil)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	close(errs)

	result.EndedAt = time.Now().UTC()
	result.ExitCode = exitCodeFromWait(waitErr)
	if ctx.Err() != nil && result.ExitCode == 0 {
		result.ExitCode = 130
	}

	var streamErr error
	for err := range errs {
		if err != nil && streamErr == nil {
			streamErr = err
		}
	}
	if writeErr := writeCapturedBlobs(store, opts, stdoutCapture, stderrCapture, &result); writeErr != nil {
		return result, writeErr
	}
	if streamErr != nil {
		return result, streamErr
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return result, waitErr
		}
	}
	return result, nil
}

func exitCodeFromWait(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func writeCapturedBlobs(store *trace.Store, opts RunOptions, stdoutCapture, stderrCapture *limitedCapture, result *RunResult) error {
	result.StdoutBytes = stdoutCapture.TotalBytes()
	result.StderrBytes = stderrCapture.TotalBytes()
	result.StdoutTruncated = stdoutCapture.Truncated()
	result.StderrTruncated = stderrCapture.Truncated()

	if store == nil {
		return nil
	}
	if opts.StdoutMode == "" {
		opts.StdoutMode = OutputModeBlob
	}
	if opts.StderrMode == "" {
		opts.StderrMode = OutputModeBlob
	}
	if opts.StdoutName == "" {
		opts.StdoutName = "process_stdout.txt"
	}
	if opts.StderrName == "" {
		opts.StderrName = "process_stderr.txt"
	}
	if opts.StdoutMIME == "" {
		opts.StdoutMIME = "text/plain"
	}
	if opts.StderrMIME == "" {
		opts.StderrMIME = "text/plain"
	}

	if opts.StdoutMode == OutputModeBlob {
		ref, err := store.WriteBlob(opts.StdoutName, stdoutCapture.Bytes(), opts.StdoutMIME)
		if err != nil {
			return fmt.Errorf("write stdout blob: %w", err)
		}
		result.StdoutRef = &ref
	}
	if opts.StderrMode == OutputModeBlob {
		ref, err := store.WriteBlob(opts.StderrName, stderrCapture.Bytes(), opts.StderrMIME)
		if err != nil {
			return fmt.Errorf("write stderr blob: %w", err)
		}
		result.StderrRef = &ref
	}
	return nil
}

func forwardSignals(cmd *exec.Cmd) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, forwardedSignals()...)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for sig := range signals {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	return func() {
		signal.Stop(signals)
		close(signals)
		<-done
	}
}
