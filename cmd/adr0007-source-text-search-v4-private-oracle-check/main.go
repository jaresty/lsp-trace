package main

import (
	"bytes"
	"fmt"
	oracle "lsp-trace/internal/adr0007sourcetextsearchv4oracle"
	validator "lsp-trace/internal/adr0007v4contractvalidator"
	"os"
	"path/filepath"
)

func main() {
	root := oracle.CasesDir
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := oracle.Check(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := compareOracleToDerive(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("oracle verified: expected=50 derive_matches=50")
}

func compareOracleToDerive(root string) error {
	rows, err := oracle.ReadRows(root)
	if err != nil {
		return err
	}
	if len(rows) != 50 {
		return fmt.Errorf("matrix count %d, want 50", len(rows))
	}
	matches := 0
	for _, row := range rows {
		res, err := oracle.EvaluateFileResult(filepath.Join(root, row.ID, "attempt.json"))
		if err != nil {
			return fmt.Errorf("%s oracle: %w", row.ID, err)
		}
		terminalBytes, err := oracle.Canonical(res.Terminal)
		if err != nil {
			return fmt.Errorf("%s oracle canonical: %w", row.ID, err)
		}
		if res.Terminal.Terminal == "COMPLETE" {
			bundle := validator.Bundle{SchemaBytes: res.SchemaBytes, RawAttemptBytes: res.RawAttemptBytes, TerminalBytes: string(terminalBytes), AdmittedSourceBytes: res.AdmittedSourceBytes, AdmittedBindingBytes: res.AdmittedBindingBytes, ToolingManifestBytes: res.ToolingManifestBytes, PredecessorManifestBytes: res.PredecessorBytes, PayloadFreezeBytes: res.PayloadFreezeBytes}
			if err := validator.ValidateBundle(bundle); err != nil {
				return fmt.Errorf("%s validator rejected oracle bundle: %w", row.ID, err)
			}
		}
		sourceBytes := res.AdmittedSourceBytes
		if len(sourceBytes) == 0 {
			sourceBytes = nil
		}
		derived, err := validator.Derive(validator.DeriveInput{RawAttemptBytes: res.RawAttemptBytes, AdmittedSourceBytes: sourceBytes, AdmittedBindingBytes: res.AdmittedBindingBytes, ToolingManifestBytes: res.ToolingManifestBytes, PredecessorManifestBytes: res.PredecessorBytes, PayloadFreezeBytes: res.PayloadFreezeBytes})
		if err != nil {
			return fmt.Errorf("%s derive: %w", row.ID, err)
		}
		if !bytes.Equal(terminalBytes, []byte(derived)) {
			return fmt.Errorf("%s oracle-vs-Derive mismatch category=terminal_bytes", row.ID)
		}
		matches++
	}
	if matches != 50 {
		return fmt.Errorf("derive match count %d, want 50", matches)
	}
	return nil
}
