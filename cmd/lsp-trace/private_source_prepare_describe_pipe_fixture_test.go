//go:build lsp_trace_controlled_describe_fixture && (darwin || linux)

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func prepareCLIHash(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }

// TEST ONLY funding. Uses the existing initializer/canonical parser; no new
// production bootstrap/issuer exists. All paths and bytes are literal fixtures.
func prepareCLIFixture(t *testing.T) (privateSourcePrepareDescribeConfig, *publication.Root, *inv.PrivateManagedInputOwnerV2, *sessionruntime.ControlledDescribePipeFixture) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	fixture, err := sessionruntime.NewControlledDescribePipeFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := fixture.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	_, decision, job, auth := readyCLIFixture(t)
	// Retain the exact canonical field order and domain/LF sealing contract.
	var identity struct{ Identity string }
	if json.Unmarshal(auth, &identity) != nil {
		t.Fatal("auth")
	}
	unsigned := bytes.Replace(auth, []byte(identity.Identity), []byte(""), 1)
	unsigned = bytes.ReplaceAll(unsigned, []byte(":8"), []byte(":100000000000000"))
	unsigned = bytes.ReplaceAll(unsigned, []byte(":1,"), []byte(":100000000000000,"))
	unsigned = bytes.ReplaceAll(unsigned, []byte(":1}"), []byte(":100000000000000}"))
	// Revision must stay one; only ceilings/tranches/window bounds change.
	unsigned = bytes.Replace(unsigned, []byte(`"revision":100000000000000,`), []byte(`"revision":1,`), 1)
	unsigned = bytes.Replace(unsigned, []byte(`"max_windows":100000000000000,`), []byte(`"max_windows":100,`), 1)
	unsigned = bytes.Replace(unsigned, []byte(`"max_window_ns":100000000000000,`), []byte(`"max_window_ns":15000000000,`), 1)
	aid := prepareCLIHash("inventory-successor/private/v2\x00lsp-trace.inventory-job-authorization.private.v2\x00" + string(unsigned))
	auth = bytes.Replace(unsigned, []byte(`"identity":""`), []byte(`"identity":"`+aid+`"`), 1)
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := inv.InitializePrivateReadyV2(context.Background(), root, job, auth, inv.PrivateReadyDecisionAssertionV2{Job: decision.Job, Authorization: aid, Reference: decision.Reference}, inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384})
	if err != nil {
		root.Close()
		t.Fatal("SETUP_INITIALIZE", err, string(auth))
	}
	owner, err := inv.OpenPrivateManagedInputOwnerV2(context.Background(), root, job, initialized.ConfirmedHead(), inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384})
	if err != nil {
		root.Close()
		t.Fatal(err)
	}
	now := time.Now()
	deadline := now.Add(12 * time.Second)
	persist := inv.PreparedDescribePersistRequestV1{Attempt: prepareCLIHash("persist"), Nonce: prepareCLIHash("nonce"), RootPath: dir, InitialHead: initialized.ConfirmedHead(), Decision: decision.Reference, Question: []byte("Explain Query"), Request: []byte("literal request"), ResponseSchema: []byte("literal response schema"), Deadline: deadline, Expires: now.Add(5 * time.Minute), RetentionUntil: now.Add(10 * time.Minute), RecordLimit: 16 << 20}
	c := privateSourcePrepareDescribeConfig{Schema: "private-source-prepare-describe.v1", AccountingRoot: dir, CanonicalJob: job, ExpectedHead: initialized.ConfirmedHead(), Verification: inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}, Workspace: dir, URI: fixture.URI(), Revision: "literal-revision", Privacy: "private-controlled", OccurrenceID: "prepare-query", Line: 1, Character: 5, CaptureAttempt: prepareCLIHash("capture"), PreparationAttempt: prepareCLIHash("preparation"), PacketAttempt: prepareCLIHash("packet"), RunID: "prepare-run", SeedNamespace: "prepare-seed", Seed: 7, Persist: persist, PacketLimit: 1 << 20}
	return c, root, owner, fixture
}

