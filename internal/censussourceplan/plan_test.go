package censussourceplan

import (
	"errors"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/censusacquisition"
)

func source(workspace, path string) censusacquisition.SourceFile {
	return censusacquisition.SourceFile{Path: path, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, filepath.FromSlash(path))}).String(), LanguageID: "go"}
}

func TestBuildExactPartitionAndCanonicalIdentity(t *testing.T) {
	workspace := t.TempDir()
	files := []censusacquisition.SourceFile{source(workspace, "c.go"), source(workspace, "a.go"), source(workspace, "b.go")}
	plan, err := Build(files, workspace, 2)
	if err != nil || plan.Identity == "" || plan.Authority != 0 || len(plan.Files) != 3 || len(plan.Partitions) != 2 || len(plan.Partitions[0].Members) != 2 || len(plan.Partitions[1].Members) != 1 {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_EXACT_PARTITION: plan=%+v err=%v", plan, err)
	}
	got := append(append([]censusacquisition.SourceFile(nil), plan.Partitions[0].Members...), plan.Partitions[1].Members...)
	if !reflect.DeepEqual(got, plan.Files) {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_EXACT_PARTITION: files=%+v partitions=%+v", plan.Files, plan.Partitions)
	}
	permuted, err := Build([]censusacquisition.SourceFile{files[1], files[2], files[0]}, workspace, 2)
	if err != nil || permuted.Identity != plan.Identity || !reflect.DeepEqual(permuted.Files, plan.Files) {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_PERMUTATION_IDENTITY: first=%+v permuted=%+v err=%v", plan, permuted, err)
	}
}

func TestBuildReturnsIndependentExactClones(t *testing.T) {
	workspace := t.TempDir()
	caller := []censusacquisition.SourceFile{source(workspace, "a.go"), source(workspace, "b.go")}
	plan, err := Build(caller, workspace, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []censusacquisition.SourceFile{source(workspace, "a.go"), source(workspace, "b.go")}
	caller[0].Path = "caller.go"
	if !reflect.DeepEqual(plan.Files, want) || !reflect.DeepEqual(plan.Partitions[0].Members, want[:1]) || !reflect.DeepEqual(plan.Partitions[1].Members, want[1:]) {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_CALLER_CLONE: caller=%+v plan=%+v", caller, plan)
	}
	plan.Files[0].Path = "files.go"
	if plan.Partitions[0].Members[0] != want[0] {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_FILES_PARTITION_CLONE: plan=%+v", plan)
	}
	plan.Partitions[0].Members[0].Path = "partition.go"
	if plan.Partitions[1].Members[0] != want[1] || caller[1] != want[1] {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_PARTITION_INDEPENDENCE: caller=%+v plan=%+v", caller, plan)
	}
}

func TestValidateAcceptsAuthenticCanonicalPlan(t *testing.T) {
	workspace := t.TempDir()
	plan, err := Build([]censusacquisition.SourceFile{
		source(workspace, "c.go"),
		source(workspace, "a.go"),
		source(workspace, "b.go"),
	}, workspace, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(plan, 2); err != nil {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_VALIDATE_AUTHENTIC_CANONICAL: err=%v plan=%+v", err, plan)
	}
}

func TestValidateRejectsMalformedExactCoverageWithoutRepair(t *testing.T) {
	clone := func(plan Plan) Plan {
		cloned := plan
		cloned.Files = append([]censusacquisition.SourceFile(nil), plan.Files...)
		cloned.Partitions = make([]Partition, len(plan.Partitions))
		for i, partition := range plan.Partitions {
			cloned.Partitions[i] = partition
			cloned.Partitions[i].Members = append([]censusacquisition.SourceFile(nil), partition.Members...)
		}
		return cloned
	}
	mutations := map[string]func(*Plan){
		"wrong later ordinal":       func(plan *Plan) { plan.Partitions[1].Ordinal = 2 },
		"empty later members":       func(plan *Plan) { plan.Partitions[1].Members = nil },
		"inconsistent later member": func(plan *Plan) { plan.Partitions[1].Members[0].Path = "different.go" },
		"later partition overlap": func(plan *Plan) {
			plan.Partitions[1].Members[0] = plan.Partitions[0].Members[len(plan.Partitions[0].Members)-1]
		},
		"later partition gap": func(plan *Plan) { plan.Partitions[1].Members = plan.Partitions[1].Members[1:] },
		"omitted exact coverage": func(plan *Plan) {
			plan.Partitions[0].Members = plan.Partitions[0].Members[:len(plan.Partitions[0].Members)-1]
		},
		"reordered exact coverage": func(plan *Plan) {
			plan.Partitions[0].Members[0], plan.Partitions[0].Members[1] = plan.Partitions[0].Members[1], plan.Partitions[0].Members[0]
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			plan, err := Build([]censusacquisition.SourceFile{
				source(workspace, "a.go"),
				source(workspace, "b.go"),
				source(workspace, "c.go"),
				source(workspace, "d.go"),
			}, workspace, 2)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&plan)
			before := clone(plan)
			if err := Validate(plan, 2); !errors.Is(err, ErrInvalidIdentity) {
				t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_VALIDATE_REJECTS_MALFORMED: err=%v plan=%+v", err, plan)
			}
			if !reflect.DeepEqual(plan, before) {
				t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_VALIDATE_DOES_NOT_REPAIR: before=%+v after=%+v", before, plan)
			}
		})
	}
}

