package containment

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	containmentreference "lsp-trace/internal/containment/reference"
)

func TestRuntimeGateAlwaysUnavailable(t *testing.T) {
	const assertion = "production runtime gate remains unavailable on every platform"
	t.Log("ASSERTION: " + assertion)
	got := NewRuntimeGate().Snapshot()
	if got.Classification != Unavailable {
		t.Fatalf("%s: got %q", assertion, got.Classification)
	}
	if got.Reason == "" || got.FailedCheck == "" || got.Platform == "" {
		t.Fatalf("%s: incomplete unavailable result: %+v", assertion, got)
	}
}

func TestReferenceClassificationCannotAuthorizeRuntime(t *testing.T) {
	const assertion = "reference VERIFIED is a distinct non-authorizing type and cannot construct a runtime verdict"
	t.Log("ASSERTION: " + assertion)
	observation := containmentreference.CompleteObservationForTest()
	got := containmentreference.Evaluate(observation)
	if got.Classification != containmentreference.Verified {
		t.Fatalf("%s: reference result=%+v", assertion, got)
	}
	if reflect.TypeOf(got) == reflect.TypeOf(NewRuntimeGate().Snapshot()) {
		t.Fatalf("%s: reference and runtime results share a type", assertion)
	}

	productionType := reflect.TypeOf(RuntimeGate{})
	for i := 0; i < productionType.NumField(); i++ {
		if productionType.Field(i).PkgPath == "" {
			t.Fatalf("%s: RuntimeGate exposes field %s", assertion, productionType.Field(i).Name)
		}
	}
}

func TestReferenceVocabularyAndFirstFailureOrder(t *testing.T) {
	const assertion = "reference classification uses exact closed vocabulary and deterministic designed first-failure order"
	t.Log("ASSERTION: " + assertion)

	wantChecks := []containmentreference.CheckID{
		containmentreference.CheckPlatformSupport,
		containmentreference.CheckPrimitiveProfile,
		containmentreference.CheckOwnerDeathCleanup,
		containmentreference.CheckCompleteDescendants,
		containmentreference.CheckCreationRace,
		containmentreference.CheckEscape,
		containmentreference.CheckTransfer,
		containmentreference.CheckReparent,
		containmentreference.CheckDelegation,
		containmentreference.CheckInheritedIOSafety,
		containmentreference.CheckControlAuthority,
		containmentreference.CheckCleanupBound,
		containmentreference.CheckSurvivorEnumeration,
		containmentreference.CheckDeathObservation,
		containmentreference.CheckReap,
		containmentreference.CheckImmutableAttestation,
	}
	if got := containmentreference.OrderedChecks(); !reflect.DeepEqual(got, wantChecks) {
		t.Fatalf("%s: checks=%q want=%q", assertion, got, wantChecks)
	}

	observation := containmentreference.CompleteObservationForTest()
	observation.PlatformSupport = false
	observation.PrimitiveProfile = false
	got := containmentreference.Evaluate(observation)
	if got.Classification != containmentreference.Unavailable || got.Reason != containmentreference.ReasonUnsupportedPlatform || got.FailedCheck != containmentreference.CheckPlatformSupport {
		t.Fatalf("%s: first result=%+v", assertion, got)
	}

	observation = containmentreference.CompleteObservationForTest()
	observation.CreationRace = false
	observation.Transfer = false
	got = containmentreference.Evaluate(observation)
	if got.Reason != containmentreference.ReasonProbeFailed || got.FailedCheck != containmentreference.CheckCreationRace {
		t.Fatalf("%s: later result=%+v", assertion, got)
	}
}

func TestClosedReasons(t *testing.T) {
	const assertion = "reference unavailable reasons are exactly the designed closed reason vocabulary"
	t.Log("ASSERTION: " + assertion)
	want := []containmentreference.Reason{
		containmentreference.ReasonUnsupportedPlatform,
		containmentreference.ReasonUnsupportedProfile,
		containmentreference.ReasonProbeFailed,
		containmentreference.ReasonIndeterminate,
		containmentreference.ReasonMutableConfiguration,
		containmentreference.ReasonInsufficientAuthorityIsolation,
		containmentreference.ReasonNativeTimeout,
	}
	if got := containmentreference.Reasons(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: reasons=%q want=%q", assertion, got, want)
	}
}

