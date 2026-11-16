package inventorysuccessor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// Accounting only: the quotation envelope is never executed and no process is
// constructed. Every case owns a fresh synthetic root; no historical grant is read.
func qualifiedSourceFixtureV1(t *testing.T, expected string) (*PrivateManagedInputOwnerV2, PrivateMixedInputRequestV2, PrivateSelectedPreparationPlanV2) {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	j, a, d := fixtureV2(t)
	a.Ceiling = countersFromV2([14]uint64{1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14, 1e14})
	a.MaxTranche = a.Ceiling
	a.MaxWindows = 100
	a.MaxWindowNS = uint64(15 * time.Second)
	a.Expires = time.Now().Add(time.Minute).UnixNano()
	sealTestV2(t, &a)
	d.Authorization = a.Identity
	k := &kernelV2{root: root}
	limits := verificationLimitsV2{4096, 64 << 20, 16384}
	head, e := k.initialize(context.Background(), j, a, d, time.Now().UnixNano(), limits)
	if e != nil {
		t.Fatal(e)
	}
	owner := &PrivateManagedInputOwnerV2{k: k, job: j, head: head.Identity, limits: limits}
	supervisor, e := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{})
	if e != nil {
		t.Fatal(e)
	}
	manager, e := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64, MaxOperations: 4}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if e != nil {
		t.Fatal(e)
	}
	selected, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "offline", Workspace: dir, Profile: "go", EnvironmentReference: "offline"})
	if e != nil {
		t.Fatal(e)
	}
	q := PrivateMixedInputRequestV2{Root: root, Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go", Deadline: time.Now().Add(10 * time.Second), Process: managedprocess.Spec{Path: "/offline/never-started", Dir: dir}}}
	envelope, e := manager.PrivateProviderWindowEnvelope(q.Start, sessionruntime.PrivateProviderWindowPlan{References: true})
	if e != nil {
		t.Fatal(e)
	}
	// JSON keeps this same guard compiling before the additive field exists.
	var config PrivateSelectedConfigurationV2
	raw, _ := json.Marshal(map[string]string{"ExpectedSourceAuthorization": expected})
	if e = json.Unmarshal(raw, &config); e != nil {
		t.Fatal(e)
	}
	config.Attempts[0] = hashLabelV2("qualified-fresh-M")
	stage, e := privateMixedChargeV2(time.Now(), q.Start.Deadline, envelope.Bounds())
	if e != nil {
		t.Fatal(e)
	}
	stage.ActiveNS += uint64(time.Second)
	plan := PrivateSelectedPreparationPlanV2{&selectedPlanV2{owner: owner, request: q, config: config, envelope: envelope, stages: [3]countersV2{stage, {Calls: 1}, {Calls: 1}}, expectedHead: head.Identity}}
	return owner, q, plan
}

func TestQualifiedSourceAdmissionV1(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("quotation-only Darwin backend; no execution")
	}
	for _, tc := range []struct {
		name, expected string
		allow          bool
	}{{"matching", hashLabelV2("source-authorization"), true}, {"wrong-source", hashLabelV2("different-qualified-header"), false}, {"legacy-absent", "", true}} {
		t.Run(tc.name, func(t *testing.T) {
			o, q, p := qualifiedSourceFixtureV1(t, tc.expected)
			before := o.head
			_, e := o.ReserveSelectedMixedInputV2(context.Background(), p.state.config.Attempts[0], q, p)
			if tc.allow {
				if e != nil {
					t.Fatalf("ASSERT qualified matching funded reservation: FAIL %v", e)
				}
				t.Log("ASSERT qualified matching funded reservation: PASS")
			} else {
				if e == nil || o.head != before {
					t.Fatal("ASSERT qualified wrong admitted source rejected before kernel: FAIL")
				}
				t.Log("ASSERT qualified wrong admitted source rejected before kernel: PASS")
			}
			if _, e = o.ReserveSelectedMixedInputV2(context.Background(), p.state.config.Attempts[0], q, p); e == nil {
				t.Fatal("ASSERT qualified reservation one-shot: FAIL")
			}
			t.Log("ASSERT qualified reservation one-shot: PASS")
		})
	}
	t.Run("concurrent", func(t *testing.T) {
		o, q, p := qualifiedSourceFixtureV1(t, hashLabelV2("source-authorization"))
		var wg sync.WaitGroup
		var mu sync.Mutex
		won := 0
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, e := o.ReserveSelectedMixedInputV2(context.Background(), p.state.config.Attempts[0], q, p); e == nil {
					mu.Lock()
					won++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if won != 1 {
			t.Fatalf("ASSERT qualified concurrent reservation once: FAIL %d", won)
		}
		t.Log("ASSERT qualified concurrent reservation once: PASS")
	})
}
