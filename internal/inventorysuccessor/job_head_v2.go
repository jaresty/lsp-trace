package inventorysuccessor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"

	"lsp-trace/internal/publication"
)

const (
	headsNamespaceV2          = "inventory-successor/heads/private/v2"
	authorizationsNamespaceV2 = "inventory-successor/authorizations/private/v2"
	reservationsNamespaceV2   = "inventory-successor/reservations/private/v2"
)

// resourceHeadV2 is ACCOUNTING custody only. No field can express a workflow
// checkpoint, source custody, candidate, Describe readiness or semantic stage.
type resourceHeadV2 struct {
	Schema                string     `json:"schema"`
	Identity              string     `json:"identity"`
	Job                   jobV2      `json:"job"`
	Sequence              uint64     `json:"sequence"`
	Previous              string     `json:"predecessor_resource_head"`
	Authorization         string     `json:"authorization_identity"`
	AuthorizationSelector string     `json:"authorization_selector"`
	ReservationSelector   string     `json:"reservation_prefix_selector"`
	Charged               countersV2 `json:"cumulative_charged"`
	Windows               uint64     `json:"consumed_windows"`
	Attempt               string     `json:"unresolved_attempt"`
	Disposition           string     `json:"disposition"`
	Event                 string     `json:"event"`
	At                    int64      `json:"at_unix_nano"`
}

// Limits are mandatory on EVERY invocation. Hard ceilings bound invocation
// allocation/work independent of declared job ceilings. No partial verification
// is authority. Longer histories stop; there is no checkpoint-proof shortcut.
type verificationLimitsV2 struct{ Records, Bytes, Work uint64 }

var errVerificationStopV2 = errors.New("NONEXECUTING_VERIFICATION_STOP")
var errConflictV2 = errors.New("private v2 resource head conflict")
var errUncertainV2 = errors.New("private v2 resource CAS uncertain; no permission returned")
var errMissingV2 = errors.New("private v2 selected resource head missing")

type casV2 func(context.Context, *publication.Root, string, publication.BoundFilePredecessor, []byte, func([]byte) error) (*publication.CompareAndReplaceReceipt, error)
type kernelV2 struct {
	root *publication.Root
	// Instance-local seams permit controlled fault tests; no shared hooks or
	// alternate provider/runtime execution engine exists.
	cas     casV2
	publish func(publication.Request) publication.Result
}
type verificationBudgetV2 struct{ records, bytes, work uint64 }

