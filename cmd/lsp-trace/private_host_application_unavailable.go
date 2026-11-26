//go:build !darwin && !linux

package main

import "io"

func runPrivateHostApplication(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_, _ = io.WriteString(stderr, "private host application unavailable on this platform\n")
	return 1
}

func runPrivateHostCohort(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_, _ = io.WriteString(stderr, "private host cohort unavailable on this platform\n")
	return 1
}
