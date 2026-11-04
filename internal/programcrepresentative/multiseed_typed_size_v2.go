// Code generated from the accepted fixed DTO field shapes; DO NOT EDIT.
package programcrepresentative

import "fmt"

func (z *canonicalSizerV2) sizePositionDTO(v PositionDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("line", true)
	z.num(uint64(v.Line))
	z.key("character", false)
	z.num(uint64(v.Character))
}
func CanonicalSizePositionDTO(v PositionDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizePositionDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeRangeDTO(v RangeDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("start", true)
	z.sizePositionDTO(v.Start)
	z.key("end", false)
	z.sizePositionDTO(v.End)
}
func CanonicalSizeRangeDTO(v RangeDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeRangeDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeArtifactBindingDTO(v ArtifactBindingDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("identity", false)
	z.str(v.Identity)
	z.key("sha256", false)
	z.str(v.SHA256)
	z.key("byte_length", false)
	z.num(uint64(v.ByteLength))
}
func CanonicalSizeArtifactBindingDTO(v ArtifactBindingDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeArtifactBindingDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeMembershipIdentityDTO(v MembershipIdentityDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("membership_id", true)
	z.str(v.MembershipID)
	z.key("execution_bundle_id", false)
	z.str(v.ExecutionBundleID)
	z.key("seed_label", false)
	z.str(v.SeedLabel)
	z.key("seed_at", false)
	z.str(v.SeedAt)
	z.key("evidence_kind", false)
	z.str(v.EvidenceKind)
	z.key("endpoint_id", false)
	z.str(v.EndpointID)
}
func CanonicalSizeMembershipIdentityDTO(v MembershipIdentityDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeMembershipIdentityDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeEvidenceIdentityDTO(v EvidenceIdentityDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("constituent_selector", false)
	z.str(v.ConstituentSelector)
	z.key("execution_bundle_id", false)
	z.str(v.ExecutionBundleID)
	z.key("relation_id", false)
	z.str(v.RelationID)
	z.key("call_site_ordinal", false)
	z.integer(v.CallSiteOrdinal)
	z.key("from_node_id", false)
	z.str(v.FromNodeID)
	z.key("to_node_id", false)
	z.str(v.ToNodeID)
	z.key("call_site", false)
	z.sizeRangeDTO(v.CallSite)
	z.key("receipt_selector", false)
	z.str(v.ReceiptSelector)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeEvidenceIdentityDTO(v EvidenceIdentityDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeEvidenceIdentityDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeCustodyIdentityDTO(v CustodyIdentityDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("workflow_key", false)
	z.str(v.WorkflowKey)
	z.key("terminal_program_c_head_selector", false)
	z.str(v.TerminalProgramCHeadSelector)
	z.key("receipt_selector", false)
	z.str(v.ReceiptSelector)
	z.key("receipt_ordinal", false)
	z.integer(v.ReceiptOrdinal)
	z.key("constituent_ordinal", false)
	z.integer(v.ConstituentOrdinal)
	z.key("publication_selector", false)
	z.str(v.PublicationSelector)
	z.key("constituent_selector", false)
	z.str(v.ConstituentSelector)
	z.key("native_v5_sha256", false)
	z.str(v.NativeV5SHA256)
	z.key("native_v5_byte_length", false)
	z.num(uint64(v.NativeV5ByteLength))
	z.key("execution_bundle_id", false)
	z.str(v.ExecutionBundleID)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeCustodyIdentityDTO(v CustodyIdentityDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeCustodyIdentityDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeSeedGroupIdentityDTOv2(v SeedGroupIdentityDTOv2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("execution_bundle_id", false)
	z.str(v.ExecutionBundleID)
	z.key("invocation_seed_ordinal", false)
	z.integer(v.InvocationSeedOrdinal)
	z.key("seed_label", false)
	z.str(v.SeedLabel)
	z.key("seed_at", false)
	z.str(v.SeedAt)
	z.key("resolved_uri", false)
	z.str(v.ResolvedURI)
	z.key("content_sha256", false)
	z.str(v.ContentSHA256)
	z.key("language_id", false)
	z.str(v.LanguageID)
	z.key("memberships", false)
	if v.Memberships == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Memberships {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeMembershipIdentityDTO(item)
	}
	z.key("prepared_targets", false)
	if v.PreparedTargets == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.PreparedTargets {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("reached_nodes", false)
	if v.ReachedNodes == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.ReachedNodes {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("relation_ids", false)
	if v.RelationIDs == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.RelationIDs {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("occurrence_ids", false)
	if v.OccurrenceIDs == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.OccurrenceIDs {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeSeedGroupIdentityDTOv2(v SeedGroupIdentityDTOv2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeSeedGroupIdentityDTOv2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeSelectionContextIdentityDTOv2(v SelectionContextIdentityDTOv2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("adapter_contract_sha256", false)
	z.str(v.AdapterContractSHA256)
	z.key("custody_identity", false)
	z.str(v.CustodyIdentity)
	z.key("constituent_selector", false)
	z.str(v.ConstituentSelector)
	z.key("execution_bundle_id", false)
	z.str(v.ExecutionBundleID)
	z.key("run_id", false)
	z.str(v.RunID)
	z.key("seed", false)
	z.num(uint64(v.Seed))
	z.key("replay_ordinal", false)
	z.num(uint64(v.ReplayOrdinal))
	z.key("community_ordinal", false)
	z.integer(v.CommunityOrdinal)
	z.key("community_artifact_sha256", false)
	z.str(v.CommunityArtifactSHA256)
	z.key("community_members", false)
	if v.CommunityMembers == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.CommunityMembers {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("seed_groups", false)
	if v.SeedGroups == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.SeedGroups {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeSeedGroupIdentityDTOv2(item)
	}
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeSelectionContextIdentityDTOv2(v SelectionContextIdentityDTOv2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeSelectionContextIdentityDTOv2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeMechanicsIdentityDTOv2(v MechanicsIdentityDTOv2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("adapter_contract_sha256", false)
	z.str(v.AdapterContractSHA256)
	z.key("custody_identity", false)
	z.str(v.CustodyIdentity)
	z.key("receipt_selector", false)
	z.str(v.ReceiptSelector)
	z.key("run_id", false)
	z.str(v.RunID)
	z.key("seed", false)
	z.num(uint64(v.Seed))
	z.key("replay_ordinal", false)
	z.num(uint64(v.ReplayOrdinal))
	z.key("community_ordinal", false)
	z.integer(v.CommunityOrdinal)
	z.key("community_artifact_sha256", false)
	z.str(v.CommunityArtifactSHA256)
	z.key("evidence", false)
	if v.Evidence == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Evidence {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeEvidenceIdentityDTO(item)
	}
	z.key("community_members", false)
	if v.CommunityMembers == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.CommunityMembers {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("selection_context", false)
	z.sizeSelectionContextIdentityDTOv2(v.SelectionContext)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeMechanicsIdentityDTOv2(v MechanicsIdentityDTOv2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeMechanicsIdentityDTOv2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeAdapterContractDTOv2(v AdapterContractDTOv2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("domains", false)
	if v.Domains == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Domains {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("native_schema_version", false)
	z.str(v.NativeSchemaVersion)
	z.key("program_c_profile_id", false)
	z.str(v.ProgramCProfileID)
	z.key("program_c_profile_sha256", false)
	z.str(v.ProgramCProfileSHA256)
	z.key("program_c_algorithm", false)
	z.str(v.ProgramCAlgorithm)
	z.key("program_c_algorithm_version", false)
	z.str(v.ProgramCAlgorithmVersion)
	z.key("run_seeds", false)
	if v.RunSeeds == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.RunSeeds {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.num(uint64(item))
	}
	z.key("replays_per_seed", false)
	z.num(uint64(v.ReplaysPerSeed))
	z.key("run_id_grammar", false)
	z.str(v.RunIDGrammar)
	z.key("occurrence_ordinal_rule", false)
	z.str(v.OccurrenceOrdinalRule)
	z.key("membership_kinds", false)
	if v.MembershipKinds == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.MembershipKinds {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("mapped_membership_kinds", false)
	if v.MappedMembershipKinds == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.MappedMembershipKinds {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("community_order_rule", false)
	z.str(v.CommunityOrderRule)
	z.key("seed_group_order_rule", false)
	z.str(v.SeedGroupOrderRule)
	z.key("seed_group_key_rule", false)
	z.str(v.SeedGroupKeyRule)
	z.key("membership_partition_rule", false)
	z.str(v.MembershipPartitionRule)
	z.key("group_occurrence_closure_rule", false)
	z.str(v.GroupOccurrenceClosureRule)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeAdapterContractDTOv2(v AdapterContractDTOv2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeAdapterContractDTOv2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeStage6ImplementationDTO(v Stage6ImplementationDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("repository_commit", true)
	z.str(v.RepositoryCommit)
	z.key("repository_tree", false)
	z.str(v.RepositoryTree)
	z.key("files", false)
	if v.Files == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Files {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeImplementationFileDTO(item)
	}
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeStage6ImplementationDTO(v Stage6ImplementationDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeStage6ImplementationDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeImplementationFileDTO(v ImplementationFileDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("path", true)
	z.str(v.Path)
	z.key("git_blob_sha1", false)
	z.str(v.GitBlobSHA1)
	z.key("sha256", false)
	z.str(v.SHA256)
	z.key("byte_length", false)
	z.num(uint64(v.ByteLength))
}
func CanonicalSizeImplementationFileDTO(v ImplementationFileDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeImplementationFileDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeCorpusIdentityDTO(v CorpusIdentityDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("frozen_corpus_sha256", true)
	z.str(v.FrozenCorpusSHA256)
	z.key("census_receipt_sha256", false)
	z.str(v.CensusReceiptSHA256)
	z.key("availability_ledger_sha256", false)
	z.str(v.AvailabilityLedgerSHA256)
	z.key("program_c_plan_sha256", false)
	z.str(v.ProgramCPlanSHA256)
	z.key("program_c_bindings_sha256", false)
	z.str(v.ProgramCBindingsSHA256)
}
func CanonicalSizeCorpusIdentityDTO(v CorpusIdentityDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeCorpusIdentityDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeQualificationWorkflowIdentityDTO(v QualificationWorkflowIdentityDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("profile", true)
	z.str(v.Profile)
	z.key("contract", false)
	z.str(v.Contract)
	z.key("census_receipt_sha256", false)
	z.str(v.CensusReceiptSHA256)
	z.key("availability_ledger_sha256", false)
	z.str(v.AvailabilityLedgerSHA256)
	z.key("source_pin_sha256", false)
	z.str(v.SourcePinSHA256)
	z.key("program_c_plan_sha256", false)
	z.str(v.ProgramCPlanSHA256)
	z.key("program_c_bindings_sha256", false)
	z.str(v.ProgramCBindingsSHA256)
	z.key("privacy_policy_sha256", false)
	z.str(v.PrivacyPolicySHA256)
	z.key("retention_policy_sha256", false)
	z.str(v.RetentionPolicySHA256)
	z.key("resource_policy_sha256", false)
	z.str(v.ResourcePolicySHA256)
	z.key("replay_policy_sha256", false)
	z.str(v.ReplayPolicySHA256)
	z.key("qualification_policy_sha256", false)
	z.str(v.QualificationPolicySHA256)
	z.key("provenance", false)
	z.str(v.Provenance)
}
func CanonicalSizeQualificationWorkflowIdentityDTO(v QualificationWorkflowIdentityDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeQualificationWorkflowIdentityDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeReceiptInventoryEntryDTO(v ReceiptInventoryEntryDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("plan_ordinal", true)
	z.integer(v.PlanOrdinal)
	z.key("receipt_ordinal", false)
	z.integer(v.ReceiptOrdinal)
	z.key("receipt_schema_version", false)
	z.str(v.ReceiptSchemaVersion)
	z.key("receipt_selector", false)
	z.str(v.ReceiptSelector)
	z.key("receipt_sha256", false)
	z.str(v.ReceiptSHA256)
	z.key("receipt_byte_length", false)
	z.num(uint64(v.ReceiptByteLength))
}
func CanonicalSizeReceiptInventoryEntryDTO(v ReceiptInventoryEntryDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeReceiptInventoryEntryDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeTerminalProgramCHeadDTO(v TerminalProgramCHeadDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("workflow_key", false)
	z.str(v.WorkflowKey)
	z.key("head_key", false)
	z.str(v.HeadKey)
	z.key("generation", false)
	z.num(uint64(v.Generation))
	z.key("selector", false)
	z.str(v.Selector)
	z.key("head_bytes_sha256", false)
	z.str(v.HeadBytesSHA256)
	z.key("byte_length", false)
	z.num(uint64(v.ByteLength))
	z.key("committed_prefix", false)
	z.integer(v.CommittedPrefix)
	z.key("constituent_count", false)
	z.integer(v.ConstituentCount)
}
func CanonicalSizeTerminalProgramCHeadDTO(v TerminalProgramCHeadDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeTerminalProgramCHeadDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeQualificationClaimCeilingDTO(v QualificationClaimCeilingDTO) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("authority", true)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("feature_identity", false)
	z.str(v.FeatureIdentity)
	z.key("completeness", false)
	z.str(v.Completeness)
	z.key("effects", false)
	z.str(v.Effects)
	z.key("reviews", false)
	z.str(v.Reviews)
	z.key("implementation_decision", false)
	z.str(v.ImplementationDecision)
	z.key("production_admission", false)
	z.str(v.ProductionAdmission)
	z.key("source_graph_complete", false)
	z.str(v.SourceGraphComplete)
	z.key("absence_inference", false)
	z.str(v.AbsenceInference)
}
func CanonicalSizeQualificationClaimCeilingDTO(v QualificationClaimCeilingDTO) (uint64, error) {
	var z canonicalSizerV2
	z.sizeQualificationClaimCeilingDTO(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeCompositionV2(v CompositionV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("mechanics", false)
	z.sizeMechanicsIdentityDTOv2(v.Mechanics)
	z.key("custody", false)
	z.sizeCustodyIdentityDTO(v.Custody)
	z.key("policy", false)
	z.sizeArtifactBindingDTO(v.Policy)
	z.key("qualification_envelope", false)
	z.sizeArtifactBindingDTO(v.QualificationEnvelope)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeCompositionV2(v CompositionV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeCompositionV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeGroupingV2(v GroupingV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("composition", false)
	z.sizeArtifactBindingDTO(v.Composition)
	z.key("limits", false)
	z.sizeArtifactBindingDTO(v.Limits)
	z.key("algorithm", false)
	z.str(v.Algorithm)
	z.key("numeric_contract", false)
	z.str(v.NumericContract)
	z.key("selection_context", false)
	z.sizeSelectionContextIdentityDTOv2(v.SelectionContext)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeGroupingV2(v GroupingV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeGroupingV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeDerivedQualificationV2(v DerivedQualificationV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("composition", false)
	z.sizeArtifactBindingDTO(v.Composition)
	z.key("grouping", false)
	z.sizeArtifactBindingDTO(v.Grouping)
	z.key("envelope", false)
	z.sizeArtifactBindingDTO(v.Envelope)
	z.key("valid_from", false)
	z.num(uint64(v.ValidFrom))
	z.key("valid_through", false)
	z.num(uint64(v.ValidThrough))
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeDerivedQualificationV2(v DerivedQualificationV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeDerivedQualificationV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizePredecessorV2(v PredecessorV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("occurrence_id", true)
	z.str(v.OccurrenceID)
	z.key("relation_id", false)
	z.str(v.RelationID)
	z.key("caller_id", false)
	z.str(v.CallerID)
	z.key("target_id", false)
	z.str(v.TargetID)
	z.key("call_site", false)
	z.sizeRangeDTO(v.CallSite)
}
func CanonicalSizePredecessorV2(v PredecessorV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizePredecessorV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeRecordV2(v RecordV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("outcome", true)
	z.str(v.Outcome)
	z.key("selected_node", false)
	z.str(v.SelectedNode)
	z.key("distance", false)
	z.integer(v.Distance)
	z.key("scc_members", false)
	if v.SCCMembers == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.SCCMembers {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("incoming_predecessors", false)
	if v.IncomingPredecessors == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.IncomingPredecessors {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizePredecessorV2(item)
	}
}
func CanonicalSizeRecordV2(v RecordV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeRecordV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeGroupResultV2(v GroupResultV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("invocation_seed_ordinal", true)
	z.integer(v.InvocationSeedOrdinal)
	z.key("seed_group_identity", false)
	z.str(v.SeedGroupIdentity)
	z.key("record", false)
	z.sizeRecordV2(v.Record)
}
func CanonicalSizeGroupResultV2(v GroupResultV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeGroupResultV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeGroupAccountingV2(v GroupAccountingV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("expected", true)
	z.num(uint64(v.Expected))
	z.key("selected", false)
	z.num(uint64(v.Selected))
	z.key("no_calls", false)
	z.num(uint64(v.NoCalls))
	z.key("calls_ineligible", false)
	z.num(uint64(v.CallsIneligible))
	z.key("unresolved", false)
	z.num(uint64(v.Unresolved))
}
func CanonicalSizeGroupAccountingV2(v GroupAccountingV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeGroupAccountingV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeResultV2(v ResultV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("composition", false)
	z.sizeCompositionV2(v.Composition)
	z.key("grouping", false)
	z.sizeGroupingV2(v.Grouping)
	z.key("qualification", false)
	z.sizeDerivedQualificationV2(v.Qualification)
	z.key("terminal", false)
	z.str(v.Terminal)
	z.key("group_results", false)
	if v.GroupResults == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.GroupResults {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeGroupResultV2(item)
	}
	z.key("group_accounting", false)
	z.sizeGroupAccountingV2(v.GroupAccounting)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeResultV2(v ResultV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeResultV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeStage6PolicyV2(v Stage6PolicyV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("admitted_kind", false)
	z.str(v.AdmittedKind)
	z.key("admitted_disposition", false)
	z.str(v.AdmittedDisposition)
	z.key("self_loop_disposition", false)
	z.str(v.SelfLoopDisposition)
	z.key("context_node_scope", false)
	z.str(v.ContextNodeScope)
	z.key("relation_occurrence_closure", false)
	z.str(v.RelationOccurrenceClosure)
	z.key("graph_direction", false)
	z.str(v.GraphDirection)
	z.key("strongly_connected_components", false)
	z.str(v.StronglyConnectedComponents)
	z.key("prepared_root_distance", false)
	z.str(v.PreparedRootDistance)
	z.key("candidate_scope", false)
	z.str(v.CandidateScope)
	z.key("prepared_root_scope", false)
	z.str(v.PreparedRootScope)
	z.key("selected_scc_members_scope", false)
	z.str(v.SelectedSCCMembersScope)
	z.key("predecessor_scope", false)
	z.str(v.PredecessorScope)
	z.key("no_calls_outcome", false)
	z.str(v.NoCallsOutcome)
	z.key("selection_failure_outcome", false)
	z.str(v.SelectionFailureOutcome)
	z.key("selected_outcome", false)
	z.str(v.SelectedOutcome)
	z.key("selection_order", false)
	if v.SelectionOrder == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.SelectionOrder {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("predecessor_order", false)
	if v.PredecessorOrder == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.PredecessorOrder {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("outer_terminal_rule", false)
	z.str(v.OuterTerminalRule)
	z.key("group_mapping_rule", false)
	z.str(v.GroupMappingRule)
	z.key("no_outer_winner", false)
	z.boolean(v.NoOuterWinner)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeStage6PolicyV2(v Stage6PolicyV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeStage6PolicyV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeStage6LimitsV2(v Stage6LimitsV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("max_communities", false)
	z.num(uint64(v.MaxCommunities))
	z.key("max_members", false)
	z.num(uint64(v.MaxMembers))
	z.key("max_occurrences", false)
	z.num(uint64(v.MaxOccurrences))
	z.key("max_candidates", false)
	z.num(uint64(v.MaxCandidates))
	z.key("max_seed_groups", false)
	z.num(uint64(v.MaxSeedGroups))
	z.key("max_memberships", false)
	z.num(uint64(v.MaxMemberships))
	z.key("max_memberships_per_group", false)
	z.num(uint64(v.MaxMembershipsPerGroup))
	z.key("max_group_occurrences", false)
	z.num(uint64(v.MaxGroupOccurrences))
	z.key("max_sum_group_occurrences", false)
	z.num(uint64(v.MaxSumGroupOccurrences))
	z.key("max_string_bytes", false)
	z.num(uint64(v.MaxStringBytes))
	z.key("max_outer_work", false)
	z.num(uint64(v.MaxOuterWork))
	z.key("max_run_source_work", false)
	z.num(uint64(v.MaxRunSourceWork))
	z.key("max_canonical_bytes", false)
	z.num(uint64(v.MaxCanonicalBytes))
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeStage6LimitsV2(v Stage6LimitsV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeStage6LimitsV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeStage6FinalizationV2(v Stage6FinalizationV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("composition_schema", false)
	z.str(v.CompositionSchema)
	z.key("grouping_schema", false)
	z.str(v.GroupingSchema)
	z.key("qualification_schema", false)
	z.str(v.QualificationSchema)
	z.key("result_schema", false)
	z.str(v.ResultSchema)
	z.key("mechanics_schema", false)
	z.str(v.MechanicsSchema)
	z.key("context_schema", false)
	z.str(v.ContextSchema)
	z.key("composition_mappings", false)
	if v.CompositionMappings == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.CompositionMappings {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("grouping_mappings", false)
	if v.GroupingMappings == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.GroupingMappings {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("group_mappings", false)
	if v.GroupMappings == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.GroupMappings {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("qualification_mappings", false)
	if v.QualificationMappings == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.QualificationMappings {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("finalization_order", false)
	if v.FinalizationOrder == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.FinalizationOrder {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeStage6FinalizationV2(v Stage6FinalizationV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeStage6FinalizationV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeQualificationSubjectV2(v QualificationSubjectV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("accepted_adr_sha256", false)
	z.str(v.AcceptedADRSHA256)
	z.key("accepted_receipt_discovery_amendment_sha256", false)
	z.str(v.AcceptedReceiptDiscoveryAmendmentSHA256)
	z.key("accepted_prequalification_mechanics_amendment_sha256", false)
	z.str(v.AcceptedPrequalificationMechanicsAmendmentSHA256)
	z.key("accepted_qualification_subject_amendment_sha256", false)
	z.str(v.AcceptedQualificationSubjectAmendmentSHA256)
	z.key("accepted_multiseed_mechanics_amendment_sha256", false)
	z.str(v.AcceptedMultiseedMechanicsAmendmentSHA256)
	z.key("accepted_downstream_amendment_sha256", false)
	z.str(v.AcceptedDownstreamAmendmentSHA256)
	z.key("adapter_contract", false)
	z.sizeAdapterContractDTOv2(v.AdapterContract)
	z.key("mechanics_schema_version", false)
	z.str(v.MechanicsSchemaVersion)
	z.key("mechanics_profile", false)
	z.str(v.MechanicsProfile)
	z.key("stage6_implementation", false)
	z.sizeStage6ImplementationDTO(v.Stage6Implementation)
	z.key("stage6_policy", false)
	z.sizeArtifactBindingDTO(v.Stage6Policy)
	z.key("stage6_limits", false)
	z.sizeArtifactBindingDTO(v.Stage6Limits)
	z.key("stage6_finalization", false)
	z.sizeArtifactBindingDTO(v.Stage6Finalization)
	z.key("corpus_identity", false)
	z.sizeCorpusIdentityDTO(v.CorpusIdentity)
	z.key("workflow_identity", false)
	z.sizeQualificationWorkflowIdentityDTO(v.WorkflowIdentity)
	z.key("source_pin_sha256", false)
	z.str(v.SourcePinSHA256)
	z.key("receipt_inventory", false)
	if v.ReceiptInventory == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.ReceiptInventory {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeReceiptInventoryEntryDTO(item)
	}
	z.key("terminal_program_c_head", false)
	z.sizeTerminalProgramCHeadDTO(v.TerminalProgramCHead)
	z.key("claim_ceiling", false)
	z.sizeQualificationClaimCeilingDTO(v.ClaimCeiling)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeQualificationSubjectV2(v QualificationSubjectV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeQualificationSubjectV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeEvidenceHeadV2(v EvidenceHeadV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("selector", false)
	z.str(v.Selector)
	z.key("workflow_key", false)
	z.str(v.WorkflowKey)
	z.key("head_key", false)
	z.str(v.HeadKey)
	z.key("generation", false)
	z.num(uint64(v.Generation))
	z.key("predecessor_head", false)
	z.str(v.PredecessorHead)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("qualification_envelope", false)
	z.sizeArtifactBindingDTO(v.QualificationEnvelope)
	z.key("artifact", false)
	z.sizeArtifactBindingDTO(v.Artifact)
	z.key("denominator", false)
	z.sizeArtifactBindingDTO(v.Denominator)
	z.key("claim_ceiling", false)
	z.sizeQualificationClaimCeilingDTO(v.ClaimCeiling)
}
func CanonicalSizeEvidenceHeadV2(v EvidenceHeadV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeEvidenceHeadV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeReviewActorV2(v ReviewActorV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("identity_kind", true)
	z.str(v.IdentityKind)
	z.key("harness_actor_id", false)
	z.str(v.HarnessActorID)
	z.key("attribution_evidence", false)
	z.sizeArtifactBindingDTO(v.AttributionEvidence)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeReviewActorV2(v ReviewActorV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeReviewActorV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeReviewAssignmentV2(v ReviewAssignmentV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("actor_kind", false)
	z.str(v.ActorKind)
	z.key("harness_actor_id", false)
	z.str(v.HarnessActorID)
	z.key("parent_actor_id", false)
	z.str(v.ParentActorID)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("scope", false)
	if v.Scope == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Scope {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
}
func CanonicalSizeReviewAssignmentV2(v ReviewAssignmentV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeReviewAssignmentV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeReviewTerminalV2(v ReviewTerminalV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("actor_kind", false)
	z.str(v.ActorKind)
	z.key("harness_actor_id", false)
	z.str(v.HarnessActorID)
	z.key("parent_actor_id", false)
	z.str(v.ParentActorID)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("scope", false)
	if v.Scope == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Scope {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("verdict", false)
	z.str(v.Verdict)
}
func CanonicalSizeReviewTerminalV2(v ReviewTerminalV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeReviewTerminalV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeReviewTranscriptSnapshotV2(v ReviewTranscriptSnapshotV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("actor_kind", false)
	z.str(v.ActorKind)
	z.key("harness_actor_id", false)
	z.str(v.HarnessActorID)
	z.key("parent_actor_id", false)
	z.str(v.ParentActorID)
	z.key("raw_transcript_selector", false)
	z.str(v.RawTranscriptSelector)
	z.key("raw_transcript_sha256", false)
	z.str(v.RawTranscriptSHA256)
	z.key("raw_transcript_byte_length", false)
	z.num(uint64(v.RawTranscriptByteLength))
	z.key("assignment_subject", false)
	z.sizeArtifactBindingDTO(v.AssignmentSubject)
	z.key("assignment_scope", false)
	if v.AssignmentScope == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.AssignmentScope {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("terminal_subject", false)
	z.sizeArtifactBindingDTO(v.TerminalSubject)
	z.key("terminal_scope", false)
	if v.TerminalScope == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.TerminalScope {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("terminal_verdict", false)
	z.str(v.TerminalVerdict)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeReviewTranscriptSnapshotV2(v ReviewTranscriptSnapshotV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeReviewTranscriptSnapshotV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeReviewTaskSeparationV2(v ReviewTaskSeparationV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("writer_session_id", false)
	z.str(v.WriterSessionID)
	z.key("reviewer_task_id", false)
	z.str(v.ReviewerTaskID)
	z.key("reviewer_parent_session_id", false)
	z.str(v.ReviewerParentSessionID)
	z.key("reviewer_isolated", false)
	z.boolean(v.ReviewerIsolated)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("review_scope", false)
	if v.ReviewScope == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.ReviewScope {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeReviewTaskSeparationV2(v ReviewTaskSeparationV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeReviewTaskSeparationV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeQualificationReviewV2(v QualificationReviewV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("review_scope", false)
	if v.ReviewScope == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.ReviewScope {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("verdict", false)
	z.str(v.Verdict)
	z.key("writer", false)
	z.sizeReviewActorV2(v.Writer)
	z.key("reviewer", false)
	z.sizeReviewActorV2(v.Reviewer)
	z.key("separation_method", false)
	z.str(v.SeparationMethod)
	z.key("separation_evidence", false)
	z.sizeArtifactBindingDTO(v.SeparationEvidence)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeQualificationReviewV2(v QualificationReviewV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeQualificationReviewV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeQualificationEnvelopeV2(v QualificationEnvelopeV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("independent_review_receipt", false)
	z.sizeArtifactBindingDTO(v.IndependentReviewReceipt)
	z.key("stage6_policy", false)
	z.sizeArtifactBindingDTO(v.Stage6Policy)
	z.key("stage6_limits", false)
	z.sizeArtifactBindingDTO(v.Stage6Limits)
	z.key("stage6_finalization", false)
	z.sizeArtifactBindingDTO(v.Stage6Finalization)
	z.key("qualification_epoch", false)
	z.num(uint64(v.QualificationEpoch))
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeQualificationEnvelopeV2(v QualificationEnvelopeV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeQualificationEnvelopeV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeRunTerminalV2(v RunTerminalV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("run_id", true)
	z.str(v.RunID)
	z.key("seed", false)
	z.num(uint64(v.Seed))
	z.key("replay_ordinal", false)
	z.num(uint64(v.ReplayOrdinal))
	z.key("outcome", false)
	z.str(v.Outcome)
	z.key("community_artifact", false)
	z.sizeArtifactBindingDTO(v.CommunityArtifact)
	z.key("community_count", false)
	z.num(uint64(v.CommunityCount))
}
func CanonicalSizeRunTerminalV2(v RunTerminalV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeRunTerminalV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeConstituentV2(v ConstituentV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("constituent_ordinal", true)
	z.integer(v.ConstituentOrdinal)
	z.key("receipt", false)
	z.sizeArtifactBindingDTO(v.Receipt)
	z.key("publication_selector", false)
	z.str(v.PublicationSelector)
	z.key("constituent_selector", false)
	z.str(v.ConstituentSelector)
	z.key("native_v5_sha256", false)
	z.str(v.NativeV5SHA256)
	z.key("native_v5_byte_length", false)
	z.num(uint64(v.NativeV5ByteLength))
	z.key("instability_sha256", false)
	z.str(v.InstabilitySHA256)
	z.key("instability_byte_length", false)
	z.num(uint64(v.InstabilityByteLength))
	z.key("instability_schema_version", false)
	z.str(v.InstabilitySchemaVersion)
	z.key("program_c_outcome", false)
	z.str(v.ProgramCOutcome)
	z.key("disposition", false)
	z.str(v.Disposition)
	z.key("classification", false)
	z.str(v.Classification)
	z.key("runs", false)
	if v.Runs == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Runs {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeRunTerminalV2(item)
	}
}
func CanonicalSizeConstituentV2(v ConstituentV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeConstituentV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeDenominatorEntryV2(v DenominatorEntryV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("constituent_ordinal", false)
	z.integer(v.ConstituentOrdinal)
	z.key("receipt", false)
	z.sizeArtifactBindingDTO(v.Receipt)
	z.key("custody", false)
	z.sizeCustodyIdentityDTO(v.Custody)
	z.key("run_id", false)
	z.str(v.RunID)
	z.key("seed", false)
	z.num(uint64(v.Seed))
	z.key("replay_ordinal", false)
	z.num(uint64(v.ReplayOrdinal))
	z.key("community_ordinal", false)
	z.integer(v.CommunityOrdinal)
	z.key("community_artifact", false)
	z.sizeArtifactBindingDTO(v.CommunityArtifact)
	z.key("mechanics", false)
	z.sizeArtifactBindingDTO(v.Mechanics)
	z.key("qualification_envelope", false)
	z.sizeArtifactBindingDTO(v.QualificationEnvelope)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeDenominatorEntryV2(v DenominatorEntryV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeDenominatorEntryV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeDenominatorAccountingV2(v DenominatorAccountingV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("p", true)
	z.num(uint64(v.P))
	z.key("c", false)
	z.num(uint64(v.C))
	z.key("e", false)
	z.num(uint64(v.E))
	z.key("r", false)
	z.num(uint64(v.R))
	z.key("i", false)
	z.num(uint64(v.I))
	z.key("evaluated", false)
	z.num(uint64(v.Evaluated))
	z.key("excluded", false)
	z.num(uint64(v.Excluded))
	z.key("unresolved", false)
	z.num(uint64(v.Unresolved))
	z.key("terminal_runs", false)
	z.num(uint64(v.TerminalRuns))
	z.key("eligible_runs", false)
	z.num(uint64(v.EligibleRuns))
	z.key("empty_runs", false)
	z.num(uint64(v.EmptyRuns))
	z.key("resource_excluded_runs", false)
	z.num(uint64(v.ResourceExcludedRuns))
	z.key("d", false)
	z.num(uint64(v.D))
	z.key("g", false)
	z.num(uint64(v.G))
}
func CanonicalSizeDenominatorAccountingV2(v DenominatorAccountingV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeDenominatorAccountingV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeDenominatorV2(v DenominatorV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("subject", false)
	z.sizeArtifactBindingDTO(v.Subject)
	z.key("qualification_envelope", false)
	z.sizeArtifactBindingDTO(v.QualificationEnvelope)
	z.key("corpus_identity", false)
	z.sizeCorpusIdentityDTO(v.CorpusIdentity)
	z.key("workflow_identity", false)
	z.sizeQualificationWorkflowIdentityDTO(v.WorkflowIdentity)
	z.key("terminal_program_c_head", false)
	z.sizeTerminalProgramCHeadDTO(v.TerminalProgramCHead)
	z.key("constituents", false)
	if v.Constituents == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Constituents {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeConstituentV2(item)
	}
	z.key("entries", false)
	if v.Entries == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Entries {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeDenominatorEntryV2(item)
	}
	z.key("accounting", false)
	z.sizeDenominatorAccountingV2(v.Accounting)
	z.key("claim_ceiling", false)
	z.sizeQualificationClaimCeilingDTO(v.ClaimCeiling)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeDenominatorV2(v DenominatorV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeDenominatorV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeLineageRecordV2(v LineageRecordV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("denominator_entry_identity", false)
	z.str(v.DenominatorEntryIdentity)
	z.key("mechanics", false)
	z.sizeArtifactBindingDTO(v.Mechanics)
	z.key("result", false)
	z.sizeArtifactBindingDTO(v.Result)
	z.key("qualification_envelope", false)
	z.sizeArtifactBindingDTO(v.QualificationEnvelope)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeLineageRecordV2(v LineageRecordV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeLineageRecordV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeLineageAccountingV2(v LineageAccountingV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("d", true)
	z.num(uint64(v.D))
	z.key("terminal_outer_results", false)
	z.num(uint64(v.TerminalOuterResults))
	z.key("unresolved_outer_results", false)
	z.num(uint64(v.UnresolvedOuterResults))
	z.key("g", false)
	z.num(uint64(v.G))
	z.key("selected_groups", false)
	z.num(uint64(v.SelectedGroups))
	z.key("no_calls_groups", false)
	z.num(uint64(v.NoCallsGroups))
	z.key("calls_ineligible_groups", false)
	z.num(uint64(v.CallsIneligibleGroups))
	z.key("unresolved_groups", false)
	z.num(uint64(v.UnresolvedGroups))
}
func CanonicalSizeLineageAccountingV2(v LineageAccountingV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeLineageAccountingV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeLineageV2(v LineageV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("profile", false)
	z.str(v.Profile)
	z.key("denominator", false)
	z.sizeArtifactBindingDTO(v.Denominator)
	z.key("qualification_envelope", false)
	z.sizeArtifactBindingDTO(v.QualificationEnvelope)
	z.key("records", false)
	if v.Records == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Records {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeLineageRecordV2(item)
	}
	z.key("accounting", false)
	z.sizeLineageAccountingV2(v.Accounting)
	z.key("claim_ceiling", false)
	z.sizeQualificationClaimCeilingDTO(v.ClaimCeiling)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("identity", false)
	z.str(v.Identity)
}
func CanonicalSizeLineageV2(v LineageV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeLineageV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeSuccessorWorkflowIdentityV2(v SuccessorWorkflowIdentityV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("profile", true)
	z.str(v.Profile)
	z.key("contract", false)
	z.str(v.Contract)
	z.key("census_receipt_sha256", false)
	z.str(v.CensusReceiptSHA256)
	z.key("availability_ledger_sha256", false)
	z.str(v.AvailabilityLedgerSHA256)
	z.key("source_pin_sha256", false)
	z.str(v.SourcePinSHA256)
	z.key("program_c_plan_sha256", false)
	z.str(v.ProgramCPlanSHA256)
	z.key("program_c_bindings_sha256", false)
	z.str(v.ProgramCBindingsSHA256)
	z.key("privacy_policy_sha256", false)
	z.str(v.PrivacyPolicySHA256)
	z.key("retention_policy_sha256", false)
	z.str(v.RetentionPolicySHA256)
	z.key("resource_policy_sha256", false)
	z.str(v.ResourcePolicySHA256)
	z.key("replay_policy_sha256", false)
	z.str(v.ReplayPolicySHA256)
	z.key("qualification_policy_sha256", false)
	z.str(v.QualificationPolicySHA256)
}
func CanonicalSizeSuccessorWorkflowIdentityV2(v SuccessorWorkflowIdentityV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeSuccessorWorkflowIdentityV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeSuccessorHeadV2(v SuccessorHeadV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("selector", false)
	z.str(v.Selector)
	z.key("workflow_key", false)
	z.str(v.WorkflowKey)
	z.key("identity", false)
	z.sizeSuccessorWorkflowIdentityV2(v.Identity)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("generation", false)
	z.num(uint64(v.Generation))
	z.key("predecessor_head", false)
	z.str(v.PredecessorHead)
	z.key("current_stage", false)
	z.str(v.CurrentStage)
	z.key("last_completed_stage", false)
	z.str(v.LastCompletedStage)
	z.key("outcome", false)
	z.str(v.Outcome)
	z.key("missing_evidence", false)
	if v.MissingEvidence == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.MissingEvidence {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.str(item)
	}
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("feature_identity", false)
	z.str(v.FeatureIdentity)
	z.key("completeness", false)
	z.str(v.Completeness)
	z.key("effects", false)
	z.str(v.Effects)
	z.key("implementation_decision", false)
	z.str(v.ImplementationDecision)
	z.key("production_admission", false)
	z.str(v.ProductionAdmission)
	z.key("source_graph_complete", false)
	z.str(v.SourceGraphComplete)
	z.key("absence_inference", false)
	z.str(v.AbsenceInference)
	z.key("prior_successor", false)
	z.sizeArtifactBindingDTO(v.PriorSuccessor)
	z.key("denominator", false)
	z.sizeArtifactBindingDTO(v.Denominator)
	z.key("lineage", false)
	z.sizeArtifactBindingDTO(v.Lineage)
}
func CanonicalSizeSuccessorHeadV2(v SuccessorHeadV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeSuccessorHeadV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeGapRequirementV2(v GapRequirementV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("id", true)
	z.str(v.ID)
	z.key("status", false)
	z.str(v.Status)
	z.key("evidence_selector", false)
	if v.EvidenceSelector == nil {
		z.add(4)
	} else {
		z.str(*v.EvidenceSelector)
	}
	z.key("evidence_byte_length", false)
	if v.EvidenceByteLength == nil {
		z.add(4)
	} else {
		z.num(uint64(*v.EvidenceByteLength))
	}
}
func CanonicalSizeGapRequirementV2(v GapRequirementV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeGapRequirementV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeComputationStateV2(v ComputationStateV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("status", true)
	z.str(v.Status)
	z.key("count", false)
	if v.Count == nil {
		z.add(4)
	} else {
		z.num(uint64(*v.Count))
	}
}
func CanonicalSizeComputationStateV2(v ComputationStateV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeComputationStateV2(v)
	return z.n, z.err
}
func (z *canonicalSizerV2) sizeGapLedgerV2(v GapLedgerV2) {
	if z.err != nil {
		return
	}
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("selector", false)
	z.str(v.Selector)
	z.key("successor_head_selector", false)
	z.str(v.SuccessorHeadSelector)
	z.key("program_c_predecessor_head", false)
	z.str(v.ProgramCPredecessorHead)
	z.key("workflow_identity", false)
	z.sizeSuccessorWorkflowIdentityV2(v.WorkflowIdentity)
	z.key("outcome", false)
	z.str(v.Outcome)
	z.key("current_stage", false)
	z.str(v.CurrentStage)
	z.key("last_completed_stage", false)
	z.str(v.LastCompletedStage)
	z.key("requirements", false)
	if v.Requirements == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, item := range v.Requirements {
		if z.err != nil {
			return
		}
		if i > 0 {
			z.add(1)
		}
		z.sizeGapRequirementV2(item)
	}
	z.key("representative_nomination", false)
	z.sizeComputationStateV2(v.RepresentativeNomination)
	z.key("candidate_preparation", false)
	z.sizeComputationStateV2(v.CandidatePreparation)
	z.key("authority", false)
	z.num(uint64(v.Authority))
	z.key("accepted", false)
	z.boolean(v.Accepted)
	z.key("feature_identity", false)
	z.str(v.FeatureIdentity)
	z.key("completeness", false)
	z.str(v.Completeness)
	z.key("effects", false)
	z.str(v.Effects)
	z.key("implementation_decision", false)
	z.str(v.ImplementationDecision)
	z.key("production_admission", false)
	z.str(v.ProductionAdmission)
	z.key("source_graph_complete", false)
	z.str(v.SourceGraphComplete)
	z.key("absence_inference", false)
	z.str(v.AbsenceInference)
}
func CanonicalSizeGapLedgerV2(v GapLedgerV2) (uint64, error) {
	var z canonicalSizerV2
	z.sizeGapLedgerV2(v)
	return z.n, z.err
}
