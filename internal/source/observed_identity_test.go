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

func observedFixture(t *testing.T, readable bool) ObservedIdentityRequest {
	t.Helper()
	root := t.TempDir()
	r, err := NewInputRecorder(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if readable {
		if err := os.WriteFile(filepath.Join(root, "input.go"), []byte("package fixture\r\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, id, err := r.ReadInput("input.go", InputSource)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.BindContribution("source-output", id); err != nil {
			t.Fatal(err)
		}
	}
	content, id, err := r.ReadInput("missing.conf", InputConfiguration)
	if err == nil || id == "" || content != nil {
		t.Fatal("fixture: missing scoped read did not produce a failure receipt")
	}
	if err := r.BindContribution("configuration-outcome", id); err != nil {
		t.Fatal(err)
	}
	ids := []string{"configuration-outcome", "unbound-output"}
	if readable {
		ids = append(ids, "source-output")
	}
	e, err := r.Evidence(ids)
	if err != nil {
		t.Fatal(err)
	}
	return ObservedIdentityRequest{Evidence: e, Acquisition: AcquisitionContext{Adapter: "bounded-input", WorkspaceURI: "file:///fixture", InvocationID: "run-1"}}
}

func observedBuild(t *testing.T, request ObservedIdentityRequest) ObservedIdentityResult {
	t.Helper()
	result, err := BuildObservedIdentity(request)
	if err != nil {
		t.Fatalf("ASSERT_OBSERVED_IDENTITY: %v", err)
	}
	return result
}

// Adapted from the retained operational-contract probe: actual unreadable bytes
// must not require a fabricated ManifestReceipt.Digest to obtain an identity.
// Every subtest logs its procedure identity through go test -v; the retained
// RED/GREEN logs bind observations to these persistent guard names.
func TestObservedIdentityActualFailure(t *testing.T) {
	for _, readable := range []bool{false, true} {
		name := "all-failed"
		if readable {
			name = "mixed"
		}
		t.Run(name, func(t *testing.T) {
			request := observedFixture(t, readable)
			result, err := BuildObservedIdentity(request)
			if err != nil {
				t.Fatalf("ASSERT_FAILED_READ_EXPLICIT_EXCLUSION_WITHOUT_INVENTED_DIGEST: %v", err)
			}
			if result.Policy != ObservedIdentityPolicyV1 || result.SnapshotID == "" {
				t.Fatal("ASSERT_OBSERVED_POLICY: missing new identity")
			}
			var manifest struct {
				SchemaVersion string `json:"schema_version"`
				Status        string `json:"status"`
				Inputs        []struct {
					Path             string              `json:"path"`
					Class            InputClass          `json:"class"`
					Status           AcquisitionStatus   `json:"status"`
					ReceiptID        string              `json:"receipt_id"`
					ContentDigest    *string             `json:"content_digest"`
					Failure          *AcquisitionFailure `json:"failure"`
					CanonicalReceipt []byte              `json:"canonical_receipt"`
				} `json:"inputs"`
				IncompleteReasons []string `json:"incomplete_reasons"`
			}
			if err := json.Unmarshal(result.Manifest, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.SchemaVersion != ObservedManifestVersionV1 || manifest.Status != "INCOMPLETE" || len(manifest.Inputs) != len(request.Evidence.Inputs) || len(manifest.IncompleteReasons) != len(request.Evidence.IncompleteReasons) {
				t.Fatal("ASSERT_OBSERVED_INCOMPLETE: observed records or incomplete state lost")
			}
			for _, in := range manifest.Inputs {
				if in.Status == Unreadable && (in.Path != "missing.conf" || in.Class != InputConfiguration || in.ReceiptID == "" || in.ContentDigest != nil || in.Failure == nil || in.Failure.Reason == "" || len(in.CanonicalReceipt) == 0) {
					t.Fatal("ASSERT_OBSERVED_FAILED_RECORD: failed observation acquired fabricated content or lost custody")
				}
			}
		})
	}
}

func cloneObserved(t *testing.T, r ObservedIdentityRequest) ObservedIdentityRequest {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out ObservedIdentityRequest
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestObservedIdentityReplay(t *testing.T) {
	r := observedFixture(t, true)
	a := observedBuild(t, r)
	reordered := cloneObserved(t, r)
	for i, j := 0, len(reordered.Evidence.Inputs)-1; i < j; i, j = i+1, j-1 {
		reordered.Evidence.Inputs[i], reordered.Evidence.Inputs[j] = reordered.Evidence.Inputs[j], reordered.Evidence.Inputs[i]
	}
	for i, j := 0, len(reordered.Evidence.Contributions)-1; i < j; i, j = i+1, j-1 {
		reordered.Evidence.Contributions[i], reordered.Evidence.Contributions[j] = reordered.Evidence.Contributions[j], reordered.Evidence.Contributions[i]
	}
	for i, j := 0, len(reordered.Evidence.IncompleteReasons)-1; i < j; i, j = i+1, j-1 {
		reordered.Evidence.IncompleteReasons[i], reordered.Evidence.IncompleteReasons[j] = reordered.Evidence.IncompleteReasons[j], reordered.Evidence.IncompleteReasons[i]
	}
	b := observedBuild(t, reordered)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("ASSERT_OBSERVED_ORDER: observation order changed identity")
	}
	r.Acquisition.Adapter = "other-adapter"
	r.Acquisition.WorkspaceURI = "file:///other"
	r.Acquisition.InvocationID = "run-2"
	c := observedBuild(t, r)
	if a.SourceID != c.SourceID || a.SnapshotID != c.SnapshotID || a.CollectionID == c.CollectionID || !bytes.Equal(a.Manifest, c.Manifest) {
		t.Fatal("ASSERT_OBSERVED_CONTEXT: acquisition must affect only collection")
	}
	r.Revision = &RevisionAttestation{System: "git", Revision: "untrusted-annotation"}
	d := observedBuild(t, r)
	if c.SourceID != d.SourceID || c.SnapshotID != d.SnapshotID || c.CollectionID != d.CollectionID {
		t.Fatal("ASSERT_OBSERVED_REVISION: annotation changed identity")
	}
	if a.Policy == IdentityPolicyV1 || a.SourceID == a.SnapshotID || a.SnapshotID == a.CollectionID {
		t.Fatal("ASSERT_OBSERVED_DOMAINS: new distinct domains required")
	}
}

func TestObservedIdentityEmpty(t *testing.T) {
	r, err := NewInputRecorder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	e, err := r.Evidence(nil)
	if err != nil {
		t.Fatal(err)
	}
	empty := observedBuild(t, ObservedIdentityRequest{Evidence: e, Acquisition: AcquisitionContext{Adapter: "bounded-input"}})
	failed := observedBuild(t, observedFixture(t, false))
	if empty.SourceID != failed.SourceID || empty.SnapshotID == failed.SnapshotID || !bytes.Contains(empty.Manifest, []byte(`"status":"INCOMPLETE"`)) {
		t.Fatal("ASSERT_OBSERVED_EMPTY: failure must affect snapshot but not empty source content")
	}
}

func TestObservedIdentityOwnership(t *testing.T) {
	r := observedFixture(t, true)
	r.Revision = &RevisionAttestation{System: "git", Revision: "v1"}
	before := cloneObserved(t, r)
	a := observedBuild(t, r)
	if !reflect.DeepEqual(before, r) {
		t.Fatal("ASSERT_OBSERVED_OWNERSHIP: builder mutated request")
	}
	saved := append([]byte(nil), a.Manifest...)
	r.Revision.Revision = "v2"
	r.Evidence.Inputs[0].CanonicalReceipt[0] = '!'
	if !bytes.Equal(saved, a.Manifest) || a.Revision.Revision != "v1" {
		t.Fatal("ASSERT_OBSERVED_OWNERSHIP: request mutation escaped into result")
	}
	a.Manifest[0] = '!'
	a.Revision.Revision = "changed-result"
	b := observedBuild(t, before)
	if !bytes.Equal(saved, b.Manifest) || b.Revision.Revision != "v1" {
		t.Fatal("ASSERT_OBSERVED_OWNERSHIP: result mutation contaminated replay")
	}
}

// Keep accounting coherent while corrupting only the canonical identity join.
func replaceObservedReference(r *ObservedIdentityRequest, oldID, newID string) {
	for i := range r.Evidence.Inputs {
		if r.Evidence.Inputs[i].ID == oldID {
			r.Evidence.Inputs[i].ID = newID
		}
	}
	for i := range r.Evidence.Contributions {
		for j, id := range r.Evidence.Contributions[i].ReceiptIDs {
			if id == oldID {
				r.Evidence.Contributions[i].ReceiptIDs[j] = newID
			}
		}
	}
	for i, s := range r.Evidence.IncompleteReasons {
		r.Evidence.IncompleteReasons[i] = strings.ReplaceAll(s, oldID, newID)
	}
}

func TestObservedIdentityRejects(t *testing.T) {
	base := observedFixture(t, true)
	// Each guard also runs an unmodified actual-recorder control, so a blanket
	// rejecting implementation is not a valid passing result for any negative.
	cases := map[string]func(*ObservedIdentityRequest){
		"receipt-id": func(r *ObservedIdentityRequest) {
			replaceObservedReference(r, r.Evidence.Inputs[0].ID, "sha256:"+strings.Repeat("0", 64))
		},
		"canonical-bytes": func(r *ObservedIdentityRequest) {
			r.Evidence.Inputs[0].CanonicalReceipt = append(r.Evidence.Inputs[0].CanonicalReceipt, ' ')
			replaceObservedReference(r, r.Evidence.Inputs[0].ID, fmt.Sprintf("sha256:%x", sha256.Sum256(r.Evidence.Inputs[0].CanonicalReceipt)))
		},
		"canonical-json-field": func(r *ObservedIdentityRequest) {
			r.Evidence.Inputs[0].CanonicalReceipt = []byte(`{"forged":true}`)
			replaceObservedReference(r, r.Evidence.Inputs[0].ID, fmt.Sprintf("sha256:%x", sha256.Sum256(r.Evidence.Inputs[0].CanonicalReceipt)))
		},
		"receipt-version":        func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Receipt.ReceiptVersion = "other" },
		"class":                  func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Class = InputDeclaration },
		"unknown-class":          func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Class = "UNKNOWN" },
		"path-escape":            func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Receipt.Item.Locator = "../escape" },
		"uri":                    func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Receipt.Item.Locator = "file:///escape" },
		"item-id":                func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Receipt.Item.ID = "different" },
		"provenance-path":        func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Receipt.Provenance.Locator = "different" },
		"provenance-revision":    func(r *ObservedIdentityRequest) { r.Evidence.Inputs[0].Receipt.Provenance.Revision = "claimed" },
		"duplicate-path":         func(r *ObservedIdentityRequest) { r.Evidence.Inputs = append(r.Evidence.Inputs, r.Evidence.Inputs[0]) },
		"evidence-version":       func(r *ObservedIdentityRequest) { r.Evidence.SchemaVersion = "other" },
		"missing-incompleteness": func(r *ObservedIdentityRequest) { r.Evidence.IncompleteReasons = nil },
		"omitted-failure-reason": func(r *ObservedIdentityRequest) {
			for i, v := range r.Evidence.IncompleteReasons {
				if strings.HasPrefix(v, "failed input acquisition:") {
					r.Evidence.IncompleteReasons = append(r.Evidence.IncompleteReasons[:i], r.Evidence.IncompleteReasons[i+1:]...)
					break
				}
			}
		},
		"omitted-binding-reason": func(r *ObservedIdentityRequest) {
			for i, v := range r.Evidence.IncompleteReasons {
				if strings.HasPrefix(v, "missing input binding") {
					r.Evidence.IncompleteReasons = append(r.Evidence.IncompleteReasons[:i], r.Evidence.IncompleteReasons[i+1:]...)
					break
				}
			}
		},
		"duplicate-reason": func(r *ObservedIdentityRequest) {
			r.Evidence.IncompleteReasons = append(r.Evidence.IncompleteReasons, r.Evidence.IncompleteReasons[0])
		},
		"empty-reason": func(r *ObservedIdentityRequest) {
			r.Evidence.IncompleteReasons = append(r.Evidence.IncompleteReasons, "")
		},
		"unknown-reference": func(r *ObservedIdentityRequest) { r.Evidence.Contributions[0].ReceiptIDs = []string{"unknown"} },
		"duplicate-reference": func(r *ObservedIdentityRequest) {
			id := r.Evidence.Inputs[0].ID
			r.Evidence.Contributions[0].ReceiptIDs = []string{id, id}
		},
		"duplicate-contribution": func(r *ObservedIdentityRequest) {
			r.Evidence.Contributions = append(r.Evidence.Contributions, r.Evidence.Contributions[0])
		},
		"empty-contribution": func(r *ObservedIdentityRequest) { r.Evidence.Contributions[0].ID = " " },
		"empty-adapter":      func(r *ObservedIdentityRequest) { r.Acquisition.Adapter = " " },
		"empty-revision":     func(r *ObservedIdentityRequest) { r.Revision = &RevisionAttestation{System: "git"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildObservedIdentity(base); err != nil {
				t.Fatalf("ASSERT_OBSERVED_REJECT_%s: valid control rejected: %v", name, err)
			}
			r := cloneObserved(t, base)
			mutate(&r)
			if _, err := BuildObservedIdentity(r); err == nil {
				t.Fatalf("ASSERT_OBSERVED_REJECT_%s: inconsistent evidence accepted", name)
			}
		})
	}
	for _, name := range []string{"unreadable-content", "unreadable-empty-content", "unreadable-digest", "readable-content", "readable-failure", "status"} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildObservedIdentity(base); err != nil {
				t.Fatalf("ASSERT_OBSERVED_REJECT_%s: valid control rejected: %v", name, err)
			}
			r := cloneObserved(t, base)
			for i := range r.Evidence.Inputs {
				in := &r.Evidence.Inputs[i]
				switch name {
				case "unreadable-content":
					if in.Receipt.Status == Unreadable {
						in.Content = []byte("fake")
					}
				case "unreadable-empty-content":
					if in.Receipt.Status == Unreadable {
						in.Content = []byte{}
					}
				case "unreadable-digest":
					if in.Receipt.Status == Unreadable {
						in.Receipt.ContentIdentity = &ContentIdentity{Algorithm: "sha256", Digest: "sha256:" + strings.Repeat("0", 64), Scope: "ACQUIRED_BYTES"}
					}
				case "readable-content":
					if in.Receipt.Status == Readable {
						in.Content = []byte("changed")
					}
				case "readable-failure":
					if in.Receipt.Status == Readable {
						in.Receipt.Failure = &AcquisitionFailure{Reason: "impossible"}
					}
				case "status":
					in.Receipt.Status = "UNKNOWN"
				}
			}
			if _, err := BuildObservedIdentity(r); err == nil {
				t.Fatalf("ASSERT_OBSERVED_REJECT_%s: inconsistent evidence accepted", name)
			}
		})
	}
}

