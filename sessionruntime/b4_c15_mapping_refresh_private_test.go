package sessionruntime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateB4C15CanonicalContractExactly44UniqueKeys(t *testing.T) {
	if err := privateC15ValidateContracts(privateC15SemanticContracts, 44); err != nil {
		t.Fatal(err)
	}
	keys := map[privateC15SemanticKey]bool{}
	for _, c := range privateC15SemanticContracts {
		keys[c.Key] = true
	}
	if len(privateC15SemanticContracts) != 44 || len(keys) != 44 {
		t.Fatalf("ASSERT_C15_CANONICAL_EXACT44 rows=%d keys=%d", len(privateC15SemanticContracts), len(keys))
	}
}

func TestPrivateB4C15PersistedMappingMutationsFailClosed(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	cloneMapping := func() privateB4C15OperationMapping {
		clone := mapping
		clone.Rows = append([]privateB4C15MappingRow(nil), mapping.Rows...)
		return clone
	}
	cloneContracts := func() []privateC15SemanticContract {
		return append([]privateC15SemanticContract(nil), privateC15SemanticContracts...)
	}
	rowIndex := func(site string) int {
		t.Helper()
		for i := range mapping.Rows {
			if mapping.Rows[i].SiteID == site {
				return i
			}
		}
		t.Fatalf("test fixture site absent: %s", site)
		return -1
	}
	contractIndex := func(site string) int {
		t.Helper()
		for i := range privateC15SemanticContracts {
			if privateC15SemanticContracts[i].Key.SiteID == site {
				return i
			}
		}
		t.Fatalf("test fixture contract absent: %s", site)
		return -1
	}
	assertFails := func(name, contains string, candidate privateB4C15OperationMapping, contracts []privateC15SemanticContract) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			_, err := privateC15VerifyPersistedMapping(repo, candidate, contracts, nil)
			if err == nil || !strings.Contains(err.Error(), contains) {
				t.Fatalf("wanted failure containing %q, got %v", contains, err)
			}
		})
	}

	changedForm := cloneContracts()
	changedForm[0].Key.BindingForm = privateC15BindingForm("UNKNOWN_V1")
	assertFails("unknown binding form", "unknown binding form", cloneMapping(), changedForm)

	wrongFunction := cloneMapping()
	wrongFunction.Rows[0].BindingFunction = "forgedFunction"
	assertFails("wrong function", "function mismatch", wrongFunction, cloneContracts())

	wrongRole := cloneMapping()
	wrongRole.Rows[0].OperationKind = "forged-role"
	assertFails("wrong role call selector", "role/call selector mismatch", wrongRole, cloneContracts())

	staleDigest := cloneMapping()
	staleDigest.Rows[0].ASTDigest = strings.Repeat("0", 64)
	assertFails("stale persisted digest", "digest mismatch", staleDigest, cloneContracts())

	duplicateKey := cloneContracts()
	duplicateKey[1].Key = duplicateKey[0].Key
	assertFails("duplicate semantic key", "duplicate semantic key", cloneMapping(), duplicateKey)

	missingRow := cloneMapping()
	missingRow.Rows = append(missingRow.Rows[:rowIndex("c15.owner.08.source-descriptor-transfer")], missingRow.Rows[rowIndex("c15.owner.08.source-descriptor-transfer")+1:]...)
	assertFails("missing persisted row", "persisted rows=43", missingRow, cloneContracts())

	extraRow := cloneMapping()
	extraRow.Rows = append(extraRow.Rows, extraRow.Rows[0])
	assertFails("extra persisted row", "persisted rows=45", extraRow, cloneContracts())

	aliasCollision := cloneContracts()
	alias43 := contractIndex("c15.alias.43.borrowed-lexical-result")
	alias44 := contractIndex("c15.alias.44.borrowed-capture-values")
	aliasCollision[alias44].Key.RoleSelector = aliasCollision[alias43].Key.RoleSelector
	assertFails("alias43 alias44 discriminator removed", "unexplained duplicate operation identity", cloneMapping(), aliasCollision)
}

func TestPrivateB4C15Alias43CanonicalDiscriminator(t *testing.T) {
	const source = `package p
const privateC15SiteC15Alias43BorrowedLexicalResult = "c15.alias.43.borrowed-lexical-result"
func PreparePrivateB4DefinitionBorrowed(){ _ = privateC15SiteC15Alias43BorrowedLexicalResult; prepareResult() }
func CommitPrivateB4DefinitionBorrowed(){ _ = privateC15SiteC15Alias43BorrowedLexicalResult; commitResult() }
`
	contract := c15c("c15.alias.43.borrowed-lexical-result", "model.go", "PreparePrivateB4DefinitionBorrowed", "manager-scoped-borrow-alias:result", privateC15MarkerAdjacentV1, "")
	occurrences, err := privateC15Discover([]privateC15Source{{"model.go", []byte(source)}}, []privateC15SemanticContract{contract})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privateC15Resolve([]privateC15SemanticContract{contract}, occurrences, false); err != nil {
		t.Fatalf("alias43 canonical match: %v", err)
	}

	weakened := []privateC15Occurrence{
		{Key: contract.Key},
		{Key: contract.Key},
	}
	if _, err := privateC15Resolve([]privateC15SemanticContract{contract}, weakened, false); err == nil || !strings.Contains(err.Error(), "found=2") {
		t.Fatalf("ASSERT_C15_ALIAS43_REMOVE_DISCRIMINATOR_FOUND2 err=%v", err)
	}

	forged := source + `func PreparePrivateB4DefinitionBorrowed(){ _ = privateC15SiteC15Alias43BorrowedLexicalResult; forged() }` + "\n"
	occurrences, err = privateC15Discover([]privateC15Source{{"model.go", []byte(forged)}}, []privateC15SemanticContract{contract})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privateC15Resolve([]privateC15SemanticContract{contract}, occurrences, false); err == nil || !strings.Contains(err.Error(), "found=2") {
		t.Fatalf("ASSERT_C15_ALIAS43_FORGED_MULTIPLE err=%v", err)
	}
}

