package retainedoperation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"lsp-trace/internal/artifactingress"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedinspection"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv3"
)

// Failure is the operation-owned retained projection failure cause.
type Failure struct {
	Phase        string
	State        string
	SelectionKey *retainedinspection.Key
	Diagnostics  []string
}

func (e *Failure) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("retained source projection failed in %s: %s", e.Phase, e.State)
}

// NewInspectHydratedHandler captures the immutable source lookup outside the request.
func NewInspectHydratedHandler(lookup retainedprojection.Lookup) operation.Handler {
	return func(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
		decoded, err := retainedinspection.Decode(request.Input)
		if err != nil {
			var tag struct {
				Mode string `json:"mode"`
			}
			if json.Unmarshal(request.Input, &tag) == nil && tag.Mode != retainedinspection.Mode {
				return operation.InspectHydratedHandler(ctx, request)
			}
			return fail("DECODE", "INVALID_REQUEST", nil, "request decoding rejected")
		}
		if decoded.Legacy != nil {
			return operation.InspectHydratedLegacyDecoded(request, *decoded.Legacy)
		}
		if decoded.Projection == nil {
			return fail("DECODE", "INVALID_REQUEST", nil, "request decoding rejected")
		}
		return inspectProjection(request, *decoded.Projection, lookup)
	}
}

func inspectProjection(request operation.Request, projection retainedinspection.Request, lookup retainedprojection.Lookup) (operation.Result, *operation.Failure) {
	raw, failure := ingress(request, projection.RetainedSourceEvidence)
	if failure != nil {
		return operation.Result{}, failure
	}
	admitted, err := retainedprojection.Admit(raw)
	if err != nil {
		return classify("ADMIT", err)
	}
	selectionRequest := retainedprojection.Request{Target: key(projection.Selection.Target), Selections: make([]retainedprojection.Key, len(projection.Selection.Selections))}
	for i, value := range projection.Selection.Selections {
		selectionRequest.Selections[i] = key(value)
	}
	plan, err := retainedprojection.Select(admitted, selectionRequest)
	if err != nil {
		return classify("SELECT", err)
	}
	resolveLimits := retainedprojection.ResolveLimits{
		MaxDistinctObjects:   projection.ResolveLimits.MaxDistinctObjects,
		MaxUniqueSourceBytes: projection.ResolveLimits.MaxUniqueSourceBytes,
		MaxLogicalSelections: projection.ResolveLimits.MaxLogicalSelections,
	}
	var resolved retainedprojection.ResolveResult
	if projection.Projection.Body == "INCLUDE" {
		if lookup == nil {
			return fail("RESOLVE", string(sourceobject.CodePolicy), nil, "source lookup policy rejected")
		}
		resolved, err = retainedprojection.Resolve(plan, lookup, resolveLimits)
	} else {
		resolved, err = retainedprojection.ResolveMetadata(plan, resolveLimits)
	}
	if err != nil {
		return classify("RESOLVE", err)
	}
	binding, err := admitted.CustodyBinding(plan)
	if err != nil {
		return classify("PROJECT", err)
	}
	limits := projection.Projection.Limits
	maxBytes, ok := checkedInt(limits.MaxSourceBytes)
	if !ok {
		return fail("PROJECT", string(sourceobject.CodeLimit), nil, "projection limit conversion rejected")
	}
	maxRanges, ok := checkedInt(limits.MaxRanges)
	if !ok {
		return fail("PROJECT", string(sourceobject.CodeLimit), nil, "projection limit conversion rejected")
	}
	maxObjects, ok := checkedInt(limits.MaxObjects)
	if !ok {
		return fail("PROJECT", string(sourceobject.CodeLimit), nil, "projection limit conversion rejected")
	}
	maxWork, ok := checkedInt(limits.MaxWork)
	if !ok {
		return fail("PROJECT", string(sourceobject.CodeLimit), nil, "projection limit conversion rejected")
	}
	maxResponse, ok := checkedInt(limits.MaxResponseBytes)
	if !ok {
		return fail("ASSEMBLE", string(sourceobject.CodeLimit), nil, "response limit conversion rejected")
	}
	policy := sourceprojection.Policy{
		PolicyID: projection.Projection.PrivacyPolicyID, BodyRequested: projection.Projection.Body == "INCLUDE",
		MaxBytes: maxBytes, MaxRanges: maxRanges, MaxObjects: maxObjects, MaxWork: maxWork, EnforceLimits: true,
	}
	wire, err := retainedprojection.AssembleV2Bounded(resolved, policy, binding, projection.Projection.PrivacyPolicyID, maxResponse)
	if err != nil {
		return classifyAssembly(err)
	}
	if paging := projection.Projection.Paging; paging != nil {
		requestDigest, err := retainedRequestDigest(projection)
		if err != nil {
			return fail("ASSEMBLE", "ASSEMBLY_FAILED", nil, "request identity rejected")
		}
		page, err := sourceprojectionv3.PaginateV2(wire, requestDigest, sourceprojectionv3.Limits{
			MaxPageBytes: paging.MaxPageBytes, MaxPages: paging.MaxPages, MaxResponseBytes: paging.MaxResponseBytes,
			MaxObjects: limits.MaxObjects, MaxRanges: limits.MaxRanges, MaxSourceBytes: limits.MaxSourceBytes, MaxWork: limits.MaxWork,
		}, paging.Cursor)
		if err != nil {
			return fail("ASSEMBLE", string(sourceobject.CodeLimit), nil, "projection paging rejected")
		}
		artifact, err := json.Marshal(page)
		if err != nil {
			return fail("ASSEMBLE", "ASSEMBLY_FAILED", nil, "result serialization rejected")
		}
		return operation.Result{Value: page, Artifact: append(artifact, '\n'), ArtifactSchemaID: retainedinspection.SourceProjectionSchemaV3ID}, nil
	}
	artifact, err := json.Marshal(wire)
	if err != nil {
		return fail("ASSEMBLE", "ASSEMBLY_FAILED", nil, "result serialization rejected")
	}
	return operation.Result{Value: wire, Artifact: append(artifact, '\n'), ArtifactSchemaID: retainedinspection.SourceProjectionSchemaID}, nil
}

