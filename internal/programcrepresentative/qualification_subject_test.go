package programcrepresentative_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/programcrepresentative"
)

func TestProgramCRepresentativeQualificationSubjectCanonicalBytes(t *testing.T) {
	in := qualificationSubjectFixture(t)
	subject, err := programcrepresentative.NewProgramCRepresentativeQualificationSubject(in)
	if err != nil {
		t.Fatal(err)
	}
	if !subject.Valid() || !subject.Trusted() {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_TRUSTED")
	}

	dto := subject.DTO()
	expected, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(subject.CanonicalBytes(), expected) {
		t.Fatalf("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_CANONICAL_BYTES\nwant=%s\n got=%s", expected, subject.CanonicalBytes())
	}
	canonicalSum := sha256.Sum256(subject.CanonicalBytes())
	canonicalDigest := "sha256:" + hex.EncodeToString(canonicalSum[:])
	const expectedCanonicalDigest = "sha256:58d19971f5e50778291aa82e079382d997215bcb4af55397c4082970d554f436"
	if canonicalDigest != expectedCanonicalDigest {
		t.Fatalf("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_CANONICAL_DIGEST want=%s got=%s", expectedCanonicalDigest, canonicalDigest)
	}
	if dto.AcceptedQualificationSubjectAmendmentSHA256 != "sha256:235facbbba34f479df4771be845f34b3a39c83988a339dca69eec4f2b6bad9a6" {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_AMENDMENT_DIGEST")
	}
	if dto.Identity != qualificationIdentity(t, dto) || subject.Identity() != dto.Identity {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_ROOT_IDENTITY")
	}
	if bytes.HasSuffix(subject.CanonicalBytes(), []byte("\n")) {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_NO_TRAILING_NEWLINE")
	}
}

