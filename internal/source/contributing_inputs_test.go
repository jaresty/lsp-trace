package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func recorderFixture(t *testing.T) (*InputRecorder, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "input"), []byte("original\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := NewInputRecorder(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r, root
}
func TestInputRecorderAccounting(t *testing.T) {
	r, _ := recorderFixture(t)
	_, id, err := r.ReadInput("input", InputSource)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.BindContribution("kept", id); err != nil {
		t.Fatal(err)
	}
	e, err := r.Evidence([]string{"omitted", "kept"})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Contributions) != 2 || len(e.IncompleteReasons) != 2 || !strings.Contains(strings.Join(e.IncompleteReasons, ";"), "omitted") || !strings.Contains(strings.Join(e.IncompleteReasons, ";"), "unobserved dependencies") {
		t.Fatal("ASSERT_INPUT_ACCOUNTING: missing explicit omitted contribution or unknown dependencies")
	}
}
func TestInputRecorderClassifiedBytes(t *testing.T) {
	r, root := recorderFixture(t)
	ids := map[string]bool{}
	for _, class := range []InputClass{InputSource, InputConfiguration, InputDeclaration, InputGeneratedMapping} {
		b, id, err := r.ReadInput("input", class)
		if err != nil {
			t.Fatal(err)
		}
		if id == "" || ids[id] || string(b) != "original\r\n" {
			t.Fatal("ASSERT_INPUT_CLASS_BYTES: classification must bind actual bytes")
		}
		ids[id] = true
	}
	if err := os.WriteFile(filepath.Join(root, "input"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	_, id, err := r.ReadInput("input", InputSource)
	if err != nil {
		t.Fatal(err)
	}
	if ids[id] {
		t.Fatal("ASSERT_INPUT_CHANGED_BYTES: changed bytes reused receipt")
	}
	e, err := r.Evidence(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Inputs) != 5 {
		t.Fatal("ASSERT_INPUT_CHANGED_BYTES: acquisition versions lost")
	}
	for _, in := range e.Inputs {
		_, content, canonical, err := CanonicalizeReceipt(in.Receipt.Item, Acquisition{Status: in.Receipt.Status, Provenance: in.Receipt.Provenance, Failure: in.Receipt.Failure}, in.Content)
		if err != nil || in.Receipt.Provenance.Mechanism != "bounded-input/"+string(in.Class) || !bytes.Equal(content, in.Content) || !bytes.Equal(canonical, in.CanonicalReceipt) || in.ID != fmt.Sprintf("sha256:%x", sha256.Sum256(canonical)) {
			t.Fatal("ASSERT_INPUT_CANONICAL: historical receipt construction changed")
		}
	}
}
func TestInputRecorderScope(t *testing.T) {
	r, root := recorderFixture(t)
	if err := os.Mkdir(filepath.Join(root, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../input", "/etc/passwd", "a/../input", ".", "..", "", "a\\b"} {
		if _, _, err := r.ReadInput(name, InputSource); err == nil {
			t.Fatalf("ASSERT_INPUT_SCOPE: accepted %q", name)
		}
	}
	if _, _, err := r.ReadInput("input", InputClass("UNKNOWN")); err == nil {
		t.Fatal("ASSERT_INPUT_CLASS: unknown classification accepted")
	}
}
func TestInputRecorderSymlink(t *testing.T) {
	r, root := recorderFixture(t)
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	b, id, err := r.ReadInput("escape", InputSource)
	if err == nil || b != nil || id == "" {
		t.Fatal("ASSERT_INPUT_SYMLINK: escape must retain failure without bytes")
	}
	e, err := r.Evidence(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Inputs) != 1 || e.Inputs[0].Receipt.Status != Unreadable {
		t.Fatal("ASSERT_INPUT_SYMLINK: failure not receipted")
	}
}
func TestInputRecorderFailure(t *testing.T) {
	r, _ := recorderFixture(t)
	b, id, err := r.ReadInput("missing", InputConfiguration)
	if err == nil || b != nil || id == "" {
		t.Fatal("ASSERT_INPUT_FAILURE: missing input must retain failure receipt")
	}
	if err = r.BindContribution("kept", id); err != nil {
		t.Fatal(err)
	}
	e, err := r.Evidence([]string{"kept"})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Inputs) != 1 || e.Inputs[0].Receipt.Status != Unreadable || len(e.IncompleteReasons) != 2 {
		t.Fatal("ASSERT_INPUT_FAILURE: failure incorrectly complete")
	}
}
func TestInputRecorderImmutable(t *testing.T) {
	r, _ := recorderFixture(t)
	b, id, err := r.ReadInput("input", InputSource)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.BindContribution("kept", id); err != nil {
		t.Fatal(err)
	}
	e, err := r.Evidence([]string{"kept"})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Inputs) != 1 {
		t.Fatal("ASSERT_INPUT_IMMUTABLE: missing observation")
	}
	before, _ := json.Marshal(e)
	b[0] = 'X'
	e.Inputs[0].Content[0] = 'Y'
	e.Inputs[0].CanonicalReceipt[0] = 'Z'
	e.Inputs[0].Receipt.ContentIdentity.Digest = "forged"
	e.Contributions[0].ReceiptIDs[0] = "forged"
	after, err := r.Evidence([]string{"kept"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(after)
	if !bytes.Equal(before, encoded) {
		t.Fatal("ASSERT_INPUT_IMMUTABLE: caller mutated retained evidence")
	}
}
func TestInputRecorderDuplicates(t *testing.T) {
	r, _ := recorderFixture(t)
	_, id, err := r.ReadInput("input", InputSource)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		refs []string
	}{{"", []string{id}}, {"none", nil}, {"unknown", []string{"sha256:invented"}}, {"duplicate", []string{id, id}}} {
		if err := r.BindContribution(tc.id, tc.refs...); err == nil {
			t.Fatalf("ASSERT_INPUT_BINDING: accepted %q", tc.id)
		}
	}
	if err = r.BindContribution("kept", id); err != nil {
		t.Fatal(err)
	}
	if err = r.BindContribution("kept", id); err == nil {
		t.Fatal("ASSERT_INPUT_DUPLICATE: contribution overwritten")
	}
	if _, err = r.Evidence([]string{"kept", "kept"}); err == nil {
		t.Fatal("ASSERT_INPUT_DUPLICATE: duplicate expected contribution accepted")
	}
}
func TestInputRecorderOrder(t *testing.T) {
	makeEvidence := func(reverse bool) InputEvidence {
		r, _ := recorderFixture(t)
		classes := []InputClass{InputSource, InputConfiguration}
		if reverse {
			classes[0], classes[1] = classes[1], classes[0]
		}
		var refs []string
		for _, c := range classes {
			_, id, e := r.ReadInput("input", c)
			if e != nil {
				t.Fatal(e)
			}
			refs = append(refs, id)
		}
		for _, id := range []string{"a", "z"} {
			if e := r.BindContribution(id, refs...); e != nil {
				t.Fatal(e)
			}
		}
		ids := []string{"a", "z"}
		if reverse {
			ids[0], ids[1] = ids[1], ids[0]
		}
		e, err := r.Evidence(ids)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	a, b := makeEvidence(false), makeEvidence(true)
	if len(a.Inputs) != 2 || !reflect.DeepEqual(a, b) {
		t.Fatal("ASSERT_INPUT_ORDER: equivalent acquisition order changed evidence")
	}
}
