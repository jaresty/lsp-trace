package qualificationpolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const SchemaID = "lsp-trace.qualification-policy.v1"

type Tuple struct {
	Language  string `json:"language"`
	Provider  string `json:"provider"`
	Version   string `json:"version"`
	Operation string `json:"operation"`
}
type ObservationRequirement struct {
	ID       string `json:"id"`
	Expected string `json:"expected"`
}
type Policy struct {
	SchemaVersion         string                   `json:"schema_version"`
	PolicyID              string                   `json:"policy_id"`
	PolicyVersion         string                   `json:"policy_version"`
	AuthorityID           string                   `json:"authority_id"`
	Tuple                 Tuple                    `json:"tuple"`
	RequiredPositive      []ObservationRequirement `json:"required_positive"`
	RequiredNegative      []ObservationRequirement `json:"required_negative"`
	MinimumRealServerRuns int                      `json:"minimum_real_server_runs"`
	MinimumRepetitions    int                      `json:"minimum_repetitions"`
	PassPredicates        []string                 `json:"pass_predicates"`
	FailurePredicates     []string                 `json:"failure_predicates"`
	CustodyReceipt        string                   `json:"custody_receipt"`
}
type Observation struct {
	ID         string `json:"id"`
	Actual     string `json:"actual"`
	Passed     bool   `json:"passed"`
	RealServer bool   `json:"real_server"`
	RunID      string `json:"run_id"`
}
type Evidence struct {
	PolicyID      string        `json:"policy_id"`
	PolicyVersion string        `json:"policy_version"`
	Tuple         Tuple         `json:"tuple"`
	Status        string        `json:"status"`
	IssuerID      string        `json:"issuer_id"`
	Observations  []Observation `json:"observations"`
}
type Trust struct {
	AuthorityID           string
	CurrentPolicyVersion  string
	TrustedCustodyReceipt string
}
type Decision struct {
	Status  string   `json:"status"`
	Blocked bool     `json:"blocked"`
	Reasons []string `json:"reasons"`
}

func Parse(raw []byte) (Policy, error) {
	var p Policy
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Policy{}, err
	}
	if dec.More() {
		return Policy{}, errors.New("qualification policy has trailing JSON")
	}
	if err := validatePolicy(p); err != nil {
		return Policy{}, err
	}
	return p, nil
}

func CanonicalBytes(p Policy) ([]byte, error) {
	if err := validatePolicy(p); err != nil {
		return nil, err
	}
	p.RequiredPositive = append([]ObservationRequirement(nil), p.RequiredPositive...)
	p.RequiredNegative = append([]ObservationRequirement(nil), p.RequiredNegative...)
	p.PassPredicates = append([]string(nil), p.PassPredicates...)
	p.FailurePredicates = append([]string(nil), p.FailurePredicates...)
	sort.Slice(p.RequiredPositive, func(i, j int) bool {
		if p.RequiredPositive[i].ID == p.RequiredPositive[j].ID {
			return p.RequiredPositive[i].Expected < p.RequiredPositive[j].Expected
		}
		return p.RequiredPositive[i].ID < p.RequiredPositive[j].ID
	})
	sort.Slice(p.RequiredNegative, func(i, j int) bool {
		if p.RequiredNegative[i].ID == p.RequiredNegative[j].ID {
			return p.RequiredNegative[i].Expected < p.RequiredNegative[j].Expected
		}
		return p.RequiredNegative[i].ID < p.RequiredNegative[j].ID
	})
	sort.Strings(p.PassPredicates)
	sort.Strings(p.FailurePredicates)
	return json.Marshal(p)
}

