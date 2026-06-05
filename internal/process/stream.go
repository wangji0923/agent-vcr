package process

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
)

type limitedCapture struct {
	max       int64
	total     int64
	truncated bool
	buffer    bytes.Buffer
}

func newLimitedCapture(max int64) *limitedCapture {
	if max <= 0 {
		max = 10 * 1024 * 1024
	}
	return &limitedCapture{max: max}
}

func (c *limitedCapture) Write(p []byte) (int, error) {
	c.total += int64(len(p))
	remaining := c.max - int64(c.buffer.Len())
	if remaining <= 0 {
		if len(p) > 0 {
			c.truncated = true
		}
		return len(p), nil
	}
	writeLen := len(p)
	if int64(writeLen) > remaining {
		writeLen = int(remaining)
		c.truncated = true
	}
	_, err := c.buffer.Write(p[:writeLen])
	if len(p) > writeLen {
		c.truncated = true
	}
	return len(p), err
}

func (c *limitedCapture) Bytes() []byte {
	return c.buffer.Bytes()
}

func (c *limitedCapture) TotalBytes() int64 {
	return c.total
}

func (c *limitedCapture) Truncated() bool {
	return c.truncated
}

func streamOutput(reader io.Reader, capture *limitedCapture, onLine func([]byte) error) error {
	bufReader := bufio.NewReader(reader)
	var callbackErr error
	for {
		chunk, err := bufReader.ReadBytes('\n')
		if len(chunk) > 0 {
			if capture != nil {
				if _, writeErr := capture.Write(chunk); writeErr != nil && callbackErr == nil {
					callbackErr = writeErr
				}
			}
			if onLine != nil {
				line := trimLineTerminator(chunk)
				if lineErr := onLine(line); lineErr != nil && callbackErr == nil {
					callbackErr = lineErr
				}
			}
		}
		if err == io.EOF || errors.Is(err, os.ErrClosed) {
			return callbackErr
		}
		if err != nil {
			return err
		}
	}
}

func trimLineTerminator(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	out := make([]byte, len(line))
	copy(out, line)
	return out
}
