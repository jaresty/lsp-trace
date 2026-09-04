package qualificationmatrix

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const SchemaVersion = "lsp-trace.qualification-matrix-profile.v1"

var mandatoryAxes = []string{"relation_family", "custody_adapter", "state", "projection_class", "transport", "publication_mode", "provider_class"}

type Axis struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}
type Product struct {
	ID      string   `json:"id"`
	Version string   `json:"version"`
	Axes    []string `json:"axes"`
}
type EquivalenceRule struct {
	ID                 string     `json:"id"`
	Version            string     `json:"version"`
	ProductID          string     `json:"product_id"`
	ExercisedTuple     []string   `json:"exercised_tuple"`
	OmittedTuples      [][]string `json:"omitted_tuples"`
	EvidenceReceiptIDs []string   `json:"evidence_receipt_ids"`
}
type Profile struct {
	SchemaVersion              string            `json:"schema_version"`
	ProfileID                  string            `json:"profile_id"`
	Version                    string            `json:"version"`
	Authority                  string            `json:"authority"`
	CustodyReceiptID           string            `json:"custody_receipt_id"`
	CustodyAuthenticationState string            `json:"custody_authentication_state"`
	Axes                       []Axis            `json:"axes"`
	Products                   []Product         `json:"products"`
	EquivalenceRules           []EquivalenceRule `json:"equivalence_rules,omitempty"`
	FoundationalCellIDs        []string          `json:"foundational_cell_ids"`
}
type Cell struct {
	ID             string            `json:"id"`
	ProductID      string            `json:"product_id"`
	ProductVersion string            `json:"product_version"`
	Values         map[string]string `json:"values"`
}
type Status string

const (
	StatusPass         Status = "PASS"
	StatusBlocked      Status = "BLOCKED"
	StatusFail         Status = "FAIL"
	StatusNotQualified Status = "NOT_QUALIFIED"
)

type Waiver struct {
	TupleID                     string     `json:"tuple_id"`
	Status                      string     `json:"status"`
	Rationale                   string     `json:"rationale"`
	ApprovingPrincipal          string     `json:"approving_principal"`
	ArtifactProducer            string     `json:"artifact_producer"`
	EvidenceProducer            string     `json:"evidence_producer"`
	PolicyID                    string     `json:"policy_id"`
	PolicyProvisioningReceiptID string     `json:"policy_provisioning_receipt_id"`
	PolicyAuthenticationState   string     `json:"policy_authentication_state"`
	ExpiresAt                   *time.Time `json:"expires_at,omitempty"`
	RevalidateWhen              string     `json:"revalidate_when,omitempty"`
	BlockedClaims               []string   `json:"blocked_claims"`
	BlockedOperations           []string   `json:"blocked_operations"`
}
type Result struct {
	CellID                string  `json:"cell_id"`
	Status                Status  `json:"status"`
	RealServerEvidence    bool    `json:"real_server_evidence"`
	EvidenceProviderClass string  `json:"evidence_provider_class,omitempty"`
	Waiver                *Waiver `json:"waiver,omitempty"`
}
type AdmissionRequest struct {
	Results             []Result `json:"results"`
	RequestedClaims     []string `json:"requested_claims,omitempty"`
	RequestedOperations []string `json:"requested_operations,omitempty"`
}

