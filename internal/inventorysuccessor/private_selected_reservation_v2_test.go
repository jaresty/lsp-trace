package inventorysuccessor

import (
	"math"
	"testing"
)

func selectedAllCountersTestV2(n uint64) countersV2 {
	var v [14]uint64
	for i := range v {
		v[i] = n
	}
	return countersFromV2(v)
}

// Pure algebra only: no kernel/root/job/grant fixtures or native publication.
func TestSelectedV2FutureFunding(t *testing.T) {
	p := resourceHeadV2{Windows: 7, Charged: selectedAllCountersTestV2(4)}
	a := authorizationV2{Ceiling: selectedAllCountersTestV2(10), MaxTranche: selectedAllCountersTestV2(3), MaxWindows: 10, Expires: 3, MaxWindowNS: 1}
	stages := [3]countersV2{selectedAllCountersTestV2(1), selectedAllCountersTestV2(2), selectedAllCountersTestV2(3)}
	if e := selectedFutureFitsV2(p, a, stages, 1, 2); e != nil {
		t.Fatal("exact cumulative funding", e)
	}
	for i := 0; i < 14; i++ {
		short := a
		v := short.Ceiling.vector()
		v[i]--
		short.Ceiling = countersFromV2(v)
		if selectedFutureFitsV2(p, short, stages, 1, 2) == nil {
			t.Fatalf("ASSERT_SELECTED_FUTURE: ceiling coordinate %d ignored", i)
		}
		short = a
		v = short.MaxTranche.vector()
		v[i]--
		short.MaxTranche = countersFromV2(v)
		if selectedFutureFitsV2(p, short, stages, 1, 2) == nil {
			t.Fatalf("ASSERT_SELECTED_FUTURE: future tranche coordinate %d ignored", i)
		}
		overflow := p
		v = overflow.Charged.vector()
		v[i] = math.MaxUint64
		overflow.Charged = countersFromV2(v)
		if selectedFutureFitsV2(overflow, a, stages, 1, 2) == nil {
			t.Fatal("cumulative overflow")
		}
	}
	short := a
	short.MaxWindows = 9
	if selectedFutureFitsV2(p, short, stages, 1, 2) == nil {
		t.Fatal("ASSERT_SELECTED_FUTURE: three windows not checked")
	}
	overflow := p
	overflow.Windows = math.MaxUint64
	if selectedFutureFitsV2(overflow, a, stages, 1, 2) == nil {
		t.Fatal("window overflow")
	}
	short = a
	short.Expires = 1
	if selectedFutureFitsV2(p, short, stages, 1, 2) == nil {
		t.Fatal("expiry ignored")
	}
	short = a
	short.MaxWindowNS = 0
	if selectedFutureFitsV2(p, short, stages, 1, 2) == nil {
		t.Fatal("duration ignored")
	}
	t.Log("ASSERT_SELECTED_FUTURE: PASS")
}

func TestSelectedV2PlanRefusesBeforeNative(t *testing.T) {
	if _, e := NewPrivateSelectedPreparationPlanV2(nil, PrivateMixedInputRequestV2{}, PrivateSelectedConfigurationV2{}); e == nil {
		t.Fatal("zero owner plan")
	}
	var owner *PrivateManagedInputOwnerV2
	if _, e := owner.ReserveSelectedMixedInputV2(nil, "", PrivateMixedInputRequestV2{}, PrivateSelectedPreparationPlanV2{}); e == nil {
		t.Fatal("zero reserve plan")
	}
}
