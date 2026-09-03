package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

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

const inspectUsage = "usage: lsp-trace inspect SELECTOR_OR_ARTIFACT (--seed LABEL | --all-seeds) [--json]"

func runInspect(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, inspectUsage)
		return 1
	}
	input := args[0]
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	seedLabel := fs.String("seed", "", "existing seed label")
	allSeeds := fs.Bool("all-seeds", false, "inspect every stored seed")
	_ = fs.Bool("json", false, "emit JSON (currently the only format)")
	if err := fs.Parse(args[1:]); err != nil {
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
		if header.SchemaVersion != graph.SchemaVersionV3 {
			return nil, fmt.Errorf("inspection requires %s", graph.SchemaVersionV3)
		}
		if _, err := schema.Validate(input, "v3"); err != nil {
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