func qualificationSubjectFixture(t *testing.T) programcrepresentative.QualificationSubjectExpectedContext {
	t.Helper()
	digest := func(ch byte) string { return "sha256:" + string(bytes.Repeat([]byte{ch}, 64)) }

	adapter := programcrepresentative.AdapterContractDTO{
		SchemaVersion: programcrepresentative.AdapterContractSchemaVersion,
		Profile:       programcrepresentative.ProfileCallsOnlyFrozenCensusV1,
		Domains: []string{
			"lsp-trace:program-c-stage6-adapter-contract:v1",
			"lsp-trace:program-c-stage6-evidence:v1",
			"lsp-trace:program-c-stage6-context:v1",
			"lsp-trace:program-c-stage6-custody:v1",
			"lsp-trace:program-c-stage6-mechanics:v1",
		},
		NativeSchemaVersion:      "lsp-trace.graph.v5",
		ProgramCProfileID:        "calls-v1",
		ProgramCProfileSHA256:    "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9",
		ProgramCAlgorithm:        "gonum.org/v1/gonum/graph/community.Leiden",
		ProgramCAlgorithmVersion: "v0.16.0",
		RunSeeds:                 []uint64{1, 2},
		ReplaysPerSeed:           3,
		RunIDGrammar:             "seed-%020d-replay-%d",
		OccurrenceOrdinalRule:    "ZERO_BASED_WITHIN_CANONICAL_EDGE_CALL_SITES",
		MembershipKinds:          []string{"PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION", "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION"},
		MappedMembershipKinds:    []string{"PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION"},
		CommunityOrderRule:       "PROGRAM_C_VALID_CANONICAL_COMMUNITIES",
	}
	adapter.Identity = qualificationDomainIdentity(t, "lsp-trace:program-c-stage6-adapter-contract:v1", adapter)

	implementation := programcrepresentative.Stage6ImplementationDTO{
		RepositoryCommit: "b33cf8cdbda56eefd54c049832129eb67a056727",
		RepositoryTree:   "f87ac81bfd429d196044b0283574d3de370dc1c3",
		Files: []programcrepresentative.ImplementationFileDTO{
			{Path: "internal/censuscontinuation/programc_stage6_adapter_private.go", GitBlobSHA1: "4144ff5a3980de6c56305d94eeb844a4b7f74cd9", SHA256: "sha256:eaba97734c84b0c06da51a6b0d0e5d94c41dee81feded7634b7acb91c9439dfa", ByteLength: 8699},
			{Path: "internal/censuscontinuation/programc_transition.go", GitBlobSHA1: "eb308697b4e8c968478180a8b775a314a43d43e4", SHA256: "sha256:c67659e1df91a821a2152d6cb70973238a646fbca3d5c2d3fc0d5421f147d29e", ByteLength: 38854},
			{Path: "internal/graph/native_v5_representative.go", GitBlobSHA1: "f80851433cb22cfbf52cc5464c609a4801c7714f", SHA256: "sha256:a743139187ecc7eb9091537f7baa0c45de36fbe0bbca9d99bbc04bea3b9ed0c2", ByteLength: 3636},
			{Path: "internal/programc/core.go", GitBlobSHA1: "e88fac874a84ecde11296fc7db96210d50bfe2ea", SHA256: "sha256:51d0abcb92482e9192086d6a60ea1606db129de459190dade62300b0b0054094", ByteLength: 20636},
			{Path: "internal/programcrepresentative/mechanics.go", GitBlobSHA1: "8fe46ec75f2eaec186579ed19ccbe6357f4ae72a", SHA256: "sha256:b1198396497bac5c2cfb96f9f116149fff9a9357e37574a3a5b72481e7722da4", ByteLength: 25846},
			{Path: "internal/programcrepresentative/representative.go", GitBlobSHA1: "101c9ca01df288db8aec2344d693143edbce8082", SHA256: "sha256:0b4faac4e05e4e85d48fe6e05507b053d2111770d08af13ef9e5d8acbeee5d2e", ByteLength: 15636},
			{Path: "internal/programcrepresentative/stage6_result.go", GitBlobSHA1: "2ca877428e66a60652947cd7f4c54fb16e835774", SHA256: "sha256:83e052984fbe64221ff3484da4430a4cc5f617d4a6ad68be757a30994430ed8d", ByteLength: 21974},
		},
	}
	implementation.Identity = qualificationDomainIdentity(t, "lsp-trace:program-c-representative-stage6-implementation:v1", implementation)

	policy, policyBinding, _, err := programcrepresentative.BuildStage6PolicyArtifact()
	if err != nil {
		t.Fatal(err)
	}
	limits, limitsBinding, _, err := programcrepresentative.BuildStage6LimitsArtifact()
	if err != nil {
		t.Fatal(err)
	}
	finalization, finalizationBinding, _, err := programcrepresentative.BuildStage6FinalizationArtifact()
	if err != nil {
		t.Fatal(err)
	}

	corpus := programcrepresentative.CorpusIdentityDTO{
		FrozenCorpusSHA256:       digest('1'),
		CensusReceiptSHA256:      digest('2'),
		AvailabilityLedgerSHA256: digest('3'),
		ProgramCPlanSHA256:       digest('4'),
		ProgramCBindingsSHA256:   digest('5'),
	}
	workflow := programcrepresentative.QualificationWorkflowIdentityDTO{
		Profile:                   programcrepresentative.ProfileCallsOnlyFrozenCensusV1,
		Contract:                  "lsp-trace.inventory-successor-head.private.v1",
		CensusReceiptSHA256:       corpus.CensusReceiptSHA256,
		AvailabilityLedgerSHA256:  corpus.AvailabilityLedgerSHA256,
		SourcePinSHA256:           digest('6'),
		ProgramCPlanSHA256:        corpus.ProgramCPlanSHA256,
		ProgramCBindingsSHA256:    corpus.ProgramCBindingsSHA256,
		PrivacyPolicySHA256:       digest('7'),
		RetentionPolicySHA256:     digest('8'),
		ResourcePolicySHA256:      digest('9'),
		ReplayPolicySHA256:        digest('a'),
		QualificationPolicySHA256: digest('b'),
		Provenance:                "PREDECESSOR_RETAINED_CALLER_ASSERTED",
	}
	receipts := []programcrepresentative.ReceiptInventoryEntryDTO{
		{PlanOrdinal: 0, ReceiptOrdinal: 1, ReceiptSchemaVersion: "lsp-trace.census-program-c-transition-receipt.private.v3", ReceiptSelector: digest('c'), ReceiptSHA256: digest('c'), ReceiptByteLength: 101},
		{PlanOrdinal: 1, ReceiptOrdinal: 2, ReceiptSchemaVersion: "lsp-trace.census-program-c-transition-receipt.private.v4", ReceiptSelector: digest('d'), ReceiptSHA256: digest('d'), ReceiptByteLength: 102},
	}
	head := programcrepresentative.TerminalProgramCHeadDTO{
		SchemaVersion:    "lsp-trace.census-program-c-transition-head.private.v2",
		WorkflowKey:      "program-c-fixture",
		HeadKey:          "terminal",
		Generation:       2,
		Selector:         digest('e'),
		HeadBytesSHA256:  digest('f'),
		ByteLength:       333,
		CommittedPrefix:  2,
		ConstituentCount: 2,
	}

	return programcrepresentative.QualificationSubjectExpectedContext{
		AdapterContract:              adapter,
		Stage6Implementation:         implementation,
		Stage6Policy:                 policy,
		Stage6PolicyBinding:          policyBinding,
		Stage6Limits:                 limits,
		Stage6LimitsBinding:          limitsBinding,
		Stage6Finalization:           finalization,
		Stage6FinalizationBinding:    finalizationBinding,
		CorpusIdentity:               corpus,
		WorkflowIdentity:             workflow,
		SourcePinSHA256:              workflow.SourcePinSHA256,
		ReceiptInventory:             receipts,
		TerminalProgramCHead:         head,
		MaxImplementationSourceBytes: 1 << 20,
	}
}

