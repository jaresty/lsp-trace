//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/censusacquisition"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

// The only injected seam is the kernel's pre-existing accounting CAS. There
// is no enumerator, provider, native-result, or production fixture seam.
func TestPreparationEnumerationFundingOrderV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_FUNDING"
	owner, x, i := preparationEnumerationFixtureV1(t, -1, false)
	reserved := false
	owner.state.k.cas = func(ctx context.Context, root *publication.Root, selector string, prior publication.BoundFilePredecessor, raw []byte, verify func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		receipt, e := publication.CompareAndReplaceBoundFile(ctx, root, selector, prior, raw, verify)
		var h resourceHeadV2
		if e == nil && receipt != nil && receipt.Committed && parseV2(raw, &h) == nil && h.Disposition == "PENDING" {
			reserved = true
			if e = os.WriteFile(filepath.Join(i.SourceRoot, "only-after-reservation.go"), []byte("package bounded\n"), 0600); e != nil {
				return receipt, e
			}
		}
		return receipt, e
	}
	r, e := owner.Enumerate(context.Background(), i, i.Attempt)
	h, re := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || re != nil || !reserved || len(r.Files()) != 2 || h.Windows != 1 {
		t.Fatal(a, "FAIL reservation precedes enumeration", e, re, r.Files())
	}
	t.Log(a + ": PASS reservation-before-enumeration")
}
func TestPreparationEnumerationUncertainV1(t *testing.T) {
	for _, mode := range []string{"reservation-cas", "reservation-readback", "completion-cas", "completion-readback"} {
		t.Run(mode, func(t *testing.T) {
			const a = "ASSERT_PREPARATION_RESULT"
			owner, x, i := preparationEnumerationFixtureV1(t, -1, false)
			target := "PENDING"
			if mode == "completion-cas" || mode == "completion-readback" {
				target = "OBSERVED"
			}
			injected := false
			owner.state.k.cas = func(ctx context.Context, root *publication.Root, selector string, prior publication.BoundFilePredecessor, raw []byte, verify func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				receipt, e := publication.CompareAndReplaceBoundFile(ctx, root, selector, prior, raw, verify)
				var h resourceHeadV2
				if e == nil && receipt != nil && receipt.Committed && parseV2(raw, &h) == nil && h.Disposition == target {
					injected = true
					if mode == "reservation-cas" || mode == "completion-cas" {
						return receipt, errors.New("controlled committed accounting uncertainty")
					}
					// Install a canonical current hint without its chain object. A current
					// filename alone is not successful chain/completion readback.
					h.Identity = ""
					h.Attempt = hashLabelV2("foreign-attempt")
					changed, err := sealV2(&h)
					if err != nil {
						return receipt, err
					}
					_, err = publication.CompareAndReplaceBoundFile(ctx, root, selector, publication.BoundFilePredecessor{Selector: selector, Digest: digest(raw), ByteLength: uint64(len(raw))}, changed, verify)
					if err != nil {
						return receipt, err
					}
				}
				return receipt, e
			}
			r, e := owner.Enumerate(context.Background(), i, i.Attempt)
			if !injected || !errors.Is(e, ErrPrivatePreparationEnumerationV1) || r.Available() || len(r.Files()) != 0 {
				t.Fatal(a, "FAIL uncertain completion exposed data", mode, injected, e)
			}
			// selected inspects the retained hint even when chain verification intentionally
			// fails. It is not treated as permission or successful readback.
			b, err := budgetV2(x.l)
			if err != nil {
				t.Fatal(err)
			}
			h, _, err := x.k.selected(context.Background(), x.j, b)
			if err != nil || h.Charged == (countersV2{}) || h.Windows != 1 {
				t.Fatal("ASSERT_PREPARATION_NO_REFUND FAIL uncertain debit", err, h.Charged)
			}
			if retry, err := owner.Enumerate(context.Background(), i, i.Attempt); err == nil || retry.Available() {
				t.Fatal("ASSERT_PREPARATION_REPLAY FAIL uncertain retry")
			}
			t.Log(a + ": PASS uncertain readback withheld; charge retained")
		})
	}
}
func TestPreparationEnumerationConcurrentV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_REPLAY"
	first, x, i := preparationEnumerationFixtureV1(t, -1, false)
	raw := sealTestV2(t, &x.j)
	second, e := OpenPrivatePreparationEnumeratorV1(context.Background(), x.k.root, raw, x.h.Identity, PrivateManagedInputVerificationV2{128, 4 << 20, 128})
	if e != nil {
		t.Fatal(e)
	}
	start := make(chan struct{})
	out := make(chan bool, 2)
	var wg sync.WaitGroup
	for _, owner := range []PrivatePreparationEnumeratorV1{first, second} {
		wg.Add(1)
		go func(o PrivatePreparationEnumeratorV1) {
			defer wg.Done()
			<-start
			r, e := o.Enumerate(context.Background(), i, i.Attempt)
			out <- e == nil && r.Available()
		}(owner)
	}
	close(start)
	wg.Wait()
	close(out)
	successes := 0
	for ok := range out {
		if ok {
			successes++
		}
	}
	h, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || successes != 1 || h.Windows != 1 {
		t.Fatal(a, "FAIL independent-handle race", successes, e)
	}
	t.Log(a + ": PASS one concurrent winner")
}
func TestPreparationEnumerationEscapedQuoteV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_FUNDING"
	_, _, i := preparationEnumerationFixtureV1(t, -1, false)
	i.Limits.MaxEntries = 65
	i.Limits.MaxAccepted = 49
	i.Limits.MaxWork = 256
	i.Limits.MaxPathBytes = 4096
	i.Limits.MaxDepth = 32
	q, ok := PrivatePreparationEnumerationQuoteV1(i, PrivateManagedInputVerificationV2{128, 4 << 20, 128}, time.Now())
	if !ok {
		t.Fatal("SETUP_ESCAPE_QUOTE")
	}
	// Fifteen shared directories, their root and 49 files fit 65 records.
	stem := strings.Repeat(strings.Repeat("&", 244)+"/", 15)
	var files []censusacquisition.SourceFile
	for n := 0; n < 49; n++ {
		name := stem + strings.Repeat("&", 200) + fmt.Sprintf("%02d.go", n)
		uri := (&url.URL{Scheme: "file", Path: filepath.Join(i.SourceRoot, name)}).String()
		files = append(files, censusacquisition.SourceFile{Path: name, URI: uri, LanguageID: "go"})
	}
	raw, e := json.Marshal(files)
	if e != nil {
		t.Fatal(e)
	}
	if uint64(len(raw)) > q[13] {
		t.Fatal(a, "FAIL escaped metadata exceeds quoted output", len(raw), q[13])
	}
	t.Log(a + ": PASS worst-case escaped metadata envelope")
}

