package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: runcmd <execution-root> <frozen-root> <repo-root>")
		os.Exit(2)
	}
	err := errors.New("predecessor location-intersection-v5-qualification-2026-10-07 run command disabled; use successor root in simulate-only mode unless an independent committed gate exists")
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
