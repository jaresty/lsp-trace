package adr0011generic

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// These fixture identities are selected from the accepted offline SOURCE vector,
// never inferred from the claimant graph being tested.
func b3Fixture(method string) (syntheticGraphSelection, []syntheticGraphRole, []claimedSourceOriginal, []HeldSourceOriginal) {
	held, graph := b2Fixture(method, false, 1)
	id := syntheticTransactionIdentity{session: "s", generation: 1, transaction: "tx-1"}
	held.identity = id
	tx := b3TestTX(id)
	querySelector, targetSelector := "sha256:f7a84828be3c950e51e2acd19a0f1d629757a22cdfd3a70963a6a34479b8c2a9", "sha256:0b3742c050df5b536136cf71027c05ea78c3069e045146836ac1a3ad466b0548"
	if method == "textDocument/definition" {
		querySelector, targetSelector = "sha256:f206d3e24e2bc0ea4e90ce00867f8e28bcc9179207b4f4025a92bef6bf686ed1", "sha256:f72b18f021a1ad3f778dd739dfbb38f3aafb39991cbb7eb91120f33904107ff5"
	}
	originalQuery, originalTarget := held.querySourceSelector, graph[len(graph)-1].selector
	replace := map[string]string{originalQuery: querySelector, originalTarget: targetSelector}
	held.querySourceSelector = querySelector
	for i := range held.roles {
		if s, ok := replace[held.roles[i].selector]; ok {
			held.roles[i].selector = s
		}
	}
	for i := range graph {
		graph[i].identity = id
		if s, ok := replace[graph[i].selector]; ok {
			graph[i].selector = s
		}
		for j := range graph[i].predecessors {
			if s, ok := replace[graph[i].predecessors[j].selector]; ok {
				graph[i].predecessors[j].selector = s
			}
		}
	}
	sources := []HeldSourceOriginal{
		{TransactionIdentity: tx, SourceSelector: querySelector, URIBytes: []byte("file:///query"), VersionBytes: []byte("buffer:v1"), ContentBytes: []byte("alpha\n"), CustodyKind: SourceOwnerBuffer},
		{TransactionIdentity: tx, SourceSelector: targetSelector, URIBytes: []byte("file:///target"), VersionBytes: []byte{}, ContentBytes: []byte{}, CustodyKind: SourceManagedVirtual},
	}
	claims := make([]claimedSourceOriginal, len(sources))
	for i, source := range sources {
		claims[i] = claimedSourceOriginal{TransactionIdentity: tx, SourceSelector: source.SourceSelector, URIBytes: bytes.Clone(source.URIBytes), VersionBytes: bytes.Clone(source.VersionBytes), ContentBytes: bytes.Clone(source.ContentBytes), ContentLength: len(source.ContentBytes), ContentSHA256: "sha256:" + digest(source.ContentBytes), CustodyKind: source.CustodyKind}
	}
	return held, graph, claims, sources
}
func b3TestTX(id syntheticTransactionIdentity) string {
	var preimage []byte
	for _, field := range []string{"ADR0011-GENERIC-TRANSACTION/1", id.session, fmt.Sprint(id.generation), id.transaction} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		preimage = append(preimage, length[:]...)
		preimage = append(preimage, field...)
	}
	hashed := sha256.Sum256(preimage)
	return fmt.Sprintf("sha256:%x", hashed)
}

func TestB3SameGraphSourceVersionAndContentContrast(t *testing.T) {
	for _, method := range []string{"textDocument/references", "textDocument/definition"} {
		held, graph, claimed, sources := b3Fixture(method)
		if err := validateSyntheticGraph(graph, held); err != nil {
			t.Fatalf("B2 fixture failed: %v", err)
		}
		if err := validateSyntheticSources(graph, held, claimed, sources); err != nil {
			t.Fatalf("B3 matching held source rejected: %v", err)
		}
		for _, tc := range []struct {
			name   string
			mutate func([]HeldSourceOriginal)
		}{
			{"version", func(s []HeldSourceOriginal) { s[0].VersionBytes = []byte("buffer:v2") }},
			{"content byte", func(s []HeldSourceOriginal) { s[0].ContentBytes = []byte("alphA\n") }},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				changed := append([]HeldSourceOriginal(nil), sources...)
				tc.mutate(changed)
				if err := validateSyntheticGraph(graph, held); err != nil {
					t.Fatalf("B2 changed with same graph: %v", err)
				}
				if err := validateSyntheticSources(graph, held, claimed, changed); err == nil {
					t.Fatal("B3 accepted changed independently held source")
				}
			})
		}
	}
}

