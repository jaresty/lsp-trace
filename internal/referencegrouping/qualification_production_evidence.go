package referencegrouping

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"strings"
)

// These are private exact-count report representations, not a public schema or
// an evaluation policy. A genuinely approved plan must explicitly use this
// interpretation. Unsupported interpretations cannot be qualified. Observation
// counts are retained evaluation evidence, not claims that this verifier ran an
// experiment or independently authenticated its producer.
type qualificationCaseSpec struct {
	ID, Arm, Partition, Corpus, Language, Provider, ProviderProfile, Revision string
	ExpectedGrouping                                                          []string
}
type qualificationMetricDefinition struct {
	Code, Definition, Aggregation string
	CaseIDs                       []string
}
type qualificationPlanEvidence struct {
	Family, Version                                            string
	Policy, Input, CorpusManifest, Implementation, Environment qualificationCommitment
	Cases                                                      []qualificationCaseSpec
	MetricDefinitions                                          []qualificationMetricDefinition
	Criteria                                                   []qualificationCriterion
	RequiredReviewerClass                                      string
	RequiredAcceptances                                        int
	FrozenUnix                                                 int64
}
type qualificationManifestEvidence struct {
	Family, Version string
	Cases           []qualificationCaseSpec
}
type qualificationIdentityEvidence struct{ Family, Version, Identity, Details string }
type qualificationObservation struct {
	CaseID, MetricCode     string
	Numerator, Denominator int
}
type qualificationCaseResult struct {
	Case                             qualificationCase
	ExpectedGrouping, ActualGrouping []string
}
type qualificationReportEvidence struct {
	Family, Version                                                            string
	Policy, Input, EvaluationPlan, CorpusManifest, Implementation, Environment qualificationCommitment
	ExecutionUnix                                                              int64
	Cases                                                                      []qualificationCaseResult
	Observations                                                               []qualificationObservation
	Metrics                                                                    []qualificationMetric
	Accounting                                                                 qualificationAccounting
	Uncertainty                                                                string
}