// observedOne obtains different outcomes through actual OS reads.
func observedOne(t *testing.T, name string, class InputClass, kind string, content []byte) ObservedIdentityRequest {
	t.Helper()
	root := t.TempDir()
	switch kind {
	case "file":
		if err := os.WriteFile(filepath.Join(root, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	r, err := NewInputRecorder(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_, id, readErr := r.ReadInput(name, class)
	if id == "" || ((readErr == nil) != (kind == "file")) {
		t.Fatalf("fixture: read status %s: %v", kind, readErr)
	}
	e, err := r.Evidence(nil)
	if err != nil {
		t.Fatal(err)
	}
	return ObservedIdentityRequest{Evidence: e, Acquisition: AcquisitionContext{Adapter: "bounded-input"}}
}

func TestObservedIdentitySeparatesFailures(t *testing.T) {
	a := observedBuild(t, observedOne(t, "input", InputConfiguration, "missing", nil))
	cases := map[string]ObservedIdentityRequest{
		"path":   observedOne(t, "other", InputConfiguration, "missing", nil),
		"class":  observedOne(t, "input", InputDeclaration, "missing", nil),
		"reason": observedOne(t, "input", InputConfiguration, "directory", nil),
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			b := observedBuild(t, request)
			if a.SourceID != b.SourceID || a.SnapshotID == b.SnapshotID || a.CollectionID == b.CollectionID {
				t.Fatalf("ASSERT_OBSERVED_FAILURE_%s: failed observation must change snapshot/collection only", name)
			}
		})
	}
	t.Run("status", func(t *testing.T) {
		b := observedBuild(t, observedOne(t, "input", InputConfiguration, "file", []byte{}))
		if a.SourceID == b.SourceID || a.SnapshotID == b.SnapshotID {
			t.Fatal("ASSERT_OBSERVED_FAILURE_status: actual empty readable file must differ from no acquired content")
		}
	})
}

func TestObservedIdentityReadableCommitment(t *testing.T) {
	r := observedOne(t, "input", InputSource, "file", []byte("one"))
	a := observedBuild(t, r)
	b := observedBuild(t, observedOne(t, "input", InputSource, "file", []byte("two")))
	if a.SourceID == b.SourceID || a.SnapshotID == b.SnapshotID {
		t.Fatal("ASSERT_OBSERVED_CONTENT: changed actual bytes lost")
	}
	var m observedManifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("one")))
	if len(m.Inputs) != 1 || m.Inputs[0].ContentDigest != want || m.Inputs[0].ReceiptID == want || !bytes.Equal(m.Inputs[0].CanonicalReceipt, r.Evidence.Inputs[0].CanonicalReceipt) {
		t.Fatal("ASSERT_OBSERVED_CONTENT: receipt JSON hash confused with actual-byte digest")
	}
	for _, class := range []InputClass{InputConfiguration, InputDeclaration, InputGeneratedMapping} {
		c := observedBuild(t, observedOne(t, "input", class, "file", []byte("one")))
		if a.SourceID != c.SourceID || a.SnapshotID == c.SnapshotID {
			t.Fatal("ASSERT_OBSERVED_CLASS: readable class affects custody, not source bytes")
		}
	}
}

