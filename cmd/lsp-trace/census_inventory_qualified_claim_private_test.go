package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/censusacquisition"
	cc "lsp-trace/internal/censuscontinuation"
	continuation "lsp-trace/internal/censussourcecontinuation"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	product "lsp-trace/internal/privatehostproductv3"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/targetpacket"
)

func qualifiedClaimFixture(t *testing.T, expected string, legacy107 ...bool) (censusInventoryCursorStore, censusInventoryCursor, product.InventoryRequestV3, string) {
	t.Helper()
	env, q, h := qualifiedAdmissionFixture(t)
	cfg, e := qualifiedAdmissionDecode(t, h)
	if e != nil {
		t.Fatal(e)
	}
	// REPAIR107 opt-in variant: preserve the full qualified identity before
	// retaining a legacy header. Existing qualified fixture calls are unchanged.
	if len(legacy107) == 1 && legacy107[0] {
		_, qualifiedRaw, err := canonicalCensusInventoryHeader(cfg.Header)
		if err != nil {
			t.Fatal("fixture qualified identity", err)
		}
		if expected == "qualified" {
			expected = censusInventoryDigest(qualifiedRaw)
		}
		cfg.Header.Schema = "private-connected-inventory-header.v1"
		cfg.Header.PlanIdentity = cfg.Header.Plan.Identity
		cfg.Header.QualifiedAdmission = nil
	}
	store, objects, heads, closeRoots, e := openCensusInventoryCursor(env.Workspace, cfg.Header.Workflow)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { closeRoots() })
	c, e := store.initialize(context.Background(), cfg.Header)
	if e != nil {
		t.Fatal(e)
	}
	f := q.AnalysisPlan.Files[0]
	seed, e := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "census-000000", Path: f.Path, Line: 1, Column: 1}}}}, env.Workspace)
	if e != nil {
		t.Fatal(e)
	}
	target := censusacquisition.PreparedTarget{URI: f.URI, LanguageID: f.LanguageID, Name: "Offline", Kind: 12, CensusOrdinal: 0, CanonicalSeedV2: seed, SymbolIdentity: fmt.Sprintf("%s#0:0:12:Offline:0", f.Path)}
	// Real durable partition claim, followed only by synthetic terminal accounting.
	cfg.ExpectedHead = censusInventoryDigest([]byte("funding-head-data"))
	cfg.Attempt = censusInventoryDigest([]byte("funding-attempt-data"))
	cfg.CanonicalJob = []byte("no authority")
	cfg.Deadline = time.Now().Add(10 * time.Second)
	request := qualifiedClaimRequest(cfg, objects, heads)
	c, _, e = store.claimPartition(context.Background(), c, cfg, request)
	if e != nil {
		t.Fatal(e)
	}
	receipt := censusInventoryDigest([]byte("offline-terminal-receipt-data"))
	record, e := store.put(context.Background(), censusInventoryTargetRecord{Schema: "private-connected-inventory-target.v1", Header: c.Header, Receipt: receipt, SeedDigest: censusInventoryDigest(seed), Ordinal: 0, Target: target})
	if e != nil {
		t.Fatal(e)
	}
	next := c
	next.IndexedPrefix = 1
	next.Index = []censusInventoryIndexEntry{{Ordinal: 0, Receipt: receipt, Targets: []string{record}}}
	next.Partitions = append([]censusInventoryPartitionState(nil), c.Partitions...)
	next.Partitions[0].State = "INDEXED"
	next.Partitions[0].Receipt = receipt
	var modes [7]censusInventoryModeState
	for i, m := range censusInventoryModes {
		modes[i] = censusInventoryModeState{Mode: m, State: "AVAILABLE"}
	}
	next.Targets = []censusInventoryTargetState{{Record: record, Modes: modes}}
	next.IndexRecord, e = store.put(context.Background(), censusInventoryDenominatorIndex{Schema: "private-connected-inventory-index.v1", Header: c.Header, Entries: next.Index})
	if e != nil {
		t.Fatal(e)
	}
	c, e = store.replace(context.Background(), c, next)
	if e != nil {
		t.Fatal(e)
	}
	pin := censusInventoryDigest([]byte("offline-declaration-only"))
	now := time.Now()
	d := product.InventoryNativeOwnerV3{Lead: ai.Lead{ID: target.SymbolIdentity, Provenance: "offline"}, AccountingRoot: env.Workspace + "-native", CanonicalJob: []byte("{}\n"), ExpectedHead: pin, Verification: inv.PrivateManagedInputVerificationV2{Records: 32, Bytes: 65536, Work: 128}, Profile: runtimeprofile.Selector{TrustDomain: "offline", Workspace: env.Workspace, Profile: "go", EnvironmentReference: "offline"}, LanguageID: "go", Provider: "gopls", ProcessPath: "/offline/never-started", ProcessArgs: []string{}, URI: f.URI, OccurrenceID: "occurrence", Revision: censusInventorySourceRevision, Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Kinds: []pa.RelationKind{pa.TypedReferencesSymbol}, Deadline: now.Add(10 * time.Second), RunID: "run", SeedNamespace: "schedule", Lineage: "lineage", Seed: 7, Attempts: [3]string{censusInventoryDigest([]byte("M")), censusInventoryDigest([]byte("P")), censusInventoryDigest([]byte("S"))}, Contract: cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, ProgramCSeed: 7, PacketMaxResponseBytes: 65536, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}, SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 1024, MaxTotalBytes: 4096, MaxWork: 128, MaxObjects: 4}, Role: sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"}}
	if expected == "matching" {
		expected = c.Header
	}
	patch, _ := json.Marshal(map[string]string{"ExpectedSourceAuthorization": expected})
	if e = json.Unmarshal(patch, &d); e != nil {
		t.Fatal(e)
	}
	input, e := d.DescriptorInput()
	if e != nil {
		t.Fatal(e)
	}
	binding := censusInventoryBridgeBinding{Root: env.Workspace, Key: cfg.Header.Workflow, Header: c.Header, Index: c.IndexRecord, Target: record, Mode: 5}
	bound, _ := censusInventoryMarshal(binding, 8192)
	app := product.ApplicationConfig{Schema: "private-host-application-config.v3", Profile: product.InventoryProfileV3, Store: env.Workspace + "-store", SourceRoot: env.Workspace, RevisionAssertion: censusInventorySourceRevision, ProducerExecutableSHA256: pin, StorageIdentitySHA256: pin, RolePromptSHA256: [2]string{censusInventoryDigest([]byte(product.InventoryRolePromptV3("interpreter"))), censusInventoryDigest([]byte(product.InventoryRolePromptV3("reviewer")))}, Grant: product.ApplicationGrant{Revision: 1, ExpiresUnixNano: now.Add(time.Minute).UnixNano(), BootstrapAttempts: 8, BootstrapEach: product.ApplicationBounds{ReadHashBytes: 4 * product.ApplicationRecordLimit, WriteBytes: product.ApplicationRecordLimit, Records: 8, ParserBytes: 3 * product.ApplicationRecordLimit, Work: 32, ActiveNS: 1000000}, Ceiling: product.ApplicationBounds{LogicalBytes: 1 << 32, ReadHashBytes: 1 << 36, WriteBytes: 1 << 36, Records: 1 << 24, ParserBytes: 1 << 36, Work: 1 << 26, ActiveNS: uint64(240 * time.Second), Calls: 16, RequestBytes: 1 << 20, ResponseBytes: 64 << 10, WireBytes: 8 << 20, OutputBytes: 4 << 20}}}
	native := product.InventoryRequestV3{Schema: "private-host-application-inventory-request.v3", Configuration: app, Inventory: product.InventoryOptionsV3{SourceRevision: censusInventorySourceRevision, Manifest: ai.Manifest{Scope: cfg.Header.Scope, Provenance: censusInventoryBindingPrefix + string(bound), Leads: []ai.Lead{d.Lead}, UntouchedGaps: []string{"remaining-denominator:" + c.Header + ":" + c.IndexRecord}}, Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin}, MaxNativeAttempts: 1, NativeRetainedUpper: 1 << 20, HostWindowMS: 1000, Targets: []product.InventoryTargetV3{product.InventoryNativeTargetV3(input)}, NativeOwners: []product.InventoryNativeOwnerV3{d}}}
	sealQualifiedClaimQ(&native)
	if e = product.ValidateInventoryRequestV3(native); e != nil {
		t.Fatal("fixture Q admission", e)
	}
	raw, _ := json.Marshal(native)
	selector, e := store.put(context.Background(), json.RawMessage(raw))
	if e != nil {
		t.Fatal(e)
	}
	state := c.Targets[0].Modes[5]
	state.State = "CONFIG_FROZEN"
	state.QDigest = native.ApprovalSHA256
	state.QSelector = selector
	state.Attempt = censusInventoryDigest([]byte("fresh-native-attempt"))
	state.Predecessor = c.Selector
	c, e = store.transition(context.Background(), c, 0, 5, "AVAILABLE", state)
	if e != nil {
		t.Fatal(e)
	}
	run := censusInventoryDigest([]byte("fresh-run"))
	state.RunID = run
	c, e = store.transition(context.Background(), c, 0, 5, "CONFIG_FROZEN", state)
	if e != nil {
		t.Fatal(e)
	}
	return store, c, native, run
}
func qualifiedClaimRequest(c censusInventoryBridgeConfig, objects continuation.ImmutableStore, heads continuation.HeadStore) continuation.Request {
	return continuation.Request{Plan: c.Header.Plan, PartitionSize: 16, WorkflowKey: c.Header.Workflow, HeadKey: c.Header.Workflow, Objects: objects, Heads: heads}
}
func sealQualifiedClaimQ(q *product.InventoryRequestV3) {
	raw, _ := json.Marshal(q.Inventory.Manifest)
	q.Configuration.SelectionRuleSHA256 = censusInventoryDigest(raw)
	raw, _ = json.Marshal(q.Inventory)
	q.Configuration.UnitPolicySHA256 = censusInventoryDigest(raw)
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = product.InventoryRequestDigestV3(*q)
}

