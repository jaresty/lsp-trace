package capturesetinspection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/captureset"
)

func fixture(t *testing.T) captureset.Manifest {
	t.Helper()
	seed := "seed-a"
	sum := sha256.Sum256([]byte(seed))
	targets := []captureset.Target{{CensusOrdinal: 0, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: "sha256:" + hex.EncodeToString(sum[:])}}
	constituents := []captureset.Constituent{{ImmutableSelector: "graph-provenance-v5/sha256/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SchemaID: captureset.NativeV5SchemaID, SHA256: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ByteLength: 1, NativeV5Identity: "native-private-id"}}
	files := captureset.Ledger{Denominator: 2, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private-file-a", Disposition: "CAPTURED"}, {Ordinal: 1, Identity: "private-file-b", Disposition: "SKIPPED"}}}
	symbols := captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private-symbol", Disposition: "CAPTURED"}}}
	m, err := captureset.Prepare(targets, constituents, files, symbols, "census.v1", "retain.v1")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestProjectIsBoundedDeterministicAndNonDisclosing(t *testing.T) {
	m := fixture(t)
	left, right := Project(m), Project(m)
	if err := Validate(left); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatal("ASSERT_CAPTURE_SET_INSPECTION_DETERMINISTIC")
	}
	raw, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"seed-a", "private-file-a", "private-file-b", "private-symbol", "native-private-id", "graph-provenance-v5/"} {
		if contains(raw, secret) {
			t.Fatalf("ASSERT_CAPTURE_SET_INSPECTION_PRIVATE_FIELD_ABSENT: %q in %s", secret, raw)
		}
	}
	if left.InspectionVersion != Version || left.TargetCount != 1 || left.BatchCount != 1 || left.ConstituentCount != 1 || left.Files.Denominator != 2 || len(left.Files.Dispositions) != 2 || left.Authority != 0 || left.SourceGraphComplete != "UNKNOWN" || left.NativeSingleCaptureCustody || left.CrossCaptureCalls || left.LeidenAdmissible {
		t.Fatalf("ASSERT_CAPTURE_SET_INSPECTION_BOUNDED_CEILINGS: %+v", left)
	}
}

func TestValidateRejectsMutatedCeilingAndOpenAccounting(t *testing.T) {
	valid := Project(fixture(t))
	bad := valid
	bad.Authority = 1
	if err := Validate(bad); err == nil {
		t.Fatal("ASSERT_CAPTURE_SET_INSPECTION_REJECTS_AUTHORITY")
	}
	bad = valid
	bad.Files.Denominator++
	if err := Validate(bad); err == nil {
		t.Fatal("ASSERT_CAPTURE_SET_INSPECTION_REJECTS_OPEN_LEDGER")
	}
}

func contains(raw []byte, value string) bool {
	for i := 0; i+len(value) <= len(raw); i++ {
		if string(raw[i:i+len(value)]) == value {
			return true
		}
	}
	return false
}
