//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/describeworker"
)

func managedFixtureV1(t *testing.T) (PrivateSourceLocalConfigV1, PrivateSourceLocalManagedRequestV1, *int) {
	t.Helper()
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(cwd, ".managed-control-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	var c PrivateSourceLocalConfigV1
	contents := [6]string{"#!/bin/sh\nexit 0\n", "tiny controlled model", "tiny native", "#!/bin/sh\nexit 0\n", privateManagedSandboxProfileV1, "tiny grammar"}
	names := [6]string{"worker", "model", "libllama.dylib", "sandbox", "profile", "grammar"}
	var pins [6]describeworker.FilePin
	for i := range pins {
		p := filepath.Join(base, names[i])
		mode := os.FileMode(0600)
		if i == 0 || i == 3 {
			mode = 0700
		}
		if e = os.WriteFile(p, []byte(contents[i]), mode); e != nil {
			t.Fatal(e)
		}
		pins[i] = describeworker.FilePin{Path: p, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(contents[i])))}
		c.Lengths[i] = uint64(len(contents[i]))
	}
	c.Config = describeworker.Config{Worker: pins[0], Model: pins[1], Library: pins[2], SandboxExecutable: pins[3], SandboxProfile: pins[4], Grammar: pins[5], RuntimeIdentity: "controlled-only", AdapterIdentity: PrivateSourceLocalWorkerAdapterV1, ModelIdentity: "not-model", ResponseVersion: describeworker.ResponseVersionV2, Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1024, StderrBytes: 128, WorkBytes: 4096, TempBytes: 4096}}
	reads := new(int)
	c.io = &controlledLocalIOV1{workerFixture: true, observe: func(event string) error {
		if event == "open" {
			*reads++
		}
		return nil
	}}
	dep := []byte("finite native dependency")
	path := filepath.Join(base, "libggml.dylib")
	if e = os.WriteFile(path, dep, 0600); e != nil {
		t.Fatal(e)
	}
	q := PrivateSourceLocalManagedRequestV1{Config: c, Count: 1, MaxBytes: 65536, MaxUses: 4, Attempt: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("managed-test"))), Deadline: time.Now().Add(14 * time.Second)}
	q.Dependencies[0] = PrivateSourceLocalDependencyV1{Pin: describeworker.FilePin{Path: path, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(dep))}, Bytes: uint64(len(dep))}
	return c, q, reads
}

