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

const (
	compositionFamily  = "COMPOSITION_RECEIPT"
	compositionVersion = "v1"
)

type CompositionLimits struct {
	MaxRawBytes, MaxOccurrences, MaxPairContributions, MaxPairs int
	MaxCanonicalBytes, MaxStringBytes, MaxDenominator           int
	MaxAccumulator                                              int
}

type CompositionCommitment struct {
	ID         string `json:"id"`
	Digest     string `json:"digest"`
	ByteLength int    `json:"byte_length"`
}

type CompositionBindings struct {
	InputID                        string                `json:"input_id"`
	InputDigest                    string                `json:"input_digest"`
	InputByteLength                int                   `json:"input_byte_length"`
	PolicyID                       string                `json:"policy_id"`
	PolicyDigest                   string                `json:"policy_digest"`
	PolicyByteLength               int                   `json:"policy_byte_length"`
	QualificationReceiptCommitment CompositionCommitment `json:"qualification_receipt_commitment"`
}

type CompositionOccurrence struct {
	OccurrenceIdentity        string `json:"occurrence_identity"`
	Kind                      string `json:"kind"`
	From                      string `json:"from"`
	To                        string `json:"to"`
	Ordinal                   int    `json:"ordinal"`
	Action                    string `json:"action"`
	RationaleCode             string `json:"rationale_code"`
	Layer                     string `json:"layer,omitempty"`
	NormalizationBasis        string `json:"normalization_basis,omitempty"`
	CapBefore                 int    `json:"cap_before,omitempty"`
	CapAfter                  int    `json:"cap_after,omitempty"`
	SourcePredecessorIdentity string `json:"source_predecessor_identity"`
	SourceCustodyIdentity     string `json:"source_custody_identity"`
	TerminalDisposition       string `json:"terminal_disposition"`
}

type PairContribution struct {
	OccurrenceIdentity        string   `json:"occurrence_identity"`
	Kind                      string   `json:"kind"`
	From                      string   `json:"from"`
	To                        string   `json:"to"`
	Weight                    Rational `json:"weight"`
	RationaleCode             string   `json:"rationale_code"`
	Layer                     string   `json:"layer,omitempty"`
	NormalizationBasis        string   `json:"normalization_basis,omitempty"`
	CapBefore                 int      `json:"cap_before,omitempty"`
	CapAfter                  int      `json:"cap_after,omitempty"`
	SourcePredecessorIdentity string   `json:"source_predecessor_identity"`
	SourceCustodyIdentity     string   `json:"source_custody_identity"`
}

func (p PairContribution) CanonicalKey() string {
	return p.OccurrenceIdentity + "\x00" + p.Kind + "\x00" + p.From + "\x00" + p.To + "\x00" + p.Layer
}

type CompositionAccounting struct {
	Included, Excluded, Capped, Normalized, Downweighted, SeparateLayer, Duplicate int
	Terminal                                                                       int
	Truncated                                                                      bool
}

type compositionAccountingJSON struct {
	Included      int  `json:"included"`
	Excluded      int  `json:"excluded"`
	Capped        int  `json:"capped"`
	Normalized    int  `json:"normalized"`
	Downweighted  int  `json:"downweighted"`
	SeparateLayer int  `json:"separate_layer"`
	Duplicate     int  `json:"duplicate"`
	Terminal      int  `json:"terminal"`
	Truncated     bool `json:"truncated"`
}

type compositionDocument struct {
	Family                     string                    `json:"family"`
	Version                    string                    `json:"version"`
	Epoch                      uint64                    `json:"epoch"`
	Bindings                   CompositionBindings       `json:"bindings"`
	Occurrences                []CompositionOccurrence   `json:"occurrences"`
	PairContributions          []PairContribution        `json:"pair_contributions"`
	SeparateLayerContributions []PairContribution        `json:"separate_layer_contributions"`
	Custody                    []EndpointCustody         `json:"custody"`
	Accounting                 compositionAccountingJSON `json:"accounting"`
	PairWeightDigest           string                    `json:"pair_weight_digest"`
}

type CompositionReceipt struct {
	canonical []byte
	digest    string
	doc       compositionDocument
}