// Standing regression at the actual pre-journal claim, not a setup-only proxy.
// All records and funding declarations are fresh synthetic data; no controller
// lifecycle, native reservation/Execute, provider or HOST is invoked.
func TestQualifiedLegacyRetainedHeaderBindingV1(t *testing.T) {
	for _, tc := range []struct {
		name, expected string
		admitted       bool
	}{
		{"absent", "", true},
		{"matching", "matching", true},
		{"cross-family", "qualified", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, before, q, run := qualifiedClaimFixture(t, tc.expected, true)
			if e := product.ValidateInventoryRequestV3(q); e != nil {
				t.Fatal("fixture Q target pins", e)
			}
			raw, e := s.objects.Get(context.Background(), before.Header)
			var h censusInventoryHeader
			if e != nil || censusInventoryDecode(raw, &h, 4<<20) != nil || h.QualifiedAdmission != nil {
				t.Fatal("fixture retained legacy header")
			}
			// Setup uses this same helper; direct control complements the actual
			// durable claim below without invoking a HOST lifecycle.
			helper := censusInventoryQualifiedNativeBinding(h, before.Header, q)
			claim, err := claimCensusInventoryConnectedQ(q, "run", run)
			admitted := err == nil && claim != nil
			if claim != nil {
				claim.close()
			}
			after, e := s.load(context.Background())
			if e != nil {
				t.Fatal("fixture durable readback", e)
			}
			advanced := after.Selector != before.Selector
			t.Logf("BINDING107 case=%s expected=%s retained=%s targetpins_valid=true helper=%t admitted=%t advanced=%t state=%s", tc.name, q.Inventory.NativeOwners[0].ExpectedSourceAuthorization, before.Header, helper, admitted, advanced, after.Targets[0].Modes[5].State)
			if helper != tc.admitted || admitted != tc.admitted || advanced != tc.admitted || tc.admitted && after.Targets[0].Modes[5].State != "CLAIMED" || !tc.admitted && after.Targets[0].Modes[5].State != "CONFIG_FROZEN" {
				t.Fatal("ASSERT qualified legacy retained-header admission: FAIL", tc.name)
			}
			t.Log("ASSERT qualified legacy retained-header admission: PASS", tc.name)
		})
	}
}

