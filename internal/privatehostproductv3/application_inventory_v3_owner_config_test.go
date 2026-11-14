//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	ai "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/targetpacket"
	"strings"
	"testing"
	"time"
)

// Pure DATA fixtures. CanonicalJob is deliberately not an issued native job.
// These tests never open an owner/root, initialize READY or create a Manager.
func ownerDataFixtureV3(t *testing.T) InventoryRequestV3 {
	t.Helper()
	h := hash([]byte("offline declaration, not funding"))
	now := time.Now()
	d := InventoryNativeOwnerV3{Lead: ai.Lead{ID: "lead", Provenance: "detached"}, AccountingRoot: "/offline/native", CanonicalJob: []byte("{}\n"), ExpectedHead: h, Verification: inv.PrivateManagedInputVerificationV2{Records: 32, Bytes: 65536, Work: 128}, Profile: runtimeprofile.Selector{TrustDomain: "offline", Workspace: "/offline/source", Profile: "go", EnvironmentReference: "offline"}, LanguageID: "go", Provider: "gopls", ProcessPath: "/offline/bin/gopls", ProcessArgs: []string{}, URI: "file:///offline/source/a.go", OccurrenceID: "occurrence", Revision: InventorySourceRevisionV3, Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Kinds: []pa.RelationKind{pa.TypedReferencesSymbol}, Deadline: now.Add(10 * time.Second), RunID: "run", SeedNamespace: "original-schedule", Lineage: "lineage", Seed: 7, Attempts: [3]string{hash([]byte("M")), hash([]byte("P")), hash([]byte("S"))}, Contract: cc.ContractInput{ContinuationID: h, ProfileID: h, WorkerPinID: h, ProgramCSeed: 7, PacketMaxResponseBytes: 65536, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}, SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 1024, MaxTotalBytes: 4096, MaxWork: 128, MaxObjects: 4}, Role: sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"}}
	v, e := d.DescriptorInput()
	if e != nil {
		t.Fatal("data fixture", e)
	}
	c := ApplicationConfig{Schema: "private-host-application-config.v3", Profile: InventoryProfileV3, Store: "/offline/store", SourceRoot: d.Profile.Workspace, RevisionAssertion: InventorySourceRevisionV3, ProducerExecutableSHA256: h, StorageIdentitySHA256: h, RolePromptSHA256: [2]string{hash([]byte(InventoryRolePromptV3("interpreter"))), hash([]byte(InventoryRolePromptV3("reviewer")))}, Grant: ApplicationGrant{Revision: 1, ExpiresUnixNano: now.Add(time.Minute).UnixNano(), BootstrapAttempts: 8, BootstrapEach: ApplicationBounds{ReadHashBytes: 4 * ApplicationRecordLimit, WriteBytes: ApplicationRecordLimit, Records: 8, ParserBytes: 3 * ApplicationRecordLimit, Work: 32, ActiveNS: 1000000}, Ceiling: ApplicationBounds{LogicalBytes: 1 << 32, ReadHashBytes: 1 << 36, WriteBytes: 1 << 36, Records: 1 << 24, ParserBytes: 1 << 36, Work: 1 << 26, ActiveNS: uint64(240 * time.Second), Calls: 16, RequestBytes: 1 << 20, ResponseBytes: 64 << 10, WireBytes: 8 << 20, OutputBytes: 4 << 20}}}
	q := InventoryRequestV3{Schema: "private-host-application-inventory-request.v3", Configuration: c, Inventory: InventoryOptionsV3{SourceRevision: InventorySourceRevisionV3, Manifest: ai.Manifest{Scope: "detached", Provenance: "offline", Leads: []ai.Lead{d.Lead}, UntouchedGaps: []string{}}, Pins: dw.HostPinsV2{WorkerSHA256: h, ModelSHA256: h, GrammarSHA256: h}, MaxNativeAttempts: 1, NativeRetainedUpper: 1 << 20, HostWindowMS: 1000, Targets: []InventoryTargetV3{InventoryNativeTargetV3(v)}, NativeOwners: []InventoryNativeOwnerV3{d}}}
	ownerSealV3(&q)
	return q
}
func ownerSealV3(q *InventoryRequestV3) {
	q.Configuration.SelectionRuleSHA256 = hash(mustJSON(q.Inventory.Manifest))
	q.Configuration.UnitPolicySHA256 = hash(mustJSON(q.Inventory))
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.ApprovalSHA256 = ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = InventoryRequestDigestV3(*q)
}
func TestApplicationInventoryV3OwnerDeclarationAdmission(t *testing.T) {
	q := ownerDataFixtureV3(t)
	if ValidateInventoryRequestV3(q) != nil {
		t.Fatal("ASSERT_OWNER_DATA_ADMISSION_NOT_AUTHORITY")
	}
	raw := mustJSON(q)
	if _, e := DecodeInventoryRequestV3(raw); e != nil {
		t.Fatal("ASSERT_OWNER_CANONICAL_REQUEST", e)
	}
	for _, s := range []string{"NativeOwners", "AccountingRoot", "Deadline"} {
		bad := bytes.Replace(raw, []byte(`"`+s+`"`), []byte(`"`+strings.ToLower(s)+`"`), 1)
		if _, e := DecodeInventoryRequestV3(bad); e == nil {
			t.Fatal("ASSERT_OWNER_ALIAS_REFUSED", s)
		}
	}
	if _, e := DecodeInventoryRequestV3(append(raw, '\n')); e == nil {
		t.Fatal("ASSERT_OWNER_NEW_GRAMMAR_CANONICAL_BYTES")
	}
	legacy := q
	legacy.Inventory = q.Inventory.Detached()
	legacy.Inventory.NativeOwners = nil
	legacy.Inventory.Manifest.Provenance = "NativeOwners"
	ownerSealV3(&legacy)
	if _, e := DecodeInventoryRequestV3(append([]byte(" "), mustJSON(legacy)...)); e != nil {
		t.Fatal("ASSERT_OWNER_LEGACY_VALUE_IS_NOT_NEW_FIELD", e)
	}
	escaped := bytes.Replace(raw, []byte(`"NativeOwners"`), []byte(`"Nativ\u0065Owners"`), 1)
	if _, e := DecodeInventoryRequestV3(escaped); e == nil {
		t.Fatal("ASSERT_OWNER_ESCAPED_NEW_FIELD_REFUSED")
	}
	original := q.Inventory.NativeOwners[0].Deadline
	v, _ := q.Inventory.NativeOwners[0].DescriptorInput()
	v.Capture.Start.Deadline = original.Add(time.Nanosecond)
	if InventoryNativeTargetV3(v) == q.Inventory.Targets[0] {
		t.Fatal("ASSERT_OWNER_ABSOLUTE_DEADLINE_FINGERPRINT")
	}
	for _, kind := range []string{"root-source", "root-store", "root-alias", "no-job", "bad-job", "no-head", "verification-overflow", "profile", "kind", "duplicate-kind", "duplicate-attempt", "role", "locator", "contract", "source-limits", "target", "nested-oversize", "duplicate-lead", "cross-lead-root", "case-cross-lead-root", "nfc-cross-lead-root"} {
		t.Run(kind, func(t *testing.T) {
			candidate := ownerDataFixtureV3(t)
			d := &candidate.Inventory.NativeOwners[0]
			switch kind {
			case "root-source":
				d.AccountingRoot = candidate.Configuration.SourceRoot
			case "root-store":
				d.AccountingRoot = candidate.Configuration.Store + "/child"
			case "root-alias":
				d.AccountingRoot = "/offline/x/../native"
			case "no-job":
				d.CanonicalJob = nil
			case "bad-job":
				d.CanonicalJob = []byte("not json")
			case "no-head":
				d.ExpectedHead = ""
			case "verification-overflow":
				d.Verification.Bytes = ^uint64(0)
			case "profile":
				d.Profile.Workspace = "/different"
			case "kind":
				d.Kinds = []pa.RelationKind{pa.TypedCalls}
			case "duplicate-kind":
				d.Kinds = append(d.Kinds, d.Kinds[0])
			case "duplicate-attempt":
				d.Attempts[1] = d.Attempts[0]
			case "role":
				d.Role.Role = "not-a-role"
			case "locator":
				d.URI = "file:///offline/source/../store/secret"
			case "contract":
				d.Contract.ContinuationID = "bad"
			case "source-limits":
				d.SourceLimits.MaxWork = 0
			case "target":
				candidate.Inventory.Targets[0].CaptureConfigurationSHA256 = hash([]byte("mismatch"))
			case "nested-oversize":
				d.ProcessArgs = []string{strings.Repeat("x", 65537)}
			case "duplicate-lead":
				candidate.Inventory.NativeOwners = append(candidate.Inventory.NativeOwners, *d)
			case "cross-lead-root", "case-cross-lead-root", "nfc-cross-lead-root":
				if kind == "nfc-cross-lead-root" {
					d.AccountingRoot = "/offline/caf\u00e9"
				}
				second := *d
				if kind == "case-cross-lead-root" {
					second.AccountingRoot = strings.ToUpper(d.AccountingRoot)
				}
				if kind == "nfc-cross-lead-root" {
					second.AccountingRoot = "/offline/cafe\u0301"
				}
				second.Lead.ID = "second"
				candidate.Inventory.Manifest.Leads = append(candidate.Inventory.Manifest.Leads, second.Lead)
				candidate.Inventory.NativeOwners = append(candidate.Inventory.NativeOwners, second)
				candidate.Inventory.MaxNativeAttempts = 2
				v, _ := second.DescriptorInput()
				candidate.Inventory.Targets = append(candidate.Inventory.Targets, InventoryNativeTargetV3(v))
			}
			ownerSealV3(&candidate)
			if ValidateInventoryRequestV3(candidate) == nil {
				t.Fatal("ASSERT_OWNER_DECLARATION_REFUSAL", kind)
			}
		})
	}
	changed := q
	changed.Inventory = q.Inventory.Detached()
	changed.Inventory.NativeOwners[0].ExpectedHead = hash([]byte("different current chain"))
	if ValidateInventoryRequestV3(changed) == nil {
		t.Fatal("ASSERT_OWNER_DECLARATIONS_DIGEST_BOUND")
	}
	if q.Inventory.NativeOwners[0].ExpectedHead == changed.Inventory.NativeOwners[0].ExpectedHead {
		t.Fatal("ASSERT_OWNER_DATA_DETACHED")
	}
	if !t.Failed() {
		t.Log("ASSERT_OWNER_DATA_ADMISSION_NOT_AUTHORITY A-pass; ASSERT_OWNER_ALIAS_REFUSED A-pass; ASSERT_OWNER_ABSOLUTE_DEADLINE_FINGERPRINT A-pass; ASSERT_OWNER_DECLARATION_REFUSAL A-pass; ASSERT_OWNER_DECLARATIONS_DIGEST_BOUND A-pass; ASSERT_OWNER_DATA_DETACHED A-pass")
	}
}
func TestApplicationInventoryV3OwnerQuoteAndDeadline(t *testing.T) {
	q := ownerDataFixtureV3(t)
	d := q.Inventory.NativeOwners[0]
	b, e := InventoryOwnerConstructionQuoteV3(d)
	if e != nil || b.LogicalBytes == 0 || b.ReadHashBytes < d.Verification.Bytes || b.Records < d.Verification.Records || b.Work < d.Verification.Work || !applicationNoSemantics(b) || b.ProviderRequests != 0 || b.ProviderMessages != 0 || b.WriteBytes != 0 {
		t.Fatal("ASSERT_OWNER_PROCESSING_QUOTE_NO_NATIVE_CREDIT", e)
	}
	d.Verification.Bytes = 64 << 20
	if _, e := InventoryOwnerConstructionQuoteV3(d); e == nil {
		t.Fatal("ASSERT_OWNER_QUOTE_FIXED_ENVELOPE")
	}
	deadline := q.Inventory.NativeOwners[0].Deadline
	if !InventoryNativeDeadlineValidV3(context.Background(), deadline) {
		t.Fatal("ASSERT_OWNER_ABSOLUTE_LIVE")
	}
	for _, d := range []time.Time{time.Time{}, time.Now().Add(-time.Second), time.Now().Add(16 * time.Second)} {
		if InventoryNativeDeadlineValidV3(context.Background(), d) {
			t.Fatal("ASSERT_OWNER_ABSOLUTE_EXPIRED_OR_TOO_LONG")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if InventoryNativeDeadlineValidV3(ctx, deadline) {
		t.Fatal("ASSERT_OWNER_CANCELLED")
	}
	ctx, cancel = context.WithDeadline(context.Background(), deadline.Add(-time.Second))
	defer cancel()
	if InventoryNativeDeadlineValidV3(ctx, deadline) {
		t.Fatal("ASSERT_OWNER_PARENT_NOT_RENEWED")
	}
	// No native accounting is opened by this call: nil journal must refuse first.
	var a *ApplicationJournal
	if a.ReserveInventoryOwnerConstructionV3(context.Background(), q, hash([]byte("run")), q.Inventory.NativeOwners[0]) == nil {
		t.Fatal("ASSERT_OWNER_NO_APPLICATION_GRANT_REFUSED")
	}
	t.Log("ASSERT_OWNER_PROCESSING_QUOTE_NO_NATIVE_CREDIT A-pass; ASSERT_OWNER_QUOTE_FIXED_ENVELOPE A-pass; ASSERT_OWNER_ABSOLUTE_EXPIRED_OR_TOO_LONG A-pass; ASSERT_OWNER_CANCELLED A-pass; ASSERT_OWNER_PARENT_NOT_RENEWED A-pass")
}

func TestApplicationInventoryV3OwnerCanonicalJobOpaque(t *testing.T) {
	q := ownerDataFixtureV3(t)
	v, e := q.Inventory.NativeOwners[0].DescriptorInput()
	if e != nil || v.Owner != nil || v.Capture.Manager != nil || v.Capture.Root != nil {
		t.Fatal("ASSERT_OWNER_DECLARATION_HAS_NO_AUTHORITY")
	}
	raw, _ := json.Marshal(v.Configuration)
	if bytes.Contains(raw, []byte("Owner")) {
		t.Fatal("ASSERT_OWNER_NO_SERIALIZED_OWNER")
	}
	t.Log("ASSERT_OWNER_DECLARATION_HAS_NO_AUTHORITY A-pass")
}

func TestApplicationInventoryV3OwnerReservation(t *testing.T) {
	// Temporary synthetic APPLICATION ledger only. No native accounting root,
	// READY initialization, provider, Manager or opaque owner is touched.
	q := ownerDataFixtureV3(t)
	c, now := applicationFixture(t)
	q.Configuration.Store = c.Store
	ownerSealV3(&q)
	j := applicationCreate(t, q.Configuration, now)
	d := q.Inventory.NativeOwners[0]
	before := j.Head().Reserved
	quote, e := InventoryOwnerConstructionQuoteV3(d)
	if e != nil {
		t.Fatal(e)
	}
	runID := hash([]byte("synthetic construction claim, no native grant"))
	if e = j.ReserveInventoryOwnerConstructionV3(context.Background(), q, runID, d); e != nil {
		t.Fatal("ASSERT_OWNER_CONSTRUCTION_PREPAID", e)
	}
	after := j.Head().Reserved
	if after.LogicalBytes != before.LogicalBytes+quote.LogicalBytes || after.ActiveNS != before.ActiveNS+quote.ActiveNS || after.Calls != before.Calls || after.RequestBytes != before.RequestBytes || after.ProviderRequests != before.ProviderRequests {
		t.Fatal("ASSERT_OWNER_CONSTRUCTION_EXACT_PROCESSING_DEBIT")
	}
	if e = j.ReserveInventoryOwnerConstructionV3(context.Background(), q, runID, d); e == nil {
		t.Fatal("ASSERT_OWNER_CONSTRUCTION_CLAIM_NO_RETRY")
	}
	if j.Head().Reserved.LogicalBytes != after.LogicalBytes {
		t.Fatal("ASSERT_OWNER_FAILED_CONSTRUCTION_NO_REFUND_OR_REGRANT")
	}
	// A separately configured too-small application ceiling refuses before any
	// native constructor. Existing API-attempt overhead remains charged.
	q2 := ownerDataFixtureV3(t)
	c2, now2 := applicationFixture(t)
	q2.Configuration.Store = c2.Store
	q2.Configuration.Grant.Ceiling.LogicalBytes = 1
	ownerSealV3(&q2)
	j2 := applicationCreate(t, q2.Configuration, now2)
	if e = j2.ReserveInventoryOwnerConstructionV3(context.Background(), q2, runID, q2.Inventory.NativeOwners[0]); e == nil {
		t.Fatal("ASSERT_OWNER_INSUFFICIENT_APPLICATION_CEILING")
	}
	t.Log("ASSERT_OWNER_CONSTRUCTION_PREPAID A-pass; ASSERT_OWNER_CONSTRUCTION_EXACT_PROCESSING_DEBIT A-pass; ASSERT_OWNER_CONSTRUCTION_CLAIM_NO_RETRY A-pass; ASSERT_OWNER_FAILED_CONSTRUCTION_NO_REFUND_OR_REGRANT A-pass; ASSERT_OWNER_INSUFFICIENT_APPLICATION_CEILING A-pass")
}

func TestApplicationInventoryV3OwnerSevenModeData(t *testing.T) {
	for _, mode := range []string{"C", "R", "D", "CR", "CD", "RD", "CRD"} {
		t.Run(mode, func(t *testing.T) {
			q := ownerDataFixtureV3(t)
			d := &q.Inventory.NativeOwners[0]
			d.Kinds = nil
			for _, k := range []struct{ letter, kind string }{{"C", "CALLS"}, {"R", "REFERENCES_SYMBOL"}, {"D", "RESOLVES_TO_DEFINITION"}} {
				if strings.Contains(mode, k.letter) {
					d.Kinds = append(d.Kinds, pa.RelationKind(k.kind))
				}
			}
			d.Role = sc.PrivateSelectedRoleV3{Kind: string(d.Kinds[0]), Role: "CALLER_OCCURRENCE"}
			if d.Role.Kind == "REFERENCES_SYMBOL" {
				d.Role.Role = "REFERENCED_SYMBOL"
			}
			if d.Role.Kind == "RESOLVES_TO_DEFINITION" {
				d.Role.Role = "DEFINITION_TARGET"
			}
			v, e := d.DescriptorInput()
			if e != nil || v.Owner != nil {
				t.Fatal("ASSERT_OWNER_SEVEN_MODE_DATA_ONLY", mode, e)
			}
			q.Inventory.Targets = []InventoryTargetV3{InventoryNativeTargetV3(v)}
			ownerSealV3(&q)
			if ValidateInventoryRequestV3(q) != nil {
				t.Fatal("ASSERT_OWNER_SEVEN_MODE_DATA_ONLY", mode)
			}
		})
	}
	t.Log("ASSERT_OWNER_SEVEN_MODE_DATA_ONLY A-pass")
}
