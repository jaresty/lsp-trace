//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	ai "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	pilot "lsp-trace/internal/censuslineagepilot"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	product "lsp-trace/internal/privatehostproductv3"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/targetpacket"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// No native owner, Manager, provider, model, real HOST or fixture is made.
func TestApplicationInventoryV3OwnerOrdinaryRoute(t *testing.T) {
	route := applicationInventoryOwnerRoute(product.InventoryRequestV3{}, nil, "", nil)
	if !route.production {
		t.Fatal("ASSERT_OWNER_ORDINARY_CLI_PRODUCTION_ROUTE")
	}
	if route.preparer == nil {
		t.Fatal("ASSERT_OWNER_DEFAULT_OFF_PREPARER")
	}
	if _, e := route.preparer(context.Background(), ai.Lead{ID: "missing"}); e == nil {
		t.Fatal("ASSERT_OWNER_MISSING_DECLARATION_REFUSED")
	}
	embedding := applicationInventoryOwnerRoute(product.InventoryRequestV3{}, nil, "", product.InventoryNativeInputsV3{})
	if embedding.production {
		t.Fatal("ASSERT_OWNER_EMBEDDING_MAP_PRESERVED")
	}
	// Structural integration guard, not a semantic CALLS claim: the ordinary
	// foreground dispatch still reaches the controller and its actual ai.Run
	// request uses the production selector only AFTER NewInventoryTransportV3.
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	fset := token.NewFileSet()
	f, e := parser.ParseFile(fset, filepath.Join(dir, "private_host_application_inventory_v3.go"), nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	var ready, routePos, run token.Pos
	override := false
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			switch x := c.Fun.(type) {
			case *ast.Ident:
				if x.Name == "applicationInventoryOwnerRoute" {
					routePos = c.Pos()
				}
			case *ast.SelectorExpr:
				if x.Sel.Name == "NewInventoryTransportV3" {
					ready = c.Pos()
				}
				if x.Sel.Name == "Run" {
					run = c.Pos()
				}
			}
		}
		if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			l, lok := a.Lhs[0].(*ast.SelectorExpr)
			r, rok := a.Rhs[0].(*ast.SelectorExpr)
			if lok && rok && l.Sel.Name == "Preparer" && r.Sel.Name == "preparer" {
				override = true
			}
		}
		return true
	})
	if ready == 0 || routePos <= ready || run <= routePos || !override {
		t.Fatal("ASSERT_OWNER_FOREGROUND_READY_ROUTE_RUN")
	}
	b, e := os.ReadFile(filepath.Join(dir, "private_host_application.go"))
	if e != nil || !strings.Contains(string(b), "runApplicationInventoryControllerV3(args, rawApplication, stdin, stdout, stderr, inventoryInputs, startup)") {
		t.Fatal("ASSERT_OWNER_ORDINARY_DISPATCH_CONNECTED")
	}
	t.Log("ASSERT_OWNER_ORDINARY_CLI_PRODUCTION_ROUTE A-pass; ASSERT_OWNER_MISSING_DECLARATION_REFUSED A-pass; ASSERT_OWNER_FOREGROUND_READY_ROUTE_RUN A-pass; ASSERT_OWNER_EMBEDDING_MAP_PRESERVED A-pass")
}

