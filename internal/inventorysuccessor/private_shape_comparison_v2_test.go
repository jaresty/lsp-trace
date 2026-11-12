package inventorysuccessor

import "testing"

// Synthetic policy metadata only: no original nomination, native producer,
// selected endpoint, direction, evidence state or admission is represented.
func syntheticShapeKindsV2() [7]shapeKindsInputV2 {
	return [7]shapeKindsInputV2{
		{"C", [3]string{"CALLS"}, 1},
		{"R", [3]string{"REFERENCES_SYMBOL"}, 1},
		{"D", [3]string{"RESOLVES_TO_DEFINITION"}, 1},
		{"CR", [3]string{"CALLS", "REFERENCES_SYMBOL"}, 2},
		{"CD", [3]string{"CALLS", "RESOLVES_TO_DEFINITION"}, 2},
		{"RD", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, 2},
		{"CRD", [3]string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, 3},
	}
}

func TestPrivateShapeComparisonV2SevenAndDirectedCrossShape(t *testing.T) {
	positive, cross := 0, 0
	for i, v := range syntheticShapeKindsV2() {
		d, err := newShapeComparisonV2(v)
		if err != nil || !d.sameShape(v) || !d.sameProducerOrder(v) || d.snapshot() != v {
			t.Fatalf("positive %s rejected", v.Mode)
		}
		positive++
		for j, other := range syntheticShapeKindsV2() {
			if i == j {
				continue
			}
			t.Run(v.Mode+"-rejects-"+other.Mode, func(t *testing.T) {
				if d.sameShape(other) || d.sameProducerOrder(other) {
					t.Fatal("cross-shape accepted")
				}
			})
			cross++
		}
	}
	if positive != 7 || cross != 42 {
		t.Fatalf("counts positive=%d cross=%d", positive, cross)
	}
	t.Logf("asserted positives=%d directed-cross-shape=%d", positive, cross)
}

