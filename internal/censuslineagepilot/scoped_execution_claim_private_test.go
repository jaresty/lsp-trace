//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
)

// In-memory store models only the publication CAS contract, not live durability.
// Input bundles and root admission use existing disposable owned-root fixtures.
type executionClaimMemoryPrivate struct {
	mu            sync.Mutex
	head          []byte
	loads, writes int
	receipt       *publication.CompareAndReplaceReceipt
	fault         func(context.Context, string, []byte) (*publication.CompareAndReplaceReceipt, error)
	loadErr       error
	barrier       chan struct{}
	arrived       atomic.Int32
}

func (s *executionClaimMemoryPrivate) Load(ctx context.Context, _ string) ([]byte, error) {
	s.mu.Lock()
	s.loads++
	raw := bytes.Clone(s.head)
	err := s.loadErr
	s.mu.Unlock()
	if s.barrier != nil {
		if s.arrived.Add(1) == 2 {
			close(s.barrier)
		}
		select {
		case <-s.barrier:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return raw, err
}
func goodExecutionCASPrivate(selector string, raw []byte) *publication.CompareAndReplaceReceipt {
	return &publication.CompareAndReplaceReceipt{Committed: true, Outcome: publication.CASOutcomeCommitted, BoundFileReceipt: &publication.BoundFileReceipt{
		FinalSelector: selector, Digest: Digest(raw), ByteLength: uint64(len(raw)), NamespaceAtomic: true,
		Mechanism: publication.BoundFileMechanism, DirectorySyncStatus: publication.DirectorySyncComplete, CrashDurability: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete, VerificationStatus: "VERIFIED",
	}}
}
func (s *executionClaimMemoryPrivate) CompareAbsent(ctx context.Context, selector string, raw []byte) (*publication.CompareAndReplaceReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if s.fault != nil {
		return s.fault(ctx, selector, raw)
	}
	if s.head != nil {
		return &publication.CompareAndReplaceReceipt{}, publication.ErrStalePredecessor
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.head = bytes.Clone(raw)
	s.receipt = goodExecutionCASPrivate(selector, raw)
	return s.receipt, nil
}

type executionClaimFixturePrivate struct {
	restart    scopeRestartResult
	roots      [3]*publication.ContentRetentionRoot
	workspace  string
	session    censusacquisition.SessionIdentity
	revision   string
	inputPaths [3]string
}

func executionClaimFixture(t *testing.T) executionClaimFixturePrivate {
	t.Helper()
	input, scope, loc := restartFixture(t, false)
	restart, err := readRetainedFeatureScope(context.Background(), input.paths, loc)
	if err != nil {
		t.Fatal(err)
	}
	a, b, _, _ := retentionRoots(t)
	c, _, _, _ := retentionRoots(t)
	return executionClaimFixturePrivate{restart, [3]*publication.ContentRetentionRoot{a, b, c}, scope.Selection().Workspace, censusacquisition.SessionIdentity{SessionID: "fixture-exact-session", Generation: 1}, strings.Repeat("a", 40), input.paths}
}
func (f executionClaimFixturePrivate) owner(t *testing.T) *scopedExecutionClaimOwnerPrivate {
	t.Helper()
	o, err := newScopedExecutionClaimPrivate(context.Background(), f.restart, f.revision, f.session, f.workspace, f.roots)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestScopedExecutionClaimPrivateExactAndCopies(t *testing.T) {
	f := executionClaimFixture(t)
	before := restartSnapshot(t, f.inputPaths)
	o := f.owner(t)
	var h scopedExecutionHeadPrivate
	if err := scopeBundleStrict(o.HeadBytes(), &h); err != nil {
		t.Fatal(err)
	}
	canonical, _ := json.Marshal(h)
	if !bytes.Equal(canonical, o.HeadBytes()) || h.Schema != scopedExecutionClaimSchemaPrivate || h.State != "CLAIMED" || h.Authority != 0 || h.Accepted || h.Completeness != "UNKNOWN" || h.FeatureIdentity != "UNRESOLVED" || h.RevisionQualification != "CALLER_ASSERTED_NOT_AUTHENTICATED" {
		t.Fatal("canonical defaults")
	}
	if h.Binding.InputHead != restartPin(f.restart.HeadBytes()) || h.Binding.Scope != restartPin(f.restart.Scope().ManifestBytes()) || h.Binding.Metadata != restartPin(f.restart.MetadataBytes()) || h.Binding.ExecutableRevision != f.revision || h.Binding.SessionID != f.session.SessionID || h.Binding.Generation != f.session.Generation || h.Binding.Workspace != f.workspace || h.Binding.Policy != scopedExecutionPolicyFixedPrivate() {
		t.Fatal("exact binding")
	}
	for i, r := range f.roots {
		id, e := r.DirectoryIdentity()
		if e != nil || id != h.Binding.OutputRoots[i] {
			t.Fatal("admitted roots", e)
		}
	}
	raw := o.HeadBytes()
	raw[0] = '!'
	if o.HeadBytes()[0] == '!' {
		t.Fatal("owner accessor alias")
	}
	s := &executionClaimMemoryPrivate{}
	out, err := o.claim(context.Background(), s)
	if err != nil || out.outcome != "CLAIMED" || !out.commitReported || out.pending == nil || !o.ownsPending(out.pending) || s.writes != 1 || !bytes.Equal(out.HeadBytes(), o.HeadBytes()) {
		t.Fatal("exact affirmative claim", err)
	}
	receipt := out.CASReceipt()
	receipt.Digest = "mutated"
	receipt.Outcome = "mutated"
	raw = out.HeadBytes()
	raw[0] = '!'
	s.receipt.Digest = "store mutated"
	if !o.ownsPending(out.pending) || out.CASReceipt().Digest != Digest(o.HeadBytes()) || out.HeadBytes()[0] == '!' {
		t.Fatal("receipt/head alias")
	}
	copyPending := *out.pending
	if o.ownsPending(&copyPending) || f.owner(t).ownsPending(out.pending) {
		t.Fatal("copied/foreign pending")
	}
	// A caller's detached restart accessors cannot alter already bound provenance.
	r := f.restart.HeadBytes()
	r[0] = '!'
	if !bytes.Equal(o.HeadBytes(), canonical) {
		t.Fatal("constructor alias")
	}
	after := restartSnapshot(t, f.inputPaths)
	if len(before) != len(after) {
		t.Fatal("input head changed")
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatal("input changed", k)
		}
	}
}

func TestScopedExecutionClaimPrivateBindingSubstitution(t *testing.T) {
	f := executionClaimFixture(t)
	original := f.owner(t)
	for _, kind := range []string{"revision", "session", "generation", "roots"} {
		t.Run(kind, func(t *testing.T) {
			changed := f
			switch kind {
			case "revision":
				changed.revision = strings.Repeat("b", 40)
			case "session":
				changed.session.SessionID += "-other"
			case "generation":
				changed.session.Generation++
			case "roots":
				changed.roots[0], changed.roots[1] = changed.roots[1], changed.roots[0]
			}
			o := changed.owner(t)
			if bytes.Equal(o.HeadBytes(), original.HeadBytes()) {
				t.Fatal("binding substitution not represented")
			}
			if o.selector != original.selector {
				t.Fatal("binding change bypassed existing output claim")
			}
			s := &executionClaimMemoryPrivate{fault: func(_ context.Context, selector string, _ []byte) (*publication.CompareAndReplaceReceipt, error) {
				return goodExecutionCASPrivate(selector, original.HeadBytes()), nil
			}}
			out, err := o.claim(context.Background(), s)
			if err == nil || out.pending != nil || !errors.Is(err, ErrIndeterminateExecution) || s.writes != 1 {
				t.Fatal("foreign canonical receipt admitted", err)
			}
		})
	}
}

func TestScopedExecutionClaimPrivateInvalidBeforeStore(t *testing.T) {
	f := executionClaimFixture(t)
	other := executionClaimFixture(t)
	for _, kind := range []string{"nil-restart", "nonterminal", "mixed-head", "mixed-metadata", "mixed-scope", "missing-receipts", "oversized-head", "oversized-metadata", "oversized-receipts", "revision", "zero-generation", "session-bound", "workspace", "workspace-bound", "nil-root", "same-root", "closed-root", "input-root", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			changed := f
			// Copy opaque retained state so negative fixtures do not corrupt siblings.
			state := *f.restart.state
			changed.restart = scopeRestartResult{&state}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "nil-restart":
				changed.restart = scopeRestartResult{}
			case "nonterminal":
				var h scopeBundleHead
				_ = json.Unmarshal(state.head, &h)
				h.State = "CLAIMED"
				state.head, _ = json.Marshal(h)
			case "mixed-head":
				state.head = other.restart.HeadBytes()
			case "mixed-metadata":
				state.metadata = other.restart.MetadataBytes()
			case "mixed-scope":
				state.scope = other.restart.Scope()
			case "missing-receipts":
				state.linked = nil
			case "oversized-head":
				state.head = bytes.Repeat([]byte("x"), scopeBundleWireLimit+1)
			case "oversized-metadata":
				state.metadata = bytes.Repeat([]byte("x"), int(scopeRetentionMetadataLimit)+1)
			case "oversized-receipts":
				state.linked = make([][]byte, 11)
			case "revision":
				changed.revision = strings.Repeat("A", 40)
			case "zero-generation":
				changed.session.Generation = 0
			case "session-bound":
				changed.session.SessionID = strings.Repeat("s", 1025)
			case "workspace":
				changed.workspace += "/../rebase"
			case "workspace-bound":
				changed.workspace = strings.Repeat("w", 4097)
			case "nil-root":
				changed.roots[0] = nil
			case "same-root":
				changed.roots[1] = changed.roots[0]
			case "closed-root":
				r, _, _, _ := retentionRoots(t)
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
				changed.roots[0] = r
			case "input-root":
				r, e := publication.OpenContentRetentionRoot(ctx, f.inputPaths[0])
				if e != nil {
					t.Fatal(e)
				}
				defer r.Close()
				changed.roots[0] = r
			case "cancel":
				cancel()
			}
			o, err := newScopedExecutionClaimPrivate(ctx, changed.restart, changed.revision, changed.session, changed.workspace, changed.roots)
			if err == nil || o != nil {
				t.Fatal("invalid constructor admitted", kind)
			}
		})
	}
}