func (r CompositionReceipt) CanonicalBytes() []byte        { return append([]byte(nil), r.canonical...) }
func (r CompositionReceipt) ID() string                    { return r.digest }
func (r CompositionReceipt) Digest() string                { return r.digest }
func (r CompositionReceipt) ByteLength() int               { return len(r.canonical) }
func (r CompositionReceipt) Bindings() CompositionBindings { return r.doc.Bindings }
func (r CompositionReceipt) Occurrences() []CompositionOccurrence {
	return append([]CompositionOccurrence(nil), r.doc.Occurrences...)
}
func (r CompositionReceipt) PairContributions() []PairContribution {
	return append([]PairContribution(nil), r.doc.PairContributions...)
}
func (r CompositionReceipt) SeparateLayerContributions() []PairContribution {
	return append([]PairContribution(nil), r.doc.SeparateLayerContributions...)
}
func (r CompositionReceipt) Custody() []EndpointCustody {
	return append([]EndpointCustody(nil), r.doc.Custody...)
}
func (r CompositionReceipt) PairWeightDigest() string { return r.doc.PairWeightDigest }
func (r CompositionReceipt) Accounting() CompositionAccounting {
	a := r.doc.Accounting
	return CompositionAccounting{a.Included, a.Excluded, a.Capped, a.Normalized, a.Downweighted, a.SeparateLayer, a.Duplicate, a.Terminal, a.Truncated}
}

func ComposeQualified(input AdmittedInput, policy GroupingPolicy, verified VerifiedQualification, epoch uint64, limits CompositionLimits) (CompositionReceipt, error) {
	if len(input.artifact.canonical) == 0 || len(policy.canonical) == 0 || !verified.Matches(policy, input, epoch) {
		return CompositionReceipt{}, errors.New("unqualified composition")
	}
	if err := validateCompositionLimits(limits); err != nil {
		return CompositionReceipt{}, err
	}
	rules := make(map[string]GroupingPolicyRule, len(policy.doc.Rules))
	for _, r := range policy.Rules() {
		rules[r.RelationKind] = r
	}
	contributions := input.Contributions()
	if len(contributions) > limits.MaxOccurrences {
		return CompositionReceipt{}, errors.New("occurrence bound")
	}
	kindCounts := make(map[string]int)
	for _, c := range contributions {
		kindCounts[c.Kind]++
	}
	seenKind := make(map[string]int)
	doc := compositionDocument{
		Family: compositionFamily, Version: compositionVersion, Epoch: epoch,
		Bindings: CompositionBindings{
			InputID: input.ID(), InputDigest: input.Digest(), InputByteLength: input.ByteLength(),
			PolicyID: policy.ID(), PolicyDigest: policy.Digest(), PolicyByteLength: policy.ByteLength(),
			QualificationReceiptCommitment: CompositionCommitment{verified.receiptID, verified.receiptDigest, verified.receiptByteLength},
		},
		Occurrences:       make([]CompositionOccurrence, 0, len(contributions)),
		PairContributions: []PairContribution{}, SeparateLayerContributions: []PairContribution{},
		Custody: input.EndpointCustody(),
	}
	for _, c := range contributions {
		rule, ok := rules[c.Kind]
		if !ok {
			return CompositionReceipt{}, errors.New("missing policy rule")
		}
		seenKind[c.Kind]++
		predecessor := input.PredecessorIdentityForContribution(c)
		custody := input.CustodyIdentityForOccurrence(c.OccurrenceIdentity)
		if predecessor == "" || custody == "" {
			return CompositionReceipt{}, errors.New("missing source identity")
		}
		o := CompositionOccurrence{
			OccurrenceIdentity: c.OccurrenceIdentity, Kind: c.Kind, From: c.From, To: c.To, Ordinal: c.Ordinal,
			Action: rule.Action, RationaleCode: rule.RationaleCode, Layer: rule.Layer, NormalizationBasis: rule.Normalization,
			SourcePredecessorIdentity: predecessor, SourceCustodyIdentity: custody,
		}
		pair := PairContribution{
			OccurrenceIdentity: c.OccurrenceIdentity, Kind: c.Kind, From: c.From, To: c.To,
			Weight: reduceRational(rule.Weight), RationaleCode: rule.RationaleCode,
			NormalizationBasis: rule.Normalization, SourcePredecessorIdentity: predecessor, SourceCustodyIdentity: custody,
		}
		switch rule.Action {
		case "INCLUDE":
			o.TerminalDisposition = "INCLUDED"
			doc.Accounting.Included++
			doc.PairContributions = append(doc.PairContributions, pair)
		case "EXCLUDE":
			o.TerminalDisposition = "EXCLUDED"
			doc.Accounting.Excluded++
		case "DOWNWEIGHT":
			o.TerminalDisposition = "DOWNWEIGHTED"
			doc.Accounting.Downweighted++
			doc.PairContributions = append(doc.PairContributions, pair)
		case "CAP":
			o.CapBefore, o.CapAfter = kindCounts[c.Kind], rule.Cap
			pair.CapBefore, pair.CapAfter = o.CapBefore, o.CapAfter
			if seenKind[c.Kind] <= rule.Cap {
				o.TerminalDisposition = "CAPPED"
				doc.Accounting.Capped++
				doc.PairContributions = append(doc.PairContributions, pair)
			} else {
				o.TerminalDisposition = "EXCLUDED"
				doc.Accounting.Excluded++
			}
		case "NORMALIZE":
			o.TerminalDisposition = "NORMALIZED"
			doc.Accounting.Normalized++
			if rule.Normalization != "BY_KIND_MULTIPLICITY" || kindCounts[c.Kind] <= 0 {
				return CompositionReceipt{}, errors.New("unsupported normalization")
			}
			denominator, err := checkedCompositionProduct(rule.Weight.Denominator, kindCounts[c.Kind])
			if err != nil {
				return CompositionReceipt{}, err
			}
			pair.Weight = reduceRational(Rational{rule.Weight.Numerator, denominator})
			doc.PairContributions = append(doc.PairContributions, pair)
		case "SEPARATE_LAYER":
			o.TerminalDisposition = "SEPARATE_LAYER"
			doc.Accounting.SeparateLayer++
			pair.Layer = rule.Layer
			doc.SeparateLayerContributions = append(doc.SeparateLayerContributions, pair)
		default:
			return CompositionReceipt{}, errors.New("unsupported action")
		}
		doc.Accounting.Terminal++
		doc.Occurrences = append(doc.Occurrences, o)
	}
	if len(doc.PairContributions)+len(doc.SeparateLayerContributions) > limits.MaxPairContributions {
		return CompositionReceipt{}, errors.New("pair contribution bound")
	}
	sort.Slice(doc.PairContributions, func(i, j int) bool {
		return doc.PairContributions[i].CanonicalKey() < doc.PairContributions[j].CanonicalKey()
	})
	sort.Slice(doc.SeparateLayerContributions, func(i, j int) bool {
		return doc.SeparateLayerContributions[i].CanonicalKey() < doc.SeparateLayerContributions[j].CanonicalKey()
	})
	if err := validatePairArithmetic(append(append([]PairContribution(nil), doc.PairContributions...), doc.SeparateLayerContributions...), limits); err != nil {
		return CompositionReceipt{}, err
	}
	digest, err := pairWeightDigest(doc.PairContributions, doc.SeparateLayerContributions)
	if err != nil {
		return CompositionReceipt{}, err
	}
	doc.PairWeightDigest = digest
	return compositionFromDocument(doc, limits)
}

