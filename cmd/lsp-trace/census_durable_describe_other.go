//go:build !darwin

package main

import (
	"errors"
	"fmt"
	"io"
)

func preflightCensusDurableDescribe(censusCLIOptions) error {
	return errors.New("normal-folder durable Describe currently requires Darwin")
}
func runCensusDurableDescribe(_ []string, _ io.Reader, _ io.Writer, diagnostic io.Writer) int {
	fmt.Fprintln(diagnostic, "normal-folder durable Describe currently requires Darwin")
	return 1
}