func ownerCommandDataV3(t *testing.T) product.InventoryRequestV3 {
	t.Helper()
	_, old := inventoryTestConfig(t)
	raw, _ := json.Marshal(old)
	var q product.InventoryRequestV3
	if json.Unmarshal(raw, &q) != nil {
		t.Fatal("data")
	}
	h := pilot.Digest([]byte("not native funding"))
	base := filepath.Dir(q.Configuration.Store)
	d := product.InventoryNativeOwnerV3{Lead: ai.Lead{ID: "lead", Provenance: "offline metadata"}, AccountingRoot: filepath.Join(base, "never-open-native"), CanonicalJob: []byte("{}\n"), ExpectedHead: h, Verification: inv.PrivateManagedInputVerificationV2{Records: 32, Bytes: 65536, Work: 128}, Profile: runtimeprofile.Selector{TrustDomain: "offline", Workspace: q.Configuration.SourceRoot, Profile: "go", EnvironmentReference: "offline"}, LanguageID: "go", Provider: "gopls", ProcessPath: filepath.Join(base, "never-launch-provider"), URI: "file://" + filepath.Join(q.Configuration.SourceRoot, "a.go"), OccurrenceID: "occurrence", Revision: product.InventorySourceRevisionV3, Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Kinds: []pa.RelationKind{pa.TypedReferencesSymbol}, Deadline: time.Now().Add(8 * time.Second), RunID: "run", SeedNamespace: "original-schedule", Lineage: "lineage", Seed: 7, Attempts: [3]string{pilot.Digest([]byte("M")), pilot.Digest([]byte("P")), pilot.Digest([]byte("S"))}, Contract: cc.ContractInput{ContinuationID: h, ProfileID: h, WorkerPinID: h, ProgramCSeed: 7, PacketMaxResponseBytes: 65536, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}, SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 1024, MaxTotalBytes: 4096, MaxWork: 128, MaxObjects: 4}, Role: sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"}}
	q.Inventory.NativeOwners = []product.InventoryNativeOwnerV3{d}
	q.Inventory.Manifest.Leads = []ai.Lead{d.Lead}
	ownerCommandSealV3(t, &q)
	return q
}
func ownerCommandSealV3(t *testing.T, q *product.InventoryRequestV3) {
	t.Helper()
	q.Inventory.Targets = nil
	for _, d := range q.Inventory.NativeOwners {
		v, e := d.DescriptorInput()
		if e != nil {
			t.Fatal(e)
		}
		q.Inventory.Targets = append(q.Inventory.Targets, product.InventoryNativeTargetV3(v))
	}
	policy, _ := json.Marshal(q.Inventory)
	manifest, _ := json.Marshal(q.Inventory.Manifest)
	q.Configuration.UnitPolicySHA256 = pilot.Digest(policy)
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.SelectionRuleSHA256 = pilot.Digest(manifest)
	q.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = product.InventoryRequestDigestV3(*q)
}

func TestApplicationInventoryV3OwnerLazyRefusal(t *testing.T) {
	q := ownerCommandDataV3(t)
	if e := product.ValidateInventoryRequestV3(q); e != nil {
		t.Fatal("fixture metadata", e)
	}
	reserved, constructed, closed := 0, 0, 0
	sentinel := errors.New("offline constructor refused; no opaque owner")
	stages := applicationInventoryOwnerStagesV3{reserve: func(context.Context, product.InventoryNativeOwnerV3) error { reserved++; return nil }, construct: func(ctx context.Context, d product.InventoryNativeOwnerV3, v product.InventoryNativeInputV3) (*applicationInventoryOwnedV3, error) {
		constructed++
		deadline, ok := ctx.Deadline()
		if !ok || !deadline.Equal(d.Deadline) || !v.Capture.Start.Deadline.Equal(d.Deadline) {
			t.Fatal("ASSERT_OWNER_EXACT_ABSOLUTE_DEADLINE")
		}
		return &applicationInventoryOwnedV3{close: func() error { closed++; return nil }}, sentinel
	}}
	p := applicationInventoryOwnerPreparerV3(q, stages)
	if reserved != 0 || constructed != 0 {
		t.Fatal("ASSERT_OWNER_NO_PRE_READY_CONSTRUCTION")
	}
	result, e := p(context.Background(), q.Inventory.Manifest.Leads[0])
	if !errors.Is(e, sentinel) || result.Status != "" || constructed != 1 || reserved != 1 || closed != 1 {
		t.Fatal("ASSERT_OWNER_ONCE_PAID_CONSTRUCTOR_CLEANUP", e, reserved, constructed, closed)
	}
	_, _ = p(context.Background(), q.Inventory.Manifest.Leads[0])
	if constructed != 1 || reserved != 1 {
		t.Fatal("ASSERT_OWNER_DUPLICATE_NO_RETRY")
	}
	for _, kind := range []string{"expired-after-wait", "too-long", "cancelled", "target", "no-grant", "missing"} {
		t.Run(kind, func(t *testing.T) {
			candidate := ownerCommandDataV3(t)
			calls := 0
			s := stages
			s.reserve = func(context.Context, product.InventoryNativeOwnerV3) error {
				calls++
				if kind == "no-grant" {
					return sentinel
				}
				return nil
			}
			s.construct = func(context.Context, product.InventoryNativeOwnerV3, product.InventoryNativeInputV3) (*applicationInventoryOwnedV3, error) {
				t.Fatal("ASSERT_OWNER_REFUSE_BEFORE_CONSTRUCTOR", kind)
				return nil, sentinel
			}
			ctx := context.Background()
			switch kind {
			case "expired-after-wait":
				candidate.Inventory.NativeOwners[0].Deadline = time.Now().Add(-time.Second)
				ownerCommandSealV3(t, &candidate)
			case "too-long":
				candidate.Inventory.NativeOwners[0].Deadline = time.Now().Add(16 * time.Second)
				ownerCommandSealV3(t, &candidate)
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "target":
				candidate.Inventory.Targets[0].CaptureConfigurationSHA256 = pilot.Digest([]byte("wrong"))
			case "missing":
				candidate.Inventory.NativeOwners = nil
			}
			p := applicationInventoryOwnerPreparerV3(candidate, s)
			r, e := p(ctx, candidate.Inventory.Manifest.Leads[0])
			if e == nil || r.Status != "" {
				t.Fatal("ASSERT_OWNER_REFUSAL_NO_NATIVE_RESULT", kind)
			}
			want := 0
			if kind == "no-grant" {
				want = 1
			}
			if calls != want {
				t.Fatal("ASSERT_OWNER_REFUSAL_BEFORE_EFFECTS", kind, calls)
			}
		})
	}
	t.Log("ASSERT_OWNER_NO_PRE_READY_CONSTRUCTION A-pass; ASSERT_OWNER_ONCE_PAID_CONSTRUCTOR_CLEANUP A-pass; ASSERT_OWNER_DUPLICATE_NO_RETRY A-pass; ASSERT_OWNER_EXACT_ABSOLUTE_DEADLINE A-pass; ASSERT_OWNER_REFUSAL_BEFORE_EFFECTS A-pass")
}

