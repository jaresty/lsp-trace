package inventorysuccessor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func readyTestInputs(t *testing.T) (jobV2, authorizationV2, []byte, []byte, PrivateReadyDecisionAssertionV2, PrivateManagedInputVerificationV2) {
	t.Helper()
	j, a, _ := fixtureV2(t)
	// Finite TEST coordinates only, not approved policy or production issuance.
	a.Expires = time.Now().Add(time.Minute).UnixNano()
	a.Ceiling = countersFromV2([14]uint64{8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8})
	a.MaxTranche = countersFromV2([14]uint64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	jb, ab := sealTestV2(t, &j), sealTestV2(t, &a)
	return j, a, jb, ab, PrivateReadyDecisionAssertionV2{j.Identity, a.Identity, a.Decision}, PrivateManagedInputVerificationV2{400, 4 << 20, 400}
}
func readyTestRoot(t *testing.T) *publication.Root {
	t.Helper()
	p := t.TempDir()
	if e := os.Chmod(p, 0700); e != nil {
		t.Fatal(e)
	}
	r, e := publication.OpenRoot(p)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	return r
}
func readyAssertRefused(t *testing.T, r *PrivateReadyInitializedV2, e error) {
	t.Helper()
	if e == nil || r != nil {
		t.Fatalf("ASSERT_READY_NO_USABLE_ON_ERROR: result=%v error=%v", r, e)
	}
}
func TestPrivateReadyActualAPIAndOpener(t *testing.T) {
	j, a, jb, ab, d, l := readyTestInputs(t)
	root := readyTestRoot(t)
	original := bytes.Clone(jb)
	r, e := InitializePrivateReadyV2(context.Background(), root, jb, ab, d, l)
	if e != nil || !r.Valid() {
		t.Fatalf("ASSERT_READY_CONNECTED_API: %v", e)
	}
	owner, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, r.CanonicalJob(), r.ConfirmedHead(), l)
	if e != nil || owner == nil {
		t.Fatalf("ASSERT_READY_UNCHANGED_OPENER: %v", e)
	}
	h, e := (&kernelV2{root: root}).reopen(context.Background(), j, verificationLimitsV2{l.Records, l.Bytes, l.Work})
	if e != nil || h.Identity != r.ConfirmedHead() || h.Job.Identity != j.Identity || h.Authorization != a.Identity || h.Event != "INITIALIZE" || h.Sequence != 1 || h.Disposition != "READY" || h.Charged != (countersV2{}) || h.Windows != 0 {
		t.Fatalf("ASSERT_READY_EXACT_ZERO_INITIAL: %+v %v", h, e)
	}
	jb[0] = 'x'
	copy := r.CanonicalJob()
	copy[0] = 'x'
	if !bytes.Equal(r.CanonicalJob(), original) || root.ValidatePrivate() != nil {
		t.Fatal("ASSERT_READY_COPIED_BORROWED_ROOT")
	}
	var zero PrivateReadyInitializedV2
	if zero.Valid() || zero.CanonicalJob() != nil || zero.ConfirmedHead() != "" || zero.CheckUsable(context.Background()) == nil {
		t.Fatal("ASSERT_READY_ZERO_INVALID")
	}
	expired := *r
	expired.expires = time.Now().Add(-time.Second).UnixNano()
	if expired.CheckUsable(context.Background()) == nil {
		t.Fatal("ASSERT_READY_ORIGINAL_EXPIRY")
	}
	c, cancel := context.WithCancel(context.Background())
	cancel()
	if r.CheckUsable(c) == nil {
		t.Fatal("ASSERT_READY_FINAL_CONTEXT")
	}
	t.Log("ASSERT_READY_CONNECTED_API PASS; ASSERT_READY_EXACT_ZERO_INITIAL PASS; ASSERT_READY_COPIED_BORROWED_ROOT PASS; ASSERT_READY_ZERO_INVALID PASS")
}
func TestPrivateReadyInputSnapshot(t *testing.T) {
	_, _, jb, ab, d, l := readyTestInputs(t)
	original := bytes.Clone(jb)
	root := readyTestRoot(t)
	k := &kernelV2{root: root}
	k.cas = func(c context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		receipt, err := publication.CompareAndReplaceBoundFile(c, r, s, p, b, v)
		jb[0] = 'x' // same-thread controlled mutation, not a racy caller
		return receipt, err
	}
	result, err := initializePrivateReadyV2(context.Background(), k, jb, ab, d, l)
	if err != nil || !bytes.Equal(result.CanonicalJob(), original) {
		t.Fatalf("ASSERT_READY_INPUT_SNAPSHOT: err=%v", err)
	}
	if _, err := OpenPrivateManagedInputOwnerV2(context.Background(), root, result.CanonicalJob(), result.ConfirmedHead(), l); err != nil {
		t.Fatalf("ASSERT_READY_SNAPSHOT_UNCHANGED_OPENER: %v", err)
	}
	t.Log("ASSERT_READY_INPUT_SNAPSHOT PASS; fixed continuation over actual CAS")
}