func validatePolicy(p Policy) error {
	if p.SchemaVersion != SchemaID {
		return fmt.Errorf("schema_version must be %q", SchemaID)
	}
	if p.PolicyID == "" || p.PolicyVersion == "" || p.AuthorityID == "" || p.CustodyReceipt == "" {
		return errors.New("policy identity, version, authority, and custody receipt are required")
	}
	if p.Tuple.Language == "" || p.Tuple.Provider == "" || p.Tuple.Version == "" || p.Tuple.Operation == "" {
		return errors.New("exact qualification tuple is required")
	}
	if p.MinimumRealServerRuns < 1 || p.MinimumRepetitions < 1 {
		return errors.New("evidence minima must be positive")
	}
	seen := map[string]bool{}
	for _, r := range append(append([]ObservationRequirement(nil), p.RequiredPositive...), p.RequiredNegative...) {
		if r.ID == "" || r.Expected == "" || seen[r.ID] {
			return errors.New("observation requirements need unique nonempty identities and expectations")
		}
		seen[r.ID] = true
	}
	return nil
}

func Validate(p Policy, e Evidence, trust Trust) Decision {
	if e.Status == "BLOCKED" {
		return Decision{Status: "BLOCKED", Blocked: true, Reasons: []string{"retained BLOCKED evidence is never PASS"}}
	}
	if e.Status == "FAIL" {
		return Decision{Status: "FAIL", Reasons: []string{"evidence reports failure"}}
	}
	reasons := []string{}
	if err := validatePolicy(p); err != nil {
		reasons = append(reasons, err.Error())
	}
	if p.AuthorityID != trust.AuthorityID {
		reasons = append(reasons, "policy authority is not verifier-trusted")
	}
	if p.CustodyReceipt != trust.TrustedCustodyReceipt {
		reasons = append(reasons, "policy custody receipt is not verifier-trusted")
	}
	if p.PolicyVersion != trust.CurrentPolicyVersion || e.PolicyVersion != p.PolicyVersion {
		reasons = append(reasons, "stale or substituted policy version")
	}
	if e.PolicyID != p.PolicyID {
		reasons = append(reasons, "policy identity mismatch")
	}
	if e.Tuple != p.Tuple {
		reasons = append(reasons, "qualification tuple mismatch")
	}
	if e.IssuerID == "" || e.IssuerID == p.AuthorityID {
		reasons = append(reasons, "self-issued status is forbidden")
	}
	if e.Status != "PASS" {
		reasons = append(reasons, "evidence does not request PASS")
	}
	byID := map[string][]Observation{}
	realRuns := map[string]bool{}
	for _, o := range e.Observations {
		byID[o.ID] = append(byID[o.ID], o)
		if o.RealServer && o.RunID != "" {
			realRuns[o.RunID] = true
		}
	}
	failure := false
	check := func(r ObservationRequirement) {
		matches := 0
		observedFailure := false
		for _, o := range byID[r.ID] {
			if !o.Passed {
				observedFailure = true
			}
			if o.Passed && o.Actual == r.Expected {
				matches++
			}
		}
		if observedFailure {
			failure = true
			reasons = append(reasons, "required observation failed: "+r.ID)
		} else if matches < p.MinimumRepetitions {
			reasons = append(reasons, "required observation missing repetitions: "+r.ID)
		}
	}
	for _, r := range p.RequiredPositive {
		check(r)
	}
	for _, r := range p.RequiredNegative {
		check(r)
	}
	if len(realRuns) < p.MinimumRealServerRuns {
		reasons = append(reasons, "insufficient real-server runs")
	}
	for _, id := range p.PassPredicates {
		found := false
		for _, o := range byID[id] {
			found = found || o.Passed
		}
		if !found {
			reasons = append(reasons, "pass predicate unsatisfied: "+id)
		}
	}
	for _, id := range p.FailurePredicates {
		for _, o := range byID[id] {
			if o.Passed {
				failure = true
				reasons = append(reasons, "failure predicate observed: "+id)
				break
			}
		}
	}
	if failure {
		return Decision{Status: "FAIL", Reasons: reasons}
	}
	if len(reasons) > 0 {
		return Decision{Status: "NOT_QUALIFIED", Reasons: reasons}
	}
	return Decision{Status: "PASS"}
}