// The iterative preflight bounds depth and token work before recursive receipt
// scanning or typed decoding. Canonical re-encoding rejects duplicate keys,
// alternate spellings, floating numeric forms and noncanonical framing.
func qualificationJSONPreflight(raw []byte, l productionQualificationLimits) error {
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.Contains(raw, []byte{'\r'}) {
		return qualificationIncomplete("artifact framing")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	depth, tokens := 0, 0
	type collectionFrame struct {
		delimiter json.Delim
		elements  int
	}
	// The existing MaxDepth check bounds this stack before every push.
	frames := []collectionFrame{}
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return qualificationIncomplete("artifact JSON")
		}
		if tokens >= l.MaxTokens {
			return qualificationIncomplete("token bound")
		}
		tokens++
		closing := token == json.Delim('}') || token == json.Delim(']')
		if last := len(frames) - 1; last >= 0 && frames[last].delimiter == '[' && !closing {
			// A container starts one element in its parent array. Its children
			// belong only to its own frame; object keys never count as elements.
			if frames[last].elements >= l.MaxRecords {
				return qualificationIncomplete("record bound")
			}
			frames[last].elements++
		}
		switch v := token.(type) {
		case json.Delim:
			if v == '{' || v == '[' {
				if depth >= l.MaxDepth {
					return qualificationIncomplete("depth bound")
				}
				depth++
				frames = append(frames, collectionFrame{delimiter: v})
			} else {
				depth--
				if depth < 0 {
					return qualificationIncomplete("depth")
				}
				frames = frames[:len(frames)-1]
			}
		case string:
			if len(v) > l.Receipt.MaxStringBytes {
				return qualificationIncomplete("string bound")
			}
		case json.Number:
			if !validInteger(string(v)) {
				return qualificationIncomplete("integer required")
			}
		}
	}
	if depth != 0 {
		return qualificationIncomplete("depth")
	}
	return nil
}
func parseProductionQualificationArtifact(raw []byte, v any, l productionQualificationLimits) error {
	if len(raw) > l.MaxArtifactBytes {
		return qualificationIncomplete("artifact byte bound")
	}
	if err := qualificationJSONPreflight(raw, l); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil || requireEOF(dec) != nil {
		return qualificationIncomplete("unsupported evidence interpretation")
	}
	canonical, err := json.Marshal(v)
	if err != nil || !bytes.Equal(append(canonical, '\n'), raw) {
		return qualificationIncomplete("noncanonical evidence")
	}
	return nil
}
func verifyProductionQualificationArtifacts(d qualificationDocument, raw [5][]byte, a qualificationApproval, l productionQualificationLimits) error {
	fail := qualificationIncomplete
	var plan qualificationPlanEvidence
	var manifest qualificationManifestEvidence
	var implementation, environment qualificationIdentityEvidence
	var report qualificationReportEvidence
	for j, v := range []any{&plan, &manifest, &implementation, &environment, &report} {
		if err := parseProductionQualificationArtifact(raw[j], v, l); err != nil {
			return err
		}
	}
	if plan.Family != "GROUPING_QUALIFICATION_PLAN" || manifest.Family != "GROUPING_QUALIFICATION_CORPUS" || report.Family != "GROUPING_QUALIFICATION_REPORT" || implementation.Family != "GROUPING_QUALIFICATION_IMPLEMENTATION" || environment.Family != "GROUPING_QUALIFICATION_ENVIRONMENT" || plan.Version != "v1" || manifest.Version != "v1" || report.Version != "v1" || implementation.Version != "v1" || environment.Version != "v1" || implementation.Identity == "" || implementation.Details == "" || environment.Identity == "" || environment.Details == "" {
		return fail("evidence family/version or identity")
	}
	if plan.Policy != d.Policy || plan.Input != d.GroupingInput || plan.CorpusManifest != d.CorpusManifest || plan.Implementation != d.Implementation || plan.Environment != d.Environment || plan.FrozenUnix != a.FrozenUnix || plan.RequiredReviewerClass != a.RequiredClass || plan.RequiredAcceptances != a.RequiredAcceptances || !reflect.DeepEqual(plan.Criteria, d.Criteria) {
		return fail("frozen plan binding or criteria")
	}
	if report.Policy != d.Policy || report.Input != d.GroupingInput || report.EvaluationPlan != d.EvaluationPlan || report.CorpusManifest != d.CorpusManifest || report.Implementation != d.Implementation || report.Environment != d.Environment || report.ExecutionUnix <= plan.FrozenUnix || report.ExecutionUnix > d.DecisionUnix || report.Uncertainty == "" {
		return fail("report subject/chronology/uncertainty")
	}
	if len(plan.Cases) != len(requiredArms) || len(report.Cases) != len(plan.Cases) || len(plan.Cases) != len(d.Cases) || !reflect.DeepEqual(plan.Cases, manifest.Cases) || len(report.Observations) > l.MaxRecords || len(plan.MetricDefinitions) != len(requiredMetricCodes) || len(report.Metrics) != len(requiredMetricCodes) {
		return fail("complete case/metric inventory")
	}
	hasCueHeldOut := false
	for j, spec := range plan.Cases {
		c := d.Cases[j]
		if spec.ID != c.ID || spec.Arm != c.Arm || spec.Partition != c.Partition || spec.Corpus != c.Corpus || spec.Language != c.Language || spec.Provider != c.Provider || spec.ProviderProfile != c.ProviderProfile || spec.Revision != c.Revision || (j > 0 && plan.Cases[j-1].ID >= spec.ID) || len(spec.ExpectedGrouping) == 0 || len(spec.ExpectedGrouping) > l.MaxRecords || !strictQualificationStrings(spec.ExpectedGrouping) {
			return fail("case/profile/revision/partition/expected-grouping binding")
		}
		r := report.Cases[j]
		if r.Case != c || !reflect.DeepEqual(r.ExpectedGrouping, spec.ExpectedGrouping) || len(r.ActualGrouping) == 0 || len(r.ActualGrouping) > l.MaxRecords || !strictQualificationStrings(r.ActualGrouping) {
			return fail("case result/expected-grouping mismatch")
		}
		if spec.Language == "cue" && spec.Partition == "HELD_OUT" && strings.HasPrefix(spec.ProviderProfile, "CUE_") && strings.HasSuffix(spec.ProviderProfile, "_EXACT_V1") && (spec.Arm == "REFERENCES_ONLY" || spec.Arm == "DEFINITIONS_ONLY" || spec.Arm == "REFERENCES_PLUS_DEFINITIONS") {
			hasCueHeldOut = true
		}
	}
	if !hasCueHeldOut {
		return fail("held-out no-CALLS CUE case missing")
	}
	// Every reported metric is derived from exactly the plan's case measurement
	// inventory, not copied from the receipt summary. The approved definition
	// supplies the experimental meaning; this interpreter only supports exact
	// integer counter sums followed by rational reduction. It never runs Leiden.
	observations := map[string]qualificationObservation{}
	for _, o := range report.Observations {
		key := o.MetricCode + "\x00" + o.CaseID
		if _, exists := observations[key]; exists || o.Numerator < 0 || o.Denominator <= 0 || o.Numerator > o.Denominator {
			return fail("metric observation")
		}
		observations[key] = o
	}
	for j, def := range plan.MetricDefinitions {
		if def.Code != d.Metrics[j].Code || def.Code != report.Metrics[j].Code || !contains(requiredMetricCodes, def.Code) || def.Definition == "" || def.Aggregation != "SUM_EXACT_COUNTS" || len(def.CaseIDs) == 0 || len(def.CaseIDs) > len(plan.Cases) || !strictQualificationStrings(def.CaseIDs) || (j > 0 && plan.MetricDefinitions[j-1].Code >= def.Code) {
			return fail("approved metric definition")
		}
		expectedIDs := []string{}
		for _, c := range plan.Cases {
			if !strings.HasPrefix(def.Code, "CROSS_SEED_INSTABILITY_") || c.Arm == strings.TrimPrefix(def.Code, "CROSS_SEED_INSTABILITY_") {
				expectedIDs = append(expectedIDs, c.ID)
			}
		}
		if !reflect.DeepEqual(expectedIDs, def.CaseIDs) {
			return fail("metric mode coverage")
		}
		numerator, denominator := 0, 0
		for _, id := range def.CaseIDs {
			index := sort.Search(len(plan.Cases), func(k int) bool { return plan.Cases[k].ID >= id })
			if index == len(plan.Cases) || plan.Cases[index].ID != id {
				return fail("metric foreign case")
			}
			key := def.Code + "\x00" + id
			o, ok := observations[key]
			if !ok {
				return fail("missing metric observation")
			}
			delete(observations, key)
			numerator, ok = qualificationCheckedSum(numerator, o.Numerator)
			if !ok {
				return fail("metric numerator overflow")
			}
			denominator, ok = qualificationCheckedSum(denominator, o.Denominator)
			if !ok {
				return fail("metric denominator overflow")
			}
		}
		divisor := gcd(numerator, denominator)
		metric := qualificationMetric{def.Code, numerator / divisor, denominator / divisor}
		if metric != d.Metrics[j] || metric != report.Metrics[j] {
			return fail("derived metric mismatch")
		}
	}
	if len(observations) != 0 || report.Accounting != d.Accounting || !qualificationAccountingChecked(d) {
		return fail("unaccounted observation or report summary")
	}
	return nil
}
func strictQualificationStrings(s []string) bool {
	for j, v := range s {
		if v == "" || (j > 0 && s[j-1] >= v) {
			return false
		}
	}
	return true
}