func TestObservedIdentityContributionCommitment(t *testing.T) {
	r := observedFixture(t, true)
	a := observedBuild(t, r)
	for i := range r.Evidence.Contributions {
		if len(r.Evidence.Contributions[i].ReceiptIDs) > 0 {
			r.Evidence.Contributions[i].ReceiptIDs = []string{r.Evidence.Inputs[0].ID, r.Evidence.Inputs[1].ID}
			break
		}
	}
	b := observedBuild(t, r)
	if a.SourceID != b.SourceID || a.SnapshotID == b.SnapshotID {
		t.Fatal("ASSERT_OBSERVED_BINDING: contribution references not committed")
	}
	for i := range r.Evidence.Contributions {
		refs := r.Evidence.Contributions[i].ReceiptIDs
		if len(refs) == 2 {
			refs[0], refs[1] = refs[1], refs[0]
		}
	}
	c := observedBuild(t, r)
	if !reflect.DeepEqual(b, c) {
		t.Fatal("ASSERT_OBSERVED_REFERENCE_ORDER: reference order changed identity")
	}
	r.Evidence.IncompleteReasons = append(r.Evidence.IncompleteReasons, "external tool inputs unobserved")
	d := observedBuild(t, r)
	if c.SourceID != d.SourceID || c.SnapshotID == d.SnapshotID {
		t.Fatal("ASSERT_OBSERVED_REASON: additional uncertainty not committed")
	}
}

