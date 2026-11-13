// Package applicationinventory schedules a bounded supplied census. A supplied
// lead manifest is not proof of whole-repository enumeration. Callbacks are
// explicit caller configuration, not self-authenticated grants. They must obey
// context and avoid retaining arguments; arbitrary Go callbacks are NOT OS-contained.
package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
)

var ErrInventoryV3 = errors.New("application inventory v3 refused")

type Lead struct{ ID, Provenance string }
type Manifest struct {
	Scope, Provenance string
	Leads             []Lead
	UntouchedGaps     []string
}
type Stage struct{ Requested, Successful, Failed int }
type Disposition struct{ LeadID, Status, Reason string }
type Report struct {
	Version, Scope, Provenance, Status, Feature, Consumer, Completeness, Absence string
	Authority                                                                    int
	Accepted                                                                     bool
	UntouchedGaps                                                                []string
	Census                                                                       []Lead // admitted snapshot, detached from caller mutation; not enumeration proof
	Dispositions                                                                 []Disposition
	Stages                                                                       map[string]Stage
	SuccessfulDescriptions, WithMembership, WithoutMembership                    int
	CallerBytes                                                                  uint64
	SemanticReservedNS                                                           int64
	CallbackContainment                                                          string
	CallerAccounting                                                             string
	CatalogRevision                                                              string
	Published                                                                    bool
}
type NativeResult struct {
	Selected sc.PrivateSelectedResultV2
	Status   string
}
type NativePreparer func(context.Context, Lead) (NativeResult, error)

// ByteExchange receives bounded detached input and a pre-reserved bounded writer.
// Output overflow poisons the exchange even if a callback ignores Write's error.
type ByteExchange func(context.Context, []byte, io.Writer) error
type Request struct {
	Manifest                              Manifest
	Preparer                              NativePreparer
	DescribeHost, GroupDecision, Reviewer ByteExchange
	Interpreter, ReviewerIdentity         string
	Pins                                  dw.HostPinsV2
	Destination                           string        // newly created owned directory; never overwrite predecessors
	MaxNativeAttempts                     int           // zero disables acquisition, never implicit live action
	NativeRetainedUpper                   uint64        // required pre-effects caller quote for preparer output; <=64MiB
	BudgetBytes                           uint64        // zero selects the quoted64MiB caller ceiling, not native credit
	HostWindow                            time.Duration // zero selects30s; cumulative reservations<=120s
	ResponseLimits                        dw.SuppliedResponseLimits
}
type boundedOutputV3 struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	max    int
	err    error
	closed bool
}

