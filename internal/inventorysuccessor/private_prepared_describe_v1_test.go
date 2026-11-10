package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

// Children receive only a test-owned temporary directory and test-binary flags.
// There is no Manager, provider, network, real job, or inherited Go handle.
func TestPreparedDescribeV1Subprocess(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, child := range []string{"Producer", "Opener"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPreparedDescribeV1"+child+"$", "-test.v", "-test.timeout=180s")
		cmd.Env = []string{"PREPARED_DESCRIBE_TEST_ROOT=" + dir}
		raw, e := cmd.CombinedOutput()
		t.Logf("%s: %s", child, raw)
		if e != nil {
			t.Fatalf("ASSERT_PREPARED_DESCRIBE_CROSS_PROCESS: child %s: %v", child, e)
		}
	}
	t.Log("ASSERT_PREPARED_DESCRIBE_CROSS_PROCESS: PASS")
}

type preparedDescribeTestFixtureV1 struct {
	owner   *PrivateManagedInputOwnerV2
	initial string
	q       PreparedDescribePersistRequestV1
	record  preparedDescribeRecordV1
}

func preparedDescribeFixtureOwnerV1(t *testing.T, dir string) (*PrivateManagedInputOwnerV2, string) {
	t.Helper()
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	})
	j, a, d := fixtureV2(t)
	a.Ceiling = countersFromV2([14]uint64{1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14})
	a.MaxTranche = a.Ceiling
	a.MaxWindows = 100
	a.Expires = time.Now().Add(time.Hour).UnixNano()
	a.MaxWindowNS = uint64(15 * time.Second)
	sealTestV2(t, &a)
	d.Authorization = a.Identity
	k := &kernelV2{root: root}
	l := verificationLimitsV2{4096, 64 << 20, 16384}
	h, e := k.initialize(context.Background(), j, a, d, time.Now().UnixNano(), l)
	if e != nil {
		t.Fatal(e)
	}
	return &PrivateManagedInputOwnerV2{k: k, job: j, head: h.Identity, limits: l}, h.Identity
}
func preparedDescribeFixtureV1(t *testing.T, dir string) preparedDescribeTestFixtureV1 {
	t.Helper()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	o, initial := preparedDescribeFixtureOwnerV1(t, dir)
	ctx := context.Background()
	native := func(label string) (resourceHeadV2, resourceHeadV2) {
		attempt := hashLabelV2(label)
		now := time.Now()
		h, e := o.k.reserve(ctx, o.job, o.head, attempt, countersV2{Calls: 1}, now.Add(10*time.Second).UnixNano(), now.UnixNano(), o.limits)
		if e != nil {
			t.Fatal(e)
		}
		done, e := o.k.observe(ctx, o.job, h.Identity, attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
		if e != nil {
			t.Fatal(e)
		}
		o.head = done.Identity
		return h, done
	}
	_, capture := native("controlled-closed-capture")
	prep, ready := native("controlled-closed-preparation")
	now := time.Now()
	q := PreparedDescribePersistRequestV1{Attempt: hashLabelV2("persistence"), Nonce: hashLabelV2("workflow-nonce"), RootPath: dir, InitialHead: initial, Decision: hashLabelV2("decision"), Question: []byte("Explain the selected definition"), Request: []byte("controlled request bytes"), ResponseSchema: []byte("controlled response schema"), Deadline: now.Add(10 * time.Second), Expires: now.Add(5 * time.Minute), RetentionUntil: now.Add(10 * time.Minute), RecordLimit: 16 << 20}
	// Deliberately controlled in-package opaque producer snapshot: NOT actual
	// source capture, not an imported public DTO, and not an old Resume handle.
	// Native accounting ancestry and all new persistence/open paths remain real.
	document := []byte("package fixture\nfunc selected() {}\n")
	r := preparedDescribeRecordV1{Schema: preparedDescribeSchemaV1, Mode: "R", Producer: o.job, RootPath: dir, InitialHead: initial, Decision: q.Decision, CaptureObserved: capture.Identity, PreparationReserved: prep.Identity, PreparationObserved: ready.Identity, PreparationAttempt: prep.Attempt, RunID: "controlled-run", SeedNamespace: "controlled-seed", Seed: 7, Definition: []byte("func selected() {}"), Document: document, Query: []byte("original query"), Input: []byte("original opaque input"), Case: []byte("controlled frozen case"), Result: []byte("controlled numeric result"), Policy: []byte("controlled R policy"), Predecessors: []preparedDescribeArtifactV1{{"RETAINED_COMPOSITE", "fixture/composite.json", "controlled-schema", []byte("exact retained predecessor")}}}
	p := &r.Provenance
	p.URI = "file:///controlled/source.go"
	p.SourceDigest = digest(document)
	p.SessionID = "controlled-session"
	p.Generation = 1
	p.DocumentVersion = "1"
	p.Encoding = "utf-16"
	p.SymbolID = "selected"
	p.Workspace = "controlled-workspace"
	p.Revision = "controlled-revision"
	p.Privacy = "controlled-private"
	p.Display.Start.Line = 1
	p.Display.End.Line = 1
	p.Display.End.Character = uint32(len(r.Definition))
	p.Selection.Start.Line = 1
	p.Selection.End.Line = 1
	p.Selection.Start.Character = 5
	p.Selection.End.Character = 13
	return preparedDescribeTestFixtureV1{o, initial, q, r}
}
func preparedDescribeFixtureReserveV1(t *testing.T, f preparedDescribeTestFixtureV1) PreparedDescribePersistOperationV1 {
	t.Helper()
	o := f.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	c, e := preparedDescribeChargeV1(f.q.RecordLimit, time.Now(), f.q.Deadline)
	if e != nil {
		t.Fatal(e)
	}
	k := preparedDescribeKernelV1(o)
	h, e := k.reserve(context.Background(), o.job, o.head, f.q.Attempt, c, f.q.Deadline.UnixNano(), time.Now().UnixNano(), o.limits)
	if e != nil {
		t.Fatal(e)
	}
	o.head = h.Identity
	op, e := preparedDescribeFreezeV1(context.Background(), o, f.q, f.record, h)
	if e != nil {
		t.Fatal(e)
	}
	return op
}
func preparedDescribeFixtureCompletionV1(t *testing.T, f preparedDescribeTestFixtureV1, a PreparedDescribeAnchorsV1) *PrivateManagedInputOwnerV2 {
	t.Helper()
	producer := f.owner
	j, auth, d := fixtureV2(t)
	j.Scope = a.CompletionScope
	j.StableID = hashLabelV2("completion-job/" + a.Workflow)
	sealTestV2(t, &j)
	auth.Job = j.Identity
	auth.Ceiling = countersFromV2([14]uint64{1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14})
	auth.MaxTranche = auth.Ceiling
	auth.MaxWindows = 100
	auth.Expires = f.q.Expires.UnixNano()
	auth.MaxWindowNS = uint64(15 * time.Second)
	sealTestV2(t, &auth)
	d.Job = j.Identity
	d.Authorization = auth.Identity
	h, e := producer.k.initialize(context.Background(), j, auth, d, time.Now().UnixNano(), producer.limits)
	if e != nil {
		t.Fatal(e)
	}
	// Open canonical bytes through the existing owner API; no unchecked job import.
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), producer.k.root, sealTestV2(t, &j), h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func preparedDescribeOpenRequestV1(f preparedDescribeTestFixtureV1, a PreparedDescribeAnchorsV1) PreparedDescribeOpenRequestV1 {
	return PreparedDescribeOpenRequestV1{Attempt: hashLabelV2("fresh-open"), Decision: f.q.Decision, Anchors: a, Deadline: time.Now().Add(10 * time.Second), Expires: f.q.Expires, RecordLimit: 16 << 20}
}
func preparedDescribeFixtureCommitV1(t *testing.T, f preparedDescribeTestFixtureV1) (PreparedDescribeV1, PreparedDescribePersistOperationV1) {
	t.Helper()
	op := preparedDescribeFixtureReserveV1(t, f)
	p, e := CommitPreparedDescribeV1(context.Background(), op)
	if e != nil || p.state == nil {
		t.Fatalf("ASSERT_PREPARED_DESCRIBE_PRODUCER_COMMIT: %v", e)
	}
	return p, op
}

type preparedDescribeTestConfigV1 struct {
	Job        []byte
	Head       string
	Anchors    PreparedDescribeAnchorsV1
	Expires    time.Time
	Provenance []byte
}

func TestPreparedDescribeV1Producer(t *testing.T) {
	dir := os.Getenv("PREPARED_DESCRIBE_TEST_ROOT")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	f := preparedDescribeFixtureV1(t, dir)
	p, op := preparedDescribeFixtureCommitV1(t, f)
	if !op.state.used || f.owner.k.root.ValidatePrivate() != nil {
		t.Fatal("ASSERT_PREPARED_BORROWED_ROOT_LIVE")
	}
	completion := preparedDescribeFixtureCompletionV1(t, f, p.Anchors())
	provenance, e := preparedDescribeEncodeV1(f.record.Provenance, 64<<10)
	if e != nil {
		t.Fatal(e)
	}
	c := preparedDescribeTestConfigV1{sealTestV2(t, &completion.job), completion.head, p.Anchors(), f.q.Expires, provenance}
	raw, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "test-config.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_PREPARED_DESCRIBE_PRODUCER_COMMIT: PASS")
}
func TestPreparedDescribeV1Opener(t *testing.T) {
	dir := os.Getenv("PREPARED_DESCRIBE_TEST_ROOT")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "test-config.json"))
	if e != nil {
		t.Fatal(e)
	}
	var c preparedDescribeTestConfigV1
	if e = json.Unmarshal(raw, &c); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, c.Job, c.Head, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	op, e := o.ReservePreparedDescribeOpenV1(context.Background(), PreparedDescribeOpenRequestV1{Attempt: hashLabelV2("fresh-open"), Decision: hashLabelV2("decision"), Anchors: c.Anchors, Deadline: time.Now().Add(10 * time.Second), Expires: c.Expires, RecordLimit: 16 << 20})
	if e != nil {
		t.Fatal(e)
	}
	p, e := OpenPreparedDescribeV1(context.Background(), op)
	if e != nil || string(p.Definition()) != "func selected() {}" || !bytes.Equal(p.Provenance(), c.Provenance) {
		t.Fatalf("ASSERT_PREPARED_DESCRIBE_EXACT_OPEN: %v %q", e, p.Definition())
	}
	// Returned data are detached; mutation cannot change the opaque verified value.
	p.Definition()[0] = 'X'
	p.Provenance()[0] = 'X'
	if string(p.Definition()) != "func selected() {}" || !bytes.Equal(p.Provenance(), c.Provenance) {
		t.Fatal("ASSERT_PREPARED_IMMUTABLE")
	}
	if again, e := OpenPreparedDescribeV1(context.Background(), op); e == nil || again.state != nil {
		t.Fatal("ASSERT_PREPARED_OPEN_ONCE")
	}
	t.Log("ASSERT_PREPARED_DESCRIBE_EXACT_OPEN: PASS")
}

