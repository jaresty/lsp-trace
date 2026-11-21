//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

// PrepareRecursiveFolder owns one fresh metadata-only preparation traversal.
// Neither the accounting proof nor the returned records grant execution.
func PrepareRecursiveFolder(ctx context.Context, root *publication.Root, i PrivatePreparationIntentV1) (records []source.Record, err error) {
	const count = 8
	const maxScheduled = 16
	const byteCap = 64 << 20
	const workCap = 1 << 32
	// Reserve independent verification capacity per scheduled batch; the
	// fixed probe budget also covers the initial accounting objects. If a
	// history needs more, verification stops rather than extending the quote.
	v := PrivateManagedInputVerificationV2{Records: 8 * maxScheduled, Work: 8 * maxScheduled, Bytes: (32 << 10) * maxScheduled}
	id := PrivatePreparationIntentIdentityV1(i)
	now := time.Now()
	if id == "" || ctx == nil || ctx.Err() != nil || !now.Before(i.Deadline) || i.Deadline.Sub(now) > 15*time.Second {
		return nil, ErrPrivatePreparationEnumerationV1
	}
	quoteIntent := i
	// One opened-directory record plus at most count entries; path queue work
	// is bounded separately by the cursor's cumulative traversal limits.
	quoteIntent.Limits.MaxEntries = count + 1
	quoteIntent.Limits.MaxAccepted = count + 1
	quoteIntent.Limits.MaxWork = count + len(i.Roots) + 1
	q, ok := PrivatePreparationEnumerationQuoteV1(quoteIntent, v, now)
	if !ok {
		return nil, ErrPrivatePreparationEnumerationV1
	}
	windows := min(uint64(maxScheduled), byteCap/q[0], workCap/q[7], uint64(i.Limits.MaxWork))
	if windows < 2 {
		return nil, ErrPrivatePreparationEnumerationV1
	}
	q[8] /= windows
	if q[8] == 0 {
		return nil, ErrPrivatePreparationEnumerationV1
	}
	charge := countersFromV2(q)
	var ceiling countersV2
	for n := uint64(0); n < windows; n++ {
		ceiling, err = addCountersV2(ceiling, charge)
		if err != nil {
			return nil, err
		}
	}
	if ceiling.LogicalBytes > byteCap || ceiling.Work > workCap {
		return nil, ErrPrivatePreparationEnumerationV1
	}
	domain := "private-folder-preparation.v1/"
	j := jobV2{Schema: jobSchemaV2, StableID: hashLabelV2(domain + i.Attempt), Application: hashLabelV2(domain + "local-census"), Scope: PrivatePreparationEnumerationScopeV1(), SourceAuthorization: id, RevisionPolicy: hashLabelV2(domain + "no-revision"), EvidencePolicies: []string{hashLabelV2(domain + "metadata-only")}, ProviderPolicies: []string{hashLabelV2(domain + "no-provider")}, SemanticPolicies: []string{hashLabelV2(domain + "no-semantic-claims")}, Privacy: hashLabelV2(domain + "private"), Retention: hashLabelV2(domain + "no-replay"), ResourceRules: PrivatePreparationEnumerationRulesV1(i), Origin: "NEW_EMPTY"}
	job, err := sealV2(&j)
	if err != nil {
		return nil, err
	}
	decision := hashLabelV2(domain + "invocation-consent/" + id)
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: ceiling, MaxWindows: windows, Expires: i.Deadline.UnixNano(), MaxTranche: charge, MaxWindowNS: q[8], Decision: decision, Assurance: assuranceV2}
	auth, err := sealV2(&a)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithDeadline(ctx, i.Deadline)
	defer cancel()
	ready, err := InitializePrivateReadyV2(ctx, root, job, auth, PrivateReadyDecisionAssertionV2{j.Identity, a.Identity, decision}, v)
	if err != nil {
		return nil, err
	}
	k := &kernelV2{root: root}
	l := verificationLimitsV2{v.Records, v.Bytes, v.Work}
	head := ready.ConfirmedHead()
	var cursor *source.RecursiveCursor
	defer func() {
		if cursor != nil {
			err = errors.Join(err, cursor.Close())
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			records = nil
		}
	}()
	for n := uint64(0); n < windows; n++ {
		attempt := hashLabelV2(domain + head)
		now = time.Now()
		deadline := now.Add(time.Duration(q[8]))
		if deadline.After(i.Deadline) {
			deadline = i.Deadline
		}
		batch, done, next, e := readFundedPreparationBatch(ctx, k, j, head, attempt, charge, now, deadline, l, func(batchCtx context.Context) ([]source.Record, bool, error) {
			if cursor == nil {
				var openErr error
				cursor, openErr = source.OpenRecursiveCursor(i.SourceRoot, i.Roots, i.PrunedDirectories, i.Limits)
				if openErr != nil {
					return nil, false, openErr
				}
			}
			return cursor.Next(batchCtx, count)
		})
		if e != nil {
			return nil, e
		}
		records = append(records, batch...)
		head = next
		if done {
			return records, nil
		}
	}
	return nil, source.ErrDiscoveryLimit
}

// readFundedPreparationBatch never calls read on replay or uncertain reserve,
// and never returns its detached result on uncertain completion.
func readFundedPreparationBatch(ctx context.Context, k *kernelV2, j jobV2, head, attempt string, charge countersV2, now, deadline time.Time, l verificationLimitsV2, read func(context.Context) ([]source.Record, bool, error)) ([]source.Record, bool, string, error) {
	batchCtx, stop := context.WithDeadline(ctx, deadline)
	defer stop()
	p, err := k.reserve(batchCtx, j, head, attempt, charge, deadline.UnixNano(), now.UnixNano(), l)
	if err != nil {
		return nil, false, "", err
	}
	current, err := k.reopen(batchCtx, j, l)
	if err != nil || batchCtx.Err() != nil || current.Identity != p.Identity || current.Attempt != attempt || current.Charged != p.Charged || current.Disposition != "PENDING" {
		return nil, false, "", ErrPrivatePreparationEnumerationV1
	}
	batch, done, readErr := read(batchCtx)
	if readErr == nil {
		raw, encodeErr := json.Marshal(batch)
		if encodeErr != nil || uint64(len(raw)) > charge.OutputBytes {
			readErr = ErrPrivatePreparationEnumerationV1
		}
	}
	disposition := "OBSERVED"
	if readErr != nil {
		disposition = "FAILED"
	}
	h, err := k.observe(batchCtx, j, p.Identity, attempt, disposition, time.Now().UnixNano(), l)
	if err != nil {
		return nil, false, "", err
	}
	current, err = k.reopen(batchCtx, j, l)
	if err != nil || batchCtx.Err() != nil || current.Identity != h.Identity || current.Attempt != attempt || current.Charged != p.Charged || current.Disposition != "OBSERVED" || readErr != nil {
		return nil, false, "", ErrPrivatePreparationEnumerationV1
	}
	return batch, done, current.Identity, nil
}
