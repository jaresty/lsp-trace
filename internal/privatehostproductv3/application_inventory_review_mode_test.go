//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInventoryReviewSameThreadReadyV3(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "mailbox")
	if err = PrepareMailbox(path); err != nil {
		t.Fatal(err)
	}
	s, err := mailboxStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	// Construct the actual future wire record, so the old strict reader is
	// exercised rather than a missing API causing a compilation failure.
	for _, role := range []string{"interpreter", "reviewer"} {
		raw, _ := json.Marshal(struct {
			Protocol, Role, LaunchID, PromptSHA256, ReviewMode string
		}{InventoryProtocolV3, role, "one-caller-context", hash([]byte(InventoryRolePromptV3(role))), "SAME_THREAD"})
		if err = s.write("ready-"+role+".json", raw); err != nil {
			t.Fatal(err)
		}
	}
	ready, err := inventoryReadReadyV3(s)
	if err != nil || ready[0].LaunchID != ready[1].LaunchID {
		t.Fatal("ASSERT_REVIEW_SAME_THREAD_READY", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_REVIEW_SAME_THREAD_READY: PASS")
}

func reviewReadyPairV3(t *testing.T, mode cat.ReviewModeV3, same bool) (string, *store) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "mailbox")
	if err = PrepareMailbox(path); err != nil {
		t.Fatal(err)
	}
	s, err := mailboxStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.close() })
	for _, role := range []string{"interpreter", "reviewer"} {
		id := "caller-" + role
		if same {
			id = "one-caller-context"
		}
		r := InventoryReadyV3{Ready: Ready{Protocol: InventoryProtocolV3, Role: role, LaunchID: id, PromptSHA256: hash([]byte(InventoryRolePromptV3(role)))}, ReviewMode: mode}
		if err = s.write("ready-"+role+".json", mustJSON(r)); err != nil {
			t.Fatal(err)
		}
	}
	return path, s
}

func TestInventoryReviewSameThreadRegistrationV3(t *testing.T) {
	for _, same := range []bool{false, true} {
		path, s := reviewReadyPairV3(t, cat.ReviewModeSameThreadV3, same)
		// Remove the generated records and exercise the actual registration API.
		for _, role := range []string{"interpreter", "reviewer"} {
			if err := s.root.Remove("ready-" + role + ".json"); err != nil {
				t.Fatal(err)
			}
		}
		for _, role := range []string{"interpreter", "reviewer"} {
			id := "caller-" + role
			if same {
				id = "registered-one-context"
			}
			r := Ready{Protocol: InventoryProtocolV3, Role: role, LaunchID: id, PromptSHA256: hash([]byte(InventoryRolePromptV3(role)))}
			if err := MarkInventoryReadyV3(path, r, cat.ReviewModeSameThreadV3); err != nil {
				t.Fatal("ASSERT_REVIEW_SAME_THREAD_REGISTRATION", same, role, err)
			}
		}
		ready, err := inventoryReadReadyV3(s)
		if err != nil || ready[0].ReviewMode != cat.ReviewModeSameThreadV3 || ready[1].ReviewMode != cat.ReviewModeSameThreadV3 {
			t.Fatal("ASSERT_REVIEW_SAME_THREAD_REGISTRATION", same, err)
		}
	}
	t.Log("ASSERT_REVIEW_SAME_THREAD_REGISTRATION: PASS")
}

func TestInventoryReviewStrictReadyV3(t *testing.T) {
	for _, mode := range []cat.ReviewModeV3{"", cat.ReviewModeSubagentsV3} {
		path, s := reviewReadyPairV3(t, mode, true)
		if _, err := inventoryReadReadyV3(s); err == nil {
			t.Fatal("ASSERT_REVIEW_STRICT_READY", mode, "reader accepted equal actors")
		}
		if err := s.root.Remove("ready-reviewer.json"); err != nil {
			t.Fatal(err)
		}
		r := Ready{Protocol: InventoryProtocolV3, Role: "reviewer", LaunchID: "one-caller-context", PromptSHA256: hash([]byte(InventoryRolePromptV3("reviewer")))}
		if err := MarkInventoryReadyV3(path, r, mode); err == nil {
			t.Fatal("ASSERT_REVIEW_STRICT_READY", mode, "registration accepted equal actors")
		}
		_, distinct := reviewReadyPairV3(t, mode, false)
		if _, err := inventoryReadReadyV3(distinct); err != nil {
			t.Fatal("ASSERT_REVIEW_STRICT_READY", mode, "distinct actors refused", err)
		}
	}
	t.Log("ASSERT_REVIEW_STRICT_READY: PASS")
}

