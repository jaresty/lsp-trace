package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Synthetic test authorization ONLY, same generous 14-coordinate fixture as the
// existing controlled owner, narrowed to two windows and the unchanged 15s bound.
func ControlledLocalOwnerForTest(t *testing.T) (*PrivateManagedInputOwnerV2, *publication.Root) {
	return controlledLocalOwnerCallsForTest(t, 1e12)
}
func ControlledLocalWorkerLimitedOwnerForTest(t *testing.T) (*PrivateManagedInputOwnerV2, *publication.Root) {
	return controlledLocalOwnerCallsForTest(t, 251000)
}
func controlledLocalOwnerCallsForTest(t *testing.T, calls uint64) (*PrivateManagedInputOwnerV2, *publication.Root) {
	t.Helper()
	x := harnessTestV2(t, func(a *authorizationV2) {
		a.Ceiling = countersFromV2([14]uint64{1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12})
		a.Ceiling.Calls = calls
		a.MaxTranche = a.Ceiling
		a.MaxWindows = 2
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
	t.Cleanup(func() { root.Close() })
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, sealTestV2(t, &x.j), x.h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	return o, root
}
func ControlledLocalHeadForTest(t *testing.T, o *PrivateManagedInputOwnerV2) (string, uint64, [14]uint64) {
	t.Helper()
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	return h.Disposition, h.Windows, h.Charged.vector()
}

func ControlledLocalGrantForTest(t *testing.T, o *PrivateManagedInputOwnerV2) string {
	t.Helper()
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	return h.Authorization
}

// Only the test variant can wrap real descriptor IO. The observer cannot mint or
// bypass owner/head/caps/pending validation. Returning an error only refuses IO.
func ControlledLocalIOForTest(c PrivateSourceLocalConfigV1, observe func(string) error) PrivateSourceLocalConfigV1 {
	c.io = &controlledLocalIOV1{observe: observe}
	return c
}

// Fragmentation changes only the slice passed to the actual os.File.Read.
// It never truncates/changes the pin, invents EOF, or returns replacement bytes.
func ControlledLocalFragmentedIOForTest(c PrivateSourceLocalConfigV1, observe func(string) error) PrivateSourceLocalConfigV1 {
	c.io = &controlledLocalIOV1{observe: observe, maxRead: 1}
	return c
}

func ControlledLocalReservationCallsForTest(t *testing.T, o *PrivateManagedInputOwnerV2, c PrivateSourceLocalConfigV1) (local, tranche, cumulative uint64) {
	t.Helper()
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	budget, e := budgetV2(o.limits)
	if e != nil {
		t.Fatal(e)
	}
	var r reservationV2
	if e = o.k.read(context.Background(), h.ReservationSelector, &r, budget); e != nil {
		t.Fatal(e)
	}
	quote, e := sourceLocalConfigChargeV1(c)
	if e != nil {
		t.Fatal(e)
	}
	return quote.Calls, r.Tranche.Calls, h.Charged.Calls
}

// Observe the actual sealed P tranche, never a caller-supplied quote.
func ControlledLocalPTrancheForTest(t *testing.T, o *PrivateManagedInputOwnerV2) [14]uint64 {
	t.Helper()
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	budget, e := budgetV2(o.limits)
	if e != nil {
		t.Fatal(e)
	}
	var r reservationV2
	if e = o.k.read(context.Background(), h.ReservationSelector, &r, budget); e != nil {
		t.Fatal(e)
	}
	return r.Tranche.vector()
}

// Compiled only into tests; impossible to select from production config/JSON.
func ControlledLocalWorkerIOForTest(c PrivateSourceLocalConfigV1, observe func(string) error) PrivateSourceLocalConfigV1 {
	c.io = &controlledLocalIOV1{observe: observe, workerFixture: true}
	return c
}
func ControlledLocalWorkerProfileForTest() string { return privateLocalSandboxProfileV1 }
func ControlledManagedProfileForTest() string     { return privateManagedSandboxProfileV1 }
func ControlledManagedRootForTest(m PrivateSourceLocalManagedV1) string {
	return m.state.stage.localStageDir
}

func ControlledManagedAdmitForTest(t *testing.T, o *PrivateManagedInputOwnerV2, c PrivateSourceLocalConfigV1) (PrivateSourceLocalManagedV1, error) {
	t.Helper()
	depPath := filepath.Join(filepath.Dir(c.Config.Library.Path), "fake-dependency")
	data := []byte("controlled-explicit-dependency")
	if e := os.WriteFile(depPath, data, 0600); e != nil {
		t.Fatal(e)
	}
	q := PrivateSourceLocalManagedRequestV1{Config: c, Count: 1, MaxBytes: 65536, MaxUses: 4, ExpectedHead: o.PreparationHeadV1(), Attempt: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("controlled-lifecycle"))), Deadline: time.Now().Add(14 * time.Second)}
	q.Dependencies[0] = PrivateSourceLocalDependencyV1{Pin: describeworker.FilePin{Path: depPath, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(data))}, Bytes: uint64(len(data))}
	quote, e := managedArtifactQuoteV1(q, time.Now(), o.limits)
	if e != nil {
		return PrivateSourceLocalManagedV1{}, e
	}
	t.Logf("ASSERT_MANAGED_LIFECYCLE_QUOTE: %v", quote.vector())
	return o.AdmitLocalArtifactsV1(context.Background(), q)
}

type controlledLocalIOV1 struct {
	workerFixture bool
	observe       func(string) error
	maxRead       int
}

func (c *controlledLocalIOV1) controlledWorkerFixture() bool    { return c.workerFixture }
func (c *controlledLocalIOV1) observeEffect(event string) error { return c.observe(event) }

func (c *controlledLocalIOV1) open(path string) (privateLocalFileV1, error) {
	if e := c.observe("open"); e != nil {
		return nil, e
	}
	f, e := openLocalPinV1(path)
	if e != nil {
		return nil, e
	}
	return &controlledLocalFileV1{f, c.observe, c.maxRead}, nil
}

type controlledLocalFileV1 struct {
	*os.File
	observe func(string) error
	maxRead int
}

func (f *controlledLocalFileV1) Read(b []byte) (int, error) {
	if e := f.observe("read"); e != nil {
		return 0, e
	}
	if f.maxRead > 0 && len(b) > f.maxRead {
		b = b[:f.maxRead]
	}
	return f.File.Read(b)
}
func (f *controlledLocalFileV1) Stat() (os.FileInfo, error) {
	if e := f.observe("stat"); e != nil {
		return nil, e
	}
	return f.File.Stat()
}
func (f *controlledLocalFileV1) Close() error {
	e := f.observe("close")
	actual := f.File.Close()
	return errors.Join(e, actual, f.observe("closed"))
}