func budgetV2(l verificationLimitsV2) (*verificationBudgetV2, error) {
	if l.Records == 0 || l.Records > 4096 || l.Bytes == 0 || l.Bytes > 64<<20 || l.Work == 0 || l.Work > 16384 {
		return nil, errVerificationStopV2
	}
	return &verificationBudgetV2{l.Records, l.Bytes, l.Work}, nil
}
func recordSelectorV2(ns, id string) string {
	return ns + "/" + strings.TrimPrefix(id, "sha256:") + ".json"
}
func currentSelectorV2(j jobV2) string {
	return headsNamespaceV2 + "/current/" + strings.TrimPrefix(j.StableID, "sha256:") + ".json"
}
func initialSelectorV2(j jobV2) string {
	return headsNamespaceV2 + "/initial/" + strings.TrimPrefix(j.StableID, "sha256:") + ".json"
}
func readyV2(h resourceHeadV2) bool { return h.Disposition == "READY" || h.Disposition == "OBSERVED" }
func validJobV2(j jobV2) bool {
	copy := j
	raw, err := sealV2(&copy)
	return err == nil && len(raw) > 0 && copy.Identity == j.Identity
}
func validateResourceHeadV2(h resourceHeadV2) error {
	if h.Schema != resourceHeadSchemaV2 || !validJobV2(h.Job) || h.Sequence == 0 || !linkV2(h.Previous) || !hashV2(h.Authorization) || h.AuthorizationSelector != recordSelectorV2(authorizationsNamespaceV2, h.Authorization) || !linkV2(h.Attempt) || h.At <= 0 {
		return errRecordV2
	}
	if h.ReservationSelector != noneV2 && !selectorV2(h.ReservationSelector, reservationsNamespaceV2) {
		return errRecordV2
	}
	switch h.Event {
	case "INITIALIZE":
		if h.Sequence != 1 || h.Previous != noneV2 || h.Windows != 0 || h.Charged != (countersV2{}) || h.Attempt != noneV2 || h.ReservationSelector != noneV2 || h.Disposition != "READY" {
			return errRecordV2
		}
	case "RENEW":
		if h.Sequence < 2 || !hashV2(h.Previous) || !readyV2(h) {
			return errRecordV2
		}
	case "RESERVE":
		if h.Sequence < 2 || !hashV2(h.Previous) || !hashV2(h.Attempt) || h.Windows == 0 || h.ReservationSelector == noneV2 || h.Disposition != "PENDING" {
			return errRecordV2
		}
	case "OBSERVE":
		if h.Sequence < 3 || !hashV2(h.Previous) || !hashV2(h.Attempt) || h.Windows == 0 || h.ReservationSelector == noneV2 {
			return errRecordV2
		}
		switch h.Disposition {
		case "OBSERVED", "FAILED", "INDETERMINATE":
		default:
			return errRecordV2
		}
	default:
		return errRecordV2
	}
	return nil
}
func selectorV2(s, ns string) bool {
	prefix := ns + "/"
	return strings.HasPrefix(s, prefix) && strings.HasSuffix(s, ".json") && hashV2("sha256:"+strings.TrimSuffix(strings.TrimPrefix(s, prefix), ".json"))
}
func (k *kernelV2) check(ctx context.Context, j jobV2) error {
	if k == nil || k.root == nil || ctx == nil || !validJobV2(j) {
		return errRecordV2
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return k.root.ValidatePrivate()
}
func (k *kernelV2) read(ctx context.Context, selector string, v any, b *verificationBudgetV2) error {
	_, _, limit := metaV2(v)
	if b == nil || b.records == 0 || b.work == 0 || b.bytes < uint64(limit) {
		return errVerificationStopV2
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.records--
	b.work--
	raw, err := k.root.ReadSelector(selector, int64(limit))
	if err != nil {
		return errRecordV2
	}
	b.bytes -= uint64(len(raw))
	if parseV2(raw, v) != nil {
		return errRecordV2
	}
	ns, id, _ := metaV2(v)
	switch ns {
	case authorizationSchemaV2:
		ns = authorizationsNamespaceV2
	case reservationSchemaV2:
		ns = reservationsNamespaceV2
	case resourceHeadSchemaV2:
		ns = headsNamespaceV2
	default:
		return errRecordV2
	}
	if selector != recordSelectorV2(ns, *id) {
		return errRecordV2
	}
	return nil
}
func (k *kernelV2) put(ctx context.Context, ns string, v any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	raw, err := sealV2(v)
	if err != nil {
		return "", err
	}
	schema, id, _ := metaV2(v)
	selector := recordSelectorV2(ns, *id)
	return selector, k.publishBytes(ctx, selector, schema, raw, true)
}
func (k *kernelV2) publishBytes(ctx context.Context, selector, schema string, raw []byte, reuse bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	publish := k.publish
	if publish == nil {
		publish = publication.NewPublisher().Publish
	}
	r := publish(publication.Request{Root: k.root, Selector: selector, ArtifactSchemaID: schema, Bytes: bytes.Clone(raw)})
	if r.Failure != nil {
		if reuse && r.Failure.Code == publication.CodeTargetExists {
			existing, err := k.root.ReadSelector(selector, int64(len(raw)))
			if err == nil && bytes.Equal(existing, raw) {
				return nil
			}
		}
		return errRecordV2
	}
	if r.Receipt == nil || r.Receipt.Digest != digest(raw) || r.Receipt.ByteLength != uint64(len(raw)) || r.Receipt.ArtifactSchemaID != schema {
		return errRecordV2
	}
	return nil
}
func (k *kernelV2) selected(ctx context.Context, j jobV2, b *verificationBudgetV2) (resourceHeadV2, []byte, error) {
	if b.records == 0 || b.work == 0 || b.bytes < headLimitV2 {
		return resourceHeadV2{}, nil, errVerificationStopV2
	}
	if err := ctx.Err(); err != nil {
		return resourceHeadV2{}, nil, err
	}
	b.records--
	b.work--
	raw, err := k.root.ReadSelectorWithAbsence(currentSelectorV2(j), headLimitV2)
	if errors.Is(err, os.ErrNotExist) {
		return resourceHeadV2{}, nil, errMissingV2
	}
	if err != nil {
		return resourceHeadV2{}, nil, errRecordV2
	}
	b.bytes -= uint64(len(raw))
	var h resourceHeadV2
	if parseV2(raw, &h) != nil || h.Job.Identity != j.Identity {
		return resourceHeadV2{}, nil, errRecordV2
	}
	return h, raw, nil
}

// verify streams the exact selected predecessor chain. The bounded set stores
// only attempt digests for replay rejection (not source, bodies, or Results).
func (k *kernelV2) verify(ctx context.Context, j jobV2, b *verificationBudgetV2, attempt string) (resourceHeadV2, []byte, error) {
	if err := k.check(ctx, j); err != nil {
		return resourceHeadV2{}, nil, err
	}
	selected, raw, err := k.selected(ctx, j, b)
	if err != nil {
		return resourceHeadV2{}, nil, err
	}
	h := selected
	seen := make(map[string]struct{})
	for {
		var a authorizationV2
		if err = k.read(ctx, h.AuthorizationSelector, &a, b); err != nil {
			return resourceHeadV2{}, nil, err
		}
		if a.Job != j.Identity || a.Identity != h.Authorization || !h.Charged.within(a.Ceiling) || h.Windows > a.MaxWindows {
			return resourceHeadV2{}, nil, errRecordV2
		}
		if h.Event == "INITIALIZE" {
			if a.Revision != 1 || a.Previous != noneV2 || a.BoundHead != noneV2 || a.Charged != (countersV2{}) || h.At >= a.Expires {
				return resourceHeadV2{}, nil, errRecordV2
			}
			// The no-replace marker is also retained evidence, not a hash-only hint.
			if b.records == 0 || b.work == 0 || b.bytes < headLimitV2 {
				return resourceHeadV2{}, nil, errVerificationStopV2
			}
			b.records--
			b.work--
			marker, e := k.root.ReadSelector(initialSelectorV2(j), headLimitV2)
			if e != nil {
				return resourceHeadV2{}, nil, errRecordV2
			}
			b.bytes -= uint64(len(marker))
			encoded, e := sealV2(&h)
			if e != nil || !bytes.Equal(marker, encoded) {
				return resourceHeadV2{}, nil, errRecordV2
			}
			return selected, raw, nil
		}
		var p resourceHeadV2
		if err = k.read(ctx, recordSelectorV2(headsNamespaceV2, h.Previous), &p, b); err != nil {
			return resourceHeadV2{}, nil, err
		}
		n, e := nextV2(p.Sequence)
		if e != nil || n != h.Sequence || p.Identity != h.Previous || p.Job.Identity != j.Identity || h.At < p.At {
			return resourceHeadV2{}, nil, errRecordV2
		}
		if err = k.verifyStep(ctx, h, p, a, b); err != nil {
			return resourceHeadV2{}, nil, err
		}
		if h.Event == "RESERVE" {
			if h.Attempt == attempt {
				return resourceHeadV2{}, nil, errConflictV2
			}
			if _, ok := seen[h.Attempt]; ok {
				return resourceHeadV2{}, nil, errRecordV2
			}
			seen[h.Attempt] = struct{}{}
		}
		h = p
	}
}
func (k *kernelV2) verifyStep(ctx context.Context, h, p resourceHeadV2, a authorizationV2, b *verificationBudgetV2) error {
	switch h.Event {
	case "RENEW":
		var old authorizationV2
		if err := k.read(ctx, p.AuthorizationSelector, &old, b); err != nil {
			return err
		}
		if !readyV2(p) || h.Disposition != p.Disposition || h.Attempt != p.Attempt || h.ReservationSelector != p.ReservationSelector || h.Charged != p.Charged || h.Windows != p.Windows || validRenewalV2(a, old, p, h.At) != nil {
			return errRecordV2
		}
	case "RESERVE":
		var r reservationV2
		if err := k.read(ctx, h.ReservationSelector, &r, b); err != nil {
			return err
		}
		charged, err := addCountersV2(p.Charged, r.Tranche)
		windows, e := nextV2(p.Windows)
		if !readyV2(p) || h.Authorization != p.Authorization || r.Job != h.Job.Identity || r.Authorization != a.Identity || r.Revision != a.Revision || r.PreviousHead != p.Identity || r.Sequence != h.Sequence || r.Attempt != h.Attempt || r.ClaimedAt != h.At || charged != h.Charged || r.Charged != h.Charged || windows != h.Windows || err != nil || e != nil || validTrancheV2(a, r.Tranche, r.ClaimedAt, r.Deadline) != nil {
			return errRecordV2
		}
	case "OBSERVE":
		var r reservationV2
		if err := k.read(ctx, h.ReservationSelector, &r, b); err != nil {
			return err
		}
		if p.Disposition != "PENDING" || h.Authorization != p.Authorization || h.ReservationSelector != p.ReservationSelector || h.Charged != p.Charged || h.Windows != p.Windows || h.Attempt != p.Attempt || r.Attempt != h.Attempt || r.Job != h.Job.Identity || r.Authorization != h.Authorization {
			return errRecordV2
		}
		if h.Disposition == "OBSERVED" && h.At >= r.Deadline {
			return errRecordV2
		}
	default:
		return errRecordV2
	}
	return nil
}
func validTrancheV2(a authorizationV2, c countersV2, now, deadline int64) error {
	if c == (countersV2{}) || !c.within(a.MaxTranche) || now <= 0 || now >= a.Expires || deadline <= now || deadline > a.Expires || uint64(deadline-now) > a.MaxWindowNS {
		return errRecordV2
	}
	return nil
}
func validRenewalV2(a, old authorizationV2, h resourceHeadV2, now int64) error {
	n, err := nextV2(old.Revision)
	if err != nil || a.Job != h.Job.Identity || a.Revision != n || a.Previous != old.Identity || a.BoundHead != h.Identity || a.Charged != h.Charged || now >= old.Expires || now >= a.Expires || !old.Ceiling.within(a.Ceiling) || !old.MaxTranche.within(a.MaxTranche) || a.MaxWindows < old.MaxWindows || a.MaxWindowNS < old.MaxWindowNS || a.Expires < old.Expires {
		return errRecordV2
	}
	return nil
}
func approvedV2(j jobV2, a authorizationV2, d decisionV2) bool {
	return d.Job == j.Identity && d.Authorization == a.Identity && d.Reference == a.Decision && hashV2(d.Reference)
}
func (k *kernelV2) commit(ctx context.Context, j jobV2, oldRaw []byte, h resourceHeadV2) (resourceHeadV2, error) {
	raw, err := sealV2(&h)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if _, err = k.put(ctx, headsNamespaceV2, &h); err != nil {
		return resourceHeadV2{}, err
	}
	predecessor := publication.BoundFilePredecessor{Absent: len(oldRaw) == 0}
	if len(oldRaw) > 0 {
		predecessor = publication.BoundFilePredecessor{Selector: currentSelectorV2(j), Digest: digest(oldRaw), ByteLength: uint64(len(oldRaw))}
	}
	if err = ctx.Err(); err != nil {
		return resourceHeadV2{}, err
	}
	cas := k.cas
	if cas == nil {
		cas = publication.CompareAndReplaceBoundFile
	}
	receipt, err := cas(ctx, k.root, currentSelectorV2(j), predecessor, raw, func(b []byte) error { var x resourceHeadV2; return parseV2(b, &x) })
	if receipt != nil && receipt.Committed {
		// A committed+error (including post-commit cancellation) is NEVER success.
		if err != nil || receipt.Outcome != publication.CASOutcomeCommitted {
			return resourceHeadV2{}, errUncertainV2
		}
		return h, nil
	}
	if err != nil {
		return resourceHeadV2{}, err
	}
	return resourceHeadV2{}, errConflictV2
}
func (k *kernelV2) initialize(ctx context.Context, j jobV2, a authorizationV2, d decisionV2, now int64, l verificationLimitsV2) (resourceHeadV2, error) {
	if err := k.check(ctx, j); err != nil {
		return resourceHeadV2{}, err
	}
	b, err := budgetV2(l)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if b.records < 2 || b.work < 2 || b.bytes < 2*headLimitV2 {
		return resourceHeadV2{}, errVerificationStopV2
	}
	copy := a
	if _, err := sealV2(&copy); err != nil || copy.Identity != a.Identity || a.Job != j.Identity || a.Revision != 1 || !approvedV2(j, a, d) || now <= 0 || now >= a.Expires {
		return resourceHeadV2{}, errRecordV2
	}
	// Confirmed absence only. Malformed, foreign and old-v1 heads refuse; no
	// import/reset path exists. Marker survives missing/torn selected-head state.
	if _, err := k.root.ReadSelectorWithAbsence(currentSelectorV2(j), headLimitV2); !errors.Is(err, os.ErrNotExist) {
		return resourceHeadV2{}, errConflictV2
	}
	if _, err := k.root.ReadSelectorWithAbsence(initialSelectorV2(j), headLimitV2); !errors.Is(err, os.ErrNotExist) {
		return resourceHeadV2{}, errConflictV2
	}
	as, err := k.put(ctx, authorizationsNamespaceV2, &a)
	if err != nil {
		return resourceHeadV2{}, err
	}
	h := resourceHeadV2{Schema: resourceHeadSchemaV2, Job: j, Sequence: 1, Previous: noneV2, Authorization: a.Identity, AuthorizationSelector: as, ReservationSelector: noneV2, Attempt: noneV2, Disposition: "READY", Event: "INITIALIZE", At: now}
	raw, err := sealV2(&h)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if err = k.publishBytes(ctx, initialSelectorV2(j), resourceHeadSchemaV2, raw, false); err != nil {
		return resourceHeadV2{}, err
	}
	return k.commit(ctx, j, nil, h)
}
func (k *kernelV2) reopen(ctx context.Context, j jobV2, l verificationLimitsV2) (resourceHeadV2, error) {
	b, err := budgetV2(l)
	if err != nil {
		return resourceHeadV2{}, err
	}
	h, _, err := k.verify(ctx, j, b, noneV2)
	return h, err
}
func (k *kernelV2) reserve(ctx context.Context, j jobV2, expected, attempt string, c countersV2, deadline, now int64, l verificationLimitsV2) (resourceHeadV2, error) {
	if !hashV2(expected) || !hashV2(attempt) {
		return resourceHeadV2{}, errRecordV2
	}
	b, err := budgetV2(l)
	if err != nil {
		return resourceHeadV2{}, err
	}
	p, raw, err := k.verify(ctx, j, b, attempt)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if p.Identity != expected || !readyV2(p) || now < p.At {
		return resourceHeadV2{}, errConflictV2
	}
	a, err := k.currentAuthorization(ctx, p, b)
	if err != nil {
		return resourceHeadV2{}, err
	}
	charged, err := addCountersV2(p.Charged, c)
	seq, e := nextV2(p.Sequence)
	windows, we := nextV2(p.Windows)
	if err != nil || e != nil || we != nil || !charged.within(a.Ceiling) || windows > a.MaxWindows || validTrancheV2(a, c, now, deadline) != nil {
		return resourceHeadV2{}, errRecordV2
	}
	r := reservationV2{Schema: reservationSchemaV2, Job: j.Identity, Authorization: a.Identity, Revision: a.Revision, PreviousHead: p.Identity, Sequence: seq, Attempt: attempt, Phase: "RESOURCE_WINDOW", Tranche: c, Charged: charged, ClaimedAt: now, Deadline: deadline, Disposition: "PENDING"}
	selector, err := k.put(ctx, reservationsNamespaceV2, &r)
	if err != nil {
		return resourceHeadV2{}, err
	}
	h := p
	h.Identity = ""
	h.Sequence = seq
	h.Previous = p.Identity
	h.ReservationSelector = selector
	h.Charged = charged
	h.Windows = windows
	h.Attempt = attempt
	h.Disposition = "PENDING"
	h.Event = "RESERVE"
	h.At = now
	return k.commit(ctx, j, raw, h)
}
func (k *kernelV2) currentAuthorization(ctx context.Context, h resourceHeadV2, b *verificationBudgetV2) (authorizationV2, error) {
	var a authorizationV2
	err := k.read(ctx, h.AuthorizationSelector, &a, b)
	return a, err
}
func (k *kernelV2) observe(ctx context.Context, j jobV2, expected, attempt, disposition string, now int64, l verificationLimitsV2) (resourceHeadV2, error) {
	if disposition != "OBSERVED" && disposition != "FAILED" && disposition != "INDETERMINATE" {
		return resourceHeadV2{}, errRecordV2
	}
	b, err := budgetV2(l)
	if err != nil {
		return resourceHeadV2{}, err
	}
	p, raw, err := k.verify(ctx, j, b, noneV2)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if p.Identity != expected || p.Disposition != "PENDING" || p.Attempt != attempt || now < p.At {
		return resourceHeadV2{}, errConflictV2
	}
	var r reservationV2
	if err = k.read(ctx, p.ReservationSelector, &r, b); err != nil {
		return resourceHeadV2{}, err
	}
	if disposition == "OBSERVED" && now >= r.Deadline {
		return resourceHeadV2{}, errRecordV2
	}
	seq, err := nextV2(p.Sequence)
	if err != nil {
		return resourceHeadV2{}, err
	}
	h := p
	h.Identity = ""
	h.Sequence = seq
	h.Previous = p.Identity
	h.Disposition = disposition
	h.Event = "OBSERVE"
	h.At = now
	return k.commit(ctx, j, raw, h)
}
func (k *kernelV2) renew(ctx context.Context, j jobV2, expected string, a authorizationV2, d decisionV2, now int64, l verificationLimitsV2) (resourceHeadV2, error) {
	b, err := budgetV2(l)
	if err != nil {
		return resourceHeadV2{}, err
	}
	p, raw, err := k.verify(ctx, j, b, noneV2)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if p.Identity != expected || !readyV2(p) || now < p.At {
		return resourceHeadV2{}, errConflictV2
	}
	copy := a
	if _, err = sealV2(&copy); err != nil || copy.Identity != a.Identity || !approvedV2(j, a, d) {
		return resourceHeadV2{}, errRecordV2
	}
	old, err := k.currentAuthorization(ctx, p, b)
	if err != nil {
		return resourceHeadV2{}, err
	}
	if validRenewalV2(a, old, p, now) != nil {
		return resourceHeadV2{}, errRecordV2
	}
	selector, err := k.put(ctx, authorizationsNamespaceV2, &a)
	if err != nil {
		return resourceHeadV2{}, err
	}
	seq, err := nextV2(p.Sequence)
	if err != nil {
		return resourceHeadV2{}, err
	}
	h := p
	h.Identity = ""
	h.Sequence = seq
	h.Previous = p.Identity
	h.Authorization = a.Identity
	h.AuthorizationSelector = selector
	h.Event = "RENEW"
	h.At = now
	return k.commit(ctx, j, raw, h)
}