func TestSourceLocalManagedAdmissionFunding(t *testing.T) {
	for _, mode := range []string{"underfunded", "bad-layout", "bad-count", "overflow", "wrong-hash", "missing", "positive"} {
		t.Run(mode, func(t *testing.T) {
			_, q, opens := managedFixtureV1(t)
			owner, _ := ControlledLocalOwnerForTest(t)
			if mode == "underfunded" {
				owner, _ = controlledLocalOwnerCallsForTest(t, 1)
			}
			q.ExpectedHead = owner.PreparationHeadV1()
			switch mode {
			case "bad-layout":
				q.Dependencies[0].Pin.Path = filepath.Join(filepath.Dir(q.Config.Config.Library.Path), "unsupported", "lib.dylib")
			case "bad-count":
				q.Count = 17
			case "overflow":
				q.Config.Lengths[1] = ^uint64(0)
			case "wrong-hash":
				q.Dependencies[0].Pin.SHA256 = "sha256:" + string(make([]byte, 64))
			case "missing":
				if e := os.Remove(q.Dependencies[0].Pin.Path); e != nil {
					t.Fatal(e)
				}
			}
			g, e := owner.AdmitLocalArtifactsV1(context.Background(), q)
			d, w, debit := ControlledLocalHeadForTest(t, owner)
			if mode != "positive" {
				if e == nil || g.state != nil {
					t.Fatal("ASSERT_MANAGED_BAD_ADMISSION", mode, e)
				}
				if mode != "missing" && (*opens != 0 || d != "READY" || w != 0 || debit != [14]uint64{}) {
					t.Fatal("ASSERT_MANAGED_LIFECYCLE_BEFORE_IO", mode, *opens, d, w)
				}
				if mode == "missing" && (d != "PENDING" || w != 1) {
					t.Fatal("ASSERT_MANAGED_ADMISSION_FAILURE_PENDING", d, w)
				}
				t.Log("ASSERT_MANAGED_REFUSAL: PASS", mode)
				return
			}
			if e != nil || *opens != 7 || d != "PENDING" || w != 1 || debit == [14]uint64{} {
				t.Fatal("ASSERT_MANAGED_GENUINE_RESERVATION", e, *opens, d, w)
			}
			quote, e := managedArtifactQuoteV1(q, time.Now(), owner.limits)
			if e != nil {
				t.Fatal(e)
			}
			for i, v := range quote.vector() {
				if i != 8 && debit[i] != v {
					t.Fatal("ASSERT_MANAGED_SEALED_QUOTE", i, debit[i], v)
				}
			}
			if debit[8] < quote.ActiveNS || debit[8] > uint64(14*time.Second) {
				t.Fatal("ASSERT_MANAGED_ORIGINAL_LIFETIME_DEBIT", debit[8], quote.ActiveNS)
			}
			if quote.ProviderRequests != 0 || quote.ProviderMessages != 0 || quote.RequestBytes != 0 || quote.ResponseBytes != 0 || quote.WireBytes != 0 || quote.WriteBytes < q.Config.Lengths[1] || quote.OutputBytes < q.Config.Lengths[1] {
				t.Fatal("ASSERT_MANAGED_FOURTEEN_CURRENCY_QUOTE", quote.vector())
			}
			c := q.Config
			c.Managed = &g
			extra, ee := managedUseQuoteV1(c)
			wantCalls := uint64(8 * 7 * 2048)
			wantMeta := wantCalls * 4096
			wantDep := q.Dependencies[0].Bytes
			if ee != nil || extra.Calls != wantCalls || extra.Work != wantMeta+4*wantDep || extra.ReadHashBytes != wantMeta+wantDep || extra.OutputBytes != func() uint64 {
				n := uint64(16384 + 7*1024)
				for i := 0; i < g.state.fileCount; i++ {
					n += 4 * uint64(len(g.state.paths[i]))
				}
				return n
			}() {
				t.Fatal("ASSERT_MANAGED_EXACT_USE_QUOTE", extra.vector(), ee)
			}
			use, e := sourceLocalConfigChargeV1(c)
			if e != nil || use.Calls <= localStageNamespaceCallsV1 || use.ReadHashBytes < q.Config.Lengths[1] || use.Work < q.Config.Lengths[1] {
				t.Fatal("ASSERT_MANAGED_USE_IS_NOT_FREE", use.vector(), e)
			}
			if e = g.Retire(context.Background()); e != nil {
				t.Fatal(e)
			}
			d, w, _ = ControlledLocalHeadForTest(t, owner)
			if d != "OBSERVED" || w != 1 {
				t.Fatal("ASSERT_MANAGED_RETIRE_OBSERVE", d, w)
			}
			if g.matches(c) {
				t.Fatal("ASSERT_MANAGED_RETIRED_STALE")
			}
			t.Log("ASSERT_MANAGED_GENUINE_RESERVATION: PASS; retained/deleted under one separate original lifecycle reservation")
		})
	}
}

func TestSourceLocalManagedOpaqueRevocation(t *testing.T) {
	c, q, _ := managedFixtureV1(t)
	o, _ := ControlledLocalOwnerForTest(t)
	q.ExpectedHead = o.PreparationHeadV1()
	g, e := o.AdmitLocalArtifactsV1(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	c.Managed = &g
	if !g.matches(c) {
		t.Fatal("ASSERT_MANAGED_VALID_HANDLE")
	}
	var zero PrivateSourceLocalManagedV1
	if zero.matches(c) {
		t.Fatal("ASSERT_MANAGED_ZERO_REFUSED")
	}
	raw, e := json.Marshal(g)
	if e != nil {
		t.Fatal(e)
	}
	var forged PrivateSourceLocalManagedV1
	if json.Unmarshal(raw, &forged) != nil || forged.matches(c) {
		t.Fatal("ASSERT_MANAGED_JSON_NOT_AUTHORITY")
	}
	bad := c
	bad.Config.Model.SHA256 = bad.Config.Worker.SHA256
	if g.matches(bad) {
		t.Fatal("ASSERT_MANAGED_CALLER_HASH_NOT_AUTHORITY")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				g.matches(c)
			}
		}()
	}
	if e = g.Revoke(); e != nil {
		t.Fatal(e)
	}
	wg.Wait()
	if g.matches(c) {
		t.Fatal("ASSERT_MANAGED_REVOKED_REFUSED")
	}
	if e = g.Retire(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_MANAGED_OPAQUE_REVOCATION: PASS")
}

