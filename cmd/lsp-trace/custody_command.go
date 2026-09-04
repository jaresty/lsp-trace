package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"lsp-trace/internal/operation"
)

const custodySchemaVersion = "lsp-trace.custody.v1"

type custodyHandler = operation.Handler
type selectorReader func(string) (generationSelector, error)

type cliCustodyLoader struct{}

func (cliCustodyLoader) Load(_ context.Context, input json.RawMessage) (operation.CustodyMaterial, *operation.Failure) {
	var path string
	if err := json.Unmarshal(input, &path); err != nil || path == "" {
		if err == nil {
			err = fmt.Errorf("custody selector path is required")
		}
		return operation.CustodyMaterial{}, &operation.Failure{Code: operation.FailureInvalidInput, Diagnostics: []string{err.Error()}, Err: err}
	}
	parent, finalName, err := openPinnedPublicationParent(path)
	if err != nil {
		return custodyUnavailable(err)
	}
	defer parent.Close()
	selectorBytes, err := readRootRegularNoFollow(parent, finalName)
	if err != nil {
		return custodyUnavailable(err)
	}
	selector, err := decodeGenerationSelector(selectorBytes)
	if err != nil {
		return custodyUnavailable(err)
	}
	generation, err := openRootDirectoryNoFollow(parent, selector.Generation)
	if err != nil {
		return custodyUnavailable(err)
	}
	defer generation.Close()
	artifact, err := readRootRegularNoFollow(generation, generationArtifactName)
	if err != nil {
		return custodyUnavailable(err)
	}
	receipt, err := readRootRegularNoFollow(generation, generationReceiptName)
	if err != nil {
		return custodyUnavailable(err)
	}
	return operation.CustodyMaterial{Artifact: artifact, Receipt: receipt}, nil
}

func custodyUnavailable(err error) (operation.CustodyMaterial, *operation.Failure) {
	return operation.CustodyMaterial{}, &operation.Failure{Code: "CUSTODY_UNAVAILABLE", Diagnostics: []string{"selected custody generation is unavailable"}, Err: err}
}

type custodySuccess struct {
	SchemaVersion string `json:"schema_version"`
	State         string `json:"state"`
	Selector      string `json:"selector"`
	Generation    string `json:"generation"`
	Artifact      string `json:"artifact"`
}

type custodyError struct {
	SchemaVersion string   `json:"schema_version"`
	State         string   `json:"state"`
	Code          string   `json:"code"`
	Diagnostics   []string `json:"diagnostics"`
}

func runCustody(args []string, stdout, stderr io.Writer) int {
	return runCustodyWithHandler(args, stdout, stderr, operation.NewVerifyHandler(cliCustodyLoader{}), readGenerationSelector)
}

func runCustodyWithHandler(args []string, stdout, stderr io.Writer, handler custodyHandler, readSelector selectorReader) int {
	if len(args) != 1 {
		return writeCustodyJSON(stdout, stderr, custodyError{custodySchemaVersion, "error", operation.FailureInvalidInput, []string{"usage: lsp-trace custody SELECTOR"}}, 1)
	}
	selectorPath := args[0]
	requestInput, err := json.Marshal(struct {
		Input string `json:"input"`
	}{selectorPath})
	if err != nil {
		return writeCustodyJSON(stdout, stderr, custodyError{custodySchemaVersion, "error", operation.FailureInternal, []string{"encode custody request"}}, 1)
	}
	_, failure := handler(context.Background(), operation.Request{Name: operation.Verify, Input: requestInput})
	if failure != nil {
		normalized := operation.NormalizeFailure(failure)
		state, code := "error", 1
		if normalized.Code == "CUSTODY_UNAVAILABLE" {
			state, code = "unavailable", 2
		}
		return writeCustodyJSON(stdout, stderr, custodyError{custodySchemaVersion, state, normalized.Code, normalized.Diagnostics}, code)
	}
	selector, err := readSelector(selectorPath)
	if err != nil {
		return writeCustodyJSON(stdout, stderr, custodyError{custodySchemaVersion, "unavailable", "CUSTODY_UNAVAILABLE", []string{"selected custody generation is unavailable"}}, 2)
	}
	generation := filepath.Join(filepath.Dir(selectorPath), selector.Generation)
	return writeCustodyJSON(stdout, stderr, custodySuccess{custodySchemaVersion, "available", selectorPath, generation, filepath.Join(generation, generationArtifactName)}, 0)
}

func writeCustodyJSON(stdout, stderr io.Writer, value any, code int) int {
	encoded, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintln(stderr, "custody: encode response")
		return 1
	}
	encoded = append(encoded, '\n')
	written, err := stdout.Write(encoded)
	if err != nil || written != len(encoded) {
		fmt.Fprintln(stderr, "custody: write response")
		return 1
	}
	return code
}