func TestPreparedDescribeV1ClosedQuote(t *testing.T) {
	now := time.Unix(1700000000, 0)
	deadline := now.Add(time.Second)
	c, e := preparedDescribeChargeV1(16<<20, now, deadline)
	if e != nil {
		t.Fatal(e)
	}
	small, e := preparedDescribeChargeV1(1, now, deadline)
	if e != nil || small.LogicalBytes < 20*(64<<20) {
		t.Fatal("ASSERT_PREPARED_REJECTING_SNAPSHOT_PREPAID", small.LogicalBytes, e)
	}
	v := c.vector()
	for i, n := range v {
		switch i {
		case 5, 6, 10, 11, 12:
			if n != 0 {
				t.Fatalf("provider-free currency %d=%d", i, n)
			}
		default:
			if n == 0 {
				t.Fatalf("uncharged currency %d", i)
			}
		}
	}
	clamped, e := preparedDescribeChargeV1(^uint64(0), now, deadline)
	if e != nil || clamped != c {
		t.Fatal("ASSERT_PREPARED_PINNED_MIN_CAP")
	}
	for _, d := range []time.Time{now, now.Add(-time.Nanosecond), now.Add(16 * time.Second)} {
		if _, e := preparedDescribeChargeV1(16<<20, now, d); e == nil {
			t.Fatal("ASSERT_PREPARED_WINDOW_REFUSAL")
		}
	}
	if preparedDescribeWindowV1(now, deadline, now) || preparedDescribeWindowV1(deadline, deadline, deadline) {
		t.Fatal("ASSERT_PREPARED_EXPIRED_EQUALITY")
	}
	t.Logf("ASSERT_PREPARED_CLOSED_14: PASS %v (five provider/transport currencies exactly zero)", v)
}

