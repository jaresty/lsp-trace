package targetpacket

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

func adr0007BuiltPacket(t *testing.T) (Packet, []byte) {
	t.Helper()
	fixture := buildV3PacketFixture(t, 2)
	result, err := Build(fixture.request)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if result.State != StatePrepared || len(result.Packets) != 1 {
		t.Fatalf("ASSERT_ADR0007_BUILD_PREPARES_ONE_PACKET: %+v", result)
	}
	var v3 v5sourcesnapshotv3.Artifact
	if err := json.Unmarshal(fixture.request.Snapshots[0].Raw, &v3); err != nil {
		t.Fatalf("decode v3: %v", err)
	}
	var v2 v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(v3.ParentSnapshot, &v2); err != nil {
		t.Fatalf("decode v2: %v", err)
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(v2.ParentSnapshot, &v1); err != nil {
		t.Fatalf("decode v1: %v", err)
	}
	return result.Packets[0], append([]byte(nil), v1.GraphV5Bytes...)
}

func adr0007SetCustody(packet *Packet, custody retainedprojection.RetainedCustodyBinding) {
	packet.Custody = custody
	packet.Projection.CustodyBinding = custody
	for i := range packet.ConsumerAlternatives {
		packet.ConsumerAlternatives[i].Custody = custody
		packet.ConsumerAlternatives[i].ReconciliationID = consumerReconciliationID(packet.ConsumerAlternatives[i])
	}
}

func adr0007Encode(t *testing.T, packet Packet) []byte {
	t.Helper()
	raw, err := EncodeCanonical(packet)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return raw
}

func adr0007RequireBaseline(t *testing.T, packet Packet) []byte {
	t.Helper()
	raw := adr0007Encode(t, packet)
	if _, err := Validate(raw); err != nil {
		t.Fatalf("ASSERT_ADR0007_BASELINE_PACKET_SEMANTICS: %v", err)
	}
	return raw
}

func TestADR0007BuildUsesGraphProvenanceCustody(t *testing.T) {
	packet, _ := adr0007BuiltPacket(t)
	if packet.Custody.GraphSchemaID != retainedprojection.GraphProvenanceV5SchemaID {
		t.Fatalf("ASSERT_ADR0007_BUILD_USES_GRAPH_PROVENANCE_CUSTODY: got=%q want=%q", packet.Custody.GraphSchemaID, retainedprojection.GraphProvenanceV5SchemaID)
	}
}

func TestADR0007ByteBackedCustodyAdmission(t *testing.T) {
	t.Run("valid_outer_bytes", func(t *testing.T) {
		packet, graphBytes := adr0007BuiltPacket(t)
		raw := adr0007RequireBaseline(t, packet)
		if _, err := ValidateWithGraph(raw, graphBytes); err != nil {
			t.Fatalf("ASSERT_ADR0007_BYTE_ADMISSION_ACCEPTS_VALID_OUTER_BYTES: %v", err)
		}
	})

	t.Run("wrong_digest", func(t *testing.T) {
		packet, graphBytes := adr0007BuiltPacket(t)
		_ = adr0007RequireBaseline(t, packet)
		custody := packet.Custody
		custody.GraphDigest = "sha256:" + strings.Repeat("0", 64)
		adr0007SetCustody(&packet, custody)
		raw := adr0007Encode(t, packet)
		if _, err := Validate(raw); err != nil {
			t.Fatalf("ASSERT_ADR0007_WRONG_DIGEST_REMAINS_TUPLE_VALID: %v", err)
		}
		if _, err := ValidateWithGraph(raw, graphBytes); err == nil || err.Error() != "graph custody digest mismatch" {
			t.Fatalf("ASSERT_ADR0007_BYTE_ADMISSION_REPORTS_EXACT_CAUSE: got=%v want=graph custody digest mismatch", err)
		}
	})

	t.Run("wrong_length", func(t *testing.T) {
		packet, graphBytes := adr0007BuiltPacket(t)
		_ = adr0007RequireBaseline(t, packet)
		custody := packet.Custody
		custody.GraphByteLength++
		adr0007SetCustody(&packet, custody)
		raw := adr0007Encode(t, packet)
		if _, err := Validate(raw); err != nil {
			t.Fatalf("ASSERT_ADR0007_WRONG_LENGTH_REMAINS_TUPLE_VALID: %v", err)
		}
		if _, err := ValidateWithGraph(raw, graphBytes); err == nil || err.Error() != "graph custody byte length mismatch" {
			t.Fatalf("ASSERT_ADR0007_BYTE_ADMISSION_REPORTS_EXACT_CAUSE: got=%v want=graph custody byte length mismatch", err)
		}
	})

	t.Run("same_length_byte_substitution", func(t *testing.T) {
		packet, graphBytes := adr0007BuiltPacket(t)
		raw := adr0007RequireBaseline(t, packet)
		substituted := append([]byte(nil), graphBytes...)
		substituted[len(substituted)/2] ^= 1
		if len(substituted) != len(graphBytes) {
			t.Fatal("ASSERT_ADR0007_SUBSTITUTION_PRESERVES_LENGTH")
		}
		if _, err := ValidateWithGraph(raw, substituted); err == nil || err.Error() != "graph custody digest mismatch" {
			t.Fatalf("ASSERT_ADR0007_BYTE_ADMISSION_REPORTS_EXACT_CAUSE: got=%v want=graph custody digest mismatch", err)
		}
	})

	t.Run("coherent_hash_invalid_envelope", func(t *testing.T) {
		packet, _ := adr0007BuiltPacket(t)
		_ = adr0007RequireBaseline(t, packet)
		invalid := []byte(`{}`)
		sum := sha256.Sum256(invalid)
		custody := packet.Custody
		custody.GraphDigest = "sha256:" + hex.EncodeToString(sum[:])
		custody.GraphByteLength = uint64(len(invalid))
		adr0007SetCustody(&packet, custody)
		raw := adr0007Encode(t, packet)
		if _, err := Validate(raw); err != nil {
			t.Fatalf("ASSERT_ADR0007_INVALID_ENVELOPE_REMAINS_TUPLE_VALID: %v", err)
		}
		if _, err := ValidateWithGraph(raw, invalid); err == nil || !strings.HasPrefix(err.Error(), "graph custody envelope:") {
			t.Fatalf("ASSERT_ADR0007_BYTE_ADMISSION_REPORTS_EXACT_CAUSE: got=%v want-prefix=graph custody envelope:", err)
		}
	})

	t.Run("inner_schema_discriminator", func(t *testing.T) {
		packet, graphBytes := adr0007BuiltPacket(t)
		_ = adr0007RequireBaseline(t, packet)
		custody := packet.Custody
		custody.GraphSchemaID = graphprovenance.GraphV5SchemaID
		adr0007SetCustody(&packet, custody)
		raw := adr0007Encode(t, packet)
		if _, err := Validate(raw); err == nil || err.Error() != "packet semantic invariant mismatch" {
			t.Fatalf("ASSERT_ADR0007_PACKET_REJECTS_INNER_SCHEMA: got=%v want=packet semantic invariant mismatch", err)
		}
		if err := retainedprojection.ValidateGraphCustody(custody, graphBytes); err == nil || err.Error() != "graph custody schema mismatch" {
			t.Fatalf("ASSERT_ADR0007_DIRECT_SCHEMA_CHECK_REPORTS_EXACT_CAUSE: got=%v want=graph custody schema mismatch", err)
		}
	})
}

func TestADR0007SyntheticCorrectionDoesNotMutatePredecessor(t *testing.T) {
	packet, graphBytes := adr0007BuiltPacket(t)
	validRaw := adr0007RequireBaseline(t, packet)

	var historical Packet
	if err := json.Unmarshal(validRaw, &historical); err != nil {
		t.Fatal(err)
	}
	historicalCustody := historical.Custody
	historicalCustody.GraphSchemaID = graphprovenance.GraphV5SchemaID
	adr0007SetCustody(&historical, historicalCustody)
	historicalRaw := adr0007Encode(t, historical)

	var predecessor, corrected Packet
	if err := json.Unmarshal(historicalRaw, &predecessor); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(historicalRaw, &corrected); err != nil {
		t.Fatal(err)
	}
	correctedCustody := corrected.Custody
	correctedCustody.GraphSchemaID = retainedprojection.GraphProvenanceV5SchemaID
	adr0007SetCustody(&corrected, correctedCustody)
	correctedRaw := adr0007Encode(t, corrected)
	validatedCorrected, err := ValidateWithGraph(correctedRaw, graphBytes)
	if err != nil {
		t.Fatalf("ASSERT_ADR0007_SYNTHETIC_CORRECTION_IS_VALID: %v", err)
	}

	predecessorRaw := adr0007Encode(t, predecessor)
	if !bytes.Equal(predecessorRaw, historicalRaw) {
		t.Fatal("ASSERT_ADR0007_SYNTHETIC_PREDECESSOR_NOT_MUTATED")
	}
	var encodedPredecessor Packet
	if err := json.Unmarshal(predecessorRaw, &encodedPredecessor); err != nil {
		t.Fatal(err)
	}
	if encodedPredecessor.PacketID == validatedCorrected.PacketID {
		t.Fatal("ASSERT_ADR0007_SYNTHETIC_VARIANTS_HAVE_DISTINCT_CANONICAL_IDENTITIES")
	}
	t.Logf("ASSERT_ADR0007_LOCAL_SYNTHETIC_LINK: predecessor_packet_id=%s corrected_packet_id=%s relationship=local-test-variant-only", encodedPredecessor.PacketID, validatedCorrected.PacketID)
}
