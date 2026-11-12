package inventorysuccessor

import (
	cc "lsp-trace/internal/censuscontinuation"
	rg "lsp-trace/internal/referencegrouping"
	tp "lsp-trace/internal/targetpacket"
	"reflect"
	"strings"
	"testing"
)

// Entirely synthetic comparison data: no retained seed7 nomination or native
// producer/source qualification is represented by this fixture.
func syntheticRDDescriptorV2(t *testing.T) rdDescriptorInputV2 {
	t.Helper()
	n := tp.PrivateNomination{
		Status: "PROVISIONAL_STRUCTURAL_HYPOTHESIS", ClaimCeiling: rg.EvaluationCeilingPrivate,
		RejectionCondition: "SPLIT_OR_WITHDRAW_IF_ADMITTED_SOURCE_REVIEW_CONTRADICTS_MEMBERSHIP",
		ID:                 "synthetic-nomination", Mode: "RD", RunID: "synthetic-run", InputID: "synthetic-input", CaseID: "synthetic-case", PolicyID: "synthetic-policy", ResultID: "synthetic-result", Lineage: "synthetic-lineage",
		Seed: 7, Slot: 0, Community: 1, Members: []string{"synthetic-query", "synthetic-definition"},
		Endpoints: []tp.PrivateTypedEndpoint{
			{Node: "synthetic-query", Kind: "REFERENCES_SYMBOL", Occurrence: "synthetic-reference", Role: "REFERENCING_OCCURRENCE", Predecessor: "synthetic-r-predecessor", Custody: "synthetic-r-custody", Disposition: "synthetic-disposition", Ordinal: 2},
			{Node: "synthetic-definition", Kind: "RESOLVES_TO_DEFINITION", Occurrence: "synthetic-definition-occurrence", Role: "DEFINITION_TARGET", Predecessor: "synthetic-d-predecessor", Custody: "synthetic-d-custody", Disposition: "synthetic-disposition", Ordinal: 3},
		},
	}
	intents, err := tp.BuildPrivateSourceIntents(n, 2)
	if err != nil {
		t.Fatal(err)
	}
	return rdDescriptorInputV2{Slot: cc.PrivateAllModeSlot{Ordinal: 0, Mode: "RD", RunID: n.RunID, SeedNamespace: "synthetic-seed-namespace", Seed: 7}, Nomination: n, Intent: intents[1], Kinds: [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION", ""}, KindCount: 2}
}

func TestPrivateRDDescriptorV2KindsAndSnapshot(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "RD-order", true: "DR-order"}[reverse], func(t *testing.T) {
			v := syntheticRDDescriptorV2(t)
			if reverse {
				v.Kinds[0], v.Kinds[1] = v.Kinds[1], v.Kinds[0]
			}
			before := syntheticRDDescriptorV2(t)
			before.Kinds = v.Kinds
			d, err := newRDDescriptorV2(v)
			if err != nil {
				t.Fatal("valid synthetic RD rejected:", err)
			}
			if d.comparisonKinds() != [2]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
				t.Fatal("comparison normalization")
			}
			if !reflect.DeepEqual(v, before) {
				t.Fatal("constructor mutated input")
			}
			if !reflect.DeepEqual(d.snapshot(), before) || !d.matches(before) {
				t.Fatal("exact producer order not preserved")
			}
			reordered := syntheticRDDescriptorV2(t)
			reordered.Kinds = before.Kinds
			reordered.Kinds[0], reordered.Kinds[1] = reordered.Kinds[1], reordered.Kinds[0]
			if d.matches(reordered) {
				t.Fatal("replay order mismatch accepted")
			}
			v.Nomination.Members[0] = "changed"
			v.Nomination.Endpoints[0].Predecessor = "changed"
			if !d.matches(before) {
				t.Fatal("input alias changed snapshot")
			}
			returned := d.snapshot()
			returned.Nomination.Members[0] = "changed"
			returned.Nomination.Endpoints[0].Role = "changed"
			if !d.matches(before) {
				t.Fatal("output alias changed snapshot")
			}
			// Copying the descriptor carries only comparison data, not a new grant.
			copy := d
			if !copy.matches(before) {
				t.Fatal("value copy lost comparison")
			}
		})
	}
}