func TestPreparedDescribeV1AdmissionAndConsumption(t *testing.T) {
	ctx := context.Background()
	if p, e := CommitPreparedDescribeV1(ctx, PreparedDescribePersistOperationV1{}); e == nil || p.state != nil {
		t.Fatal("zero persist")
	}
	if p, e := OpenPreparedDescribeV1(ctx, PreparedDescribeOpenOperationV1{}); e == nil || p.state != nil {
		t.Fatal("zero open")
	}
	f := preparedDescribeFixtureV1(t, t.TempDir())
	// Unissued/local/foreign provenance cannot enter the public producer API.
	for _, prep := range []PrivateSourceRPreparationOperationV1{{}, {state: &privateSourceRPreparationStateV1{used: true, owner: f.owner, local: &PrivateSourceLocalConfigV1{}}}, {state: &privateSourceRPreparationStateV1{used: true, owner: &PrivateManagedInputOwnerV2{}}}} {
		op, e := f.owner.ReservePreparedDescribeV1(ctx, prep, PreparedSourceRV1{state: &preparedSourceRStateV1{}}, f.q)
		if e == nil || op.state != nil {
			t.Fatal("ASSERT_PREPARED_WRONG_PREPARATION")
		}
	}
	op := preparedDescribeFixtureReserveV1(t, f)
	before, e := f.owner.k.reopen(ctx, f.owner.job, f.owner.limits)
	if e != nil {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	p, e := CommitPreparedDescribeV1(canceled, op)
	if e == nil || p.state != nil || !op.state.used {
		t.Fatal("ASSERT_PREPARED_CANCELED_CONSUMED")
	}
	p, e = CommitPreparedDescribeV1(ctx, op)
	if e == nil || p.state != nil {
		t.Fatal("ASSERT_PREPARED_COMMIT_ONCE")
	}
	after, e := f.owner.k.reopen(ctx, f.owner.job, f.owner.limits)
	if e != nil || after.Charged != before.Charged || after.Windows != before.Windows || after.Disposition != "PENDING" {
		t.Fatal("ASSERT_PREPARED_NO_REFUND", e)
	}
	if e = f.owner.k.root.ValidatePrivate(); e != nil {
		t.Fatal("ASSERT_PREPARED_BORROWED_ROOT_LIVE", e)
	}
	t.Log("ASSERT_PREPARED_CANCELED_CONSUMED: PASS; ASSERT_PREPARED_NO_REFUND: PASS; ASSERT_PREPARED_BORROWED_ROOT_LIVE: PASS")
}
func TestPreparedDescribeV1Underfund(t *testing.T) {
	// All fourteen are considered. Five have a required amount of zero because no
	// provider/transport is invoked; uint64 cannot represent less than zero.
	for i := 0; i < 14; i++ {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			f := preparedDescribeFixtureV1(t, t.TempDir())
			o := f.owner
			c, e := preparedDescribeChargeV1(f.q.RecordLimit, time.Now(), f.q.Deadline)
			if e != nil {
				t.Fatal(e)
			}
			if c.vector()[i] == 0 {
				t.Logf("currency %d: zero required, no underfund state", i)
				return
			}
			// Renewal cannot shrink grants. Build a distinct admitted fixture job with
			// the exact existing initial authorization ceiling short by this currency.
			root := o.k.root
			j, a, d := fixtureV2(t)
			j.StableID = hashLabelV2("short-job")
			sealTestV2(t, &j)
			a.Job = j.Identity
			a.Ceiling = countersFromV2([14]uint64{1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14})
			a.MaxTranche = a.Ceiling
			short := a.MaxTranche.vector()
			short[i] = c.vector()[i] - 1
			a.MaxTranche = countersFromV2(short)
			a.MaxWindows = 100
			a.MaxWindowNS = uint64(15 * time.Second)
			a.Expires = time.Now().Add(time.Hour).UnixNano()
			sealTestV2(t, &a)
			d.Job = j.Identity
			d.Authorization = a.Identity
			k := &kernelV2{root: root}
			h, e := k.initialize(context.Background(), j, a, d, time.Now().UnixNano(), o.limits)
			if e != nil {
				t.Fatal(e)
			}
			before := h.Identity
			h, e = k.reserve(context.Background(), j, before, f.q.Attempt, c, f.q.Deadline.UnixNano(), time.Now().UnixNano(), o.limits)
			if e == nil || h.Identity != "" {
				t.Fatal("ASSERT_PREPARED_UNDERFUNDED_NATIVE")
			}
			after, e := k.reopen(context.Background(), j, o.limits)
			if e != nil || after.Identity != before {
				t.Fatal("ASSERT_PREPARED_REFUSE_BEFORE_EFFECTS")
			}
		})
	}
}