func TestPrivateB4C15Owner08SelfBound(t *testing.T) {
	const site = "c15.owner.08.source-descriptor-transfer"
	const source = `package p
const privateC15SiteC15Owner08SourceDescriptorTransfer = "c15.owner.08.source-descriptor-transfer"
func transferStates(){ dst.reserveDescriptor(privateC15SiteC15Owner08SourceDescriptorTransfer) }
`
	contract := c15c(site, "model.go", "transferStates", "atomic-custody-transfer", privateC15SelfBoundV1, "")
	occurrences, err := privateC15Discover([]privateC15Source{{"model.go", []byte(source)}}, []privateC15SemanticContract{contract})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privateC15Resolve([]privateC15SemanticContract{contract}, occurrences, false); err != nil {
		t.Fatalf("owner08 self-bound: %v", err)
	}
	missing := strings.ReplaceAll(source, "privateC15SiteC15Owner08SourceDescriptorTransfer", "missingIdentity")
	occurrences, err = privateC15Discover([]privateC15Source{{"model.go", []byte(missing)}}, []privateC15SemanticContract{contract})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privateC15Resolve([]privateC15SemanticContract{contract}, occurrences, false); err == nil || !strings.Contains(err.Error(), "found=0") {
		t.Fatalf("ASSERT_C15_OWNER08_MISSING_IDENTITY err=%v", err)
	}
}

func TestPrivateB4C15StaleDigestFailsWithoutNormalization(t *testing.T) {
	const source = `package p
const privateC15SiteC15Copy02ParamsIngress = "c15.copy.02.params-ingress"
func copyPrivateB4Params(){ _ = privateC15SiteC15Copy02ParamsIngress; copyNow() }
`
	contract := c15c("c15.copy.02.params-ingress", "model.go", "copyPrivateB4Params", "reserve-before-append-copy", privateC15MarkerAdjacentV1, "forged-old")
	occurrences, err := privateC15Discover([]privateC15Source{{"model.go", []byte(source)}}, []privateC15SemanticContract{contract})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privateC15Resolve([]privateC15SemanticContract{contract}, occurrences, true); err == nil || !strings.Contains(err.Error(), "stale operation digest") {
		t.Fatalf("ASSERT_C15_STALE_DIGEST_FAIL err=%v", err)
	}
	if contract.FrozenDigest != "forged-old" {
		t.Fatal("ASSERT_C15_STALE_DIGEST_NORMALIZED")
	}
}

func TestPrivateB4C15DiscoveryPermutationDeterministic(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	base, err := privateC15Generate(repo, privateC15SemanticContracts, mapping, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, c := range privateC15SemanticContracts {
		paths[c.Path] = true
	}
	permutation := make([]int, 0, len(paths))
	for i := len(paths) - 1; i >= 0; i-- {
		permutation = append(permutation, i)
	}
	reversed, err := privateC15Generate(repo, privateC15SemanticContracts, mapping, false, permutation)
	if err != nil {
		t.Fatal(err)
	}
	if !privateC15GenerationEqual(base, reversed) {
		t.Fatal("ASSERT_C15_DISCOVERY_ORDER_CHANGED_OUTPUT")
	}
	if base.Rows != 44 || base.UniqueKeys != 44 || base.Denominator != 44 {
		t.Fatalf("ASSERT_C15_GENERATED_EXACT44 rows=%d keys=%d denominator=%d", base.Rows, base.UniqueKeys, base.Denominator)
	}
}

func TestPrivateB4C15VerifyCanonicalMapping(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	generation, err := privateC15Generate(repo, privateC15SemanticContracts, mapping, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generation.Mapping, mustReadC15File(t, "operation-mapping.json")) {
		t.Fatal("ASSERT_C15_CANONICAL_MAPPING_DIFF")
	}
}

func TestPrivateB4C15RefreshExactMapping(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	generation, err := privateC15Generate(repo, privateC15SemanticContracts, mapping, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ASSERT_C15_MAPPING_DRY_RUN rows=%d keys=%d denominator=%d changes=%d\n%s", generation.Rows, generation.UniqueKeys, generation.Denominator, generation.Changes, generation.Comparison)
	if os.Getenv("C15_REFRESH_MAPPING") != "1" {
		return
	}
	if err := privateC15AtomicRefresh(filepath.Join("testdata", "c15-census-v1"), generation); err != nil {
		t.Fatal(err)
	}
}

func mustReadC15File(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "c15-census-v1", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
