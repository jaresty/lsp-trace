package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/adr0011c18"
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

type c18MethodResultObservation struct {
	coarse []adr0011c18.Point
	events []adr0011c18.Event
}

func c18RunAttachedReferences(t *testing.T, ctx context.Context, raw json.RawMessage) (c18MethodResultObservation, *ReferenceEvaluation, []byte, error) {
	t.Helper()
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	root, _ := journalRoot(t)
	var observed c18MethodResultObservation
	ctx = adr0011c18.WithObserver(ctx, func(point adr0011c18.Point) { observed.coarse = append(observed.coarse, point) })
	ctx = adr0011c18.WithEventObserver(ctx, func(event adr0011c18.Event) { observed.events = append(observed.events, event) })
	ctx, receipt := adr0011c18.BeginRuntime(ctx)
	for _, point := range []adr0011c18.Point{
		adr0011c18.PointPreflight, adr0011c18.PointDeadline, adr0011c18.PointHeaderFrame,
		adr0011c18.PointCumulativeWire, adr0011c18.PointMessageCount, adr0011c18.PointDecodedMessage,
		adr0011c18.PointRawResult, adr0011c18.PointSourceBuffer,
	} {
		if receipt == nil || !receipt.ReachRuntime(point) {
			t.Fatalf("C18_TEST_SETUP: direct method-result runtime activation refused %v", point)
		}
	}
	if !receipt.SealRuntime() {
		t.Fatal("C18_TEST_SETUP: direct method-result runtime seal refused")
	}
	observed = c18MethodResultObservation{}
	eval, final, err := RunPrivateAttachedReferencesContext(ctx, root, key, raw, privateIdentity(key, raw))
	return observed, eval, final, err
}

func TestADR0011C18AttachedReferencesOrdersObjectEventRetentionReadback(t *testing.T) {
	raw := json.RawMessage("[" + attachedLocation + "]")
	observed, eval, final, err := c18RunAttachedReferences(t, context.Background(), raw)
	if err != nil || eval == nil || len(final) == 0 {
		t.Fatalf("BLOCKED_NOT_RED: real method-result path failed: err=%v eval=%+v bytes=%d", err, eval, len(final))
	}
	coarse := []adr0011c18.Point{adr0011c18.PointMethodResultEntered, adr0011c18.PointMethodResultReturned}
	if !reflect.DeepEqual(observed.coarse, coarse) {
		t.Fatalf("BLOCKED_NOT_RED: inert observer order changed: %v", observed.coarse)
	}
	want := []adr0011c18.Event{
		{Point: adr0011c18.PointObjectEvent},
		{Point: adr0011c18.PointRetention},
		{Point: adr0011c18.PointReadback},
	}
	if !reflect.DeepEqual(observed.events, want) {
		t.Fatalf("C18_METHODRESULT_SUFFIX_ORDER: omission, duplication, or order mismatch\nwant=%v\n got=%v", want, observed.events)
	}
}

func TestADR0011C18AttachedReferencesReadbackFailureIsFailClosed(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	raw := json.RawMessage("[" + attachedLocation + "]")
	root, dir := journalRoot(t)
	seenReceipt := 0
	trace := func(event publication.BoundFileTraceEvent) {
		if event.Stage != "RECEIPT" {
			return
		}
		seenReceipt++
		if seenReceipt == 2 {
			entries, _ := os.ReadDir(dir)
			if len(entries) > 0 {
				_ = os.Remove(filepath.Join(dir, entries[len(entries)-1].Name()))
			}
		}
	}
	eval, final, err := runPrivateAttachedReferences(root, key, raw, privateIdentity(key, raw), trace)
	if seenReceipt < 2 || err == nil || eval != nil || final != nil {
		t.Fatalf("C18_METHODRESULT_READBACK_FAIL_CLOSED: seen=%d err=%v eval=%+v bytes=%q", seenReceipt, err, eval, final)
	}
}

func TestADR0011C18AttachedReferencesObserverDefaultOffPreservesBytes(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	raw := json.RawMessage("[" + attachedLocation + "]")
	plainRoot, _ := journalRoot(t)
	plainEval, plainFinal, plainErr := RunPrivateAttachedReferences(plainRoot, key, raw, privateIdentity(key, raw))
	observed, observedEval, observedFinal, observedErr := c18RunAttachedReferences(t, context.Background(), raw)
	if plainErr != nil || observedErr != nil || !reflect.DeepEqual(plainEval, observedEval) || !bytes.Equal(plainFinal, observedFinal) {
		t.Fatalf("C18_METHODRESULT_DEFAULT_OFF: observer changed evaluation/bytes/error: plain=%v observed=%v coarse=%v events=%v", plainErr, observedErr, observed.coarse, observed.events)
	}
}
