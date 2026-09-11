//go:build windows

package artifactingress

import "errors"

func makeFIFO(string) error { return errors.New("FIFO unavailable on Windows") }