func TestPreparedDescribeV1ReceiptGuards(t *testing.T) {
	raw := []byte("exact")
	s := "fixture/x.json"
	good := publication.BoundFileReceipt{FinalSelector: s, Digest: digest(raw), ByteLength: uint64(len(raw)), Mechanism: publication.BoundFileMechanism, NamespaceAtomic: true, VerificationStatus: "VERIFIED", CloseStatus: publication.CloseComplete, DirectorySyncStatus: publication.DirectorySyncComplete}
	if !preparedDescribeReceiptV1(&good, s, raw) {
		t.Fatal("good receipt")
	}
	for _, mutate := range []func(*publication.BoundFileReceipt){func(r *publication.BoundFileReceipt) { r.CloseStatus = publication.CloseFailed }, func(r *publication.BoundFileReceipt) { r.VerificationStatus = "COMMITTED_VERIFICATION_FAILED" }, func(r *publication.BoundFileReceipt) { r.DirectorySyncStatus = publication.DirectorySyncFailed }, func(r *publication.BoundFileReceipt) { r.Digest = hashLabelV2("foreign") }, func(r *publication.BoundFileReceipt) { r.NamespaceAtomic = false }} {
		bad := good
		mutate(&bad)
		if preparedDescribeReceiptV1(&bad, s, raw) {
			t.Fatal("ASSERT_PREPARED_BAD_RECEIPT")
		}
	}
	t.Log("ASSERT_PREPARED_BAD_RECEIPT: PASS")
}

