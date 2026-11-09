//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

// Independent arithmetic from trusted existing verification limits. Two bounded
// verifications, sixteen head-sized commit/readback allowances, eight closed
// decode/validate/canonical/hash/copy passes. This is a synthetic test grant,
// not a production grant or a measured physical-resource claim.
func reviewVerificationChargeV1(l verificationLimitsV2) countersV2 {
	data := 8 * (2*l.Bytes + 16*headLimitV2)
	return countersV2{LogicalBytes: data, ReadHashBytes: data, WriteBytes: 4 * (recordLimitV2 + 2*headLimitV2), Records: 2*l.Records + 16, ParserBytes: data, Work: data + 2*l.Work + 64, Calls: 2*l.Work + 64, OutputBytes: 8*(headLimitV2+recordLimitV2) + 128*l.Records}
}
func reviewHistoryOwnerV1(t *testing.T, funded bool) (*PrivateManagedInputOwnerV2, uint64, uint64) {
	t.Helper()
	l := verificationLimitsV2{4096, 64 << 20, 16384}
	fee := reviewVerificationChargeV1(l).ParserBytes + 16384
	path := t.TempDir()
	if e := os.Chmod(path, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	j, a, d := fixtureV2(t)
	values := make([]string, 32)
	for i := range values {
		values[i] = hashLabelV2(fmt.Sprintf("controlled-history-%02d", i))
	}
	sort.Strings(values)
	j.EvidencePolicies = append([]string(nil), values...)
	j.ProviderPolicies = append([]string(nil), values...)
	j.SemanticPolicies = append([]string(nil), values...)
	sealTestV2(t, &j)
	a.Job = j.Identity
	a.Ceiling = countersFromV2([14]uint64{1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12})
	a.Ceiling.ParserBytes = fee + 16384
	if funded {
		a.Ceiling.ParserBytes = 2 * fee
	}
	a.MaxTranche = a.Ceiling
	a.MaxTranche.ParserBytes = fee
	a.MaxWindows = 2
	a.MaxWindowNS = uint64(15 * time.Second)
	a.Expires = 4102444800000000000
	sealTestV2(t, &a)
	d.Job = j.Identity
	d.Authorization = a.Identity
	d.Reference = a.Decision
	k := &kernelV2{root: root}
	h, e := k.initialize(context.Background(), j, a, d, 10, l)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	attempt := hashLabelV2("controlled-prior-metadata-window")
	prior := reviewVerificationChargeV1(l)
	prior.ParserBytes = fee
	prior.ActiveNS = uint64(time.Second)
	h, e = k.reserve(context.Background(), j, h.Identity, attempt, prior, now.Add(time.Second).UnixNano(), now.UnixNano(), l)
	if e != nil {
		t.Fatal(e)
	}
	h, e = k.observe(context.Background(), j, h.Identity, attempt, "OBSERVED", time.Now().UnixNano(), l)
	if e != nil {
		t.Fatal(e)
	}
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, sealTestV2(t, &j), h.Identity, PrivateManagedInputVerificationV2{l.Records, l.Bytes, l.Work})
	if e != nil {
		t.Fatal(e)
	}
	// Measurement is test validation only. Production quote arithmetic must not
	// read history to learn its fee, nor use these observed bytes as authority.
	b, e := budgetV2(l)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = k.verify(context.Background(), j, b, noneV2); e != nil {
		t.Fatal(e)
	}
	history := l.Bytes - b.bytes
	if history <= 16384 {
		t.Fatal("ASSERT_REVIEW_GENUINE_HISTORY_OVER_16K", history)
	}
	return o, fee, history
}
func TestSourceLocalManagedReviewParserBounds(t *testing.T) {
	_, q, opens := managedFixtureV1(t)
	for _, l := range []verificationLimitsV2{{0, 64 << 20, 16384}, {4096, ^uint64(0), 16384}, {4096, 64 << 20, ^uint64(0)}} {
		if _, e := managedArtifactQuoteV1(q, time.Now(), l); e == nil {
			t.Error("ASSERT_REVIEW_PARSER_TRUSTED_BOUND_REFUSAL", l)
		}
	}
	if *opens != 0 {
		t.Fatal("ASSERT_REVIEW_PARSER_BOUND_BEFORE_IO", *opens)
	}
	t.Log("ASSERT_REVIEW_PARSER_TRUSTED_BOUND_REFUSAL: PASS; invalid/overflowing bounds cannot quote or create IO credit")
}
func TestSourceLocalManagedReviewParser(t *testing.T) {
	for _, funded := range []bool{false, true} {
		name := "insufficient"
		if funded {
			name = "funded"
		}
		t.Run(name, func(t *testing.T) {
			_, q, opens := managedFixtureV1(t)
			o, fee, history := reviewHistoryOwnerV1(t, funded)
			q.ExpectedHead = o.PreparationHeadV1()
			q.Deadline = time.Now().Add(14 * time.Second)
			callbackBytes, callbackCount := uint64(0), 0
			o.k.cas = func(ctx context.Context, root *publication.Root, selector string, predecessor publication.BoundFilePredecessor, raw []byte, verify func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				return publication.CompareAndReplaceBoundFile(ctx, root, selector, predecessor, raw, func(b []byte) error { callbackBytes += uint64(len(b)); callbackCount++; return verify(b) })
			}
			quoted, e := managedArtifactQuoteV1(q, time.Now(), o.limits)
			if e != nil {
				t.Fatal(e)
			}
			if quoted.ParserBytes < fee {
				t.Errorf("ASSERT_REVIEW_PARSER_COMPLETE_QUOTE: quoted=%d required=%d history=%d", quoted.ParserBytes, fee, history)
			}
			g, e := o.AdmitLocalArtifactsV1(context.Background(), q)
			if g.state != nil {
				t.Cleanup(func() { ControlledManagedCloseForTest(g) })
			}
			d, w, debit := ControlledLocalHeadForTest(t, o)
			if !funded {
				if e == nil || g.state != nil || *opens != 0 || d != "OBSERVED" || w != 1 || debit[4] != fee {
					t.Errorf("ASSERT_REVIEW_PARSER_BEFORE_ARTIFACT_IO: error=%v generation=%v opens=%d head=%s windows=%d parser=%d", e, g.state != nil, *opens, d, w, debit[4])
				}
				if !t.Failed() {
					t.Log("ASSERT_REVIEW_PARSER_BEFORE_ARTIFACT_IO: PASS; funded predecessor retained; no lifecycle reservation or artifact IO")
				}
				return
			}
			if e != nil || g.state == nil || *opens != 7 || d != "PENDING" || w != 2 {
				t.Fatalf("ASSERT_REVIEW_PARSER_FUNDED_LIFECYCLE: %v opens=%d head=%s windows=%d", e, *opens, d, w)
			}
			if debit[4]-fee < fee {
				t.Errorf("ASSERT_REVIEW_PARSER_SEALED_DEBIT: lifecycle=%d required=%d", debit[4]-fee, fee)
			}
			b, _ := budgetV2(o.limits)
			if _, _, e = o.k.verify(context.Background(), o.job, b, noneV2); e != nil {
				t.Fatal(e)
			}
			pendingBytes := o.limits.Bytes - b.bytes
			if quoted.ParserBytes < history+pendingBytes {
				t.Errorf("ASSERT_REVIEW_PARSER_ACTUAL_PASSES: quote=%d reads=%d", quoted.ParserBytes, history+pendingBytes)
			}
			if e = g.Retire(context.Background()); e != nil {
				t.Fatal(e)
			}
			if callbackCount == 0 || callbackCount > 16 || quoted.ParserBytes < 8*(history+pendingBytes+callbackBytes) {
				t.Errorf("ASSERT_REVIEW_PARSER_COMMIT_PASSES: callbacks=%d bytes=%d quote=%d", callbackCount, callbackBytes, quoted.ParserBytes)
			}
			d, w, final := ControlledLocalHeadForTest(t, o)
			if d != "OBSERVED" || w != 2 || final != debit {
				t.Fatal("ASSERT_REVIEW_PARSER_NO_RESET", d, w, final, debit)
			}
			if !t.Failed() {
				t.Logf("ASSERT_REVIEW_PARSER_COMPLETE_QUOTE: PASS; ASSERT_REVIEW_PARSER_FUNDED_LIFECYCLE: PASS; history=%d pending=%d commit_callbacks=%d commit_bytes=%d quote=%d", history, pendingBytes, callbackCount, callbackBytes, quoted.ParserBytes)
			}
		})
	}
}
