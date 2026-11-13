package programcadmission

import (
	"bytes"
	"encoding"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/sessionruntime"
)

// Test-only reflection pins the explicit visitor's transitive encoding surface.
// No production reflective preflight or generic encoder is introduced.
func TestSealedBindingBudget18ClosedShape(t *testing.T) {
	for _, tc := range []struct {
		typ  reflect.Type
		want string
	}{
		{reflect.TypeOf(CompositeSourceBinding{}), "CompositeID:string:|CompositeOutputSHA256:string:|ClaimCeiling:string:|Authority:int:|SourceGraphComplete:string:|Compatibility:programcadmission.CompatibilityReference:|Constituents:[]programcadmission.ConstituentReference:|Completeness:programcadmission.CompositeCompleteness:"},
		{reflect.TypeOf(CompatibilityReference{}), "WorkspaceURI:string:|SourceRevision:string:|PositionEncoding:string:|RevisionCustody:string:|AcquisitionSemantics:string:|PrivacyPolicy:string:|ServerCommand:string:|ServerVersion:string:|LanguageID:string:|EvidenceSemantics:json.RawMessage:|SensitivityPolicy:json.RawMessage:"},
		{reflect.TypeOf(CompositeCompleteness{}), "AllTraversalComplete:bool:|AnyTruncated:bool:|WholeWorkspace:bool:|PerInput:[]json.RawMessage:"},
		{reflect.TypeOf(ConstituentReference{}), "Identity:string:|SHA256:string:|SchemaVersion:string:|GraphSHA256:string:|GraphSchemaID:string:|ByteLength:int:|GraphByteLength:int:|BytesBase64:string:|SessionID:string:|InvocationID:string:|Generation:uint64:|RevisionCustody:string:|SourcePolicy:string:|WorkspaceURI:string:|AnalyzedVersion:string:|DependencyCompleteness:string:|CaptureBudget:graphprovenance.CaptureBudgetV2:|Supplies:[]graphprovenance.SupplyReceiptV2:|Captures:[]graphprovenance.Receipt:|Bindings:[]graphprovenance.BindingV2:|Invocation:json.RawMessage:|Seeds:json.RawMessage:|SeedMemberships:json.RawMessage:|Frontier:json.RawMessage:|Diagnostics:json.RawMessage:|Summary:json.RawMessage:|Slice:json.RawMessage:"},
		{reflect.TypeOf(graphprovenance.CaptureBudgetV2{}), "RootStatus:string:root_status|Attempts:int:attempts|ChargedBytes:int:charged_bytes"},
		{reflect.TypeOf(graphprovenance.SupplyReceiptV2{}), "RequestID:string:request_id|Status:string:status|Observation:acquisition.Supply:observation|Receipt:*graphprovenance.Receipt:receipt"},
		{reflect.TypeOf(acquisition.Supply{}), "URI:string:uri|LanguageID:string:language_id,omitempty|Observation:json.RawMessage:observation,omitempty"},
		{reflect.TypeOf(graphprovenance.Receipt{}), "ID:string:id|URI:string:uri|Path:string:path|Classification:string:classification|AnalyzedVersion:string:analyzed_version|Status:string:status|Content:[]uint8:content|CanonicalReceipt:[]uint8:canonical_receipt|Supply:*graphprovenance.SupplyMetadata:supply,omitempty"},
		{reflect.TypeOf(graphprovenance.SupplyMetadata{}), "SessionID:string:session_id|Generation:uint64:generation|Version:int:document_version|Method:string:method|Params:json.RawMessage:params"},
		{reflect.TypeOf(graphprovenance.BindingV2{}), "Pointer:string:pointer|URI:string:uri|Attribution:string:attribution|AnchorStatus:string:anchor_status|ReceiptIDs:[]string:receipt_ids"},
	} {
		var fields []string
		for i := 0; i < tc.typ.NumField(); i++ {
			f := tc.typ.Field(i)
			fields = append(fields, f.Name+":"+f.Type.String()+":"+f.Tag.Get("json"))
			if f.Anonymous || f.PkgPath != "" {
				t.Fatal("ASSERT_CLOSED_SHAPE_EXPORT", tc.typ, f.Name)
			}
		}
		if got := strings.Join(fields, "|"); got != tc.want {
			t.Fatalf("ASSERT_CLOSED_SHAPE %s\ngot %s\nwant %s", tc.typ, got, tc.want)
		}
		for _, typ := range []reflect.Type{tc.typ, reflect.PointerTo(tc.typ)} {
			if typ.Implements(reflect.TypeOf((*json.Marshaler)(nil)).Elem()) || typ.Implements(reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()) {
				t.Fatal("ASSERT_CLOSED_SHAPE_NO_CUSTOM_ENCODER", typ)
			}
		}
	}
	t.Log("ASSERT_CLOSED_SHAPE: PASS")
}

