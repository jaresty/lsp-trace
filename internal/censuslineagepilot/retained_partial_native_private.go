package censuslineagepilot

// This producer is deliberately adjacent to the qualified retained owner, not a
// graph-provenance registration. Parsed wire data establishes consistency only.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"reflect"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/filter"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inspection"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const DerivedRetainedPartialPrivateV1 = "DERIVED_RETAINED_PARTIAL_PRIVATE_V1"
const MaxDerivedRetainedPartialPrivateBytes = 8 << 20
const derivedRetainedPartialSchema = "lsp-trace.derived-retained-partial-native-input.private.v1"

// These are invocation-local accounting bounds, not new live acquisition limits.
// Read bytes were already charged by the full qualifier. Derivation uses only
// detached immutable bytes, never the qualifier's borrowed directory handles.
type RetainedPartialDerivationBudgetPrivate struct {
	MaxInputBytes  uint64
	MaxOutputBytes uint64
	MaxWorkBytes   uint64
}

// This manifest binds POLICY as a first-class artifact. It is NOT the ordinary
// graph.v5 replay_input_manifest and grants no original capture identity.
type DerivedRetainedPartialInputManifestPrivate struct {
	Schema                          string                                `json:"schema"`
	Profile                         string                                `json:"profile"`
	Status                          string                                `json:"status"`
	Origin                          string                                `json:"origin"`
	QualificationID                 string                                `json:"qualification_id"`
	QualificationReportSHA256       string                                `json:"qualification_report_sha256"`
	PinsSHA256                      string                                `json:"pins_sha256"`
	Records                         [8]RetainedPartialObjectPinPrivate    `json:"records"`
	RetainedInputSHA256             string                                `json:"retained_input_sha256"`
	OriginalAcquisitionGraphSHA256  string                                `json:"original_acquisition_graph_sha256"`
	CanonicalNativeGraphSHA256      string                                `json:"canonical_native_graph_sha256"`
	SourceCarrierSHA256             string                                `json:"source_private_carrier_sha256"`
	SourcePolicySHA256              string                                `json:"source_policy_sha256"`
	SourcePolicy                    graphprovenance.SourcePolicyPrivateV1 `json:"source_policy"`
	SourceReadBudget                graphprovenance.CaptureBudgetV2       `json:"source_read_budget"`
	CanonicalSeedsSHA256            string                                `json:"canonical_seeds_sha256"`
	CanonicalSeeds                  []byte                                `json:"canonical_seeds"`
	CallerAssertedCodeHEAD          string                                `json:"caller_asserted_code_head"`
	VerifierSourcePin               string                                `json:"verifier_source_pin_proposal"`
	ProviderCommand                 string                                `json:"observed_provider_command"`
	SessionID                       string                                `json:"observed_session_id"`
	Generation                      uint64                                `json:"observed_generation"`
	ProviderIdentityQualification   string                                `json:"provider_identity_qualification"`
	Authority                       int                                   `json:"authority"`
	Accepted                        bool                                  `json:"accepted"`
	SourceCompleteness              string                                `json:"source_completeness"`
	FeatureIdentity                 string                                `json:"feature_identity"`
	Partial                         bool                                  `json:"partial"`
	CanonicalComplete               bool                                  `json:"canonical_complete"`
	ReplayAdmitted                  bool                                  `json:"replay_admitted"`
	OriginalNativeV5IdentityClaimed bool                                  `json:"original_native_v5_identity_claimed"`
	ProgramCAdmission               string                                `json:"program_c_admission"`
	Targets                         int                                   `json:"targets"`
	Requests                        int                                   `json:"requests"`
	Nodes                           int                                   `json:"nodes"`
	Calls                           int                                   `json:"calls"`
	CallSites                       int                                   `json:"call_sites"`
	Frontiers                       int                                   `json:"frontiers"`
	WithheldCount                   int                                   `json:"withheld_count"`
	SourceStatuses                  map[string]int                        `json:"source_statuses"`
	QualifiedReadBytes              uint64                                `json:"qualified_read_bytes"`
	QualifiedReservedWorkBytes      uint64                                `json:"qualified_reserved_work_bytes"`
}

