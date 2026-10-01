package adr0011genericv5proposal

import (
	"fmt"
	"testing"
)

func b4Frame(s string) []byte { return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(s), s)) }
func b4Base() B4Input {
	role := B4RoleBinding{Session: "s", Generation: 1, Transaction: "t", Workspace: "file:///w", URI: "file:///w/a.go", Source: []byte("abc"), Language: ptrB4("go")}
	return B4Input{Query: role, SourceRole: role, Applicability: role, Session: "s", Generation: 1, Transaction: "t", Workspace: "file:///w", Method: "textDocument/references", URI: "file:///w/a.go", SourceURI: "file:///w/a.go", Source: []byte("abc"), QuerySource: []byte("abc"), WriteOrdinal: 8, WriteCompleted: true, WriteFrame: b4Frame(`{"jsonrpc":"2.0","id":3,"method":"textDocument/references","params":{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}}`), Frames: []B4Frame{
		{1, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)},
		{2, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":1.0,"result":{"capabilities":{"referencesProvider":false}}`)},
		{3, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)},
		{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file","language":"go"}]}}]}}`)},
		{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
	}, Language: ptrB4("go")}
}
func ptrB4(s string) *string { return &s }
func TestB4REDVerifiedExchangeAndTargetWrite(t *testing.T) {
	in := b4Base()
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1.0,"result":{"capabilities":{"referencesProvider":false}}}`)
	if got := ReplayB4(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("held successful registration: %+v", got)
	}
}
func TestB4REDPendingUnregisterAndLateReply(t *testing.T) {
	in := b4Base()
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1.0,"result":{"capabilities":{"referencesProvider":false}}}`)
	in.Frames = append(in.Frames, B4Frame{6, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":2,"method":"client/unregisterCapability","params":{"unregisterations":[{"id":"x","method":"textDocument/references"}]}}`)})
	if got := ReplayB4(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("pending unregister: %+v", got)
	}
	in.Frames = append(in.Frames, B4Frame{7, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":2,"result":null}`)})
	if got := ReplayB4(in); got.Outcome != "UNSUPPORTED" {
		t.Fatalf("ack unregister: %+v", got)
	}
}
func TestB4REDInvalidTypedAndChronology(t *testing.T) {
	cases := map[string]func(*B4Input){"premature": func(in *B4Input) { in.Frames[2].Ordinal = 9 }, "wrong ID": func(in *B4Input) { in.Frames[4].Bytes = b4Frame(`{"jsonrpc":"2.0","id":true,"result":null}`) }, "duplicate decoded key": func(in *B4Input) {
		in.Frames[4].Bytes = b4Frame(`{"jsonrpc":"2.0","id":"r","\u0069d":"r","result":null}`)
	}, "reversed reply": func(in *B4Input) { in.Frames[4].Ordinal = 3 }, "unmatched reply": func(in *B4Input) { in.Frames[4].Bytes = b4Frame(`{"jsonrpc":"2.0","id":"other","result":null}`) }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := b4Base()
			in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1.0,"result":{"capabilities":{}}}`)
			change(&in)
			if got := ReplayB4(in); got.Outcome != "INVALID_CHRONOLOGY" {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestB4REDHeldRoleMismatch(t *testing.T) {
	in := b4Base()
	in.Applicability.Transaction = "other"
	if got := ReplayB4(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("%+v", got)
	}
}
func TestB4StaticDefinitionAndConflict(t *testing.T) {
	in := b4Base()
	in.Method = "textDocument/definition"
	in.WriteFrame = b4Frame(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{}}`)
	in.Frames = in.Frames[:3]
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"definitionProvider":{"workDoneProgress":false}}}}`)
	if got := ReplayB4(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("static: %+v", got)
	}
	in.Frames = append(in.Frames, B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":2,"method":"client/registerCapability","params":{"registrations":[{"id":"d","method":"textDocument/definition"}]}}`)}, B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":2,"result":null}`)})
	if got := ReplayB4(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("conflict: %+v", got)
	}
}
func TestB4ErrorAndPendingRegistration(t *testing.T) {
	in := b4Base()
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	in.Frames = in.Frames[:4]
	if got := ReplayB4(in); got.Outcome != "UNKNOWN" {
		t.Fatalf("pending: %+v", got)
	}
	in.Frames = append(in.Frames, B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","error":{"code":-1,"message":"no"}}`)})
	if got := ReplayB4(in); got.Outcome != "UNSUPPORTED" {
		t.Fatalf("error: %+v", got)
	}
}
func TestB4NotebookSelectorHeldContext(t *testing.T) {
	in := b4Base()
	in.NotebookCell = true
	in.NotebookURI = ptrB4("file:///w/book")
	in.NotebookType = ptrB4("python")
	for _, r := range []*B4RoleBinding{&in.Query, &in.SourceRole, &in.Applicability} {
		r.NotebookCell = true
		r.NotebookURI = in.NotebookURI
		r.NotebookType = in.NotebookType
	}
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	in.Frames[3].Bytes = b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"notebook":{"notebookType":"python","scheme":"file"},"language":"go"}]}}]}}`)
	if got := ReplayB4(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("notebook: %+v", got)
	}
}
func TestB4REDMissingWriteIsPartial(t *testing.T) {
	in := b4Base()
	in.WriteFrame = nil
	if got := ReplayB4(in); got.Outcome != "PARTIAL" {
		t.Fatalf("%+v", got)
	}
}
func TestB4REDUnknownGlobAndClaimant(t *testing.T) {
	in := b4Base()
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1.0,"result":{"capabilities":{}}}`)
	in.Frames[3].Bytes = b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file","pattern":"**/*.go"}]}}]}}`)
	in.ClaimedOutcome = "SUPPORTED"
	if got := ReplayB4(in); got.Outcome != "UNKNOWN" || got.ClaimMatches {
		t.Fatalf("glob must not authorize: %+v", got)
	}
}