func TestB3MissingHeldSourceFailsClosed(t *testing.T) {
	held, graph, claimed, _ := b3Fixture("textDocument/references")
	if err := validateSyntheticSources(graph, held, claimed, nil); err == nil || !strings.Contains(err.Error(), "held") {
		t.Fatalf("missing held SOURCE accepted or misclassified: %v", err)
	}
}

func TestB3HeldAndClaimedSourceSubstitutions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*syntheticGraphSelection, *[]claimedSourceOriginal, *[]HeldSourceOriginal)
	}{
		{"missing held target", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) { *s = (*s)[:1] }},
		{"extra held source", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			*s = append(*s, (*s)[1])
		}},
		{"duplicate held selector", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].SourceSelector = (*s)[0].SourceSelector
		}},
		{"query source not first", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0], (*s)[1] = (*s)[1], (*s)[0]
		}},
		{"held cross transaction", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].TransactionIdentity = b3TestTX(syntheticTransactionIdentity{session: "s", generation: 1, transaction: "foreign"})
		}},
		{"held selector substitution", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].SourceSelector = "sha256:" + digest([]byte("wrong"))
		}},
		{"held URI byte", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].URIBytes = []byte("file:///targeT")
		}},
		{"held custody", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].CustodyKind = SourceImmutableSnapshot
		}},
		{"held unknown custody", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].CustodyKind = "OTHER"
		}},
		{"query versus target substitution", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0].URIBytes, (*s)[1].URIBytes = (*s)[1].URIBytes, (*s)[0].URIBytes
		}},
		{"missing claimant source", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) { *c = (*c)[:1] }},
		{"extra claimant source", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			*c = append(*c, (*c)[1])
		}},
		{"duplicate claimant selector", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[1].SourceSelector = (*c)[0].SourceSelector
		}},
		{"claimant selector swap", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].SourceSelector, (*c)[1].SourceSelector = (*c)[1].SourceSelector, (*c)[0].SourceSelector
		}},
		{"claimant transaction", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].TransactionIdentity = "sha256:" + digest([]byte("foreign"))
		}},
		{"claimant URI", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].URIBytes = []byte("file:///querY")
		}},
		{"claimant version", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].VersionBytes = []byte("buffer:v2")
		}},
		{"claimant content", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].ContentBytes = []byte("alphA\n")
		}},
		{"claimant length", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].ContentLength++
		}},
		{"claimant digest", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].ContentSHA256 = "sha256:" + digest([]byte("other"))
		}},
		{"claimant custody", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].CustodyKind = SourceManagedVirtual
		}},
		{"absent held version", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].VersionBytes = nil
		}},
		{"absent claimant version", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[1].VersionBytes = nil
		}},
		{"absent held content", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[1].ContentBytes = nil
		}},
		{"absent claimant content", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[1].ContentBytes = nil
		}},
		{"claimant verified flag cannot override byte mismatch", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].ContentBytes = []byte("alphA\n")
			(*c)[0].Verified = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			held, graph, claimed, sources := b3Fixture("textDocument/references")
			tc.change(&held, &claimed, &sources)
			if err := validateSyntheticGraph(graph, held); err != nil {
				t.Fatalf("B2 graph changed by SOURCE-only mutation: %v", err)
			}
			if err := validateSyntheticSources(graph, held, claimed, sources); err == nil {
				t.Fatal("B3 accepted SOURCE substitution")
			}
		})
	}
}

func TestB3EligibilityToCopyPresentEmptyMutationFailsClosed(t *testing.T) {
	for _, field := range []string{"version", "content"} {
		t.Run(field, func(t *testing.T) {
			held, graph, claims, sources := b3Fixture("textDocument/references")
			if err := validateSyntheticGraph(graph, held); err != nil {
				t.Fatal(err)
			}
			hook := func() {
				if field == "version" {
					claims[1].VersionBytes = nil
				} else {
					claims[1].ContentBytes = nil
				}
			}
			if err := validateSyntheticSourcesAtSnapshot(graph, held, claims, sources, digest, hook); err == nil {
				t.Fatalf("post-eligibility absent claimant %s passed B3", field)
			}
		})
	}
}