func (w *boundedOutputV3) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, ErrInventoryV3
	}
	if w.err != nil {
		return 0, w.err
	}
	if len(p) > w.max-w.buffer.Len() {
		w.err = ErrInventoryV3
		return 0, w.err
	}
	return w.buffer.Write(p)
}
func (w *boundedOutputV3) finish() ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.err != nil {
		return nil, w.err
	}
	return append([]byte(nil), w.buffer.Bytes()...), nil
}
func exchangeV3(ctx context.Context, b *sc.CallerBudgetV3, fn ByteExchange, in []byte, max int, window time.Duration) ([]byte, error) {
	if fn == nil || len(in) > 8<<20 || max < 1 || max > 8<<20 || b.Reserve(ctx, uint64(len(in))*4+uint64(max)*16+4096) != nil {
		return nil, ErrInventoryV3
	}
	c, cancel, e := b.Exchange(ctx, window)
	if e != nil {
		return nil, e
	}
	defer cancel()
	w := &boundedOutputV3{max: max}
	input := append([]byte(nil), in...)
	e = fn(c, input, w)
	raw, outputErr := w.finish()
	if e != nil || outputErr != nil || c.Err() != nil {
		return nil, errors.Join(e, outputErr, c.Err())
	}
	return raw, nil
}
func Run(ctx context.Context, q Request) (out Report, err error) {
	out = Report{Version: "lsp-trace.application-inventory.v3", Status: "PROVISIONAL", Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", CallbackContainment: "COOPERATIVE_NOT_OS_PHYSICAL", Stages: map[string]Stage{}}
	for _, name := range []string{"census", "native_preparation", "describe", "group_decision", "review", "catalog", "publication"} {
		out.Stages[name] = Stage{}
	}
	if ctx == nil || ctx.Err() != nil {
		return out, ErrInventoryV3
	}
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	if len(q.Manifest.Leads) > 10000 || len(q.Manifest.UntouchedGaps) > 10000 || q.MaxNativeAttempts < 0 || q.MaxNativeAttempts > 100 || len(q.Manifest.Scope) == 0 || len(q.Manifest.Scope) > 4096 || len(q.Manifest.Provenance) == 0 || len(q.Manifest.Provenance) > 4096 {
		return out, ErrInventoryV3
	}
	if q.BudgetBytes == 0 {
		q.BudgetBytes = 64 << 20
	}
	deadline, _ := ctx.Deadline()
	budget, e := sc.NewCallerBudgetV3(deadline, q.BudgetBytes)
	if e != nil {
		return out, e
	}
	defer func() {
		n, s := budget.Usage()
		out.CallerBytes = n
		out.SemanticReservedNS = int64(s)
		if ctx.Err() != nil {
			out.Published = false
			err = errors.Join(err, ctx.Err())
		}
	}()
	// Reserve all admitted lead storage, validation and eventual report copies before
	// snapshot; counts/lengths are bounded before loops. Duplicates reject admission.
	var n uint64 = 32768 + uint64(len(q.Manifest.Leads))*1024 + uint64(len(q.Manifest.UntouchedGaps))*128
	for _, l := range q.Manifest.Leads {
		if len(l.ID) == 0 || len(l.ID) > 256 || len(l.Provenance) > 4096 {
			return out, ErrInventoryV3
		}
		n += uint64(len(l.ID)+len(l.Provenance)) * 12
	}
	for _, g := range q.Manifest.UntouchedGaps {
		if len(g) > 4096 {
			return out, ErrInventoryV3
		}
		n += uint64(len(g)) * 12
	}
	if budget.Reserve(ctx, n) != nil {
		return out, ErrInventoryV3
	}
	manifest := q.Manifest
	manifest.Leads = append([]Lead(nil), manifest.Leads...)
	manifest.UntouchedGaps = append([]string(nil), manifest.UntouchedGaps...)
	seen := map[string]bool{}
	for _, l := range manifest.Leads {
		if seen[l.ID] {
			return out, ErrInventoryV3
		}
		seen[l.ID] = true
	}
	out.Scope = manifest.Scope
	out.Provenance = manifest.Provenance
	out.UntouchedGaps = manifest.UntouchedGaps
	out.Census = manifest.Leads // existing pre-reserved immutable admission snapshot
	for _, l := range manifest.Leads {
		out.Dispositions = append(out.Dispositions, Disposition{l.ID, "UNTOUCHED", "NOT_ATTEMPTED"})
	}
	stage := func(name string, ok bool) {
		s := out.Stages[name]
		s.Requested++
		if ok {
			s.Successful++
		} else {
			s.Failed++
		}
		out.Stages[name] = s
	}
	for range manifest.Leads {
		stage("census", true)
	}
	if q.Preparer == nil || q.MaxNativeAttempts == 0 {
		return out, ErrInventoryV3
	}
	if q.HostWindow == 0 {
		q.HostWindow = 30 * time.Second
	}
	if q.HostWindow <= 0 || q.HostWindow > 30*time.Second || len(q.Interpreter) == 0 || len(q.Interpreter) > 256 || q.NativeRetainedUpper == 0 || q.NativeRetainedUpper > 64<<20 {
		return out, ErrInventoryV3
	}
	if q.ResponseLimits == (dw.SuppliedResponseLimits{}) {
		q.ResponseLimits = dw.SuppliedResponseLimits{MaxAdapterBytes: 64 << 10, MaxSemanticBytes: 16 << 10}
	}
	if !q.ResponseLimits.Valid() {
		return out, ErrInventoryV3
	}
	descriptions := []sc.BoundSelectedDescriptionV3{}
	for i, l := range manifest.Leads {
		if i >= q.MaxNativeAttempts || ctx.Err() != nil {
			break
		}
		if budget.Exhausted() {
			err = ErrInventoryV3
			break
		}
		if budget.Reserve(ctx, q.NativeRetainedUpper+4096) != nil {
			err = ErrInventoryV3
			break
		}
		out.Dispositions[i] = Disposition{l.ID, "PREPARATION_FAILED", "CALLER_PREPARER_REFUSED"}
		native, e := q.Preparer(ctx, l)
		if e != nil || ctx.Err() != nil {
			stage("native_preparation", false)
			continue
		}
		if native.Status == "OBSERVED_EMPTY" || native.Status == "UNAVAILABLE" {
			stage("native_preparation", true)
			out.Dispositions[i] = Disposition{l.ID, native.Status, "NATIVE_PREPARER_REPORTED_NOT_WHOLE_REPOSITORY"}
			continue
		}
		if native.Status != "SELECTED" || !native.Selected.Available() {
			stage("native_preparation", false)
			continue
		}
		retained, quoteErr := native.Selected.CallerRetentionUpperV3(ctx, budget)
		if quoteErr != nil || retained > q.NativeRetainedUpper {
			stage("native_preparation", false)
			out.Dispositions[i] = Disposition{l.ID, "PREPARATION_FAILED", "CALLER_PREPARER_OUTPUT_QUOTE_EXCEEDED"}
			err = ErrInventoryV3
			break
		}
		stage("native_preparation", true)
		// Pins must match this exact native prompt. They are supplied by configuration,
		// not authenticated execution. No source body is reacquired or rehashed here.
		_, _, r, _ := native.Selected.Selected()
		pins := q.Pins
		pins.PromptSHA256 = "sha256:" + r.Envelope.InputSHA256
		attempt, e := sc.PrepareSelectedDescribeV3(ctx, native.Selected, sc.CallerDescribeBindingV3{AttemptID: l.ID, Interpreter: q.Interpreter, Pins: pins, Deadline: time.Now().Add(q.HostWindow), Limits: q.ResponseLimits, Budget: budget})
		if e != nil {
			stage("describe", false)
			out.Dispositions[i] = Disposition{l.ID, "DESCRIBE_FAILED", "OPAQUE_JOIN_OR_CALLER_LIMIT"}
			continue
		}
		request, e := attempt.Request(ctx)
		if e != nil {
			stage("describe", false)
			out.Dispositions[i] = Disposition{l.ID, "DESCRIBE_FAILED", "CALLER_LIMIT"}
			continue
		}
		if budget.Reserve(ctx, uint64(len(request.Envelope.Prompt))*12+8192) != nil {
			stage("describe", false)
			out.Dispositions[i] = Disposition{l.ID, "DESCRIBE_FAILED", "CALLER_LIMIT"}
			err = ErrInventoryV3
			break
		}
		raw, e := json.Marshal(request)
		if e == nil {
			raw, e = exchangeV3(ctx, budget, q.DescribeHost, raw, q.ResponseLimits.MaxAdapterBytes, q.HostWindow)
		}
		// Consume attempts even on callback refusal/cancellation, without exposing data.
		bound, bindErr := sc.CompleteSelectedDescribeV3(ctx, attempt, raw)
		if e != nil || bindErr != nil {
			stage("describe", false)
			out.Dispositions[i] = Disposition{l.ID, "DESCRIBE_FAILED", "STRICT_HOST_OR_BINDER_REFUSED"}
			continue
		}
		stage("describe", true)
		descriptions = append(descriptions, bound)
		out.SuccessfulDescriptions++
		out.WithoutMembership++ // remains unmatched on any later stage refusal
		out.Dispositions[i] = Disposition{l.ID, "DESCRIBED_UNMATCHED", "NO_SEMANTIC_MEMBERSHIP"}
	}
	// Exhausted/cancelled paths keep all unvisited lead dispositions; they cannot
	// report delivery success. Stage conservation remains explicit on every path.
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if err != nil {
		return out, err
	}
	nativeDescriptions := make([]cat.NativeBoundDescriptionV3, len(descriptions))
	for i, d := range descriptions {
		nativeDescriptions[i] = d
	}
	base, e := cat.BuildNativeV3(ctx, budget, q.Interpreter, nativeDescriptions)
	stage("catalog", e == nil)
	if e != nil {
		return out, e
	}
	baseRaw, e := base.Bytes(ctx)
	if e != nil {
		return out, e
	}
	groups, e := exchangeV3(ctx, budget, q.GroupDecision, baseRaw, 64<<10, q.HostWindow)
	if e != nil {
		stage("group_decision", false)
		return out, e
	}
	proposed, e := cat.ApplyNativeDecisionV3(ctx, base, groups, false)
	stage("group_decision", e == nil)
	stage("catalog", e == nil)
	if e != nil {
		return out, e
	}
	if q.ReviewerIdentity == q.Interpreter || len(q.ReviewerIdentity) == 0 || len(q.ReviewerIdentity) > 256 {
		stage("review", false)
		return out, ErrInventoryV3
	}
	proposedRaw, e := proposed.Bytes(ctx)
	if e != nil {
		return out, e
	}
	reviewedRaw, e := exchangeV3(ctx, budget, q.Reviewer, proposedRaw, 64<<10, q.HostWindow)
	if e != nil {
		stage("review", false)
		return out, e
	}
	if budget.Reserve(ctx, uint64(len(reviewedRaw))*8+4096) != nil {
		stage("review", false)
		return out, ErrInventoryV3
	}
	var decision cat.NativeDecisionV3
	if cat.DecodeNativeV3(reviewedRaw, &decision) != nil || decision.Actor != q.ReviewerIdentity {
		stage("review", false)
		return out, ErrInventoryV3
	}
	corrected, e := cat.ApplyNativeDecisionV3(ctx, proposed, reviewedRaw, true)
	stage("review", e == nil)
	stage("catalog", e == nil)
	if e != nil {
		return out, e
	}
	// Stage a charged reconciliation; no corrected report state escapes until
	// every fallible read and per-lead/count check below has succeeded.
	withMembership, withoutMembership := corrected.MembershipCounts()
	revision := corrected.Revision()
	if budget.Reserve(ctx, uint64(len(out.Dispositions))*192+uint64(len(descriptions))*1024+8192) != nil {
		return out, ErrInventoryV3
	}
	reconciled := append([]Disposition(nil), out.Dispositions...)
	finalRaw, e := corrected.Bytes(ctx)
	if e != nil {
		return out, e
	}
	var final cat.NativeCatalogWireV3
	if budget.Reserve(ctx, uint64(len(finalRaw))*8+4096) != nil || cat.DecodeNativeV3(finalRaw, &final) != nil {
		return out, ErrInventoryV3
	}
	membership := map[string]bool{}
	for _, g := range final.Groups {
		for _, id := range g.Members {
			membership[id] = true
		}
	}
	// Description order follows processing, evidence order is deterministic by ID.
	j := 0
	for i := range reconciled {
		if reconciled[i].Status == "DESCRIBED_UNMATCHED" {
			if j >= len(descriptions) {
				return out, ErrInventoryV3
			}
			ev, e := descriptions[j].Evidence(ctx)
			if e != nil {
				return out, e
			}
			j++
			if membership[ev.ID] {
				reconciled[i].Status = "DESCRIBED_WITH_MEMBERSHIP"
				reconciled[i].Reason = "SUPPLIED_SEMANTIC_PROPOSAL_REVIEWED_UNAUTHENTICATED"
			}
		}
	}
	if len(reconciled) != len(out.Census) || j != len(descriptions) {
		return out, ErrInventoryV3
	}
	matched, unmatched := 0, 0
	for i, d := range reconciled {
		if d.LeadID != out.Census[i].ID {
			return out, ErrInventoryV3
		}
		switch d.Status {
		case "DESCRIBED_WITH_MEMBERSHIP":
			matched++
		case "DESCRIBED_UNMATCHED":
			unmatched++
		}
	}
	if matched != withMembership || unmatched != withoutMembership || matched+unmatched != out.SuccessfulDescriptions {
		return out, ErrInventoryV3
	}
	// One report-state commit; there is no intervening fallible operation.
	out.WithMembership, out.WithoutMembership, out.CatalogRevision, out.Dispositions = withMembership, withoutMembership, revision, reconciled
	// Reserve reporting, all publication bytes, write and readback before effects.
	if budget.Reserve(ctx, n+uint64(len(finalRaw))*2+32768) != nil {
		return out, ErrInventoryV3
	}
	out.CallerAccounting = "PRE_PUBLICATION_SNAPSHOT_FINAL_IN_MANIFEST"
	out.Published = true
	stage("publication", true)
	callerBytes, semanticReserved := budget.Usage()
	out.CallerBytes, out.SemanticReservedNS = callerBytes, int64(semanticReserved)
	report, e := json.Marshal(out)
	if e != nil {
		out.Published = false
		out.Stages["publication"] = Stage{Requested: 1, Failed: 1}
		return out, e
	}
	_, e = PublishV3(ctx, q.Destination, []ArtifactV3{{"catalog-base.json", baseRaw}, {"catalog-proposed.json", proposedRaw}, {"catalog-corrected.json", finalRaw}, {"report.json", report}}, budget)
	if e != nil {
		out.Published = false
		out.Stages["publication"] = Stage{Requested: 1, Failed: 1}
		return out, e
	}
	out.Published = true
	return out, nil
}
