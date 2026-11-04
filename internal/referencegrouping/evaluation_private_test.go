package referencegrouping

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/programcadmission"
)

func evaluationAdmittedFixture(t *testing.T, kinds []programcadmission.RelationKind) AdmittedInput {
	t.Helper()
	f := referenceFixture(t, kinds)
	a, e := BuildInput(f.request())
	if e != nil {
		t.Fatal(e)
	}
	i, e := AdmitInput(a, f.predecessors)
	if e != nil {
		t.Fatal(e)
	}
	return i
}
func TestEvaluationPrivateAllKindCombinations(t *testing.T) {
	kinds := []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}
	for mask := 1; mask < 8; mask++ {
		t.Run(string(rune('0'+mask)), func(t *testing.T) {
			selected := []programcadmission.RelationKind{}
			for j, k := range kinds {
				if mask&(1<<j) != 0 {
					selected = append(selected, k)
				}
			}
			input := evaluationAdmittedFixture(t, selected)
			policy := stage5Policy(t, map[string]GroupingPolicyRule{"REFERENCES_SYMBOL": {Action: "DOWNWEIGHT", RationaleCode: "CONTROLLED_FRACTION", Weight: Rational{1, 4}}, "RESOLVES_TO_DEFINITION": {Action: "DOWNWEIGHT", RationaleCode: "CONTROLLED_FRACTION", Weight: Rational{1, 4}}})
			p, e := ProjectEvaluationPrivate(input, policy, stage5Limits())
			if e != nil {
				t.Fatal(e)
			}
			if len(p.Occurrences()) != len(input.Contributions()) || p.Accounting().Terminal != len(input.Contributions()) || len(p.Custody()) != 2*len(input.Contributions()) || !reflect.DeepEqual(p.Custody(), input.EndpointCustody()) {
				t.Fatal("ASSERT_EVALUATION_EXACT_TYPED_LEDGER")
			}
			for j, o := range p.Occurrences() {
				c := input.Contributions()[j]
				if o.OccurrenceIdentity != c.OccurrenceIdentity || o.Kind != c.Kind || o.From != c.From || o.To != c.To || o.Ordinal != c.Ordinal || o.SourcePredecessorIdentity != input.PredecessorIdentityForContribution(c) || o.SourceCustodyIdentity != input.CustodyIdentityForOccurrence(c.OccurrenceIdentity) {
					t.Fatal("ASSERT_EVALUATION_EXACT_TYPED_LEDGER")
				}
			}
			if _, e := ComposeQualified(input, policy, VerifiedQualification{}, 8, stage5Limits()); e == nil {
				t.Fatal("ASSERT_EVALUATION_NO_QUALIFICATION_BOOTSTRAP")
			}
			if _, e := ParseCompositionReceipt(p.CanonicalBytes(), stage5Limits()); e == nil {
				t.Fatal("ASSERT_EVALUATION_NOT_COMPOSITION_RECEIPT")
			}
			t.Log("ASSERT_EVALUATION_EXACT_TYPED_LEDGER: PASS")
		})
	}
}
func TestEvaluationPrivateActionDispositionAndInactiveUniverse(t *testing.T) {
	input := evaluationAdmittedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	for _, action := range []string{"INCLUDE", "EXCLUDE", "DOWNWEIGHT", "CAP", "NORMALIZE", "SEPARATE_LAYER"} {
		t.Run(action, func(t *testing.T) {
			r := GroupingPolicyRule{Action: action, RationaleCode: "CONTROLLED_ACTION", Weight: Rational{1, 1}}
			switch action {
			case "EXCLUDE":
				r.Weight = Rational{0, 1}
			case "DOWNWEIGHT":
				r.Weight = Rational{1, 4}
			case "CAP":
				r.Cap = 1
			case "NORMALIZE":
				r.Normalization = "BY_KIND_MULTIPLICITY"
			case "SEPARATE_LAYER":
				r.Layer = "CONTROLLED_LAYER"
			}
			p, e := ProjectEvaluationPrivate(input, stage5Policy(t, map[string]GroupingPolicyRule{"REFERENCES_SYMBOL": r}), stage5Limits())
			if e != nil {
				t.Fatal(e)
			}
			if len(p.NodeIdentities()) != 2 || len(p.Occurrences()) != 2 || p.Accounting().Terminal != 2 {
				t.Fatal("ASSERT_EVALUATION_INACTIVE_UNIVERSE")
			}
			pairs, layer := p.PairContributions(), p.SeparateLayerContributions()
			want := 2
			if action == "EXCLUDE" || action == "SEPARATE_LAYER" {
				want = 0
			}
			if action == "CAP" {
				want = 1
			}
			if len(pairs) != want {
				t.Fatalf("ASSERT_EVALUATION_ACTION_LEDGER %s", action)
			}
			if action == "CAP" && (pairs[0].OccurrenceIdentity != input.Contributions()[0].OccurrenceIdentity || p.Occurrences()[1].TerminalDisposition != "EXCLUDED" || p.Accounting().Capped != 1 || p.Accounting().Excluded != 1) {
				t.Fatal("ASSERT_EVALUATION_CANONICAL_WHOLE_KIND_CAP")
			}
			if action == "NORMALIZE" {
				for _, pair := range pairs {
					if pair.Weight != (Rational{1, 2}) {
						t.Fatal("ASSERT_EVALUATION_EXACT_NORMALIZATION")
					}
				}
			}
			if action == "SEPARATE_LAYER" && (len(layer) != 2 || layer[0].Layer != "CONTROLLED_LAYER") {
				t.Fatal("ASSERT_EVALUATION_SEPARATE_LAYER")
			}
			if !strings.Contains(string(p.CanonicalBytes()), EvaluationUniversePrivate) || !strings.Contains(string(p.CanonicalBytes()), "SOURCE_NODE_UNIVERSE_UNKNOWN") {
				t.Fatal("ASSERT_EVALUATION_EXPLICIT_UNIVERSE_CEILING")
			}
			t.Log("ASSERT_EVALUATION_ACTION_LEDGER: PASS")
		})
	}
}
func TestEvaluationPrivateImmutableBindingsCopiesAndPermutation(t *testing.T) {
	f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	request := f.request()
	a, e := BuildInput(request)
	if e != nil {
		t.Fatal(e)
	}
	i, e := AdmitInput(a, f.predecessors)
	if e != nil {
		t.Fatal(e)
	}
	probes, e := CalibrationProbesPrivate(8)
	if e != nil {
		t.Fatal(e)
	}
	p, e := ProjectEvaluationPrivate(i, probes[2], stage5Limits())
	if e != nil {
		t.Fatal(e)
	}
	reverse(request.Predecessors)
	reverse(request.EndpointCustody)
	reverse(request.CandidateOutcomes)
	a2, e := BuildInput(request)
	if e != nil {
		t.Fatal(e)
	}
	i2, e := AdmitInput(a2, f.predecessors)
	if e != nil {
		t.Fatal(e)
	}
	p2, e := ProjectEvaluationPrivate(i2, probes[2], stage5Limits())
	if e != nil || !bytes.Equal(p.CanonicalBytes(), p2.CanonicalBytes()) || p.ID() != p2.ID() {
		t.Fatal("ASSERT_EVALUATION_PERMUTATION_STABLE")
	}
	binding := p.Bindings()
	if binding.Input != (CompositionCommitment{i.ID(), i.Digest(), i.ByteLength()}) || binding.Policy != (CompositionCommitment{probes[2].ID(), probes[2].Digest(), probes[2].ByteLength()}) {
		t.Fatal("ASSERT_EVALUATION_EXACT_BINDINGS")
	}
	before := p.CanonicalBytes()
	raw := p.CanonicalBytes()
	raw[0] ^= 1
	nodes := p.NodeIdentities()
	nodes[0] = "mutated"
	pairs := p.PairContributions()
	pairs[0].From = "mutated"
	custody := p.Custody()
	custody[0].Endpoint = "mutated"
	occ := p.Occurrences()
	occ[0].From = "mutated"
	if !bytes.Equal(before, p.CanonicalBytes()) || CheckEvaluationProjectionPrivate(p, raw) == nil || CheckEvaluationProjectionPrivate(EvaluationProjectionPrivate{}, before) == nil {
		t.Fatal("ASSERT_EVALUATION_OPAQUE_COPIES_AND_CHECK")
	}
	other, e := ProjectEvaluationPrivate(i, probes[0], stage5Limits())
	if e != nil {
		t.Fatal(e)
	}
	if CheckEvaluationProjectionPrivate(p, other.CanonicalBytes()) == nil {
		t.Fatal("ASSERT_EVALUATION_FOREIGN_POLICY_REFUSED")
	}
	var doc map[string]any
	if e := json.Unmarshal(before, &doc); e != nil {
		t.Fatal(e)
	}
	if doc["Authority"] != float64(0) || doc["Accepted"] != false || doc["Completeness"] != "UNKNOWN" || doc["SourceNodeUniverse"] != "UNKNOWN" || doc["PolicyStatus"] != "UNQUALIFIED" {
		t.Fatal("ASSERT_EVALUATION_NONADMISSIBLE")
	}
	t.Log("ASSERT_EVALUATION_BINDINGS_AND_COPIES: PASS")
}
func TestEvaluationPrivateNegativeSeparateLayerIsRetainedNotBase(t *testing.T) {
	input := evaluationAdmittedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	policy := stage5Policy(t, map[string]GroupingPolicyRule{"RESOLVES_TO_DEFINITION": {Action: "SEPARATE_LAYER", RationaleCode: "CONTROLLED_NEGATIVE_LAYER", Layer: "DEFINITIONS", Weight: Rational{-1, 2}}})
	p, e := ProjectEvaluationPrivate(input, policy, stage5Limits())
	if e != nil {
		t.Fatal(e)
	}
	separate := p.SeparateLayerContributions()
	if len(separate) != 1 || separate[0].Weight != (Rational{-1, 2}) || separate[0].SourceCustodyIdentity == "" || p.Accounting().SeparateLayer != 1 {
		t.Fatal("ASSERT_EVALUATION_SIGNED_LAYER_RETAINED")
	}
	for _, pair := range p.PairContributions() {
		if pair.Kind == "RESOLVES_TO_DEFINITION" || pair.Weight.Numerator <= 0 {
			t.Fatal("ASSERT_EVALUATION_SIGNED_LAYER_NOT_BASE")
		}
	}
	t.Log("ASSERT_EVALUATION_SIGNED_LAYER_RETAINED: PASS")
}