func TestB3InjectedDigestCollisionStillComparesFullBytes(t *testing.T) {
	held, graph, claimed, sources := b3Fixture("textDocument/definition")
	constant := func([]byte) string { return strings.Repeat("0", 64) }
	for i := range claimed {
		claimed[i].ContentSHA256 = "sha256:" + constant(nil)
	}
	if err := validateSyntheticSourcesWithDigest(graph, held, claimed, sources, constant); err != nil {
		t.Fatalf("baseline stubbed digest rejected: %v", err)
	}
	claimed[0].ContentBytes = []byte("alphA\n") // same length and same injected digest
	if err := validateSyntheticSourcesWithDigest(graph, held, claimed, sources, constant); err == nil || !strings.Contains(err.Error(), "complete content bytes") {
		t.Fatalf("digest collision bypassed byte check: %v", err)
	}
}

func TestB3SourceEligibilityBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*syntheticGraphSelection, *[]claimedSourceOriginal, *[]HeldSourceOriginal)
	}{
		{"session 1025", func(h *syntheticGraphSelection, _ *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			h.identity.session = strings.Repeat("s", 1025)
		}},
		{"transaction 1025", func(h *syntheticGraphSelection, _ *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			h.identity.transaction = strings.Repeat("t", 1025)
		}},
		{"session invalid UTF8", func(h *syntheticGraphSelection, _ *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			h.identity.session = "\xff"
		}},
		{"URI 16385", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0].URIBytes = []byte(strings.Repeat("u", 16385))
		}},
		{"URI invalid UTF8", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0].URIBytes = []byte{0xff}
		}},
		{"version 4097", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0].VersionBytes = []byte(strings.Repeat("v", 4097))
		}},
		{"version invalid UTF8", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0].VersionBytes = []byte{0xff}
		}},
		{"content 4194305", func(_ *syntheticGraphSelection, _ *[]claimedSourceOriginal, s *[]HeldSourceOriginal) {
			(*s)[0].ContentBytes = bytes.Repeat([]byte("x"), 4194305)
		}},
		{"claimed content 4194305", func(_ *syntheticGraphSelection, c *[]claimedSourceOriginal, _ *[]HeldSourceOriginal) {
			(*c)[0].ContentBytes = bytes.Repeat([]byte("x"), 4194305)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			held, graph, claims, sources := b3Fixture("textDocument/references")
			tc.change(&held, &claims, &sources)
			if err := validateSyntheticSources(graph, held, claims, sources); err == nil {
				t.Fatal("B3 accepted over-bound or malformed source")
			}
		})
	}
	held, graph, claims, sources := b3Fixture("textDocument/references")
	if err := validateSyntheticSources(graph, held, claims, sources); err != nil {
		t.Fatalf("present empty version/content rejected: %v", err)
	}
	sources = make([]HeldSourceOriginal, 257)
	if err := validateSyntheticSources(graph, held, claims, sources); err == nil {
		t.Fatal("257 held sources accepted")
	}
}