func TestPrivateSourcePrepareDescribeCleanupGate(t *testing.T) {
	for _, fault := range []string{"manager", "data", "root", "cancel", "deadline", "healthy"} {
		t.Run(fault, func(t *testing.T) {
			c, root, owner, fixture := prepareCLIFixture(t)
			if fault == "deadline" {
				c.Persist.Deadline = time.Now().Add(2 * time.Second)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			data, err := os.CreateTemp(t.TempDir(), "owned-config")
			if err != nil {
				t.Fatal(err)
			}
			reports := privatePrepareDescribeCloseReports{data: data}
			committed := false
			headBeforeCleanup := ""
			reports.manager = func() error {
				entries, e := os.ReadDir(filepath.Join(c.AccountingRoot, "inventory-successor/prepared-describe/private/v1/selected"))
				selected := 0
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".json") {
						selected++
					}
				}
				if e != nil || selected != 1 || fixture.Manager().Census().Workers != 0 {
					t.Fatal("SETUP_COMMIT_NOT_REACHED", e, len(entries))
				}
				committed = true
				headBeforeCleanup = owner.PreparationHeadV1()
				t.Log("ASSERT_PREPARE_ACTUAL_COMMIT: PASS")
				if fault == "manager" {
					return errors.New("reported shutdown error after actual Shutdown")
				}
				return nil
			}
			reports.dataReport = func() error {
				if _, e := data.Stat(); e == nil {
					t.Fatal("data not actually closed")
				}
				if fault == "data" {
					return errors.New("reported data close error after actual Close")
				}
				return nil
			}
			reports.root = func() error {
				if root.ValidatePrivate() == nil {
					t.Fatal("root not actually closed")
				}
				if fault == "root" {
					return errors.New("reported root close error after actual Close")
				}
				if fault == "cancel" {
					cancel()
				}
				if fault == "deadline" {
					time.Sleep(time.Until(c.Persist.Deadline) + time.Millisecond)
				}
				return nil
			}
			staged, err := privateSourcePrepareDescribeOwned(ctx, c, root, owner, fixture.Manager(), fixture.StartRequest(c.Persist.Deadline), reports)
			if !committed {
				t.Fatal("SETUP_COMMIT_NOT_REACHED")
			}
			if owner.PreparationHeadV1() != headBeforeCleanup {
				t.Fatal("ASSERT_PREPARE_NO_EXTRA_OBSERVE")
			}
			// Reopen only test-owned canonical accounting after real closes; no
			// production repair or Observe occurs on cleanup failure.
			checkRoot, openErr := publication.OpenRoot(c.AccountingRoot)
			if openErr != nil {
				t.Fatal(openErr)
			}
			_, verifyErr := inv.OpenPrivateManagedInputOwnerV2(context.Background(), checkRoot, c.CanonicalJob, headBeforeCleanup, c.Verification)
			checkCloseErr := checkRoot.Close()
			if verifyErr != nil || checkCloseErr != nil {
				t.Fatal("ASSERT_PREPARE_RETAINED_NATIVE_HEAD", verifyErr, checkCloseErr)
			}
			if fault != "healthy" {
				if err == nil || len(staged.Packet) != 0 || staged.Continuation != (inv.PreparedDescribeAnchorsV1{}) {
					t.Fatal("ASSERT_PREPARE_CLEANUP_ZERO", fault, err, len(staged.Packet))
				}
				var out, diag bytes.Buffer
				if finishPrivateSourcePrepareDescribe(ctx, c.Persist.Deadline, staged, err, &out, &diag) == 0 || out.Len() != 0 {
					t.Fatal("ASSERT_PREPARE_CLEANUP_NO_OUTPUT")
				}
				t.Log("ASSERT_PREPARE_CLEANUP_ZERO: PASS", fault)
			} else {
				if err != nil || len(staged.Packet) == 0 {
					t.Fatal("ASSERT_PREPARE_HEALTHY", err)
				}
				var packet inv.PrepareDescribePacketV1
				if json.Unmarshal(staged.Packet, &packet) != nil || string(packet.Definition) != "func Query() {}" || !bytes.Equal(packet.Question, c.Persist.Question) || !bytes.Equal(packet.Request, c.Persist.Request) || !bytes.Equal(packet.ResponseSchema, c.Persist.ResponseSchema) || len(packet.Provenance) == 0 || packet.Authority != 0 || packet.Accepted || packet.FeatureIdentity != "UNRESOLVED" || packet.Completeness != "UNKNOWN" || packet.AbsenceInference != "PROHIBITED" || packet.HOSTCost != "UNKNOWN_NOT_FREE" {
					t.Fatal("ASSERT_PREPARE_EXACT_PACKET", string(staged.Packet))
				}
				var out, diag bytes.Buffer
				if finishPrivateSourcePrepareDescribe(ctx, c.Persist.Deadline, staged, nil, &out, &diag) != 0 || !bytes.Equal(out.Bytes(), staged.Packet) {
					t.Fatal("ASSERT_PREPARE_EXACT_OUTPUT")
				}
				if strings.Contains(out.String(), "sourceConfirmed") || strings.Contains(out.String(), "handoff") {
					t.Fatal("opaque authority leaked")
				}
				prepareCLIFreshOpen(t, c, staged, packet.Provenance)
				t.Log("ASSERT_PREPARE_HEALTHY: PASS; ASSERT_PREPARE_EXACT_PACKET: PASS")
			}
		})
	}
}

type prepareCLIChildData struct {
	Job        []byte
	Head       string
	Anchors    inv.PreparedDescribeAnchorsV1
	Expires    time.Time
	Provenance []byte
	Decision   string
}