func ParseCompositionReceipt(raw []byte, limits CompositionLimits) (CompositionReceipt, error) {
	if err := validateCompositionLimits(limits); err != nil {
		return CompositionReceipt{}, err
	}
	if len(raw) == 0 || len(raw) > limits.MaxRawBytes || raw[len(raw)-1] != '\n' || (len(raw) > 1 && raw[len(raw)-2] == '\n') || bytes.Contains(raw, []byte{'\r'}) {
		return CompositionReceipt{}, errors.New("invalid canonical framing")
	}
	var doc compositionDocument
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return CompositionReceipt{}, err
	}
	if err := requireCompositionEOF(dec); err != nil {
		return CompositionReceipt{}, err
	}
	r, err := compositionFromDocument(doc, limits)
	if err != nil {
		return CompositionReceipt{}, err
	}
	if !bytes.Equal(raw, r.canonical) {
		return CompositionReceipt{}, errors.New("noncanonical composition receipt")
	}
	return r, nil
}

func compositionFromDocument(doc compositionDocument, limits CompositionLimits) (CompositionReceipt, error) {
	if err := validateCompositionDocument(doc, limits); err != nil {
		return CompositionReceipt{}, err
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return CompositionReceipt{}, err
	}
	if b.Len() > limits.MaxCanonicalBytes {
		return CompositionReceipt{}, errors.New("canonical bound")
	}
	h := sha256.Sum256(b.Bytes())
	return CompositionReceipt{canonical: append([]byte(nil), b.Bytes()...), digest: fmt.Sprintf("sha256:%x", h), doc: cloneCompositionDocument(doc)}, nil
}

