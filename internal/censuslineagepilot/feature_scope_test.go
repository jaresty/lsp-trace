//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censussourceplan"
)

var fixtureScopePaths = []string{"cmd/lsp-trace/filter_command.go", "cmd/lsp-trace/inspect_command.go", "internal/filter/filter.go", "internal/inspection/projection.go", "cmd/lsp-trace/main.go", "internal/schema/schema.go"}

const fixtureRevision = "8436b4ea25f24f73b1aebb71c63918b61f4ad274"

func scopeFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range fixtureScopePaths {
		f := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("package fixture // "+p+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func TestFeatureScopeRolesBytesAndPlan(t *testing.T) {
	root := scopeFixture(t)
	b, err := freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := b.Files()
	if len(files) != 6 {
		t.Fatalf("P1 fixed roles: %d", len(files))
	}
	for i, f := range files {
		if f.Path != fixtureScopePaths[i] || f.Role != map[bool]string{true: "NOMINATION", false: "CONTEXT_ONLY"}[i < 4] {
			t.Fatalf("P1 role %d: %+v", i, f)
		}
		raw, err := os.ReadFile(filepath.Join(root, f.Path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, f.Bytes) || f.ByteLength != uint64(len(raw)) || f.SHA256 != Digest(raw) {
			t.Fatalf("P2 observed bytes: %+v", f)
		}
	}
	s := b.Selection()
	if len(s.Files) != 4 {
		t.Fatal("P5 nomination only")
	}
	p := b.PartitionPlan()
	want, err := censussourceplan.Build(sourceFiles(s.Files), root, 2)
	if err != nil || !reflect.DeepEqual(p.SourcePlan(), want) {
		t.Fatalf("P5 Build equivalence: %v", err)
	}
	sb, _ := EncodeSourceSelection(s)
	if p.SourceSelectionSHA256 != Digest(sb) || p.SourceSelectionByteLength != uint64(len(sb)) {
		t.Fatal("P5 exact linkage")
	}
	var manifest map[string]any
	if json.Unmarshal(b.ManifestBytes(), &manifest) != nil || manifest["authority"] != float64(0) || manifest["accepted"] != false || manifest["feature_identity"] != "UNRESOLVED" || manifest["source_graph_complete"] != "UNKNOWN" {
		t.Fatal("P6 ceiling")
	}
	before := b.ManifestBytes()
	files[0].Bytes[0] = 'X'
	files[0].Path = "wrong"
	s.Files[0].Path = "wrong"
	p.Plan.Partitions[0].Members[0].Path = "wrong"
	before[0] = 'X'
	if b.Files()[0].Path != fixtureScopePaths[0] || b.Files()[0].Bytes[0] == 'X' || b.Selection().Files[0].Path == "wrong" || b.PartitionPlan().Plan.Partitions[0].Members[0].Path == "wrong" || b.ManifestBytes()[0] == 'X' {
		t.Fatal("P4 alias isolation")
	}
	again, err := freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if err != nil || !bytes.Equal(again.ManifestBytes(), b.ManifestBytes()) {
		t.Fatal("P2 deterministic manifest")
	}
	pins := b.Files()
	pins[0].SHA256 = Digest([]byte("wrong"))
	if bad, e := freezeFeatureScope(context.Background(), root, fixtureRevision, pins); e == nil || bad.state != nil {
		t.Fatal("P3 expected pin comparison")
	}
}
func TestFeatureScopeRefusals(t *testing.T) {
	for _, kind := range []string{"missing", "empty", "symlink-file", "symlink-directory", "oversized", "cancel", "nil-context", "revision", "short-revision", "root"} {
		t.Run(kind, func(t *testing.T) {
			root := scopeFixture(t)
			ctx := context.Background()
			rev := fixtureRevision
			target := filepath.Join(root, fixtureScopePaths[0])
			switch kind {
			case "missing":
				os.Remove(target)
			case "empty":
				os.WriteFile(target, nil, 0644)
			case "nil-context":
				ctx = nil
			case "short-revision":
				rev = "abc"
			case "symlink-file":
				os.Remove(target)
				os.Symlink(filepath.Join(root, fixtureScopePaths[1]), target)
			case "symlink-directory":
				dir := filepath.Join(root, "internal/filter")
				os.Rename(dir, dir+"-old")
				os.Symlink(dir+"-old", dir)
			case "oversized":
				os.WriteFile(target, make([]byte, featureScopeFileLimit+1), 0644)
			case "cancel":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "revision":
				rev = strings.ToUpper(rev)
			case "root":
				root += "/../" + filepath.Base(root)
			}
			b, e := freezeFeatureScope(ctx, root, rev, nil)
			if e == nil || b.state != nil {
				t.Fatalf("P3 reject %s", kind)
			}
		})
	}
}
func TestFeatureScopeManifestGolden(t *testing.T) {
	raw, err := canonicalFeatureScope(scopeManifest{SchemaVersion: featureScopeSchema, Domain: featureScopeDomain, Authority: 0, Accepted: false, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", FeatureIdentity: "UNRESOLVED", RevisionAssertion: fixtureRevision, RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", IncludedRule: "EXACT_FOUR_NOMINATION_TWO_CONTEXT", ExcludedRule: "ALL_OTHER_PATHS_EXCLUDED_EXTERNAL_RELATIONS_ARE_FRONTIER_GAPS", Root: "/fixture", RootIdentity: scopeFileIdentity{Device: 1, Inode: 2}, Files: []scopeManifestFile{}, PartitionSize: 2})
	want := `{"schema_version":"lsp-trace.feature-scope.private.v1","domain":"lsp-trace:feature-scope:private:v1","authority":0,"accepted":false,"completeness":"UNKNOWN","source_graph_complete":"UNKNOWN","feature_identity":"UNRESOLVED","revision_assertion":"8436b4ea25f24f73b1aebb71c63918b61f4ad274","revision_qualification":"CALLER_ASSERTED_NOT_AUTHENTICATED","included_rule":"EXACT_FOUR_NOMINATION_TWO_CONTEXT","excluded_rule":"ALL_OTHER_PATHS_EXCLUDED_EXTERNAL_RELATIONS_ARE_FRONTIER_GAPS","root":"/fixture","root_identity":{"device":1,"inode":2},"files":[],"partition_size":2,"selection_sha256":"","selection_byte_length":0,"plan_sha256":"","plan_byte_length":0}`
	if err != nil || string(raw) != want {
		t.Fatalf("P2 canonical golden: %s %v", raw, err)
	}
}
