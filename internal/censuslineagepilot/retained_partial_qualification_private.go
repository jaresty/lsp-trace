package censuslineagepilot

// Offline inspection qualification, NOT acquisition replay or native admission.
// Borrowed capabilities must not be used or closed concurrently by their owner.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/filter"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inspection"
	"lsp-trace/internal/publication"
)

const retainedPartialQualificationPolicy = "EXACT_RETAINED_PARTIAL_INSPECTION_PRIVATE_V1"
const retainedPartialNativeBlocked = "INSPECTION_QUALIFIED_NATIVE_BLOCKED_NATIVE_CUSTODY"

// No source paths, mutable heads, runtime clients or permission booleans occur in
// this input. The original input roots are needed by the existing full retained
// reader; config pathnames are comparison labels, NEVER opened by this function.
type ExactRetainedPartialQualificationInput struct {
	state *exactRetainedPartialQualificationInput
}
type RetainedPartialObjectPinPrivate struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	Bytes uint64 `json:"bytes"`
}
type RetainedPartialQualificationPinsPrivate struct {
	Schema               string                             `json:"schema"`
	Objects              [8]RetainedPartialObjectPinPrivate `json:"objects"`
	CodeHEAD             string                             `json:"caller_asserted_code_head"`
	VerifierSourceSHA256 string                             `json:"verifier_source_sha256"`
	PolicySHA256         string                             `json:"policy_sha256"`
	MaxReadBytes         uint64                             `json:"max_read_bytes"`
	MaxWorkBytes         uint64                             `json:"max_work_bytes"`
}
type exactRetainedPartialQualificationInput struct {
	pins   RetainedPartialQualificationPinsPrivate
	input  [3]*publication.ContentRetentionRoot
	output [2]*publication.ContentRetentionRoot
}
type opaqueRetainedPartialNativeQualification struct{ report []byte }

// ReportBytes is detached inspection metadata. The type has no admission,
// publication, transition-plan, client, receipt or runtime capability methods.
func (q opaqueRetainedPartialNativeQualification) ReportBytes() []byte { return bytes.Clone(q.report) }

func RetainedPartialQualificationPolicyDigestPrivate() string {
	return Digest([]byte(retainedPartialQualificationPolicy + ":paired-eight:retained-input-prefix:private-parser:canonical-25:recomputed-inspection-filter:no-native-authority"))
}
func RetainedPartialQualificationVerifierPinPrivate() string { return retainedPartialVerifierSourcePin }

var retainedPartialRecordNames = [8]string{"configuration", "input-observation", "claim", "discovery", "canonical-request-seeds", "bounded-partial-artifact", "inspection-filter-views", "outcome"}

// Canonical JSON rejects unknown, duplicate, case-folded, missing, reordered and
// differently versioned fields before any root read. Array shape is preserved.
func NewExactRetainedPartialQualificationInput(raw []byte, input [3]*publication.ContentRetentionRoot, output [2]*publication.ContentRetentionRoot) (ExactRetainedPartialQualificationInput, error) {
	var p RetainedPartialQualificationPinsPrivate
	if scopeBundleStrict(raw, &p) != nil || p.Schema != "lsp-trace.retained-partial-qualification-input.private.v1" || !featureScopeRevision.MatchString(p.CodeHEAD) || p.PolicySHA256 != RetainedPartialQualificationPolicyDigestPrivate() || p.VerifierSourceSHA256 != retainedPartialVerifierSourcePin || p.MaxReadBytes == 0 || p.MaxReadBytes > 128<<20 || p.MaxWorkBytes == 0 || p.MaxWorkBytes > 2<<30 {
		return ExactRetainedPartialQualificationInput{}, ErrInvalid
	}
	var totalBytes uint64
	for i, pin := range p.Objects {
		limit := uint64(4 << 20)
		if i == 0 {
			limit = 32 << 10
		}
		if i == 7 {
			limit = 256 << 10
		}
		if pin.Name != retainedPartialRecordNames[i] || !validScopeBundleDigest(pin.ID) || pin.Bytes == 0 || pin.Bytes > limit {
			return ExactRetainedPartialQualificationInput{}, ErrInvalid
		}
		totalBytes += pin.Bytes
		if totalBytes > 16<<20 {
			return ExactRetainedPartialQualificationInput{}, ErrInvalid
		}
		for j := 0; j < i; j++ {
			if p.Objects[j].ID == pin.ID {
				return ExactRetainedPartialQualificationInput{}, ErrInvalid
			}
		}
	}
	for _, r := range input {
		if r == nil {
			return ExactRetainedPartialQualificationInput{}, ErrInvalid
		}
	}
	for _, r := range output {
		if r == nil {
			return ExactRetainedPartialQualificationInput{}, ErrInvalid
		}
	}
	return ExactRetainedPartialQualificationInput{&exactRetainedPartialQualificationInput{p, input, output}}, nil
}