func validateCompositionDocument(doc compositionDocument, limits CompositionLimits) error {
	if doc.Family != compositionFamily || doc.Version != compositionVersion || doc.Epoch == 0 {
		return errors.New("invalid composition identity")
	}
	b := doc.Bindings
	if b.InputID == "" || b.InputDigest == "" || b.InputByteLength <= 0 || b.PolicyID == "" || b.PolicyDigest == "" || b.PolicyByteLength <= 0 || b.QualificationReceiptCommitment.ID == "" || b.QualificationReceiptCommitment.Digest == "" || b.QualificationReceiptCommitment.ByteLength <= 0 {
		return errors.New("invalid composition bindings")
	}
	if len(doc.Occurrences) > limits.MaxOccurrences || len(doc.PairContributions)+len(doc.SeparateLayerContributions) > limits.MaxPairContributions {
		return errors.New("composition collection bound")
	}
	if !sort.SliceIsSorted(doc.Occurrences, func(i, j int) bool {
		return doc.Occurrences[i].OccurrenceIdentity < doc.Occurrences[j].OccurrenceIdentity
	}) || !sort.SliceIsSorted(doc.Custody, func(i, j int) bool {
		a, b := doc.Custody[i], doc.Custody[j]
		if a.Endpoint != b.Endpoint {
			return a.Endpoint < b.Endpoint
		}
		if a.OccurrenceIdentity != b.OccurrenceIdentity {
			return a.OccurrenceIdentity < b.OccurrenceIdentity
		}
		return a.EndpointRole < b.EndpointRole
	}) || !sort.SliceIsSorted(doc.PairContributions, func(i, j int) bool {
		return doc.PairContributions[i].CanonicalKey() < doc.PairContributions[j].CanonicalKey()
	}) || !sort.SliceIsSorted(doc.SeparateLayerContributions, func(i, j int) bool {
		return doc.SeparateLayerContributions[i].CanonicalKey() < doc.SeparateLayerContributions[j].CanonicalKey()
	}) {
		return errors.New("noncanonical pair order")
	}
	if maxCompositionString(doc) > limits.MaxStringBytes {
		return errors.New("composition string bound")
	}
	if err := validatePairArithmetic(append(append([]PairContribution(nil), doc.PairContributions...), doc.SeparateLayerContributions...), limits); err != nil {
		return err
	}
	digest, err := pairWeightDigest(doc.PairContributions, doc.SeparateLayerContributions)
	if err != nil || digest != doc.PairWeightDigest {
		return errors.New("pair weight digest")
	}
	if err := validateCompositionReplay(doc); err != nil {
		return err
	}
	return nil
}

func checkedCompositionProduct(a, b int) (int, error) {
	if a <= 0 || b <= 0 || a > math.MaxInt/b {
		return 0, errors.New("composition product overflow")
	}
	return a * b, nil
}

func validateCompositionLimits(l CompositionLimits) error {
	if l.MaxRawBytes <= 0 || l.MaxOccurrences <= 0 || l.MaxPairContributions <= 0 || l.MaxPairs <= 0 || l.MaxCanonicalBytes <= 0 || l.MaxStringBytes <= 0 || l.MaxDenominator <= 0 || l.MaxAccumulator < 0 {
		return errors.New("invalid composition limits")
	}
	return nil
}

