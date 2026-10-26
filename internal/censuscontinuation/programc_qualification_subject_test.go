package censuscontinuation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/programcrepresentative"
)

func TestProgramCQualificationEvidenceAuthorityBoundarySurface(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "programc_qualification_subject.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var evidenceBuilder *ast.FuncDecl
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok && typeSpec.Name.Name == "ProgramCRepresentativeWorkflowAuthority" {
					t.Fatal("ASSERT_CENSUS_QUALIFICATION_API_NO_WORKFLOW_AUTHORITY_INPUT")
				}
			}
		case *ast.FuncDecl:
			switch value.Name.Name {
			case "BuildProgramCRepresentativeQualificationSubject":
				t.Fatal("ASSERT_CENSUS_QUALIFICATION_API_NO_TRUSTED_SUBJECT_RETURN")
			case "BuildProgramCRepresentativeQualificationEvidence":
				evidenceBuilder = value
			}
		}
	}
	if evidenceBuilder == nil || evidenceBuilder.Type.Params.NumFields() != 2 || evidenceBuilder.Type.Results.NumFields() != 2 {
		t.Fatal("ASSERT_CENSUS_QUALIFICATION_EVIDENCE_API_SHAPE")
	}
	requestType, ok := evidenceBuilder.Type.Params.List[1].Type.(*ast.Ident)
	if !ok || requestType.Name != "ProgramCTransitionVerificationRequest" {
		t.Fatal("ASSERT_CENSUS_QUALIFICATION_API_NO_WORKFLOW_AUTHORITY_INPUT")
	}
	resultType, ok := evidenceBuilder.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || resultType.Name != "ProgramCRepresentativeQualificationEvidence" {
		t.Fatal("ASSERT_CENSUS_QUALIFICATION_API_NO_TRUSTED_SUBJECT_RETURN")
	}
	forbiddenCall := false
	ast.Inspect(evidenceBuilder.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "NewProgramCRepresentativeQualificationSubject" {
			forbiddenCall = true
		}
		return true
	})
	if forbiddenCall {
		t.Fatal("ASSERT_CENSUS_QUALIFICATION_API_NO_TRUSTED_SUBJECT_CONSTRUCTION")
	}
}

func TestProgramCQualificationEvidenceUsesQualificationVerifierDuringAuthoritativeReconstruction(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "programc_qualification_subject.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var evidenceBuilder *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "BuildProgramCRepresentativeQualificationEvidence" {
			evidenceBuilder = function
			break
		}
	}
	if evidenceBuilder == nil {
		t.Fatal("ASSERT_PROGRAM_C_QUALIFICATION_VERIFIER_IN_AUTHORITATIVE_LOOP missing evidence builder")
	}
	verifierCall := false
	postVerificationPreflight := false
	ast.Inspect(evidenceBuilder.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		switch identifier.Name {
		case "verifyProgramCQualificationAuthority":
			verifierCall = true
		case "preflightProgramCRepresentativeCorpus":
			postVerificationPreflight = true
		}
		return true
	})
	if !verifierCall {
		t.Fatal("ASSERT_PROGRAM_C_QUALIFICATION_VERIFIER_IN_AUTHORITATIVE_LOOP")
	}
	if postVerificationPreflight {
		t.Fatal("ASSERT_PROGRAM_C_QUALIFICATION_NO_POST_VERIFICATION_PREFLIGHT")
	}
}

func TestProgramCQualificationEvidenceDefensiveCopies(t *testing.T) {
	evidence := ProgramCRepresentativeQualificationEvidence{
		adapterContract: programcrepresentative.AdapterContractDTO{
			Domains: []string{"domain"}, RunSeeds: []uint64{1}, MembershipKinds: []string{"member"}, MappedMembershipKinds: []string{"mapped"},
		},
		stage6Implementation: programcrepresentative.Stage6ImplementationDTO{Files: []programcrepresentative.ImplementationFileDTO{{Path: "file"}}},
		stage6Policy: programcrepresentative.Stage6PolicyDTO{
			SelectionOrder: []string{"selection"}, PredecessorOrder: []string{"predecessor"},
		},
		stage6Finalization: programcrepresentative.Stage6FinalizationDTO{
			CompositionMappings: []string{"composition"}, EvidenceMappings: []string{"evidence"},
			GroupingMappings: []string{"grouping"}, ContextMappings: []string{"context"},
			QualificationMappings: []string{"qualification"}, FinalizationOrder: []string{"finalization"},
		},
		receiptInventory: []programcrepresentative.ReceiptInventoryEntryDTO{{ReceiptSelector: "receipt"}},
	}
	first := evidence.QualificationSubjectExpectedContext()
	first.AdapterContract.Domains[0] = "mutated"
	first.AdapterContract.RunSeeds[0] = 2
	first.AdapterContract.MembershipKinds[0] = "mutated"
	first.AdapterContract.MappedMembershipKinds[0] = "mutated"
	first.Stage6Implementation.Files[0].Path = "mutated"
	first.Stage6Policy.SelectionOrder[0] = "mutated"
	first.Stage6Policy.PredecessorOrder[0] = "mutated"
	first.Stage6Finalization.CompositionMappings[0] = "mutated"
	first.Stage6Finalization.EvidenceMappings[0] = "mutated"
	first.Stage6Finalization.GroupingMappings[0] = "mutated"
	first.Stage6Finalization.ContextMappings[0] = "mutated"
	first.Stage6Finalization.QualificationMappings[0] = "mutated"
	first.Stage6Finalization.FinalizationOrder[0] = "mutated"
	first.ReceiptInventory[0].ReceiptSelector = "mutated"

	second := evidence.QualificationSubjectExpectedContext()
	if second.AdapterContract.Domains[0] != "domain" || second.AdapterContract.RunSeeds[0] != 1 || second.AdapterContract.MembershipKinds[0] != "member" || second.AdapterContract.MappedMembershipKinds[0] != "mapped" ||
		second.Stage6Implementation.Files[0].Path != "file" || second.Stage6Policy.SelectionOrder[0] != "selection" || second.Stage6Policy.PredecessorOrder[0] != "predecessor" ||
		second.Stage6Finalization.CompositionMappings[0] != "composition" || second.Stage6Finalization.EvidenceMappings[0] != "evidence" || second.Stage6Finalization.GroupingMappings[0] != "grouping" ||
		second.Stage6Finalization.ContextMappings[0] != "context" || second.Stage6Finalization.QualificationMappings[0] != "qualification" || second.Stage6Finalization.FinalizationOrder[0] != "finalization" ||
		second.ReceiptInventory[0].ReceiptSelector != "receipt" || second.WorkflowIdentity != (programcrepresentative.QualificationWorkflowIdentityDTO{}) || second.SourcePinSHA256 != "" {
		t.Fatal("ASSERT_PROGRAM_C_QUALIFICATION_EVIDENCE_DEFENSIVE_COPIES")
	}
}

