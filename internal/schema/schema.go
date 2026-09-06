package schema

import (
	"embed"
	"fmt"
	"strings"

	"lsp-trace/internal/graph"
)

//go:embed schemas/*.json
var files embed.FS

const (
	FamilyGraph                    = "graph"
	FamilyInspect                  = "inspect"
	FamilyFilter                   = "filter"
	FamilySourceManifest           = "source-manifest"
	FamilyTrustProvisioningReceipt = "trust-provisioning-receipt"
	FamilySourceDenominator        = "source-denominator"
	FamilyTypeOverlayQualification = "analysis-only-type-overlay-qualification"

	SourceDenominatorVersionV1        = "lsp-trace.source-denominator.v1"
	TypeOverlayQualificationVersionV1 = "lsp-trace.analysis-only-type-overlay-qualification.v1"
)

var familyVersions = map[string]map[string]string{
	FamilyGraph:                    {"v1": graph.SchemaVersionV1, "v2": graph.SchemaVersionV2, "v3": graph.SchemaVersionV3, "v4": "lsp-trace.graph.v4"},
	FamilyInspect:                  {"v1": "lsp-trace.inspect.v1"},
	FamilyFilter:                   {"v1": "lsp-trace.filter.v1"},
	FamilySourceManifest:           {"v1": "lsp-trace.source-manifest.v1"},
	FamilyTrustProvisioningReceipt: {"v1": "lsp-trace.trust-provisioning-receipt.v1"},
	FamilySourceDenominator:        {"v1": SourceDenominatorVersionV1},
	FamilyTypeOverlayQualification: {"v1": TypeOverlayQualificationVersionV1},
}

var versionFields = map[string]string{
	FamilyGraph:                    "schema_version",
	FamilyInspect:                  "inspection_schema_version",
	FamilyFilter:                   "filter_schema_version",
	FamilySourceManifest:           "source_manifest_schema_version",
	FamilyTrustProvisioningReceipt: "trust_provisioning_receipt_schema_version",
	FamilySourceDenominator:        "denominator_schema_version",
	FamilyTypeOverlayQualification: "schema_version",
}

func normalizeFamily(family, version string) (string, string, error) {
	family, version = strings.TrimSpace(family), strings.TrimSpace(version)
	versions, ok := familyVersions[family]
	if !ok {
		return "", "", fmt.Errorf("unsupported schema family %q", family)
	}
	if full, ok := versions[version]; ok {
		return version, full, nil
	}
	for short, full := range versions {
		if version == full {
			return short, full, nil
		}
	}
	return "", "", fmt.Errorf("unsupported schema version %q for family %q", version, family)
}

// BytesFor returns the exact committed Draft 2020-12 schema bytes for a family and version.
func BytesFor(family, version string) ([]byte, error) {
	_, full, err := normalizeFamily(family, version)
	if err != nil {
		return nil, err
	}
	return files.ReadFile("schemas/" + full + ".schema.json")
}

// Bytes preserves the graph-family schema lookup compatibility alias.
func Bytes(version string) ([]byte, error) { return BytesFor(FamilyGraph, version) }

// ValidateFor runs structural validation before family-specific semantics.
func ValidateFor(data []byte, family, requested string) (string, error) {
	structural, err := ValidateStructure(data, family, requested)
	if err != nil {
		return "", err
	}
	if err := ValidateSemantics(data, structural); err != nil {
		return "", err
	}
	return structural.Version, nil
}

// Validate preserves the graph-family validation compatibility alias.
func Validate(data []byte, requested string) (string, error) {
	return ValidateFor(data, FamilyGraph, requested)
}
