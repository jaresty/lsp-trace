package inventorysuccessor

import (
	"context"
	"lsp-trace/internal/publication"
	"path/filepath"
	"testing"
	"time"

	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/targetpacket"
)

func ControlledRPrepCanonicalOwnerForTest(t *testing.T) (*PrivateManagedInputOwnerV2, *publication.Root) {
	t.Helper()
	o, root := ControlledManagedInputOwnerForTest(t, false)
	path, e := filepath.EvalSymlinks(root.Path())
	if e != nil {
		t.Fatal(e)
	}
	canonical, e := publication.OpenRoot(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = canonical.Close() })
	reopened, e := OpenPrivateManagedInputOwnerV2(context.Background(), canonical, sealTestV2(t, &o.job), o.head, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	return reopened, canonical
}

func ControlledRPrepShortOwnerForTest(t *testing.T) (*PrivateManagedInputOwnerV2, *publication.Root) {
	t.Helper()
	x := harnessTestV2(t, func(a *authorizationV2) {
		a.Ceiling = countersFromV2([14]uint64{1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12})
		a.MaxTranche = a.Ceiling
		a.MaxTranche.OutputBytes = 8 << 20 // existing mixed quote fits; preparation does not
		a.MaxWindows = 100
		a.MaxWindowNS = uint64(15 * time.Second)
		a.Expires = 4102444800000000000
	})
	path, e := filepath.EvalSymlinks(x.k.root.Path())
	if e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = root.Close() })
	owner, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, sealTestV2(t, &x.j), x.h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	return owner, root
}

func ControlledRPrepCLIJobForTest(t *testing.T, o *PrivateManagedInputOwnerV2) ([]byte, string) {
	t.Helper()
	return sealTestV2(t, &o.job), o.head
}

// Inspect already-paid genuine state without another Snapshot/hash/evaluation.
func ControlledOriginalTargetRetentionForTest(t *testing.T, p PreparedSourceRV1) {
	t.Helper()
	if p.state == nil {
		t.Fatal("ASSERT_ORIGINAL_RETENTION_GENUINE")
	}
	before, ok := p.RetainedPayloadUpper()
	if !ok {
		t.Fatal("ASSERT_ORIGINAL_RETENTION_GENUINE")
	}
	copyState := *p.state
	copyState.originalTarget = originalQueryTargetV1{}
	after, ok := (PreparedSourceRV1{state: &copyState}).RetainedPayloadUpper()
	o := &p.state.originalTarget
	want := uint64(0)
	for _, s := range []string{o.uri, o.sourceDigest, o.sessionID, o.documentVersion, o.encoding, o.symbolID, o.workspace, o.revision, o.privacy} {
		want += uint64(len(s))
	}
	if !ok || want == 0 || before-after != want {
		t.Fatal("ASSERT_ORIGINAL_RETENTION_EXACT_DELTA", before, after, want)
	}
	t.Logf("ASSERT_ORIGINAL_RETENTION_EXACT_DELTA: PASS payload=%d fixed=512 (fixed overhead charged even for empty comparison control)", want)
}

func TestSourceRPrepOriginalTargetZero(t *testing.T) {
	for _, p := range []PreparedSourceRV1{{}, {state: &preparedSourceRStateV1{}}} {
		if p.MatchesOriginalQueryTarget(targetpacket.PrivateSourceUnit{}) {
			t.Fatal("ASSERT_ORIGINAL_UNISSUED_NO_MATCH")
		}
		if id, e := p.OriginalQueryTargetID(nil); e == nil || id != "" {
			t.Fatal("ASSERT_ORIGINAL_UNISSUED_NO_MATCH")
		}
	}
}

func TestSourceRPrepQuote(t *testing.T) {
	now := time.Unix(1000000000, 0)
	m := rg.AllModeMetadataPrivate{Input: rg.CompositionCommitment{ByteLength: 4096}, Contributions: 2, Custody: 4, Predecessors: 7}
	c, e := sourceRPreparationChargeV1(m, now, now.Add(10*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if c.ProviderRequests != 0 || c.ProviderMessages != 0 || c.RequestBytes != 0 || c.ResponseBytes != 0 || c.WireBytes != 0 || c.ActiveNS != uint64(10*time.Second) || c.Work != c.LogicalBytes+36+49152 || c.ReadHashBytes != c.LogicalBytes+3*(64<<20) {
		t.Fatal("ASSERT_RPREP_EXACT_QUOTE", c)
	}
	t.Logf("ASSERT_RPREP_EXACT_QUOTE: PASS %+v", c)
	for _, mutate := range []func(*rg.AllModeMetadataPrivate){func(v *rg.AllModeMetadataPrivate) { v.Input.ByteLength = int(^uint(0) >> 1) }, func(v *rg.AllModeMetadataPrivate) { v.Contributions = int(^uint(0) >> 1) }, func(v *rg.AllModeMetadataPrivate) { v.Custody = -1 }, func(v *rg.AllModeMetadataPrivate) { v.Predecessors = 4097 }} {
		bad := m
		mutate(&bad)
		if _, e := sourceRPreparationChargeV1(bad, now, now.Add(time.Second)); e == nil {
			t.Fatal("ASSERT_RPREP_OVERFLOW_REFUSAL")
		}
	}
	for _, deadline := range []time.Time{now, now.Add(-time.Second), now.Add(16 * time.Second)} {
		if _, e := sourceRPreparationChargeV1(m, now, deadline); e == nil {
			t.Fatal("ASSERT_RPREP_DEADLINE_REFUSAL")
		}
	}
	if p, e := ExecuteRPreparationV1(context.Background(), PrivateSourceRPreparationOperationV1{}, PrivateSourceRPreparationRequestV1{}); e == nil || p.state != nil {
		t.Fatal("ASSERT_RPREP_ZERO_CANNOT_MINT")
	}
}

// Test-only observation of an existing issued owner's actual persisted head.
// Does not expose job bytes, issue funding, reset state or create production APIs.
func ControlledRPrepDebitForTest(t *testing.T, o *PrivateManagedInputOwnerV2) (string, [14]uint64) {
	t.Helper()
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	return h.Disposition, h.Charged.vector()
}
func ControlledRPrepExpectedChargeForTest(t *testing.T, q PrivateSourceRPreparationRequestV1) [14]uint64 {
	t.Helper()
	m, e := sourceRPreparationMetadataV1(q)
	if e != nil {
		t.Fatal(e)
	}
	c, e := sourceRPreparationChargeV1(m, time.Now(), q.Deadline)
	if e != nil {
		t.Fatal(e)
	}
	return c.vector()
}