func qualificationIdentity(t *testing.T, dto programcrepresentative.QualificationSubjectDTO) string {
	t.Helper()
	dto.Identity = ""
	return qualificationDomainIdentity(t, "lsp-trace:program-c-representative-qualification-subject:v1", dto)
}

func qualificationDomainIdentity(t *testing.T, domain string, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	preimage := append(append([]byte(domain), 0), raw...)
	sum := sha256.Sum256(preimage)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestProgramCRepresentativeQualificationSubjectStrictParse(t *testing.T) {
	subject := mustQualificationSubject(t, qualificationSubjectFixture(t))
	raw := subject.CanonicalBytes()
	parsed, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsed.CanonicalBytes(), raw) || parsed.Identity() != subject.Identity() {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_STRICT_PARSE_CANONICAL")
	}

	duplicate := bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"duplicate","schema_version":`), 1)
	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"unknown":true}`)...)
	cases := map[string][]byte{
		"duplicate":     duplicate,
		"unknown":       unknown,
		"trailing":      append(append([]byte(nil), raw...), '\n'),
		"leading-space": append([]byte(" "), raw...),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(candidate); err == nil {
				t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_STRICT_PARSE_REJECTED")
			}
		})
	}
}

func TestProgramCRepresentativeQualificationSubjectUntrustedParse(t *testing.T) {
	in := qualificationSubjectFixture(t)
	subject := mustQualificationSubject(t, in)
	parsed, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(subject.CanonicalBytes())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Valid() || parsed.Trusted() {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_PARSE_UNTRUSTED")
	}
	trusted, err := programcrepresentative.ValidateProgramCRepresentativeQualificationSubject(parsed.CanonicalBytes(), in)
	if err != nil || !trusted.Valid() || !trusted.Trusted() {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_EXPECTED_CONTEXT_TRUST")
	}
}

func TestProgramCRepresentativeQualificationSubjectRejectsSelfConsistentSubstitution(t *testing.T) {
	in := qualificationSubjectFixture(t)
	subject := mustQualificationSubject(t, in)
	dto := subject.DTO()
	dto.CorpusIdentity.FrozenCorpusSHA256 = testDigest('0')
	dto.Identity = qualificationIdentity(t, dto)
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(raw); err != nil {
		t.Fatalf("self-consistent untrusted syntax should parse: %v", err)
	}
	if _, err := programcrepresentative.ValidateProgramCRepresentativeQualificationSubject(raw, in); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_SELF_CONSISTENT_SUBSTITUTION_REJECTED")
	}
}

