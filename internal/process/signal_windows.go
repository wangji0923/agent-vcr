//go:build windows

package process

import "os"

func forwardedSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
