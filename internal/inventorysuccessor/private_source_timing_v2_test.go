//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"path/filepath"
	"testing"
	"time"
)

func ControlledHOSTTimingSnapshotForTest(o *PrivateManagedInputOwnerV2) (string, string, [14]uint64, uint64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	return h.Identity, h.Disposition, h.Charged.vector(), h.Windows, e
}
func ControlledHOSTTimingOwnerForTest(t *testing.T, p PrivateSourceTimingV2, w sessionruntime.PrivateProviderWindowBounds, shortage ...int) (*PrivateManagedInputOwnerV2, *publication.Root) {
	t.Helper()
	l := PrivateManagedInputVerificationV2{4096, 64 << 20, 16384}
	mixed, _, sum, tranche, window, e := PrivateSourceHOSTQuotesV2(p, w, l)
	if e != nil {
		t.Fatal(e)
	}
	j, a, _ := fixtureV2(t)
	j.ResourceRules = p.Rules()
	jb := sealTestV2(t, &j)
	a.Job = j.Identity
	a.Expires = time.Now().Add(time.Duration(window) + time.Minute).UnixNano()
	a.MaxWindowNS = window
	a.MaxWindows = 2
	if len(shortage) > 0 {
		i := shortage[0]
		if i == -2 {
			sum[0] = mixed[0]
			tranche[0] = min(tranche[0], sum[0])
		} else if i >= 0 && i < 14 {
			sum[i] = 0
			tranche[i] = 0
		}
	}
	a.Ceiling = countersFromV2(sum)
	a.MaxTranche = countersFromV2(tranche)
	ab := sealTestV2(t, &a)
	root := readyTestRoot(t)
	ready, e := InitializePrivateReadyV2(context.Background(), root, jb, ab, PrivateReadyDecisionAssertionV2{j.Identity, a.Identity, a.Decision}, l)
	if e != nil {
		t.Fatal(e)
	}
	path, e := filepath.EvalSymlinks(root.Path())
	if e != nil {
		t.Fatal(e)
	}
	canonical, e := publication.OpenRoot(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = canonical.Close() })
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), canonical, ready.CanonicalJob(), ready.ConfirmedHead(), l)
	if e != nil {
		t.Fatal(e)
	}
	return o, canonical
}
