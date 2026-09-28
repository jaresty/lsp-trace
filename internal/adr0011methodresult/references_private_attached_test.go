package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
)

func privateIdentity(key lspwire.RequestKey, raw []byte) PrivateTransitionIdentity {
	o := testTransitionOwner()
	return PrivateTransitionIdentity{o.Transaction, adr0011requestkey.Encode(key), o.Invocation, o.ResponseRead, o.RawSelector, chainDigest(raw)}
}

func TestRunPrivateAttachedReferences(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	for _, tc := range []struct {
		name, raw string
		count     int
	}{{"empty", "[]", 2}, {"equal-locations", "[" + attachedLocation + "," + attachedLocation + "]", 6}} {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := journalRoot(t)
			raw := json.RawMessage(tc.raw)
			identity := privateIdentity(key, raw)
			eval, final, err := RunPrivateAttachedReferences(root, key, raw, identity)
			if err != nil || eval == nil || len(final) == 0 || eval.P != (tc.count-2)/2 {
				t.Fatalf("no complete evaluation: %v %+v", err, eval)
			}
			owner := referenceTransitionOwner{identity.Transaction, identity.RequestKey, identity.Invocation, identity.ResponseRead, identity.RawSelector, identity.RawDigest}
			events, err := decodeReferenceTransitionPrefix(final, owner)
			if err != nil || len(events) != tc.count || ReplayReferenceTransitionJournal(root, final, owner) != nil {
				t.Fatalf("invalid replay/count: %v %d", err, len(events))
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != tc.count {
				t.Fatalf("immutable prefix count: %v %d", err, len(entries))
			}
			final[0] ^= 1
			stored, err := publication.ReadVerifiedBoundFile(root, referenceTransitionJournalSelector(journalBytes(owner, events...)), referenceTransitionMaxBytes)
			if err != nil || bytes.Equal(final, stored) {
				t.Fatal("returned bytes alias storage")
			}
		})
	}
}

func TestRunPrivateAttachedReferencesFailClosed(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	raw := json.RawMessage("[" + attachedLocation + "]")
	for _, tc := range []struct {
		name   string
		change func(*PrivateTransitionIdentity, *json.RawMessage, *lspwire.RequestKey)
	}{{"missing-raw", func(i *PrivateTransitionIdentity, r *json.RawMessage, k *lspwire.RequestKey) { *r = nil }},
		{"raw-substitution", func(i *PrivateTransitionIdentity, r *json.RawMessage, k *lspwire.RequestKey) {
			*r = json.RawMessage("[]")
		}},
		{"key-substitution", func(i *PrivateTransitionIdentity, r *json.RawMessage, k *lspwire.RequestKey) { k.ID++ }},
		{"missing-owner", func(i *PrivateTransitionIdentity, r *json.RawMessage, k *lspwire.RequestKey) { i.RawSelector = "" }},
		{"malformed", func(i *PrivateTransitionIdentity, r *json.RawMessage, k *lspwire.RequestKey) {
			*r = json.RawMessage("[{")
			i.RawDigest = chainDigest(*r)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := journalRoot(t)
			identity, input, selected := privateIdentity(key, raw), append(json.RawMessage(nil), raw...), key
			tc.change(&identity, &input, &selected)
			eval, final, err := RunPrivateAttachedReferences(root, selected, input, identity)
			if err == nil || eval != nil || final != nil {
				t.Fatalf("false success: %v %+v %q", err, eval, final)
			}
			entries, e := os.ReadDir(dir)
			if e != nil || len(entries) != 0 {
				t.Fatalf("unexpected journal: %v %d", e, len(entries))
			}
		})
	}
	for _, tc := range []struct {
		name, stage string
		occurrence  int
	}{
		{"precommit", "TARGET", 1}, {"postcommit", "RECEIPT", 1}, {"readback", "RECEIPT", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := journalRoot(t)
			seen := 0
			trace := func(event publication.BoundFileTraceEvent) {
				if event.Stage != tc.stage {
					return
				}
				seen++
				if seen != tc.occurrence {
					return
				}
				entries, _ := os.ReadDir(dir)
				if tc.stage == "TARGET" {
					// A no-replace collision prevents the first callback from committing.
					owner := privateIdentity(key, raw)
					o := referenceTransitionOwner{owner.Transaction, owner.RequestKey, owner.Invocation, owner.ResponseRead, owner.RawSelector, owner.RawDigest}
					first := journalBytes(o, referenceTransitionEvent{"QUERY_BEGIN", -1})
					_ = os.WriteFile(filepath.Join(dir, referenceTransitionJournalSelector(first)), []byte("collision"), 0600)
				} else if len(entries) > 0 {
					_ = os.Remove(filepath.Join(dir, entries[len(entries)-1].Name()))
				}
			}
			eval, final, err := runPrivateAttachedReferences(root, key, raw, privateIdentity(key, raw), trace)
			if seen < tc.occurrence || err == nil || eval != nil || final != nil {
				t.Fatalf("false success: seen=%d err=%v eval=%+v bytes=%q", seen, err, eval, final)
			}
		})
	}
}