func TestProductionContainmentDoesNotImportReference(t *testing.T) {
	const assertion = "no production Go file imports the containment reference package"
	t.Log("ASSERTION: " + assertion)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"reference"+string(filepath.Separator)) {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range file.Imports {
			if strings.Contains(spec.Path.Value, "/internal/containment/reference") {
				t.Errorf("%s: %s imports %s", assertion, path, spec.Path.Value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
}

func TestLocalAuthorityAndEvidenceCeilingDocumented(t *testing.T) {
	const assertion = "local containment documentation states authority and native-evidence ceilings"
	t.Log("ASSERTION: " + assertion)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "AUTHORITY.md"))
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	text := string(body)
	for _, required := range []string{
		"does not prove native containment",
		"cannot authorize a production runtime verdict",
		"all platforms remain unavailable",
		"CONTAINMENT_UNAVAILABLE",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("%s: missing %q", assertion, required)
		}
	}
}

func TestNativeProofScaffoldIsNonAuthorizing(t *testing.T) {
	const assertionAuthority = "native-proof scaffolding defines no authorization result or conversion path"
	const assertionSupport = "native-proof scaffolding contains no supported/verified state claim"
	t.Log("ASSERTION: " + assertionAuthority)
	t.Log("ASSERTION: " + assertionSupport)

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package")
	}
	path := filepath.Join(filepath.Dir(filename), "native_proof.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: scaffold unavailable: %v", assertionAuthority, err)
		t.Errorf("%s: scaffold unavailable: %v", assertionSupport, err)
		return
	}
	text := string(body)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, body, 0)
	if err != nil {
		t.Fatalf("%s: parse scaffold: %v", assertionAuthority, err)
	}
	forbidden := map[string]bool{"Verified": true, "Supported": true, "Classification": true, "runtimeResult": true, "Snapshot": true, "RuntimeGate": true}
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && forbidden[ident.Name] {
			t.Errorf("%s: forbidden authority identifier %q", assertionAuthority, ident.Name)
		}
		return true
	})
	for _, required := range []string{"type nativeProof interface", "Evidence() nativeEvidence", "type nativeEvidence struct"} {
		if !strings.Contains(text, required) {
			t.Errorf("%s: missing %q", assertionSupport, required)
		}
	}
}

func TestProductionAPIHasNoForgeableVerifiedConstructor(t *testing.T) {
	const assertion = "production containment exports no VERIFIED value, attestation, injectable classifier, or reference conversion"
	t.Log("ASSERTION: " + assertion)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package")
	}
	dir := filepath.Dir(filename)
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := packages["containment"]
	for _, file := range pkg.Files {
		for _, declaration := range file.Decls {
			switch node := declaration.(type) {
			case *ast.FuncDecl:
				if node.Name.IsExported() {
					allowedConstructor := node.Recv == nil && node.Name.Name == "NewRuntimeGate"
					allowedSnapshot := node.Recv != nil && node.Name.Name == "Snapshot"
					if !allowedConstructor && !allowedSnapshot {
						t.Errorf("%s: exported callable %s", assertion, node.Name.Name)
					}
				}
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					switch value := spec.(type) {
					case *ast.TypeSpec:
						if value.Name.IsExported() && value.Name.Name != "RuntimeGate" && value.Name.Name != "Snapshot" && value.Name.Name != "Classification" && value.Name.Name != "Reason" && value.Name.Name != "CheckID" {
							t.Errorf("%s: exported type %s", assertion, value.Name.Name)
						}
					case *ast.ValueSpec:
						for _, name := range value.Names {
							if name.IsExported() && name.Name != "Unavailable" {
								t.Errorf("%s: exported value %s", assertion, name.Name)
							}
						}
					}
				}
			}
		}
	}
}