func TestQualifiedRetainedHeaderEveryOwnerV1(t *testing.T) {
	t.Run("legacy-second-mismatch", func(t *testing.T) {
		s, c, q, _ := qualifiedClaimFixture(t, "matching", true)
		raw, e := s.objects.Get(context.Background(), c.Header)
		var h censusInventoryHeader
		if e != nil || censusInventoryDecode(raw, &h, 4<<20) != nil {
			t.Fatal("fixture header")
		}
		other := q.Inventory.NativeOwners[0]
		other.ExpectedSourceAuthorization = censusInventoryDigest([]byte("different retained scope"))
		q.Inventory.NativeOwners = append(q.Inventory.NativeOwners, other)
		if censusInventoryQualifiedNativeBinding(h, c.Header, q) {
			t.Fatal("ASSERT every owner retained-header binding: FAIL")
		}
		t.Log("ASSERT every owner retained-header binding: PASS")
	})
	t.Run("qualified-two-matching", func(t *testing.T) {
		s, c, q, _ := qualifiedClaimFixture(t, "matching")
		raw, e := s.objects.Get(context.Background(), c.Header)
		var h censusInventoryHeader
		if e != nil || censusInventoryDecode(raw, &h, 4<<20) != nil {
			t.Fatal("fixture header")
		}
		q.Inventory.NativeOwners = append(q.Inventory.NativeOwners, q.Inventory.NativeOwners[0])
		if censusInventoryQualifiedNativeBinding(h, c.Header, q) {
			t.Fatal("ASSERT qualified exactly one owner retained: FAIL")
		}
		t.Log("ASSERT qualified exactly one owner retained: PASS")
	})
}