type DerivedRetainedPartialNativeInputPrivate struct {
	Schema              string                                     `json:"schema"`
	Manifest            DerivedRetainedPartialInputManifestPrivate `json:"manifest"`
	NativeGraph         []byte                                     `json:"native_graph"`
	QualificationReport []byte                                     `json:"qualification_report"`
	// Exact record lineage retains genuine raw request/element references,
	// acquisition receipts and source bytes. None is a fabricated capture receipt.
	Records [8][]byte `json:"records"`
}

// DerivedRetainedPartialNativePrivate names the existing producer-only value.
// Copies share immutable producer state; JSON decoding cannot create that state.
// This alias adds no constructor and is not an admission or replay capability.
type DerivedRetainedPartialNativePrivate = opaqueDerivedRetainedPartialNativePrivate

type derivedRetainedPartialSealPrivate struct {
	self *derivedRetainedPartialSealPrivate
	raw  []byte
}
type opaqueDerivedRetainedPartialNativePrivate struct {
	raw  []byte
	seal *derivedRetainedPartialSealPrivate
}

// ReadDerivedRetainedPartialNativePrivate returns detached bytes only from an
// actual producer result. The closed parser independently checks consistency;
// parsing or copying exported DTOs cannot mint this producer lineage.
func ReadDerivedRetainedPartialNativePrivate(r DerivedRetainedPartialNativePrivate) ([]byte, error) {
	if r.seal == nil || r.seal.self != r.seal || len(r.raw) == 0 || !bytes.Equal(r.raw, r.seal.raw) {
		return nil, ErrInvalid
	}
	return bytes.Clone(r.seal.raw), nil
}

func (r opaqueDerivedRetainedPartialNativePrivate) Bytes() []byte { return bytes.Clone(r.raw) }
func (r opaqueDerivedRetainedPartialNativePrivate) NativeGraphBytes() []byte {
	var e DerivedRetainedPartialNativeInputPrivate
	if json.Unmarshal(r.raw, &e) != nil {
		return nil
	}
	return e.NativeGraph
}
func (r opaqueDerivedRetainedPartialNativePrivate) ManifestBytes() []byte {
	var e DerivedRetainedPartialNativeInputPrivate
	if json.Unmarshal(r.raw, &e) != nil {
		return nil
	}
	b, _ := json.Marshal(e.Manifest)
	return b
}

