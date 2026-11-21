package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"lsp-trace/internal/publication"
)

// WithLocalFolderControllerV1 lends a one-shot, freshly prepaid controller
// envelope while the original acquisition is present. Its callback is NOT a
// semantic engine: the existing application controller owns all role ordering.
// No persisted diagnostic can reconstruct this acquisition or callback permit.
func WithLocalFolderControllerV1(ctx context.Context, root *publication.Root, acquisition PrivateDiscoveryOperationV2, child [14]uint64, consume func([14]uint64, string) error) (err error) {
	if ctx == nil || ctx.Err() != nil || root == nil || acquisition.state == nil || consume == nil {
		return ErrPrivateDiscoveryOperationV2
	}
	s := acquisition.state
	s.mu.Lock()
	if !s.completed || s.executionErr != nil || s.observation.Failed || !s.observation.CleanupJoined || s.controllerClaimed || root == s.owner.k.root {
		s.mu.Unlock()
		return ErrPrivateDiscoveryOperationV2
	}
	s.controllerClaimed = true // uncertain/failed admission is never replayable
	binding, originalAttempt := s.binding, s.attempt
	s.mu.Unlock()
	grant := countersFromV2(child)
	// Existing normal application upper bounds, not a new larger ceiling.
	cap := countersV2{LogicalBytes: 1 << 32, ReadHashBytes: 1 << 36, WriteBytes: 1 << 36, Records: 1 << 24, ParserBytes: 1 << 36, Work: 1 << 26, ActiveNS: uint64(180 * time.Second), Calls: 16, RequestBytes: 1 << 20, ResponseBytes: 64 << 10, WireBytes: 8 << 20, OutputBytes: 4 << 20}
	if !grant.within(cap) || grant.Calls < 2 || grant.ActiveNS == 0 {
		return ErrPrivateDiscoveryOperationV2
	}
	now := time.Now()
	if !now.Before(binding.Deadline) {
		return ErrPrivateDiscoveryOperationV2
	}
	// The parent pays for the executable hash and kernel bounded verification in
	// addition to (not from) the child's frozen application allocation.
	v := PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}
	prelude := countersV2{LogicalBytes: 64 << 10, ReadHashBytes: 64<<20 + 8*v.Bytes, WriteBytes: 4 * v.Bytes, Records: 8*v.Records + 128, ParserBytes: 8 * v.Bytes, Work: 1 << 20, ActiveNS: uint64(binding.Deadline.Sub(now))}
	charge, e := addCountersV2(grant, prelude)
	if e != nil || !charge.within(cap) {
		return errors.Join(ErrPrivateDiscoveryOperationV2, e)
	}
	domain := "local-folder-controller.v1/"
	encoded, e := json.Marshal(child)
	if e != nil {
		return e
	}
	attempt := hashLabelV2(domain + originalAttempt)
	j := jobV2{Schema: jobSchemaV2, StableID: attempt, Application: hashLabelV2(domain + "census"), Scope: hashLabelV2(domain + "one-existing-controller"), SourceAuthorization: binding.DenominatorDigest, RevisionPolicy: hashLabelV2(domain + binding.Revision), EvidencePolicies: []string{hashLabelV2(domain + "retained-originals")}, ProviderPolicies: []string{hashLabelV2(domain + "no-provider-credit")}, SemanticPolicies: []string{hashLabelV2(domain + "existing-V3-controller")}, Privacy: hashLabelV2(domain + "private"), Retention: hashLabelV2(domain + "one-shot"), ResourceRules: hashLabelV2(domain + string(encoded)), Origin: "NEW_EMPTY"}
	job, e := sealV2(&j)
	if e != nil {
		return e
	}
	decision := hashLabelV2(domain + "invocation-consent/" + attempt)
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: charge, MaxTranche: charge, MaxWindows: 1, MaxWindowNS: charge.ActiveNS, Expires: binding.Deadline.UnixNano(), Decision: decision, Assurance: assuranceV2}
	auth, e := sealV2(&a)
	if e != nil {
		return e
	}
	ready, e := InitializePrivateReadyV2(ctx, root, job, auth, PrivateReadyDecisionAssertionV2{Job: j.Identity, Authorization: a.Identity, Reference: decision}, v)
	if e != nil {
		return e
	}
	k := &kernelV2{root: root}
	limits := verificationLimitsV2{v.Records, v.Bytes, v.Work}
	pending, e := k.reserve(ctx, j, ready.ConfirmedHead(), attempt, charge, binding.Deadline.UnixNano(), time.Now().UnixNano(), limits)
	if e != nil {
		return e
	}
	finished := false
	defer func() {
		if !finished {
			_, e := k.observe(ctx, j, pending.Identity, attempt, "FAILED", time.Now().UnixNano(), limits)
			err = errors.Join(err, e)
		}
	}()
	checked, e := k.reopen(ctx, j, limits)
	if e != nil || checked.Identity != pending.Identity || checked.Disposition != "PENDING" || checked.Attempt != attempt || checked.Charged != charge {
		return errors.Join(ErrPrivateDiscoveryOperationV2, e)
	}
	// All executable IO follows reservation and independent exact-head readback.
	path, e := os.Executable()
	if e != nil {
		return e
	}
	producer, e := folderControllerExecutableDigest(ctx, path, 64<<20)
	if e != nil {
		return e
	}
	if err = consume(child, producer); err != nil {
		return err
	}
	completed, e := k.observe(ctx, j, pending.Identity, attempt, "OBSERVED", time.Now().UnixNano(), limits)
	if e != nil {
		return e
	}
	finished = true
	readback, e := k.reopen(ctx, j, limits)
	if e != nil || readback.Identity != completed.Identity || readback.Disposition != "OBSERVED" || readback.Charged != charge {
		return errors.Join(ErrPrivateDiscoveryOperationV2, e)
	}
	return nil
}

func folderControllerExecutableDigest(ctx context.Context, path string, limit int64) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	before, e := f.Stat()
	if e != nil || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return "", ErrPrivateDiscoveryOperationV2
	}
	h := sha256.New()
	buffer := make([]byte, 32<<10)
	var n int64
	for n < before.Size() {
		if e = ctx.Err(); e != nil {
			return "", e
		}
		want := min(int64(len(buffer)), before.Size()-n)
		got, err := io.ReadFull(f, buffer[:want])
		if err != nil {
			return "", err
		}
		h.Write(buffer[:got])
		n += int64(got)
	}
	after, e := f.Stat()
	visible, ve := os.Stat(path)
	if e != nil || ve != nil || !os.SameFile(before, after) || !os.SameFile(after, visible) || before.Size() != after.Size() || before.ModTime() != after.ModTime() || after.Size() != visible.Size() || after.ModTime() != visible.ModTime() || ctx.Err() != nil {
		return "", ErrPrivateDiscoveryOperationV2
	}
	// Self-observed bytes, not authentication of a caller or model execution.
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