func TestEvaluationPrivateBoundsAndFiveProbes(t *testing.T) {
	probes, e := CalibrationProbesPrivate(8)
	if e != nil || len(probes) != 5 {
		t.Fatal("ASSERT_EVALUATION_FIVE_PROBES", e)
	}
	for j, name := range []string{"H", "U", "L", "B", "N"} {
		for _, r := range probes[j].Rules() {
			if !strings.HasSuffix(r.RationaleCode, name) {
				t.Fatal("ASSERT_EVALUATION_PROBE_IDENTITY")
			}
			if r.RelationKind == "CALLS" {
				if r.Action != "INCLUDE" || r.Weight != (Rational{1, 1}) {
					t.Fatal("ASSERT_EVALUATION_PROBE_CALLS_UNCHANGED")
				}
				continue
			}
			switch name {
			case "H":
				if r.Action != "EXCLUDE" || r.Weight != (Rational{0, 1}) {
					t.Fatal("ASSERT_EVALUATION_PROBE_H")
				}
			case "U":
				if r.Action != "INCLUDE" {
					t.Fatal("ASSERT_EVALUATION_PROBE_U")
				}
			case "L":
				if r.Action != "DOWNWEIGHT" || r.Weight != (Rational{1, 4}) {
					t.Fatal("ASSERT_EVALUATION_PROBE_L")
				}
			case "B":
				if r.Action != "CAP" || r.Cap != 4 || r.Weight != (Rational{1, 1}) {
					t.Fatal("ASSERT_EVALUATION_PROBE_B")
				}
			case "N":
				if r.Action != "NORMALIZE" || r.Normalization != "BY_KIND_MULTIPLICITY" {
					t.Fatal("ASSERT_EVALUATION_PROBE_N")
				}
			}
		}
	}
	if _, e := CalibrationProbesPrivate(0); e == nil {
		t.Fatal("ASSERT_EVALUATION_INVALID_EPOCH")
	}
	i := evaluationAdmittedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	for _, mutate := range []func(*CompositionLimits){func(l *CompositionLimits) { l.MaxOccurrences = 1 }, func(l *CompositionLimits) { l.MaxPairContributions = 1 }, func(l *CompositionLimits) { l.MaxRawBytes = i.ByteLength() - 1 }, func(l *CompositionLimits) { l.MaxStringBytes = 1 }, func(l *CompositionLimits) { l.MaxCanonicalBytes = 1 }, func(l *CompositionLimits) { l.MaxOccurrences = EvaluationMaxOccurrencesPrivate + 1 }, func(l *CompositionLimits) { l.MaxDenominator = 1 << 31 }, func(l *CompositionLimits) { l.MaxAccumulator = -1 }} {
		l := stage5Limits()
		mutate(&l)
		if _, e := ProjectEvaluationPrivate(i, probes[2], l); e == nil {
			t.Fatal("ASSERT_EVALUATION_BOUND_REFUSAL")
		}
	}
	if _, e := ProjectEvaluationPrivate(AdmittedInput{}, probes[2], stage5Limits()); e == nil {
		t.Fatal("ASSERT_EVALUATION_MISSING_INPUT")
	}
	if _, e := ProjectEvaluationPrivate(i, GroupingPolicy{}, stage5Limits()); e == nil {
		t.Fatal("ASSERT_EVALUATION_MISSING_POLICY")
	}
	t.Log("ASSERT_EVALUATION_FIVE_PROBES_AND_BOUNDS: PASS")
}