func TestPreparedDescribeV1CommitFaults(t *testing.T) {
	for _, fault := range []string{"wrong-mode", "wrong-predecessor", "wrong-body", "canceled", "genesis-error", "genesis-close", "observe-error", "observe-committed-error"} {
		t.Run(fault, func(t *testing.T) {
			f := preparedDescribeFixtureV1(t, t.TempDir())
			op := preparedDescribeFixtureReserveV1(t, f)
			o := f.owner
			charged, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			switch fault {
			case "wrong-mode":
				op.state.record.Mode = "D"
			case "wrong-predecessor":
				op.state.record.PreparationReserved = hashLabelV2("foreign")
			case "wrong-body":
				op.state.record.Definition = []byte("different")
			}
			original := publication.CompareAndReplaceBoundFile
			o.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				genesis := strings.Contains(s, "/selected/")
				observe := s == currentSelectorV2(o.job) && bytes.Contains(b, []byte(`"event":"OBSERVE"`))
				if (fault == "genesis-error" && genesis) || (fault == "observe-error" && observe) {
					return nil, errors.New("controlled precommit refusal")
				}
				receipt, e := original(ctx, r, s, p, b, v)
				if receipt != nil && receipt.Committed {
					if fault == "genesis-close" && genesis {
						receipt.BoundFileReceipt.CloseStatus = publication.CloseFailed
					}
					if fault == "observe-committed-error" && observe {
						return receipt, errors.New("controlled postcommit uncertainty")
					}
				}
				return receipt, e
			}
			ctx := context.Background()
			if fault == "canceled" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			p, e := CommitPreparedDescribeV1(ctx, op)
			if e == nil || p.state != nil {
				t.Fatal("ASSERT_PREPARED_FAULT_ZERO", fault, e)
			}
			after, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil || after.Charged != charged.Charged || after.Windows != charged.Windows {
				t.Fatal("ASSERT_PREPARED_FAULT_RETAINS_CHARGE", e)
			}
			if e = o.k.root.ValidatePrivate(); e != nil {
				t.Fatal("ASSERT_PREPARED_BORROWED_ROOT_LIVE", e)
			}
			t.Log("ASSERT_PREPARED_FAULT_ZERO: PASS", fault)
		})
	}
}

