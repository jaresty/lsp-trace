package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"

	"lsp-trace/internal/publication"
)

type scopeCopyAttempt struct {
	outcome string
	receipt *publication.BoundFileReceipt
}
type scopeObjectVerification struct{ state *scopeObjectVerificationState }
type scopeObjectVerificationState struct {
	digest, referencesDigest string
	length                   uint64
	ordinal                  int
	authority                uint8
}
type scopeRetentionAttempt struct {
	phase           string
	primary, backup scopeCopyAttempt
	verified        scopeObjectVerification
}

// These per-call seams are package-private fixture injection, not caller trust
// flags. Normal execution always uses the concrete admitted root methods.
type scopeRetentionOps struct {
	publish func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error)
	read    func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error)
}

func defaultScopeRetentionOps() scopeRetentionOps {
	return scopeRetentionOps{
		publish: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
			return r.Publish(ctx, id, raw)
		},
		read: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, limit int64) ([]byte, error) {
			return r.Read(ctx, id, limit)
		},
	}
}

// Reconstruct the scope from the exact nine references and re-derive the plan.
// No boolean, cached budget or caller-edited object list serves as admission.
func validateScopeRetentionPlan(p scopeRetentionPlan, availableBytes, availableWork uint64) error {
	if p.state == nil || len(p.state.references) != 9 || len(p.state.objects) < 1 || len(p.state.objects) > 9 || uint64(len(p.state.metadata)) > scopeRetentionMetadataLimit {
		return ErrInvalid
	}
	for _, obj := range p.state.objects {
		if len(obj.raw) == 0 || int64(len(obj.raw)) > publication.ContentRetentionObjectLimit || Digest(obj.raw) != obj.digest {
			return ErrInvalid
		}
	}
	rawAt := func(i int) []byte {
		r := p.state.references[i]
		if r.Object < 0 || r.Object >= len(p.state.objects) {
			return nil
		}
		return p.state.objects[r.Object].raw
	}
	manifest := rawAt(8)
	var m scopeManifest
	if len(manifest) > int(scopeRetentionMetadataLimit) || json.Unmarshal(manifest, &m) != nil || len(m.Files) != 6 {
		return ErrInvalid
	}
	state := &featureScopeState{manifest: manifest, selection: rawAt(6), plan: rawAt(7), files: make([]scopeObservedFile, 6)}
	for i := range state.files {
		state.files[i] = scopeObservedFile{m.Files[i], rawAt(i)}
	}
	derived, err := deriveScopeRetentionPlan(featureScope{state}, availableBytes, availableWork)
	if err != nil || !reflect.DeepEqual(derived.state, p.state) {
		return ErrInvalid
	}
	return nil
}

// Only confirmed absence permits publication. A not-exist error joined with a
// close/cancellation failure is not absence admission; never erase siblings.
func retentionOnlyAbsent(err error, depth int) bool {
	if err == nil || depth > 16 {
		return false
	}
	if err == os.ErrNotExist {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !retentionOnlyAbsent(child, depth+1) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return retentionOnlyAbsent(wrapped, depth+1)
	}
	return errors.Is(err, os.ErrNotExist)
}

func retainScopeObject(ctx context.Context, plan scopeRetentionPlan, ordinal int, primary, backup *publication.ContentRetentionRoot, availableBytes, availableWork uint64) (scopeRetentionAttempt, error) {
	return retainScopeObjectWithOps(ctx, plan, ordinal, primary, backup, availableBytes, availableWork, defaultScopeRetentionOps())
}

// One object, TWO independent no-replace storage operations; no head, workflow
// transition, rollback, deletion, retry or whole-bundle retained claim. Roots are
// borrowed: object handles close/reopen for every read while directories stay
// pinned. Owner must close root capabilities separately and account close errors.
func retainScopeObjectWithOps(ctx context.Context, plan scopeRetentionPlan, ordinal int, primary, backup *publication.ContentRetentionRoot, availableBytes, availableWork uint64, ops scopeRetentionOps) (out scopeRetentionAttempt, err error) {
	out.phase = "PREFLIGHT"
	out.primary.outcome = "NOT_ATTEMPTED"
	out.backup.outcome = "NOT_ATTEMPTED"
	if ctx == nil || ops.publish == nil || ops.read == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = validateScopeRetentionPlan(plan, availableBytes, availableWork); err != nil {
		return out, err
	}
	if ordinal < 0 || ordinal >= len(plan.state.objects) {
		return out, ErrInvalid
	}
	return retainScopePairBytes(ctx, plan.state.objects[ordinal], Digest(plan.state.metadata), ordinal, primary, backup, ops)
}

