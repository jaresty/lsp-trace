package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"lsp-trace/internal/providerconformance"
)

const providerConformanceUsage = "usage: lsp-trace provider conformance --executable ABSOLUTE_PATH --input PATH|- [--arg VALUE...]"

func runProviderConformance(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("provider conformance", flag.ContinueOnError)
	fs.SetOutput(stderr)
	executable := fs.String("executable", "", "host-supplied absolute provider executable")
	input := fs.String("input", "", "generic conformance request JSON path or -")
	var providerArgs stringsFlag
	fs.Var(&providerArgs, "arg", "provider argument (repeatable)")
	wall := fs.Duration("wall-time", 5*time.Second, "wall time bound")
	grace := fs.Duration("termination-grace", 100*time.Millisecond, "termination grace bound")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || *executable == "" || *input == "" {
		if err == nil {
			fmt.Fprintln(stderr, providerConformanceUsage)
		}
		return 1
	}
	var data []byte
	var err error
	if *input == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(*input)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var req providerconformance.Request
	d := json.NewDecoder(bytesReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&req); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = d.Decode(&struct{}{}); err != io.EOF {
		fmt.Fprintln(stderr, "exactly one request JSON value required")
		return 1
	}
	req.Limits.WallTime = *wall
	req.Limits.TerminationGrace = *grace
	cfg := providerconformance.Config{Executable: *executable, Arguments: providerArgs, Provider: req.Provider, Protocol: req.Protocol, Capabilities: req.Capabilities, Limits: req.Limits}
	report := providerconformance.Run(context.Background(), cfg, req)
	encoded, err := json.Marshal(report)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	if report.Outcome == providerconformance.OutcomePassed || report.Outcome == providerconformance.OutcomeEmpty {
		return 0
	}
	if report.Outcome == providerconformance.OutcomeUnsupported || report.Outcome == providerconformance.OutcomeUnavailable || report.Outcome == providerconformance.OutcomePartial {
		return 2
	}
	return 1
}

type byteReader struct {
	data []byte
	off  int
}

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }
func (r *byteReader) Read(p []byte) (int, error) {
	if r.off == len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}