func TestPreparedDescribeV1FreshOpenRejects(t *testing.T) {
	for _, fault := range []string{"body", "provenance", "root", "job", "genesis", "predecessor", "observe-missing", "decision", "expiry", "canceled", "unselected-observe"} {
		t.Run(fault, func(t *testing.T) {
			f := preparedDescribeFixtureV1(t, t.TempDir())
			p, _ := preparedDescribeFixtureCommitV1(t, f)
			a := p.Anchors()
			completion := preparedDescribeFixtureCompletionV1(t, f, a)
			q := preparedDescribeOpenRequestV1(f, a)
			switch fault {
			case "body", "provenance":
				path := filepath.Join(a.RootPath, preparedDescribeSelectorV1("manifests", a.Manifest))
				raw, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				needle, replacement := []byte("controlled-session"), []byte("different-session")
				if fault == "body" {
					needle = []byte("ZnVuYyBzZWxlY3RlZCgpIHt9")
					replacement = []byte("ZnVuYyBkaWZmZXJlbnQoKSB7fQ==")
				}
				changed := bytes.Replace(raw, needle, replacement, 1)
				if bytes.Equal(changed, raw) {
					t.Fatal("non-exercising tamper")
				}
				if e = os.WriteFile(path, changed, 0600); e != nil {
					t.Fatal(e)
				}
			case "root":
				q.Anchors.RootPath = t.TempDir()
			case "job":
				q.Anchors.ProducerJob = hashLabelV2("foreign-job")
			case "genesis":
				q.Anchors.Genesis = hashLabelV2("foreign-genesis")
			case "predecessor":
				if e := os.Remove(filepath.Join(a.RootPath, recordSelectorV2(headsNamespaceV2, f.record.PreparationReserved))); e != nil {
					t.Fatal(e)
				}
			case "observe-missing":
				q.Anchors.Observed = hashLabelV2("missing-observe")
			case "decision":
				q.Decision = hashLabelV2("wrong-decision")
			case "expiry":
				q.Expires = time.Now()
			case "unselected-observe":
				// A canonical, successful-looking but unselected OBSERVE record is not proof.
				h, e := f.owner.k.reopen(context.Background(), f.owner.job, f.owner.limits)
				if e != nil {
					t.Fatal(e)
				}
				h.At++
				h.Identity = ""
				sealTestV2(t, &h)
				if _, e = f.owner.k.put(context.Background(), headsNamespaceV2, &h); e != nil {
					t.Fatal(e)
				}
				q.Anchors.Observed = h.Identity
			}
			ctx := context.Background()
			if fault == "canceled" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			op, e := completion.ReservePreparedDescribeOpenV1(ctx, q)
			if e == nil {
				got, e := OpenPreparedDescribeV1(ctx, op)
				if e == nil || got.state != nil {
					t.Fatal("ASSERT_PREPARED_OPEN_REJECTS", fault)
				}
			} else if op.state != nil {
				t.Fatal("ASSERT_PREPARED_OPEN_RESERVE_ZERO")
			}
			t.Log("ASSERT_PREPARED_OPEN_REJECTS: PASS", fault)
		})
	}
}