func richBinding18(text string, raw json.RawMessage) CompositeSourceBinding {
	var out CompositeSourceBinding
	var fill func(reflect.Value)
	rawType := reflect.TypeOf(json.RawMessage{})
	fill = func(v reflect.Value) {
		if v.Type() == rawType {
			v.SetBytes(raw)
			return
		}
		switch v.Kind() {
		case reflect.String:
			v.SetString(text)
		case reflect.Int:
			v.SetInt(-int64(^uint(0)>>1) - 1)
		case reflect.Uint64:
			v.SetUint(math.MaxUint64)
		case reflect.Bool:
			v.SetBool(true)
		case reflect.Pointer:
			v.Set(reflect.New(v.Type().Elem()))
			fill(v.Elem())
		case reflect.Slice:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				v.SetBytes([]byte{0, 1, 254, 255})
				return
			}
			v.Set(reflect.MakeSlice(v.Type(), 2, 2))
			for i := 0; i < v.Len(); i++ {
				fill(v.Index(i))
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				fill(v.Field(i))
			}
		default:
			panic("unsupported test shape")
		}
	}
	fill(reflect.ValueOf(&out).Elem())
	return out
}

func TestSealedBindingBudget18EncodingUpperBound(t *testing.T) {
	for _, text := range []string{"", "plain/base64+ABC=", "\"\\\b\f\n\r\t\x00\x1f<>&", "é日本\u2028\u2029", string([]byte{0xff, 0xfe, 0xc0, 0x80})} {
		for _, raw := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage{'"', 0xff, '"'}, json.RawMessage("\"<>&\u2028\u2029\""), json.RawMessage(` { "s": "<>&\u2028\u2029", "a": [true,false,-9223372036854775808,18446744073709551615], "nested":{"k":"\\u003c"} } `)} {
			source := richBinding18(text, raw)
			bound, ok := privateCompositeBindingBound(source)
			encoded, err := json.Marshal(source)
			if err != nil || !ok || bound > sessionruntime.PrivateGroupingQueryMaxBytes || uint64(len(encoded)) >= bound {
				t.Fatalf("ASSERT_ENCODING_UPPER_BOUND len=%d bound=%d ok=%v err=%v", len(encoded), bound, ok, err)
			}
			cloned := CloneCompositeSourceBinding(source)
			cloneRaw, err := json.Marshal(cloned)
			if err != nil || uint64(len(cloneRaw)) >= bound {
				t.Fatal("ASSERT_CLONE_ENCODING_BOUND", err)
			}
		}
	}
	// Nil versus non-nil empty is deliberately not normalized by the estimator.
	for _, source := range []CompositeSourceBinding{{}, {Constituents: []ConstituentReference{}, Completeness: CompositeCompleteness{PerInput: []json.RawMessage{}}}, {Constituents: []ConstituentReference{{Captures: []graphprovenance.Receipt{{Content: []byte{}, CanonicalReceipt: []byte{}}}, Bindings: []graphprovenance.BindingV2{{ReceiptIDs: []string{}}}, Supplies: []graphprovenance.SupplyReceiptV2{}}}}} {
		bound, ok := privateCompositeBindingBound(source)
		encoded, err := json.Marshal(CloneCompositeSourceBinding(source))
		if !ok || err != nil || uint64(len(encoded)) >= bound {
			t.Fatal("ASSERT_EMPTY_ENCODING_BOUND", bound, len(encoded), err)
		}
	}
	optionalEmpty := CompositeSourceBinding{Authority: int(^uint(0) >> 1), Constituents: []ConstituentReference{{Supplies: []graphprovenance.SupplyReceiptV2{{Observation: acquisition.Supply{Observation: json.RawMessage{}}}}}}}
	bound, ok := privateCompositeBindingBound(optionalEmpty)
	encoded, err := json.Marshal(optionalEmpty)
	if !ok || err != nil || uint64(len(encoded)) >= bound {
		t.Fatal("ASSERT_OPTIONAL_EMPTY_RAW_OMITTED", err)
	}
	t.Log("ASSERT_ENCODING_UPPER_BOUND: PASS")
}

