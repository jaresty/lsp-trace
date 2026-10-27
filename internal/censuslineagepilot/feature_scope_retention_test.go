//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func retentionFixture(t *testing.T, equal bool) featureScope {
	t.Helper()
	root := scopeFixture(t)
	if equal {
		for _, path := range featureScopePaths() {
			if err := os.WriteFile(filepath.Join(root, path), []byte("package fixture\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	scope, err := freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}
func TestScopeRetentionPlanOrderingDedupAndIsolation(t *testing.T) {
	for _, equal := range []bool{false, true} {
		t.Run(map[bool]string{false: "distinct", true: "equal"}[equal], func(t *testing.T) {
			scope := retentionFixture(t, equal)
			plan, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
			if err != nil {
				t.Fatal(err)
			}
			again, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
			if err != nil || !bytes.Equal(again.metadataBytes(), plan.metadataBytes()) || again.state.budget != plan.state.budget {
				t.Fatal("nondeterministic plan", err)
			}
			refs := plan.references()
			if len(refs) != 9 {
				t.Fatal(len(refs))
			}
			wantObjects := 9
			if equal {
				wantObjects = 4
			}
			if len(plan.state.objects) != wantObjects {
				t.Fatal(len(plan.state.objects))
			}
			for i, ref := range refs {
				var want []byte
				switch {
				case i < 6:
					want = scope.Files()[i].Bytes
					if ref.Path != featureScopePaths()[i] || ref.Role != scope.Files()[i].Role {
						t.Fatal(ref)
					}
				case i == 6:
					want = scope.SelectionBytes()
					if ref.Role != "SOURCE_SELECTION" {
						t.Fatal(ref)
					}
				case i == 7:
					want = scope.PlanBytes()
					if ref.Role != "PARTITION_PLAN" {
						t.Fatal(ref)
					}
				case i == 8:
					want = scope.ManifestBytes()
					if ref.Role != "SCOPE_MANIFEST" {
						t.Fatal(ref)
					}
				}
				if !bytes.Equal(plan.objectBytes(ref.Object), want) || ref.SHA256 != Digest(want) || ref.ByteLength != uint64(len(want)) {
					t.Fatal(ref)
				}
				if equal && i < 6 && ref.Object != 0 {
					t.Fatal("storage-only dedup")
				}
			}
			if refs[8].SHA256 == scope.ManifestDigest() {
				t.Fatal("raw storage digest confused with domain digest")
			}
			metadata := plan.metadataBytes()
			var decoded struct {
				Authority  int                       `json:"authority"`
				References []scopeRetentionReference `json:"active_references"`
			}
			if json.Unmarshal(metadata, &decoded) != nil || decoded.Authority != 0 || !reflect.DeepEqual(decoded.References, refs) {
				t.Fatal("metadata linkage")
			}
			refs[0].Role = "changed"
			metadata[0] = 'X'
			raw := plan.objectBytes(0)
			raw[0] = 'X'
			scope.state.files[0].Bytes[0] = 'Y'
			scope.state.manifest[0] = 'Y'
			if plan.references()[0].Role == "changed" || plan.metadataBytes()[0] == 'X' || plan.objectBytes(0)[0] == 'X' || plan.objectBytes(0)[0] == 'Y' {
				t.Fatal("alias escape")
			}
			if plan.objectBytes(-1) != nil || plan.objectBytes(99) != nil {
				t.Fatal("ordinal bound")
			}
		})
	}
}
func TestScopeRetentionPlanRejectsForgedScope(t *testing.T) {
	for _, kind := range []string{"zero", "empty-state", "body", "digest", "length", "path", "role", "uri", "identity", "selection", "partition", "manifest", "authority", "count", "noncanonical"} {
		t.Run(kind, func(t *testing.T) {
			scope := retentionFixture(t, false)
			switch kind {
			case "zero":
				scope = featureScope{}
			case "empty-state":
				scope = featureScope{&featureScopeState{}}
			case "body":
				scope.state.files[0].Bytes[0] ^= 1
			case "digest":
				scope.state.files[0].SHA256 = Digest([]byte("wrong"))
			case "length":
				scope.state.files[0].ByteLength++
			case "path":
				scope.state.files[0].Path = "other.go"
			case "role":
				scope.state.files[4].Role = "NOMINATION"
			case "uri":
				scope.state.files[0].URI = "file:///wrong"
			case "identity":
				scope.state.files[0].Identity.Inode = 0
			case "selection":
				scope.state.selection = append(scope.state.selection, ' ')
			case "partition":
				scope.state.plan = append(scope.state.plan, ' ')
			case "manifest":
				scope.state.manifest = []byte(`{}`)
			case "authority":
				scope.state.manifest = bytes.Replace(scope.state.manifest, []byte(`"authority":0`), []byte(`"authority":1`), 1)
			case "count":
				scope.state.files = scope.state.files[:5]
			case "noncanonical":
				scope.state.manifest = append(scope.state.manifest, ' ')
			}
			plan, err := deriveScopeRetentionPlan(scope, math.MaxUint64, math.MaxUint64)
			if err == nil || plan.state != nil {
				t.Fatal("forged scope accepted")
			}
		})
	}
}
func TestScopeRetentionBudgetEqualityOverflowAndRefusal(t *testing.T) {
	scope := retentionFixture(t, false)
	plan, err := deriveScopeRetentionPlan(scope, math.MaxUint64, math.MaxUint64)
	if err != nil {
		t.Fatal(err)
	}
	b := plan.state.budget
	if _, err = deriveScopeRetentionPlan(scope, b.bytes, b.work); err != nil {
		t.Fatal("budget equality", err)
	}
	for _, limit := range []scopeRetentionBudget{{b.bytes - 1, b.work}, {b.bytes, b.work - 1}, {0, 0}} {
		p, err := deriveScopeRetentionPlan(scope, limit.bytes, limit.work)
		if err == nil || p.state != nil {
			t.Fatal("short budget accepted")
		}
	}
	if _, err = retentionAdd(math.MaxUint64, 1); err == nil {
		t.Fatal("addition overflow")
	}
	if _, err = retentionMultiply(math.MaxUint64, 2); err == nil {
		t.Fatal("multiplication overflow")
	}
	for _, v := range [][4]uint64{{math.MaxUint64, 1, 1, 9}, {1, 2, 1, 9}, {1, 1, math.MaxUint64, 9}, {1, 1, 1, 10}, {scopeRetentionByteCeiling, 1, 1, 9}, {1, 1, scopeRetentionWorkCeiling, 9}} {
		if _, err = retentionBudget(v[0], v[1], v[2], v[3]); err == nil {
			t.Fatal("invalid budget", v)
		}
	}
	// This unit has no writable capability. Even failure must not alter source bytes.
	var manifest scopeManifest
	if err := json.Unmarshal(scope.ManifestBytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	for _, f := range scope.Files() {
		raw, e := os.ReadFile(filepath.Join(manifest.Root, f.Path))
		if e != nil || !bytes.Equal(raw, f.Bytes) {
			t.Fatal("plan wrote source", e)
		}
	}
	if (scopeRetentionPlan{}).metadataBytes() != nil || (scopeRetentionPlan{}).references() != nil || (scopeRetentionPlan{}).objectBytes(0) != nil {
		t.Fatal("zero plan")
	}
}

func TestScopeRetentionSourceBounds(t *testing.T) {
	root := scopeFixture(t)
	sizes := []int{featureScopeFileLimit, featureScopeFileLimit, featureScopeFileLimit, featureScopeFileLimit - 2, 1, 1}
	for i, path := range featureScopePaths() {
		if err := os.WriteFile(filepath.Join(root, path), bytes.Repeat([]byte{byte(i)}, sizes[i]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	scope, err := freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
	if err != nil || len(plan.references()) != 9 {
		t.Fatal("exact total bound rejected", err)
	}
	var unique, largest uint64
	for _, object := range plan.state.objects {
		size := uint64(len(object.raw))
		unique += size
		if size > largest {
			largest = size
		}
	}
	if plan.state.budget.bytes != 2*(unique+largest+scopeRetentionMetadataLimit) {
		t.Fatal("both copies or staging omitted")
	}
	scope.state.files[5].Bytes = []byte{1, 2}
	if p, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling); err == nil || p.state != nil {
		t.Fatal("plus-one source total accepted")
	}
	scope.state.files[0].Bytes = make([]byte, featureScopeFileLimit+1)
	if p, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling); err == nil || p.state != nil {
		t.Fatal("plus-one file bound accepted")
	}
}

// Substitute an ancestor above the named source root at the admission hook.
// Every fixture path remains inside a test-owned parent, including moved trees.
func TestFeatureScopeHigherAncestorSubstitution(t *testing.T) {
	root := scopeFixture(t)
	ancestor := filepath.Dir(root)
	// Do not rename t.TempDir's harness-owned parent: create our own higher ancestor.
	owned := filepath.Join(ancestor, "owned-ancestor")
	if err := os.Mkdir(owned, 0755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(owned, "source")
	if err := os.Rename(root, nested); err != nil {
		t.Fatal(err)
	}
	moved := owned + "-moved"
	t.Cleanup(func() { os.RemoveAll(owned); os.RemoveAll(moved) })
	fired := false
	scope, err := freezeFeatureScopeWithOps(context.Background(), nested, fixtureRevision, nil, scopeReadOps{hook: func(stage, path string) error {
		if stage == "root-opened" && !fired {
			fired = true
			if e := os.Rename(owned, moved); e != nil {
				return e
			}
			return os.Mkdir(owned, 0755)
		}
		return nil
	}})
	if !fired || err == nil || scope.state != nil {
		t.Fatal("higher ancestor substitution accepted", err)
	}
}
