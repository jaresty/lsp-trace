package programcrepresentative

import (
	"fmt"
	"strings"
	"testing"
)

func TestMultiSeedV2OutputAdmissionExactCap(t *testing.T) {
	t.Log("procedure: exact typed selected-record JSON size at 64 MiB admits; +1 refuses before DTO-copy/serialization stage; checked uint64 arithmetic refuses overflow")
	r := RepresentativeRecord{Outcome: "SELECTED", SelectedNode: "target", SCCMembers: []string{"target"}, IncomingPredecessors: []RepresentativePredecessor{}}
	html := strings.Repeat("<", MaxStringBytesV2)
	for i := 0; i < 11; i++ {
		caller := html
		if i == 10 {
			caller = ""
		}
		r.IncomingPredecessors = append(r.IncomingPredecessors, RepresentativePredecessor{OccurrenceID: fmt.Sprintf("occurrence-%02d", i), RelationID: fmt.Sprintf("relation-%02d", i), CallerID: caller, TargetID: "target"})
	}
	dto := RecordV2{Outcome: r.Outcome, SelectedNode: r.SelectedNode, SCCMembers: r.SCCMembers, IncomingPredecessors: []PredecessorV2{}}
	for _, p := range r.IncomingPredecessors {
		dto.IncomingPredecessors = append(dto.IncomingPredecessors, PredecessorV2{OccurrenceID: p.OccurrenceID, RelationID: p.RelationID, CallerID: p.CallerID, TargetID: p.TargetID})
	}
	base, err := CanonicalSizeRecordV2(dto)
	if err != nil || base >= MaxArtifactCanonicalBytesV2 {
		t.Fatal("CAP_FIXTURE_BASE_ADMISSION")
	}
	remaining := MaxArtifactCanonicalBytesV2 - base
	padding := strings.Repeat("<", int(remaining/6)) + strings.Repeat("z", int(remaining%6))
	if len(padding) > MaxStringBytesV2 {
		t.Fatal("CAP_FIXTURE_STRING_BOUND")
	}
	r.IncomingPredecessors[10].CallerID = padding
	dto.IncomingPredecessors[10].CallerID = padding
	n, err := CanonicalSizeRecordV2(dto)
	if err != nil || n != MaxArtifactCanonicalBytesV2 || admitSelectedRecordCopyV2(r) != nil {
		t.Fatalf("TYPED_RECORD_CAP_EQUALITY: n=%d err=%v", n, err)
	}
	r.IncomingPredecessors[10].CallerID += "z"
	if err := admitSelectedRecordCopyV2(r); err == nil {
		t.Fatal("TYPED_RECORD_CAP_PLUS_ONE_REFUSAL")
	}
	if _, err := checkedSumV2(^uint64(0), 1); err == nil {
		t.Fatal("CHECKED_OUTPUT_ADMISSION_SUM_OVERFLOW")
	}
	if _, err := checkedProductV2(^uint64(0), 2); err == nil {
		t.Fatal("CHECKED_OUTPUT_ADMISSION_PRODUCT_OVERFLOW")
	}
}
