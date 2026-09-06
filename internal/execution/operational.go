package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/verification"
)

type OperationalFile struct {
	Path  string            `json:"path"`
	Class source.InputClass `json:"class"`
}

// OperationalInput has no grants, verifier context, policy override or host path.
// Every listed read is required for authenticated-required execution.
type OperationalInput struct {
	SourceRoot           string                         `json:"source_root"`
	Inputs               []OperationalFile              `json:"inputs"`
	RequireAuthenticated bool                           `json:"require_authenticated"`
	TrustReceipt         []byte                         `json:"trust_receipt,omitempty"`
	GitAttestation       *schema.GitAttestationEvidence `json:"git_attestation,omitempty"`
	Revision             *source.RevisionAttestation    `json:"revision,omitempty"`
}

func validateOperationalInput(in *OperationalInput) error {
	if !filepath.IsAbs(in.SourceRoot) || filepath.Clean(in.SourceRoot) != in.SourceRoot || strings.ContainsRune(in.SourceRoot, '\x00') || strings.HasPrefix(filepath.ToSlash(in.SourceRoot), "//") {
		return fmt.Errorf("source_root must be a canonical absolute filesystem path")
	}
	if len(in.Inputs) == 0 {
		return fmt.Errorf("operational inputs must not be empty")
	}
	seen := map[string]bool{}
	for _, item := range in.Inputs {
		name := item.Path
		if name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00:") || seen[name] {
			return fmt.Errorf("operational paths must be unique canonical root-relative paths")
		}
		seen[name] = true
		switch item.Class {
		case source.InputSource, source.InputConfiguration, source.InputDeclaration, source.InputGeneratedMapping:
		default:
			return fmt.Errorf("unknown operational input class")
		}
	}
	return nil
}

func operationalWorkspaceURI(root string) string {
	uriPath := filepath.ToSlash(root)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String()
}

func NewProductionExecutorWithTrust(trust *custodyevidence.HostTrustStore) operation.Executor {
	return ProductionExecutor{trust: trust}
}

func operationalHandlers(input ProductionInput, root *publication.Root, trust *custodyevidence.HostTrustStore, e *custodyevidence.Evidence) map[operation.CustodyStage]operation.CustodyStageHandler {
	h := map[operation.CustodyStage]operation.CustodyStageHandler{}
	op := input.Operational
	for _, stage := range []operation.CustodyStage{operation.StageDiscovery, operation.StageReceipt, operation.StageManifest, operation.StageSnapshot, operation.StageAdmission, operation.StagePublication} {
		stage := stage
		h[stage] = func(ctx context.Context, request operation.CustodyRequest, _ operation.CustodyResponse) (operation.StageResult, error) {
			if err := ctx.Err(); err != nil {
				return operation.StageResult{}, err
			}
			if string(stage) == input.FailAt {
				return operation.StageResult{}, fmt.Errorf("injected %s failure", stage)
			}
			switch stage {
			case operation.StageDiscovery:
				return stageJSON(op.Inputs)
			case operation.StageReceipt:
				recorder, err := source.NewInputRecorder(op.SourceRoot)
				if err != nil {
					return operation.StageResult{}, err
				}
				e.SchemaVersion = custodyevidence.Version
				e.RequireAuthenticated = op.RequireAuthenticated
				e.Outputs = []custodyevidence.Output{}
				ids := []string{}
				// Retain already-observed receipts even if cancellation or a
				// later read/binding error exits before normal stage completion.
				defer func() {
					if retained, err := recorder.Evidence(ids); err == nil {
						e.InputEvidence = retained
						e.AcquisitionStatus = custodyevidence.AcquisitionStatus(retained)
					}
					_ = recorder.Close()
				}()
				for _, item := range op.Inputs {
					if err := ctx.Err(); err != nil {
						return operation.StageResult{}, err
					}
					content, id, readErr := recorder.ReadInput(item.Path, item.Class)
					if id == "" {
						return operation.StageResult{}, readErr
					}
					status := source.Readable
					if readErr != nil {
						status = source.Unreadable
					}
					// The actual consumer is this retained byte bundle, not an external parser.
					// Bind precisely the output built from ReadInput's returned bytes/status.
					output := custodyevidence.Output{ID: "input:" + item.Path, Path: item.Path, Status: status, Content: append([]byte(nil), content...)}
					e.Outputs = append(e.Outputs, output)
					ids = append(ids, output.ID)
					if err := recorder.BindContribution(output.ID, id); err != nil {
						return operation.StageResult{}, err
					}
				}
				sort.Slice(e.Outputs, func(i, j int) bool { return e.Outputs[i].ID < e.Outputs[j].ID })
				e.InputEvidence, err = recorder.Evidence(ids)
				if err != nil {
					return operation.StageResult{}, err
				}
				e.AcquisitionStatus = custodyevidence.AcquisitionStatus(e.InputEvidence)
				return stageJSON(e.InputEvidence)
			case operation.StageManifest:
				identity, err := source.BuildObservedIdentity(source.ObservedIdentityRequest{Evidence: e.InputEvidence, Acquisition: source.AcquisitionContext{Adapter: custodyevidence.Adapter, WorkspaceURI: operationalWorkspaceURI(op.SourceRoot), InvocationID: request.OperationID}, Revision: op.Revision})
				if err != nil {
					return operation.StageResult{}, err
				}
				e.Identity = identity
				return operation.StageResult{Artifact: append([]byte(nil), identity.Manifest...)}, nil
			case operation.StageSnapshot:
				return stageJSON(e.Identity)
			case operation.StageAdmission:
				// Rejected trust material remains evidence in permissive mode. The stage's
				// historical Admitted bit is permission to continue, not authentication.
				e.Admission, _ = trust.Admit(e.Identity.Policy, e.Identity.SnapshotID, op.TrustReceipt, op.GitAttestation)
				e.PublicationPermitted = custodyevidence.MayPublish(op.RequireAuthenticated, e.AcquisitionStatus, e.Admission.Status)
				if err := custodyevidence.Validate(*e); err != nil {
					return operation.StageResult{}, err
				}
				result, err := stageJSON(e.Admission)
				allowed := e.PublicationPermitted
				result.Admitted = &allowed
				return result, err
			case operation.StagePublication:
				encoded, err := json.Marshal(e)
				if err != nil {
					return operation.StageResult{}, err
				}
				if _, err := custodyevidence.ValidateFor(encoded, schema.FamilyOperationalCustody, "v1"); err != nil {
					return operation.StageResult{}, err
				}
				if !e.PublicationPermitted {
					return operation.StageResult{}, fmt.Errorf("operational publication not permitted")
				}
				refs := []string{}
				for _, in := range e.InputEvidence.Inputs {
					refs = append(refs, in.ID)
				}
				completion := publication.Complete(publication.NewPublisher(), root, "artifact.json", publication.AdmittedArtifact{CanonicalBytes: encoded, ArtifactSchemaID: custodyevidence.Version, Generation: "1", ArtifactReferences: refs})
				if completion.Err() != nil {
					return operation.StageResult{}, completion.Err()
				}
				receipt, err := verification.ReceiptBytes(encoded, verification.DirectoryDurabilityChecked)
				if err != nil {
					return operation.StageResult{}, err
				}
				if result := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: "receipt.json", Bytes: receipt, ArtifactSchemaID: "lsp-trace.publication-receipt.v1"}); result.Err() != nil {
					return operation.StageResult{}, result.Err()
				}
				return stageJSON(completion.Completion)
			}
			return operation.StageResult{}, fmt.Errorf("unknown operational custody stage")
		}
	}
	return h
}
