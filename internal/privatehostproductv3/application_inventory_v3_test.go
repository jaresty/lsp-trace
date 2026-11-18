//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	ai "lsp-trace/internal/applicationinventory"
	dr "lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Reuses only the inspected temporary-filesystem configuration helper, never a
// native fixture, original controller, campaign, provider, model or grant mint.
func inventoryProductFixtureV3(t *testing.T, modes ...cat.ReviewModeV3) (InventoryRequestV3, string, *ApplicationJournal, *InventoryTransportV3) {
	t.Helper()
	var mode cat.ReviewModeV3
	if len(modes) == 1 {
		mode = modes[0]
	}
	q, id, j, mailbox := inventoryProductSetupV3(t, mode)
	tr, e := NewInventoryTransportV3(j, q, id, mailbox)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(tr.Close)
	return q, id, j, tr
}

func inventoryProductSetupV3(t *testing.T, mode cat.ReviewModeV3) (InventoryRequestV3, string, *ApplicationJournal, string) {
	t.Helper()
	c, now := applicationFixture(t)
	c.Schema = "private-host-application-config.v3"
	c.Profile = InventoryProfileV3
	c.RevisionAssertion = InventorySourceRevisionV3
	c.SourceRoot = filepath.Join(filepath.Dir(c.Store), "source")
	if e := os.Mkdir(c.SourceRoot, 0700); e != nil {
		t.Fatal(e)
	}
	c.Grant.Ceiling.Work = 1 << 26
	d := hash([]byte("scripted physical adapter only"))
	q := InventoryRequestV3{Schema: "private-host-application-inventory-request.v3", Configuration: c, Inventory: InventoryOptionsV3{SourceRevision: InventorySourceRevisionV3, Manifest: ai.Manifest{Scope: "all seven configurations", Provenance: "synthetic detached metadata", Leads: []ai.Lead{}, UntouchedGaps: []string{"non-Go unvisited"}}, Pins: dw.HostPinsV2{WorkerSHA256: d, ModelSHA256: d, GrammarSHA256: d}, MaxNativeAttempts: 1, NativeRetainedUpper: 1 << 20, HostWindowMS: 1000}}
	q.Inventory.ReviewMode = mode
	q.Configuration.RolePromptSHA256 = [2]string{hash([]byte(InventoryRolePromptV3("interpreter"))), hash([]byte(InventoryRolePromptV3("reviewer")))}
	q.Configuration.SelectionRuleSHA256 = hash(mustJSON(q.Inventory.Manifest))
	q.Configuration.UnitPolicySHA256 = hash(mustJSON(q.Inventory))
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.ApprovalSHA256 = ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = InventoryRequestDigestV3(q)
	j, e := CreateApplication(q.Configuration, now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { j.Close() })
	id := hash([]byte("synthetic ticket; no native owner"))
	mailbox := InventoryMailboxPathV3(c.Store, id)
	if e = os.MkdirAll(filepath.Dir(mailbox), 0700); e != nil {
		t.Fatal(e)
	}
	if e = PrepareMailbox(mailbox); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"interpreter", "reviewer"} {
		id := "scripted-" + role
		if mode.AllowsSameActor() {
			id = "scripted-one-context"
		}
		if e = MarkInventoryReadyV3(mailbox, Ready{Protocol: InventoryProtocolV3, Role: role, LaunchID: id, PromptSHA256: hash([]byte(InventoryRolePromptV3(role)))}, mode); e != nil {
			t.Fatal(e)
		}
	}
	return q, id, j, mailbox
}
func inventoryProductDescribeV3() []byte {
	prompt := "SYNTHETIC TRANSPORT ONLY; NOT NATIVE SELECTED"
	r := dr.Record{RecordID: "scripted", Envelope: dr.Envelope{Protocol: dr.Protocol, MessageType: dr.MessageType, SourceRevision: InventorySourceRevisionV3, Prompt: prompt, InputBytes: len(prompt), InputSHA256: strings.TrimPrefix(hash([]byte(prompt)), "sha256:")}}
	return mustJSON(r)
}
func inventoryProductClaimV3(t *testing.T, tr *InventoryTransportV3) (InventoryFrameV3, []byte, inventoryExchangeV3) {
	t.Helper()
	payload := inventoryProductDescribeV3()
	pins := tr.Binding.Pins
	prompt, e := inventoryPayloadV3("Describe", payload)
	if e != nil {
		t.Fatal(e)
	}
	pins.PromptSHA256 = prompt
	f := InventoryFrameV3{Protocol: InventoryProtocolV3, Operation: "Describe", Role: "interpreter", ConfigSHA256: tr.Binding.ConfigSHA256, Grant: tr.Binding.Grant, Sequence: 1, ApplicationSHA256: tr.Binding.ApplicationSHA256, RunID: tr.Binding.RunID, SourceRevision: InventorySourceRevisionV3, LaunchID: tr.Binding.Ready[0].LaunchID, PromptSHA256: tr.Binding.Ready[0].PromptSHA256, Pins: pins, DeadlineUnixNano: time.Now().Add(time.Second).UnixNano(), Payload: payload}
	f.RequestDigest = hash(mustJSON(f))
	raw := mustJSON(f)
	claim, e := tr.reserve(f, raw)
	if e != nil {
		t.Fatal(e)
	}
	return f, raw, claim
}
func TestApplicationInventoryV3ControllerCrashRecovery(t *testing.T) {
	for _, kind := range []string{"empty", "malformed", "overflow", "oversized-file"} {
		t.Run(kind, func(t *testing.T) {
			q, id, j, tr := inventoryProductFixtureV3(t)
			f, raw, claim := inventoryProductClaimV3(t, tr)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			type physical struct {
				raw []byte
				err error
			}
			done := make(chan physical, 1)
			go func() {
				returned, e := exchangePhysicalRoot(ctx, tr.held, tr.Binding.Mailbox, time.Second, cohortFrameHeader{Protocol: f.Protocol, Role: f.Role, RequestDigest: f.RequestDigest, ConfigSHA256: f.ConfigSHA256, Grant: f.Grant, Sequence: 1}, raw)
				done <- physical{returned, e}
			}()
			event, e := WaitInventoryV3(ctx, tr.Binding.Mailbox, "interpreter", 0, q, id)
			if e != nil || event.Bundle == nil {
				t.Fatal(e)
			}
			payload := []byte{}
			if kind == "malformed" {
				payload = []byte("{bad")
			}
			if kind == "overflow" {
				payload = bytes.Repeat([]byte("x"), 65537)
			}
			if kind == "oversized-file" {
				payload = bytes.Repeat([]byte("x"), 70000)
				staged := filepath.Join(tr.Binding.Mailbox, ".reply-00000001.complete")
				if e = os.WriteFile(staged, payload, 0600); e != nil {
					t.Fatal(e)
				}
				e = os.Rename(staged, filepath.Join(tr.Binding.Mailbox, "reply-00000001.json"))
			} else {
				e = SubmitInventoryV3(tr.Binding.Mailbox, "interpreter", 1, payload, q, id)
			}
			if e != nil {
				t.Fatal(e)
			}
			returned := <-done
			observedBytes := min(len(payload), 65537)
			if len(returned.raw) != observedBytes || (returned.err != nil) != (len(payload) > 65536) {
				t.Fatal("ASSERT_V3_BOUNDED_PHYSICAL_PREFIX_BEFORE_ERROR", len(returned.raw), returned.err)
			}
			// Crash boundary: shared physical delivery completed, no own-copy/return commit.
			if j.Head().Observed.Calls != 0 || j.implementation.exists("inventory-return", claim.Reservation.EventID) {
				t.Fatal("ASSERT_V3_CRASH_BEFORE_OWN_COPY")
			}
			before := j.Head().Reserved
			mailbox := tr.Binding.Mailbox
			tr.Close()
			j.Close()
			changed := Ready{Protocol: InventoryProtocolV3, Role: "interpreter", LaunchID: "changed-after-crash", PromptSHA256: q.Configuration.RolePromptSHA256[0]}
			if e = os.WriteFile(filepath.Join(mailbox, "ready-interpreter.json"), mustJSON(changed), 0600); e != nil {
				t.Fatal(e)
			}
			recovered, e := OpenApplication(q.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			defer recovered.Close()
			if e = recovered.ReconcileInventoryReturnsV3(q, id); e != nil {
				t.Fatal("ASSERT_V3_CHANGED_READY_COST_RECOVERY", e)
			}
			if recovered.Head().Observed.Calls != 1 || recovered.Head().Observed.ResponseBytes != uint64(observedBytes) || recovered.Head().Reserved != before || recovered.Head().LastCompletePublicationSHA256 != "" {
				t.Fatal("ASSERT_V3_RECOVERY_PRESERVES_BYTES_WITHOUT_GRANT_OR_AUTHORITY")
			}
			observed := recovered.Head().Observed
			if e = recovered.ReconcileInventoryReturnsV3(q, id); e != nil {
				t.Fatal("ASSERT_V3_RECOVERY_IDEMPOTENT", e)
			}
			after := recovered.Head()
			if after.Observed.Calls != observed.Calls || after.Observed.RequestBytes != observed.RequestBytes || after.Observed.ResponseBytes != observed.ResponseBytes || after.Observed.WireBytes != observed.WireBytes || after.Observed.OutputBytes != observed.OutputBytes || after.Reserved != before {
				t.Fatal("ASSERT_V3_RECOVERY_IDEMPOTENT_NO_REGRANT_OR_SEMANTIC_REPLAY")
			}
			if _, e = os.Stat(filepath.Join(mailbox, "request-00000002.json")); !os.IsNotExist(e) {
				t.Fatal("ASSERT_V3_RECOVERY_NO_RESEND")
			}
			if kind == "empty" {
				for n := 2; n < 16; n++ {
					if e := recovered.ReconcileInventoryReturnsV3(q, id); e != nil {
						t.Fatal(e)
					}
				}
				if e := recovered.ReconcileInventoryReturnsV3(q, id); e != ErrPaused || recovered.Head().Observed.Calls != observed.Calls {
					t.Fatal("ASSERT_V3_COST_RECOVERY_FINITE_PREPAID", e)
				}
			}
			// A conflicting own response copy or mailbox reply never replaces known bytes.
			if kind == "malformed" {
				name := applicationObjectName("inventory-response-0", claim.Reservation.EventID)
				if e = os.WriteFile(filepath.Join(q.Configuration.Store, name), []byte("evil"), 0600); e != nil {
					t.Fatal(e)
				}
				if recovered.ReconcileInventoryReturnsV3(q, id) == nil {
					t.Fatal("ASSERT_V3_OWN_COPY_CONFLICT_REFUSED")
				}
			}
			t.Log("ASSERT_V3_CRASH_BEFORE_OWN_COPY; ASSERT_V3_CHANGED_READY_COST_RECOVERY; ASSERT_V3_RECOVERY_NO_RESEND", kind)
		})
	}
}
func TestApplicationInventoryV3ControllerBindingsAndBounds(t *testing.T) {
	q, id, j, tr := inventoryProductFixtureV3(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e := (InventoryNativeInputsV3{}).Preparer(q.Configuration.SourceRoot, InventorySourceRevisionV3)(ctx, ai.Lead{ID: "missing-live-owner"}); e == nil {
		t.Fatal("ASSERT_V3_ABSENT_LIVE_OWNER_REFUSED")
	}
	v := q.Inventory.Detached()
	v.Manifest.UntouchedGaps[0] = "changed"
	if q.Inventory.Manifest.UntouchedGaps[0] == "changed" {
		t.Fatal("ASSERT_V3_CENSUS_DETACHED")
	}
	f, raw, _ := inventoryProductClaimV3(t, tr)
	if _, e := parseCohortFrame(raw); e == nil {
		t.Fatal("ASSERT_LEGACY_CLOSED_TO_V3")
	}
	for _, kind := range []string{"role", "digest", "config", "grant", "worker", "model", "grammar", "source", "sequence", "operation", "unknown", "duplicate"} {
		g := f
		switch kind {
		case "role":
			g.Role = "reviewer"
		case "digest":
			g.RequestDigest = hash([]byte("wrong"))
		case "config":
			g.ConfigSHA256 = "bad"
		case "grant":
			g.Grant = "bad"
		case "worker":
			g.Pins.WorkerSHA256 = "bad"
		case "model":
			g.Pins.ModelSHA256 = "bad"
		case "grammar":
			g.Pins.GrammarSHA256 = "bad"
		case "source":
			g.SourceRevision = "changed"
		case "sequence":
			g.Sequence = 17
		case "operation":
			g.Operation = "cohort"
		}
		if kind != "digest" {
			g.RequestDigest = ""
			g.RequestDigest = hash(mustJSON(g))
		}
		candidate := mustJSON(g)
		if kind == "unknown" {
			candidate = append(candidate[:len(candidate)-1], []byte(`,"Unknown":true}`)...)
		}
		if kind == "duplicate" {
			candidate = append(candidate[:len(candidate)-1], []byte(`,"Role":"interpreter"}`)...)
		}
		if _, e := parseInventoryFrameV3(candidate); e == nil {
			t.Fatal("ASSERT_V3_CLOSED_FRAME_REFUSED", kind)
		}
	}
	before := j.Head().Reserved
	if _, e := NewInventoryTransportV3(j, q, id, tr.Binding.Mailbox); e == nil {
		t.Fatal("ASSERT_V3_NO_NEW_GRANT")
	}
	if j.Head().Reserved.Calls != before.Calls {
		t.Fatal("ASSERT_V3_NO_REGRANT_CALLS")
	}
	if e := j.ReconcileInventoryReturnsV3(q, hash([]byte("foreign ticket"))); e == nil {
		t.Fatal("ASSERT_V3_RECOVERY_TICKET_BOUND")
	}
	t.Log("ASSERT_LEGACY_CLOSED_TO_V3; ASSERT_V3_ABSENT_LIVE_OWNER_REFUSED; ASSERT_V3_CLOSED_FRAME_REFUSED")
}
func TestApplicationInventoryV3ControllerLegacyControls(t *testing.T) {
	c, now := applicationFixture(t)
	raw := mustJSON(c)
	j := applicationCreate(t, c, now)
	if j.Head().Schema != "private-host-application-head.v1" || !bytes.Equal(raw, mustJSON(c)) || c.RolePromptSHA256 != [2]string{hash([]byte(CohortRolePrompt("interpreter"))), hash([]byte(CohortRolePrompt("reviewer")))} {
		t.Fatal("ASSERT_LEGACY_CONFIG_BYTES_AND_PROMPTS")
	}
	var decoded ApplicationConfig
	if json.Unmarshal(raw, &decoded) != nil || decoded != c {
		t.Fatal("ASSERT_LEGACY_CONFIG_ROUNDTRIP")
	}
	t.Log(fmt.Sprintf("ASSERT_LEGACY_CONFIG_BYTES_AND_PROMPTS %d bytes", len(raw)))
	for _, navigation := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy-physical-navigation-%t", navigation), func(t *testing.T) {
			mailbox, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if e := os.Chmod(mailbox, 0700); e != nil {
				t.Fatal(e)
			}
			for _, role := range []string{"interpreter", "reviewer"} {
				if e := MarkCohortReady(mailbox, Ready{Protocol: CohortProtocol, Role: role, LaunchID: "legacy-" + role, PromptSHA256: hash([]byte(CohortRolePrompt(role)))}); e != nil {
					t.Fatal(e)
				}
			}
			var raw, reply []byte
			if navigation {
				task := CohortNavigationTask{Protocol: CohortNavigationProtocol, Role: "interpreter", Sequence: 1}
				task.RequestDigest = hash(mustJSON(task))
				raw = mustJSON(task)
				reply = mustJSON(CohortNavigationResponse{Protocol: CohortNavigationProtocol, RequestDigest: task.RequestDigest, Groups: []CohortNavigation{}})
			} else {
				task := CohortTask{Protocol: CohortProtocol, Role: "interpreter", Sequence: 1}
				task.RequestDigest = hash(mustJSON(task))
				raw = mustJSON(task)
				reply = mustJSON(CohortResponse{Protocol: CohortProtocol, RequestDigest: task.RequestDigest, Items: []CohortDecision{}})
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			type physical struct {
				raw []byte
				err error
			}
			done := make(chan physical, 1)
			go func() {
				got, e := (CohortMailbox{Directory: mailbox, ExchangeTimeout: time.Second}).ExchangeCohort(ctx, raw)
				done <- physical{got, e}
			}()
			event, e := WaitCohort(ctx, mailbox, "interpreter", 0)
			if e != nil || event.Bundle == nil {
				t.Fatal("ASSERT_LEGACY_PHYSICAL_BUNDLE", e)
			}
			if e := SubmitCohort(mailbox, "interpreter", 1, reply); e != nil {
				t.Fatal("ASSERT_LEGACY_TYPED_SUBMIT", e)
			}
			got := <-done
			if got.err != nil || !bytes.Equal(got.raw, reply) {
				t.Fatal("ASSERT_LEGACY_PHYSICAL_RETURN_BYTES", got.err)
			}
		})
	}
}
