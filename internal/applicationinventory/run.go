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
	// Durable ledger snapshots include conservative reservations, including unused
	// claimed slices. They are not refunded by a later failure or process exit.
	PublicationRunReservedBytes uint64 `json:",omitempty"`
	PublicationRunWork          uint64 `json:",omitempty"`
	PublicationRunPackets       uint64 `json:",omitempty"`
	SemanticReservedNS          int64
	CallbackContainment         string
	CallerAccounting            string
	CatalogRevision             string
	Published                   bool
	ReviewMode                  cat.ReviewModeV3 `json:",omitempty"`
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
	ReviewMode                            cat.ReviewModeV3 `json:",omitempty"`
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
	out = initialInventoryReport()
	if ctx == nil || ctx.Err() != nil || !q.ReviewMode.Valid() {
		return out, ErrInventoryV3
	}
	out.ReviewMode = q.ReviewMode
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
	var n uint64
	out, n, e = admitInventoryManifest(ctx, q, budget, out)
	if e != nil {
		return out, e
	}
	manifest := Manifest{Scope: out.Scope, Provenance: out.Provenance, Leads: out.Census, UntouchedGaps: out.UntouchedGaps}
	stage := func(name string, ok bool) { recordInventoryStage(&out, name, ok) }
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
		descriptions = append(descriptions, bound)
		recordInventoryDescription(&out, i)
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
	base, e := cat.BuildNativeV3(ctx, budget, q.Interpreter, nativeDescriptions, q.ReviewMode)
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
	proposed, e := applyInventoryDecision(ctx, budget, base, groups, false, q.Interpreter)
	stage("group_decision", e == nil)
	stage("catalog", e == nil)
	if e != nil {
		return out, e
	}
	if (q.ReviewerIdentity == q.Interpreter && !q.ReviewMode.AllowsSameActor()) || len(q.ReviewerIdentity) == 0 || len(q.ReviewerIdentity) > 256 {
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
	// Shared transition owns the single charged decode and exact reviewer check.
	corrected, e := applyInventoryDecision(ctx, budget, proposed, reviewedRaw, true, q.ReviewerIdentity)
	stage("review", e == nil)
	stage("catalog", e == nil)
	if e != nil {
		return out, e
	}
	out, finalRaw, e := reconcileInventory(ctx, budget, out, descriptions, corrected)
	if e != nil {
		return out, e
	}
	return publishInventory(ctx, q, budget, out, n, baseRaw, proposedRaw, finalRaw)
}