// b3Many uses independently constructed held sources, replacing the synthetic
// B2 fixture's SOURCE selectors only after source-specific computation. The
// single-target fixture above additionally pins two exact cross-language vectors.
func b3Many(t *testing.T, targets int) (syntheticGraphSelection, []syntheticGraphRole, []claimedSourceOriginal, []HeldSourceOriginal) {
	t.Helper()
	method := "textDocument/references"
	held, graph := b2Fixture(method, false, targets)
	id := syntheticTransactionIdentity{session: "s", generation: 1, transaction: "tx-1"}
	held.identity = id
	tx := b3TestTX(id)
	var sources []HeldSourceOriginal
	replaced := make(map[string]string)
	sourceIndex := 0
	for _, ref := range held.roles {
		if ref.role != "SOURCE" {
			continue
		}
		source := HeldSourceOriginal{TransactionIdentity: tx, CustodyKind: SourceManagedVirtual, VersionBytes: []byte{}, ContentBytes: []byte{}}
		if sourceIndex == 0 {
			source.URIBytes = []byte("file:///query")
			source.VersionBytes = []byte("buffer:v1")
			source.ContentBytes = []byte("alpha\n")
			source.CustodyKind = SourceOwnerBuffer
		} else if sourceIndex == 1 {
			source.URIBytes = []byte("file:///target")
		} else {
			source.URIBytes = []byte(fmt.Sprintf("file:///target/%03d", sourceIndex))
		}
		selector, err := heldSourceSelector(method, id, source, tx)
		if err != nil {
			t.Fatal(err)
		}
		source.SourceSelector = selector
		replaced[ref.selector] = selector
		sources = append(sources, source)
		sourceIndex++
	}
	if sources[0].SourceSelector != "sha256:f7a84828be3c950e51e2acd19a0f1d629757a22cdfd3a70963a6a34479b8c2a9" {
		t.Fatal("query selector disagrees with offline original")
	}
	held.querySourceSelector = sources[0].SourceSelector
	sort.Slice(sources[1:], func(i, j int) bool { return sources[1+i].SourceSelector < sources[1+j].SourceSelector })
	for i := range held.roles {
		if selector, ok := replaced[held.roles[i].selector]; ok {
			held.roles[i].selector = selector
		}
	}
	for i := range graph {
		graph[i].identity = id
		if selector, ok := replaced[graph[i].selector]; ok {
			graph[i].selector = selector
		}
		for j := range graph[i].predecessors {
			if selector, ok := replaced[graph[i].predecessors[j].selector]; ok {
				graph[i].predecessors[j].selector = selector
			}
		}
	}
	claims := make([]claimedSourceOriginal, len(sources))
	for i, s := range sources {
		claims[i] = claimedSourceOriginal{TransactionIdentity: tx, SourceSelector: s.SourceSelector, URIBytes: bytes.Clone(s.URIBytes), VersionBytes: bytes.Clone(s.VersionBytes), ContentBytes: bytes.Clone(s.ContentBytes), ContentLength: len(s.ContentBytes), ContentSHA256: "sha256:" + digest(s.ContentBytes), CustodyKind: s.CustodyKind}
	}
	return held, graph, claims, sources
}

func TestB3TargetOrderAnd256SourceBoundary(t *testing.T) {
	held, graph, claims, sources := b3Many(t, 2)
	if err := validateSyntheticSources(graph, held, claims, sources); err != nil {
		t.Fatalf("two targets sorted rejected: %v", err)
	}
	sources[1], sources[2] = sources[2], sources[1]
	if err := validateSyntheticSources(graph, held, claims, sources); err == nil || !strings.Contains(err.Error(), "order") {
		t.Fatalf("misordered targets accepted or misclassified: %v", err)
	}
	held, graph, claims, sources = b3Many(t, 255)
	if len(sources) != 256 {
		t.Fatal("boundary fixture did not select 256 distinct sources")
	}
	if err := validateSyntheticGraph(graph, held); err != nil {
		t.Fatalf("256 SOURCE B2 rejected: %v", err)
	}
	if err := validateSyntheticSources(graph, held, claims, sources); err != nil {
		t.Fatalf("256 SOURCE B3 rejected: %v", err)
	}
}

func TestB3InclusiveSourceByteCaps(t *testing.T) {
	id := syntheticTransactionIdentity{session: strings.Repeat("s", 1024), generation: 1, transaction: strings.Repeat("t", 1024)}
	if _, err := heldSourceTX(id); err != nil {
		t.Fatalf("identity inclusive byte caps rejected: %v", err)
	}
	uri := []byte(strings.Repeat("u", 16384))
	version := []byte(strings.Repeat("v", 4096))
	content := bytes.Repeat([]byte("x"), 4194304)
	if err := sourceByteBounds(uri, version, content); err != nil {
		t.Fatalf("SOURCE inclusive byte caps rejected: %v", err)
	}
}

func TestB3SnapshotsDetachAllMutableSourceSlices(t *testing.T) {
	held, graph, claims, sources := b3Fixture("textDocument/references")
	copiedGraph, copiedHeld := snapshotB3Graph(graph, held)
	copiedClaims, copiedSources := snapshotB3Sources(claims, sources)
	original := copiedSources[0].ContentBytes[0]
	sources[0].ContentBytes[0] = 'X'
	claims[0].ContentBytes[0] = 'Y'
	graph[b2Index(graph, "QUERY")].predecessors[0].selector = "foreign"
	held.roles[0].selector = "foreign"
	if copiedSources[0].ContentBytes[0] != original || copiedClaims[0].ContentBytes[0] == 'Y' || copiedGraph[b2Index(copiedGraph, "QUERY")].predecessors[0].selector == "foreign" || copiedHeld.roles[0].selector == "foreign" {
		t.Fatal("B3 snapshot retained mutable aliases")
	}
}