func ValidateProfile(p Profile) error {
	axes, products, raw, err := validateCore(p)
	if err != nil {
		return err
	}
	_ = axes
	byID := map[string]bool{}
	tuples := map[string]bool{}
	tupleIDs := map[string]string{}
	for _, c := range raw {
		byID[c.ID] = true
		key := tupleKey(c.ProductID, valuesForProduct(c, products[c.ProductID].Axes))
		tuples[key] = true
		tupleIDs[key] = c.ID
	}
	seenRules := map[string]bool{}
	omittedIDs := map[string]bool{}
	for i, r := range p.EquivalenceRules {
		if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.Version) == "" || strings.TrimSpace(r.ProductID) == "" {
			return fmt.Errorf("equivalence rule %d requires id, version, and product_id", i)
		}
		if seenRules[r.ID] {
			return fmt.Errorf("duplicate equivalence rule %q", r.ID)
		}
		seenRules[r.ID] = true
		if _, ok := products[r.ProductID]; !ok {
			return fmt.Errorf("equivalence rule %q references unsupported product %q", r.ID, r.ProductID)
		}
		if len(r.EvidenceReceiptIDs) == 0 {
			return fmt.Errorf("equivalence rule %q requires retained evidence receipt", r.ID)
		}
		exercised := tupleKey(r.ProductID, r.ExercisedTuple)
		if !tuples[exercised] {
			return fmt.Errorf("equivalence rule %q exercised tuple is not generated", r.ID)
		}
		if len(r.OmittedTuples) == 0 {
			return fmt.Errorf("equivalence rule %q must identify omitted tuples", r.ID)
		}
		seen := map[string]bool{}
		for _, tuple := range r.OmittedTuples {
			k := tupleKey(r.ProductID, tuple)
			if k == exercised || !tuples[k] || seen[k] {
				return fmt.Errorf("equivalence rule %q has unsupported omitted tuple", r.ID)
			}
			seen[k] = true
			omittedIDs[tupleIDs[k]] = true
		}
	}
	if len(p.FoundationalCellIDs) == 0 {
		return fmt.Errorf("foundational_cell_ids is required")
	}
	seenFound := map[string]bool{}
	for _, id := range p.FoundationalCellIDs {
		if !byID[id] {
			return fmt.Errorf("foundational cell %q is not generated", id)
		}
		if omittedIDs[id] {
			return fmt.Errorf("non-waivable foundational cell %q cannot be removed by equivalence", id)
		}
		if seenFound[id] {
			return fmt.Errorf("duplicate foundational cell %q", id)
		}
		seenFound[id] = true
	}
	return nil
}