func TestPrivateRDDescriptorV2KindRejection(t *testing.T) {
	cases := []struct {
		name  string
		kinds [3]string
		count uint8
	}{
		{"empty", [3]string{}, 0}, {"nil-equivalent", [3]string{}, 2},
		{"R-only", [3]string{"REFERENCES_SYMBOL"}, 1},
		{"D-only", [3]string{"RESOLVES_TO_DEFINITION"}, 1},
		{"unknown", [3]string{"REFERENCES_SYMBOL", "UNKNOWN"}, 2},
		{"CALLS", [3]string{"CALLS", "RESOLVES_TO_DEFINITION"}, 2},
		{"duplicate-R", [3]string{"REFERENCES_SYMBOL", "REFERENCES_SYMBOL"}, 2},
		{"duplicate-D", [3]string{"RESOLVES_TO_DEFINITION", "RESOLVES_TO_DEFINITION"}, 2},
		{"duplicate-same-set", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION", "REFERENCES_SYMBOL"}, 3},
		{"tail", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION", "CALLS"}, 2},
		{"missing-count", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, 1},
		{"overflow-count", [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := syntheticRDDescriptorV2(t)
			v.Kinds = tc.kinds
			v.KindCount = tc.count
			d, err := newRDDescriptorV2(v)
			if err != errRDDescriptorV2 || !reflect.DeepEqual(d, rdDescriptorV2{}) {
				t.Fatal("invalid kinds accepted or nonclosed error")
			}
		})
	}
}

func TestPrivateRDDescriptorV2BindingPerturbations(t *testing.T) {
	// Every field of the selected bounded projection is changed independently.
	// Some mutations remain coherent comparison data; none may match the original.
	base := syntheticRDDescriptorV2(t)
	d, err := newRDDescriptorV2(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*rdDescriptorInputV2){
		"slot.ordinal":   func(v *rdDescriptorInputV2) { v.Slot.Ordinal++ },
		"slot.mode":      func(v *rdDescriptorInputV2) { v.Slot.Mode = "R" },
		"slot.run":       func(v *rdDescriptorInputV2) { v.Slot.RunID += "x" },
		"slot.namespace": func(v *rdDescriptorInputV2) { v.Slot.SeedNamespace += "x" },
		"slot.seed":      func(v *rdDescriptorInputV2) { v.Slot.Seed++ },
	}
	// Reflect only in tests: enumerate the existing contract fields, not invented
	// authority labels or unchecked production maps.
	for _, group := range []string{"Nomination", "Intent", "Endpoint"} {
		var typ reflect.Type
		switch group {
		case "Nomination":
			typ = reflect.TypeOf(base.Nomination)
		case "Intent":
			typ = reflect.TypeOf(base.Intent)
		case "Endpoint":
			typ = reflect.TypeOf(base.Intent.Endpoint)
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Struct {
				continue
			}
			group, name := group, field.Name
			mutations[group+"."+name] = func(v *rdDescriptorInputV2) {
				r := reflect.ValueOf(v).Elem()
				if group == "Endpoint" {
					r = r.FieldByName("Intent").FieldByName("Endpoint")
				} else {
					r = r.FieldByName(group)
				}
				f := r.FieldByName(name)
				switch f.Kind() {
				case reflect.String:
					f.SetString(f.String() + "x")
				case reflect.Int:
					f.SetInt(f.Int() + 1)
				case reflect.Uint64:
					f.SetUint(f.Uint() + 1)
				default:
					t.Fatal("uncovered scalar")
				}
			}
		}
	}
	mutations["nomination.members"] = func(v *rdDescriptorInputV2) { v.Nomination.Members[0] += "x" }
	mutations["nomination.member-order"] = func(v *rdDescriptorInputV2) {
		v.Nomination.Members[0], v.Nomination.Members[1] = v.Nomination.Members[1], v.Nomination.Members[0]
	}
	mutations["nomination.endpoint-order"] = func(v *rdDescriptorInputV2) {
		v.Nomination.Endpoints[0], v.Nomination.Endpoints[1] = v.Nomination.Endpoints[1], v.Nomination.Endpoints[0]
	}
	for i := 0; i < reflect.TypeOf(base.Nomination.Endpoints[0]).NumField(); i++ {
		name := reflect.TypeOf(base.Nomination.Endpoints[0]).Field(i).Name
		mutations["nomination.endpoint."+name] = func(v *rdDescriptorInputV2) {
			f := reflect.ValueOf(&v.Nomination.Endpoints[0]).Elem().FieldByName(name)
			if f.Kind() == reflect.String {
				f.SetString(f.String() + "x")
			} else {
				f.SetInt(f.Int() + 1)
			}
		}
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			v := syntheticRDDescriptorV2(t)
			mutate(&v)
			if d.matches(v) {
				t.Fatal("binding perturbation accepted")
			}
		})
	}
	// Coherent relabelings must also fail correspondence, even when all local
	// slot/nomination/intent equality checks still hold.
	for _, which := range []string{"seed", "ordinal", "run", "lineage", "nomination", "input", "case", "policy", "result", "community", "endpoint"} {
		t.Run("coherent-"+which, func(t *testing.T) {
			v := syntheticRDDescriptorV2(t)
			switch which {
			case "seed":
				v.Slot.Seed++
				v.Nomination.Seed++
				v.Intent.Seed++
			case "ordinal":
				v.Slot.Ordinal++
				v.Nomination.Slot++
				v.Intent.Slot++
			case "run":
				v.Slot.RunID += "x"
				v.Nomination.RunID += "x"
				v.Intent.RunID += "x"
			case "lineage":
				v.Nomination.Lineage += "x"
				v.Intent.Lineage += "x"
			case "nomination":
				v.Nomination.ID += "x"
				v.Intent.NominationID += "x"
			case "input":
				v.Nomination.InputID += "x"
				v.Intent.InputID += "x"
			case "case":
				v.Nomination.CaseID += "x"
				v.Intent.CaseID += "x"
			case "policy":
				v.Nomination.PolicyID += "x"
				v.Intent.PolicyID += "x"
			case "result":
				v.Nomination.ResultID += "x"
				v.Intent.ResultID += "x"
			case "community":
				v.Nomination.Community++
				v.Intent.Community++
			case "endpoint":
				v.Nomination.Endpoints[1].Predecessor += "x"
				v.Intent.Endpoint.Predecessor += "x"
			}
			if _, err := newRDDescriptorV2(v); err != nil {
				t.Fatal("coherent data rejected", err)
			}
			if d.matches(v) {
				t.Fatal("coherent binding mismatch accepted")
			}
		})
	}
}