func TestApplicationInventoryV3OwnerCleanupBarrier(t *testing.T) {
	// A non-native status is enough to test the result-suppression barrier. No
	// zero opaque selected value is ever asserted to be available/native evidence.
	for _, kind := range []string{"clean", "shutdown", "close", "cancelled", "expired", "prior"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deadline := time.Now().Add(time.Second)
			order := ""
			sentinel := errors.New("controlled cleanup failure")
			var prior error
			owned := &applicationInventoryOwnedV3{shutdown: func(got context.Context) error {
				order += "shutdown/"
				if got != ctx {
					t.Fatal("ASSERT_OWNER_NO_CLEANUP_CONTEXT_RENEWAL")
				}
				if kind == "cancelled" {
					cancel()
				}
				if kind == "shutdown" {
					return sentinel
				}
				return nil
			}, close: func() error {
				order += "close"
				if kind == "close" {
					return sentinel
				}
				return nil
			}}
			if kind == "expired" {
				deadline = time.Now().Add(-time.Second)
			}
			if kind == "prior" {
				prior = sentinel
			}
			r, e := finishApplicationInventoryOwnerV3(ctx, deadline, ai.NativeResult{Status: "UNAVAILABLE"}, prior, owned)
			if order != "shutdown/close" {
				t.Fatal("ASSERT_OWNER_CLEANUP_ORDER")
			}
			if kind == "clean" {
				if e != nil || r.Status != "UNAVAILABLE" {
					t.Fatal("ASSERT_OWNER_CLEANUP_CLEAN_CONTROL")
				}
			} else if e == nil || r.Status != "" {
				t.Fatal("ASSERT_OWNER_CLEANUP_FAILURE_SUPPRESSES_RESULT", kind)
			}
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_OWNER_CLEANUP_ORDER A-pass; ASSERT_OWNER_CLEANUP_FAILURE_SUPPRESSES_RESULT A-pass; ASSERT_OWNER_CLEANUP_CLEAN_CONTROL A-pass")
	}
}