func TestScopedExecutionClaimPrivatePresentLostAndNoRetry(t *testing.T) {
	f := executionClaimFixture(t)
	for _, head := range [][]byte{f.owner(t).HeadBytes(), []byte("malformed"), {}, bytes.Repeat([]byte("x"), scopedExecutionClaimLimitPrivate+1)} {
		s := &executionClaimMemoryPrivate{head: head}
		o := f.owner(t)
		for i := 0; i < 2; i++ {
			out, err := o.claim(context.Background(), s)
			if !errors.Is(err, ErrIndeterminateExecution) || out.pending != nil {
				t.Fatal("present/repeated authorization", err)
			}
		}
		if s.writes != 0 || s.loads != 1 {
			t.Fatal("present head caused CAS/reload")
		}
	}
	s := &executionClaimMemoryPrivate{}
	out, err := f.owner(t).claim(context.Background(), s)
	if err != nil || out.pending == nil {
		t.Fatal(err)
	}
	out.pending = nil // pending capability lost, not reconstructed from durable head
	reopened, err := f.owner(t).claim(context.Background(), s)
	if !errors.Is(err, ErrIndeterminateExecution) || reopened.pending != nil || s.writes != 1 {
		t.Fatal("restart replay", err)
	}
}

func TestScopedExecutionClaimPrivateConcurrentOnePending(t *testing.T) {
	f := executionClaimFixture(t)
	for _, sameOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "two-owners", true: "same-owner"}[sameOwner], func(t *testing.T) {
			a, b := f.owner(t), f.owner(t)
			s := &executionClaimMemoryPrivate{}
			if sameOwner {
				b = a
			} else {
				s.barrier = make(chan struct{})
			}
			var wg sync.WaitGroup
			var caps atomic.Int32
			errs := make(chan error, 2)
			for _, o := range []*scopedExecutionClaimOwnerPrivate{a, b} {
				wg.Add(1)
				go func(o *scopedExecutionClaimOwnerPrivate) {
					defer wg.Done()
					out, err := o.claim(context.Background(), s)
					if out.pending != nil {
						if !o.ownsPending(out.pending) {
							errs <- ErrInvalid
						}
						caps.Add(1)
					} else if !errors.Is(err, ErrConflict) && !errors.Is(err, ErrIndeterminateExecution) {
						errs <- err
					}
				}(o)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal("unexpected concurrent result", err)
			}
			if caps.Load() != 1 || s.writes != map[bool]int{true: 1, false: 2}[sameOwner] {
				t.Fatal("concurrent claim count", caps.Load(), s.writes)
			}
		})
	}
}

