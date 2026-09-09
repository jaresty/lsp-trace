package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lsp-trace/internal/seedbinding"
)

type localSeedValidationReceipt struct {
	SchemaVersion string                  `json:"schema_version"`
	Status        seedbinding.Status      `json:"status"`
	Code          string                  `json:"code"`
	Provenance    seedbinding.CustodyMode `json:"provenance"`
	Authenticated bool                    `json:"authenticated"`
}

func runSeedCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "validate-local" {
		fmt.Fprintln(stderr, "usage: lsp-trace-mcp seed validate-local --workspace ABS --seed-manifest ABS")
		return 2
	}
	fs := flag.NewFlagSet("seed validate-local", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "absolute workspace root")
	manifestPath := fs.String("seed-manifest", "", "absolute v3 seed manifest path")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 || !filepath.IsAbs(*workspace) || !filepath.IsAbs(*manifestPath) {
		fmt.Fprintln(stderr, "seed validate-local requires absolute --workspace and --seed-manifest")
		return 2
	}
	f, err := os.Open(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, seedbinding.LocatorInvalid)
		return 1
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() {
		fmt.Fprintln(stderr, seedbinding.LocatorInvalid)
		return 1
	}
	raw, err := io.ReadAll(io.LimitReader(f, seedbinding.MaxManifestBytes+1))
	after, statErr := f.Stat()
	if err != nil || statErr != nil || len(raw) > seedbinding.MaxManifestBytes || !os.SameFile(before, after) || before.Size() != after.Size() || after.Size() != int64(len(raw)) || !before.ModTime().Equal(after.ModTime()) {
		fmt.Fprintln(stderr, seedbinding.LocatorInvalid)
		return 1
	}
	manifest, err := seedbinding.DecodeV3(raw)
	if err != nil || manifest.CustodyMode != seedbinding.CallerAssertedLocal {
		fmt.Fprintln(stderr, seedbinding.LocatorInvalid)
		return 1
	}
	canonical, err := json.Marshal(manifest)
	if err != nil || !bytes.Equal(canonical, raw) {
		fmt.Fprintln(stderr, seedbinding.LocatorInvalid)
		return 1
	}
	outcome := seedbinding.ValidateMechanical(context.Background(), *workspace, manifest, nil)
	receipt := localSeedValidationReceipt{
		SchemaVersion: "lsp-trace.seed-local-validation-receipt.v1", Status: outcome.Status,
		Code: outcome.Terminal, Provenance: outcome.Provenance, Authenticated: false,
	}
	if err := json.NewEncoder(stdout).Encode(receipt); err != nil {
		fmt.Fprintln(stderr, seedbinding.BindingUnavailable)
		return 1
	}
	if outcome.Status != seedbinding.Match {
		return 1
	}
	return 0
}
