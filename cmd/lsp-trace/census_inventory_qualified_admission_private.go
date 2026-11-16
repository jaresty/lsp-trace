//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"

	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
	product "lsp-trace/internal/privatehostproductv3"
)

const censusInventoryQualifiedHeaderSchema = "private-connected-inventory-qualified-header.v1"

// This extension is complete preparation DATA. Neither its family nor its
// caller-declared universe is source authority or filesystem verification.
// Omission preserves the pre-extension legacy header's exact canonical bytes.
type censusInventoryQualifiedAdmission struct {
	Qualified           censussourceplan.QualifiedInputV1
	Envelope            censusQualifiedCLIEnvelopePrivate
	PreparationIdentity string
}

// One bounded validation/canonicalization path for every header reference.
// Validate the original caller envelope, then compare the FULL rebuilt qualified
// input. Never substitute AnalysisPlan.Identity for the qualified family digest.
func canonicalCensusInventoryHeader(h censusInventoryHeader) (censusInventoryHeader, []byte, error) {
	fail := func() (censusInventoryHeader, []byte, error) {
		return censusInventoryHeader{}, nil, errCensusInventoryPrivate
	}
	if h.Schema == "" && h.QualifiedAdmission == nil {
		h.Schema = "private-connected-inventory-header.v1"
	}
	raw, e := censusInventoryMarshal(h, 4<<20)
	if e != nil || h.SourceRevision != censusInventorySourceRevision || h.Workspace == "" || h.Workflow == "" || len(h.Workflow) > 4096 {
		return fail()
	}
	if h.QualifiedAdmission == nil {
		if h.Schema != "private-connected-inventory-header.v1" || h.PlanIdentity != h.Plan.Identity || censussourceplan.Validate(h.Plan, 16) != nil {
			return fail()
		}
		return h, raw, nil
	}
	if h.Schema != censusInventoryQualifiedHeaderSchema {
		return fail()
	}
	x := h.QualifiedAdmission
	envRaw, e := json.Marshal(x.Envelope)
	if e != nil {
		return fail()
	}
	env, e := decodeCensusQualifiedCLIPrivate(envRaw, inventoryscope.LimitsV1{})
	if e != nil {
		return fail()
	}
	q, e := validateCensusQualifiedCLIPrivate(env)
	if e != nil || censussourceplan.ValidateQualifiedV1(x.Qualified, inventoryscope.LimitsV1{}) != nil {
		return fail()
	}
	rebuilt, e := json.Marshal(q)
	if e != nil {
		return fail()
	}
	supplied, e := json.Marshal(x.Qualified)
	if e != nil || !bytes.Equal(rebuilt, supplied) {
		return fail()
	}
	_, identity, e := censusQualifiedCLIIdentityPrivate(env, q)
	plan, e2 := json.Marshal(h.Plan)
	expected, e3 := json.Marshal(q.AnalysisPlan)
	if e != nil || e2 != nil || e3 != nil || !bytes.Equal(plan, expected) || x.PreparationIdentity != identity || h.PlanIdentity != q.QualifiedDigest || h.Workspace != q.Workspace || h.Scope != q.Scope.ScopeDigest || h.Provenance != env.Universe.Claim || env.Preparation.RevisionAssertion != h.SourceRevision || q.PartitionSize != 16 {
		return fail()
	}
	return h, raw, nil
}

// Expected binding remains data; the successor checks the actual admitted job.
func censusInventoryQualifiedNativeBinding(h censusInventoryHeader, digest string, q product.InventoryRequestV3) bool {
	_, raw, e := canonicalCensusInventoryHeader(h)
	if e != nil || censusInventoryDigest(raw) != digest {
		return false
	}
	// A legacy header does not erase an explicit source expectation. Bind
	// every nonempty expectation before either family's eligibility rule.
	for _, owner := range q.Inventory.NativeOwners {
		if owner.ExpectedSourceAuthorization != "" && owner.ExpectedSourceAuthorization != digest {
			return false
		}
	}
	if h.QualifiedAdmission == nil {
		return true
	}
	return len(q.Inventory.NativeOwners) == 1 && q.Inventory.NativeOwners[0].ExpectedSourceAuthorization == digest
}

func censusInventoryPartitionAvailable(h censusInventoryHeader, ordinal int) bool {
	if ordinal < 0 || ordinal >= len(h.Plan.Partitions) {
		return false
	}
	if h.QualifiedAdmission == nil {
		return true
	}
	if h.QualifiedAdmission.Qualified.NoWork {
		return false
	}
	for _, f := range h.Plan.Partitions[ordinal].Members {
		if f.Disposition != "" {
			return false
		}
	}
	return len(h.Plan.Partitions[ordinal].Members) > 0
}
