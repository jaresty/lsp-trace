package inventorysuccessor

import "errors"

// shapeKindsInputV2 is selected policy metadata, NOT InputEligibleKinds (the
// admission universe). Mode is a census shape, never ACQUIRE/ISSUE/REPLAY.
// Kinds and KindCount retain the PrivateManagedInputBindingV2 field types.
// Arbitrary caller labels are data: coherence does not prove native policy.
type shapeKindsInputV2 struct {
	Mode      string
	Kinds     [3]string
	KindCount uint8
}

// shapeComparisonV2 is a pure, nonadmitting value snapshot. It proves neither
// nomination correspondence nor selected endpoints, CALLS incoming/outgoing or
// endpoint coverage. Unknown, observed-empty/nonempty, excluded and missing
// evidence states are not encoded here and cannot imply presence or absence.
// Original source/definition handles, whole-record Resume joins and native
// funding remain future stage gates. No serializer or integration is provided.
// The frozen RD descriptor additionally compares its full nomination tuple;
// future reuse requires identical behavior, not replacement by this kind check.
type shapeComparisonV2 struct {
	value shapeKindsInputV2
	valid bool
}

var errShapeComparisonV2 = errors.New("private shape comparison refused")

func newShapeComparisonV2(v shapeKindsInputV2) (shapeComparisonV2, error) {
	if !validShapeKindsV2(v) {
		return shapeComparisonV2{}, errShapeComparisonV2
	}
	return shapeComparisonV2{value: v, valid: true}, nil
}

func validShapeKindsV2(v shapeKindsInputV2) bool {
	// Exact policy sets from censuscontinuation.privateAllModeKinds, not the
	// input admission universe. The private policy helper cannot be imported;
	// no handoff, opaque input or schedule is minted just to compare data.
	var expected uint8
	switch v.Mode {
	case "C":
		expected = 1
	case "R":
		expected = 2
	case "D":
		expected = 4
	case "CR":
		expected = 3
	case "CD":
		expected = 5
	case "RD":
		expected = 6
	case "CRD":
		expected = 7
	default:
		return false
	}
	if v.KindCount == 0 || v.KindCount > 3 {
		return false
	}
	var seen uint8
	for i, k := range v.Kinds {
		if i >= int(v.KindCount) {
			if k != "" {
				return false
			}
			continue
		}
		var bit uint8
		switch k {
		case "CALLS":
			bit = 1
		case "REFERENCES_SYMBOL":
			bit = 2
		case "RESOLVES_TO_DEFINITION":
			bit = 4
		default:
			return false
		}
		if seen&bit != 0 {
			return false
		}
		seen |= bit
	}
	return seen == expected
}

// sameShape ignores only legal producer permutations. Both operands must be
// coherent raw metadata; sorting/deduplication never repairs an invalid input.
func (d shapeComparisonV2) sameShape(v shapeKindsInputV2) bool {
	return d.valid && validShapeKindsV2(v) && d.value.Mode == v.Mode
}

// sameProducerOrder is exact kind-only replay correspondence, NOT the RD
// descriptor's full nomination/intents correspondence or native authority.
func (d shapeComparisonV2) sameProducerOrder(v shapeKindsInputV2) bool {
	return d.valid && validShapeKindsV2(v) && d.value == v
}

func (d shapeComparisonV2) comparisonKinds() [3]string {
	if !d.valid {
		return [3]string{}
	}
	// Sort the occupied prefix of a value copy; the unused tail stays empty.
	k := d.value.Kinds
	for i := 1; i < int(d.value.KindCount); i++ {
		for j := i; j > 0 && k[j] < k[j-1]; j-- {
			k[j], k[j-1] = k[j-1], k[j]
		}
	}
	return k
}

func (d shapeComparisonV2) snapshot() shapeKindsInputV2 {
	if !d.valid {
		return shapeKindsInputV2{}
	}
	return d.value
}
