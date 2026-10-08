//go:build !darwin

package describeworker

import "errors"

func makeRunnerFIFO(string) error {
	return errors.New("runner FIFO unsupported platform")
}

func runnerProcessAlive(int) bool {
	return false
}