func TestObservedIdentityRejectsConflictingReads(t *testing.T) {
	for _, kind := range []string{"class", "bytes", "status"} {
		t.Run(kind, func(t *testing.T) {
			r, root := recorderFixture(t)
			_, _, err := r.ReadInput("input", InputSource)
			if err != nil {
				t.Fatal(err)
			}
			single, err := r.Evidence(nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := BuildObservedIdentity(ObservedIdentityRequest{Evidence: single, Acquisition: AcquisitionContext{Adapter: "test"}}); err != nil {
				t.Fatalf("ASSERT_OBSERVED_CONFLICT_%s: valid control: %v", kind, err)
			}
			class := InputSource
			switch kind {
			case "class":
				class = InputConfiguration
			case "bytes":
				if err := os.WriteFile(filepath.Join(root, "input"), []byte("new version"), 0600); err != nil {
					t.Fatal(err)
				}
			case "status":
				if err := os.Remove(filepath.Join(root, "input")); err != nil {
					t.Fatal(err)
				}
			}
			_, id, err := r.ReadInput("input", class)
			if id == "" || ((err != nil) != (kind == "status")) {
				t.Fatal("fixture: conflicting read not captured")
			}
			e, err := r.Evidence(nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(e.Inputs) != 2 {
				t.Fatal("fixture: recorder lost conflicting versions")
			}
			if _, err := BuildObservedIdentity(ObservedIdentityRequest{Evidence: e, Acquisition: AcquisitionContext{Adapter: "test"}}); err == nil {
				t.Fatalf("ASSERT_OBSERVED_CONFLICT_%s: conflicting observations silently selected", kind)
			}
		})
	}
}

func TestObservedIdentityCanonicalScope(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "file:///escape", "a/../b", "a\\b", "a\x00b", ".", "..", "invalid-\xff"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			r := observedOne(t, "input", InputSource, "file", []byte("one"))
			observedBuild(t, r)
			in := &r.Evidence.Inputs[0]
			// Coherent test forgery isolates scope from receipt/hash consistency.
			receipt, content, canonical, err := CanonicalizeReceipt(DiscoveredItem{ID: name, Locator: name}, Acquisition{Status: Readable, Provenance: Provenance{Mechanism: "bounded-input/SOURCE", Locator: name}}, in.Content)
			if err != nil {
				t.Fatal(err)
			}
			*in = InputReceipt{ID: fmt.Sprintf("sha256:%x", sha256.Sum256(canonical)), Class: InputSource, Receipt: receipt, Content: content, CanonicalReceipt: canonical}
			if _, err := BuildObservedIdentity(r); err == nil {
				t.Fatal("ASSERT_OBSERVED_CANONICAL_SCOPE: coherent out-of-scope receipt accepted")
			}
		})
	}
}