func TestScopedExecutionClaimPrivateCASQualification(t *testing.T) {
	f := executionClaimFixture(t)
	for _, kind := range []string{"nil", "missing-inner", "empty", "not-committed", "unknown", "verification-failed", "false-committed", "error-with-good", "cancel-with-good", "stale", "wrapped-stale", "mixed-stale", "stale-good", "stale-not-committed", "selector", "digest", "length", "atomic", "mechanism", "directory", "durability", "close", "verification", "noncanonical-bytes"} {
		t.Run(kind, func(t *testing.T) {
			o := f.owner(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &executionClaimMemoryPrivate{fault: func(_ context.Context, selector string, raw []byte) (*publication.CompareAndReplaceReceipt, error) {
				r := goodExecutionCASPrivate(selector, raw)
				switch kind {
				case "nil":
					return nil, nil
				case "missing-inner":
					r.BoundFileReceipt = nil
				case "empty":
					return &publication.CompareAndReplaceReceipt{}, nil
				case "not-committed":
					r.Outcome = publication.CASOutcomeNotCommitted
					r.Committed = false
				case "unknown":
					r.Outcome = "UNKNOWN"
				case "verification-failed":
					r.Outcome = publication.CASOutcomeCommittedVerificationFailed
				case "false-committed":
					r.Committed = false
				case "error-with-good":
					return r, errors.New("directory failure")
				case "cancel-with-good":
					cancel()
					return r, nil
				case "stale":
					return &publication.CompareAndReplaceReceipt{}, publication.ErrStalePredecessor
				case "wrapped-stale":
					return &publication.CompareAndReplaceReceipt{}, errors.Join(publication.ErrStalePredecessor, publication.ErrStalePredecessor)
				case "mixed-stale":
					return &publication.CompareAndReplaceReceipt{}, errors.Join(publication.ErrStalePredecessor, context.Canceled)
				case "stale-good":
					return r, publication.ErrStalePredecessor
				case "stale-not-committed":
					return &publication.CompareAndReplaceReceipt{Outcome: publication.CASOutcomeNotCommitted}, publication.ErrStalePredecessor
				case "selector":
					r.FinalSelector = "feature-scope-bundle-head-private-v1-foreign.json"
				case "digest":
					r.Digest = Digest([]byte("other"))
				case "length":
					r.ByteLength++
				case "atomic":
					r.NamespaceAtomic = false
				case "mechanism":
					r.Mechanism = "OTHER"
				case "directory":
					r.DirectorySyncStatus = "OTHER"
				case "durability":
					r.CrashDurability = "OTHER"
				case "close":
					r.CloseStatus = "OTHER"
				case "verification":
					r.VerificationStatus = "OTHER"
				case "noncanonical-bytes":
					return goodExecutionCASPrivate(selector, append(bytes.Clone(raw), '\n')), nil
				}
				return r, nil
			}}
			out, err := o.claim(ctx, s)
			conflict := kind == "stale" || kind == "wrapped-stale"
			if out.pending != nil || o.pending != nil || err == nil || s.writes != 1 || errors.Is(err, ErrConflict) != conflict || errors.Is(err, ErrIndeterminateExecution) == conflict {
				t.Fatal("CAS classification", kind, out.outcome, err)
			}
			if out.cas != nil && out.cas.Committed && (!out.commitReported || !bytes.Equal(out.HeadBytes(), o.HeadBytes())) {
				t.Fatal("lost committed artifact reference")
			}
			again, e := o.claim(context.Background(), s)
			if again.pending != nil || !errors.Is(e, ErrIndeterminateExecution) || s.writes != 1 {
				t.Fatal("retried uncertain CAS", e)
			}
		})
	}
}

func TestScopedExecutionClaimPrivateLoadBoundsAndCancellation(t *testing.T) {
	f := executionClaimFixture(t)
	for _, kind := range []string{"load-error", "cancel-before", "cancel-after-load", "oversized-owner", "empty-owner"} {
		t.Run(kind, func(t *testing.T) {
			o := f.owner(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &executionClaimMemoryPrivate{}
			var store scopedExecutionClaimStorePrivate = s
			switch kind {
			case "load-error":
				s.loadErr = errors.New("read/close uncertain")
			case "cancel-before":
				cancel()
			case "cancel-after-load":
				store = executionClaimCancelLoadPrivate{s, cancel}
			case "oversized-owner":
				o.raw = bytes.Repeat([]byte("x"), scopedExecutionClaimLimitPrivate+1)
			case "empty-owner":
				o.raw = nil
			}
			out, err := o.claim(ctx, store)
			if err == nil || out.pending != nil || s.writes != 0 {
				t.Fatal("prewrite refusal", err)
			}
			if (kind == "oversized-owner" || kind == "empty-owner" || kind == "cancel-before") && s.loads != 0 {
				t.Fatal("bounds/cancellation after traffic")
			}
		})
	}
}

type executionClaimCancelLoadPrivate struct {
	*executionClaimMemoryPrivate
	cancel context.CancelFunc
}

func (s executionClaimCancelLoadPrivate) Load(ctx context.Context, key string) ([]byte, error) {
	raw, err := s.executionClaimMemoryPrivate.Load(ctx, key)
	s.cancel()
	return raw, err
}