func TestPreparedDescribeV1NoObserveNoEligibility(t *testing.T) {
	f := preparedDescribeFixtureV1(t, t.TempDir())
	op := preparedDescribeFixtureReserveV1(t, f)
	o := f.owner
	// Refuse before persistence Observe selection; staged manifest and selected
	// genesis physically exist, but no successful native Observe can authorize Open.
	o.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		if s == currentSelectorV2(o.job) && bytes.Contains(b, []byte(`"event":"OBSERVE"`)) {
			return nil, errors.New("controlled missing Observe")
		}
		return publication.CompareAndReplaceBoundFile(ctx, r, s, p, b, v)
	}
	p, e := CommitPreparedDescribeV1(context.Background(), op)
	if e == nil || p.state != nil {
		t.Fatal("missing Observe commit")
	}
	raw, e := preparedDescribeEncodeV1(op.state.record, op.state.record.RecordLimit)
	if e != nil {
		t.Fatal(e)
	}
	r := op.state.record
	g := preparedDescribeGenesisV1{preparedDescribeSchemaV1 + "/genesis", r.Workflow, digest(raw), r.PersistReserved, r.Created}
	graw, e := preparedDescribeEncodeV1(g, recordLimitV2)
	if e != nil {
		t.Fatal(e)
	}
	a := PreparedDescribeAnchorsV1{r.RootPath, r.Producer.Identity, r.InitialHead, r.Workflow, digest(raw), digest(graw), hashLabelV2("no-observe"), r.CompletionScope}
	if _, e := publication.ReadVerifiedBoundFile(o.k.root, preparedDescribeSelectorV1("selected", r.Workflow), recordLimitV2); e != nil {
		t.Fatal("genesis not selected", e)
	}
	// Remove the fixture fault seam before fresh canonical admission.
	o.k.cas = nil
	c := preparedDescribeFixtureCompletionV1(t, f, a)
	q := preparedDescribeOpenRequestV1(f, a)
	open, e := c.ReservePreparedDescribeOpenV1(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	got, e := OpenPreparedDescribeV1(context.Background(), open)
	if e == nil || got.state != nil {
		t.Fatal("ASSERT_PREPARED_GENESIS_NOT_ELIGIBLE")
	}
	t.Log("ASSERT_PREPARED_GENESIS_NOT_ELIGIBLE: PASS")
}

func TestPreparedDescribeV1ConcurrentConsume(t *testing.T) {
	f := preparedDescribeFixtureV1(t, t.TempDir())
	op := preparedDescribeFixtureReserveV1(t, f)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := CommitPreparedDescribeV1(context.Background(), op)
			mu.Lock()
			defer mu.Unlock()
			if e == nil && p.state != nil {
				success++
			} else if p.state != nil {
				t.Error("error returned handle")
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatal("ASSERT_PREPARED_ONE_COMMIT", success)
	}
	t.Log("ASSERT_PREPARED_ONE_COMMIT: PASS")
}