func retainedRequestDigest(request retainedinspection.Request) (string, error) {
	identity := request
	if request.Projection.Paging != nil {
		paging := *request.Projection.Paging
		paging.Cursor = ""
		identity.Projection.Paging = &paging
	}
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func ingress(request operation.Request, evidence retainedinspection.Evidence) ([]byte, *operation.Failure) {
	if evidence.InlineSnapshotV2 != "" {
		return []byte(evidence.InlineSnapshotV2), nil
	}
	config := artifactingress.Config{MaxBytes: artifactingress.HydrationCoreMaxBytes, PublicationRoot: request.PublicationRoot, ArtifactStore: request.ArtifactStore, Validate: ValidateIngress}
	var admitted artifactingress.Result
	var err error
	if source := evidence.PublicationSnapshotV2; source != nil {
		receipt := publication.Receipt{Digest: source.ArtifactDigest, ByteLength: source.ArtifactByteLength, ArtifactSchemaID: source.ArtifactSchemaID, PublicationMechanism: source.PublicationMechanism, Generation: source.Generation, VerificationSelector: source.VerificationSelector}
		admitted, err = config.FromSelector(artifactingress.SelectorRequest{Selector: source.Selector, Receipt: receipt, Expected: artifactingress.Expected{SchemaID: retainedinspection.SourceSnapshotSchemaID, ByteLength: source.ArtifactByteLength, Generation: source.Generation}})
	} else if source := evidence.ContentAddressedSnapshotV2; source != nil {
		admitted, err = config.FromContent(artifactingress.ContentRequest{ID: source.ID, Expected: artifactingress.Expected{SchemaID: retainedinspection.SourceSnapshotSchemaID, ByteLength: source.ArtifactByteLength, Generation: source.Generation}})
	} else {
		_, failure := fail("INGRESS", "ADMISSION_FAILED", nil, "retained snapshot ingress rejected")
		return nil, failure
	}
	if err != nil {
		_, failure := fail("INGRESS", "ADMISSION_FAILED", nil, "retained snapshot ingress rejected")
		return nil, failure
	}
	return admitted.Bytes, nil
}

// ValidateIngress accepts only the exact retained source snapshot schema identity.
func ValidateIngress(schemaID string, _ []byte) error {
	if schemaID != retainedinspection.SourceSnapshotSchemaID {
		return errors.New("retained snapshot schema mismatch")
	}
	return nil
}

func key(value retainedinspection.Key) retainedprojection.Key {
	return retainedprojection.Key{GraphSubjectID: value.GraphSubjectID, LogicalSourceID: value.LogicalSourceID}
}

func classify(phase string, err error) (operation.Result, *operation.Failure) {
	var projectionErr *retainedprojection.Error
	if errors.As(err, &projectionErr) {
		var selection *retainedinspection.Key
		if projectionErr.Key != (retainedprojection.Key{}) {
			selection = &retainedinspection.Key{GraphSubjectID: projectionErr.Key.GraphSubjectID, LogicalSourceID: projectionErr.Key.LogicalSourceID}
		}
		return fail(phase, string(projectionErr.Code), selection, "retained projection rejected")
	}
	var objectErr *sourceobject.Error
	if errors.As(err, &objectErr) {
		return fail(phase, string(objectErr.Code), nil, "source object lookup rejected")
	}
	return fail(phase, "PROJECTION_FAILED", nil, "retained projection rejected")
}

func classifyAssembly(err error) (operation.Result, *operation.Failure) {
	var assemblyErr *retainedprojection.AssemblyError
	if errors.As(err, &assemblyErr) {
		phase := "ASSEMBLE"
		if assemblyErr.Code == retainedprojection.CodeProjectionFailed {
			phase = "PROJECT"
		}
		var selection *retainedinspection.Key
		if assemblyErr.Key != (retainedprojection.Key{}) {
			selection = &retainedinspection.Key{GraphSubjectID: assemblyErr.Key.GraphSubjectID, LogicalSourceID: assemblyErr.Key.LogicalSourceID}
		}
		return fail(phase, string(assemblyErr.Code), selection, "retained projection assembly rejected")
	}
	return fail("ASSEMBLE", "ASSEMBLY_FAILED", nil, "retained projection assembly rejected")
}

func fail(phase, state string, selection *retainedinspection.Key, diagnostic string) (operation.Result, *operation.Failure) {
	cause := &Failure{Phase: phase, State: state, SelectionKey: selection, Diagnostics: []string{diagnostic}}
	return operation.Result{}, &operation.Failure{Code: state, Diagnostics: append([]string(nil), cause.Diagnostics...), Err: cause}
}

func checkedInt(value uint64) (int, bool) {
	if value > uint64(math.MaxInt) {
		return 0, false
	}
	return int(value), true
}