// Atom checks avoid aggregate slack hiding a missed escape or base64 byte.
func TestSealedBindingBudget18AtomBounds(t *testing.T) {
	for _, text := range []string{"", "ascii", "\"\\\x00\b\t\n\r\x1f<>&", "é日本\u2028\u2029", string([]byte{0xff, 0xc0, 0x80}), strings.Repeat("<>&", 128)} {
		b := privateBindingBudget{ok: true}
		b.str(text)
		raw, err := json.Marshal(text)
		if err != nil || !b.ok || b.total < uint64(len(raw)) {
			t.Fatal("ASSERT_ATOM_STRING_BOUND", b.total, len(raw), err)
		}
	}
	t.Log("ASSERT_ATOM_STRING_BOUND: PASS")
	for _, raw := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage("\"<>&\u2028\u2029\""), json.RawMessage("\"" + strings.Repeat("<>&", 128) + "\""), json.RawMessage{'"', 0xff, '"'}} {
		b := privateBindingBudget{ok: true}
		b.raw(raw)
		encoded, err := json.Marshal(raw)
		if err != nil || !b.ok || b.total < uint64(len(encoded)) {
			t.Fatal("ASSERT_ATOM_RAW_BOUND", b.total, len(encoded), err)
		}
	}
	for _, raw := range [][]byte{nil, {}, {0}, {0, 1}, {0, 1, 2}, {0, 1, 2, 3}} {
		b := privateBindingBudget{ok: true}
		b.bytes(raw)
		encoded, _ := json.Marshal(raw)
		if !b.ok || b.total < uint64(len(encoded)) {
			t.Fatal("ASSERT_ATOM_BASE64_BOUND", b.total, len(encoded))
		}
	}
	t.Log("ASSERT_ATOM_RAW_BOUND: PASS")
	t.Log("ASSERT_ATOM_BASE64_BOUND: PASS")
	t.Log("ASSERT_ATOM_BOUNDS: PASS")
}

func TestSealedBindingBudget18ArithmeticAndRefusal(t *testing.T) {
	cap := sessionruntime.PrivateGroupingQueryMaxBytes
	for _, tc := range []struct {
		total, n uint64
		ok       bool
	}{{0, cap, true}, {cap, 0, true}, {cap, 1, false}, {math.MaxUint64, 0, false}, {0, math.MaxUint64, false}} {
		b := privateBindingBudget{total: tc.total, ok: true}
		if b.add(tc.n) != tc.ok {
			t.Fatal("ASSERT_BUDGET_ARITHMETIC", tc)
		}
	}
	for _, raw := range []json.RawMessage{json.RawMessage{}, json.RawMessage("{"), json.RawMessage("null null"), json.RawMessage(strings.Repeat("[", 10001) + strings.Repeat("]", 10001))} {
		source := CompositeSourceBinding{Compatibility: CompatibilityReference{EvidenceSemantics: raw}}
		if _, ok := privateCompositeBindingBound(source); ok {
			t.Fatal("ASSERT_INVALID_RAW_REFUSED")
		}
	}
	for _, source := range []CompositeSourceBinding{
		{CompositeID: strings.Repeat("x", int(cap))},
		{Compatibility: CompatibilityReference{EvidenceSemantics: json.RawMessage(strings.Repeat(" ", int(cap)) + "null")}},
		{Constituents: []ConstituentReference{{Captures: []graphprovenance.Receipt{{Content: make([]byte, int(cap))}}}}},
	} {
		if _, ok := privateCompositeBindingBound(source); ok {
			t.Fatal("ASSERT_OVERSIZE_REFUSED")
		}
	}
	for _, raw := range [][]byte{nil, {}, {0}, {0, 1}, {0, 1, 2}, {0, 1, 2, 3}} {
		b := privateBindingBudget{ok: true}
		b.bytes(raw)
		encoded, _ := json.Marshal(raw)
		if !b.ok || b.total < uint64(len(encoded)) {
			t.Fatal("ASSERT_BASE64_BOUND", len(raw), b.total, len(encoded))
		}
	}
	var p privateLeidenProjection
	if binding, bound, ok := p.PrivateCompositeSourceBindingBudget(); ok || bound != 0 || !reflect.DeepEqual(binding, CompositeSourceBinding{}) {
		t.Fatal("ASSERT_INVALID_PROJECTION_REFUSED")
	}
	b := privateBindingBudget{total: cap - 2, ok: true}
	b.str("")
	if !b.ok || b.total != cap {
		t.Fatal("ASSERT_EXACT_CEILING")
	}
	if b.add(1) {
		t.Fatal("ASSERT_OVER_CEILING")
	}
	base, ok := privateCompositeBindingBound(CompositeSourceBinding{})
	if !ok || base >= cap {
		t.Fatal("SETUP_BOUNDARY_BASE")
	}
	at := CompositeSourceBinding{CompositeID: strings.Repeat("x", int(cap-base))}
	bound, ok := privateCompositeBindingBound(at)
	encoded, err := json.Marshal(at)
	if !ok || bound != cap || err != nil || uint64(len(encoded)) >= bound {
		t.Fatal("ASSERT_COMPUTED_BUDGET_AT_CEILING", bound, len(encoded), err)
	}
	at.CompositeID += "x"
	if _, ok := privateCompositeBindingBound(at); ok {
		t.Fatal("ASSERT_COMPUTED_BUDGET_OVER_CEILING")
	}
	t.Log("ASSERT_INVALID_RAW_REFUSED: PASS")
	t.Log("ASSERT_BUDGET_REFUSAL: PASS")
}

