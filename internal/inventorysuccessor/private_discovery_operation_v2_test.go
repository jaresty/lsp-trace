package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func discoverySyntheticOwnerV2(t *testing.T) (*PrivateDiscoveryOwnerV2, harnessV2, PrivateDiscoveryBindingV2) {
	return discoverySyntheticCeilingV2(t, -1)
}
func discoverySyntheticCeilingV2(t *testing.T, underfunded int) (*PrivateDiscoveryOwnerV2, harnessV2, PrivateDiscoveryBindingV2) {
	return discoverySyntheticConfigurationV2(t, underfunded, nil)
}
func discoverySyntheticConfigurationV2(t *testing.T, underfunded int, c *sessionruntime.PrivateDiscoveryConstruction) (*PrivateDiscoveryOwnerV2, harnessV2, PrivateDiscoveryBindingV2) {
	t.Helper()
	l := sessionruntime.PrivateDiscoveryLimits{SourceBytes: 1025, RequestBytes: 65536, ResponseBytes: 65536, WireBytes: 131072, RetainedBytes: 1 << 20, Work: 1 << 24, Messages: 16, Requests: 8, FrameBytes: 8192, HeaderBytes: 1024}
	b := PrivateDiscoveryBindingV2{Revision: "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73", PlanIdentity: hashLabelV2("plan"), DenominatorDigest: hashLabelV2("full source denominator"), MemberDigest: hashLabelV2("members"), Workspace: "/synthetic", RuntimeFingerprint: hashLabelV2("runtime"), Deadline: time.Now().Add(10 * time.Second), Limits: l}
	if c != nil {
		b.MemberDigest, b.RuntimeFingerprint = PrivateDiscoveryConstructionIdentityV2(*c)
		b.Workspace = c.Start.Profile.Workspace().String()
		b.Deadline = c.Deadline
		b.Limits = c.Limits
	}
	j, a, _ := fixtureV2(t)
	j.Scope = PrivateDiscoveryScopeDigestV2()
	j.SourceAuthorization = b.DenominatorDigest
	j.ResourceRules = PrivateDiscoveryResourceRulesV2(b)
	raw := sealTestV2(t, &j)
	a.Job = j.Identity
	v := [14]uint64{}
	for i := range v {
		v[i] = 1e15
	}
	if underfunded >= 0 {
		v[underfunded] = 0
	}
	a.Ceiling = countersFromV2(v)
	a.MaxTranche = a.Ceiling
	a.MaxWindowNS = uint64(15 * time.Second)
	a.Expires = time.Now().Add(time.Minute).UnixNano()
	sealTestV2(t, &a)
	path := t.TempDir()
	if e := os.Chmod(path, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	k := &kernelV2{root: root}
	limits := verificationLimitsV2{400, 4 << 20, 400}
	h, e := k.initialize(context.Background(), j, a, decisionV2{j.Identity, a.Identity, a.Decision}, time.Now().UnixNano(), limits)
	if e != nil {
		t.Fatal(e)
	}
	x := harnessV2{k, j, a, h, limits}
	b.PolicyDigest = PrivateDiscoveryPolicyDigestV2(raw)
	o, e := OpenPrivateDiscoveryOwnerV2(context.Background(), root, raw, h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	return o, x, b
}

type discoveryKernelChildV2 struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	output *io.PipeWriter
	once   sync.Once
}

func newDiscoveryKernelChildV2() *discoveryKernelChildV2 {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &discoveryKernelChildV2{input: input, stdin: stdin, stdout: stdout, output: output}
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			m, e := reader.Read()
			if e != nil || m.Method == "exit" {
				return
			}
			if len(m.ID) == 0 {
				continue
			}
			result := json.RawMessage(`[]`)
			if m.Method == "initialize" {
				result = json.RawMessage(`{"capabilities":{"documentSymbolProvider":true,"callHierarchyProvider":true}}`)
			}
			if m.Method == "shutdown" {
				result = json.RawMessage(`null`)
			}
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: result}) != nil {
				return
			}
		}
	}()
	return c
}
func (c *discoveryKernelChildV2) Stdin() io.WriteCloser { return c.stdin }
func (c *discoveryKernelChildV2) Stdout() io.ReadCloser { return c.stdout }
func (c *discoveryKernelChildV2) Teardown(context.Context) managedprocess.TeardownObservation {
	c.once.Do(func() { c.input.Close(); c.stdin.Close(); c.stdout.Close(); c.output.Close() })
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}, Census: managedprocess.GroupCensusObservation{Bounded: true}}
}
func (c *discoveryKernelChildV2) Close() managedprocess.ResourceObservation {
	c.Teardown(context.Background())
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type discoveryKernelStarterV2 struct{ child sessionruntime.Child }

func (s discoveryKernelStarterV2) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func TestPrivateDiscoveryV2FundedClosedRuntime(t *testing.T) {
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	selected, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "synthetic", Workspace: workspace, Profile: "go", EnvironmentReference: "offline"})
	if e != nil {
		t.Fatal(e)
	}
	c := sessionruntime.PrivateDiscoveryConstruction{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (sessionruntime.MaxDocumentSupplyBytes + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}}
	for i := 0; i < 16; i++ {
		path := filepath.Join(workspace, fmt.Sprintf("f%d.go", i))
		if e = os.WriteFile(path, []byte("package synthetic\n"), 0600); e != nil {
			t.Fatal(e)
		}
		c.Members = append(c.Members, sessionruntime.PrivateDiscoveryMember{Path: filepath.Base(path), URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"})
	}
	owner, x, b := discoverySyntheticConfigurationV2(t, -1, &c)
	factories := 0
	c.NewStarter = func() (sessionruntime.Starter, error) {
		factories++
		h, e := x.k.reopen(context.Background(), x.j, x.l)
		if e != nil || h.Disposition != "PENDING" || h.Windows != 1 {
			t.Fatal("ASSERT_DISCOVERY_CONFIRMED_BEFORE_FACTORY", e)
		}
		for dimension, value := range h.Charged.vector() {
			if value == 0 {
				t.Fatalf("ASSERT_DISCOVERY_ALL_CURRENCIES_BEFORE_FACTORY dimension=%d", dimension)
			}
		}
		return discoveryKernelStarterV2{newDiscoveryKernelChildV2()}, nil
	}
	op, e := owner.ReservePartition(context.Background(), b, hashLabelV2("funded-closed-runtime"))
	if e != nil || factories != 0 {
		t.Fatal("reserve", e)
	}
	supplied := 0
	e = op.Execute(context.Background(), c, func(ctx context.Context, s *sessionruntime.PrivateDiscoverySession) error {
		for _, member := range c.Members {
			if e := s.Supply(ctx, member.URI, member.LanguageID); e != nil {
				return e
			}
			supplied++
			if _, e := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: member.URI}}); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil || factories != 1 || supplied != 16 {
		t.Fatal("ASSERT_DISCOVERY_REAL_OWNER_CLOSED_RUNTIME", factories, supplied, e)
	}
	if e = op.Complete(context.Background()); e != nil {
		t.Fatal("ASSERT_DISCOVERY_OBSERVE_AFTER_JOIN", e)
	}
	h, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || h.Disposition != "OBSERVED" || h.Windows != 1 {
		t.Fatal("ASSERT_DISCOVERY_REAL_KERNEL_OBSERVATION", e)
	}
	// A second fresh temporary kernel proves the concrete production starter's
	// supervisor constructor is reached only after all fourteen charges. Empty
	// process Path deliberately refuses before pipe creation or execution.
	actualOwner, actualKernel, actualBinding := discoverySyntheticConfigurationV2(t, -1, &c)
	constructed := 0
	c.NewStarter = func() (sessionruntime.Starter, error) {
		h, e := actualKernel.k.reopen(context.Background(), actualKernel.j, actualKernel.l)
		if e != nil || h.Disposition != "PENDING" {
			t.Fatal("ASSERT_STDERR_REAL_FACTORY_PENDING", e)
		}
		for _, v := range h.Charged.vector() {
			if v == 0 {
				t.Fatal("ASSERT_STDERR_REAL_FACTORY_ALL14")
			}
		}
		constructed++
		backend, e := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096})
		return sessionruntime.ManagedStarter{Manager: backend}, e
	}
	actual, e := actualOwner.ReservePartition(context.Background(), actualBinding, hashLabelV2("actual-managed-stderr"))
	if e != nil {
		t.Fatal(e)
	}
	if e = actual.Execute(context.Background(), c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error {
		t.Fatal("no ready provider")
		return nil
	}); e == nil || constructed != 1 {
		t.Fatal("ASSERT_STDERR_ACTUAL_PREFUNDED_NO_PROCESS", constructed, e)
	}
	if actual.Complete(context.Background()) == nil {
		t.Fatal("ASSERT_STDERR_FAILED_START_NOT_OBSERVED")
	}
	t.Log("ASSERT_DISCOVERY_FUNDED_CLOSED_RUNTIME PASS (scripted pipes, no native authority); ASSERT_STDERR_REAL_FACTORY_ALL14 PASS")
}
func TestPrivateDiscoveryV2FundingAndOneShot(t *testing.T) {
	o, x, b := discoverySyntheticOwnerV2(t)
	op, e := o.ReservePartition(context.Background(), b, hashLabelV2("discovery-attempt"))
	if e != nil || op.state == nil {
		t.Fatal("ASSERT_DISCOVERY_RESERVE_CONFIRMED", e)
	}
	h, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || h.Disposition != "PENDING" || h.Windows != 1 {
		t.Fatal("ASSERT_DISCOVERY_REAL_KERNEL_PENDING", e)
	}
	calls := 0
	construction := sessionruntime.PrivateDiscoveryConstruction{NewStarter: func() (sessionruntime.Starter, error) { calls++; return nil, errors.New("synthetic refusal") }}
	alias := op
	if op.Execute(context.Background(), construction, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }) == nil || alias.Execute(context.Background(), construction, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }) == nil || calls != 0 {
		t.Fatal("ASSERT_DISCOVERY_COPIES_ONE_STATE_NO_START_ON_MISMATCH")
	}
	if _, e = OpenPrivateDiscoveryOwnerV2(context.Background(), x.k.root, sealTestV2(t, &x.j), x.h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384}); e == nil {
		t.Fatal("ASSERT_DISCOVERY_STALE_HEAD_REFUSED")
	}
	if _, e = OpenPrivateDiscoveryOwnerV2(context.Background(), x.k.root, sealTestV2(t, &x.j), h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384}); e == nil {
		t.Fatal("ASSERT_DISCOVERY_SPENT_CHAIN_REFUSED")
	}
	closedOwner, _, closedBinding := discoverySyntheticOwnerV2(t)
	closed, e := closedOwner.ReservePartition(context.Background(), closedBinding, hashLabelV2("close-before-use"))
	if e != nil {
		t.Fatal(e)
	}
	if closed.Complete(context.Background()) == nil {
		t.Fatal("ASSERT_DISCOVERY_UNUSED_NOT_OBSERVED")
	}
	if closed.Execute(context.Background(), construction, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }) == nil || closed.state.used || calls != 0 {
		t.Fatal("ASSERT_DISCOVERY_TERMINAL_WINDOW_CANNOT_START")
	}
	t.Log("ASSERT_DISCOVERY_FUNDING_ONE_SHOT PASS")
}
func TestPrivateDiscoveryV2AllCurrenciesAndBounds(t *testing.T) {
	_, _, b := discoverySyntheticOwnerV2(t)
	c, e := privateDiscoveryChargeV2(b, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range c.vector() {
		if v == 0 {
			t.Fatalf("ASSERT_DISCOVERY_ALL_FOURTEEN currency=%d", i)
		}
	}
	for _, mutate := range []func(*PrivateDiscoveryBindingV2){func(b *PrivateDiscoveryBindingV2) { b.Deadline = time.Now().Add(-time.Second) }, func(b *PrivateDiscoveryBindingV2) { b.Deadline = time.Now().Add(16 * time.Second) }, func(b *PrivateDiscoveryBindingV2) { b.Limits.Work = math.MaxUint64 }, func(b *PrivateDiscoveryBindingV2) { b.MemberDigest = "forged" }} {
		bad := b
		mutate(&bad)
		if _, e := privateDiscoveryChargeV2(bad, time.Now()); e == nil {
			t.Fatal("ASSERT_DISCOVERY_BOUND_REFUSED_BEFORE_EFFECT")
		}
	}
	for dimension := 0; dimension < 14; dimension++ {
		o, x, b := discoverySyntheticCeilingV2(t, dimension)
		op, e := o.ReservePartition(context.Background(), b, hashLabelV2("underfunded"))
		if e == nil || op.state != nil {
			t.Fatalf("ASSERT_DISCOVERY_UNDERFUNDED_REFUSED dimension=%d", dimension)
		}
		sameHeadTestV2(t, x, x.h)
	}
	o, x, b := discoverySyntheticOwnerV2(t)
	b.MemberDigest = hashLabelV2("foreign partition")
	if _, e := o.ReservePartition(context.Background(), b, hashLabelV2("foreign")); e == nil {
		t.Fatal("ASSERT_DISCOVERY_WRONG_PARTITION_POLICY_REFUSED")
	}
	sameHeadTestV2(t, x, x.h)
	ordinary := harnessTestV2(t, nil)
	if _, e := OpenPrivateDiscoveryOwnerV2(context.Background(), ordinary.k.root, sealTestV2(t, &ordinary.j), ordinary.h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384}); e == nil {
		t.Fatal("ASSERT_DISCOVERY_NATIVE_JOB_NOT_BORROWED")
	}
	t.Log("ASSERT_DISCOVERY_ALL_CURRENCIES_BOUNDS PASS")
}
func TestPrivateDiscoveryV2UncertainReservationNoPermission(t *testing.T) {
	o, x, b := discoverySyntheticOwnerV2(t)
	o.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		result, e := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
		if e != nil {
			return result, e
		}
		return result, errors.New("synthetic committed error")
	}
	op, e := o.ReservePartition(context.Background(), b, hashLabelV2("uncertain"))
	if e == nil || op.state != nil || !o.uncertain {
		t.Fatal("ASSERT_DISCOVERY_CAS_UNCERTAIN_NO_PERMISSION")
	}
	h, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || h.Disposition != "PENDING" {
		t.Fatal("ASSERT_DISCOVERY_UNCERTAIN_CHARGE_RETAINED", e)
	}
	t.Log("ASSERT_DISCOVERY_CAS_UNCERTAIN PASS")
}
