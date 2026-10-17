# Historical evidence: prospective private-B4 source install infallibility

## Status and prior RED meaning

This is a historical source-shape assertion removed from ordinary Go test discovery by repository-green disposition C. It was prospective evidence for a proposal surface, not a production behavior contract. At integrated HEAD `45adb359ea72df107b29d7041cdf26847dd4b62a`, the ordinary test failed at `ASSERT_B4_RESERVATION_STORAGE_FIXED_ARRAY`; that RED meant the proposed fixed-array reservation and post-transfer infallibility shape was absent, not that current production behavior was defective.

## Provenance

- Original path: `sessionruntime/b4_source_acquisition_private_test.go`
- Original test: `TestPrivateB4SourceInstallPathIsProspectivelyInfallible`
- Integrated HEAD: `45adb359ea72df107b29d7041cdf26847dd4b62a`
- Historical assertion source follows verbatim.

```go
func TestPrivateB4SourceInstallPathIsProspectivelyInfallible(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_B4_POST_TRANSFER_SOURCE_LOCATION")
	}
	productionPath := strings.TrimSuffix(file, "b4_source_acquisition_private_test.go") + "b4_definition_private.go"
	production, err := os.ReadFile(productionPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(production)
	installStart := strings.Index(source, "func (m *Manager) installPrivateB4Locked")
	installEnd := strings.Index(source[installStart:], "\n}\n")
	if installStart < 0 || installEnd < 0 {
		t.Fatal("ASSERT_B4_POST_TRANSFER_FUNCTION_BOUNDARIES")
	}
	managerSource, err := os.ReadFile(strings.TrimSuffix(file, "b4_source_acquisition_private_test.go") + "sessionruntime.go")
	if err != nil {
		t.Fatal(err)
	}
	manager := string(managerSource)
	if !strings.Contains(manager, "privateB4Leases          [privateB4MaxSlots]privateB4Slot") ||
		strings.Contains(manager, "privateB4Leases: make(") {
		t.Fatal("ASSERT_B4_RESERVATION_STORAGE_FIXED_ARRAY")
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, productionPath, production, 0)
	if err != nil {
		t.Fatalf("ASSERT_B4_ADMISSION_AST_PARSE err=%v", err)
	}
	var admission *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "admitPrivateB4SourcesC17Locked" {
			admission = candidate
			break
		}
	}
	if admission == nil || admission.Body == nil {
		t.Fatal("ASSERT_B4_ADMISSION_FUNCTION_BOUNDARY")
	}
	callName := func(call *ast.CallExpr) string {
		switch function := call.Fun.(type) {
		case *ast.Ident:
			return function.Name
		case *ast.SelectorExpr:
			return function.Sel.Name
		default:
			return ""
		}
	}
	isTransferredStore := func(statement ast.Stmt) bool {
		expression, ok := statement.(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := expression.X.(*ast.CallExpr)
		if !ok || callName(call) != "Store" || len(call.Args) != 1 {
			return false
		}
		state, ok := call.Args[0].(*ast.Ident)
		return ok && state.Name == "privateB4SourceTransferred"
	}
	var sealPosition, transferPosition token.Pos
	finalizeCalls := 0
	ast.Inspect(admission.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch callName(call) {
		case "seal":
			sealPosition = call.Pos()
		case "Store":
			if len(call.Args) == 1 {
				if state, ok := call.Args[0].(*ast.Ident); ok && state.Name == "privateB4SourceTransferred" && transferPosition == token.NoPos {
					transferPosition = call.Pos()
				}
			}
		case "finalize":
			finalizeCalls++
		}
		return true
	})
	if transferPosition == token.NoPos {
		t.Fatal("ASSERT_B4_EXACT_TRANSFER_STATE_BOUNDARY")
	}
	if sealPosition == token.NoPos || sealPosition >= transferPosition {
		t.Fatal("ASSERT_C17_ACQUISITION_SEAL_PRECEDES_TRANSFER")
	}
	if finalizeCalls != 1 {
		t.Fatalf("ASSERT_C17_ACQUISITION_ONE_POST_TRANSFER_FINALIZER calls=%d", finalizeCalls)
	}
	transferStatement := -1
	for index, statement := range admission.Body.List {
		if statement.Pos() <= transferPosition && transferPosition <= statement.End() {
			transferStatement = index
			break
		}
	}
	if transferStatement < 0 || len(admission.Body.List)-transferStatement != 3 {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	transferLoop, ok := admission.Body.List[transferStatement].(*ast.RangeStmt)
	if !ok || transferLoop.Body == nil || len(transferLoop.Body.List) != 1 || !isTransferredStore(transferLoop.Body.List[0]) {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	finalizeStatement, ok := admission.Body.List[transferStatement+1].(*ast.ExprStmt)
	if !ok {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	finalizeCall, ok := finalizeStatement.X.(*ast.CallExpr)
	if !ok || callName(finalizeCall) != "finalize" || len(finalizeCall.Args) != 0 {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	terminalReturn, ok := admission.Body.List[transferStatement+2].(*ast.ReturnStmt)
	if !ok || len(terminalReturn.Results) != 1 {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	success, ok := terminalReturn.Results[0].(*ast.BasicLit)
	if !ok || success.Kind != token.STRING || success.Value != `""` {
		t.Fatal("ASSERT_B4_TRANSFER_HAS_NO_POST_START_REFUSAL_PATH")
	}
	install := source[installStart : installStart+installEnd+3]
	if strings.Contains(install, "privateB4Leases[reservation.token]") || !strings.Contains(install, "privateB4Leases[reservation.slot]") {
		t.Fatalf("ASSERT_B4_POST_TRANSFER_INSTALL_USES_PRESELECTED_SLOT body=%s", install)
	}
	for _, forbidden := range []string{"make(", "if ", "return"} {
		if strings.Contains(install, forbidden) {
			t.Fatalf("ASSERT_B4_POST_TRANSFER_INSTALL_INFALLIBLE forbidden=%q body=%s", forbidden, install)
		}
	}
}
```
