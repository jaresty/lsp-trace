package adr0011closure

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestInertClosureReplayRequiresPredecessors(t *testing.T) {
	root := rootForTest(t)
	raw := []byte(`{"owner":"owner-one","kind":"source"}`)
	sum := sha256.Sum256(raw)
	dep := reference{Role: "source", Selector: "source.json", Digest: "sha256:" + hex.EncodeToString(sum[:])}
	ref, pending, err := publish(root, "owner-one", "proposal", []reference{dep})
	if err != nil || pending != nil {
		t.Fatalf("ASSERT_CLOSURE_FIXTURE: %v %+v", err, pending)
	}
	reads := 0
	resolve := func(selector string, limit int) ([]byte, error) {
		reads++
		if selector != dep.Selector || limit != maxPredecessorBytes {
			return nil, errors.New("wrong bounded read")
		}
		return append([]byte(nil), raw...), nil
	}
	verify := map[string]predecessorVerifier{"source": func(got []byte, owner string) error {
		if owner != "owner-one" || string(got) != string(raw) {
			return errors.New("wrong source")
		}
		return nil
	}}
	if err := replayWithPredecessors(root, ref, "owner-one", "proposal", []reference{dep}, resolve, verify); err != nil || reads != 1 {
		t.Fatalf("ASSERT_CLOSURE_BOUND_PREDECESSORS: %v reads=%d", err, reads)
	}
	reads = 0
	bad := ref
	bad.Digest = dep.Digest
	if err := replayWithPredecessors(root, bad, "owner-one", "proposal", []reference{dep}, resolve, verify); err == nil || reads != 0 {
		t.Fatalf("ASSERT_CLOSURE_REPLAY_BEFORE_RESOLUTION: %v reads=%d", err, reads)
	}
	if err := replayWithPredecessors(root, ref, "owner-one", "proposal", []reference{dep}, nil, verify); err == nil {
		t.Fatal("ASSERT_CLOSURE_NO_UNRESOLVED_PREDECESSORS")
	}
}

func TestPredecessorIndependentResolution(t *testing.T) {
	raw := []byte(`{"owner":"owner-one","kind":"source","value":"trusted"}`)
	sum := sha256.Sum256(raw)
	ref := reference{Role: "source", Selector: "source.json", Digest: "sha256:" + hex.EncodeToString(sum[:])}
	reads, checks := 0, 0
	resolver := func(selector string, limit int) ([]byte, error) {
		reads++
		if selector != ref.Selector || limit != maxPredecessorBytes {
			t.Fatal("ASSERT_EXACT_BOUNDED_SELECTOR")
		}
		return append([]byte(nil), raw...), nil
	}
	verifier := func(got []byte, owner string) error {
		checks++
		if owner != "owner-one" || string(got) != string(raw) {
			return errors.New("semantic or owner mismatch")
		}
		return nil
	}
	verify := map[string]predecessorVerifier{"source": verifier}
	if err := resolvePredecessors("owner-one", []reference{ref}, resolver, verify); err != nil || reads != 1 || checks != 1 {
		t.Fatalf("ASSERT_INDEPENDENT_READ_AND_VERIFICATION: %v %d %d", err, reads, checks)
	}
	cases := []struct {
		name      string
		refs      []reference
		owner     string
		read      predecessorResolver
		verifiers map[string]predecessorVerifier
	}{
		{"missing", []reference{ref}, "owner-one", func(string, int) ([]byte, error) { return nil, errors.New("missing") }, verify},
		{"changed", []reference{ref}, "owner-one", func(string, int) ([]byte, error) { return []byte("changed"), nil }, verify},
		{"oversize", []reference{ref}, "owner-one", func(string, int) ([]byte, error) { return make([]byte, maxPredecessorBytes+1), nil }, verify},
		{"duplicate", []reference{ref, ref}, "owner-one", resolver, verify},
		{"selector-duplicate", []reference{ref, {Role: "target", Selector: ref.Selector, Digest: ref.Digest}}, "owner-one", resolver, verify},
		{"role-substitution", []reference{{Role: "target", Selector: ref.Selector, Digest: ref.Digest}}, "owner-one", resolver, verify},
		{"missing-verifier", []reference{ref}, "owner-one", resolver, nil},
		{"missing-resolver", []reference{ref}, "owner-one", nil, verify},
		{"cross-owner", []reference{ref}, "owner-two", resolver, verify},
		{"rehash-impostor", nil, "owner-one", nil, nil},
	}
	impostor := []byte(`{"owner":"owner-two","kind":"source","value":"trusted"}`)
	impostorSum := sha256.Sum256(impostor)
	cases[len(cases)-1].refs = []reference{{Role: "source", Selector: ref.Selector, Digest: "sha256:" + hex.EncodeToString(impostorSum[:])}}
	cases[len(cases)-1].read = func(string, int) ([]byte, error) { return impostor, nil }
	cases[len(cases)-1].verifiers = map[string]predecessorVerifier{"source": func(got []byte, owner string) error {
		if strings.Contains(string(got), `"owner":"`+owner+`"`) {
			return nil
		}
		return errors.New("owner mismatch")
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := resolvePredecessors(tc.owner, tc.refs, tc.read, tc.verifiers); err == nil {
				t.Fatal("ASSERT_PREDECESSOR_REJECTION")
			}
		})
	}
}