func TestPrivateRDDescriptorV2InvalidSnapshots(t *testing.T) {
	cases := map[string]func(*rdDescriptorInputV2){
		"namespace-missing":  func(v *rdDescriptorInputV2) { v.Slot.SeedNamespace = "" },
		"negative-slot":      func(v *rdDescriptorInputV2) { v.Slot.Ordinal = -1 },
		"slot-upper":         func(v *rdDescriptorInputV2) { v.Slot.Ordinal = 64 },
		"negative-community": func(v *rdDescriptorInputV2) { v.Nomination.Community = -1 },
		"missing-id":         func(v *rdDescriptorInputV2) { v.Nomination.InputID = "" },
		"oversized": func(v *rdDescriptorInputV2) {
			v.Nomination.Lineage = strings.Repeat("x", rg.EvaluationMaxStringPrivate+1)
		},
		"invalid-utf8":      func(v *rdDescriptorInputV2) { v.Nomination.Lineage = "\xff" },
		"nil-members":       func(v *rdDescriptorInputV2) { v.Nomination.Members = nil },
		"empty-members":     func(v *rdDescriptorInputV2) { v.Nomination.Members = []string{} },
		"duplicate-members": func(v *rdDescriptorInputV2) { v.Nomination.Members[1] = v.Nomination.Members[0] },
		"nil-endpoints":     func(v *rdDescriptorInputV2) { v.Nomination.Endpoints = nil },
		"empty-endpoints":   func(v *rdDescriptorInputV2) { v.Nomination.Endpoints = []tp.PrivateTypedEndpoint{} },
		"too-many-members":  func(v *rdDescriptorInputV2) { v.Nomination.Members = make([]string, rg.EvaluationMaxNodesPrivate+1) },
		"too-many-endpoints": func(v *rdDescriptorInputV2) {
			v.Nomination.Endpoints = make([]tp.PrivateTypedEndpoint, 2*rg.EvaluationMaxOccurrencesPrivate+rg.EvaluationMaxNodesPrivate+1)
		},
		"non-RD-endpoint": func(v *rdDescriptorInputV2) {
			v.Nomination.Endpoints[0].Kind = "CALLS"
			v.Nomination.Endpoints[0].Role = "CALLER_OCCURRENCE"
		},
		"wrong-role":             func(v *rdDescriptorInputV2) { v.Nomination.Endpoints[1].Role = "QUERY" },
		"negative-local-ordinal": func(v *rdDescriptorInputV2) { v.Nomination.Endpoints[1].Ordinal = -1 },
		"missing-occurrence":     func(v *rdDescriptorInputV2) { v.Nomination.Endpoints[1].Occurrence = "" },
		"unknown-node":           func(v *rdDescriptorInputV2) { v.Nomination.Endpoints[1].Node = "other" },
		"ambiguous-selected-endpoint": func(v *rdDescriptorInputV2) {
			v.Nomination.Endpoints = append(v.Nomination.Endpoints, v.Nomination.Endpoints[1])
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := syntheticRDDescriptorV2(t)
			mutate(&v)
			got, err := newRDDescriptorV2(v)
			if err != errRDDescriptorV2 || !reflect.DeepEqual(got, rdDescriptorV2{}) {
				t.Fatal("invalid snapshot accepted or nonclosed error")
			}
		})
	}
	if (rdDescriptorV2{}).matches(syntheticRDDescriptorV2(t)) {
		t.Fatal("zero snapshot matched")
	}
	// Optional predecessor/custody/disposition are deliberately not invented:
	// preserve empty values when the existing endpoint contract permits them.
	v := syntheticRDDescriptorV2(t)
	v.Nomination.Endpoints[1].Predecessor = ""
	v.Nomination.Endpoints[1].Custody = ""
	v.Nomination.Endpoints[1].Disposition = ""
	intents, err := tp.BuildPrivateSourceIntents(v.Nomination, 2)
	if err != nil {
		t.Fatal(err)
	}
	v.Intent = intents[1]
	d, err := newRDDescriptorV2(v)
	if err != nil || !d.matches(v) {
		t.Fatal("optional metadata filled or rejected")
	}
}