func validatePairArithmetic(pairs []PairContribution, limits CompositionLimits) error {
	groups := make(map[string][]Rational)
	for _, p := range pairs {
		r := reduceRational(p.Weight)
		if r != p.Weight || r.Denominator <= 0 || r.Denominator > limits.MaxDenominator {
			return errors.New("invalid pair rational")
		}
		key := p.Kind + "\x00" + p.From + "\x00" + p.To + "\x00" + p.Layer
		groups[key] = append(groups[key], r)
	}
	if len(groups) > limits.MaxPairs {
		return errors.New("pair bound")
	}
	maxAccumulator := limits.MaxAccumulator
	if maxAccumulator == 0 {
		maxAccumulator = math.MaxInt
	}
	for _, weights := range groups {
		common := 1
		for _, r := range weights {
			factor := r.Denominator / gcd(common, r.Denominator)
			if factor != 0 && common > math.MaxInt/factor {
				return errors.New("pair denominator lcm overflow")
			}
			common *= factor
			if common > limits.MaxDenominator {
				return errors.New("pair denominator bound")
			}
		}
		acc := 0
		for _, r := range weights {
			factor := common / r.Denominator
			term, err := checkedSignedCompositionProduct(r.Numerator, factor, math.MaxInt)
			if err != nil {
				return err
			}
			if (term > 0 && acc > math.MaxInt-term) || (term < 0 && acc < -math.MaxInt-term) {
				return errors.New("pair numerator accumulator overflow")
			}
			acc += term
		}
		aggregate := reduceRational(Rational{acc, common})
		if aggregate.Denominator <= 0 || reduceRational(aggregate) != aggregate {
			return errors.New("invalid reduced pair aggregate")
		}
		if aggregate.Numerator > maxAccumulator || aggregate.Numerator < -maxAccumulator {
			return errors.New("pair numerator accumulator overflow")
		}
	}
	return nil
}

func checkedSignedCompositionProduct(value, factor, limit int) (int, error) {
	if factor < 0 || limit < 0 || value > limit || value < -limit {
		return 0, errors.New("pair numerator multiplication overflow")
	}
	if value > 0 && factor > limit/value {
		return 0, errors.New("pair numerator multiplication overflow")
	}
	if value < 0 && factor > limit/-value {
		return 0, errors.New("pair numerator multiplication overflow")
	}
	return value * factor, nil
}

