//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	pilot "lsp-trace/internal/censuslineagepilot"
	dr "lsp-trace/internal/describerequest"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func inventoryTestTyped(t *testing.T, q inventoryTestRequest) product.InventoryRequestV3 {
	t.Helper()
	raw, _ := json.Marshal(q)
	v, e := product.DecodeInventoryRequestV3(raw)
	if e != nil {
		t.Fatal("typed fixture", e)
	}
	return v
}
func inventoryTestOpenTransport(t *testing.T, s inventoryTestSession) (*product.ApplicationJournal, *product.InventoryTransportV3) {
	t.Helper()
	q := inventoryTestTyped(t, s.q)
	j, e := product.OpenApplication(q.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { j.Close() })
	tr, e := product.NewInventoryTransportV3(j, q, s.runID, product.InventoryMailboxPathV3(q.Configuration.Store, s.runID))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(tr.Close)
	return j, tr
}
func inventoryTestDescribe() []byte {
	prompt := "SYNTHETIC DESCRIBE TRANSPORT ONLY, NOT NATIVE SELECTED"
	r := dr.Record{RecordID: "synthetic", Envelope: dr.Envelope{Protocol: dr.Protocol, MessageType: dr.MessageType, SourceRevision: product.InventorySourceRevisionV3, InputSHA256: strings.TrimPrefix(pilot.Digest([]byte(prompt)), "sha256:"), InputBytes: len(prompt), Prompt: prompt}, Lineage: dr.Lineage{Completeness: "UNKNOWN"}}
	b, _ := json.Marshal(r)
	return b
}
func inventoryTestAwaitDescribe(t *testing.T, s inventoryTestSession) applicationHostEvent {
	t.Helper()
	n, raw, e := inventoryTestEntry([]string{"wait", s.path, s.runID, "interpreter", "0", "1500"}, nil)
	if n != 0 {
		t.Fatal("wait", e)
	}
	var event applicationHostEvent
	if json.Unmarshal(raw, &event) != nil || event.Event != "NEEDS_HOST" || event.Bundle == nil {
		t.Fatal("ASSERT_V3_SCRIPTED_DESCRIBE_ENTRY_MAILBOX", string(raw))
	}
	f, e2 := product.ReadInventoryBundleV3(*event.Bundle)
	if e2 != nil || f.Operation != "Describe" {
		t.Fatal("Describe", e2)
	}
	return event
}
func TestApplicationInventoryV3ControllerBindingsAndBounds(t *testing.T) {
	t.Run("ready-alias-and-prompt", func(t *testing.T) {
		path, q := inventoryTestConfig(t)
		n, raw, e := inventoryTestEntry([]string{"setup", path}, nil)
		if n != 0 {
			t.Fatal(e)
		}
		var v struct{ RunID string }
		json.Unmarshal(raw, &v)
		n, _, _ = inventoryTestEntry([]string{"register", path, v.RunID, "interpreter", "synthetic-one", "wrong"}, nil)
		if n == 0 {
			t.Fatal("ASSERT_V3_WRONG_PROMPT_REFUSED")
		}
		n, _, e = inventoryTestEntry([]string{"register", path, v.RunID, "interpreter", "synthetic-one", q.Configuration.RolePromptSHA256[0]}, nil)
		if n != 0 {
			t.Fatal(e)
		}
		n, _, _ = inventoryTestEntry([]string{"register", path, v.RunID, "reviewer", "synthetic-one", q.Configuration.RolePromptSHA256[1]}, nil)
		if n == 0 {
			t.Fatal("ASSERT_V3_ROLE_ALIAS_REFUSED")
		}
		if _, e := os.Stat(applicationRoleReadyPath(q.Configuration.Store, v.RunID, "reviewer")); !os.IsNotExist(e) {
			t.Fatal("ASSERT_V3_ALIAS_NO_EFFECT")
		}
	})
	t.Run("scripted-describe-production-entry", func(t *testing.T) {
		s := inventoryTestSetup(t)
		j, tr := inventoryTestOpenTransport(t, s)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		done := make(chan error, 1)
		var out bytes.Buffer
		go func() { done <- tr.Exchange("Describe")(ctx, inventoryTestDescribe(), &out) }()
		event := inventoryTestAwaitDescribe(t, s)
		// All alterations enter the same production entry and cannot publish a reply.
		for _, kind := range []string{"worker", "model", "grammar", "grant"} {
			q := s.q
			switch kind {
			case "worker":
				q.Inventory.Pins.WorkerSHA256 = pilot.Digest([]byte("foreign-worker"))
			case "model":
				q.Inventory.Pins.ModelSHA256 = pilot.Digest([]byte("foreign-model"))
			case "grammar":
				q.Inventory.Pins.GrammarSHA256 = pilot.Digest([]byte("foreign-grammar"))
			case "grant":
				q.Configuration.Grant.ExpiresUnixNano++
			}
			inventoryTestSeal(&q)
			inventoryTestWrite(t, s.path, q)
			if n, _, _ := inventoryTestEntry([]string{"respond", s.path, s.runID, "interpreter", fmt.Sprint(event.GlobalSequence)}, []byte("{}")); n == 0 {
				t.Fatal("ASSERT_V3_PIN_SUBSTITUTION_REFUSED", kind)
			}
		}
		inventoryTestWrite(t, s.path, s.q)
		for _, args := range [][]string{{"respond", s.path, s.runID, "reviewer", "1"}, {"respond", s.path, s.runID, "interpreter", "2"}, {"respond", s.path, s.runID, "interpreter", "17"}} {
			if n, _, _ := inventoryTestEntry(args, []byte("{}")); n == 0 {
				t.Fatal("ASSERT_V3_ROLE_SEQUENCE_REFUSED")
			}
		}
		if _, e := os.Stat(filepath.Join(tr.Binding.Mailbox, "reply-00000001.json")); !os.IsNotExist(e) {
			t.Fatal("ASSERT_V3_BAD_BINDING_NO_RESPONSE")
		}
		if n, _, e := inventoryTestEntry([]string{"respond", s.path, s.runID, "interpreter", "1"}, []byte("{}")); n != 0 {
			t.Fatal(e)
		}
		if e := <-done; e != nil {
			t.Fatal(e)
		}
		if out.String() != "{}" || j.Head().Observed.Calls != 1 || j.Head().Observed.ResponseBytes != 2 {
			t.Fatal("ASSERT_V3_SCRIPTED_DESCRIBE_PHYSICAL_ACCOUNTING")
		}
		t.Log("ASSERT_V3_SCRIPTED_DESCRIBE_ENTRY_MAILBOX (not native-selected Describe)")
	})
	t.Run("directory-swap", func(t *testing.T) {
		s := inventoryTestSetup(t)
		j, tr := inventoryTestOpenTransport(t, s)
		before := j.Head()
		old := tr.Binding.Mailbox + "-held"
		if e := os.Rename(tr.Binding.Mailbox, old); e != nil {
			t.Fatal(e)
		}
		if e := os.Mkdir(tr.Binding.Mailbox, 0700); e != nil {
			t.Fatal(e)
		}
		if e := tr.Exchange("Describe")(context.Background(), inventoryTestDescribe(), &bytes.Buffer{}); e == nil {
			t.Fatal("ASSERT_V3_DIRECTORY_SWAP_REFUSED")
		}
		if j.Head().Observed != before.Observed {
			t.Fatal("ASSERT_V3_SWAP_NO_EXCHANGE")
		}
	})
	t.Run("aggregate-wrapper-prenative", func(t *testing.T) {
		s := inventoryTestSetup(t)
		j, tr := inventoryTestOpenTransport(t, s)
		before := j.Head()
		raw := []byte(`{"huge":"` + strings.Repeat("x", 1<<20) + `"}`)
		if e := tr.Exchange("Describe")(context.Background(), raw, &bytes.Buffer{}); e == nil {
			t.Fatal("ASSERT_V3_WRAPPER_OVERSIZE_REFUSED")
		}
		if j.Head() != before {
			t.Fatal("ASSERT_V3_OVERSIZE_NO_EFFECTS")
		}
	})
}
func TestApplicationInventoryV3ControllerReturnedBytes(t *testing.T) {
	for _, kind := range []string{"empty", "malformed", "overflow", "cancel-without-reply", "cancel-with-reply"} {
		t.Run(kind, func(t *testing.T) {
			s := inventoryTestSetup(t)
			j, tr := inventoryTestOpenTransport(t, s)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- tr.Exchange("Describe")(ctx, inventoryTestDescribe(), &bytes.Buffer{}) }()
			inventoryTestAwaitDescribe(t, s)
			payload := []byte{}
			switch kind {
			case "malformed":
				payload = []byte("{bad")
			case "overflow":
				payload = bytes.Repeat([]byte("x"), 65537)
			case "cancel-with-reply":
				payload = []byte("{cancelled}")
			}
			if kind != "cancel-without-reply" {
				if n, _, e := inventoryTestEntry([]string{"respond", s.path, s.runID, "interpreter", "1"}, payload); n != 0 {
					t.Fatal(e)
				}
			}
			if strings.HasPrefix(kind, "cancel") {
				cancel()
			}
			if e := <-done; e == nil {
				t.Fatal("ASSERT_V3_INVALID_PHYSICAL_RETURN_REFUSED")
			}
			if j.Head().Observed.Calls != 1 || j.Head().Observed.ResponseBytes != uint64(len(payload)) {
				t.Fatalf("ASSERT_V3_ALL_RETURNED_BYTES_CHARGED: kind=%s got=%d want=%d", kind, j.Head().Observed.ResponseBytes, len(payload))
			}
			before := j.Head().Reserved
			if e := j.ReconcileInventoryReturnsV3(inventoryTestTyped(t, s.q), s.runID); e != nil && kind != "cancel-without-reply" {
				t.Fatal(e)
			}
			if j.Head().Reserved != before {
				t.Fatal("ASSERT_V3_RECOVERY_NO_REGRANT")
			}
		})
	}
}