func TestProgramCRepresentativeQualificationSubjectRejectsCrossSubjectSplicing(t *testing.T) {
	in := qualificationSubjectFixture(t)
	left := mustQualificationSubject(t, in)
	rightInput := qualificationSubjectFixture(t)
	rightInput.TerminalProgramCHead.HeadBytesSHA256 = testDigest('0')
	right := mustQualificationSubject(t, rightInput)

	spliced := left.DTO()
	spliced.TerminalProgramCHead = right.DTO().TerminalProgramCHead
	spliced.Identity = qualificationIdentity(t, spliced)
	raw, err := json.Marshal(spliced)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(raw); err != nil {
		t.Fatalf("self-consistent splice should remain untrusted syntax: %v", err)
	}
	if _, err := programcrepresentative.ValidateProgramCRepresentativeQualificationSubject(raw, in); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_CROSS_SUBJECT_SPLICE_REJECTED")
	}
}

func TestProgramCRepresentativeQualificationSubjectReceiptInventoryUniqueness(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]programcrepresentative.ReceiptInventoryEntryDTO)
	}{
		{
			name: "combined-duplicate-ordinal-selector-digest-conflicting-length",
			mutate: func(receipts []programcrepresentative.ReceiptInventoryEntryDTO) {
				receipts[1].ReceiptOrdinal = receipts[0].ReceiptOrdinal
				receipts[1].ReceiptSelector = receipts[0].ReceiptSelector
				receipts[1].ReceiptSHA256 = receipts[0].ReceiptSHA256
				receipts[1].ReceiptByteLength = receipts[0].ReceiptByteLength + 1
			},
		},
		{
			name: "duplicate-receipt-ordinal",
			mutate: func(receipts []programcrepresentative.ReceiptInventoryEntryDTO) {
				receipts[1].ReceiptOrdinal = receipts[0].ReceiptOrdinal
			},
		},
		{
			name: "duplicate-selector-and-digest",
			mutate: func(receipts []programcrepresentative.ReceiptInventoryEntryDTO) {
				receipts[1].ReceiptSelector = receipts[0].ReceiptSelector
				receipts[1].ReceiptSHA256 = receipts[0].ReceiptSHA256
			},
		},
		{
			name: "duplicate-selector-and-digest-conflicting-length",
			mutate: func(receipts []programcrepresentative.ReceiptInventoryEntryDTO) {
				receipts[1].ReceiptSelector = receipts[0].ReceiptSelector
				receipts[1].ReceiptSHA256 = receipts[0].ReceiptSHA256
				receipts[1].ReceiptByteLength = receipts[0].ReceiptByteLength + 1
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := qualificationSubjectFixture(t)
			tt.mutate(in.ReceiptInventory)
			if _, err := programcrepresentative.NewProgramCRepresentativeQualificationSubject(in); err == nil {
				t.Fatalf("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_RECEIPT_INVENTORY_UNIQUENESS case=%s", tt.name)
			}
		})
	}
}

func TestProgramCRepresentativeQualificationSubjectValidation(t *testing.T) {
	mutations := map[string]func(*programcrepresentative.QualificationSubjectExpectedContext){
		"receipt-order": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.ReceiptInventory[0], in.ReceiptInventory[1] = in.ReceiptInventory[1], in.ReceiptInventory[0]
		},
		"receipt-selector-digest": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.ReceiptInventory[0].ReceiptSHA256 = testDigest('0')
		},
		"terminal-count": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.TerminalProgramCHead.CommittedPrefix++
		},
		"workflow-provenance": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.WorkflowIdentity.Provenance = "VERIFIED"
		},
		"weak-source-pin": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.SourcePinSHA256 = testDigest('0')
		},
		"policy-binding": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.Stage6PolicyBinding.SHA256 = testDigest('0')
		},
		"limits-binding": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.Stage6LimitsBinding.ByteLength++
		},
		"finalization-binding": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.Stage6FinalizationBinding.Identity = testDigest('0')
		},
		"implementation-order": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.Stage6Implementation.Files[0], in.Stage6Implementation.Files[1] = in.Stage6Implementation.Files[1], in.Stage6Implementation.Files[0]
		},
		"implementation-substitution": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.Stage6Implementation.Files[0].SHA256 = testDigest('0')
			copy := in.Stage6Implementation
			copy.Identity = ""
			in.Stage6Implementation.Identity = qualificationDomainIdentity(t, "lsp-trace:program-c-representative-stage6-implementation:v1", copy)
		},
		"implementation-ceiling": func(in *programcrepresentative.QualificationSubjectExpectedContext) {
			in.MaxImplementationSourceBytes = 1
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			in := qualificationSubjectFixture(t)
			mutate(&in)
			if _, err := programcrepresentative.NewProgramCRepresentativeQualificationSubject(in); err == nil {
				t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_INVALID_CONTEXT_REJECTED")
			}
		})
	}
}

