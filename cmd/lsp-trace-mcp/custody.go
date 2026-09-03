package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/verification"
)

const (
	generationArtifactName = "artifact.json"
	generationReceiptName  = "receipt.json"
)

// commandCustodyLoader adapts the CLI selector-file custody contract without
// granting filesystem authority to the transport-neutral verify handler.
type commandCustodyLoader struct{}

func (commandCustodyLoader) Load(_ context.Context, input json.RawMessage) (operation.CustodyMaterial, *operation.Failure) {
	var selectorPath string
	if err := json.Unmarshal(input, &selectorPath); err != nil || selectorPath == "" {
		if err == nil {
			err = fmt.Errorf("verification selector path is required")
		}
		return custodyFailure(err)
	}
	material, err := loadCustodyMaterial(selectorPath)
	if err != nil {
		return custodyFailure(err)
	}
	return material, nil
}

func custodyFailure(err error) (operation.CustodyMaterial, *operation.Failure) {
	return operation.CustodyMaterial{}, &operation.Failure{
		Code: operation.FailureInvalidInput, Diagnostics: []string{err.Error()}, Err: err,
	}
}

func loadCustodyMaterial(path string) (operation.CustodyMaterial, error) {
	parentPath, finalName := filepath.Dir(path), filepath.Base(path)
	if finalName == "." || finalName == string(filepath.Separator) || finalName == "" {
		return operation.CustodyMaterial{}, fmt.Errorf("invalid custody selector")
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return operation.CustodyMaterial{}, err
	}
	defer parent.Close()
	selectorBytes, err := readRootRegularNoFollow(parent, finalName)
	if err != nil {
		return operation.CustodyMaterial{}, err
	}
	selector, err := verification.DecodeSelector(selectorBytes)
	if err != nil {
		return operation.CustodyMaterial{}, err
	}
	generation, err := openRootDirectoryNoFollow(parent, selector.Generation)
	if err != nil {
		return operation.CustodyMaterial{}, fmt.Errorf("incomplete selected generation: %w", err)
	}
	defer generation.Close()
	artifact, err := readRootRegularNoFollow(generation, generationArtifactName)
	if err != nil {
		return operation.CustodyMaterial{}, fmt.Errorf("incomplete selected generation: %w", err)
	}
	receipt, err := readRootRegularNoFollow(generation, generationReceiptName)
	if err != nil {
		return operation.CustodyMaterial{}, fmt.Errorf("incomplete selected generation: %w", err)
	}
	return operation.CustodyMaterial{Artifact: artifact, Receipt: receipt}, nil
}

func readRootRegularNoFollow(root *os.Root, name string) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("no-follow regular file required")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("file identity changed")
	}
	return io.ReadAll(file)
}

func openRootDirectoryNoFollow(root *os.Root, name string) (*os.Root, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("no-follow directory required")
	}
	child, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		child.Close()
		return nil, fmt.Errorf("directory identity changed")
	}
	return child, nil
}