func TestPrivateReadyRefusalsBeforeWrites(t *testing.T) {
	for _, name := range []string{"job-oversize", "auth-oversize", "unknown", "duplicate", "trailing", "noncanonical", "expired", "foreign-auth", "wrong-job", "wrong-auth", "wrong-ref", "missing-decision", "budget", "cancelled", "closed-root", "nonprivate"} {
		t.Run(name, func(t *testing.T) {
			j, a, jb, ab, d, l := readyTestInputs(t)
			root := readyTestRoot(t)
			ctx := context.Background()
			switch name {
			case "job-oversize":
				jb = bytes.Repeat([]byte("x"), recordLimitV2+1)
			case "auth-oversize":
				ab = bytes.Repeat([]byte("x"), recordLimitV2+1)
			case "unknown":
				jb = bytes.Replace(jb, []byte(`{"schema":`), []byte(`{"extra":0,"schema":`), 1)
			case "duplicate":
				jb = bytes.Replace(jb, []byte(`{"schema":`), []byte(`{"schema":"bad","schema":`), 1)
			case "trailing":
				ab = append(ab, []byte("{}")...)
			case "noncanonical":
				jb = append([]byte(" "), jb...)
			case "expired":
				a.Expires = time.Now().Add(-time.Second).UnixNano()
				ab = sealTestV2(t, &a)
				d.Authorization = a.Identity
			case "foreign-auth":
				a.Job = hashLabelV2("foreign")
				ab = sealTestV2(t, &a)
				d.Authorization = a.Identity
			case "wrong-job":
				d.Job = hashLabelV2("foreign")
			case "wrong-auth":
				d.Authorization = hashLabelV2("foreign")
			case "wrong-ref":
				d.Reference = hashLabelV2("foreign")
			case "missing-decision":
				d = PrivateReadyDecisionAssertionV2{}
			case "budget":
				l.Records = 2
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "closed-root":
				root.Close()
			case "nonprivate":
				if e := os.Chmod(root.Path(), 0755); e != nil {
					t.Fatal(e)
				}
			}
			r, e := InitializePrivateReadyV2(ctx, root, jb, ab, d, l)
			readyAssertRefused(t, r, e)
			entries, e := os.ReadDir(root.Path())
			if e != nil || len(entries) != 0 {
				t.Fatalf("ASSERT_READY_REFUSE_BEFORE_WRITES: %s %v %v", name, entries, e)
			}
			_ = j
			t.Log("ASSERT_READY_REFUSE_BEFORE_WRITES PASS")
		})
	}
}
func TestPrivateReadyExistingAndPartialState(t *testing.T) {
	for _, name := range []string{"duplicate", "marker-without-current", "conflicting-authorization", "foreign-job"} {
		t.Run(name, func(t *testing.T) {
			j, a, jb, ab, d, l := readyTestInputs(t)
			root := readyTestRoot(t)
			if name == "conflicting-authorization" {
				sel := recordSelectorV2(authorizationsNamespaceV2, a.Identity)
				if x := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: sel, ArtifactSchemaID: authorizationSchemaV2, Bytes: []byte("conflict")}); x.Failure != nil {
					t.Fatal(x.Failure)
				}
			} else {
				target := j
				if name == "foreign-job" {
					target.Scope = hashLabelV2("foreign-scope")
					sealTestV2(t, &target)
					a.Job = target.Identity
					sealTestV2(t, &a)
				}
				if _, e := (&kernelV2{root: root}).initialize(context.Background(), target, a, decisionV2{target.Identity, a.Identity, a.Decision}, time.Now().UnixNano(), verificationLimitsV2{l.Records, l.Bytes, l.Work}); e != nil {
					t.Fatal(e)
				}
				if name == "marker-without-current" {
					if e := os.Remove(filepath.Join(root.Path(), currentSelectorV2(j))); e != nil {
						t.Fatal(e)
					}
				}
			}
			before := readyFiles(t, root.Path())
			r, e := InitializePrivateReadyV2(context.Background(), root, jb, ab, d, l)
			readyAssertRefused(t, r, e)
			after := readyFiles(t, root.Path())
			for p, b := range before {
				if !bytes.Equal(after[p], b) {
					t.Fatalf("ASSERT_READY_RETAINS_PRIOR_BYTES: %s", p)
				}
			}
			t.Log("ASSERT_READY_RETAINS_PRIOR_BYTES PASS")
		})
	}
}
func readyFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			out[p] = b
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestPrivateReadyFixedKernelFaultWitnesses(t *testing.T) {
	for _, name := range []string{"committed-error", "postcommit-cancel", "cancel-after-success", "changed-observed-head", "readback-failure", "root-invalid-after-commit", "publish-failure", "expire-after-commit"} {
		t.Run(name, func(t *testing.T) {
			j, a, jb, ab, d, l := readyTestInputs(t)
			root := readyTestRoot(t)
			k := &kernelV2{root: root}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "expire-after-commit" {
				a.Expires = time.Now().Add(time.Second).UnixNano()
				ab = sealTestV2(t, &a)
				d.Authorization = a.Identity
			}
			k.cas = func(c context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				receipt, e := publication.CompareAndReplaceBoundFile(c, r, s, p, b, v)
				if e != nil {
					return receipt, e
				}
				switch name {
				case "committed-error":
					return receipt, errors.New("TEST committed error")
				case "postcommit-cancel":
					cancel()
					return receipt, ctx.Err()
				case "cancel-after-success":
					cancel()
				case "expire-after-commit":
					time.Sleep(time.Until(time.Unix(0, a.Expires)) + time.Millisecond)
				case "readback-failure":
					if e := os.WriteFile(filepath.Join(r.Path(), s), []byte("{}\n"), 0600); e != nil {
						t.Fatal(e)
					}
				case "root-invalid-after-commit":
					if e := os.Chmod(r.Path(), 0755); e != nil {
						t.Fatal(e)
					}
				case "changed-observed-head":
					var h resourceHeadV2
					if e := parseV2(b, &h); e != nil {
						t.Fatal(e)
					}
					h.At++
					changed := sealTestV2(t, &h)
					for _, sel := range []string{s, initialSelectorV2(j)} {
						if e := os.WriteFile(filepath.Join(r.Path(), sel), changed, 0600); e != nil {
							t.Fatal(e)
						}
					}
				}
				return receipt, nil
			}
			if name == "publish-failure" {
				k.publish = func(q publication.Request) publication.Result {
					if q.Selector == initialSelectorV2(j) {
						return publication.Result{Failure: &publication.Failure{Code: publication.CodePublicationFailed}}
					}
					return publication.NewPublisher().Publish(q)
				}
			}
			r, e := initializePrivateReadyV2(ctx, k, jb, ab, d, l)
			readyAssertRefused(t, r, e)
			if name != "publish-failure" {
				if _, e := root.ReadSelector(initialSelectorV2(j), headLimitV2); e != nil {
					t.Fatalf("ASSERT_READY_PARTIAL_MARKER_RETAINED: %v", e)
				}
			}
			if name == "committed-error" && !errors.Is(e, errUncertainV2) {
				t.Fatalf("ASSERT_READY_UNCERTAIN_PROPAGATED: %v", e)
			}
			t.Log("ASSERT_READY_NO_USABLE_ON_ERROR PASS; fixed kernel seam over actual CAS/publication, not public physical-fault injection")
		})
	}
}
