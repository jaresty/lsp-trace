// Package sourcecontinuation is the upper producer-owned bridge. The lower
// census package cannot import inventorysuccessor and cannot admit sources.
package sourcecontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	inv "lsp-trace/internal/inventorysuccessor"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
	"time"
)

const PrivateAllModeSourcesVersion = cc.PrivateAllModeSourcesVersion
const StopAfterPrivateSourceAdmissions = cc.StopAfterPrivateSourceAdmissions
const StopAfterPrivateSourceRequests = cc.StopAfterPrivateSourceRequests

type PrivateAllModeSourcesRequest struct {
	PolicyID    string
	Limits      tp.PrivateSourceLimits
	Deadline    time.Time
	Originals   []inv.PrivateProducerOriginals
	Definitions []sessionruntime.PrivateGroupingSourceEvidence
	Objects     []sourceobject.Object
}
type Request struct {
	PrivateSources *PrivateAllModeSourcesRequest
	PrivateAllMode *cc.PrivateAllModeHandoff
	Handoff        cc.CommittedHandoff
	Workspace      string
	Contract       cc.ContinuationContract
	Worker         cc.Worker
	WorkerV2       cc.WorkerV2
	Store          cc.Store
	FreshCapture   *cc.FreshCaptureDependencies
	StopAfter      cc.StopAfter
}
type ResumeRequest struct {
	PrivateSources        *PrivateAllModeSourcesResume
	PrivateAllMode        *cc.PrivateAllModeResume
	Selector, Workspace   string
	UseCommittedWorkspace bool
	Store                 cc.Store
	Worker                cc.Worker
	WorkerV2              cc.WorkerV2
	FreshCapture          *cc.FreshCaptureDependencies
	StopAfter             cc.StopAfter
}
type Result struct {
	cc.Result
	PrivateSources *PrivateAllModeSourcesResult
}
type PrivateSourceOutcome struct {
	Intent                              int
	Status, Gap, AdmissionID, RequestID string
}
type PrivateSourceCounts struct{ Intents, Admitted, Unresolved, Requests int }
type PrivateAllModeSourcesResult struct {
	Version, Stage, ClaimCeiling, HandoffID, CheckpointID, Durability string
	Counts                                                            PrivateSourceCounts
	Outcomes                                                          []PrivateSourceOutcome
	Units                                                             []tp.PrivateSourceUnit
	Requests                                                          []describerequest.Record
	Expected                                                          PrivateAllModeSourcesExpected `json:"-"`
}

// Record alone is only wire format. This opaque expectation retains actual
// admitted units, exact producer original handles and original source checkpoint.
type PrivateAllModeSourcesExpected struct{ state *expected }
type expected struct {
	handoff                    cc.PrivateAllModeHandoff
	contract                   cc.ContinuationContract
	request                    PrivateAllModeSourcesRequest
	stop                       cc.StopAfter
	checkpoint, baseCheckpoint string
	admissions                 []inv.PrivateSourceAdmission
}
type PrivateAllModeSourcesResume struct {
	Version         string
	Expected        PrivateAllModeSourcesExpected
	Originals       []inv.PrivateProducerOriginals
	Definitions     []sessionruntime.PrivateGroupingSourceEvidence
	ExpectedHandoff cc.PrivateAllModeHandoff
}
type checkpoint struct {
	Version, Stage, Status, ClaimCeiling, HandoffID, ContractID, PolicyID, Workspace, BaseCheckpoint string
	Deadline                                                                                         time.Time
	Limits                                                                                           tp.PrivateSourceLimits
	Artifacts                                                                                        []cc.ArtifactRef
}
type material struct {
	kind string
	raw  []byte
}
type boundedStore interface {
	cc.Store
	MaxObjectBytes() int64
}

func digest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func failure(e error) Result {
	return Result{Result: cc.Result{PrivateStage: "PRIVATE_SOURCES", PrivateCode: "PRIVATE_SOURCE_STAGE_REFUSED", Err: e}}
}
func sourceStop(s cc.StopAfter) bool {
	return s == StopAfterPrivateSourceAdmissions || s == StopAfterPrivateSourceRequests
}

