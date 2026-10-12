package referencegrouping

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
)

const groupingPolicyFamily = "GROUPING_POLICY"
const groupingPolicyVersion = "v1"

type Rational struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}
type GroupingPolicyLimits struct{ MaxRules, MaxCanonicalBytes, MaxStringBytes, MaxAbsoluteNumerator, MaxDenominator, MaxCap int }
type GroupingPolicyParseLimits struct{ MaxRawBytes, MaxRules, MaxCanonicalBytes, MaxStringBytes int }
type GroupingPolicyRule struct {
	RelationKind, Action, RationaleCode string
	Weight                              Rational
	Cap                                 int `json:",omitempty"`
	Normalization, Layer                string
}
type GroupingPolicyRequest struct {
	PolicyID      string
	Epoch         uint64
	EligibleKinds []string
	Rules         []GroupingPolicyRule
	Limits        GroupingPolicyLimits
}

type groupingPolicyRuleJSON struct {
	RelationKind  string   `json:"relation_kind"`
	Action        string   `json:"action"`
	RationaleCode string   `json:"rationale_code"`
	Weight        Rational `json:"weight"`
	Cap           int      `json:"cap,omitempty"`
	Normalization string   `json:"normalization,omitempty"`
	Layer         string   `json:"layer,omitempty"`
}
type groupingPolicyLimitsJSON struct {
	MaxRules             int `json:"max_rules"`
	MaxCanonicalBytes    int `json:"max_canonical_bytes"`
	MaxStringBytes       int `json:"max_string_bytes"`
	MaxAbsoluteNumerator int `json:"max_absolute_numerator"`
	MaxDenominator       int `json:"max_denominator"`
	MaxCap               int `json:"max_cap"`
}
type groupingPolicyDocument struct {
	Family        string                   `json:"family"`
	Version       string                   `json:"version"`
	PolicyID      string                   `json:"policy_id"`
	Epoch         uint64                   `json:"epoch"`
	EligibleKinds []string                 `json:"eligible_kinds"`
	Rules         []groupingPolicyRuleJSON `json:"rules"`
	Limits        groupingPolicyLimitsJSON `json:"limits"`
}
type GroupingPolicy struct {
	canonical            []byte
	digest, domainDigest string
	doc                  groupingPolicyDocument
}

func (p GroupingPolicy) CanonicalBytes() []byte { return append([]byte(nil), p.canonical...) }
func (p GroupingPolicy) ID() string             { return p.digest }
func (p GroupingPolicy) Digest() string         { return p.digest }
func (p GroupingPolicy) ByteLength() int        { return len(p.canonical) }
func (p GroupingPolicy) DomainDigest() string   { return p.domainDigest }
func (p GroupingPolicy) EligibleKinds() []string {
	return append([]string(nil), p.doc.EligibleKinds...)
}
func (p GroupingPolicy) Rules() []GroupingPolicyRule {
	out := make([]GroupingPolicyRule, len(p.doc.Rules))
	for i, r := range p.doc.Rules {
		out[i] = GroupingPolicyRule{r.RelationKind, r.Action, r.RationaleCode, r.Weight, r.Cap, r.Normalization, r.Layer}
	}
	return out
}

