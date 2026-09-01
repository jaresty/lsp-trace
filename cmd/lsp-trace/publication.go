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

const (
	generationArtifactName = "artifact.json"
	generationReceiptName  = "receipt.json"
)

var (
	openPublicationDirectory = os.Open
	syncPublicationDirectory = func(dir *os.File) error { return dir.Sync() }
)

func syncPublishedDirectory(path string) error {
	dir, err := openPublicationDirectory(path)
	if err != nil {
		return fmt.Errorf("open publication directory %q: %w", path, err)
	}
	if err := syncPublicationDirectory(dir); err != nil {
		_ = dir.Close()
		return fmt.Errorf("sync publication directory %q: %w", path, err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close publication directory %q: %w", path, err)
	}
	return nil
}

type generationSelector struct {
	Generation string `json:"generation"`
}

func readGenerationSelector(path string) (generationSelector, error) {
	var selector generationSelector
	data, err := os.ReadFile(path)
	if err != nil {
		return selector, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selector); err != nil {
		return selector, fmt.Errorf("malformed generation selector: %w", err)
	}
	if selector.Generation == "" || filepath.Base(selector.Generation) != selector.Generation {
		return selector, fmt.Errorf("malformed generation selector: invalid generation")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return selector, fmt.Errorf("malformed generation selector: trailing JSON content")
	}
	return selector, nil
}

type publicationFailureRecord struct {
	Version                    string `json:"version"`
	Target                     string `json:"target"`
	Error                      string `json:"error"`
	ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
}

func retainPublicationFailure(path string, data []byte, publishErr error) (string, error) {
	dir := filepath.Dir(path)
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no existing ancestor for publication failure record")
		}
		dir = parent
	}
	record, err := json.Marshal(publicationFailureRecord{
		Version:                    "lsp-trace.publication-failure.v1",
		Target:                     path,
		Error:                      publishErr.Error(),
		ExactSerializedBytesDigest: graph.ExactBytesDigest(data),
	})
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".publication-failure-*.json")
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
	if err := f.Chmod(0600); err != nil {
		return "", err
	}
	if _, err := f.Write(append(record, '\n')); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := syncPublishedDirectory(dir); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}

type custodyReceipt struct {
	ReceiptVersion             string `json:"receipt_version"`
	ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
	DigestAlgorithm            string `json:"digest_algorithm"`
	DigestScope                string `json:"digest_scope"`
	IntegrityClaim             string `json:"integrity_claim"`
	AuthenticityClaim          bool   `json:"authenticity_claim"`
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
	return syncPublishedDirectory(dir)
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
	generationDir, err := os.MkdirTemp(dir, ".lsp-trace-generation-*")
	if err != nil {
		return err
	}
	selected := false
	defer func() {
		if !selected {
			_ = os.RemoveAll(generationDir)
		}
	}()
	if err := os.Chmod(generationDir, 0700); err != nil {
		return err
	}
	artifactPath := filepath.Join(generationDir, generationArtifactName)
	receiptPath := filepath.Join(generationDir, generationReceiptName)
	if err := writePrivateSyncedFile(artifactPath, data); err != nil {
		return err
	}
	if err := writePrivateSyncedFile(receiptPath, receipt); err != nil {
		return err
	}
	staged, err := os.ReadFile(artifactPath)
	if err != nil || !bytes.Equal(staged, data) {
		return fmt.Errorf("staged artifact byte validation failed: %v", err)
	}
	if err := syncPublishedDirectory(generationDir); err != nil {
		return err
	}
	selectorData, err := json.Marshal(generationSelector{Generation: filepath.Base(generationDir)})
	if err != nil {
		return err
	}
	selectorTemp, err := stage(dir, ".lsp-trace-selector-*", append(selectorData, '\n'))
	if err != nil {
		return err
	}
	defer os.Remove(selectorTemp)
	if err := os.Rename(selectorTemp, path); err != nil {
		return err
	}
	selected = true
	_ = os.Remove(path + ".receipt.json")
	return syncPublishedDirectory(dir)
}

func writePrivateSyncedFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
func runVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace verify PATH")
		return 1
	}
	path := args[0]
	selector, err := readGenerationSelector(path)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 1
	}
	generationDir := filepath.Join(filepath.Dir(path), selector.Generation)
	data, err := os.ReadFile(filepath.Join(generationDir, generationArtifactName))
	if err != nil {
		fmt.Fprintf(stderr, "verify: incomplete selected generation: %v\n", err)
		return 1
	}
	receiptData, err := os.ReadFile(filepath.Join(generationDir, generationReceiptName))
	if err != nil {
		fmt.Fprintf(stderr, "verify receipt: incomplete selected generation: %v\n", err)
		return 1
	}
	var receipt custodyReceipt
	decoder := json.NewDecoder(bytes.NewReader(receiptData))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&receipt); err != nil {
		fmt.Fprintf(stderr, "verify receipt: malformed: %v\n", err)
		return 1
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		fmt.Fprintln(stderr, "verify receipt: malformed: trailing JSON content")
		return 1
	}
	if receipt.ReceiptVersion != "lsp-trace.byte-custody-receipt.v1" || receipt.DigestScope != graph.ByteDigestScope || receipt.DigestAlgorithm != "SHA-256" || receipt.IntegrityClaim != "INTEGRITY_AND_CUSTODY_ONLY" || receipt.AuthenticityClaim {
		fmt.Fprintln(stderr, "verify receipt: receipt metadata mismatch")
		return 1
	}
	if receipt.ExactSerializedBytesDigest != graph.ExactBytesDigest(data) {
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
