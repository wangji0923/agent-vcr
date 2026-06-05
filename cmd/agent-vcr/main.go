package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/agent-vcr/agent-vcr/internal/cli"
)

type exitCoder interface {
	ExitCode() int
}

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		var coder exitCoder
		if errors.As(err, &coder) {
			os.Exit(coder.ExitCode())
		}
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