// Preflight precedes even metadata publication; no capture, evaluator or worker
// is reachable. All finite ceilings come from the existing contract.
func preflight(ctx context.Context, q PrivateAllModeSourcesRequest, c cc.ContinuationContract, store cc.Store, workspace string, stop cc.StopAfter) error {
	bad := func(s string) error { return errors.New(s) }
	if ctx == nil || ctx.Err() != nil || !sourceStop(stop) || !q.Limits.Valid() || q.Deadline.IsZero() || !time.Now().Before(q.Deadline) || q.Deadline.After(time.Now().Add(15*time.Second)) || len(q.PolicyID) == 0 || len(q.PolicyID) > 4096 {
		return bad("private source context/policy/stop/deadline")
	}
	if d, ok := ctx.Deadline(); ok && d.After(q.Deadline) {
		return bad("private source context exceeds original deadline")
	}
	bs, ok := store.(boundedStore)
	if !ok || bs.MaxObjectBytes() <= 0 {
		return bad("private source finite store")
	}
	l := c.CaptureLimits()
	if q.PolicyID != c.PacketSourcePolicy().PolicyID {
		return bad("private source policy mismatch")
	}
	if q.Limits.MaxDocumentBytes > l.MaxSourceBytes || q.Limits.MaxTotalBytes > l.MaxTotalSourceBytes || q.Limits.MaxObjects > l.MaxBindings || q.Limits.MaxWork > l.MaxWork || len(q.Originals) > q.Limits.MaxObjects || len(q.Definitions) > q.Limits.MaxObjects || len(q.Objects) > q.Limits.MaxObjects {
		return bad("private source exceeds existing contract")
	}
	upper := uint64(16384)
	seen := map[string]bool{}
	for _, o := range q.Originals {
		b := o.Bound()
		if b == 0 || b > 8<<20 || o.InputID() == "" || o.Workspace() != workspace || o.SourceByteLength() > q.Limits.MaxDocumentBytes || o.Deadline().Before(q.Deadline) || seen[o.InputID()] {
			return bad("private source original bound/deadline")
		}
		seen[o.InputID()] = true
		if b > uint64(l.MaxParentBytes)/4 || upper+b*4 > uint64(l.MaxParentBytes) {
			return bad("private source aggregate original work bound")
		}
		upper += b * 4
	}
	def := uint64(0)
	for _, d := range q.Definitions {
		b, ok := d.EncodedBound()
		if !ok || d.SourceByteLength() > q.Limits.MaxDocumentBytes || b > sessionruntime.PrivateGroupingQueryMaxBytes || b > uint64(8<<20)-def {
			return bad("private definition aggregate bound")
		}
		def += b
	}
	total := 0
	for _, obj := range q.Objects {
		if len(obj.Identity.Digest) != 71 || len(obj.Bytes) > q.Limits.MaxDocumentBytes || obj.Identity.ByteLength > uint64(q.Limits.MaxDocumentBytes) || len(obj.Bytes) > q.Limits.MaxTotalBytes-total {
			return bad("private provided source bound")
		}
		total += len(obj.Bytes)
	}
	return nil
}
func prepare(ctx context.Context, base cc.PrivateAllModeResult, h cc.PrivateAllModeHandoff, c cc.ContinuationContract, q PrivateAllModeSourcesRequest, store cc.Store, workspace string, stop cc.StopAfter) (PrivateAllModeSourcesResult, []material, []inv.PrivateSourceAdmission, error) {
	bad := func(s string) (PrivateAllModeSourcesResult, []material, []inv.PrivateSourceAdmission, error) {
		return PrivateAllModeSourcesResult{}, nil, nil, errors.New(s)
	}
	if err := preflight(ctx, q, c, store, workspace, stop); err != nil {
		return bad(err.Error())
	}
	if len(base.Intents) > q.Limits.MaxWork || len(base.Intents) > 1000 {
		return bad("private source intent count")
	}
	l := c.CaptureLimits()
	work := len(base.Intents) + len(base.Nominations) + len(q.Objects) + len(q.Definitions) + len(q.Originals)
	upper := int64(16384)
	sourceBytes := 0
	byInput := map[string]inv.PrivateProducerOriginals{}
	for _, o := range q.Originals {
		byInput[o.InputID()] = o
		upper += int64(o.Bound()) * 4
	}
	def := uint64(0)
	for _, d := range q.Definitions {
		b, _ := d.EncodedBound()
		def += b
	}
	for _, o := range q.Objects {
		sourceBytes += len(o.Bytes)
	}
	type key struct {
		input    string
		endpoint tp.PrivateTypedEndpoint
	}
	unique := map[key]bool{}
	for _, i := range base.Intents {
		k := key{i.InputID, i.Endpoint}
		if _, ok := byInput[i.InputID]; ok && !unique[k] {
			unique[k] = true
			b := def + uint64(sourceBytes)*4
			if b > uint64(l.MaxParentBytes) || int64(b) > int64(l.MaxParentBytes)-upper {
				return bad("private source aggregate original work bound")
			}
			upper += int64(b)
			work += len(q.Definitions) + len(q.Objects) + 32
		}
	}
	if work > q.Limits.MaxWork || upper > int64(l.MaxParentBytes) {
		return bad("private source aggregate work bound")
	}
	capBytes := store.(boundedStore).MaxObjectBytes()
	if capBytes > int64(l.MaxArtifactBytes) {
		capBytes = int64(l.MaxArtifactBytes)
	}
	if capBytes > 8<<20 {
		capBytes = 8 << 20
	}
	if int64(sourceBytes)*12+int64(len(base.Intents))*16384 > int64(l.MaxParentBytes) {
		return bad("private source output aggregate preflight")
	}
	for id, o := range byInput {
		p, err := inv.PreparePrivateProducerOriginals(o, q.Limits)
		if err != nil {
			return bad(err.Error())
		}
		byInput[id] = p
	}
	cache := map[key]inv.PrivateSourceAdmission{}
	gaps := map[key]string{}
	records := map[string]describerequest.Record{}
	seen := map[string]bool{}
	var admissions []inv.PrivateSourceAdmission
	out := PrivateAllModeSourcesResult{Version: PrivateAllModeSourcesVersion, Stage: string(stop), ClaimCeiling: rg.EvaluationCeilingPrivate, HandoffID: h.ID(), Durability: "PROCESS_LOCAL_EXACT_EXPECTED_SOURCE_PROOFS_REQUIRED;NOT_INDEPENDENT_RESTART_ADMISSION"}
	out.Counts.Intents = len(base.Intents)
	for n, i := range base.Intents {
		if ctx.Err() != nil || !time.Now().Before(q.Deadline) {
			return bad("private source cancelled or deadline")
		}
		row := PrivateSourceOutcome{Intent: n, Status: "UNRESOLVED", Gap: i.Gap}
		if i.Endpoint.Disposition == "EXCLUDED" {
			row.Gap = "GROUPING_ENDPOINT_EXCLUDED"
			out.Counts.Unresolved++
			out.Outcomes = append(out.Outcomes, row)
			continue
		}
		o, found := byInput[i.InputID]
		if !found {
			if i.Endpoint.Kind != "" {
				row.Gap = "NO_OPAQUE_PRODUCER_ORIGINALS"
			}
			out.Counts.Unresolved++
			out.Outcomes = append(out.Outcomes, row)
			continue
		}
		k := key{i.InputID, i.Endpoint}
		a, cached := cache[k]
		gap := gaps[k]
		if cached {
			var err error
			a, err = inv.RebindPrivateSourceAdmission(i, a)
			if err != nil {
				return bad(err.Error())
			}
		} else if gap == "" {
			a, gap = inv.AdmitPrivateEndpointSource(i, o, q.Definitions, q.Objects, q.Limits)
			if a.Valid() {
				cache[k] = a
			} else {
				gaps[k] = gap
			}
		}
		if !a.Valid() {
			row.Gap = gap
			out.Counts.Unresolved++
			out.Outcomes = append(out.Outcomes, row)
			continue
		}
		u, err := a.Unit()
		if err != nil {
			return bad(err.Error())
		}
		admissions = append(admissions, a)
		row.Status = "SOURCE_ADMITTED"
		row.Gap = ""
		row.AdmissionID = u.ID
		out.Counts.Admitted++
		if !seen[u.ID] {
			seen[u.ID] = true
			out.Units = append(out.Units, u)
		}
		if stop == StopAfterPrivateSourceRequests {
			r, exists := records[u.ID]
			if !exists {
				recordCap := int(capBytes)
				if c.PacketMaxResponseBytes() < recordCap {
					recordCap = c.PacketMaxResponseBytes()
				}
				if a.Bound() > recordCap {
					return bad("private source request bound")
				}
				// Only this opaque-admission branch may publish an eligible stage request.
				r, err = describerequest.FormatPrivateSourceRequestCandidate(u, c.DescribeDeadlineMS(), recordCap)
				if err != nil {
					return bad(err.Error())
				}
				records[u.ID] = r
				out.Requests = append(out.Requests, r)
			}
			row.RequestID = r.RecordID
		}
		out.Outcomes = append(out.Outcomes, row)
	}
	var err error
	out.Requests, err = describerequest.CanonicalPrivateSourceRequests(out.Requests)
	if err != nil {
		return bad(err.Error())
	}
	out.Counts.Requests = len(out.Requests)
	if out.Counts.Admitted+out.Counts.Unresolved != out.Counts.Intents || len(out.Outcomes) != out.Counts.Intents {
		return bad("private source outcome imbalance")
	}
	outputUpper := int64(16384 + len(out.Outcomes)*2048)
	for _, u := range out.Units {
		outputUpper += int64(len(u.Body)*6 + 32768)
	}
	for _, r := range out.Requests {
		outputUpper += int64(len(r.Envelope.Prompt)*6 + 32768)
	}
	if outputUpper > capBytes || outputUpper*2+upper > int64(l.MaxParentBytes) {
		return bad("private source encoded output bound")
	}
	ob, err := json.Marshal(out)
	if err != nil {
		return bad(err.Error())
	}
	rb, err := describerequest.Bytes(out.Requests)
	if err != nil {
		return bad(err.Error())
	}
	mats := []material{{"private_source_admissions", ob}, {"private_source_requests", rb}}
	cp := checkpoint{Version: PrivateAllModeSourcesVersion, Stage: string(stop), Status: string(cc.StatusPaused), ClaimCeiling: rg.EvaluationCeilingPrivate, HandoffID: h.ID(), ContractID: c.ID(), PolicyID: q.PolicyID, Workspace: workspace, BaseCheckpoint: base.CheckpointID, Deadline: q.Deadline, Limits: q.Limits}
	total := int64(0)
	for _, m := range mats {
		if len(m.raw) == 0 || int64(len(m.raw)) > capBytes {
			return bad("private source actual object bound")
		}
		total += int64(len(m.raw))
		cp.Artifacts = append(cp.Artifacts, cc.ArtifactRef{Kind: m.kind, ID: digest(m.raw), Digest: digest(m.raw), ByteLength: uint64(len(m.raw))})
	}
	raw, err := json.Marshal(cp)
	if err != nil || int64(len(raw)) > capBytes || total+int64(len(raw)) > int64(l.MaxParentBytes) {
		return bad("private source actual aggregate/checkpoint bound")
	}
	out.CheckpointID = digest(raw)
	mats = append(mats, material{"private_source_checkpoint", raw})
	return out, mats, admissions, nil
}
func paused(out PrivateAllModeSourcesResult) Result {
	return Result{Result: cc.Result{Status: cc.StatusPaused, HandoffID: out.HandoffID, CheckpointID: out.CheckpointID, PrivateStage: out.Stage, RequestCount: out.Counts.Requests, PreparationCount: out.Counts.Admitted}, PrivateSources: &out}
}
func Run(ctx context.Context, req Request) Result {
	if req.PrivateSources == nil {
		return Result{Result: cc.Run(ctx, cc.Request{PrivateAllMode: req.PrivateAllMode, Handoff: req.Handoff, Workspace: req.Workspace, Contract: req.Contract, Worker: req.Worker, WorkerV2: req.WorkerV2, Store: req.Store, FreshCapture: req.FreshCapture, StopAfter: req.StopAfter})}
	}
	if req.PrivateAllMode == nil || req.Handoff.HandoffID() != "" || req.FreshCapture != nil {
		return failure(errors.New("private source exclusive opaque handoff"))
	}
	if err := preflight(ctx, *req.PrivateSources, req.Contract, req.Store, req.Workspace, req.StopAfter); err != nil {
		return failure(err)
	}
	metadata, err := cc.PreparePrivateAllModeSourceIntents(ctx, *req.PrivateAllMode, req.Contract, req.Store, req.Workspace)
	if err != nil {
		return failure(err)
	}
	out, mats, admissions, err := prepare(ctx, metadata, *req.PrivateAllMode, req.Contract, *req.PrivateSources, req.Store, req.Workspace, req.StopAfter)
	if err != nil {
		return failure(err)
	}
	base := cc.Run(ctx, cc.Request{PrivateAllMode: req.PrivateAllMode, Workspace: req.Workspace, Contract: req.Contract, Store: req.Store, StopAfter: cc.StopAfterSourceIntents})
	if base.Err != nil {
		return Result{Result: base}
	}
	if base.CheckpointID != metadata.CheckpointID {
		return failure(errors.New("private source metadata checkpoint mismatch"))
	}
	for _, m := range mats {
		if ctx.Err() != nil {
			return failure(ctx.Err())
		}
		id, err := req.Store.Put(ctx, m.raw)
		if err != nil || id != digest(m.raw) {
			return failure(errors.New("private source immutable Put refused"))
		}
	}
	if ctx.Err() != nil || !time.Now().Before(req.PrivateSources.Deadline) {
		return failure(errors.New("private source cancelled before expectation"))
	}
	q := *req.PrivateSources
	q.Originals = append([]inv.PrivateProducerOriginals(nil), q.Originals...)
	q.Definitions = append([]sessionruntime.PrivateGroupingSourceEvidence(nil), q.Definitions...)
	q.Objects = append([]sourceobject.Object(nil), q.Objects...)
	for n := range q.Objects {
		q.Objects[n].Bytes = append([]byte(nil), q.Objects[n].Bytes...)
	}
	out.Expected = PrivateAllModeSourcesExpected{&expected{*req.PrivateAllMode, req.Contract, q, req.StopAfter, out.CheckpointID, base.CheckpointID, admissions}}
	return paused(out)
}
func Resume(ctx context.Context, req ResumeRequest) Result {
	if req.PrivateSources == nil {
		return Result{Result: cc.Resume(ctx, cc.ResumeRequest{PrivateAllMode: req.PrivateAllMode, Selector: req.Selector, Workspace: req.Workspace, UseCommittedWorkspace: req.UseCommittedWorkspace, Store: req.Store, Worker: req.Worker, WorkerV2: req.WorkerV2, FreshCapture: req.FreshCapture, StopAfter: req.StopAfter})}
	}
	r := req.PrivateSources
	s := r.Expected.state
	if s == nil || r.Version != PrivateAllModeSourcesVersion || req.PrivateAllMode != nil || req.UseCommittedWorkspace || req.FreshCapture != nil || req.StopAfter != s.stop || req.Selector != s.checkpoint || !r.ExpectedHandoff.Same(s.handoff) || len(r.Originals) != len(s.request.Originals) || len(r.Definitions) != len(s.request.Definitions) {
		return failure(errors.New("private source expected/version/selector/handoff mismatch"))
	}
	for n, o := range r.Originals {
		if !o.Same(s.request.Originals[n]) {
			return failure(errors.New("private source opaque original substitution"))
		}
	}
	for n, d := range r.Definitions {
		if d != s.request.Definitions[n] {
			return failure(errors.New("private source opaque definition substitution"))
		}
	}
	if err := preflight(ctx, s.request, s.contract, req.Store, req.Workspace, s.stop); err != nil {
		return failure(err)
	}
	base := cc.Resume(ctx, cc.ResumeRequest{PrivateAllMode: &cc.PrivateAllModeResume{Version: cc.PrivateAllModeVersion, Expected: s.handoff, Contract: s.contract}, Selector: s.baseCheckpoint, Workspace: req.Workspace, Store: req.Store, StopAfter: cc.StopAfterSourceIntents})
	if base.Err != nil {
		return Result{Result: base}
	}
	out, mats, _, err := prepare(ctx, *base.PrivateAllMode, s.handoff, s.contract, s.request, req.Store, req.Workspace, s.stop)
	if err != nil {
		return failure(err)
	}
	if out.CheckpointID != req.Selector {
		return failure(errors.New("private source expected checkpoint"))
	}
	for _, m := range mats {
		if ctx.Err() != nil {
			return failure(ctx.Err())
		}
		raw, err := req.Store.Get(ctx, digest(m.raw))
		if err != nil || len(raw) != len(m.raw) || !bytes.Equal(raw, m.raw) {
			return failure(errors.New("private source persisted bytes mismatch"))
		}
	}
	out.Expected = r.Expected
	return paused(out)
}
