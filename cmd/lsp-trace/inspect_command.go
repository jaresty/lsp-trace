package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/ancillaryinspection"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/inspection"
	"lsp-trace/internal/schema"
)

// Compatibility aliases keep adjacent CLI adapters source-stable while projection
// semantics live in the importable inspection package.
type inspectSummary = inspection.Summary
type inspectBundle = inspection.Bundle
type inspectProjection = inspection.Projection
type inspectAllSeed = inspection.AllSeed
type inspectRecords = inspection.Records
type inspectAccounting = inspection.Accounting
type inspectAllProjection = inspection.AllProjection

func projectSeedInspection(data []byte, label string) (inspectProjection, error) {
	return inspection.ProjectSeed(data, label)
}

func projectAllSeedInspection(data []byte) (inspectAllProjection, error) {
	return inspection.ProjectAllSeeds(data)
}

func validateAllSeedAccounting(projection inspectAllProjection) error {
	return inspection.ValidateAllSeedAccounting(projection)
}

const inspectUsage = "usage: lsp-trace inspect SELECTOR_OR_ARTIFACT (--seed LABEL | --all-seeds) [--json]\n       lsp-trace inspect SELECTOR_OR_ARTIFACT --all-seeds --ancillary [--page] [--cursor TOKEN] --json\n       lsp-trace inspect ARTIFACT --hydrated [--node ID | --relation ID | --sibling-relation ID] [options]"

func runInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, inspectUsage)
		fs.PrintDefaults()
	}
	if len(args) == 0 {
		fs.Usage()
		return 1
	}
	if args[0] == "-h" || args[0] == "--help" {
		fs.Usage()
		return 0
	}
	input := args[0]
	seedLabel := fs.String("seed", "", "existing seed label")
	allSeeds := fs.Bool("all-seeds", false, "inspect every stored seed")
	ancillary := fs.Bool("ancillary", false, "emit lsp-trace.inspect-ancillary.v1 delivery")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	hydrated := addHydratedFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	legacyVisited, hydratedVisited := false, false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" || f.Name == "all-seeds" {
			legacyVisited = true
		}
		if hydratedFlag(f.Name) {
			hydratedVisited = true
		}
	})
	if hydrated.enabled {
		if legacyVisited || *ancillary || fs.NArg() != 0 {
			fmt.Fprintln(stderr, "inspect: INVALID_INPUT: hydrated and legacy selectors are incompatible")
			return 1
		}
		return runInspectHydrated(input, hydrated, *jsonOutput, stdout, stderr)
	}
	if *ancillary {
		if !*allSeeds || *seedLabel != "" || fs.NArg() != 0 {
			fmt.Fprintln(stderr, inspectUsage)
			return 1
		}
		if hydrated.request.Page && !*jsonOutput {
			fmt.Fprintln(stderr, "inspect ancillary: INVALID_INPUT: --page requires --json")
			return 1
		}
		data, err := loadInspectArtifact(input)
		if err != nil {
			fmt.Fprintf(stderr, "inspect ancillary: %v\n", err)
			return 1
		}
		raw, _ := json.Marshal(data)
		r := ancillaryinspection.Request{Input: raw, Ancillary: true, Page: hydrated.request.Page, Cursor: hydrated.request.Cursor, Policy: ancillaryinspection.Policy{MaxPageBytes: hydrated.request.CorePolicy.MaxPageBytes, MaxPages: hydrated.request.CorePolicy.MaxPages, MaxOutputBytes: hydrated.request.CorePolicy.MaxOutputBytes}}
		r.Selector.AllSeeds = true
		view, err := ancillaryinspection.Inspect(r)
		if err != nil {
			fmt.Fprintf(stderr, "inspect ancillary: %v\n", err)
			return 1
		}
		encoded, _ := json.Marshal(view)
		_, err = stdout.Write(append(encoded, '\n'))
		if err != nil {
			fmt.Fprintf(stderr, "inspect ancillary: %v\n", err)
			return 1
		}
		return 0
	}
	if hydratedVisited {
		fmt.Fprintln(stderr, "inspect: INVALID_INPUT: focused options require --hydrated or --ancillary")
		return 1
	}
	if (*seedLabel == "") == !*allSeeds || fs.NArg() != 0 {
		fmt.Fprintln(stderr, inspectUsage)
		return 1
	}
	data, err := loadInspectArtifact(input)
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	var projection any
	if *allSeeds {
		projection, err = inspection.ProjectAllSeeds(data)
	} else {
		projection, err = inspection.ProjectSeed(data, *seedLabel)
	}
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	if err := schema.ValidateInspection(encoded); err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	encoded = append(encoded, '\n')
	if _, err := stdout.Write(encoded); err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	return 0
}

func loadInspectArtifact(path string) ([]byte, error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(input, &header); err != nil {
		return nil, fmt.Errorf("malformed input: %w", err)
	}
	if header.SchemaVersion != "" {
		version := ""
		switch header.SchemaVersion {
		case graph.SchemaVersionV3:
			version = "v3"
		case graph.SchemaVersionV5:
			version = "v5"
		default:
			return nil, fmt.Errorf("inspection requires %s or %s", graph.SchemaVersionV3, graph.SchemaVersionV5)
		}
		if _, err := schema.Validate(input, version); err != nil {
			return nil, err
		}
		return input, nil
	}

	artifact, _, err := loadCustodiedGeneration(path)
	if err != nil {
		return nil, err
	}
	if _, err := schema.Validate(artifact, "v3"); err != nil {
		return nil, err
	}
	return artifact, nil
}
