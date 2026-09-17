package operation

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/artifactingress"
	"lsp-trace/internal/hydratedinspection"
	"lsp-trace/internal/publication"
)

const InspectHydrated Name = "inspect_hydrated"

// InspectHydratedHandler accepts inline JSON or immutable artifacts admitted from
// process-pinned roots. It never accepts a host path from MCP input.
func InspectHydratedHandler(_ context.Context, request Request) (Result, *Failure) {
	probe := hydratedinspection.DefaultRequest()
	if err := json.Unmarshal(request.Input, &probe); err != nil {
		return Result{}, invalidHydrated()
	}
	if probe.PublicationSelector == nil && probe.ContentAddressedArtifact == nil {
		r, err := hydratedinspection.Decode(request.Input)
		if err != nil {
			return Result{}, invalidHydrated()
		}
		return inspectHydrated(r, false)
	}
	return InspectHydratedLegacyDecoded(request, probe)
}

// InspectHydratedLegacyDecoded preserves the admitted legacy path for callers
// that have already decoded the V1/V2 union exactly once.
func InspectHydratedLegacyDecoded(request Request, probe hydratedinspection.Request) (Result, *Failure) {
	if probe.PublicationSelector == nil && probe.ContentAddressedArtifact == nil {
		return inspectHydrated(probe, false)
	}
	config := artifactingress.Config{
		MaxBytes:        artifactingress.HydrationCoreMaxBytes,
		PublicationRoot: request.PublicationRoot,
		ArtifactStore:   request.ArtifactStore,
		Validate:        ValidateHydratedIngress,
	}
	var admitted artifactingress.Result
	var err error
	if source := probe.PublicationSelector; source != nil {
		receipt := publication.Receipt{Digest: source.ArtifactDigest, ByteLength: source.ArtifactByteLength, ArtifactSchemaID: source.ArtifactSchemaID, PublicationMechanism: source.PublicationMechanism, Generation: source.Generation, VerificationSelector: source.VerificationSelector}
		admitted, err = config.FromSelector(artifactingress.SelectorRequest{Selector: source.Selector, Receipt: receipt, Expected: artifactingress.Expected{SchemaID: source.ArtifactSchemaID, ByteLength: source.ArtifactByteLength, Generation: source.Generation}})
	} else {
		source := probe.ContentAddressedArtifact
		admitted, err = config.FromContent(artifactingress.ContentRequest{ID: source.ID, Expected: artifactingress.Expected{SchemaID: source.ArtifactSchemaID, ByteLength: source.ArtifactByteLength, Generation: source.Generation}})
	}
	if err != nil {
		return Result{}, &Failure{Code: string(ingressCode(err)), Err: errors.New("hydrated artifact admission rejected")}
	}
	probe.Input = string(admitted.Bytes)
	probe.PublicationSelector = nil
	probe.ContentAddressedArtifact = nil
	if err := hydratedinspection.CheckAdmitted(probe); err != nil {
		return Result{}, invalidHydrated()
	}
	return inspectHydrated(probe, true)
}

func inspectHydrated(r hydratedinspection.Request, admitted bool) (Result, *Failure) {
	var v hydratedinspection.View
	var err error
	if admitted {
		v, err = hydratedinspection.InspectAdmitted(r)
	} else {
		v, err = hydratedinspection.Inspect(r)
	}
	if err != nil {
		return Result{}, &Failure{Code: "INPUT_INVALID", Err: err}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return Result{}, &Failure{Code: FailureInternal, Err: err}
	}
	return Result{Value: v, Artifact: append(raw, '\n')}, nil
}

func invalidHydrated() *Failure {
	return &Failure{Code: FailureInvalidInput, Err: errors.New("invalid hydrated inspection request")}
}

func ingressCode(err error) artifactingress.FailureCode {
	var failure *artifactingress.Failure
	if errors.As(err, &failure) {
		return failure.Code
	}
	return artifactingress.CustodyFailed
}

// ValidateHydratedIngress binds a declared schema identity to validated hydrated bytes.
func ValidateHydratedIngress(schemaID string, raw []byte) error {
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if json.Unmarshal(raw, &header) != nil || header.SchemaVersion == "" {
		return errors.New("artifact schema mismatch")
	}
	want := "https://jaresty.github.io/lsp-trace/schemas/" + header.SchemaVersion + ".schema.json"
	if schemaID != want {
		return errors.New("artifact schema mismatch")
	}
	r := hydratedinspection.DefaultRequest()
	r.Input = string(raw)
	r.NodeIDs = []string{"__schema_validation__"}
	_, err := hydratedinspection.InspectAdmitted(r)
	return err
}