func TestProgramCRepresentativeCorpusPreflightAllBoundaries(t *testing.T) {
	entry := programCStage6CompleteEntryFixture(t)
	adapted, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil {
		t.Fatal(err)
	}
	adaptations := []programCStage6Adaptation{
		{classification: programCStage6Empty},
		adapted,
		adapted,
		{classification: programCStage6ResourceIncomplete},
	}
	observed := observedProgramCRepresentativeLimits(adapted.mechanics)
	observed.MaxCanonicalBytes = 1
	if err := measureProgramCRepresentativeCorpus(adaptations, observed); err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_PREFLIGHT_AT_BOUNDARY err=%v observed=%+v", err, observed)
	}

	checks := []struct {
		name   string
		mutate func(*programcrepresentative.Stage6LimitsDTO)
	}{
		{"communities", func(v *programcrepresentative.Stage6LimitsDTO) { v.MaxCommunities-- }},
		{"members", func(v *programcrepresentative.Stage6LimitsDTO) { v.MaxMembers-- }},
		{"candidates", func(v *programcrepresentative.Stage6LimitsDTO) { v.MaxCandidates-- }},
		{"occurrences", func(v *programcrepresentative.Stage6LimitsDTO) { v.MaxOccurrences-- }},
		{"string-bytes", func(v *programcrepresentative.Stage6LimitsDTO) { v.MaxStringBytes-- }},
		{"work", func(v *programcrepresentative.Stage6LimitsDTO) { v.MaxWork-- }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			limit := observed
			check.mutate(&limit)
			if err := measureProgramCRepresentativeCorpus(adaptations, limit); err == nil {
				t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_PREFLIGHT_PLUS_ONE_%s", check.name)
			}
		})
	}
}

func TestProgramCRepresentativeCorpusPreflightClassifications(t *testing.T) {
	limits := programcrepresentative.Stage6LimitsDTO{
		MaxCommunities: math.MaxInt, MaxMembers: math.MaxInt, MaxOccurrences: math.MaxInt,
		MaxCandidates: math.MaxInt, MaxStringBytes: math.MaxInt, MaxWork: math.MaxInt,
	}
	if err := measureProgramCRepresentativeCorpus([]programCStage6Adaptation{{classification: programCStage6Empty}, {classification: programCStage6ResourceIncomplete}}, limits); err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_PREFLIGHT_ZERO_ELIGIBLE_CLASSIFICATIONS err=%v", err)
	}
	for name, adaptation := range map[string]programCStage6Adaptation{
		"unknown":                {},
		"complete-without-entry": {classification: programCStage6Complete},
		"empty-with-entry":       {classification: programCStage6Empty, mechanics: []programcrepresentative.Mechanics{{}}},
		"resource-with-entry":    {classification: programCStage6ResourceIncomplete, mechanics: []programcrepresentative.Mechanics{{}}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := measureProgramCRepresentativeCorpus([]programCStage6Adaptation{adaptation}, limits); err == nil {
				t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_PREFLIGHT_CLASSIFICATION_%s", name)
			}
		})
	}
}

func observedProgramCRepresentativeLimits(mechanics []programcrepresentative.Mechanics) programcrepresentative.Stage6LimitsDTO {
	type run struct{ communities, members, occurrences int }
	runs := map[string]*run{}
	out := programcrepresentative.Stage6LimitsDTO{}
	for _, value := range mechanics {
		dto := value.DTO()
		measurement := runs[dto.RunID]
		if measurement == nil {
			measurement = &run{occurrences: len(dto.Evidence)}
			runs[dto.RunID] = measurement
		}
		measurement.communities++
		measurement.members += len(dto.CommunityMembers)
		if len(dto.CommunityMembers) > out.MaxCandidates {
			out.MaxCandidates = len(dto.CommunityMembers)
		}
		if got := int(maximumStringBytes(reflect.ValueOf(dto))); got > out.MaxStringBytes {
			out.MaxStringBytes = got
		}
	}
	for _, measurement := range runs {
		if measurement.communities > out.MaxCommunities {
			out.MaxCommunities = measurement.communities
		}
		if measurement.members > out.MaxMembers {
			out.MaxMembers = measurement.members
		}
		if measurement.occurrences > out.MaxOccurrences {
			out.MaxOccurrences = measurement.occurrences
		}
		if measurement.occurrences+measurement.members > out.MaxWork {
			out.MaxWork = measurement.occurrences + measurement.members
		}
	}
	return out
}
