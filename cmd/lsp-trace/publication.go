package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lsp-trace/internal/graph"
)

type custodyReceipt struct {
	ReceiptVersion    string `json:"receipt_version"`
	Digest            string `json:"digest"`
	DigestAlgorithm   string `json:"digest_algorithm"`
	DigestScope       string `json:"digest_scope"`
	IntegrityClaim    string `json:"integrity_claim"`
	AuthenticityClaim bool   `json:"authenticity_claim"`
}

func marshalResult(result graph.Result, pretty bool) ([]byte, error) {
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(result, "", "  ")
	} else {
		b, err = json.Marshal(result)
	}
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func receiptBytes(data []byte) ([]byte, error) {
	r := custodyReceipt{"lsp-trace.byte-custody-receipt.v1", graph.ExactBytesDigest(data), "SHA-256", graph.ByteDigestScope, "INTEGRITY_AND_CUSTODY_ONLY", false}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func stage(dir, pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err = io.Copy(f, bytes.NewReader(data)); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}
func publishArtifact(path string, data []byte) error {
	dir := filepath.Dir(path)
	artifactTemp, err := stage(dir, ".lsp-trace-artifact-*", data)
	if err != nil {
		return err
	}
	defer os.Remove(artifactTemp)
	staged, err := os.ReadFile(artifactTemp)
	if err != nil || !bytes.Equal(staged, data) {
		return fmt.Errorf("staged artifact byte validation failed: %v", err)
	}
	if err = os.Rename(artifactTemp, path); err != nil {
		return err
	}
	if err = os.Remove(path + ".receipt.json"); err != nil && !os.IsNotExist(err) {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func publishBundle(path string, data []byte) error {
	if err := graph.ValidateSemanticBundle(bytes.TrimSpace(data)); err != nil {
		return fmt.Errorf("staged semantic validation: %w", err)
	}
	receipt, err := receiptBytes(data)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	artifactTemp, err := stage(dir, ".lsp-trace-artifact-*", data)
	if err != nil {
		return err
	}
	defer os.Remove(artifactTemp)
	receiptTemp, err := stage(dir, ".lsp-trace-receipt-*", receipt)
	if err != nil {
		return err
	}
	defer os.Remove(receiptTemp)
	staged, err := os.ReadFile(artifactTemp)
	if err != nil || !bytes.Equal(staged, data) {
		return fmt.Errorf("staged artifact byte validation failed: %v", err)
	}
	if err = os.Chmod(artifactTemp, 0600); err != nil {
		return err
	}
	if err = os.Chmod(receiptTemp, 0600); err != nil {
		return err
	}
	// Portable filesystems cannot atomically replace two names together. Replacing the
	// artifact first and receipt second minimizes exposure; verifiers reject either
	// transitional mismatch rather than accepting an inconsistent pair.
	if err = os.Rename(artifactTemp, path); err != nil {
		return err
	}
	receiptPath := path + ".receipt.json"
	if err = os.Rename(receiptTemp, receiptPath); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
func runVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace verify PATH")
		return 1
	}
	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 1
	}
	receiptData, err := os.ReadFile(path + ".receipt.json")
	if err != nil {
		fmt.Fprintf(stderr, "verify receipt: %v\n", err)
		return 1
	}
	var receipt custodyReceipt
	decoder := json.NewDecoder(bytes.NewReader(receiptData))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&receipt); err != nil {
		fmt.Fprintf(stderr, "verify receipt: malformed: %v\n", err)
		return 1
	}
	if receipt.ReceiptVersion != "lsp-trace.byte-custody-receipt.v1" || receipt.DigestScope != graph.ByteDigestScope || receipt.DigestAlgorithm != "SHA-256" || receipt.IntegrityClaim != "INTEGRITY_AND_CUSTODY_ONLY" || receipt.AuthenticityClaim {
		fmt.Fprintln(stderr, "verify receipt: receipt metadata mismatch")
		return 1
	}
	if receipt.Digest != graph.ExactBytesDigest(data) {
		fmt.Fprintln(stderr, "verify receipt: exact-byte integrity mismatch")
		return 1
	}
	if err = graph.ValidateSemanticBundle(bytes.TrimSpace(data)); err != nil {
		fmt.Fprintf(stderr, "verify semantic receipt: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "verified integrity and custody")
	return 0
}