// DeriveRetainedPartialNativePrivate accepts only a constructor-sealed actual
// qualification and exact pins. Copying that opaque value shares immutable state;
// decoding its report, mixing qualifications, and zero values grant nothing.
// Repetition is pure derivation, not replay. Context is borrowed, non-concurrent.
func DeriveRetainedPartialNativePrivate(ctx context.Context, q opaqueRetainedPartialNativeQualification, pins RetainedPartialQualificationPinsPrivate, budget RetainedPartialDerivationBudgetPrivate) (opaqueDerivedRetainedPartialNativePrivate, error) {
	var out opaqueDerivedRetainedPartialNativePrivate
	if ctx == nil || q.evidence == nil || q.evidence.self != q.evidence || ctx.Err() != nil {
		return out, ErrInvalid
	}
	var report retainedPartialQualificationReport
	if strictPartialJSON(q.report, &report, 32<<10) != nil || report.Pins != pins {
		return out, ErrInvalid
	}
	n := uint64(len(q.report))
	for _, b := range q.evidence.records {
		n += uint64(len(b))
	}
	// Reserve before parse/copy/hash. Output bound is finite and private; these
	// deliberately conservative multiples include revalidation and base64 copies.
	if budget.MaxInputBytes == 0 || budget.MaxInputBytes > MaxDerivedRetainedPartialPrivateBytes || n > budget.MaxInputBytes || budget.MaxOutputBytes == 0 || budget.MaxOutputBytes > MaxDerivedRetainedPartialPrivateBytes || budget.MaxWorkBytes == 0 || budget.MaxWorkBytes > 2<<30 || n*256 > budget.MaxWorkBytes {
		return out, errors.New("private derivation budget")
	}
	e := DerivedRetainedPartialNativeInputPrivate{Schema: derivedRetainedPartialSchema, QualificationReport: bytes.Clone(q.report)}
	for i, b := range q.evidence.records {
		e.Records[i] = bytes.Clone(b)
	}
	native, manifest, err := buildDerivedRetainedPartialPrivate(e)
	if err != nil {
		return out, err
	}
	e.NativeGraph, e.Manifest = native, manifest
	raw, err := json.Marshal(e)
	if err != nil || uint64(len(raw)) > budget.MaxOutputBytes {
		return out, errors.New("private derived output budget")
	}
	if _, err = ParseDerivedRetainedPartialNativePrivate(raw); err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	seal := &derivedRetainedPartialSealPrivate{raw: bytes.Clone(raw)}
	seal.self = seal
	return opaqueDerivedRetainedPartialNativePrivate{raw: raw, seal: seal}, nil
}

// ParseDerivedRetainedPartialNativePrivate is a CLOSED consistency parser, NOT a
// producer, admission, source authority, publication or Program C capability.
// The size and lexical-depth checks precede typed decoding; canonical equality
// rejects unknown/duplicate/case-alias/missing fields, including nested records.
func ParseDerivedRetainedPartialNativePrivate(raw []byte) (DerivedRetainedPartialNativeInputPrivate, error) {
	var e DerivedRetainedPartialNativeInputPrivate
	if len(raw) == 0 || len(raw) > MaxDerivedRetainedPartialPrivateBytes || preflightDerivedRetainedPartialPrivate(raw) != nil || strictjson.RejectDuplicates(raw) != nil || strictPartialJSON(raw, &e, MaxDerivedRetainedPartialPrivateBytes) != nil || e.Schema != derivedRetainedPartialSchema {
		return DerivedRetainedPartialNativeInputPrivate{}, ErrInvalid
	}
	native, manifest, err := buildDerivedRetainedPartialPrivate(e)
	if err != nil {
		return DerivedRetainedPartialNativeInputPrivate{}, err
	}
	if !bytes.Equal(native, e.NativeGraph) || !reflect.DeepEqual(manifest, e.Manifest) {
		return DerivedRetainedPartialNativeInputPrivate{}, errors.New("private native input binding")
	}
	return e, nil
}