func TestPreparationEnumerationBoundsV1(t *testing.T) {
	const a = "ASSERT_PREPARATION_FUNDING"
	_, _, i := preparationEnumerationFixtureV1(t, -1, false)
	v := PrivateManagedInputVerificationV2{128, 4 << 20, 128}
	now := time.Now()
	q, ok := PrivatePreparationEnumerationQuoteV1(i, v, now)
	if !ok || q[5] != 0 || q[6] != 0 || q[10] != 0 || q[11] != 0 || q[12] != 0 || q[0] > 64<<20 || q[7] > 1<<32 || q[8] > uint64(15*time.Second) {
		t.Fatal(a, "FAIL metadata-only bounded quote", q)
	}
	for _, kind := range []string{"expired", "long", "entries", "work", "path", "depth", "verification-records", "verification-bytes", "verification-work", "output"} {
		candidate := preparationIntentCopyV1(i)
		verification := v
		switch kind {
		case "expired":
			candidate.Deadline = now
		case "long":
			candidate.Deadline = now.Add(16 * time.Second)
		case "entries":
			candidate.Limits.MaxEntries = 1000001
		case "work":
			candidate.Limits.MaxWork = 1 << 27
		case "path":
			candidate.Limits.MaxPathBytes = 4097
		case "depth":
			candidate.Limits.MaxDepth = 65
		case "verification-records":
			verification.Records = 129
		case "verification-bytes":
			verification.Bytes = 4<<20 + 1
		case "verification-work":
			verification.Work = 129
		case "output":
			candidate.Limits.MaxEntries = 1000000
		}
		if _, ok := PrivatePreparationEnumerationQuoteV1(candidate, verification, now); ok {
			t.Fatal(a, "FAIL unbounded quote", kind)
		}
	}
	t.Log(a + ": PASS finite limits; no provider reservation")
}