type retainedPartialInputObservation struct {
	Head     []byte   `json:"head"`
	Scope    []byte   `json:"scope"`
	Metadata []byte   `json:"metadata"`
	Links    [][]byte `json:"links"`
}
type retainedPartialClaimObservation struct {
	Outcome        string                                `json:"outcome"`
	Head           []byte                                `json:"head"`
	CAS            *publication.CompareAndReplaceReceipt `json:"cas,omitempty"`
	CommitReported bool                                  `json:"commit_reported"`
}
type retainedPartialRequestObservation struct {
	Batch    censusacquisition.BatchRequest `json:"batch"`
	Manifest acquisitionmanifest.Manifest   `json:"manifest"`
}

type retainedPartialQualificationReport struct {
	Schema                string                                    `json:"schema"`
	Status                string                                    `json:"status"`
	QualificationID       string                                    `json:"qualification_id"`
	Pins                  RetainedPartialQualificationPinsPrivate   `json:"pins"`
	Roots                 [5]publication.RetentionDirectoryIdentity `json:"roots"`
	Authority             int                                       `json:"authority"`
	Accepted              bool                                      `json:"accepted"`
	Completeness          string                                    `json:"source_completeness"`
	FeatureIdentity       string                                    `json:"feature_identity"`
	Partial               bool                                      `json:"partial"`
	CanonicalComplete     bool                                      `json:"canonical_complete"`
	RevisionQualification string                                    `json:"revision_qualification"`
	VerifierQualification string                                    `json:"verifier_qualification"`
	SeedSHA256            string                                    `json:"seed_sha256"`
	Targets               int                                       `json:"targets"`
	Requests              int                                       `json:"requests"`
	Nodes                 int                                       `json:"nodes"`
	Calls                 int                                       `json:"calls"`
	CallSites             int                                       `json:"call_sites"`
	Frontiers             int                                       `json:"frontiers"`
	SourceStatuses        map[string]int                            `json:"source_statuses"`
	ReadBytes             uint64                                    `json:"read_bytes"`
	ReservedWorkBytes     uint64                                    `json:"reserved_work_bytes"`
	Missing               []string                                  `json:"missing_native_obligations"`
}

