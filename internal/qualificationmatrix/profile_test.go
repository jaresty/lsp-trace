package qualificationmatrix

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func testProfile(t *testing.T) Profile {
	t.Helper()
	p := Profile{
		SchemaVersion:              SchemaVersion,
		ProfileID:                  "substrate-v1",
		Version:                    "1.0.0",
		Authority:                  "release-council",
		CustodyReceiptID:           "receipt-profile-1",
		CustodyAuthenticationState: "AUTHENTICATED",
		Axes: []Axis{
			{Name: "relation_family", Members: []string{"CALLS", "TYPE_RELATION"}},
			{Name: "custody_adapter", Members: []string{"GIT_WORKTREE", "IMMUTABLE_MANIFEST"}},
			{Name: "state", Members: []string{"COMPLETE", "PARTIAL", "FAILED", "UNSUPPORTED", "UNKNOWN"}},
			{Name: "projection_class", Members: []string{"DIRECTED", "UNDIRECTED"}},
			{Name: "transport", Members: []string{"CLI", "MCP"}},
			{Name: "publication_mode", Members: []string{"INLINE", "IMMUTABLE"}},
			{Name: "provider_class", Members: []string{"TYPESCRIPT_LANGUAGE_SERVER", "ELIXIR_LS"}},
		},
		Products: []Product{
			{ID: "relation-transport", Version: "1", Axes: []string{"relation_family", "transport"}},
			{ID: "custody", Version: "1", Axes: []string{"custody_adapter"}},
			{ID: "state", Version: "1", Axes: []string{"state"}},
			{ID: "projection", Version: "1", Axes: []string{"projection_class"}},
			{ID: "publication", Version: "1", Axes: []string{"publication_mode"}},
			{ID: "provider", Version: "1", Axes: []string{"provider_class"}},
		},
	}
	cells, err := Generate(Profile{
		SchemaVersion: p.SchemaVersion, ProfileID: p.ProfileID, Version: p.Version,
		Authority: p.Authority, CustodyReceiptID: p.CustodyReceiptID, CustodyAuthenticationState: p.CustodyAuthenticationState,
		Axes: p.Axes, Products: p.Products,
	})
	if err == nil && len(cells) > 0 {
		p.FoundationalCellIDs = []string{cells[0].ID}
	} else {
		p.FoundationalCellIDs = []string{"placeholder"}
	}
	return p
}

func TestProfileRequiresNormativeIdentityAndMandatoryAxes(t *testing.T) {
	p := testProfile(t)
	if err := ValidateProfile(p); err != nil {
		t.Fatalf("ASSERT_PROFILE_COMPLETE: %v", err)
	}
	p.Axes = p.Axes[1:]
	if err := ValidateProfile(p); err == nil || !strings.Contains(err.Error(), "relation_family") {
		t.Fatalf("ASSERT_PROFILE_MISSING_AXIS_REJECTED: %v", err)
	}
}

func TestGeneratorProducesAtomicDeterministicProduct(t *testing.T) {
	p := testProfile(t)
	first, err := Generate(p)
	if err != nil {
		t.Fatalf("ASSERT_GENERATOR_ACCEPTS_PROFILE: %v", err)
	}
	second, err := Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 17 {
		t.Fatalf("ASSERT_EXACT_PRODUCT_ROWS: got %d want 17", len(first))
	}
	for i := range first {
		if !matchesDeclaredProduct(first[i], p.Products) {
			t.Fatalf("ASSERT_ATOMIC_CELL_%d: %#v", i, first[i])
		}
		if first[i].ID != second[i].ID {
			t.Fatalf("ASSERT_STABLE_CELL_ID_%d: %q != %q", i, first[i].ID, second[i].ID)
		}
	}
	changed := p
	changed.Products = append([]Product(nil), p.Products...)
	changed.Products[0].Version = "2"
	versioned, err := Generate(changed)
	if err != nil {
		t.Fatal(err)
	}
	originalID := findCellID(first, "relation-transport", map[string]string{"relation_family": "CALLS", "transport": "CLI"})
	versionedID := findCellID(versioned, "relation-transport", map[string]string{"relation_family": "CALLS", "transport": "CLI"})
	if versionedID == originalID {
		t.Fatalf("ASSERT_PRODUCT_VERSION_BINDS_CELL_ID: %q", originalID)
	}
	b1, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) || len(b1) == 0 || b1[len(b1)-1] != '\n' {
		t.Fatalf("ASSERT_CANONICAL_BYTES: equal=%v bytes=%q", bytes.Equal(b1, b2), b1)
	}
}