func TestObservedIdentityFixedVectors(t *testing.T) {
	// Independently encoded with Python json.dumps, struct.pack('>Q'), hashlib.
	a := observedBuild(t, observedOne(t, "input", InputSource, "file", []byte("one")))
	if a.SourceID != "sha256:49a3cc2fdee3ef7af9a91a368a4026ad60822458bc2f30a40b37197dd726937a" || a.SnapshotID != "sha256:3c8e59a0ebd8461c965bae2a4b86d2d3a17ecf1a9535e038b880d04d34c97708" || a.CollectionID != "sha256:48f97a004134976867f1ad360ee2470b5d9f612b62d9cce5f8c8a1d70c0f32e8" {
		t.Fatal("ASSERT_OBSERVED_VECTORS: new policy byte contract drift")
	}
}

func TestObservedIdentityLossyStrings(t *testing.T) {
	base := observedFixture(t, true)
	cases := map[string]func(*ObservedIdentityRequest){
		"adapter":    func(r *ObservedIdentityRequest) { r.Acquisition.Adapter = "\xff" },
		"workspace":  func(r *ObservedIdentityRequest) { r.Acquisition.WorkspaceURI = "\xff" },
		"invocation": func(r *ObservedIdentityRequest) { r.Acquisition.InvocationID = "\xff" },
		"revision":   func(r *ObservedIdentityRequest) { r.Revision = &RevisionAttestation{System: "git", Revision: "\xff"} },
		"reason": func(r *ObservedIdentityRequest) {
			r.Evidence.IncompleteReasons = append(r.Evidence.IncompleteReasons, "\xff")
		},
		"contribution": func(r *ObservedIdentityRequest) { r.Evidence.Contributions[0].ID = "\xff" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			observedBuild(t, base)
			r := cloneObserved(t, base)
			mutate(&r)
			if _, err := BuildObservedIdentity(r); err == nil {
				t.Fatal("ASSERT_OBSERVED_LOSSY_STRING: invalid UTF-8 aliases replacement character")
			}
		})
	}
}

func TestObservedIdentityLegacySeparation(t *testing.T) {
	r := observedOne(t, "input", InputSource, "file", []byte("one"))
	a := observedBuild(t, r)
	in := r.Evidence.Inputs[0]
	legacy, err := BuildIdentity(IdentityRequest{Receipts: []ManifestReceipt{{ID: in.ID, Path: "input", Digest: in.Receipt.ContentIdentity.Digest}}, Decisions: []ManifestDecision{{ReceiptID: in.ID, State: ManifestInclude}}, Acquisition: r.Acquisition})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{legacy.SourceID, legacy.SnapshotID, legacy.CollectionID, legacy.LegacyManifestID} {
		if a.SourceID == id || a.SnapshotID == id || a.CollectionID == id {
			t.Fatal("ASSERT_OBSERVED_LEGACY: observed policy silently aliases historical identity")
		}
	}
}