func validateCompositionReplay(doc compositionDocument) error {
	occurrences := make(map[string]CompositionOccurrence, len(doc.Occurrences))
	kindCounts := make(map[string]int)
	for _, o := range doc.Occurrences {
		if o.OccurrenceIdentity == "" || o.Kind == "" || o.From == "" || o.To == "" || o.Ordinal < 0 || o.RationaleCode == "" || o.SourcePredecessorIdentity == "" || o.SourceCustodyIdentity == "" || occurrences[o.OccurrenceIdentity].OccurrenceIdentity != "" {
			return errors.New("invalid occurrence")
		}
		occurrences[o.OccurrenceIdentity] = o
		kindCounts[o.Kind]++
	}
	base := make(map[string]PairContribution, len(doc.PairContributions))
	separate := make(map[string]PairContribution, len(doc.SeparateLayerContributions))
	for _, collection := range []struct {
		pairs    []PairContribution
		separate bool
	}{{doc.PairContributions, false}, {doc.SeparateLayerContributions, true}} {
		for _, p := range collection.pairs {
			o, ok := occurrences[p.OccurrenceIdentity]
			if !ok || base[p.OccurrenceIdentity].OccurrenceIdentity != "" || separate[p.OccurrenceIdentity].OccurrenceIdentity != "" || p.Kind != o.Kind || p.From != o.From || p.To != o.To || p.RationaleCode != o.RationaleCode || p.Layer != o.Layer || p.NormalizationBasis != o.NormalizationBasis || p.CapBefore != o.CapBefore || p.CapAfter != o.CapAfter || p.SourcePredecessorIdentity != o.SourcePredecessorIdentity || p.SourceCustodyIdentity != o.SourceCustodyIdentity || p.Weight.Denominator <= 0 || reduceRational(p.Weight) != p.Weight {
				return errors.New("invalid pair contribution")
			}
			if collection.separate {
				separate[p.OccurrenceIdentity] = p
			} else {
				if p.Weight.Numerator < 0 {
					return errors.New("negative base pair contribution")
				}
				base[p.OccurrenceIdentity] = p
			}
		}
	}
	accounting := compositionAccountingJSON{Terminal: len(doc.Occurrences)}
	seenKind := make(map[string]int)
	weightByRule := make(map[string]Rational)
	for _, o := range doc.Occurrences {
		seenKind[o.Kind]++
		hasBase := base[o.OccurrenceIdentity].OccurrenceIdentity != ""
		hasSeparate := separate[o.OccurrenceIdentity].OccurrenceIdentity != ""
		valid := false
		switch o.Action {
		case "INCLUDE":
			valid = o.TerminalDisposition == "INCLUDED" && hasBase && !hasSeparate && emptyCompositionActionMetadata(o) && base[o.OccurrenceIdentity].Weight == (Rational{1, 1})
			accounting.Included++
		case "EXCLUDE":
			valid = o.TerminalDisposition == "EXCLUDED" && !hasBase && !hasSeparate && emptyCompositionActionMetadata(o)
			accounting.Excluded++
		case "DOWNWEIGHT":
			weight := base[o.OccurrenceIdentity].Weight
			valid = o.TerminalDisposition == "DOWNWEIGHTED" && hasBase && !hasSeparate && emptyCompositionActionMetadata(o) && weight.Numerator > 0 && weight.Numerator <= weight.Denominator
			accounting.Downweighted++
		case "NORMALIZE":
			valid = o.TerminalDisposition == "NORMALIZED" && hasBase && !hasSeparate && o.Layer == "" && o.NormalizationBasis == "BY_KIND_MULTIPLICITY" && o.CapBefore == 0 && o.CapAfter == 0 && base[o.OccurrenceIdentity].Weight == reduceRational(Rational{1, kindCounts[o.Kind]})
			accounting.Normalized++
		case "SEPARATE_LAYER":
			weight := separate[o.OccurrenceIdentity].Weight
			weightAllowed := weight == (Rational{1, 1}) || (o.Kind == "RESOLVES_TO_DEFINITION" && weight == (Rational{-1, 2}))
			valid = o.TerminalDisposition == "SEPARATE_LAYER" && !hasBase && hasSeparate && o.Layer != "" && o.NormalizationBasis == "" && o.CapBefore == 0 && o.CapAfter == 0 && weightAllowed
			accounting.SeparateLayer++
		case "CAP":
			within := seenKind[o.Kind] <= o.CapAfter
			weight := base[o.OccurrenceIdentity].Weight
			weightAllowed := weight == (Rational{1, 1}) || (o.Kind == "REFERENCES_SYMBOL" && weight == (Rational{1, 2}))
			valid = o.CapBefore == kindCounts[o.Kind] && o.CapAfter > 0 && o.Layer == "" && o.NormalizationBasis == "" && ((within && o.TerminalDisposition == "CAPPED" && hasBase && !hasSeparate && weightAllowed) || (!within && o.TerminalDisposition == "EXCLUDED" && !hasBase && !hasSeparate))
			if within {
				accounting.Capped++
			} else {
				accounting.Excluded++
			}
		}
		if !valid {
			return errors.New("occurrence replay mismatch")
		}
		var pair PairContribution
		if hasBase {
			pair = base[o.OccurrenceIdentity]
		} else if hasSeparate {
			pair = separate[o.OccurrenceIdentity]
		}
		if pair.OccurrenceIdentity != "" {
			key := o.Kind + "\x00" + o.Action + "\x00" + o.RationaleCode + "\x00" + o.Layer + "\x00" + o.NormalizationBasis + fmt.Sprintf("\x00%d\x00%d", o.CapBefore, o.CapAfter)
			if prior, ok := weightByRule[key]; ok && prior != pair.Weight {
				return errors.New("pair weight replay mismatch")
			}
			weightByRule[key] = pair.Weight
		}
	}
	if accounting != doc.Accounting || doc.Accounting.Duplicate != 0 || doc.Accounting.Truncated {
		return errors.New("composition accounting")
	}
	return validateCompositionCustody(doc, occurrences)
}

func emptyCompositionActionMetadata(o CompositionOccurrence) bool {
	return o.Layer == "" && o.NormalizationBasis == "" && o.CapBefore == 0 && o.CapAfter == 0
}