// Metadata is a separate final item, never an extra ordinal in the validated plan.
func retainScopeMetadata(ctx context.Context, plan scopeRetentionPlan, primary, backup *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
	if err := validateScopeRetentionPlan(plan, scopeRetentionByteCeiling, scopeRetentionWorkCeiling); err != nil {
		return scopeRetentionAttempt{phase: "PREFLIGHT"}, err
	}
	return retainScopePairBytes(ctx, scopeRetentionObject{Digest(plan.state.metadata), plan.metadataBytes()}, Digest(plan.state.metadata), -1, primary, backup, defaultScopeRetentionOps())
}

func retainScopePairBytes(ctx context.Context, obj scopeRetentionObject, referencesDigest string, ordinal int, primary, backup *publication.ContentRetentionRoot, ops scopeRetentionOps) (out scopeRetentionAttempt, err error) {
	out.phase = "PREFLIGHT"
	out.primary.outcome, out.backup.outcome = "NOT_ATTEMPTED", "NOT_ATTEMPTED"
	if ctx == nil || ops.publish == nil || ops.read == nil {
		return out, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	same, err := primary.SameDirectory(backup)
	if err != nil {
		return out, err
	}
	if same {
		return out, ErrInvalid
	}
	verify := func(root *publication.ContentRetentionRoot) error {
		raw, e := ops.read(ctx, root, obj.digest, int64(len(obj.raw)))
		if e != nil {
			return e
		}
		if !bytes.Equal(raw, obj.raw) || Digest(raw) != obj.digest {
			return ErrInvalid
		}
		return ctx.Err()
	}
	publish := func(root *publication.ContentRetentionRoot, evidence *scopeCopyAttempt, copyName string) error {
		out.phase = copyName + "_PRE_READ"
		if e := ctx.Err(); e != nil {
			return e
		}
		// Existing objects are admitted only by a close-aware read BEFORE any
		// publication attempt. Late ErrExist may hide legacy cleanup failures.
		raw, readErr := ops.read(ctx, root, obj.digest, int64(len(obj.raw)))
		if readErr == nil {
			evidence.outcome = "EXISTING_UNVERIFIED"
			if !bytes.Equal(raw, obj.raw) || Digest(raw) != obj.digest {
				return ErrInvalid
			}
			if e := ctx.Err(); e != nil {
				return e
			}
			evidence.outcome = "EXISTING_VERIFIED"
			return nil
		}
		if !retentionOnlyAbsent(readErr, 0) {
			evidence.outcome = "EXISTING_UNVERIFIED"
			return readErr
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		out.phase = copyName + "_PUBLICATION"
		evidence.outcome = "INDETERMINATE"
		receipt, e := ops.publish(ctx, root, obj.digest, bytes.Clone(obj.raw))
		if receipt != nil {
			copied := *receipt
			evidence.receipt = &copied
			if receipt.NamespaceAtomic {
				evidence.outcome = "NAMESPACE_COMMITTED"
			}
		}
		if e != nil {
			return e
		}
		if receipt == nil || !receipt.NamespaceAtomic || receipt.FinalSelector != obj.digest || receipt.Digest != obj.digest || receipt.ByteLength != uint64(len(obj.raw)) || receipt.Mechanism != publication.BoundFileMechanism || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CrashDurability != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.VerificationStatus != "VERIFIED" {
			return ErrInvalid
		}
		return ctx.Err()
	}
	if err = publish(primary, &out.primary, "PRIMARY"); err != nil {
		return out, err
	}
	if err = publish(backup, &out.backup, "BACKUP"); err != nil {
		return out, err
	}
	// Every ReadContentObserved call opens, hashes, checks exact bytes and closes
	// its object handle fail-closed. Repeat on both exact pinned roots, rather than
	// reopening ambient directory paths or trusting an earlier successful read.
	for pass := 0; pass < 2; pass++ {
		out.phase = "PRIMARY_REOPEN"
		if err = verify(primary); err != nil {
			return out, err
		}
		out.phase = "BACKUP_REOPEN"
		if err = verify(backup); err != nil {
			return out, err
		}
	}
	out.phase = "OBJECT_COPIES_VERIFIED"
	out.verified = scopeObjectVerification{&scopeObjectVerificationState{obj.digest, referencesDigest, uint64(len(obj.raw)), ordinal, 0}}
	return out, nil
}