// This is a helper-only clone-isolation test, not a forged admission fixture.
func TestSealedBindingBudget18CloneIsolation(t *testing.T) {
	source := richBinding18("one", json.RawMessage(`{"v":"one"}`))
	before, _ := json.Marshal(source)
	clone := CloneCompositeSourceBinding(source)
	clone.Constituents[0].Invocation[0] = '['
	clone.Constituents[0].Captures[0].Content[0] = 99
	clone.Constituents[0].Supplies[0].Observation.Observation[0] = '['
	clone.Completeness.PerInput[0][0] = '['
	after, _ := json.Marshal(source)
	// The test builder deliberately shares raw leaves, but the clone must not.
	if !bytes.Equal(before, after) {
		t.Fatal("ASSERT_DEFENSIVE_BINDING_ISOLATION")
	}
	t.Log("ASSERT_DEFENSIVE_BINDING_ISOLATION: PASS")
}

func ownerOrder18(src []byte) bool {
	f, err := parser.ParseFile(token.NewFileSet(), "owner.go", src, 0)
	if err != nil {
		return false
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "PrivateCompositeSourceBindingBudget" {
			continue
		}
		var buf bytes.Buffer
		if format.Node(&buf, token.NewFileSet(), fn.Body) != nil {
			return false
		}
		return buf.String() == `{
	if !p.calls.Valid() {
		return CompositeSourceBinding{}, 0, false
	}
	bound, ok := privateCompositeBindingBound(p.compositeSource)
	if !ok {
		return CompositeSourceBinding{}, 0, false
	}
	return p.CompositeSourceBinding(), bound, true
}`
	}
	return false
}

func TestSealedBindingBudget18OwnerOrderAST(t *testing.T) {
	src, err := os.ReadFile("leiden_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	if !ownerOrder18(src) {
		t.Fatal("ASSERT_OWNER_BUDGET_BEFORE_CLONE")
	}
	// A clone in either refusing branch must be rejected by the same AST reader.
	bad := bytes.ReplaceAll(src, []byte("return CompositeSourceBinding{}, 0, false"), []byte("return p.CompositeSourceBinding(), 0, false"))
	if ownerOrder18(bad) {
		t.Fatal("ASSERT_OWNER_AST_DISCRIMINATOR")
	}
	// The representation is closed to structs at the source level as well as
	// field/type/tag/method-set pinned above. Interfaces and aliases are forbidden.
	for path, names := range map[string][]string{
		"admission.go":                      {"CompositeSourceBinding", "CompatibilityReference", "CompositeCompleteness", "ConstituentReference"},
		"../graphprovenance/evidence.go":    {"Receipt", "SupplyMetadata"},
		"../graphprovenance/v2.go":          {"SupplyReceiptV2"},
		"../graphprovenance/binding_v2.go":  {"BindingV2"},
		"../graphprovenance/captures_v2.go": {"CaptureBudgetV2"},
		"../acquisition/types.go":           {"Supply"},
	} {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			for _, name := range names {
				if ts.Name.Name == name {
					_, isStruct := ts.Type.(*ast.StructType)
					if !isStruct || ts.Assign.IsValid() {
						t.Fatal("ASSERT_CLOSED_AST_STRUCT", path, name)
					}
					found[name] = true
				}
			}
			return true
		})
		for _, name := range names {
			if !found[name] {
				t.Fatal("ASSERT_CLOSED_AST_FOUND", path, name)
			}
		}
	}
	visitor, err := parser.ParseFile(token.NewFileSet(), "private_binding_budget.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(visitor, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && id.Name == "json" && s.Sel.Name != "Valid" {
					t.Fatal("ASSERT_PREFLIGHT_NOT_ENCODING", s.Sel.Name)
				}
			}
		}
		return true
	})
	for _, imp := range visitor.Imports {
		if imp.Path.Value == `"reflect"` {
			t.Fatal("ASSERT_PREFLIGHT_NOT_REFLECTIVE")
		}
	}
	t.Log("ASSERT_OWNER_BUDGET_BEFORE_CLONE: PASS; CLONE_ON_REFUSAL_MUTATION_REJECTED")
}