func TestApplicationInventoryV3OwnerExpiredForegroundConservation(t *testing.T) {
	q := ownerCommandDataV3(t)
	q.Inventory.NativeOwners[0].Deadline = time.Now().Add(-time.Second)
	ownerCommandSealV3(t, &q)
	path := filepath.Join(filepath.Dir(q.Configuration.Store), "owner-request.json")
	inventoryTestWrite(t, path, q)
	n, raw, errout := inventoryTestEntry([]string{"setup", path}, nil)
	var setup struct{ Schema, RunID string }
	if n != 0 || json.Unmarshal(raw, &setup) != nil || !validApplicationDigest(setup.RunID) {
		t.Fatal("ASSERT_OWNER_EXPIRED_METADATA_SETUP_NOT_EXECUTION", n, string(raw), errout)
	}
	for i, role := range []string{"interpreter", "reviewer"} {
		n, _, errout = inventoryTestEntry([]string{"register", path, setup.RunID, role, "synthetic-" + role, q.Configuration.RolePromptSHA256[i]}, nil)
		if n != 0 {
			t.Fatal(errout)
		}
	}
	// This inspected helper scripts GroupDecision/Reviewer only. Its old-shaped
	// value supplies expected counts, never rewrites or decodes the new request.
	s := inventoryTestSession{path: path, runID: setup.RunID, q: inventoryTestRequest{Configuration: q.Configuration, Inventory: inventoryTestOptions{Manifest: q.Inventory.Manifest}}}
	terminal := inventoryTestRun(t, s, nil)
	if terminal.DispositionCounts["PREPARATION_FAILED"] != 1 || terminal.SourceLeads != 1 || terminal.Dispositions != 1 || terminal.Accepted || terminal.Authority != 0 {
		t.Fatal("ASSERT_OWNER_EXPIRED_FOREGROUND_CONSERVES_CENSUS")
	}
	var pub product.InventoryPublicationV3
	if product.ReadApplicationRecord(q.Configuration.Store, terminal.InventoryHead, &pub) != nil || product.VerifyInventoryPublicationV3(pub, q) != nil {
		t.Fatal("ASSERT_OWNER_NEW_DECLARATION_EXACT_Q_PUBLICATION")
	}
	if _, e := os.Lstat(q.Inventory.NativeOwners[0].AccountingRoot); !os.IsNotExist(e) {
		t.Fatal("ASSERT_OWNER_EXPIRED_NO_NATIVE_ROOT_CREATED")
	}
	n, _, errout = inventoryTestEntry([]string{"finalize", path, setup.RunID}, inventoryTestReturns(s, terminal))
	if n != 0 {
		t.Fatal("ASSERT_OWNER_NEW_DECLARATION_CHARGED_FINALIZATION", errout)
	}
	t.Log("ASSERT_OWNER_EXPIRED_FOREGROUND_CONSERVES_CENSUS A-pass; ASSERT_OWNER_NEW_DECLARATION_EXACT_Q_PUBLICATION A-pass; ASSERT_OWNER_EXPIRED_NO_NATIVE_ROOT_CREATED A-pass; ASSERT_OWNER_NEW_DECLARATION_CHARGED_FINALIZATION A-pass")
}

func TestApplicationInventoryV3OwnerRootAdmission(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(base, "native"), filepath.Join(base, "source"), filepath.Join(base, "store")}
	for _, p := range paths {
		if err = os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err = admitApplicationInventoryOwnerPathsV3(context.Background(), paths...); err != nil {
		t.Fatal("ASSERT_OWNER_CANONICAL_ROOTS_CONTROL", err)
	}
	if e := admitApplicationInventoryOwnerPathsV3(context.Background(), paths[0], paths[0], paths[2]); e == nil {
		t.Fatal("ASSERT_OWNER_PHYSICAL_ROOT_ALIAS_REFUSED")
	}
	alias := filepath.Join(base, "alias")
	if err = os.Symlink(paths[0], alias); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{alias, filepath.Join(alias, "nested"), base + "/x/../native", "relative"} {
		if err = admitApplicationInventoryOwnerPathsV3(context.Background(), p, paths[1], paths[2]); err == nil {
			t.Fatal("ASSERT_OWNER_NOFOLLOW_ROOT_ALIAS", p)
		}
	}
	t.Log("ASSERT_OWNER_CANONICAL_ROOTS_CONTROL A-pass; ASSERT_OWNER_NOFOLLOW_ROOT_ALIAS A-pass; ASSERT_OWNER_PHYSICAL_ROOT_ALIAS_REFUSED A-pass")
}
