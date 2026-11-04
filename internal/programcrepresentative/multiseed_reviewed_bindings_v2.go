package programcrepresentative

// This declaration-only file is excluded from its own later reviewed seam
// manifest. Binding-only revisions may supply real reviewed pins/provenance;
// they must not change verifier or admission semantics.
func BuildStage6ImplementationV2(maxSourceBytes uint64) (Stage6ImplementationDTO, error) {
	return Stage6ImplementationDTO{}, ErrMultiSeedV2ReviewRequired
}

// ReviewedHarnessActorBindingV2 is non-wire repository-owned provenance data.
// Actor.Projection must be nil; Projection binds the exact native harness
// assignment/terminal projection independently inspected during real review.
// Caller review DTOs and synthetic actors cannot populate this declaration.
type ReviewedHarnessActorBindingV2 struct {
	Actor      RecordedReviewActorV2
	Projection ArtifactBindingDTO
}

// ReviewedHarnessActorBindingsV2 remains unissued until the subject's actual
// independent harness review exists. A future data-only return must allocate
// fresh literals and contain actual inspected session/task provenance, not
// aliases, caller assurances, test anchors, or invented actors.
func ReviewedHarnessActorBindingsV2() ([]ReviewedHarnessActorBindingV2, error) {
	return nil, ErrMultiSeedV2ReviewRequired
}