func validateCore(p Profile) (map[string]Axis, map[string]Product, []Cell, error) {
	if p.SchemaVersion != SchemaVersion {
		return nil, nil, nil, fmt.Errorf("schema_version must be %q", SchemaVersion)
	}
	for n, v := range map[string]string{"profile_id": p.ProfileID, "version": p.Version, "authority": p.Authority, "custody_receipt_id": p.CustodyReceiptID} {
		if strings.TrimSpace(v) == "" {
			return nil, nil, nil, fmt.Errorf("%s is required", n)
		}
	}
	if p.CustodyAuthenticationState != "AUTHENTICATED" {
		return nil, nil, nil, fmt.Errorf("custody_authentication_state must be AUTHENTICATED")
	}
	axes, err := axisIndex(p.Axes)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, n := range mandatoryAxes {
		if _, ok := axes[n]; !ok {
			return nil, nil, nil, fmt.Errorf("mandatory axis %q is required", n)
		}
	}
	if len(p.Products) == 0 {
		return nil, nil, nil, fmt.Errorf("at least one Cartesian product is required")
	}
	products := map[string]Product{}
	used := map[string]bool{}
	var out []Cell
	seenCells := map[string]bool{}
	for i, product := range p.Products {
		if strings.TrimSpace(product.ID) == "" || strings.TrimSpace(product.Version) == "" {
			return nil, nil, nil, fmt.Errorf("product %d requires id and version", i)
		}
		if _, ok := products[product.ID]; ok {
			return nil, nil, nil, fmt.Errorf("duplicate product %q", product.ID)
		}
		if len(product.Axes) == 0 {
			return nil, nil, nil, fmt.Errorf("product %q has no axes", product.ID)
		}
		seenAxis := map[string]bool{}
		for _, n := range product.Axes {
			if _, ok := axes[n]; !ok {
				return nil, nil, nil, fmt.Errorf("product %q references unsupported axis %q", product.ID, n)
			}
			if seenAxis[n] {
				return nil, nil, nil, fmt.Errorf("product %q contains duplicate axis %q", product.ID, n)
			}
			seenAxis[n] = true
			used[n] = true
		}
		products[product.ID] = product
		tuples := []map[string]string{{}}
		for _, name := range product.Axes {
			var next []map[string]string
			for _, base := range tuples {
				for _, member := range axes[name].Members {
					v := make(map[string]string, len(base)+1)
					for k, x := range base {
						v[k] = x
					}
					v[name] = member
					next = append(next, v)
				}
			}
			tuples = next
		}
		for _, v := range tuples {
			id := cellID(product.ID, product.Version, v)
			if seenCells[id] {
				return nil, nil, nil, fmt.Errorf("duplicate generated tuple %s", id)
			}
			seenCells[id] = true
			out = append(out, Cell{ID: id, ProductID: product.ID, ProductVersion: product.Version, Values: v})
		}
	}
	for _, n := range mandatoryAxes {
		if !used[n] {
			return nil, nil, nil, fmt.Errorf("mandatory axis %q is not assigned to a Cartesian product", n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return axes, products, out, nil
}

func axisIndex(in []Axis) (map[string]Axis, error) {
	out := map[string]Axis{}
	for i, a := range in {
		if strings.TrimSpace(a.Name) == "" {
			return nil, fmt.Errorf("axis %d has empty name", i)
		}
		if _, ok := out[a.Name]; ok {
			return nil, fmt.Errorf("duplicate axis %q", a.Name)
		}
		if len(a.Members) == 0 {
			return nil, fmt.Errorf("axis %q has no members", a.Name)
		}
		seen := map[string]bool{}
		for _, m := range a.Members {
			if strings.TrimSpace(m) == "" || strings.Contains(m, ",") {
				return nil, fmt.Errorf("axis %q has invalid composite member %q", a.Name, m)
			}
			if seen[m] {
				return nil, fmt.Errorf("axis %q has duplicate member %q", a.Name, m)
			}
			seen[m] = true
		}
		out[a.Name] = a
	}
	return out, nil
}

func Generate(p Profile) ([]Cell, error) {
	_, products, raw, err := validateCore(p)
	if err != nil {
		return nil, err
	}
	if len(p.FoundationalCellIDs) > 0 {
		if err := ValidateProfile(p); err != nil {
			return nil, err
		}
	} else {
		q := p
		q.FoundationalCellIDs = []string{raw[0].ID}
		if err := ValidateProfile(q); err != nil {
			return nil, err
		}
	}
	omitted := map[string]bool{}
	for _, r := range p.EquivalenceRules {
		for _, v := range r.OmittedTuples {
			omitted[tupleKey(r.ProductID, v)] = true
		}
	}
	out := make([]Cell, 0, len(raw))
	for _, c := range raw {
		if !omitted[tupleKey(c.ProductID, valuesForProduct(c, products[c.ProductID].Axes))] {
			out = append(out, c)
		}
	}
	return out, nil
}
func cellID(product, version string, v map[string]string) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("QUALIFICATION_CELL_V1\x00")
	b.WriteString(product)
	b.WriteByte(0)
	b.WriteString(version)
	b.WriteByte(0)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v[k])
		b.WriteByte(0)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "qcell_" + hex.EncodeToString(sum[:])
}
func valuesForProduct(c Cell, axes []string) []string {
	out := make([]string, len(axes))
	for i, a := range axes {
		out[i] = c.Values[a]
	}
	return out
}
func tupleKey(product string, v []string) string { return product + "\x00" + strings.Join(v, "\x00") }

func CanonicalBytes(p Profile) ([]byte, error) {
	if err := ValidateProfile(p); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var clone Profile
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil, err
	}
	sort.Slice(clone.Axes, func(i, j int) bool { return clone.Axes[i].Name < clone.Axes[j].Name })
	for i := range clone.Axes {
		sort.Strings(clone.Axes[i].Members)
	}
	sort.Slice(clone.Products, func(i, j int) bool { return clone.Products[i].ID < clone.Products[j].ID })
	sort.Strings(clone.FoundationalCellIDs)
	sort.Slice(clone.EquivalenceRules, func(i, j int) bool { return clone.EquivalenceRules[i].ID < clone.EquivalenceRules[j].ID })
	for i := range clone.EquivalenceRules {
		sort.Strings(clone.EquivalenceRules[i].EvidenceReceiptIDs)
		sort.Slice(clone.EquivalenceRules[i].OmittedTuples, func(a, b int) bool {
			return strings.Join(clone.EquivalenceRules[i].OmittedTuples[a], "\x00") < strings.Join(clone.EquivalenceRules[i].OmittedTuples[b], "\x00")
		})
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(clone); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func ProgramBAdmitted(p Profile, req AdmissionRequest, now time.Time) error {
	if err := ValidateProfile(p); err != nil {
		return err
	}
	cells, err := Generate(p)
	if err != nil {
		return err
	}
	required := map[string]Cell{}
	for _, c := range cells {
		required[c.ID] = c
	}
	foundational := map[string]bool{}
	for _, id := range p.FoundationalCellIDs {
		foundational[id] = true
	}
	found := map[string]bool{}
	for _, r := range req.Results {
		cell, ok := required[r.CellID]
		if !ok {
			return fmt.Errorf("unknown or reduced cell %q", r.CellID)
		}
		if found[r.CellID] {
			return fmt.Errorf("duplicate result for cell %q", r.CellID)
		}
		found[r.CellID] = true
		if !r.RealServerEvidence {
			return fmt.Errorf("cell %q lacks required real-server evidence", r.CellID)
		}
		if want := cell.Values["provider_class"]; want != "" && r.EvidenceProviderClass != "" && r.EvidenceProviderClass != want {
			return fmt.Errorf("companion provider %q cannot replace native provider %q", r.EvidenceProviderClass, want)
		}
		switch r.Status {
		case StatusPass:
			if r.Waiver != nil {
				return fmt.Errorf("PASS cell %q cannot carry a waiver", r.CellID)
			}
			continue
		case StatusBlocked, StatusFail, StatusNotQualified:
		default:
			return fmt.Errorf("cell %q has unsupported status %q", r.CellID, r.Status)
		}
		if r.Waiver == nil {
			return fmt.Errorf("cell %q is %s without APPROVED_WAIVER", r.CellID, r.Status)
		}
		if foundational[r.CellID] {
			return fmt.Errorf("non-waivable foundational cell %q cannot carry a waiver", r.CellID)
		}
		if err := validateWaiver(*r.Waiver, r.CellID, now); err != nil {
			return err
		}
		for _, x := range req.RequestedClaims {
			if contains(r.Waiver.BlockedClaims, x) {
				return fmt.Errorf("blocked claim %q depends on waiver for cell %q", x, r.CellID)
			}
		}
		for _, x := range req.RequestedOperations {
			if contains(r.Waiver.BlockedOperations, x) {
				return fmt.Errorf("blocked operation %q depends on waiver for cell %q", x, r.CellID)
			}
		}
	}
	for id := range required {
		if !found[id] {
			return fmt.Errorf("missing required cell %q", id)
		}
	}
	return nil
}
func validateWaiver(w Waiver, id string, now time.Time) error {
	if w.TupleID != id {
		return fmt.Errorf("waiver tuple %q does not match cell %q", w.TupleID, id)
	}
	if w.Status != "APPROVED_WAIVER" {
		return fmt.Errorf("waiver for cell %q is not APPROVED_WAIVER", id)
	}
	for n, v := range map[string]string{"rationale": w.Rationale, "approving_principal": w.ApprovingPrincipal, "policy_id": w.PolicyID, "policy_provisioning_receipt_id": w.PolicyProvisioningReceiptID} {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("waiver %s is required", n)
		}
	}
	if w.PolicyAuthenticationState != "AUTHENTICATED" {
		return fmt.Errorf("waiver policy must be independently AUTHENTICATED")
	}
	if w.ApprovingPrincipal == w.ArtifactProducer || w.ApprovingPrincipal == w.EvidenceProducer {
		return fmt.Errorf("waiver approving principal must be distinct from artifact and evidence producers")
	}
	if w.ExpiresAt == nil && strings.TrimSpace(w.RevalidateWhen) == "" {
		return fmt.Errorf("waiver requires bounded expiry or revalidation condition")
	}
	if w.ExpiresAt != nil && !now.Before(*w.ExpiresAt) {
		return fmt.Errorf("waiver for cell %q expired", id)
	}
	if len(w.BlockedClaims) == 0 || len(w.BlockedOperations) == 0 {
		return fmt.Errorf("waiver must name blocked claims and operations")
	}
	return nil
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
