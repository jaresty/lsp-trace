//go:build !darwin && !linux

package main

import "io"

func writeCensusCapacityFailure(io.Writer, error) bool { return false }
