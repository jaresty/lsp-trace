package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const InspectionVersionV1 = "lsp-trace.inspect.v1"

// InspectionBytes returns the exact committed Draft 2020-12 inspection schema bytes.
func InspectionBytes(version string) ([]byte, error) {
	if version != InspectionVersionV1 {
		return nil, fmt.Errorf("unsupported inspection schema version %q", version)
	}
	return files.ReadFile("schemas/" + InspectionVersionV1 + ".schema.json")
}

// ValidateInspection validates an inspection projection independently of graph schemas.
func ValidateInspection(data []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid inspection JSON: %w", err)
	}
	raw, err := InspectionBytes(InspectionVersionV1)
	if err != nil {
		return err
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("embedded inspection schema %s: %w", InspectionVersionV1, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	resource := "https://jaresty.github.io/lsp-trace/schemas/" + InspectionVersionV1 + ".schema.json"
	if err := compiler.AddResource(resource, schemaDoc); err != nil {
		return fmt.Errorf("embedded inspection schema %s: %w", InspectionVersionV1, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return fmt.Errorf("embedded inspection schema %s: %w", InspectionVersionV1, err)
	}
	if err := compiled.Validate(doc); err != nil {
		return fmt.Errorf("inspection schema validation %s: %w", InspectionVersionV1, err)
	}
	return nil
}

const inspectionAuthority = "NON_AUTHORITATIVE_DERIVED_VIEW"

type allSeedInspection struct {
	InspectionSchemaVersion string `json:"inspection_schema_version"`
	ProjectionKind          string `json:"projection_kind"`
	Authority               string `json:"authority"`
	Records                 struct {
		Nodes                 []map[string]any `json:"nodes"`
		CallRelations         []map[string]any `json:"call_relations"`
		DispatchRelationships []map[string]any `json:"dispatch_relationships"`
		SiblingCandidates     []map[string]any `json:"sibling_candidates"`
		Diagnostics           []map[string]any `json:"diagnostics"`
		Terminals             []map[string]any `json:"terminals"`
		Frontier              []map[string]any `json:"frontier"`
	} `json:"records"`
	Seeds []struct {
		PreparationStatus string `json:"preparation_status"`
		Seed              struct {
			Label string `json:"label"`
		} `json:"seed"`
		SeedMemberships []struct {
			SeedLabel    string `json:"seed_label"`
			EvidenceKind string `json:"evidence_kind"`
			EndpointID   string `json:"endpoint_id"`
		} `json:"seed_memberships"`
		NativeNodeIDs               []string `json:"native_node_ids"`
		NativeCallRelationIDs       []string `json:"native_call_relation_ids"`
		DiscoveryNominationIDs      []string `json:"discovery_nomination_ids"`
		CorrelatedDiagnosticIndexes []int    `json:"correlated_diagnostic_indexes"`
	} `json:"seeds"`
	Accounting inspectionAccounting `json:"accounting"`
}

type inspectionAccounting struct {
	RequestedSeedCount                     int    `json:"requested_seed_count"`
	SuccessfulSeedCount                    int    `json:"successful_seed_count"`
	FailedSeedCount                        int    `json:"failed_seed_count"`
	SuccessfulSeedWithMembershipCount      int    `json:"successful_seed_with_membership_count"`
	SuccessfulSeedWithoutMembershipCount   int    `json:"successful_seed_without_membership_count"`
	GlobalNodeRecordCount                  int    `json:"global_node_record_count"`
	GlobalCallRelationRecordCount          int    `json:"global_call_relation_record_count"`
	GlobalDispatchRelationshipRecordCount  int    `json:"global_dispatch_relationship_record_count"`
	GlobalSiblingCandidateRecordCount      int    `json:"global_sibling_candidate_record_count"`
	GlobalDiagnosticRecordCount            int    `json:"global_diagnostic_record_count"`
	GlobalTerminalRecordCount              int    `json:"global_terminal_record_count"`
	GlobalFrontierRecordCount              int    `json:"global_frontier_record_count"`
	SeedMembershipRecordCount              int    `json:"seed_membership_record_count"`
	SeedNodeReferenceCount                 int    `json:"seed_node_reference_count"`
	SeedCallRelationReferenceCount         int    `json:"seed_call_relation_reference_count"`
	SeedDiscoveryNominationReferenceCount  int    `json:"seed_discovery_nomination_reference_count"`
	SeedCorrelatedDiagnosticReferenceCount int    `json:"seed_correlated_diagnostic_reference_count"`
	Truncated                              bool   `json:"truncated"`
	TraversalComplete                      bool   `json:"traversal_complete"`
	SourceGraphComplete                    string `json:"source_graph_complete"`
}

// ValidateAllSeedInspection admits an arbitrary ALL_SEEDS inspection document by
// applying structural validation followed by semantic, reference, and accounting checks.
func ValidateAllSeedInspection(data []byte) error {
	if err := ValidateInspection(data); err != nil {
		return err
	}
	var projection allSeedInspection
	if err := json.Unmarshal(data, &projection); err != nil {
		return fmt.Errorf("invalid inspection JSON: %w", err)
	}
	if projection.InspectionSchemaVersion != InspectionVersionV1 || projection.ProjectionKind != "ALL_SEEDS" || projection.Authority != inspectionAuthority {
		return fmt.Errorf("input must be %s ALL_SEEDS", InspectionVersionV1)
	}

	nodes, err := inspectionIdentities("NODE", projection.Records.Nodes, "id")
	if err != nil {
		return err
	}
	calls, err := inspectionIdentities("CALL_RELATION", projection.Records.CallRelations, "relation_id")
	if err != nil {
		return err
	}
	dispatches, err := inspectionIdentities("DISPATCH_RELATIONSHIP", projection.Records.DispatchRelationships, "relation_id")
	if err != nil {
		return err
	}
	siblings, err := inspectionIdentities("SIBLING_CANDIDATE", projection.Records.SiblingCandidates, "relation_id")
	if err != nil {
		return err
	}

	expected := inspectionAccounting{
		RequestedSeedCount:                    len(projection.Seeds),
		GlobalNodeRecordCount:                 len(projection.Records.Nodes),
		GlobalCallRelationRecordCount:         len(projection.Records.CallRelations),
		GlobalDispatchRelationshipRecordCount: len(projection.Records.DispatchRelationships),
		GlobalSiblingCandidateRecordCount:     len(projection.Records.SiblingCandidates),
		GlobalDiagnosticRecordCount:           len(projection.Records.Diagnostics),
		GlobalTerminalRecordCount:             len(projection.Records.Terminals),
		GlobalFrontierRecordCount:             len(projection.Records.Frontier),
		Truncated:                             projection.Accounting.Truncated,
		TraversalComplete:                     projection.Accounting.TraversalComplete,
		SourceGraphComplete:                   projection.Accounting.SourceGraphComplete,
	}
	labels := map[string]struct{}{}
	for _, seed := range projection.Seeds {
		label := seed.Seed.Label
		if _, duplicate := labels[label]; duplicate {
			return fmt.Errorf("duplicate seed label %q in inspection input", label)
		}
		labels[label] = struct{}{}
		if seed.PreparationStatus == "FAILED" {
			expected.FailedSeedCount++
			if len(seed.SeedMemberships)+len(seed.NativeNodeIDs)+len(seed.NativeCallRelationIDs)+len(seed.DiscoveryNominationIDs)+len(seed.CorrelatedDiagnosticIndexes) != 0 {
				return fmt.Errorf("failed seed %q has filterable references", label)
			}
		} else if seed.PreparationStatus == "SUCCEEDED" {
			expected.SuccessfulSeedCount++
			if len(seed.SeedMemberships) == 0 {
				expected.SuccessfulSeedWithoutMembershipCount++
			} else {
				expected.SuccessfulSeedWithMembershipCount++
			}
		} else {
			return fmt.Errorf("invalid preparation status %q for seed %q", seed.PreparationStatus, label)
		}
		if err := validateInspectionReferences("NODE", label, seed.NativeNodeIDs, nodes); err != nil {
			return err
		}
		if err := validateInspectionReferences("CALL_RELATION", label, seed.NativeCallRelationIDs, calls); err != nil {
			return err
		}
		nominations := make([]string, 0, len(seed.SeedMemberships))
		dispatchReferences := make([]string, 0, len(seed.SeedMemberships))
		siblingReferences := make([]string, 0, len(seed.SeedMemberships))
		for _, membership := range seed.SeedMemberships {
			if membership.SeedLabel != label {
				return fmt.Errorf("seed membership label %q does not match seed %q", membership.SeedLabel, label)
			}
			switch membership.EvidenceKind {
			case "DISPATCH_ASSOCIATION":
				dispatchReferences = append(dispatchReferences, membership.EndpointID)
				nominations = append(nominations, membership.EndpointID)
			case "SIBLING_CANDIDATE":
				siblingReferences = append(siblingReferences, membership.EndpointID)
				nominations = append(nominations, membership.EndpointID)
			}
		}
		if err := validateInspectionReferences("DISPATCH_RELATIONSHIP", label, dispatchReferences, dispatches); err != nil {
			return err
		}
		if err := validateInspectionReferences("SIBLING_CANDIDATE", label, siblingReferences, siblings); err != nil {
			return err
		}
		if !reflect.DeepEqual(nominations, seed.DiscoveryNominationIDs) {
			return fmt.Errorf("discovery nomination references for seed %q do not match seed memberships", label)
		}
		seenDiagnostics := map[int]struct{}{}
		for _, index := range seed.CorrelatedDiagnosticIndexes {
			if index < 0 || index >= len(projection.Records.Diagnostics) {
				return fmt.Errorf("unresolved DIAGNOSTIC_CORRELATION reference %d for seed %q", index, label)
			}
			if _, duplicate := seenDiagnostics[index]; duplicate {
				return fmt.Errorf("duplicate DIAGNOSTIC_CORRELATION reference %d for seed %q", index, label)
			}
			seenDiagnostics[index] = struct{}{}
		}
		expected.SeedMembershipRecordCount += len(seed.SeedMemberships)
		expected.SeedNodeReferenceCount += len(seed.NativeNodeIDs)
		expected.SeedCallRelationReferenceCount += len(seed.NativeCallRelationIDs)
		expected.SeedDiscoveryNominationReferenceCount += len(seed.DiscoveryNominationIDs)
		expected.SeedCorrelatedDiagnosticReferenceCount += len(seed.CorrelatedDiagnosticIndexes)
	}
	if !reflect.DeepEqual(expected, projection.Accounting) {
		return fmt.Errorf("invalid inspection accounting: got %+v want %+v", projection.Accounting, expected)
	}
	return nil
}

func inspectionIdentities(namespace string, records []map[string]any, field string) (map[string]struct{}, error) {
	identities := make(map[string]struct{}, len(records))
	for _, record := range records {
		identity, ok := record[field].(string)
		if !ok || identity == "" {
			return nil, fmt.Errorf("%s record has invalid %s", namespace, field)
		}
		if _, duplicate := identities[identity]; duplicate {
			return nil, fmt.Errorf("duplicate %s record identity %q", namespace, identity)
		}
		identities[identity] = struct{}{}
	}
	return identities, nil
}

func validateInspectionReferences(namespace, label string, references []string, identities map[string]struct{}) error {
	if err := rejectDuplicateReferences(namespace, label, references); err != nil {
		return err
	}
	for _, reference := range references {
		if _, found := identities[reference]; !found {
			return fmt.Errorf("unresolved %s reference %q for seed %q", namespace, reference, label)
		}
	}
	return nil
}

func rejectDuplicateReferences(namespace, label string, references []string) error {
	seen := map[string]struct{}{}
	for _, reference := range references {
		if _, duplicate := seen[reference]; duplicate {
			return fmt.Errorf("duplicate %s reference %q for seed %q", namespace, reference, label)
		}
		seen[reference] = struct{}{}
	}
	return nil
}
