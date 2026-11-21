//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

func preparationEnumerationFixtureV1(t *testing.T, underfunded int, limit bool) (PrivatePreparationEnumeratorV1, harnessV2, PrivatePreparationIntentV1) {
	t.Helper()
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(workspace, "a.go"), []byte("package bounded\n"), 0600); e != nil {
		t.Fatal(e)
	}
	i := PrivatePreparationIntentV1{Schema: "private-preparation-intent.v1", SourceRoot: workspace, Attempt: hashLabelV2("preparation-attempt"), Roots: []string{"."}, Limits: source.Limits{MaxEntries: 32, MaxAccepted: 16, MaxWork: 64, MaxPathBytes: 256, MaxDepth: 8}, Deadline: time.Now().Add(10 * time.Second)}
	if limit {
		i.Limits.MaxEntries = 1
		i.Limits.MaxAccepted = 1
	}
	j, a, _ := fixtureV2(t)
	j.Scope = PrivatePreparationEnumerationScopeV1()
	j.SourceAuthorization = PrivatePreparationIntentIdentityV1(i)
	j.ResourceRules = PrivatePreparationEnumerationRulesV1(i)
	raw := sealTestV2(t, &j)
	a.Job = j.Identity
	values := [14]uint64{}
	for n := range values {
		values[n] = 1e15
	}
	if underfunded >= 0 {
		values[underfunded] = 0
	}
	a.Ceiling = countersFromV2(values)
	a.MaxTranche = a.Ceiling
	a.MaxWindows = 1
	a.MaxWindowNS = uint64(15 * time.Second)
	a.Expires = time.Now().Add(time.Minute).UnixNano()
	sealTestV2(t, &a)
	accounting := t.TempDir()
	if e := os.Chmod(accounting, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(accounting)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	k := &kernelV2{root: root}
	v := verificationLimitsV2{128, 4 << 20, 128}
	h, e := k.initialize(context.Background(), j, a, decisionV2{j.Identity, a.Identity, a.Decision}, time.Now().UnixNano(), v)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := OpenPrivatePreparationEnumeratorV1(context.Background(), root, raw, h.Identity, PrivateManagedInputVerificationV2{128, 4 << 20, 128})
	if e != nil {
		t.Fatal(e)
	}
	return owner, harnessV2{k, j, a, h, v}, i
}
func TestPreparationEnumerationIntentV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_INTENT"
	owner, _, i := preparationEnumerationFixtureV1(t, -1, false)
	id := PrivatePreparationIntentIdentityV1(i)
	if !hashV2(id) {
		t.Fatal(a, "FAIL valid content-free intent")
	}
	mutations := []func(*PrivatePreparationIntentV1){func(v *PrivatePreparationIntentV1) { v.Roots = []string{"sub"} }, func(v *PrivatePreparationIntentV1) { v.Includes = []string{"*.go"} }, func(v *PrivatePreparationIntentV1) { v.Excludes = []string{"skip.go"} }, func(v *PrivatePreparationIntentV1) { v.PrunedDirectories = []string{"skip"} }, func(v *PrivatePreparationIntentV1) { v.Limits.MaxWork++ }, func(v *PrivatePreparationIntentV1) { v.Deadline = v.Deadline.Add(time.Millisecond) }}
	for n, mutate := range mutations {
		v := preparationIntentCopyV1(i)
		mutate(&v)
		if PrivatePreparationIntentIdentityV1(v) == id {
			t.Fatal(a, "FAIL unbound intent", n)
		}
	}
	for _, kind := range []string{"schema", "absolute-scope", "escape", "zero", "duplicate", "empty-deadline"} {
		v := preparationIntentCopyV1(i)
		switch kind {
		case "schema":
			v.Schema = "wrong"
		case "absolute-scope":
			v.Roots = []string{"/other"}
		case "escape":
			v.Roots = []string{"../other"}
		case "zero":
			v.Limits.MaxWork = 0
		case "duplicate":
			v.Roots = []string{".", "."}
		case "empty-deadline":
			v.Deadline = time.Time{}
		}
		if PrivatePreparationIntentIdentityV1(v) != "" {
			t.Fatal(a, "FAIL invalid intent", kind)
		}
	}
	foreign := preparationIntentCopyV1(i)
	foreign.Excludes = []string{"a.go"}
	if r, e := owner.Enumerate(context.Background(), foreign, i.Attempt); e == nil || r.Available() {
		t.Fatal(a, "FAIL foreign authorized intent")
	}
	t.Log(a + ": PASS")
}
func TestPreparationEnumerationAttemptV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_INTENT"
	owner, x, i := preparationEnumerationFixtureV1(t, -1, false)
	r, e := owner.Enumerate(context.Background(), i, hashLabelV2("not-approved-attempt"))
	h, re := x.k.reopen(context.Background(), x.j, x.l)
	if e == nil || r.Available() || re != nil || h.Windows != 0 {
		t.Fatal(a, "FAIL unapproved attempt", e, re)
	}
	t.Log(a + ": PASS approved-attempt binding")
}