func buildDerivedRetainedPartialPrivate(e DerivedRetainedPartialNativeInputPrivate) ([]byte, DerivedRetainedPartialInputManifestPrivate, error) {
	var m DerivedRetainedPartialInputManifestPrivate
	fail := func() ([]byte, DerivedRetainedPartialInputManifestPrivate, error) { return nil, m, ErrInvalid }
	var r retainedPartialQualificationReport
	if strictPartialJSON(e.QualificationReport, &r, 32<<10) != nil || r.Schema != "lsp-trace.retained-partial-qualification-report.private.v1" || r.Status != retainedPartialNativeBlocked || r.Authority != 0 || r.Accepted || r.Completeness != "UNKNOWN" || r.FeatureIdentity != "UNRESOLVED" || !r.Partial || r.CanonicalComplete || r.RevisionQualification != "CALLER_ASSERTED_NOT_AUTHENTICATED" || r.VerifierQualification != "SOURCE_DIGEST_PROPOSAL_NOT_AUTHENTICATED_BINARY" {
		return fail()
	}
	p := r.Pins
	if p.Schema != "lsp-trace.retained-partial-qualification-input.private.v1" || !featureScopeRevision.MatchString(p.CodeHEAD) || p.VerifierSourceSHA256 != retainedPartialVerifierSourcePin || p.PolicySHA256 != RetainedPartialQualificationPolicyDigestPrivate() || p.MaxReadBytes == 0 || p.MaxReadBytes > 128<<20 || p.MaxWorkBytes == 0 || p.MaxWorkBytes > 2<<30 || r.ReadBytes > p.MaxReadBytes || r.ReservedWorkBytes > p.MaxWorkBytes {
		return fail()
	}
	var total uint64
	for i, b := range e.Records {
		pin := p.Objects[i]
		limit := 4 << 20
		if i == 0 {
			limit = 32 << 10
		}
		if i == 7 {
			limit = 256 << 10
		}
		if pin.Name != retainedPartialRecordNames[i] || len(b) == 0 || len(b) > limit || uint64(len(b)) != pin.Bytes || Digest(b) != pin.ID {
			return fail()
		}
		total += uint64(len(b))
		for j := 0; j < i; j++ {
			if p.Objects[j].ID == pin.ID {
				return fail()
			}
		}
	}
	if total > MaxDerivedRetainedPartialPrivateBytes {
		return fail()
	}
	for i, root := range r.Roots {
		for j := 0; j < i; j++ {
			if root == r.Roots[j] {
				return fail()
			}
		}
	}
	identity, _ := json.Marshal(struct {
		Pins  RetainedPartialQualificationPinsPrivate
		Roots [5]publication.RetentionDirectoryIdentity
	}{p, r.Roots})
	if Digest(identity) != r.QualificationID {
		return fail()
	}
	return finishDerivedRetainedPartialPrivate(e, r)
}

