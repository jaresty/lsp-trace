package inventorysuccessor

import (
	rg "lsp-trace/internal/referencegrouping"
	"reflect"
	"testing"
	"time"
)

func TestSourceLocalScalarComposition(t *testing.T) {
	now := time.Unix(1000000000, 0)
	m := rg.AllModeMetadataPrivate{}
	m.Input.ByteLength = 1
	m.Contributions = 1
	base, err := sourceRPreparationChargeV1(m, now, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	// All coordinates are distinct, so omission/reuse of any old credit is visible.
	local := countersFromV2([14]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14})
	joined, err := sourceLocalChargeV1(m, now, now.Add(time.Second), local)
	want, addErr := addCountersV2(base, local)
	if err != nil || addErr != nil || joined != want {
		t.Fatalf("ASSERT_LOCAL_COMPLETE_JOIN_BEFORE_EFFECT: got=%+v want=%+v err=%v", joined, want, err)
	}
	t.Log("ASSERT_LOCAL_COMPLETE_JOIN_BEFORE_EFFECT: PASS (scalar composition only; no reservation or execution)")
}

func TestSourceLocalReservationOverflow(t *testing.T) {
	now := time.Unix(1000000000, 0)
	m := rg.AllModeMetadataPrivate{}
	m.Input.ByteLength = 1
	m.Contributions = 1
	base, err := sourceRPreparationChargeV1(m, now, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 14; i++ {
		if reflect.ValueOf(base).Field(i).Uint() == 0 {
			continue
		}
		var coordinates [14]uint64
		coordinates[i] = ^uint64(0)
		got, err := sourceLocalChargeV1(m, now, now.Add(time.Second), countersFromV2(coordinates))
		if err == nil || got != (countersV2{}) {
			t.Fatalf("ASSERT_LOCAL_JOIN_OVERFLOW_REFUSED: coordinate=%d got=%+v err=%v", i, got, err)
		}
	}
	if got, err := sourceLocalChargeV1(m, now, now, countersV2{}); err == nil || got != (countersV2{}) {
		t.Fatal("ASSERT_LOCAL_JOIN_ORIGINAL_DEADLINE_REFUSED", got, err)
	}
	t.Log("ASSERT_LOCAL_JOIN_OVERFLOW_REFUSED: PASS; ASSERT_LOCAL_JOIN_ORIGINAL_DEADLINE_REFUSED: PASS (scalar only)")
}
