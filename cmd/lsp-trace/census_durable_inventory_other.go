//go:build !darwin && !linux

package main

import (
	"errors"
	"io"
)

func preflightCensusInventoryStore(workspace, explicit string) (string, string, error) {
	return "", "", errors.New("durable inventory unsupported on this platform")
}

func runCensusDurable(args []string, input io.Reader, out, diagnostic io.Writer) int {
	return runCensusDurableDescribe(args, input, out, diagnostic)
}
