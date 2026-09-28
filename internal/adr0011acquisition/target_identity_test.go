package adr0011acquisition

import (
	"strings"
	"testing"

	"lsp-trace/internal/adr0011querytarget"
)

const targetIdentityRaw = `[{"name":"Alpha","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":3,"character":0}},"selectionRange":{"start":{"line":1,"character":2},"end":{"line":2,"character":0}}}]`

func targetIdentityFixture() (targetResultRef, targetResultRef, targetResultRef, adr0011querytarget.Query) {
	ref := func(role, name, digit string) targetResultRef {
		digest := "sha256:" + strings.Repeat(digit, 64)
		return targetResultRef{role, "adr0011-references-issuance-v1-" + name + "-" + strings.Repeat(digit, 64) + ".json", digest}
	}
	return ref(sourceIdentityRole, "source", "a"), ref(revisionIdentityRole, "revision", "b"), ref(targetResultRole, "target-result", "c"), adr0011querytarget.Query{OccurrenceID: "sha256:" + strings.Repeat("1", 64), URI: "file:///a.go", Encoding: "utf-16", DocumentVersion: "1", SourceDigest: "sha256:" + strings.Repeat("d", 64), SessionID: "session", Generation: 1, Line: 1, Character: 3}
}
func TestTargetIdentityIndependentGoldenAndMutations(t *testing.T) {
	source, revision, result, q := targetIdentityFixture()
	symbol, target, err := targetIdentityV1(source, revision, result, q, []byte(targetIdentityRaw))
	// These literals were computed by an independent Python struct.pack('>Q') +
	// json.dumps(sort_keys=True,separators=(',',':')) + hashlib.sha256 fixture.
	if err != nil || symbol != "sha256:5243cbed04932ac0efdf67b7b7c173b59f21ecba28e8708019649eecec76dbb6" || target != "sha256:d8ec19f7307f6d36aca0951675f73dfc42a26ef37e411fc4ead4b50eaa3574b5" {
		t.Fatalf("independent golden: %s %s %v", symbol, target, err)
	}
	q.OccurrenceID = "sha256:" + strings.Repeat("2", 64)
	sameSymbol, differentTarget, err := targetIdentityV1(source, revision, result, q, []byte(targetIdentityRaw))
	if err != nil || sameSymbol != symbol || differentTarget == target {
		t.Fatal("equal-location occurrence identity collapsed")
	}
	q.OccurrenceID = "sha256:" + strings.Repeat("1", 64)
	if _, _, err = targetIdentityV1(revision, source, result, q, []byte(targetIdentityRaw)); err == nil {
		t.Fatal("swapped typed refs accepted")
	}
	source.Selector = "../" + source.Selector
	if _, _, err = targetIdentityV1(source, revision, result, q, []byte(targetIdentityRaw)); err == nil {
		t.Fatal("non-derived selector accepted")
	}
	source, _, _, _ = targetIdentityFixture()
	source.Digest = "sha256:" + strings.Repeat("A", 64)
	if _, _, err = targetIdentityV1(source, revision, result, q, []byte(targetIdentityRaw)); err == nil {
		t.Fatal("noncanonical digest accepted")
	}
	source, _, _, _ = targetIdentityFixture()
	for _, id := range []string{string([]byte{0xff}), "e\u0301", "occ-1"} {
		q.OccurrenceID = id
		if _, _, err = targetIdentityV1(source, revision, result, q, []byte(targetIdentityRaw)); err == nil {
			t.Fatalf("invalid UTF8/NFC accepted: %q", id)
		}
	}
}
func TestTargetIdentityCanonicalRefAndPosition(t *testing.T) {
	source, _, _, _ := targetIdentityFixture()
	got := string(targetIdentityRefJSON(source))
	want := `{"digest":"` + source.Digest + `","schema_version":"` + source.SchemaVersion + `","selector":"` + source.Selector + `"}`
	if got != want || got == `{"selector":"`+source.Selector+`","schema_version":"`+source.SchemaVersion+`","digest":"`+source.Digest+`"}` {
		t.Fatal("reference key permutation")
	}
	p := adr0011querytarget.Position{Line: ^uint32(0), Character: ^uint32(0)}
	r := adr0011querytarget.Range{Start: p, End: p}
	if string(targetIdentityRangeJSON(r)) != `{"end":{"character":4294967295,"line":4294967295},"start":{"character":4294967295,"line":4294967295}}` {
		t.Fatal("uint32 position overflow or key order")
	}
}

func TestTargetIdentityLPBoundaryAndOrder(t *testing.T) {
	a := targetIdentityHash([]byte("ab"), []byte("c"))
	if a == targetIdentityHash([]byte("a"), []byte("bc")) || a == targetIdentityHash([]byte("c"), []byte("ab")) {
		t.Fatal("LP boundary or order ignored")
	}
	// Eight-byte big-endian prefix for a three-byte field.
	if got := targetIdentityHash([]byte("abc")); got != "sha256:c3494ca1a2cf8eeb8a11ded316fb55b83c3bbbedb6313cd50415251e5d09e12f" {
		t.Fatalf("LP prefix: %s", got)
	}
}
