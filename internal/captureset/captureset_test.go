package captureset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func dg(s string) string { x := sha256.Sum256([]byte(s)); return "sha256:" + hex.EncodeToString(x[:]) }
func fixture(n int) ([]Target, []Constituent, Ledger, Ledger) {
	ts := make([]Target, n)
	for i := range ts {
		b := string(rune('A'+i%26)) + string(rune(i))
		ts[i] = Target{CensusOrdinal: i, CanonicalSeedV2: b, CanonicalSeedV2SHA256: dg(b)}
	}
	cs := make([]Constituent, (n+62)/63)
	for i := range cs {
		digest := dg("c" + string(rune(i)))
		cs[i] = Constituent{ImmutableSelector: ConstituentSelectorPrefix + strings.TrimPrefix(digest, "sha256:"), SchemaID: "lsp-trace.graph-provenance.v5", SHA256: digest, ByteLength: i + 1, NativeV5Identity: "v5-" + string(rune('a'+i))}
	}
	l := Ledger{Denominator: 1, Entries: []LedgerEntry{{Ordinal: 0, Identity: "x", Disposition: "processed"}}}
	return ts, cs, l, l
}
func TestPrepareCanonicalBatchesAndIdentity(t *testing.T) {
	ts, cs, f, s := fixture(130)
	m, err := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	if err != nil {
		t.Fatal(err)
	}
	if got := []int{m.Batches[0].TargetCount, m.Batches[1].TargetCount, m.Batches[2].TargetCount}; got[0] != 63 || got[1] != 63 || got[2] != 4 {
		t.Fatalf("ASSERT_MAXIMAL_BATCHES: %v", got)
	}
	a, _ := EncodeCanonical(m)
	b, _ := EncodeCanonical(m)
	if !bytes.Equal(a, b) || m.LogicalDigest == "" {
		t.Fatal("ASSERT_CANONICAL_IDENTITY")
	}
	if m.Authority != 0 || m.SourceGraphComplete != "UNKNOWN" || m.NativeSingleCaptureCustody || m.LeidenAdmissible || len(m.CrossCaptureCalls) != 0 || m.Disclosure != "PRIVATE" {
		t.Fatal("ASSERT_ZERO_AUTHORITY")
	}
}
func TestVersionedSchemaAcceptsCanonicalAndRejectsUnknown(t *testing.T) {
	ts, cs, f, s := fixture(64)
	m, _ := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	raw, _ := EncodeCanonical(m)
	if err := ValidateSchema(raw); err != nil {
		t.Fatalf("ASSERT_SCHEMA_GREEN: %v", err)
	}
	bad := bytes.Replace(raw, []byte(`"authority":0`), []byte(`"unknown":1,"authority":0`), 1)
	if err := ValidateSchema(bad); err == nil {
		t.Fatal("ASSERT_SCHEMA_UNKNOWN_REJECTED")
	}
}

func TestValidatePlanningTargetsRejectsDuplicateCensusOrdinal(t *testing.T) {
	targets, _, _, _ := fixture(2)
	targets[1].CensusOrdinal = targets[0].CensusOrdinal
	if err := ValidatePlanningTargets(targets); err == nil || !strings.Contains(err.Error(), "duplicate census ordinal") {
		t.Fatalf("ASSERT_DUPLICATE_CENSUS_ORDINAL_REJECTED: %v", err)
	}
}

func TestValidatePlanningTargetsPreservesNonemptySeedCompatibility(t *testing.T) {
	seed := " "
	target := Target{CensusOrdinal: 0, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: dg(seed)}
	if err := ValidatePlanningTargets([]Target{target}); err != nil {
		t.Fatalf("ASSERT_NONEMPTY_SEED_ACCEPTED: %v", err)
	}
}

func TestStrictAndMutationRejection(t *testing.T) {
	ts, cs, f, s := fixture(64)
	m, _ := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	raw, _ := EncodeCanonical(m)
	for name, bad := range map[string][]byte{"eof": append(append([]byte{}, raw...), []byte("{}")...), "unknown": bytes.Replace(raw, []byte(`"authority":0`), []byte(`"surplus":1,"authority":0`), 1), "mutation": bytes.Replace(raw, []byte(cs[0].SHA256), []byte(dg("wrong")), 1)} {
		if _, err := Decode(bad); err == nil {
			t.Fatalf("ASSERT_REJECT_%s", name)
		}
	}
}
func TestRejectMissingDuplicateReorderedForeignAndBounds(t *testing.T) {
	ts, cs, f, s := fixture(64)
	base, _ := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	clone := func() Manifest {
		x := base
		x.Constituents = append([]Constituent(nil), base.Constituents...)
		return x
	}
	cases := map[string]Manifest{}
	x := clone()
	x.Constituents = x.Constituents[:1]
	cases["missing"] = x
	x = clone()
	x.Constituents = []Constituent{x.Constituents[0], x.Constituents[0]}
	cases["duplicate"] = x
	x = clone()
	x.Constituents[0], x.Constituents[1] = x.Constituents[1], x.Constituents[0]
	cases["reordered"] = x
	x = clone()
	x.Constituents[0].SchemaID = "foreign"
	cases["foreign"] = x
	wantErrors := map[string]string{
		"missing":   "missing constituent or batch",
		"duplicate": "duplicate constituent",
		"reordered": "reordered constituent",
		"foreign":   "foreign constituent",
	}
	for n, m := range cases {
		m.LogicalDigest = ""
		m.ImmutableSelector = ""
		if _, err := EncodeCanonical(m); err == nil || !strings.Contains(err.Error(), wantErrors[n]) {
			t.Fatalf("ASSERT_REJECT_%s: err=%v", n, err)
		}
	}
	too := make([]Target, MaxTargets+1)
	if _, err := Prepare(too, nil, f, s, "c", "d"); err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatal("ASSERT_TARGET_BOUND")
	}
	f.Denominator = MaxResources + 1
	if _, err := Prepare(ts, cs, f, s, "c", "d"); err == nil {
		t.Fatal("ASSERT_RESOURCE_BOUND")
	}
}
func TestRedactionHasDistinctIdentity(t *testing.T) {
	ts, cs, f, s := fixture(64)
	m, _ := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	r, err := Redact(m)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeCanonical(r)
	if err != nil {
		t.Fatal(err)
	}
	redacted, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if redacted.LogicalDigest == m.LogicalDigest || redacted.ImmutableSelector == m.ImmutableSelector {
		t.Fatal("ASSERT_REDACTION_IDENTITY")
	}
}