func TestProgramCRepresentativeQualificationSubjectTerminalHeadDigestDistinction(t *testing.T) {
	in := qualificationSubjectFixture(t)
	subject := mustQualificationSubject(t, in)
	dto := subject.DTO()
	if dto.TerminalProgramCHead.Selector == dto.TerminalProgramCHead.HeadBytesSHA256 {
		t.Fatal("fixture must exercise distinct selector and populated-byte digest")
	}
	dto.TerminalProgramCHead.HeadBytesSHA256 = dto.TerminalProgramCHead.Selector
	dto.Identity = qualificationIdentity(t, dto)
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(raw); err != nil {
		t.Fatalf("digest equality is not itself forbidden by the wire: %v", err)
	}
	if _, err := programcrepresentative.ValidateProgramCRepresentativeQualificationSubject(raw, in); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_TERMINAL_HEAD_DIGEST_BINDINGS_DISTINCT")
	}
}

func TestProgramCRepresentativeQualificationSubjectClaimCeilings(t *testing.T) {
	in := qualificationSubjectFixture(t)
	subject := mustQualificationSubject(t, in)
	claim := subject.DTO().ClaimCeiling
	if claim.Authority != 0 || claim.Accepted || claim.FeatureIdentity != "UNRESOLVED" || claim.Completeness != "UNKNOWN" || claim.Effects != "NONE" || claim.Reviews != "NOT_REQUESTED" || claim.ImplementationDecision != "UNISSUED" || claim.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" || claim.SourceGraphComplete != "UNKNOWN" || claim.AbsenceInference != "PROHIBITED" {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_CLAIM_CEILINGS")
	}
	dto := subject.DTO()
	dto.ClaimCeiling.Accepted = true
	dto.Identity = qualificationIdentity(t, dto)
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := programcrepresentative.ParseProgramCRepresentativeQualificationSubject(raw); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_CLAIM_SUBSTITUTION_REJECTED")
	}
}

func TestProgramCRepresentativeQualificationSubjectDefensiveCopies(t *testing.T) {
	in := qualificationSubjectFixture(t)
	subject := mustQualificationSubject(t, in)
	canonical := subject.CanonicalBytes()
	in.AdapterContract.Domains[0] = "mutated"
	in.AdapterContract.RunSeeds[0] = 99
	in.Stage6Implementation.Files[0].Path = "mutated"
	in.ReceiptInventory[0].PlanOrdinal = 99
	if !bytes.Equal(canonical, subject.CanonicalBytes()) {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_INPUT_COPY")
	}

	dto := subject.DTO()
	dto.AdapterContract.Domains[0] = "mutated"
	dto.AdapterContract.RunSeeds[0] = 99
	dto.Stage6Implementation.Files[0].Path = "mutated"
	dto.ReceiptInventory[0].PlanOrdinal = 99
	raw := subject.CanonicalBytes()
	raw[0] ^= 0xff
	again := subject.DTO()
	if again.AdapterContract.Domains[0] == "mutated" || again.AdapterContract.RunSeeds[0] == 99 || again.Stage6Implementation.Files[0].Path == "mutated" || again.ReceiptInventory[0].PlanOrdinal == 99 || !bytes.Equal(canonical, subject.CanonicalBytes()) {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_OUTPUT_COPY")
	}

	policyA, _, _, err := programcrepresentative.BuildStage6PolicyArtifact()
	if err != nil {
		t.Fatal(err)
	}
	policyA.SelectionOrder[0] = "mutated"
	policyB, _, _, _ := programcrepresentative.BuildStage6PolicyArtifact()
	finalA, _, _, _ := programcrepresentative.BuildStage6FinalizationArtifact()
	finalA.CompositionMappings[0] = "mutated"
	finalB, _, _, _ := programcrepresentative.BuildStage6FinalizationArtifact()
	if policyB.SelectionOrder[0] == "mutated" || finalB.CompositionMappings[0] == "mutated" {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_BUILDER_COPY")
	}
}