func BuildGroupingPolicy(req GroupingPolicyRequest) (GroupingPolicy, error) {
	kinds := append([]string(nil), req.EligibleKinds...)
	sort.Strings(kinds)
	rules := make([]groupingPolicyRuleJSON, len(req.Rules))
	for i, r := range req.Rules {
		r.Weight = reduceRational(r.Weight)
		rules[i] = groupingPolicyRuleJSON{r.RelationKind, r.Action, r.RationaleCode, r.Weight, r.Cap, r.Normalization, r.Layer}
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].RelationKind < rules[j].RelationKind })
	d := groupingPolicyDocument{groupingPolicyFamily, groupingPolicyVersion, req.PolicyID, req.Epoch, kinds, rules, groupingPolicyLimitsJSON{req.Limits.MaxRules, req.Limits.MaxCanonicalBytes, req.Limits.MaxStringBytes, req.Limits.MaxAbsoluteNumerator, req.Limits.MaxDenominator, req.Limits.MaxCap}}
	if err := validateGroupingPolicy(d); err != nil {
		return GroupingPolicy{}, err
	}
	return groupingPolicyFromDocument(d, req.Limits.MaxCanonicalBytes)
}
func ParseGroupingPolicy(raw []byte, l GroupingPolicyParseLimits) (GroupingPolicy, error) {
	if l.MaxRawBytes <= 0 || len(raw) > l.MaxRawBytes || len(raw) < 2 || raw[len(raw)-1] != '\n' || raw[len(raw)-2] == '\n' || bytes.Contains(raw, []byte("\r")) {
		return GroupingPolicy{}, errors.New("invalid framing")
	}
	if err := scanPolicyJSON(raw[:len(raw)-1], l); err != nil {
		return GroupingPolicy{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var d groupingPolicyDocument
	if err := dec.Decode(&d); err != nil {
		return GroupingPolicy{}, err
	}
	if err := requireEOF(dec); err != nil {
		return GroupingPolicy{}, err
	}
	if err := validateGroupingPolicy(d); err != nil {
		return GroupingPolicy{}, err
	}
	if len(d.Rules) > l.MaxRules || maxPolicyString(d) > l.MaxStringBytes {
		return GroupingPolicy{}, errors.New("bound exceeded")
	}
	if !sort.StringsAreSorted(d.EligibleKinds) || !rulesSorted(d.Rules) {
		return GroupingPolicy{}, errors.New("noncanonical order")
	}
	max := l.MaxCanonicalBytes
	if d.Limits.MaxCanonicalBytes < max {
		max = d.Limits.MaxCanonicalBytes
	}
	p, err := groupingPolicyFromDocument(d, max)
	if err != nil || !bytes.Equal(p.canonical, raw) {
		return GroupingPolicy{}, errors.New("noncanonical policy")
	}
	return p, nil
}
func groupingPolicyFromDocument(d groupingPolicyDocument, max int) (GroupingPolicy, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(d); err != nil {
		return GroupingPolicy{}, err
	}
	if max <= 0 || b.Len() > max {
		return GroupingPolicy{}, errors.New("canonical bound")
	}
	h := sha256.Sum256(b.Bytes())
	dh := sha256.Sum256(append([]byte("lsp-trace:grouping-policy:v1\x00"), b.Bytes()...))
	return GroupingPolicy{append([]byte(nil), b.Bytes()...), fmt.Sprintf("sha256:%x", h), fmt.Sprintf("sha256:%x", dh), d}, nil
}
func validateGroupingPolicy(d groupingPolicyDocument) error {
	if d.Family != groupingPolicyFamily || d.Version != groupingPolicyVersion || d.PolicyID == "" || d.Epoch == 0 {
		return errors.New("invalid identity")
	}
	if len(d.EligibleKinds) != 3 || len(d.Rules) != 3 || len(d.Rules) > d.Limits.MaxRules || d.Limits.MaxRules <= 0 || d.Limits.MaxCanonicalBytes <= 0 || d.Limits.MaxStringBytes <= 0 || d.Limits.MaxAbsoluteNumerator <= 0 || d.Limits.MaxDenominator <= 0 || d.Limits.MaxCap <= 0 {
		return errors.New("invalid limits")
	}
	allowedKind := map[string]bool{"CALLS": true, "REFERENCES_SYMBOL": true, "RESOLVES_TO_DEFINITION": true}
	seen := map[string]bool{}
	for _, k := range d.EligibleKinds {
		if !allowedKind[k] || seen[k] {
			return errors.New("invalid kind")
		}
		seen[k] = true
	}
	for _, r := range d.Rules {
		if !seen[r.RelationKind] || r.RationaleCode == "" || r.RationaleCode == "UNREGISTERED" || seen["rule:"+r.RelationKind] {
			return errors.New("invalid rule")
		}
		seen["rule:"+r.RelationKind] = true
		if r.Weight.Denominator <= 0 || r.Weight.Denominator > d.Limits.MaxDenominator || r.Weight.Numerator == math.MinInt || abs(r.Weight.Numerator) > d.Limits.MaxAbsoluteNumerator || reduceRational(r.Weight) != r.Weight {
			return errors.New("invalid rational")
		}
		neutral := r.Weight == (Rational{Numerator: 1, Denominator: 1})
		switch r.Action {
		case "INCLUDE":
			if !neutral || r.Cap != 0 || r.Normalization != "" || r.Layer != "" {
				return errors.New("include fields")
			}
		case "EXCLUDE":
			if r.Weight != (Rational{Numerator: 0, Denominator: 1}) || r.Cap != 0 || r.Normalization != "" || r.Layer != "" {
				return errors.New("exclude fields")
			}
		case "CAP":
			legacy := r.RelationKind == "REFERENCES_SYMBOL" && r.Weight == (Rational{Numerator: 1, Denominator: 2})
			if (!neutral && !legacy) || r.Cap <= 0 || r.Cap > d.Limits.MaxCap || r.Normalization != "" || r.Layer != "" {
				return errors.New("cap fields")
			}
		case "NORMALIZE":
			if !neutral || r.Normalization == "" || r.Cap != 0 || r.Layer != "" {
				return errors.New("normalize fields")
			}
		case "DOWNWEIGHT":
			if r.Weight.Numerator <= 0 || r.Weight.Numerator > r.Weight.Denominator || r.Cap != 0 || r.Normalization != "" || r.Layer != "" {
				return errors.New("downweight fields")
			}
		case "SEPARATE_LAYER":
			legacy := r.RelationKind == "RESOLVES_TO_DEFINITION" && r.Weight == (Rational{Numerator: -1, Denominator: 2})
			if (!neutral && !legacy) || r.Layer == "" || r.Cap != 0 || r.Normalization != "" {
				return errors.New("layer fields")
			}
		default:
			return errors.New("action")
		}
	}
	if maxPolicyString(d) > d.Limits.MaxStringBytes {
		return errors.New("string bound")
	}
	return nil
}
func reduceRational(r Rational) Rational {
	if r.Denominator <= 0 || r.Numerator == math.MinInt {
		return r
	}
	g := gcd(abs(r.Numerator), r.Denominator)
	if g == 0 {
		g = 1
	}
	r.Numerator /= g
	r.Denominator /= g
	return r
}
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func rulesSorted(r []groupingPolicyRuleJSON) bool {
	for i := 1; i < len(r); i++ {
		if r[i-1].RelationKind >= r[i].RelationKind {
			return false
		}
	}
	return true
}
func maxPolicyString(d groupingPolicyDocument) int {
	m := len(d.Family)
	visit := func(s string) {
		if len(s) > m {
			m = len(s)
		}
	}
	visit(d.Version)
	visit(d.PolicyID)
	for _, s := range d.EligibleKinds {
		visit(s)
	}
	for _, r := range d.Rules {
		visit(r.RelationKind)
		visit(r.Action)
		visit(r.RationaleCode)
		visit(r.Normalization)
		visit(r.Layer)
	}
	return m
}
func validSignedInteger(s string) bool {
	if len(s) > 0 && s[0] == '-' {
		return len(s) > 1 && validInteger(s[1:]) && s[1:] != "0"
	}
	return validInteger(s)
}

func scanPolicyJSON(raw []byte, l GroupingPolicyParseLimits) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(json.Token, string) error
	walk = func(t json.Token, f string) error {
		switch v := t.(type) {
		case string:
			if len(v) > l.MaxStringBytes {
				return errors.New("string bound")
			}
		case json.Number:
			if !validSignedInteger(string(v)) {
				return errors.New("number")
			}
		case json.Delim:
			switch v {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					k, _ := dec.Token()
					ks, ok := k.(string)
					if !ok || seen[ks] {
						return errors.New("key")
					}
					seen[ks] = true
					n, e := dec.Token()
					if e != nil {
						return e
					}
					if e = walk(n, ks); e != nil {
						return e
					}
				}
				_, e := dec.Token()
				return e
			case '[':
				limit := 3
				if f == "rules" {
					limit = l.MaxRules
				}
				n := 0
				for dec.More() {
					n++
					if n > limit {
						return errors.New("array bound")
					}
					x, e := dec.Token()
					if e != nil {
						return e
					}
					if e = walk(x, ""); e != nil {
						return e
					}
				}
				_, e := dec.Token()
				return e
			}
		}
		return nil
	}
	t, e := dec.Token()
	if e != nil {
		return e
	}
	if e = walk(t, ""); e != nil {
		return e
	}
	if _, e = dec.Token(); e != io.EOF {
		return errors.New("trailing")
	}
	return nil
}