func TestPreparationEnumerationFundingV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_FUNDING"
	for _, currency := range []int{0, 1, 2, 3, 4, 7, 8, 9, 13} {
		owner, x, i := preparationEnumerationFixtureV1(t, currency, false)
		r, e := owner.Enumerate(context.Background(), i, i.Attempt)
		h, re := x.k.reopen(context.Background(), x.j, x.l)
		if !errors.Is(e, ErrPrivatePreparationEnumerationV1) || r.Available() || re != nil || h.Charged != (countersV2{}) || h.Windows != 0 {
			t.Fatal(a, "FAIL underfunded effects", currency, e, re)
		}
	}
	t.Log(a + ": PASS")
}
func TestPreparationEnumerationReplayV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_REPLAY"
	owner, x, i := preparationEnumerationFixtureV1(t, -1, false)
	attempt := i.Attempt
	first, e := owner.Enumerate(context.Background(), i, attempt)
	if e != nil || !first.Available() {
		t.Fatal("SETUP_ENUMERATION", e)
	}
	copy := owner
	if r, e := copy.Enumerate(context.Background(), i, attempt); e == nil || r.Available() {
		t.Fatal(a, "FAIL copied owner replay")
	}
	raw := sealTestV2(t, &x.j)
	h, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil {
		t.Fatal(e)
	}
	fresh, e := OpenPrivatePreparationEnumeratorV1(context.Background(), x.k.root, raw, h.Identity, PrivateManagedInputVerificationV2{128, 4 << 20, 128})
	if e == nil {
		if r, err := fresh.Enumerate(context.Background(), i, i.Attempt); err == nil || r.Available() {
			t.Fatal(a, "FAIL reopened job replay")
		}
	}
	t.Log(a + ": PASS")
}
func TestPreparationEnumerationNoRefundV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_NO_REFUND"
	owner, x, i := preparationEnumerationFixtureV1(t, -1, true)
	var reserved countersV2
	owner.state.k.cas = func(ctx context.Context, root *publication.Root, selector string, prior publication.BoundFilePredecessor, raw []byte, verify func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		receipt, e := publication.CompareAndReplaceBoundFile(ctx, root, selector, prior, raw, verify)
		var h resourceHeadV2
		if e == nil && receipt != nil && receipt.Committed && parseV2(raw, &h) == nil && h.Disposition == "PENDING" {
			reserved = h.Charged
		}
		return receipt, e
	}
	r, e := owner.Enumerate(context.Background(), i, i.Attempt)
	h, re := x.k.reopen(context.Background(), x.j, x.l)
	if reserved == (countersV2{}) {
		t.Fatal("SETUP_NO_RESERVATION: not a no-refund witness")
	}
	if e == nil || r.Available() || re != nil || h.Windows != 1 || h.Disposition != "FAILED" || h.Charged != reserved {
		t.Fatal(a, "FAIL failure debit", e, re, h.Disposition, h.Charged)
	}
	before := h.Charged
	if r, e = owner.Enumerate(context.Background(), i, i.Attempt); e == nil || r.Available() {
		t.Fatal(a, "FAIL refused-window reused")
	}
	h, re = x.k.reopen(context.Background(), x.j, x.l)
	if re != nil || h.Charged != before {
		t.Fatal(a, "FAIL refund after refusal")
	}
	t.Log(a + ": PASS")
}
func TestPreparationEnumerationResultV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_RESULT"
	owner, x, i := preparationEnumerationFixtureV1(t, -1, false)
	r, e := owner.Enumerate(context.Background(), i, i.Attempt)
	if e != nil || !r.Available() {
		t.Fatal(a, "FAIL completion handoff", e)
	}
	f := r.Facts()
	h, re := x.k.reopen(context.Background(), x.j, x.l)
	if re != nil || !hashV2(f.ReservationHead) || f.CompletionHead != h.Identity || h.Disposition != "OBSERVED" || f.Intent != PrivatePreparationIntentIdentityV1(i) || f.Authority != 0 || f.Accepted || f.Assurance != assuranceV2 || f.Completeness != "UNKNOWN" {
		t.Fatal(a, "FAIL completion readback", f, re)
	}
	files := r.Files()
	if len(files) != 1 || files[0].Path != "a.go" {
		t.Fatal(a, "FAIL existing enumeration", files)
	}
	files[0].Path = "changed"
	if r.Files()[0].Path != "a.go" {
		t.Fatal(a, "FAIL detached metadata")
	}
	if !hashV2(f.Intent) || f.Intent == f.CompletionHead {
		t.Fatal(a, "FAIL intent conflated with execution")
	}
	t.Log(a + ": PASS")
}
