package censusacquisition

import (
	"bytes"
	"errors"
	"fmt"

	"lsp-trace/internal/census"
	"lsp-trace/internal/seedformat"
)

// ValidateBatchSeeds is a pure validator. It neither executes acquisition nor
// admits or returns an authoritative batch result.
func ValidateBatchSeeds(request BatchRequest, workspace string) error {
	seenOrdinal, seenIdentity, seenCoordinate, seenCanonical := map[int]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, target := range request.Targets {
		if target.CensusOrdinal < 0 || seenOrdinal[target.CensusOrdinal] || target.SymbolIdentity == "" || seenIdentity[target.SymbolIdentity] {
			return fmt.Errorf("target %d duplicate ordinal or identity", i)
		}
		coordinate := fmt.Sprintf("%s\x00%d\x00%d", target.URI, target.Position().Line, target.Position().Character)
		if seenCoordinate[coordinate] || seenCanonical[string(target.CanonicalSeedV2)] {
			return fmt.Errorf("target %d duplicate coordinate or canonical bytes", i)
		}
		if err := reconcileSeed(target, workspace); err != nil {
			return fmt.Errorf("target %d: %w", i, err)
		}
		seenOrdinal[target.CensusOrdinal], seenIdentity[target.SymbolIdentity], seenCoordinate[coordinate], seenCanonical[string(target.CanonicalSeedV2)] = true, true, true, true
	}
	provided, err := seedformat.Decode(request.CanonicalSeedsV2, workspace)
	if err != nil {
		return fmt.Errorf("invalid provided canonical Seeds V2: %w", err)
	}
	canonical, err := seedformat.EncodeCanonical(provided, workspace)
	if err != nil || !bytes.Equal(canonical, request.CanonicalSeedsV2) {
		return errors.New("provided Seeds V2 is not canonical")
	}
	recomputed, err := combineSeedsForPlanning(request.Targets, workspace, PlanningConfig{DownDepth: request.DownDepth, UpDepth: request.UpDepth})
	if err != nil || !bytes.Equal(recomputed, request.CanonicalSeedsV2) {
		return errors.New("provided Seeds V2 does not exactly match ordered targets")
	}
	return nil
}

// CombineCanonicalSeeds is a pure canonicalization helper for package-main
// tests using the census default traversal depths.
func CombineCanonicalSeeds(targets []PreparedTarget, workspace string) ([]byte, error) {
	return combineSeedsForPlanning(targets, workspace, PlanningConfig{DownDepth: census.DefaultDownDepth, UpDepth: census.DefaultUpDepth})
}

// ValidateBatchArtifact is a pure closed validation step. Successful validation
// does not mint custody, execute acquisition, or produce an authoritative result.
func ValidateBatchArtifact(raw []byte, request BatchRequest) error {
	_, err := admit(raw, request)
	return err
}
