package describerequest

import (
	"testing"

	"lsp-trace/internal/targetpacket"
)

func TestValidateRecordSecondAlternativeRetainsIntegrity(t *testing.T) {
	p := targetpacket.Packet{PacketID: "packet", CensusID: "census", ConsumerResolution: targetpacket.ConsumerResolved}
	p.Custody.GraphDigest = "sha256:abcd"
	first, err := makeRecord(p, "alternative-0", 0, "first prompt", 90000)
	if err != nil {
		t.Fatal(err)
	}
	second, err := makeRecord(p, "alternative-1", 1, "second prompt", 90000)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate([]Record{first, second}); err != nil {
		t.Fatalf("ASSERT_BATCH_ORDERED_ALTERNATIVES: %v", err)
	}
	if err := ValidateRecord(second); err != nil {
		t.Fatalf("ASSERT_VALID_SECOND_ALTERNATIVE_INDIVIDUALLY: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Record)
	}{
		{"prompt", func(r *Record) { r.Envelope.Prompt += " forged" }},
		{"record_id", func(r *Record) { r.RecordID = "forged" }},
		{"lineage", func(r *Record) { r.Lineage.LineageIdentity = "forged" }},
		{"negative_ordinal", func(r *Record) { r.Lineage.AlternativeOrdinal = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := second
			tc.mutate(&bad)
			if err := ValidateRecord(bad); err == nil {
				t.Fatal("ASSERT_FORGED_INDIVIDUAL_REJECTED: accepted")
			}
		})
	}
	if err := Validate([]Record{second}); err == nil {
		t.Fatal("ASSERT_BATCH_STILL_REJECTS_ORPHAN_SECOND_ALTERNATIVE: accepted")
	}
}