// QualifyRetainedPartialNativePrivate executes only retained-content reads and
// existing validators/projections. Repeated verification is deterministic and
// read-only; there is no one-shot token and no successful workflow transition.
func QualifyRetainedPartialNativePrivate(ctx context.Context, in ExactRetainedPartialQualificationInput) (out opaqueRetainedPartialNativeQualification, err error) {
	if ctx == nil || in.state == nil {
		return out, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	s := *in.state // pins are value-only; no mutable caller slice is retained.
	p := s.pins
	var report retainedPartialQualificationReport
	all := append(append([]*publication.ContentRetentionRoot{}, s.input[:]...), s.output[:]...)
	for i, r := range all {
		id, e := r.DirectoryIdentity()
		if e != nil {
			return out, e
		}
		report.Roots[i] = id
		for j := 0; j < i; j++ {
			if id == report.Roots[j] {
				return out, ErrInvalid
			}
		}
	}
	// Reserve byte-equivalent parse/copy/hash work BEFORE reads. This is an
	// invocation-local conservative accounting bound, not a global CPU/heap quota.
	work := scopeRetentionWorkCeiling
	for _, pin := range p.Objects {
		work += 128 * pin.Bytes
	}
	if work > p.MaxWorkBytes {
		return out, errors.New("qualification work reservation")
	}
	var readBytes uint64
	read := func(c context.Context, r *publication.ContentRetentionRoot, id string, n int64, partial bool) ([]byte, error) {
		if n < 1 || n > 4<<20 || uint64(n) > p.MaxReadBytes-readBytes {
			return nil, errors.New("qualification read budget")
		}
		readBytes += uint64(n)
		if e := c.Err(); e != nil {
			return nil, e
		}
		var b []byte
		var e error
		if partial {
			b, e = r.ReadPartialInspectionV2Private(c, id, n)
		} else {
			b, e = r.Read(c, id, n)
		}
		if e != nil {
			return nil, e
		}
		if len(b) != int(n) || Digest(b) != id {
			return nil, ErrInvalid
		}
		return b, nil
	}
	var raw [8][]byte
	for i, pin := range p.Objects {
		a, e := read(ctx, s.output[0], pin.ID, int64(pin.Bytes), true)
		if e != nil {
			return out, e
		}
		b, e := read(ctx, s.output[1], pin.ID, int64(pin.Bytes), true)
		if e != nil {
			return out, e
		}
		if !bytes.Equal(a, b) {
			return out, ErrInvalid
		}
		raw[i] = a
	}
	var cfg ScopedRehearsalConfigPrivate
	if scopeBundleStrict(raw[0], &cfg) != nil || cfg.Schema != "lsp-trace.scoped-rehearsal.private.v4" || cfg.CodeHEAD != p.CodeHEAD || ValidateScopedRehearsalConfigPurePrivate(cfg) != nil {
		return out, errors.New("qualification configuration binding")
	}
	if cfg.OutputIdentities[0] != report.Roots[3] || cfg.OutputIdentities[1] != report.Roots[4] {
		return out, ErrInvalid
	}
	// Borrow supplied capabilities. Never open config.InputRoots or output paths.
	ops := defaultScopeRestartOps()
	ops.open = func(_ context.Context, path string) (*publication.ContentRetentionRoot, error) {
		for i, label := range cfg.InputRoots {
			if path == label {
				return s.input[i], nil
			}
		}
		return nil, ErrInvalid
	}
	ops.close = func(*publication.ContentRetentionRoot) error { return nil }
	ops.get = func(c context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		return read(c, r, id, n, false)
	}
	chargeMetadata := func(n uint64) error {
		if n > p.MaxReadBytes-readBytes {
			return ErrInvalid
		}
		readBytes += n
		return ctx.Err()
	}
	ops.head = func(c context.Context, r *publication.ContentRetentionRoot, id string) ([]byte, error) {
		if e := chargeMetadata(scopeBundleWireLimit); e != nil {
			return nil, e
		}
		return r.ReadScopeBundleHead(c, id)
	}
	ops.receipt = func(c context.Context, r *publication.ContentRetentionRoot, id string) ([]byte, error) {
		if e := chargeMetadata(scopeBundleWireLimit); e != nil {
			return nil, e
		}
		return r.ReadScopeBundleReceipt(c, id)
	}
	restart, e := readRetainedFeatureScopeWithOps(ctx, cfg.InputRoots, cfg.Locator, ops)
	if e != nil {
		return out, fmt.Errorf("qualification retained input: %w", e)
	}
	observation := retainedPartialInputObservation{restart.HeadBytes(), restart.Scope().ManifestBytes(), restart.MetadataBytes(), restart.LinkedReceiptBytes()}
	if !canonicalEqualPartial(raw[1], observation) {
		return out, errors.New("qualification input observation")
	}
	var claim retainedPartialClaimObservation
	if scopeBundleStrict(raw[2], &claim) != nil || claim.Outcome != "CLAIMED" || !claim.CommitReported {
		return out, errors.New("qualification claim observation")
	}
	h, e := parseScopedExecutionHeadPrivate(claim.Head)
	if e != nil {
		return out, e
	}
	b := h.Binding
	if b.InputHead != cfg.Locator.Head || b.Scope != cfg.Locator.Scope || b.Metadata != cfg.Locator.Metadata || b.ExecutableRevision != cfg.CodeHEAD || b.SessionID != cfg.Session.SessionID || b.Generation != cfg.Session.Generation || b.Workspace != cfg.Workspace || b.Policy != scopedExecutionPartialInspectionPolicyV2Private() || b.OutputRoots != cfg.OutputIdentities {
		return out, errors.New("qualification claim binding")
	}
	keyRaw, _ := json.Marshal(b.OutputRoots[2])
	selector := "feature-scope-execution-head-private-v1-" + Digest(keyRaw)[7:] + ".json"
	if !scopedExecutionGoodCASPrivate(claim.CAS, selector, claim.Head) {
		return out, errors.New("qualification retained CAS receipt")
	}
	var discovery censusacquisition.Discovery
	if strictPartialJSON(raw[3], &discovery, 4<<20) != nil || discovery.Session != cfg.Session || discovery.Workspace != cfg.Workspace || discovery.Closure != censusacquisition.DiscoveryClosureSuccessful || len(discovery.ProviderFailures) != 0 || len(discovery.IncompleteCauses) != 0 {
		return out, errors.New("qualification discovery")
	}
	core := censusacquisition.Core{Planning: &censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 63}}
	batches, e := core.PlanDiscovery(cfg.Session, discovery)
	if e != nil || len(batches) != 1 || len(batches[0].Targets) != 25 {
		return out, errors.New("qualification canonical batch")
	}
	total, request := 25000, 5000
	wanted := retainedPartialRequestObservation{batches[0], batches[0].AcquisitionManifest(acquisitionmanifest.Limits{TimeoutMS: &total, RequestTimeoutMS: &request})}
	if !canonicalEqualPartial(raw[4], wanted) {
		return out, errors.New("qualification request/seed binding")
	}
	carrier, e := graphprovenance.ParseSourceWithholdingPrivateV1(raw[5])
	if e != nil {
		return out, fmt.Errorf("qualification private source: %w", e)
	}
	if !carrier.FrozenQueryScope || carrier.Policy.SessionID != cfg.Session.SessionID || carrier.Policy.Generation != cfg.Session.Generation || carrier.Policy.WorkspaceURI != (&url.URL{Scheme: "file", Path: cfg.Workspace}).String() || carrier.Evidence.Acquisition.Graph.Summary.Complete {
		return out, errors.New("qualification partial scope")
	}
	files := restart.Scope().Files()
	for _, member := range carrier.Policy.Members {
		found := false
		for _, file := range files {
			if member.URI == file.URI {
				found = member.SHA256 == file.SHA256 && member.Role == file.Role
			}
		}
		if !found {
			return out, errors.New("qualification source nomination/hash")
		}
	}
	req, e := wanted.Manifest.Request(acquisitionmanifest.SliceV3)
	if e != nil {
		return out, e
	}
	actual := carrier.Evidence.Acquisition.Request
	req.Context = actual.Context
	for i, member := range carrier.Policy.Members {
		req.FrozenQueryURIsPrivate[i] = member.URI
	}
	if !reflect.DeepEqual(req, actual) {
		return out, errors.New("qualification acquisition request binding")
	}
	if e = validateExactPartial25Private(wanted.Manifest, batches[0].CanonicalSeedsV2, files); e != nil {
		return out, e
	}
	projected, e := inspection.ProjectAllSeeds(carrier.Evidence.GraphBytes)
	if e != nil {
		return out, e
	}
	if e = inspection.ValidateAllSeedAccounting(projected); e != nil {
		return out, e
	}
	projectedRaw, e := json.Marshal(projected)
	if e != nil {
		return out, e
	}
	compared, e := filter.ProjectPairwise(projectedRaw, wanted.Manifest.Root.ID, wanted.Manifest.RequiredTargets[0].ID)
	if e != nil {
		return out, e
	}
	views := struct {
		Inspection any `json:"inspection"`
		Filter     any `json:"filter"`
	}{projected, compared}
	if !canonicalEqualPartial(raw[6], views) {
		return out, errors.New("qualification recomputed inspection/filter views")
	}
	var outcome ScopedPartialOutcomeObservationPrivate
	if strictPartialJSON(raw[7], &outcome, 256<<10) != nil || outcome.Schema != "lsp-trace.scoped-partial-inspection-observation.private.v1" || outcome.Authority != 0 || outcome.Accepted || outcome.Completeness != "UNKNOWN" || outcome.FeatureIdentity != "UNRESOLVED" || outcome.RevisionQualification != "CALLER_ASSERTED_NOT_AUTHENTICATED" || outcome.Outcome != "PARTIAL_INSPECTION_RETAINED" || outcome.AcquireCalls != 1 || outcome.Constituents != 0 || outcome.Failure != "" || outcome.CloseFailure != "" || outcome.Purpose != "PARTIAL_INSPECTION" || outcome.Admission != "NOT_ATTEMPTED" || outcome.SourceCompleteness != "UNKNOWN" || outcome.CanonicalComplete || outcome.OutcomeReceipt != nil || len(outcome.Records) != 7 || outcome.WithheldCount != carrier.WithheldCount {
		return out, errors.New("qualification outcome binding")
	}
	expectedBudget := ScopedPartialEffectiveBudgetObservationPrivate{cfg.Budget, 30, 5, 256, 8 << 20, 64, 2 << 20}
	expectedDiscovery := &ScopedDiscoveryPolicyObservationPrivate{"one-partial-inspection-private-v2", 256, 4, 64, 63}
	if outcome.EffectiveBudget != expectedBudget || !reflect.DeepEqual(outcome.DiscoveryPolicy, expectedDiscovery) || outcome.Runtime.PositionEncoding != actual.Context.PositionEncoding || outcome.Runtime.ServerCommand != cfg.Server || !outcome.Runtime.CallHierarchySupport || !outcome.Runtime.DocumentSymbolSupport {
		return out, errors.New("qualification effective policy observation")
	}
	for i, rec := range outcome.Records {
		pin := p.Objects[i]
		good := func(status string, r *publication.BoundFileReceipt) bool {
			return status == "EXISTING_VERIFIED" && r == nil || status == "NAMESPACE_COMMITTED" && scopeBundleGoodReceipt(r, pin.ID, pin.Bytes)
		}
		if rec.Name != pin.Name || rec.ID != pin.ID || uint64(rec.Bytes) != pin.Bytes || rec.Phase != "OBJECT_COPIES_VERIFIED" || !rec.Verified || rec.Error != "" || !good(rec.Primary, rec.PrimaryReceipt) || !good(rec.Backup, rec.BackupReceipt) {
			return out, errors.New("qualification outcome record reference")
		}
	}
	statuses := map[string]int{}
	for _, capture := range carrier.Evidence.Captures {
		statuses[capture.Status]++
	}
	if !reflect.DeepEqual(statuses, outcome.SourceDispositionCounts) {
		return out, errors.New("qualification source classification")
	}
	for i, r := range all {
		id, e := r.DirectoryIdentity()
		if e != nil || id != report.Roots[i] {
			return out, ErrInvalid
		}
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	identity, _ := json.Marshal(struct {
		Pins  RetainedPartialQualificationPinsPrivate
		Roots [5]publication.RetentionDirectoryIdentity
	}{p, report.Roots})
	report.Schema = "lsp-trace.retained-partial-qualification-report.private.v1"
	report.Status = retainedPartialNativeBlocked
	report.QualificationID = Digest(identity)
	report.Pins = p
	report.Completeness = "UNKNOWN"
	report.FeatureIdentity = "UNRESOLVED"
	report.Partial = true
	report.RevisionQualification = "CALLER_ASSERTED_NOT_AUTHENTICATED"
	report.VerifierQualification = "SOURCE_DIGEST_PROPOSAL_NOT_AUTHENTICATED_BINARY"
	report.SeedSHA256 = Digest(batches[0].CanonicalSeedsV2)
	report.Targets = len(actual.RequiredTargets) + 1
	ar := carrier.Evidence.Acquisition
	report.Requests = len(ar.Requests)
	report.Nodes = len(ar.Graph.Nodes)
	report.Calls = len(ar.Graph.Edges)
	report.Frontiers = len(ar.Graph.Frontier)
	for _, edge := range ar.Graph.Edges {
		report.CallSites += len(edge.CallSites)
	}
	report.SourceStatuses = statuses
	report.ReadBytes = readBytes
	report.ReservedWorkBytes = work
	report.Missing = []string{"REQUIRED_NATIVE_GRAPH_V5", "ORDINARY_OR_APPROVED_PRIVATE_SOURCE_CARRIER", "NATIVE_INPUT_MANIFEST_SCOPE_POLICY_BINDING", "VERIFIED_NATIVE_PUBLICATION_AND_PLAN_SOURCE"}
	reportRaw, e := json.Marshal(report)
	if e != nil || len(reportRaw) > 32<<10 {
		return out, ErrInvalid
	}
	return opaqueRetainedPartialNativeQualification{reportRaw}, nil
}

func canonicalEqualPartial(raw []byte, v any) bool {
	b, e := json.Marshal(v)
	return e == nil && bytes.Equal(raw, b)
}
func strictPartialJSON(raw []byte, v any, limit int) error {
	if len(raw) == 0 || len(raw) > limit {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || !canonicalEqualPartial(raw, v) {
		return ErrInvalid
	}
	return nil
}
