package adr0011closure

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func rootForTest(t *testing.T) *publication.Root {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}
func depsForTest() []reference {
	return []reference{
		{"source-" + strings.Repeat("a", 64) + ".json", "sha256:" + strings.Repeat("a", 64), "source"},
		{"target-" + strings.Repeat("b", 64) + ".json", "sha256:" + strings.Repeat("b", 64), "target"},
	}
}
func TestInertClosureIndependentExpectedSet(t *testing.T) {
	root := rootForTest(t)
	deps := depsForTest()
	ref, uncertain, err := publish(root, "owner-one", "proposal", deps)
	if err != nil || uncertain != nil {
		t.Fatalf("ASSERT_CLOSURE_PUBLISH_VERIFIED: ref=%+v uncertainty=%+v err=%v", ref, uncertain, err)
	}
	if err := replay(root, ref, "owner-one", "proposal", deps); err != nil {
		t.Fatalf("ASSERT_CLOSURE_EXACT_REPLAY: %v", err)
	}
	cases := map[string][]reference{
		"substituted": append([]reference{{deps[0].Selector, "sha256:" + strings.Repeat("c", 64), deps[0].Role}}, deps[1]),
		"omitted":     deps[:1],
		"duplicated":  append(append([]reference{}, deps...), deps[1]),
	}
	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			if replay(root, ref, "owner-one", "proposal", expected) == nil {
				t.Fatal("ASSERT_CLOSURE_EXPECTATION_MISMATCH_DENIED")
			}
		})
	}
	if replay(root, ref, "owner-two", "proposal", deps) == nil {
		t.Fatal("ASSERT_CLOSURE_CROSS_OWNER_DENIED")
	}
	if replay(root, ref, "owner-one", "candidate", deps) == nil {
		t.Fatal("ASSERT_CLOSURE_CROSS_ROLE_DENIED")
	}
	if replay(root, reference{Selector: ref.Selector, Digest: deps[0].Digest, Role: ref.Role}, "owner-one", "proposal", deps) == nil {
		t.Fatal("ASSERT_CLOSURE_DIGEST_SUBSTITUTION_DENIED")
	}
}

func TestInertClosureCommittedUnverified(t *testing.T) {
	root := rootForTest(t)
	deps := depsForTest()
	original := readExact
	readExact = func(*publication.Root, string) ([]byte, error) { return nil, errors.New("synthetic readback failure") }
	defer func() { readExact = original }()
	ref, uncertain, err := publish(root, "owner-one", "proposal", deps)
	if err == nil || ref != (reference{}) || uncertain == nil || uncertain.Selector == "" || uncertain.Digest == "" {
		t.Fatalf("ASSERT_CLOSURE_COMMITTED_UNVERIFIED: ref=%+v uncertainty=%+v err=%v", ref, uncertain, err)
	}
	readExact = original
	if replay(root, reference{Selector: uncertain.Selector, Digest: uncertain.Digest, Role: "proposal"}, "owner-one", "proposal", deps) != nil {
		t.Fatal("ASSERT_CLOSURE_UNCERTAIN_BYTES_STILL_PRIVATE_READABLE")
	}
	if next, pending, err := publish(root, "owner-one", "proposal", deps); err == nil || next != (reference{}) || pending != nil {
		t.Fatalf("ASSERT_CLOSURE_NO_REPLACE: ref=%+v uncertainty=%+v err=%v", next, pending, err)
	}
}

func TestInertClosurePersistedByteSubstitution(t *testing.T) {
	root := rootForTest(t)
	deps := depsForTest()
	ref, _, err := publish(root, "owner-one", "proposal", deps)
	if err != nil {
		t.Fatal(err)
	}
	original := readExact
	defer func() { readExact = original }()
	for _, tc := range []struct {
		name   string
		mutate func(*record)
	}{
		{"owner", func(r *record) { r.Owner = "owner-two" }},
		{"role", func(r *record) { r.Role = "candidate" }},
		{"dependency", func(r *record) { r.Dependencies[0].Digest = "sha256:" + strings.Repeat("c", 64) }},
		{"omission", func(r *record) { r.Dependencies = r.Dependencies[:1] }},
		{"duplicate", func(r *record) { r.Dependencies = append(r.Dependencies, r.Dependencies[1]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readExact = func(root *publication.Root, selector string) ([]byte, error) {
				raw, e := original(root, selector)
				if e != nil {
					return nil, e
				}
				var r record
				if e = json.Unmarshal(raw, &r); e != nil {
					return nil, e
				}
				tc.mutate(&r)
				return json.Marshal(r)
			}
			if replay(root, ref, "owner-one", "proposal", deps) == nil {
				t.Fatal("ASSERT_CLOSURE_PERSISTED_SUBSTITUTION_DENIED")
			}
		})
	}
}

func TestInertClosureRejectsInvalidDependenciesBeforePublication(t *testing.T) {
	root := rootForTest(t)
	deps := depsForTest()
	cases := map[string][]reference{
		"duplicate":      {deps[0], deps[0]},
		"unsorted":       {deps[1], deps[0]},
		"missing-digest": {{Selector: deps[0].Selector, Role: deps[0].Role}},
		"oversized":      make([]reference, maxDependencies+1),
	}
	for name, list := range cases {
		t.Run(name, func(t *testing.T) {
			if ref, pending, err := publish(root, "owner-one", "proposal", list); err == nil || ref != (reference{}) || pending != nil {
				t.Fatalf("ASSERT_CLOSURE_INVALID_DEPENDENCY_DENIED: %+v %+v %v", ref, pending, err)
			}
		})
	}
}