func TestBuildRefusesInvalidInputs(t *testing.T) {
	workspace := t.TempDir()
	uri := func(path string) string {
		return (&url.URL{Scheme: "file", Path: filepath.Join(workspace, filepath.FromSlash(path))}).String()
	}
	for _, tc := range []struct {
		name      string
		assertion string
		files     []censusacquisition.SourceFile
		size      int
		err       error
	}{
		{name: "empty enumeration", assertion: "ASSERT_CENSUS_SOURCE_PLAN_EMPTY_ENUMERATION_REFUSED", size: 1, err: ErrInvalidIdentity},
		{name: "duplicate path", assertion: "ASSERT_CENSUS_SOURCE_PLAN_DUPLICATE_IDENTITY", files: []censusacquisition.SourceFile{source(workspace, "a.go"), source(workspace, "a.go")}, size: 1, err: ErrDuplicateIdentity},
		{name: "missing path", assertion: "ASSERT_CENSUS_SOURCE_PLAN_MISSING_PATH", files: []censusacquisition.SourceFile{{URI: uri("a.go"), LanguageID: "go"}}, size: 1, err: ErrInvalidIdentity},
		{name: "missing uri", assertion: "ASSERT_CENSUS_SOURCE_PLAN_MISSING_URI", files: []censusacquisition.SourceFile{{Path: "a.go", LanguageID: "go"}}, size: 1, err: ErrInvalidIdentity},
		{name: "malformed uri", assertion: "ASSERT_CENSUS_SOURCE_PLAN_MALFORMED_URI", files: []censusacquisition.SourceFile{{Path: "a.go", URI: "://bad", LanguageID: "go"}}, size: 1, err: ErrInvalidIdentity},
		{name: "reparable noncanonical path", assertion: "ASSERT_CENSUS_SOURCE_PLAN_DOES_NOT_REPAIR_PATH", files: []censusacquisition.SourceFile{{Path: "dir/../a.go", URI: uri("a.go"), LanguageID: "go"}}, size: 1, err: ErrInvalidIdentity},
		{name: "path uri mismatch", assertion: "ASSERT_CENSUS_SOURCE_PLAN_PATH_URI_AGREEMENT", files: []censusacquisition.SourceFile{{Path: "a.go", URI: uri("b.go"), LanguageID: "go"}}, size: 1, err: ErrInvalidIdentity},
		{name: "invalid disposition", assertion: "ASSERT_CENSUS_SOURCE_PLAN_PRE_DISPOSITION", files: []censusacquisition.SourceFile{{Path: "a.go", URI: uri("a.go"), LanguageID: "go", Disposition: "selected"}}, size: 1, err: ErrInvalidIdentity},
		{name: "parent path", assertion: "ASSERT_CENSUS_SOURCE_PLAN_PARENT_PATH", files: []censusacquisition.SourceFile{{Path: "../a.go", URI: uri("a.go")}}, size: 1, err: ErrInvalidIdentity},
		{name: "zero size", assertion: "ASSERT_CENSUS_SOURCE_PLAN_ZERO_SIZE", files: []censusacquisition.SourceFile{source(workspace, "a.go")}, size: 0, err: ErrInvalidPartitionSize},
		{name: "oversize", assertion: "ASSERT_CENSUS_SOURCE_PLAN_OVERSIZE", files: []censusacquisition.SourceFile{source(workspace, "a.go")}, size: MaxPartitionSize + 1, err: ErrInvalidPartitionSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.files, workspace, tc.size)
			if !errors.Is(err, tc.err) {
				t.Fatalf("%s: got=%v want=%v", tc.assertion, err, tc.err)
			}
		})
	}
}