func finishDerivedRetainedPartialPrivate(e DerivedRetainedPartialNativeInputPrivate, r retainedPartialQualificationReport) ([]byte, DerivedRetainedPartialInputManifestPrivate, error) {
	var m DerivedRetainedPartialInputManifestPrivate
	fail := func() ([]byte, DerivedRetainedPartialInputManifestPrivate, error) { return nil, m, ErrInvalid }
	var cfg ScopedRehearsalConfigPrivate
	if scopeBundleStrict(e.Records[0], &cfg) != nil || ValidateScopedRehearsalConfigPurePrivate(cfg) != nil || cfg.CodeHEAD != r.Pins.CodeHEAD || cfg.OutputIdentities[0] != r.Roots[3] || cfg.OutputIdentities[1] != r.Roots[4] {
		return fail()
	}
	var observation retainedPartialInputObservation
	var scope scopeManifest
	if strictPartialJSON(e.Records[1], &observation, 4<<20) != nil || scopeBundleStrict(observation.Scope, &scope) != nil || Digest(observation.Scope) != cfg.Locator.Scope.ID || uint64(len(observation.Scope)) != cfg.Locator.Scope.ByteLength || scope.Authority != 0 || scope.Accepted || scope.Completeness != "UNKNOWN" || scope.FeatureIdentity != "UNRESOLVED" {
		return fail()
	}
	if Digest(observation.Head) != cfg.Locator.Head.ID || uint64(len(observation.Head)) != cfg.Locator.Head.ByteLength || Digest(observation.Metadata) != cfg.Locator.Metadata.ID || uint64(len(observation.Metadata)) != cfg.Locator.Metadata.ByteLength {
		return fail()
	}
	var request retainedPartialRequestObservation
	var discovery censusacquisition.Discovery
	if strictPartialJSON(e.Records[3], &discovery, 4<<20) != nil || discovery.Session != cfg.Session || discovery.Workspace != cfg.Workspace || discovery.Closure != censusacquisition.DiscoveryClosureSuccessful || len(discovery.ProviderFailures) != 0 || len(discovery.IncompleteCauses) != 0 || strictPartialJSON(e.Records[4], &request, 4<<20) != nil {
		return fail()
	}
	core := censusacquisition.Core{Planning: &censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 63}}
	batches, err := core.PlanDiscovery(cfg.Session, discovery)
	if err != nil || len(batches) != 1 || !reflect.DeepEqual(batches[0], request.Batch) || !bytes.Equal(batches[0].CanonicalSeedsV2, request.Batch.CanonicalSeedsV2) {
		return fail()
	}
	carrier, err := graphprovenance.ParseSourceWithholdingPrivateV1(e.Records[5])
	if err != nil {
		return nil, m, err
	}
	a := carrier.Evidence.Acquisition
	if carrier.Policy.WorkspaceURI != (&url.URL{Scheme: "file", Path: cfg.Workspace}).String() {
		return fail()
	}
	if !carrier.FrozenQueryScope || carrier.ReplayAdmitted || carrier.Accepted || carrier.Authority != 0 || carrier.Policy.SessionID != cfg.Session.SessionID || carrier.Policy.Generation != cfg.Session.Generation || a.Graph.Summary.Complete || a.Graph.Invocation.Expansion.TopmostSiblings || len(a.Graph.SiblingCandidates) != 0 || acquisition.ValidateResult(a) != nil {
		return fail()
	}
	files := make([]scopeObservedFile, len(scope.Files))
	for i, f := range scope.Files {
		files[i].scopeManifestFile = f
	}
	if validateExactPartial25Private(request.Manifest, request.Batch.CanonicalSeedsV2, files) != nil {
		return fail()
	}
	for _, member := range carrier.Policy.Members {
		found := false
		for _, file := range scope.Files {
			if member.URI == file.URI {
				found = member.SHA256 == file.SHA256 && member.Role == file.Role
			}
		}
		if !found {
			return fail()
		}
	}
	expectedRequest, err := request.Manifest.Request(acquisitionmanifest.SliceV3)
	if err != nil {
		return fail()
	}
	expectedRequest.Context = a.Request.Context
	for i, member := range carrier.Policy.Members {
		expectedRequest.FrozenQueryURIsPrivate[i] = member.URI
	}
	if !reflect.DeepEqual(expectedRequest, a.Request) {
		return fail()
	}
	if err := validateDerivedRetainedPartialProofPrivate(e.Records, r, cfg, carrier); err != nil {
		return nil, m, err
	}
	// The private typed projection must preserve original process-context metadata;
	// historical Result decoding cannot recover omitted environment/cwd secrets.
	native, err := graph.ProjectRetainedV3ToV5Private(carrier.Evidence.GraphBytes, carrier.Policy.WorkspaceURI, cfg.Server)
	if err != nil {
		return nil, m, err
	}
	if detected, err := schema.ValidateFor(native, schema.FamilyGraph, "v5"); err != nil || detected != graph.SchemaVersionV5 {
		return nil, m, errors.New("derived native graph v5 validation")
	}
	statuses := map[string]int{}
	for _, capture := range carrier.Evidence.Captures {
		statuses[capture.Status]++
	}
	sites := 0
	for _, edge := range a.Graph.Edges {
		sites += len(edge.CallSites)
	}
	if r.Targets != len(a.Request.RequiredTargets)+1 || r.Targets != 25 || r.Requests != len(a.Requests) || r.Nodes != len(a.Graph.Nodes) || r.Calls != len(a.Graph.Edges) || r.CallSites != sites || r.Frontiers != len(a.Graph.Frontier) || r.SeedSHA256 != Digest(request.Batch.CanonicalSeedsV2) || !reflect.DeepEqual(statuses, r.SourceStatuses) {
		return fail()
	}
	pins, _ := json.Marshal(r.Pins)
	policy, _ := json.Marshal(carrier.Policy)
	m = DerivedRetainedPartialInputManifestPrivate{
		Schema: "lsp-trace.derived-retained-partial-input-manifest.private.v1", Profile: DerivedRetainedPartialPrivateV1,
		Status: "VALIDATED_PRIVATE_INPUT_MANIFEST", Origin: "DERIVED_RETAINED_INSPECTION", QualificationID: r.QualificationID,
		QualificationReportSHA256: Digest(e.QualificationReport), PinsSHA256: Digest(pins), Records: r.Pins.Objects,
		RetainedInputSHA256: Digest(e.Records[1]), OriginalAcquisitionGraphSHA256: Digest(carrier.Evidence.GraphBytes), CanonicalNativeGraphSHA256: Digest(native),
		SourceCarrierSHA256: Digest(e.Records[5]), SourcePolicySHA256: Digest(policy), SourcePolicy: carrier.Policy, SourceReadBudget: carrier.Evidence.CaptureBudget,
		CanonicalSeedsSHA256: Digest(request.Batch.CanonicalSeedsV2), CanonicalSeeds: bytes.Clone(request.Batch.CanonicalSeedsV2),
		CallerAssertedCodeHEAD: cfg.CodeHEAD, VerifierSourcePin: r.Pins.VerifierSourceSHA256, ProviderCommand: cfg.Server,
		SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, ProviderIdentityQualification: "ORIGINAL_OBSERVATION_NOT_LIVE_LEASE",
		SourceCompleteness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Partial: true, ProgramCAdmission: "NOT_PROGRAM_C_ADMITTED",
		Targets: r.Targets, Requests: r.Requests, Nodes: r.Nodes, Calls: r.Calls, CallSites: r.CallSites, Frontiers: r.Frontiers,
		WithheldCount: carrier.WithheldCount, SourceStatuses: statuses, QualifiedReadBytes: r.ReadBytes, QualifiedReservedWorkBytes: r.ReservedWorkBytes,
	}
	return native, m, nil
}

