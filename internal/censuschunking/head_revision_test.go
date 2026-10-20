package censuschunking

import (
	"bytes"
	"testing"
)

func TestParseCoordinatorHeadRevisionStrict(t *testing.T) {
	head := coordinatorHead{SchemaVersion: CoordinatorHeadVersion, WorkflowKey: "workflow", RootSelector: "root", PlanDigest: "plan", IndexDigest: "index", SessionID: "session", SessionGeneration: 1, WorkspaceDigest: "workspace", Generation: 3, State: headCommitted, ClaimOrdinal: -1}
	setHeadIdentity(&head)
	raw, err := encodeCoordinator(head)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseCoordinatorHeadRevision(raw)
	if err != nil || got != (HeadRevision{Generation: head.Generation, Selector: head.ImmutableSelector}) {
		t.Fatalf("ASSERT_HEAD_REVISION_VALID got=%+v err=%v", got, err)
	}
	cases := map[string][]byte{
		"empty":        nil,
		"malformed":    []byte(`{"bad":true}`),
		"noncanonical": append(bytes.Clone(raw), '\n'),
		"tampered":     bytes.Replace(bytes.Clone(raw), []byte(`"generation":3`), []byte(`"generation":4`), 1),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCoordinatorHeadRevision(candidate); err == nil {
				t.Fatal("ASSERT_HEAD_REVISION_REJECTS_INVALID")
			}
		})
	}
}

func TestParseCoordinatorHeadRevisionRejectsSelfRehashedStructuralViolations(t *testing.T) {
	valid := coordinatorHead{SchemaVersion: CoordinatorHeadVersion, WorkflowKey: "workflow", RootSelector: "root", PlanDigest: "plan", IndexDigest: "index", SessionID: "session", SessionGeneration: 1, WorkspaceDigest: "workspace", Generation: 3, State: headCommitted, ClaimOrdinal: -1}
	rehashed := func(t *testing.T, mutate func(*coordinatorHead)) []byte {
		t.Helper()
		h := valid
		mutate(&h)
		setHeadIdentity(&h)
		raw, err := encodeCoordinator(h)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	cases := map[string]func(*coordinatorHead){
		"unknown-state":             func(h *coordinatorHead) { h.State = "UNKNOWN" },
		"empty-workflow":            func(h *coordinatorHead) { h.WorkflowKey = "" },
		"empty-root":                func(h *coordinatorHead) { h.RootSelector = "" },
		"empty-plan":                func(h *coordinatorHead) { h.PlanDigest = "" },
		"empty-index":               func(h *coordinatorHead) { h.IndexDigest = "" },
		"empty-session":             func(h *coordinatorHead) { h.SessionID = "" },
		"zero-session-generation":   func(h *coordinatorHead) { h.SessionGeneration = 0 },
		"empty-workspace":           func(h *coordinatorHead) { h.WorkspaceDigest = "" },
		"zero-generation":           func(h *coordinatorHead) { h.Generation = 0 },
		"negative-prefix":           func(h *coordinatorHead) { h.CommittedPrefix = -1 },
		"committed-claim-ordinal":   func(h *coordinatorHead) { h.ClaimOrdinal = 0 },
		"committed-idempotency":     func(h *coordinatorHead) { h.IdempotencyKey = "claim" },
		"committed-receipt":         func(h *coordinatorHead) { h.ReceiptSelector = "receipt" },
		"committed-manifest":        func(h *coordinatorHead) { h.ManifestSelector = "manifest" },
		"committed-terminal":        func(h *coordinatorHead) { h.TerminalSelector = "terminal" },
		"claimed-wrong-ordinal":     func(h *coordinatorHead) { h.State = headClaimed; h.ClaimOrdinal = 1; h.IdempotencyKey = "claim" },
		"claimed-empty-idempotency": func(h *coordinatorHead) { h.State = headClaimed; h.ClaimOrdinal = 0 },
		"claimed-receipt-only": func(h *coordinatorHead) {
			h.State = headClaimed
			h.ClaimOrdinal = 0
			h.IdempotencyKey = "claim"
			h.ReceiptSelector = "receipt"
		},
		"claimed-manifest-only": func(h *coordinatorHead) {
			h.State = headClaimed
			h.ClaimOrdinal = 0
			h.IdempotencyKey = "claim"
			h.ManifestSelector = "manifest"
		},
		"claimed-terminal": func(h *coordinatorHead) {
			h.State = headClaimed
			h.ClaimOrdinal = 0
			h.IdempotencyKey = "claim"
			h.TerminalSelector = "terminal"
		},
		"terminal-claim": func(h *coordinatorHead) {
			h.State = headTerminal
			h.ClaimOrdinal = 0
			h.IdempotencyKey = "claim"
			h.TerminalSelector = "terminal"
		},
		"terminal-receipt": func(h *coordinatorHead) {
			h.State = headTerminal
			h.TerminalSelector = "terminal"
			h.ReceiptSelector = "receipt"
		},
		"terminal-manifest": func(h *coordinatorHead) {
			h.State = headTerminal
			h.TerminalSelector = "terminal"
			h.ManifestSelector = "manifest"
		},
		"terminal-empty-selector": func(h *coordinatorHead) { h.State = headTerminal },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCoordinatorHeadRevision(rehashed(t, mutate)); err == nil {
				t.Fatal("ASSERT_HEAD_REVISION_REJECTS_SELF_REHASHED_STRUCTURAL_VIOLATION")
			}
		})
	}
}