func TestEquivalenceReductionRequiresVersionedEvidenceAndExactOmissions(t *testing.T) {
	p := testProfile(t)
	all, err := Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	var omittedID string
	for _, cell := range all {
		if cell.ProductID == "relation-transport" && cell.Values["relation_family"] == "CALLS" && cell.Values["transport"] == "MCP" {
			omittedID = cell.ID
		}
	}
	p.EquivalenceRules = []EquivalenceRule{{ID: "same-transport", Version: "1", ProductID: "relation-transport", ExercisedTuple: []string{"CALLS", "CLI"}, OmittedTuples: [][]string{{"CALLS", "MCP"}}, EvidenceReceiptIDs: []string{"receipt-equivalence-1"}}}
	if err := ValidateProfile(p); err != nil {
		t.Fatalf("ASSERT_EQUIVALENCE_ACCEPTED: %v", err)
	}
	foundations := p.FoundationalCellIDs
	p.FoundationalCellIDs = []string{omittedID}
	if err := ValidateProfile(p); err == nil || !strings.Contains(err.Error(), "non-waivable") {
		t.Fatalf("ASSERT_FOUNDATIONAL_REDUCTION_REJECTED: %v", err)
	}
	p.FoundationalCellIDs = foundations
	p.EquivalenceRules[0].EvidenceReceiptIDs = nil
	if err := ValidateProfile(p); err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("ASSERT_UNSUPPORTED_REDUCTION_REJECTED: %v", err)
	}
}

func TestWaiverCannotAdmitFoundationalExpiredOrProducerApprovedCell(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	p := testProfile(t)
	cells, err := Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	results := passingResults(cells)
	results[0].Status = StatusBlocked
	results[0].Waiver = validWaiver(cells[0].ID, now)
	err = ProgramBAdmitted(p, AdmissionRequest{Results: results}, now)
	if err == nil || !strings.Contains(err.Error(), "non-waivable") {
		t.Fatalf("ASSERT_FOUNDATIONAL_WAIVER_REJECTED: %v", err)
	}

	p.FoundationalCellIDs = []string{cells[1].ID}
	expired := now.Add(-time.Hour)
	results[0].Waiver.ExpiresAt = &expired
	err = ProgramBAdmitted(p, AdmissionRequest{Results: results}, now)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("ASSERT_EXPIRED_WAIVER_REJECTED: %v", err)
	}

	results[0].Waiver = validWaiver(cells[0].ID, now)
	results[0].Waiver.ApprovingPrincipal = results[0].Waiver.ArtifactProducer
	err = ProgramBAdmitted(p, AdmissionRequest{Results: results}, now)
	if err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("ASSERT_SELF_APPROVED_WAIVER_REJECTED: %v", err)
	}
}

func TestAdmissionEnforcesBlockedOperationsAndAdmitsCompleteMatrix(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	p := testProfile(t)
	cells, err := Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	p.FoundationalCellIDs = []string{cells[1].ID}
	results := passingResults(cells)
	results[0].Status = StatusBlocked
	results[0].Waiver = validWaiver(cells[0].ID, now)
	err = ProgramBAdmitted(p, AdmissionRequest{Results: results, RequestedOperations: []string{"pagerank"}}, now)
	if err == nil || !strings.Contains(err.Error(), "blocked operation") {
		t.Fatalf("ASSERT_BLOCKED_OPERATION_ENFORCED: %v", err)
	}
	if err := ProgramBAdmitted(p, AdmissionRequest{Results: results}, now); err != nil {
		t.Fatalf("ASSERT_PROGRAM_B_ADMITTED_WITH_VALID_WAIVER: %v", err)
	}
	if err := ProgramBAdmitted(p, AdmissionRequest{Results: results[:len(results)-1]}, now); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("ASSERT_MISSING_CELL_REJECTED: %v", err)
	}
}

func findCellID(cells []Cell, product string, values map[string]string) string {
	for _, cell := range cells {
		if cell.ProductID != product {
			continue
		}
		match := len(cell.Values) == len(values)
		for key, value := range values {
			if cell.Values[key] != value {
				match = false
			}
		}
		if match {
			return cell.ID
		}
	}
	return ""
}

func matchesDeclaredProduct(c Cell, products []Product) bool {
	for _, product := range products {
		if len(c.Values) != len(product.Axes) {
			continue
		}
		matched := true
		for _, axis := range product.Axes {
			if _, ok := c.Values[axis]; !ok {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func values(c Cell, axes []string) []string {
	out := make([]string, len(axes))
	for i, a := range axes {
		out[i] = c.Values[a]
	}
	return out
}
func passingResults(cells []Cell) []Result {
	out := make([]Result, len(cells))
	for i, c := range cells {
		out[i] = Result{CellID: c.ID, Status: StatusPass, RealServerEvidence: true}
	}
	return out
}
func validWaiver(id string, now time.Time) *Waiver {
	expiry := now.Add(24 * time.Hour)
	return &Waiver{TupleID: id, Status: "APPROVED_WAIVER", Rationale: "bounded provider gap", ApprovingPrincipal: "release-council", ArtifactProducer: "artifact-bot", EvidenceProducer: "evidence-bot", PolicyID: "waiver-policy-v1", PolicyProvisioningReceiptID: "receipt-policy-1", PolicyAuthenticationState: "AUTHENTICATED", ExpiresAt: &expiry, BlockedClaims: []string{"native provider pass"}, BlockedOperations: []string{"pagerank"}}
}