func TestSourceLocalManagedReplacement(t *testing.T) {
	c, q, _ := managedFixtureV1(t)
	o, _ := ControlledLocalOwnerForTest(t)
	q.ExpectedHead = o.PreparationHeadV1()
	old, e := o.AdmitLocalArtifactsV1(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	oldRoot := old.state.stage.localStageDir
	if e = old.Revoke(); e != nil {
		t.Fatal(e)
	}
	if e = old.Retire(context.Background()); e != nil {
		t.Fatal(e)
	}
	q.ExpectedHead = o.PreparationHeadV1()
	q.Attempt = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("replacement-generation")))
	q.Deadline = time.Now().Add(14 * time.Second)
	replacement, e := o.AdmitLocalArtifactsV1(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	c.Managed = &replacement
	if old.state == replacement.state || oldRoot == replacement.state.stage.localStageDir || old.matches(c) || !replacement.matches(c) {
		t.Fatal("ASSERT_MANAGED_NEW_IMMUTABLE_GENERATION")
	}
	if e = replacement.Retire(context.Background()); e != nil {
		t.Fatal(e)
	}
	d, w, _ := ControlledLocalHeadForTest(t, o)
	if d != "OBSERVED" || w != 2 {
		t.Fatal("ASSERT_MANAGED_REPLACEMENT_INDEPENDENT_DEBIT", d, w)
	}
	t.Log("ASSERT_MANAGED_NEW_IMMUTABLE_GENERATION: PASS; old handle remains retired; replacement separately debited")
}

func TestSourceLocalManagedRevalidation(t *testing.T) {
	for _, mode := range []string{"cancelled-lifetime", "file-mode", "directory-mode", "content-change", "missing-dependency", "undeclared-dependency"} {
		t.Run(mode, func(t *testing.T) {
			_, q, _ := managedFixtureV1(t)
			o, _ := ControlledLocalOwnerForTest(t)
			q.ExpectedHead = o.PreparationHeadV1()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			g, e := o.AdmitLocalArtifactsV1(ctx, q)
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { ControlledManagedCloseForTest(g) })
			s := g.state
			switch mode {
			case "cancelled-lifetime":
				cancel()
			case "file-mode":
				e = os.Chmod(s.paths[1], 0600)
			case "directory-mode":
				e = os.Chmod(s.stage.localStageDir, 0750)
			case "content-change":
				e = os.Chmod(s.paths[1], 0600)
				if e == nil {
					e = os.WriteFile(s.paths[1], make([]byte, q.Config.Lengths[1]), 0600)
				}
				if e == nil {
					e = os.Chmod(s.paths[1], 0400)
				}
			case "missing-dependency":
				e = os.Remove(s.paths[6])
			case "undeclared-dependency":
				e = os.WriteFile(filepath.Join(s.stage.localStageDir, "lib", "undeclared.dylib"), []byte("not-admitted"), 0400)
			}
			if e != nil {
				t.Fatal(e)
			}
			s.mu.Lock()
			e = s.checkLocked()
			s.mu.Unlock()
			if e == nil {
				t.Fatal("ASSERT_MANAGED_REVALIDATION_REFUSAL", mode)
			}
			if g.Retire(context.Background()) == nil {
				t.Fatal("ASSERT_MANAGED_UNCERTAIN_NO_OBSERVE", mode)
			}
			d, w, _ := ControlledLocalHeadForTest(t, o)
			if d != "PENDING" || w != 1 {
				t.Fatal("ASSERT_MANAGED_UNCERTAIN_NO_OBSERVE", mode, d, w)
			}
			t.Log("ASSERT_MANAGED_REVALIDATION_REFUSAL: PASS", mode)
		})
	}
}
