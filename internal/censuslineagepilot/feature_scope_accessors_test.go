//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestFeatureScopeCanonicalLinkageAndCopies(t *testing.T) {
	b, e := freezeFeatureScope(context.Background(), scopeFixture(t), fixtureRevision, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, p, m := b.SelectionBytes(), b.PlanBytes(), b.ManifestBytes()
	var parsed scopeManifest
	if e = json.Unmarshal(m, &parsed); e != nil {
		t.Fatal(e)
	}
	canonical, e := canonicalFeatureScope(parsed)
	if e != nil || !bytes.Equal(canonical, m) {
		t.Fatal("P2 manifest canonical round trip")
	}
	if parsed.SelectionSHA256 != Digest(s) || parsed.SelectionByteLength != uint64(len(s)) || parsed.PlanSHA256 != Digest(p) || parsed.PlanByteLength != uint64(len(p)) {
		t.Fatal("P5 manifest exact linkage")
	}
	if b.ManifestDigest() != Digest(append([]byte(featureScopeDomain+"\x00"), m...)) {
		t.Fatal("P2 manifest domain")
	}
	s[0] = 'X'
	p[0] = 'X'
	m[0] = 'X'
	if b.SelectionBytes()[0] == 'X' || b.PlanBytes()[0] == 'X' || b.ManifestBytes()[0] == 'X' {
		t.Fatal("P4 byte accessor aliases")
	}
	pins := b.Files()
	other, e := freezeFeatureScope(context.Background(), parsed.Root, fixtureRevision, pins)
	if e != nil {
		t.Fatal(e)
	}
	pins[0].Bytes[0] = 'X'
	if other.Files()[0].Bytes[0] == 'X' {
		t.Fatal("P4 comparison pins alias")
	}
	for _, mutate := range []func([]scopeObservedFile) []scopeObservedFile{
		func(p []scopeObservedFile) []scopeObservedFile { return p[:5] },
		func(p []scopeObservedFile) []scopeObservedFile { p[0].ByteLength++; return p },
		func(p []scopeObservedFile) []scopeObservedFile { p[0].Role = "CONTEXT_ONLY"; return p },
		func(p []scopeObservedFile) []scopeObservedFile { p[0].Path = "arbitrary.go"; return p },
	} {
		bad, e := freezeFeatureScope(context.Background(), parsed.Root, fixtureRevision, mutate(b.Files()))
		if e == nil || bad.state != nil {
			t.Fatal("P3 invalid comparison pins")
		}
	}
	var zero featureScope
	if zero.Files() != nil || zero.ManifestBytes() != nil || zero.SelectionBytes() != nil || zero.PlanBytes() != nil || zero.ManifestDigest() != "" || zero.Selection().Files != nil || zero.PartitionPlan().Plan.Files != nil {
		t.Fatal("P4 zero bundle")
	}
}
