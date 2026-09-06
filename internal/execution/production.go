package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
	"lsp-trace/internal/verification"
)

const ExecutionSchemaVersion = "lsp-trace.execution.v1"

type ProductionInput struct {
	Root        string            `json:"root"`
	Source      string            `json:"source,omitempty"`
	FailAt      string            `json:"fail_at,omitempty"`
	Operational *OperationalInput `json:"operational,omitempty"`
}

type ProductionArtifact struct {
	SchemaVersion string                    `json:"schema_version"`
	Operation     operation.Name            `json:"operation"`
	Response      operation.CustodyResponse `json:"response"`
	Artifact      string                    `json:"artifact,omitempty"`
	Receipt       string                    `json:"receipt,omitempty"`
	Operational   *custodyevidence.Evidence `json:"operational,omitempty"`
}

type ProductionExecutor struct {
	trust *custodyevidence.HostTrustStore
}

func NewProductionExecutor() operation.Executor { return ProductionExecutor{} }

func (executor ProductionExecutor) Execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	if request.Name != operation.CustodyExecute {
		return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
	}
	var input ProductionInput
	if err := decodeProductionInput(request.Input, &input); err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Diagnostics: []string{err.Error()}, Err: err}
	}
	root, err := publication.OpenRoot(input.Root)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Diagnostics: []string{err.Error()}, Err: err}
	}
	defer root.Close()

	handlers := productionHandlers(input, root)
	var evidence *custodyevidence.Evidence
	if input.Operational != nil {
		evidence = &custodyevidence.Evidence{}
		handlers = operationalHandlers(input, root, executor.trust, evidence)
	}
	custody, err := operation.NewCustodyOperation(handlers)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: err}
	}
	logical, _ := json.Marshal(struct {
		Source string `json:"source"`
	}{input.Source})
	if input.Operational != nil {
		logical, _ = json.Marshal(input.Operational)
	}
	response, custodyFailure := custody.ExecuteCustody(ctx, operation.CustodyRequest{OperationID: request.RequestID, Input: logical})
	artifact := ProductionArtifact{SchemaVersion: ExecutionSchemaVersion, Operation: operation.CustodyExecute, Response: response, Operational: evidence}
	if custodyFailure != nil {
		diagnostics := []string{string(custodyFailure.Stage)}
		// Error transports historically retain diagnostics, not Result.Value.
		// Preserve the operational artifact there without publishing a rejected
		// authenticated-required operation. Early failures may be pre-identity.
		if evidence != nil {
			retained, _ := json.Marshal(artifact)
			diagnostics = append(diagnostics, "operational_failure_evidence:"+string(retained))
		}
		return operation.Result{}, &operation.Failure{Code: custodyFailure.Code, Diagnostics: diagnostics, Err: custodyFailure}
	}
	artifact.Artifact = input.Root + "/artifact.json"
	artifact.Receipt = input.Root + "/receipt.json"
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: err}
	}
	return operation.Result{Value: artifact, Artifact: encoded, LogicalDigest: "sha256:" + response.LogicalDigest}, nil
}

func decodeProductionInput(raw []byte, out *ProductionInput) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return fmt.Errorf("invalid production execution request: %w", err)
	}
	if wrapped, ok := members["request"]; ok {
		if len(members) != 1 {
			return errors.New("invalid production execution request wrapper")
		}
		raw = wrapped
		if err := json.Unmarshal(raw, &members); err != nil {
			return err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("invalid production execution request: %w", err)
	}
	if out.Root == "" {
		if out.Operational == nil {
			return errors.New("root and source are required")
		}
		return errors.New("root is required")
	}
	if out.Operational != nil {
		if _, present := members["source"]; present {
			return errors.New("operational and supplied source are mutually exclusive")
		}
		return validateOperationalInput(out.Operational)
	}
	if out.Source == "" {
		return errors.New("root and source are required")
	}
	return nil
}

func productionHandlers(input ProductionInput, root *publication.Root) map[operation.CustodyStage]operation.CustodyStageHandler {
	var manifest, receipt []byte
	fail := operation.CustodyStage(input.FailAt)
	h := make(map[operation.CustodyStage]operation.CustodyStageHandler, 6)
	for _, stage := range []operation.CustodyStage{operation.StageDiscovery, operation.StageReceipt, operation.StageManifest, operation.StageSnapshot, operation.StageAdmission, operation.StagePublication} {
		stage := stage
		h[stage] = func(_ context.Context, _ operation.CustodyRequest, _ operation.CustodyResponse) (operation.StageResult, error) {
			if fail == stage {
				return operation.StageResult{}, fmt.Errorf("injected %s failure", stage)
			}
			switch stage {
			case operation.StageDiscovery:
				return stageJSON(map[string]any{"locator": "hermetic/input.go", "source_digest": productionDigest(input.Source)})
			case operation.StageReceipt:
				_, _, encoded, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: "source-1", Locator: "hermetic/input.go"}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "production", Locator: "hermetic/input.go"}}, []byte(input.Source))
				return operation.StageResult{Artifact: encoded}, err
			case operation.StageManifest:
				var err error
				manifest, err = AssembleDecisionManifest([]source.ManifestReceipt{{ID: "source-1", Path: "input.go", Digest: productionDigest(input.Source)}}, []source.ManifestDecision{{ReceiptID: "source-1", State: source.ManifestInclude}})
				return operation.StageResult{Artifact: manifest}, err
			case operation.StageSnapshot:
				bound, err := source.BindSnapshotTrust(source.SnapshotTrustRequest{Receipts: []source.ManifestReceipt{{ID: "source-1", Path: "input.go", Digest: productionDigest(input.Source)}}, Decisions: []source.ManifestDecision{{ReceiptID: "source-1", State: source.ManifestInclude}}})
				if err != nil {
					return operation.StageResult{}, err
				}
				return stageJSON(map[string]any{"snapshot_identity": bound.SourceSnapshotID})
			case operation.StageAdmission:
				admitted := true
				result, err := stageJSON(map[string]any{"status": "MISSING_TRUST"})
				result.Admitted = &admitted
				return result, err
			case operation.StagePublication:
				completion := publication.Complete(publication.NewPublisher(), root, "artifact.json", publication.AdmittedArtifact{CanonicalBytes: manifest, ArtifactSchemaID: "lsp-trace.source-custody-manifest.v1", Generation: "1", ArtifactReferences: []string{"source-1"}})
				if completion.Err() != nil {
					return operation.StageResult{}, completion.Err()
				}
				var err error
				receipt, err = verification.ReceiptBytes(manifest, verification.DirectoryDurabilityChecked)
				if err != nil {
					return operation.StageResult{}, err
				}
				if published := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: "receipt.json", Bytes: receipt, ArtifactSchemaID: "lsp-trace.publication-receipt.v1"}); published.Err() != nil {
					return operation.StageResult{}, published.Err()
				}
				return stageJSON(completion.Completion)
			}
			return operation.StageResult{}, errors.New("unknown custody stage")
		}
	}
	return h
}

func stageJSON(v any) (operation.StageResult, error) {
	b, e := json.Marshal(v)
	return operation.StageResult{Artifact: b}, e
}
func productionDigest(v string) string {
	s := sha256.Sum256([]byte(v))
	return "sha256:" + hex.EncodeToString(s[:])
}
