package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/censussourcecontinuation"
)

const d1 = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
const d2 = "sha256:2222222222222222222222222222222222222222222222222222222222222222"

func fixture(t *testing.T) (Manifest, []byte) {
	t.Helper()
	m, err := NewManifest(ManifestInput{LineageID: "lineage-1", SourceSelectionSHA256: d1, SourceSelectionByteLength: 7, PartitionPlanSHA256: d2, PartitionPlanByteLength: 9, Artifacts: []Artifact{{Kind: "capture", Selector: d1, SHA256: d1, ByteLength: 11}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	return m, raw
}

func TestManifestCanonicalGoldenAndNoAuthorityFields(t *testing.T) {
	_, raw := fixture(t)
	want := `{"schema_version":"lsp-trace.census-lineage-pilot-manifest.private.v1","authority":0,"accepted":false,"lineage_id":"lineage-1","source_selection_sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111","source_selection_byte_length":7,"partition_plan_sha256":"sha256:2222222222222222222222222222222222222222222222222222222222222222","partition_plan_byte_length":9,"artifacts":[{"kind":"capture","selector":"sha256:1111111111111111111111111111111111111111111111111111111111111111","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111","byte_length":11}],"effects":"NONE","feature_identity":"UNRESOLVED","completeness":"UNKNOWN","source_graph_complete":"UNKNOWN","absence_inference":"PROHIBITED"}`
	if string(raw) != want {
		t.Fatalf("golden mismatch\n%s", raw)
	}
	for _, forbidden := range []string{"semantic_authority", "public_authority"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatal(forbidden)
		}
	}
	if _, err := ParseManifest(raw); err != nil {
		t.Fatal(err)
	}
}

func TestManifestNilEmptyOrderingDuplicateAndAliasIsolation(t *testing.T) {
	base := ManifestInput{LineageID: "l", SourceSelectionSHA256: d1, SourceSelectionByteLength: 1, PartitionPlanSHA256: d2, PartitionPlanByteLength: 1}
	for _, a := range [][]Artifact{nil, {}, {{Kind: "b", Selector: d1, SHA256: d1, ByteLength: 1}, {Kind: "a", Selector: d2, SHA256: d2, ByteLength: 1}}, {{Kind: "a", Selector: d1, SHA256: d1, ByteLength: 1}, {Kind: "a", Selector: d1, SHA256: d1, ByteLength: 1}}, {{Kind: "a", Selector: d1, SHA256: d1, ByteLength: 1}, {Kind: "b", Selector: d1, SHA256: d1, ByteLength: 1}}} {
		base.Artifacts = a
		if _, err := NewManifest(base); err == nil {
			t.Fatalf("accepted %#v", a)
		}
	}
	a := []Artifact{{Kind: "a", Selector: d1, SHA256: d1, ByteLength: 1}}
	base.Artifacts = a
	m, err := NewManifest(base)
	if err != nil {
		t.Fatal(err)
	}
	a[0].Kind = "mutated"
	if m.Artifacts[0].Kind != "a" {
		t.Fatal("constructor alias")
	}
	copy := m.ArtifactsCopy()
	copy[0].Kind = "mutated"
	if m.Artifacts[0].Kind != "a" {
		t.Fatal("copy alias")
	}
}

func TestStrictManifestAndHeadWire(t *testing.T) {
	_, raw := fixture(t)
	unknown := append(bytes.TrimSuffix(raw, []byte("}")), []byte(`,"x":1}`)...)
	duplicate := bytes.Replace(raw, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1)
	for _, bad := range [][]byte{append(bytes.Clone(raw), '\n'), unknown, duplicate, bytes.Replace(raw, []byte(d1), []byte("sha256:ABC"), 1), bytes.Replace(raw, []byte(`"authority":0`), []byte(`"authority":1`), 1), bytes.Replace(raw, []byte(`"accepted":false`), []byte(`"accepted":true`), 1)} {
		if _, err := ParseManifest(bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	_, headRaw, err := newHead("w", "lineage-1", d1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseHeadRevision(headRaw); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append(bytes.Clone(headRaw), '\n'), bytes.Replace(headRaw, []byte(`"state":"COMMITTED"`), []byte(`"state":"OTHER"`), 1), bytes.Replace(headRaw, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1)} {
		if _, err = ParseHeadRevision(bad); err == nil {
			t.Fatalf("accepted head %s", bad)
		}
	}
}

type objects struct {
	raw                 map[string][]byte
	puts, gets          int
	putErr, getErr      error
	badSelector, badGet bool
}

func (s *objects) Put(_ context.Context, raw []byte) (string, error) {
	s.puts++
	if s.putErr != nil {
		return "", s.putErr
	}
	if s.raw == nil {
		s.raw = map[string][]byte{}
	}
	key := Digest(raw)
	s.raw[key] = bytes.Clone(raw)
	if s.badSelector {
		return d2, nil
	}
	return key, nil
}
func (s *objects) Get(_ context.Context, key string) ([]byte, error) {
	s.gets++
	if s.getErr != nil {
		return nil, s.getErr
	}
	b := bytes.Clone(s.raw[key])
	if s.badGet && len(b) > 0 {
		b[0] ^= 1
	}
	return b, nil
}

type heads struct {
	raw        []byte
	loads, cas int
	outcome    censussourcecontinuation.CASOutcome
	casErr     error
}

func (s *heads) Load(context.Context, string) ([]byte, error) {
	s.loads++
	return bytes.Clone(s.raw), nil
}
func (s *heads) CompareAndSwap(_ context.Context, _ string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	s.cas++
	if s.outcome == "" {
		s.outcome = censussourcecontinuation.CASCommitted
	}
	if s.outcome == censussourcecontinuation.CASCommitted {
		s.raw = bytes.Clone(raw)
	}
	return censussourcecontinuation.CASResult{Outcome: s.outcome, Err: s.casErr}
}

func request(t *testing.T, o *objects, h *heads) Request {
	m, raw := fixture(t)
	return Request{WorkflowKey: "w", HeadKey: "h", Manifest: m, ManifestBytes: raw, Objects: o, Heads: h}
}

func TestCoordinateOnePutOneCASReopenAndAlreadyCommitted(t *testing.T) {
	o, h := &objects{}, &heads{}
	r := request(t, o, h)
	got, err := Coordinate(context.Background(), r)
	if err != nil || got.State != "COMMITTED" || o.puts != 1 || o.gets != 1 || h.cas != 1 {
		t.Fatalf("got=%+v err=%v calls=%d/%d/%d", got, err, o.puts, o.gets, h.cas)
	}
	r.Expected = got.Head
	_, err = Coordinate(context.Background(), r)
	if !errors.Is(err, ErrAlreadyCommitted) || o.puts != 1 || h.cas != 1 || o.gets != 2 {
		t.Fatalf("already err=%v calls=%d/%d/%d", err, o.puts, o.gets, h.cas)
	}
	beforePut, beforeCAS := o.puts, h.cas
	verified, err := VerifyReopen(context.Background(), "w", "h", "lineage-1", r.ManifestBytes, o, h)
	if err != nil || verified.ManifestSelector != Digest(r.ManifestBytes) || o.puts != beforePut || h.cas != beforeCAS {
		t.Fatalf("verify=%+v err=%v", verified, err)
	}
}

func TestCommittedHeadBindsWorkflowAndManifestSelector(t *testing.T) {
	o, h := &objects{}, &heads{}
	r := request(t, o, h)
	committed, err := Coordinate(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(h.raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["workflow_key"] != r.WorkflowKey || wire["manifest_selector"] != committed.ManifestSelector {
		t.Fatalf("head does not bind request custody: %s", h.raw)
	}
}

func TestDifferentManifestSameLineageCannotBeAlreadyCommitted(t *testing.T) {
	o, h := &objects{}, &heads{}
	r := request(t, o, h)
	committed, err := Coordinate(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	other := r.Manifest
	other.SourceSelectionByteLength++
	otherRaw, err := EncodeManifest(other)
	if err != nil {
		t.Fatal(err)
	}
	otherSelector := Digest(otherRaw)
	o.raw[otherSelector] = bytes.Clone(otherRaw)
	r.Manifest, r.ManifestBytes, r.Expected = other, otherRaw, committed.Head
	puts, cas := o.puts, h.cas
	if _, err = Coordinate(context.Background(), r); !errors.Is(err, ErrConflict) || o.puts != puts || h.cas != cas {
		t.Fatalf("different manifest err=%v puts=%d cas=%d", err, o.puts, h.cas)
	}
	if _, err = VerifyReopen(context.Background(), r.WorkflowKey, r.HeadKey, other.LineageID, otherRaw, o, h); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reopen accepted independently present manifest: %v", err)
	}
}

func TestWrongWorkflowRejectsBeforeWrite(t *testing.T) {
	o, h := &objects{}, &heads{}
	r := request(t, o, h)
	committed, err := Coordinate(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	r.Expected, r.WorkflowKey = committed.Head, "other-workflow"
	puts, cas := o.puts, h.cas
	if _, err = Coordinate(context.Background(), r); !errors.Is(err, ErrInvalid) || o.puts != puts || h.cas != cas {
		t.Fatalf("wrong workflow err=%v puts=%d cas=%d", err, o.puts, h.cas)
	}
}

func TestTamperedHeadManifestSelectorRejected(t *testing.T) {
	_, raw, err := newHead("w", "lineage-1", d1, 1)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(raw, []byte(d1), []byte(d2), 1)
	if _, err := ParseHeadRevision(tampered); err == nil {
		t.Fatal("accepted tampered head canonical bytes")
	}
}

func TestCoordinateConflictFailuresAndCancellation(t *testing.T) {
	t.Run("conflict before put", func(t *testing.T) {
		o, h := &objects{}, &heads{}
		r := request(t, o, h)
		r.Expected = censussourcecontinuation.HeadRevision{Generation: 1, Selector: d1}
		_, err := Coordinate(context.Background(), r)
		if !errors.Is(err, ErrConflict) || o.puts != 0 || h.cas != 0 {
			t.Fatalf("%v %d %d", err, o.puts, h.cas)
		}
	})
	t.Run("stale no retry", func(t *testing.T) {
		o, h := &objects{}, &heads{outcome: censussourcecontinuation.CASNotCommitted}
		_, err := Coordinate(context.Background(), request(t, o, h))
		if !errors.Is(err, ErrConflict) || o.puts != 1 || h.cas != 1 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("uncertain", func(t *testing.T) {
		o, h := &objects{}, &heads{outcome: censussourcecontinuation.CASCommitUncertain}
		_, err := Coordinate(context.Background(), request(t, o, h))
		if !errors.Is(err, ErrIndeterminateExecution) || h.cas != 1 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("cancel shaped cas", func(t *testing.T) {
		o, h := &objects{}, &heads{outcome: censussourcecontinuation.CASNotCommitted, casErr: context.Canceled}
		_, err := Coordinate(context.Background(), request(t, o, h))
		if !errors.Is(err, ErrIndeterminateExecution) || h.cas != 1 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("put get mismatch", func(t *testing.T) {
		o, h := &objects{badGet: true}, &heads{}
		_, err := Coordinate(context.Background(), request(t, o, h))
		if !errors.Is(err, ErrInvalid) || h.cas != 0 || o.puts != 1 {
			t.Fatalf("%v", err)
		}
	})
	t.Run("pre cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		o, h := &objects{}, &heads{}
		_, err := Coordinate(ctx, request(t, o, h))
		if !errors.Is(err, context.Canceled) || o.puts != 0 || h.cas != 0 {
			t.Fatalf("%v", err)
		}
	})
}

func TestValidateDurableRootRejectsUnsafePaths(t *testing.T) {
	for _, p := range []string{"", "relative", "/private/tmp", "/private/tmp/child", "/tmp/../tmp"} {
		if ValidateDurableRoot(p) == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if ValidateDurableRoot(link) == nil {
		t.Fatal("accepted symlink")
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ValidateDurableRoot(file) == nil {
		t.Fatal("accepted file")
	}
	open := filepath.Join(base, "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	if ValidateDurableRoot(open) == nil {
		t.Fatal("accepted open permissions")
	}
}

func TestNoForbiddenLiteralInCanonicalWire(t *testing.T) {
	_, raw := fixture(t)
	if strings.Contains(string(raw), "semantic_authority") || strings.Contains(string(raw), "public_authority") {
		t.Fatal("forbidden authority field")
	}
}
