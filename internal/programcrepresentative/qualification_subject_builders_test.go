package programcrepresentative_test

import (
	"reflect"
	"testing"

	"lsp-trace/internal/programcrepresentative"
)

func TestProgramCRepresentativeQualificationPureBuilders(t *testing.T) {
	adapterA, err := programcrepresentative.BuildAdapterContract("program-c-algorithm-v2")
	if err != nil {
		t.Fatal(err)
	}
	adapterB, err := programcrepresentative.BuildAdapterContract("program-c-algorithm-v2")
	if err != nil || !reflect.DeepEqual(adapterA, adapterB) {
		t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_ADAPTER_BUILDER_DETERMINISTIC err=%v", err)
	}
	if _, err := programcrepresentative.BuildAdapterContract(""); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_SUBJECT_ADAPTER_BUILDER_REJECTS_EMPTY_ALGORITHM")
	}

	implementationA, err := programcrepresentative.BuildStage6Implementation(135281)
	if err != nil {
		t.Fatal(err)
	}
	implementationB, err := programcrepresentative.BuildStage6Implementation(135281)
	if err != nil || !reflect.DeepEqual(implementationA, implementationB) || implementationA.RepositoryCommit != "b33cf8cdbda56eefd54c049832129eb67a056727" {
		t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_IMPLEMENTATION_BUILDER_EXACT err=%v implementation=%+v", err, implementationA)
	}
	if _, err := programcrepresentative.BuildStage6Implementation(135280); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_SUBJECT_IMPLEMENTATION_SOURCE_BOUND")
	}
	implementationA.Files[0].Path = "mutated"
	implementationC, err := programcrepresentative.BuildStage6Implementation(135281)
	if err != nil || implementationC.Files[0].Path == "mutated" {
		t.Fatalf("ASSERT_PROGRAM_C_SUBJECT_IMPLEMENTATION_DEFENSIVE_COPY err=%v", err)
	}
}