func TestPrivateShapeComparisonV2PermutationsAndCopies(t *testing.T) {
	// Explicit complete legal permutations: three singles, six pairs, six triples.
	perms := [6][3]int{{0, 1, 2}, {1, 0, 2}, {0, 2, 1}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	count, reordered := 0, 0
	for _, base := range syntheticShapeKindsV2() {
		n := 1
		if base.KindCount == 2 {
			n = 2
		}
		if base.KindCount == 3 {
			n = 6
		}
		canonical, err := newShapeComparisonV2(base)
		if err != nil {
			t.Fatal(err)
		}
		for p := 0; p < n; p++ {
			v := base
			for i := 0; i < int(v.KindCount); i++ {
				v.Kinds[i] = base.Kinds[perms[p][i]]
			}
			before := v
			d, err := newShapeComparisonV2(v)
			if err != nil {
				t.Fatal("legal permutation rejected", v)
			}
			if d.comparisonKinds() != base.Kinds {
				t.Fatal("normalized copy differs", v.Mode, p)
			}
			if !canonical.sameShape(v) || !d.sameShape(base) {
				t.Fatal("legal permutation shape mismatch")
			}
			wantExact := v == base
			if canonical.sameProducerOrder(v) != wantExact || d.sameProducerOrder(base) != wantExact {
				t.Fatal("producer-order distinction lost")
			}
			if !wantExact {
				reordered++
			}
			if v != before || d.snapshot() != before || !d.sameProducerOrder(before) {
				t.Fatal("constructor/input/order mutated")
			}
			v.Kinds[0] = "changed-input"
			out := d.snapshot()
			out.Mode, out.Kinds[0], out.KindCount = "changed-output", "changed-output", 255
			normalized := d.comparisonKinds()
			normalized[0] = "changed-normalized"
			copy := d
			if copy.snapshot() != before || d.snapshot() != before || !d.sameProducerOrder(before) || d.comparisonKinds() != base.Kinds {
				t.Fatal("copy alias mutated retained value")
			}
			count++
		}
	}
	if count != 15 || reordered != 8 {
		t.Fatalf("permutation counts=%d reordered=%d", count, reordered)
	}
	t.Logf("asserted legal-permutations=%d reordered=%d", count, reordered)
}

func TestPrivateShapeComparisonV2RejectRawMetadata(t *testing.T) {
	cases := []struct {
		name string
		v    shapeKindsInputV2
	}{
		{"zero", shapeKindsInputV2{}},
		{"empty-mode", shapeKindsInputV2{"", [3]string{"CALLS"}, 1}},
		{"unknown-mode", shapeKindsInputV2{"X", [3]string{"CALLS"}, 1}},
		{"mode-case", shapeKindsInputV2{"c", [3]string{"CALLS"}, 1}},
		{"mode-whitespace", shapeKindsInputV2{" C", [3]string{"CALLS"}, 1}},
		{"empty-set", shapeKindsInputV2{"C", [3]string{}, 0}},
		{"empty-actual", shapeKindsInputV2{"C", [3]string{}, 1}},
		{"zero-count", shapeKindsInputV2{"C", [3]string{"CALLS"}, 0}},
		{"count-four", shapeKindsInputV2{"CRD", [3]string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, 4}},
		{"count-overflow", shapeKindsInputV2{"C", [3]string{"CALLS"}, 255}},
		{"unknown-kind", shapeKindsInputV2{"C", [3]string{"UNKNOWN"}, 1}},
		{"kind-case", shapeKindsInputV2{"C", [3]string{"calls"}, 1}},
		{"duplicate-single", shapeKindsInputV2{"C", [3]string{"CALLS", "CALLS"}, 2}},
		{"duplicate-pair", shapeKindsInputV2{"CR", [3]string{"CALLS", "CALLS"}, 2}},
		{"duplicate-same-set", shapeKindsInputV2{"RD", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION", "REFERENCES_SYMBOL"}, 3}},
		{"duplicate-triple", shapeKindsInputV2{"CRD", [3]string{"CALLS", "REFERENCES_SYMBOL", "REFERENCES_SYMBOL"}, 3}},
		{"missing-kind", shapeKindsInputV2{"CR", [3]string{"CALLS"}, 1}},
		{"excess-kind", shapeKindsInputV2{"C", [3]string{"CALLS", "REFERENCES_SYMBOL"}, 2}},
		{"nonempty-tail-one", shapeKindsInputV2{"C", [3]string{"CALLS", "REFERENCES_SYMBOL"}, 1}},
		{"nonempty-tail-two", shapeKindsInputV2{"C", [3]string{"CALLS", "", "UNKNOWN"}, 1}},
		{"nonempty-tail-pair", shapeKindsInputV2{"RD", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION", "CALLS"}, 2}},
		{"hole", shapeKindsInputV2{"CRD", [3]string{"CALLS", "", "RESOLVES_TO_DEFINITION"}, 3}},
		{"mode-kind-mismatch", shapeKindsInputV2{"R", [3]string{"CALLS"}, 1}},
		{"admission-universe-not-R", shapeKindsInputV2{"R", [3]string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.v
			d, err := newShapeComparisonV2(tc.v)
			if err != errShapeComparisonV2 || d != (shapeComparisonV2{}) || d.snapshot() != (shapeKindsInputV2{}) || d.comparisonKinds() != ([3]string{}) {
				t.Fatal("invalid metadata not closed")
			}
			if tc.v != before {
				t.Fatal("invalid input mutated")
			}
			for _, valid := range syntheticShapeKindsV2() {
				good, err := newShapeComparisonV2(valid)
				if err != nil {
					t.Fatal(err)
				}
				if good.sameShape(tc.v) || good.sameProducerOrder(tc.v) || d.sameShape(valid) || d.sameProducerOrder(valid) {
					t.Fatal("invalid metadata compared equal")
				}
			}
		})
	}
	t.Logf("asserted invalid-raw-cases=%d", len(cases))
}

func TestPrivateShapeComparisonV2ExhaustiveRawDomain(t *testing.T) {
	// Independent finite oracle: enumerate raw vectors, never normalize first.
	// Includes known kinds, empty/unknown, all counts through overflow boundary,
	// all seven modes and unknown/empty modes (5^3 * 6 * 9 = 6750 cases).
	values := [5]string{"", "CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION", "UNKNOWN"}
	counts := [6]uint8{0, 1, 2, 3, 4, 255}
	modes := [9]string{"C", "R", "D", "CR", "CD", "RD", "CRD", "", "X"}
	checked, accepted := 0, 0
	for _, mode := range modes {
		for _, n := range counts {
			for _, a := range values {
				for _, b := range values {
					for _, c := range values {
						v := shapeKindsInputV2{mode, [3]string{a, b, c}, n}
						want := false
						for _, base := range syntheticShapeKindsV2() {
							if mode != base.Mode || n != base.KindCount {
								continue
							}
							matches := 0
							for i := 0; i < int(n); i++ {
								occurrences := 0
								for j := 0; j < int(n); j++ {
									if v.Kinds[j] == base.Kinds[i] {
										occurrences++
									}
								}
								if occurrences == 1 {
									matches++
								}
							}
							tailEmpty := true
							for i := int(n); i < 3; i++ {
								if v.Kinds[i] != "" {
									tailEmpty = false
								}
							}
							want = matches == int(n) && tailEmpty
						}
						d, err := newShapeComparisonV2(v)
						if (err == nil) != want {
							t.Fatalf("raw-domain classification differs: %+v want=%v", v, want)
						}
						if want {
							accepted++
						} else if d != (shapeComparisonV2{}) {
							t.Fatal("invalid domain snapshot nonzero")
						}
						checked++
					}
				}
			}
		}
	}
	if checked != 6750 || accepted != 15 {
		t.Fatalf("domain counts=%d accepted=%d", checked, accepted)
	}
	t.Logf("asserted raw-domain=%d accepted=%d", checked, accepted)
}