// The private envelope has only small typed arrays; large evidence is byte
// carriers. Bound lexical depth, tokens and array cardinality before allocation.
func preflightDerivedRetainedPartialPrivate(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var counts []int
	tokens := 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ErrInvalid
		}
		tokens++
		if tokens > 32768 {
			return ErrInvalid
		}
		if len(counts) > 0 {
			counts[len(counts)-1]++
			if counts[len(counts)-1] > 512 {
				return ErrInvalid
			}
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				counts = append(counts, 0)
				if len(counts) > 32 {
					return ErrInvalid
				}
			case '}', ']':
				if len(counts) == 0 {
					return ErrInvalid
				}
				counts = counts[:len(counts)-1]
			}
		}
	}
	if len(counts) != 0 {
		return ErrInvalid
	}
	return nil
}

func validateDerivedRetainedPartialProofPrivate(raw [8][]byte, r retainedPartialQualificationReport, cfg ScopedRehearsalConfigPrivate, carrier graphprovenance.SourceWithholdingPrivateV1) error {
	var claim retainedPartialClaimObservation
	if scopeBundleStrict(raw[2], &claim) != nil || claim.Outcome != "CLAIMED" || !claim.CommitReported {
		return ErrInvalid
	}
	h, err := parseScopedExecutionHeadPrivate(claim.Head)
	if err != nil {
		return err
	}
	b := h.Binding
	if b.InputHead != cfg.Locator.Head || b.Scope != cfg.Locator.Scope || b.Metadata != cfg.Locator.Metadata || b.ExecutableRevision != cfg.CodeHEAD || b.SessionID != cfg.Session.SessionID || b.Generation != cfg.Session.Generation || b.Workspace != cfg.Workspace || b.Policy != scopedExecutionPartialInspectionPolicyV2Private() || b.OutputRoots != cfg.OutputIdentities {
		return ErrInvalid
	}
	key, _ := json.Marshal(b.OutputRoots[2])
	if !scopedExecutionGoodCASPrivate(claim.CAS, "feature-scope-execution-head-private-v1-"+Digest(key)[7:]+".json", claim.Head) {
		return ErrInvalid
	}
	var request retainedPartialRequestObservation
	if strictPartialJSON(raw[4], &request, 4<<20) != nil {
		return ErrInvalid
	}
	total, timeout := 25000, 5000
	expectedManifest := request.Batch.AcquisitionManifest(acquisitionmanifest.Limits{TimeoutMS: &total, RequestTimeoutMS: &timeout})
	if !reflect.DeepEqual(request.Manifest, expectedManifest) {
		return ErrInvalid
	}
	projected, err := inspection.ProjectAllSeeds(carrier.Evidence.GraphBytes)
	if err != nil {
		return err
	}
	if inspection.ValidateAllSeedAccounting(projected) != nil {
		return ErrInvalid
	}
	projectedRaw, _ := json.Marshal(projected)
	compared, err := filter.ProjectPairwise(projectedRaw, request.Manifest.Root.ID, request.Manifest.RequiredTargets[0].ID)
	if err != nil {
		return err
	}
	views := struct {
		Inspection any `json:"inspection"`
		Filter     any `json:"filter"`
	}{projected, compared}
	if !canonicalEqualPartial(raw[6], views) {
		return ErrInvalid
	}
	var o ScopedPartialOutcomeObservationPrivate
	if strictPartialJSON(raw[7], &o, 256<<10) != nil || o.Schema != "lsp-trace.scoped-partial-inspection-observation.private.v1" || o.Authority != 0 || o.Accepted || o.Completeness != "UNKNOWN" || o.FeatureIdentity != "UNRESOLVED" || o.RevisionQualification != "CALLER_ASSERTED_NOT_AUTHENTICATED" || o.Outcome != "PARTIAL_INSPECTION_RETAINED" || o.AcquireCalls != 1 || o.Constituents != 0 || o.Failure != "" || o.CloseFailure != "" || o.Purpose != "PARTIAL_INSPECTION" || o.Admission != "NOT_ATTEMPTED" || o.SourceCompleteness != "UNKNOWN" || o.CanonicalComplete || o.OutcomeReceipt != nil || len(o.Records) != 7 || o.WithheldCount != carrier.WithheldCount || !reflect.DeepEqual(o.SourceDispositionCounts, r.SourceStatuses) {
		return ErrInvalid
	}
	expectedBudget := ScopedPartialEffectiveBudgetObservationPrivate{cfg.Budget, 30, 5, 256, 8 << 20, 64, 2 << 20}
	expectedDiscovery := &ScopedDiscoveryPolicyObservationPrivate{"one-partial-inspection-private-v2", 256, 4, 64, 63}
	if o.EffectiveBudget != expectedBudget || !reflect.DeepEqual(o.DiscoveryPolicy, expectedDiscovery) || o.Runtime.PositionEncoding != carrier.Evidence.Acquisition.Request.Context.PositionEncoding || o.Runtime.ServerCommand != cfg.Server || !o.Runtime.CallHierarchySupport || !o.Runtime.DocumentSymbolSupport {
		return ErrInvalid
	}
	for i, rec := range o.Records {
		pin := r.Pins.Objects[i]
		good := func(status string, receipt *publication.BoundFileReceipt) bool {
			return status == "EXISTING_VERIFIED" && receipt == nil || status == "NAMESPACE_COMMITTED" && scopeBundleGoodReceipt(receipt, pin.ID, pin.Bytes)
		}
		if rec.Name != pin.Name || rec.ID != pin.ID || uint64(rec.Bytes) != pin.Bytes || rec.Phase != "OBJECT_COPIES_VERIFIED" || !rec.Verified || rec.Error != "" || !good(rec.Primary, rec.PrimaryReceipt) || !good(rec.Backup, rec.BackupReceipt) {
			return ErrInvalid
		}
	}
	return nil
}
