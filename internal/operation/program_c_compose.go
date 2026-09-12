package operation

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/programccompose"
)

type programCComposeInput struct {
	Captures []struct {
		Input         string `json:"input"`
		Identity      string `json:"identity"`
		SHA256        string `json:"sha256"`
		ByteLength    int    `json:"byte_length"`
		ExactMetadata struct {
			WorkspaceIdentity    string `json:"workspace_identity"`
			RevisionCustody      string `json:"revision_custody"`
			PositionEncoding     string `json:"position_encoding"`
			AcquisitionSemantics string `json:"acquisition_semantics"`
			PrivacyPolicy        string `json:"privacy_policy"`
		} `json:"exact_metadata"`
	} `json:"captures"`
}

// ProgramCComposeHandler exposes deterministic composition without promoting the
// result to native single-capture custody or admitting it to Leiden.
func ProgramCComposeHandler(_ context.Context, request Request) (Result, *Failure) {
	var input programCComposeInput
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	captures := make([]programccompose.Input, len(input.Captures))
	for i, c := range input.Captures {
		captures[i] = programccompose.Input{Bytes: []byte(c.Input), Identity: c.Identity, SHA256: c.SHA256, ByteLength: c.ByteLength, ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: c.ExactMetadata.WorkspaceIdentity, RevisionCustody: c.ExactMetadata.RevisionCustody, PositionEncoding: c.ExactMetadata.PositionEncoding, AcquisitionSemantics: c.ExactMetadata.AcquisitionSemantics, PrivacyPolicy: c.ExactMetadata.PrivacyPolicy}}
	}
	result, err := programccompose.Compose(captures)
	if err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	return Result{Artifact: result.Bytes, LogicalDigest: result.Artifact.OutputSHA256}, nil
}
