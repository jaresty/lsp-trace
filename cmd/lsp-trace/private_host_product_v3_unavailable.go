//go:build !darwin && !linux

package main

import "io"

func runPrivateHostProductV3([]string, io.Reader, io.Writer, io.Writer) int { return 1 }