func prepareCLIFreshOpen(t *testing.T, c privateSourcePrepareDescribeConfig, p inv.StagedPrepareDescribeV1, provenance []byte) {
	t.Helper()
	_, decision, job, auth := readyCLIFixture(t)
	var old struct {
		Identity, Scope string
		StableID        string `json:"stable_id"`
	}
	if json.Unmarshal(job, &old) != nil {
		t.Fatal("job")
	}
	job = bytes.Replace(job, []byte(`"scope":"`+old.Scope+`"`), []byte(`"scope":"`+p.Continuation.CompletionScope+`"`), 1)
	job = bytes.Replace(job, []byte(`"stable_id":"`+old.StableID+`"`), []byte(`"stable_id":"`+prepareCLIHash("unit2-independent-completion")+`"`), 1)
	seal := func(schema string, raw []byte) ([]byte, string) {
		var v struct{ Identity string }
		if json.Unmarshal(raw, &v) != nil {
			t.Fatal("seal")
		}
		unsigned := bytes.Replace(raw, []byte(`"identity":"`+v.Identity+`"`), []byte(`"identity":""`), 1)
		id := prepareCLIHash("inventory-successor/private/v2\x00" + schema + "\x00" + string(unsigned))
		return bytes.Replace(unsigned, []byte(`"identity":""`), []byte(`"identity":"`+id+`"`), 1), id
	}
	job, jid := seal("lsp-trace.inventory-job-identity.private.v2", job)
	auth = bytes.Replace(auth, []byte(old.Identity), []byte(jid), 1)
	auth = bytes.ReplaceAll(auth, []byte(":8"), []byte(":100000000000000"))
	auth = bytes.ReplaceAll(auth, []byte(":1,"), []byte(":100000000000000,"))
	auth = bytes.ReplaceAll(auth, []byte(":1}"), []byte(":100000000000000}"))
	auth = bytes.Replace(auth, []byte(`"revision":100000000000000,`), []byte(`"revision":1,`), 1)
	auth = bytes.Replace(auth, []byte(`"max_windows":100000000000000,`), []byte(`"max_windows":100,`), 1)
	auth = bytes.Replace(auth, []byte(`"max_window_ns":100000000000000,`), []byte(`"max_window_ns":15000000000,`), 1)
	auth, aid := seal("lsp-trace.inventory-job-authorization.private.v2", auth)
	root, e := publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	initialized, e := inv.InitializePrivateReadyV2(context.Background(), root, job, auth, inv.PrivateReadyDecisionAssertionV2{Job: jid, Authorization: aid, Reference: decision.Reference}, c.Verification)
	closeErr := root.Close()
	if e != nil || closeErr != nil {
		t.Fatal("completion bootstrap", e, closeErr)
	}
	data := prepareCLIChildData{job, initialized.ConfirmedHead(), p.Continuation, c.Persist.Expires, provenance, decision.Reference}
	raw, e := json.Marshal(data)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(c.AccountingRoot, "prepare-child.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrivateSourcePrepareDescribeFreshChild$", "-test.v", "-test.timeout=15s")
	child.Env = append(os.Environ(), "UNIT2_PREPARE_OPEN_ROOT="+c.AccountingRoot)
	output, e := child.CombinedOutput()
	if e != nil || !bytes.Contains(output, []byte("ASSERT_PREPARE_CLI_FRESH_OPEN: PASS")) {
		t.Fatalf("fresh Open %v %s", e, output)
	}
	t.Logf("fresh child after real root close:\n%s", output)
}
func TestPrivateSourcePrepareDescribeFreshChild(t *testing.T) {
	dir := os.Getenv("UNIT2_PREPARE_OPEN_ROOT")
	if dir == "" {
		t.Skip("fresh-process helper")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "prepare-child.json"))
	if e != nil {
		t.Fatal(e)
	}
	var c prepareCLIChildData
	if json.Unmarshal(raw, &c) != nil {
		t.Fatal("child data")
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	}()
	owner, e := inv.OpenPrivateManagedInputOwnerV2(context.Background(), root, c.Job, c.Head, inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384})
	if e != nil {
		t.Fatal(e)
	}
	op, e := owner.ReservePreparedDescribeOpenV1(context.Background(), inv.PreparedDescribeOpenRequestV1{Attempt: prepareCLIHash("fresh-unit2-open"), Decision: c.Decision, Anchors: c.Anchors, Deadline: time.Now().Add(10 * time.Second), Expires: c.Expires, RecordLimit: 16 << 20})
	if e != nil {
		t.Fatal(e)
	}
	opened, e := inv.OpenPreparedDescribeV1(context.Background(), op)
	if e != nil || string(opened.Definition()) != "func Query() {}" || !bytes.Equal(opened.Provenance(), c.Provenance) {
		t.Fatal("ASSERT_PREPARE_CLI_FRESH_OPEN", e)
	}
	t.Log("ASSERT_PREPARE_CLI_FRESH_OPEN: PASS")
}