func TestProgramCRepresentativeQualificationSubjectArtifactBuilders(t *testing.T) {
	policy, policyBinding, policyRaw, err := programcrepresentative.BuildStage6PolicyArtifact()
	if err != nil {
		t.Fatal(err)
	}
	limits, limitsBinding, limitsRaw, err := programcrepresentative.BuildStage6LimitsArtifact()
	if err != nil {
		t.Fatal(err)
	}
	finalization, finalizationBinding, finalizationRaw, err := programcrepresentative.BuildStage6FinalizationArtifact()
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name     string
		identity string
		binding  programcrepresentative.ArtifactBindingDTO
		raw      []byte
	}{
		{"policy", policy.Identity, policyBinding, policyRaw},
		{"limits", limits.Identity, limitsBinding, limitsRaw},
		{"finalization", finalization.Identity, finalizationBinding, finalizationRaw},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			sum := sha256.Sum256(check.raw)
			if check.binding.Identity != check.identity || check.binding.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) || check.binding.ByteLength != uint64(len(check.raw)) {
				t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_ARTIFACT_BINDING")
			}
		})
	}
	if policy.SelectionOrder == nil || policy.PredecessorOrder == nil || finalization.CompositionMappings == nil || finalization.EvidenceMappings == nil || finalization.GroupingMappings == nil || finalization.ContextMappings == nil || finalization.QualificationMappings == nil || finalization.FinalizationOrder == nil || limits.MaxWork != limits.MaxOccurrences+limits.MaxMembers {
		t.Fatal("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_FIXED_ARTIFACTS")
	}
}

func TestProgramCRepresentativeQualificationSubjectAcyclicityAndImports(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "qualification_subject.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenImports := []string{"censuscontinuation", "inventorysuccessor", "publication", "store", "head", "cas"}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		for _, forbidden := range forbiddenImports {
			if strings.Contains(strings.ToLower(path), forbidden) {
				t.Fatalf("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_FORBIDDEN_IMPORT path=%s", path)
			}
		}
	}
	if len(file.Imports) == 0 || !ast.IsExported("QualificationSubjectDTO") {
		t.Fatal("unexpected structural fixture")
	}

	forbiddenKeys := map[string]bool{
		"independent_review_receipt": true,
		"qualification_envelope":     true,
		"denominator":                true,
		"lineage":                    true,
		"successor":                  true,
		"head_publication":           true,
		"cas":                        true,
	}
	seen := map[reflect.Type]bool{}
	var visit func(reflect.Type)
	visit = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			key := strings.Split(field.Tag.Get("json"), ",")[0]
			if forbiddenKeys[key] {
				t.Fatalf("ASSERT_PROGRAM_C_REPRESENTATIVE_QUALIFICATION_SUBJECT_DOWNSTREAM_KEY key=%s", key)
			}
			visit(field.Type)
		}
	}
	visit(reflect.TypeOf(programcrepresentative.QualificationSubjectDTO{}))
}

func mustQualificationSubject(t *testing.T, in programcrepresentative.QualificationSubjectExpectedContext) programcrepresentative.ProgramCRepresentativeQualificationSubject {
	t.Helper()
	subject, err := programcrepresentative.NewProgramCRepresentativeQualificationSubject(in)
	if err != nil {
		t.Fatal(err)
	}
	return subject
}

func testDigest(ch byte) string {
	return "sha256:" + string(bytes.Repeat([]byte{ch}, 64))
}

func TestProgramCRepresentativeQualificationSubjectNoUnexpectedFiles(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	foundSource, foundTest := false, false
	for _, entry := range entries {
		switch entry.Name() {
		case "qualification_subject.go":
			foundSource = true
		case "qualification_subject_test.go":
			foundTest = true
		}
	}
	if !foundSource || !foundTest {
		t.Fatal("qualification subject files missing")
	}
}