func TestPreparedDescribeV1OpenReadbackErrors(t *testing.T) {
	for _, kind := range []string{"genesis", "selected", "manifests"} {
		t.Run(kind, func(t *testing.T) {
			f := preparedDescribeFixtureV1(t, t.TempDir())
			p, _ := preparedDescribeFixtureCommitV1(t, f)
			a := p.Anchors()
			o := preparedDescribeFixtureCompletionV1(t, f, a)
			q := preparedDescribeOpenRequestV1(f, a)
			op, err := o.ReservePreparedDescribeOpenV1(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			before, err := o.k.reopen(context.Background(), o.job, o.limits)
			if err != nil {
				t.Fatal(err)
			}
			id := a.Genesis
			if kind == "selected" {
				id = a.Workflow
			}
			if kind == "manifests" {
				id = a.Manifest
			}
			if err := os.Remove(filepath.Join(a.RootPath, preparedDescribeSelectorV1(kind, id))); err != nil {
				t.Fatal(err)
			}
			// Establish an actual checked-reader error, not a malformed admission.
			raw, readErr := preparedDescribeReadV1(context.Background(), o.k, kind, id, q.RecordLimit)
			if raw != nil || !errors.Is(readErr, os.ErrNotExist) {
				t.Fatal("readback refusal not exercised", readErr)
			}
			got, err := OpenPreparedDescribeV1(context.Background(), op)
			if err == nil || got.state != nil {
				t.Fatal("ASSERT_PREPARED_OPEN_READER_ZERO", kind, err)
			}
			after, err := o.k.reopen(context.Background(), o.job, o.limits)
			if err != nil || after.Identity != before.Identity || after.Charged != before.Charged || after.Windows != before.Windows {
				t.Fatal("ASSERT_PREPARED_READER_RETAINS_CHARGE", kind, err)
			}
			// Successful descriptor-relative read proves the borrowed root handle,
			// not merely its pathname/metadata, still works after the failed Open.
			if _, err := publication.ReadVerifiedBoundFileChecked(o.k.root, currentSelectorV2(o.job), headLimitV2); err != nil {
				t.Fatal("ASSERT_PREPARED_READER_ROOT_LIVE", err)
			}
			t.Log("ASSERT_PREPARED_OPEN_READER_ZERO: PASS; ASSERT_PREPARED_READER_RETAINS_CHARGE: PASS; ASSERT_PREPARED_READER_ROOT_LIVE: PASS", kind)
		})
	}
}

// This guard is run only by the recorded temporary compiling reader-error
// counterprobe. The environment selects a test, never a production behavior.
// No production callback, close-status boolean, or cross-package fault hook is
// added. The probe establishes Unit 1 error propagation, not an OS close fault.
func TestPreparedDescribeV1CommitReaderErrorCounterprobe(t *testing.T) {
	if os.Getenv("LSP_TRACE_PREPARED_READER_COUNTERPROBE") != "compiled-reader-error" {
		t.Skip("requires the recorded transient compiled reader-error counterprobe")
	}
	f := preparedDescribeFixtureV1(t, t.TempDir())
	op := preparedDescribeFixtureReserveV1(t, f)
	o := f.owner
	before, err := o.k.reopen(context.Background(), o.job, o.limits)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := preparedDescribeEncodeV1(op.state.record, op.state.record.RecordLimit)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CommitPreparedDescribeV1(context.Background(), op)
	if err == nil || got.state != nil {
		t.Fatal("ASSERT_PREPARED_COMMIT_READER_ZERO", err)
	}
	// Exact staged bytes demonstrate that this was not an earlier serialization,
	// reserve or publish refusal: the fault sits at direct data readback.
	staged, err := publication.ReadVerifiedBoundFileChecked(o.k.root, preparedDescribeSelectorV1("manifests", digest(raw)), int64(op.state.record.RecordLimit))
	if err != nil || !bytes.Equal(staged, raw) {
		t.Fatal("readback boundary not reached", err)
	}
	_, err = os.Lstat(filepath.Join(o.k.root.Path(), preparedDescribeSelectorV1("selected", op.state.record.Workflow)))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ASSERT_PREPARED_READER_NO_GENESIS", err)
	}
	after, err := o.k.reopen(context.Background(), o.job, o.limits)
	if err != nil || after.Identity != before.Identity || after.Charged != before.Charged || after.Windows != before.Windows {
		t.Fatal("ASSERT_PREPARED_READER_RETAINS_CHARGE", err)
	}
	t.Log("ASSERT_PREPARED_COMMIT_READER_ZERO: PASS; ASSERT_PREPARED_READER_NO_GENESIS: PASS; ASSERT_PREPARED_READER_RETAINS_CHARGE: PASS; ASSERT_PREPARED_READER_ROOT_LIVE: PASS")
}