func TestInventoryReviewModeBindingV3(t *testing.T) {
	q, id, j, path := inventoryProductSetupV3(t, "")
	// The approved request stays byte-for-byte unchanged; only readiness is
	// replaced with a different, valid declaration under distinct identities.
	s, err := mailboxStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	for _, role := range []string{"interpreter", "reviewer"} {
		if err = s.root.Remove("ready-" + role + ".json"); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{"interpreter", "reviewer"} {
		r := Ready{Protocol: InventoryProtocolV3, Role: role, LaunchID: "scripted-" + role, PromptSHA256: hash([]byte(InventoryRolePromptV3(role)))}
		if err = MarkInventoryReadyV3(path, r, cat.ReviewModeSameThreadV3); err != nil {
			t.Fatal(err)
		}
	}
	before := j.Head().Reserved
	if tr, err := NewInventoryTransportV3(j, q, id, path); err == nil {
		tr.Close()
		t.Fatal("ASSERT_REVIEW_MODE_BINDING", "mismatched readiness admitted")
	}
	if j.Head().Reserved != before {
		t.Fatal("ASSERT_REVIEW_MODE_BINDING", "refusal reserved native credit")
	}
	changed := q
	changed.Inventory.ReviewMode = cat.ReviewModeSameThreadV3
	if _, err := DecodeInventoryRequestV3(mustJSON(changed)); err == nil {
		t.Fatal("ASSERT_REVIEW_MODE_BINDING", "changed mode retained old approval")
	}
	changed.Inventory.ReviewMode = "invented-mode"
	changed.Configuration.UnitPolicySHA256 = hash(mustJSON(changed.Inventory))
	changed.Configuration.ProducerConfigSHA256 = changed.Configuration.UnitPolicySHA256
	changed.Configuration.ApprovalSHA256 = ApplicationConfigDigest(changed.Configuration)
	changed.ApprovalSHA256 = InventoryRequestDigestV3(changed)
	if ValidateInventoryRequestV3(changed) == nil {
		t.Fatal("ASSERT_REVIEW_MODE_BINDING", "invalid mode accepted with fresh seals")
	}
	for _, mode := range []cat.ReviewModeV3{cat.ReviewModeSameThreadV3, cat.ReviewModeSubagentsV3} {
		request, _, _, transport := inventoryProductFixtureV3(t, mode)
		run := request.Inventory.RunRequest(context.Background(), InventoryNativeInputsV3{}, request.Configuration.SourceRoot, "unused", "actor", "reviewer", transport)
		if run.ReviewMode != mode || transport.Binding.Ready[0].ReviewMode != mode || transport.Binding.Ready[1].ReviewMode != mode {
			t.Fatal("ASSERT_REVIEW_MODE_BINDING", mode, "runtime/binding propagation")
		}
	}
	t.Log("ASSERT_REVIEW_MODE_BINDING: PASS")
}

func TestInventoryReviewLegacyEncodingV3(t *testing.T) {
	q, _, _, transport := inventoryProductFixtureV3(t)
	for _, ready := range transport.Binding.Ready {
		if !bytes.Equal(mustJSON(ready), mustJSON(ready.Ready)) {
			t.Fatal("ASSERT_REVIEW_LEGACY_READY_ENCODING", "legacy record changed")
		}
	}
	var options map[string]json.RawMessage
	if json.Unmarshal(mustJSON(q.Inventory), &options) != nil || options["ReviewMode"] != nil {
		t.Fatal("ASSERT_REVIEW_LEGACY_READY_ENCODING", "legacy options gained a mode")
	}
	t.Log("ASSERT_REVIEW_LEGACY_READY_ENCODING: PASS")
}

func TestInventoryReviewSameThreadExchangeV3(t *testing.T) {
	q, id, j, transport := inventoryProductFixtureV3(t, cat.ReviewModeSameThreadV3)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload := mustJSON(cat.NativeCatalogWireV3{Version: "lsp-trace.native-catalog.v3", Revision: hash([]byte("scripted catalog")), Completeness: "UNKNOWN", Absence: "PROHIBITED", ReviewMode: cat.ReviewModeSameThreadV3})
	for i, operation := range []string{"GroupDecision", "Reviewer"} {
		role := inventoryRoleV3(operation)
		var output bytes.Buffer
		done := make(chan error, 1)
		go func() { done <- transport.Exchange(operation)(ctx, payload, &output) }()
		event, err := WaitInventoryV3(ctx, transport.Binding.Mailbox, role, 0, q, id)
		if err != nil || event.Event != "NEEDS_HOST" || event.Bundle == nil {
			t.Fatal("ASSERT_REVIEW_SAME_THREAD_EXCHANGE", operation, event.Event, err)
		}
		if err = SubmitInventoryV3(transport.Binding.Mailbox, role, uint64(i+1), []byte("{}\n"), q, id); err != nil {
			t.Fatal("ASSERT_REVIEW_SAME_THREAD_EXCHANGE", operation, err)
		}
		if err = <-done; err != nil || output.String() != "{}\n" {
			t.Fatal("ASSERT_REVIEW_SAME_THREAD_EXCHANGE", operation, err, output.String())
		}
	}
	if j.Head().Observed.Calls != 2 || j.Head().Observed.ResponseBytes != 6 {
		t.Fatal("ASSERT_REVIEW_SAME_THREAD_EXCHANGE", "physical return accounting")
	}
	t.Log("ASSERT_REVIEW_SAME_THREAD_EXCHANGE: PASS")
}