func TestQualifiedControllerClaimSourceV1(t *testing.T) {
	for _, tc := range []struct{ name, expected string }{{"matching", "matching"}, {"missing", ""}, {"wrong", censusInventoryDigest([]byte("other header"))}} {
		t.Run(tc.name, func(t *testing.T) {
			expected := tc.expected
			s, c, q, run := qualifiedClaimFixture(t, expected)
			claim, e := claimCensusInventoryConnectedQ(q, "run", run)
			if expected == "matching" {
				if e != nil || claim == nil {
					t.Fatal("ASSERT qualified controller matching claim admitted: FAIL", e)
				}
				claim.close()
				t.Log("ASSERT qualified controller matching claim admitted: PASS")
			} else {
				if e == nil {
					if claim != nil {
						claim.close()
					}
					t.Fatal("ASSERT qualified controller missing or wrong source rejected: FAIL")
				}
				after, e := s.load(context.Background())
				if e != nil || after.Selector != c.Selector {
					t.Fatal("ASSERT qualified source refusal precedes durable native claim: FAIL")
				}
				t.Log("ASSERT qualified controller missing or wrong source rejected: PASS")
			}
		})
	}
	t.Run("ordinary-bypass", func(t *testing.T) {
		_, _, q, run := qualifiedClaimFixture(t, "matching")
		q.Inventory.Manifest.Provenance = "ordinary"
		sealQualifiedClaimQ(&q)
		if c, e := claimCensusInventoryConnectedQ(q, "run", run); e == nil {
			if c != nil {
				c.close()
			}
			t.Fatal("ASSERT qualified expected binding cannot bypass connected claim: FAIL")
		}
		t.Log("ASSERT qualified expected binding cannot bypass connected claim: PASS")
	})
}
