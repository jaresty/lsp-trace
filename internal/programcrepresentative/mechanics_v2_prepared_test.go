package programcrepresentative

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

func TestPreparedMechanicsV2PublicShapeIsNarrow(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "mechanics_v2.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	types := map[string]*ast.StructType{}
	functions := map[string]bool{}
	preparedMethods := []string{}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range declaration.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if ok {
					types[typeSpec.Name.Name] = structure
				}
			}
		case *ast.FuncDecl:
			functions[declaration.Name.Name] = true
			if declaration.Recv != nil && len(declaration.Recv.List) == 1 {
				if receiver, ok := declaration.Recv.List[0].Type.(*ast.Ident); ok && receiver.Name == "PreparedMechanicsV2" {
					preparedMethods = append(preparedMethods, declaration.Name.Name)
				}
			}
		}
	}

	assertPreparedMechanicsV2Fields(t, types["MechanicsCommonInputV2"], []string{
		"WorkflowKey", "TerminalProgramCHeadSelector", "ReceiptSelector", "ReceiptOrdinal",
		"ConstituentOrdinal", "PublicationSelector", "ConstituentSelector", "Source",
		"ProgramCAlgorithmVersion",
	})
	assertPreparedMechanicsV2Fields(t, types["MechanicsContextInputV2"], []string{
		"RunID", "Seed", "ReplayOrdinal", "CommunityOrdinal", "CommunityArtifactSHA256", "CommunityMembers",
	})

	prepared := types["PreparedMechanicsV2"]
	if prepared == nil || prepared.Fields == nil || len(prepared.Fields.List) == 0 {
		t.Fatal("ASSERT_PREPARED_MECHANICS_V2_DECLARATIONS missing opaque prepared state")
	}
	hasValidityMarker := false
	for _, field := range prepared.Fields.List {
		for _, name := range field.Names {
			if ast.IsExported(name.Name) {
				t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_DECLARATIONS exported prepared field %q", name.Name)
			}
			if name.Name == "validity" {
				hasValidityMarker = true
			}
		}
	}
	if !hasValidityMarker || !functions["PrepareMechanicsV2"] {
		t.Fatal("ASSERT_PREPARED_MECHANICS_V2_DECLARATIONS missing validity marker or prepare function")
	}

	if !reflect.DeepEqual(preparedMethods, []string{"SourceCounts"}) {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_DECLARATIONS methods=%v", preparedMethods)
	}
}

func TestPreparedMechanicsV2RejectsFabricatedAndCrossBoundState(t *testing.T) {
	validity := &preparedMechanicsValidityV2{marker: 1}
	state := &preparedMechanicsStateV2{
		validity: validity,
		contract: AdapterContractDTOv2{
			Identity: "sha256:" + strings.Repeat("1", 64), ProgramCAlgorithmVersion: "algorithm-a",
		},
		custody: CustodyIdentityDTO{
			Identity: "sha256:" + strings.Repeat("2", 64), NativeV5SHA256: "sha256:" + strings.Repeat("3", 64),
		},
		receiptSelector: "sha256:" + strings.Repeat("4", 64), constituentSelector: "constituent-a",
		bundleID: "bundle-a", evidence: []EvidenceIdentityDTO{}, seedGroups: []SeedGroupIdentityDTOv2{},
	}
	fabricated := PreparedMechanicsV2{state: state, validity: validity}
	context := MechanicsContextInputV2{
		RunID: "seed-00000000000000000001-replay-1", Seed: 1, ReplayOrdinal: 1,
		CommunityOrdinal: 0, CommunityArtifactSHA256: "sha256:" + strings.Repeat("5", 64),
		CommunityMembers: []string{"member"},
	}
	if _, err := NewPreparedMechanicsV2(fabricated, context); err == nil || err.Error() != "invalid prepared v2 mechanics" {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_FABRICATED err=%v", err)
	}

	crossState := *state
	crossState.receiptSelector = "sha256:" + strings.Repeat("6", 64)
	crossState.custody.NativeV5SHA256 = "sha256:" + strings.Repeat("7", 64)
	crossState.contract.ProgramCAlgorithmVersion = "algorithm-b"
	crossBound := PreparedMechanicsV2{state: &crossState, validity: validity}
	if _, err := NewPreparedMechanicsV2(crossBound, context); err == nil || err.Error() != "invalid prepared v2 mechanics" {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_CROSS_RECEIPT_SOURCE_ALGORITHM err=%v", err)
	}
}

func assertPreparedMechanicsV2Fields(t *testing.T, structure *ast.StructType, want []string) {
	t.Helper()
	if structure == nil || structure.Fields == nil {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_DECLARATIONS missing fields want=%v", want)
	}
	got := make([]string, 0, len(structure.Fields.List))
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			got = append(got, name.Name)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_DECLARATIONS fields=%v want=%v", got, want)
	}
}