func validateCompositionCustody(doc compositionDocument, occurrences map[string]CompositionOccurrence) error {
	byOccurrence := make(map[string][]EndpointCustody, len(occurrences))
	for _, c := range doc.Custody {
		if _, ok := occurrences[c.OccurrenceIdentity]; !ok {
			return errors.New("unknown custody occurrence")
		}
		byOccurrence[c.OccurrenceIdentity] = append(byOccurrence[c.OccurrenceIdentity], c)
	}
	if len(doc.Custody) != 2*len(occurrences) {
		return errors.New("custody occurrence mismatch")
	}
	for identity, o := range occurrences {
		sourceRole, targetRole := compositionEndpointRoles(o.Kind)
		rows := byOccurrence[identity]
		if sourceRole == "" || len(rows) != 2 {
			return errors.New("invalid custody roles")
		}
		matched := map[string]bool{}
		for _, c := range rows {
			if c.PredecessorSelector != o.SourcePredecessorIdentity || c.Custody != "CALLER_ASSERTED" || matched[c.EndpointRole] {
				return errors.New("invalid custody row")
			}
			if (c.EndpointRole == sourceRole && c.Endpoint != o.From) || (c.EndpointRole == targetRole && c.Endpoint != o.To) || (c.EndpointRole != sourceRole && c.EndpointRole != targetRole) {
				return errors.New("custody endpoint or role mismatch")
			}
			matched[c.EndpointRole] = true
		}
		if compositionCustodyIdentity(rows) != o.SourceCustodyIdentity {
			return errors.New("occurrence custody identity mismatch")
		}
	}
	return nil
}

func compositionEndpointRoles(kind string) (string, string) {
	switch kind {
	case "CALLS":
		return "CALLER_OCCURRENCE", "CALLEE_TARGET"
	case "REFERENCES_SYMBOL":
		return "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
	case "RESOLVES_TO_DEFINITION":
		return "QUERY_OCCURRENCE", "DEFINITION_TARGET"
	default:
		return "", ""
	}
}

func compositionCustodyIdentity(rows []EndpointCustody) string {
	rows = append([]EndpointCustody(nil), rows...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Endpoint != rows[j].Endpoint {
			return rows[i].Endpoint < rows[j].Endpoint
		}
		if rows[i].EndpointRole != rows[j].EndpointRole {
			return rows[i].EndpointRole < rows[j].EndpointRole
		}
		if rows[i].PredecessorSelector != rows[j].PredecessorSelector {
			return rows[i].PredecessorSelector < rows[j].PredecessorSelector
		}
		return rows[i].Custody < rows[j].Custody
	})
	h := sha256.New()
	h.Write([]byte("lsp-trace:reference-grouping-occurrence-custody:v1\x00"))
	for _, c := range rows {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\n", c.Endpoint, c.OccurrenceIdentity, c.EndpointRole, c.PredecessorSelector, c.Custody)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func pairWeightDigest(base, separate []PairContribution) (string, error) {
	payload := struct {
		Base     []PairContribution `json:"base"`
		Separate []PairContribution `json:"separate"`
	}{base, separate}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte("lsp-trace:reference-grouping-pair-weights:v1\x00"), raw...))
	return fmt.Sprintf("sha256:%x", h), nil
}

func cloneCompositionDocument(doc compositionDocument) compositionDocument {
	doc.Occurrences = append([]CompositionOccurrence(nil), doc.Occurrences...)
	doc.PairContributions = append([]PairContribution(nil), doc.PairContributions...)
	doc.SeparateLayerContributions = append([]PairContribution(nil), doc.SeparateLayerContributions...)
	doc.Custody = append([]EndpointCustody(nil), doc.Custody...)
	return doc
}

func maxCompositionString(doc compositionDocument) int {
	max := 0
	visit := func(values ...string) {
		for _, value := range values {
			if len(value) > max {
				max = len(value)
			}
		}
	}
	visit(doc.Family, doc.Version, doc.PairWeightDigest, doc.Bindings.InputID, doc.Bindings.InputDigest, doc.Bindings.PolicyID, doc.Bindings.PolicyDigest, doc.Bindings.QualificationReceiptCommitment.ID, doc.Bindings.QualificationReceiptCommitment.Digest)
	for _, o := range doc.Occurrences {
		visit(o.OccurrenceIdentity, o.Kind, o.From, o.To, o.Action, o.RationaleCode, o.Layer, o.NormalizationBasis, o.SourcePredecessorIdentity, o.SourceCustodyIdentity, o.TerminalDisposition)
	}
	for _, p := range append(append([]PairContribution(nil), doc.PairContributions...), doc.SeparateLayerContributions...) {
		visit(p.OccurrenceIdentity, p.Kind, p.From, p.To, p.RationaleCode, p.Layer, p.NormalizationBasis, p.SourcePredecessorIdentity, p.SourceCustodyIdentity)
	}
	for _, c := range doc.Custody {
		visit(c.Endpoint, c.OccurrenceIdentity, c.EndpointRole, c.PredecessorSelector, c.Custody)
	}
	return max
}

func requireCompositionEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
