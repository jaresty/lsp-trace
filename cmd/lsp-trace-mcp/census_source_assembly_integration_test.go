package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"
)

func TestCensusRuntimeRunDelegatesSourceProgressionToAssembly(t *testing.T) {
	raw, err := os.ReadFile("census_completion.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCensusSourceAssemblyRun(raw); err != nil {
		t.Fatal(err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "census_completion.go", raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == "lsp-trace/internal/censussourcedriver" || path == "lsp-trace/internal/censussourcecontinuation" {
			t.Fatalf("ASSERT_COMMAND_RUNTIME_OBSOLETE_DIRECT_IMPORT path=%q", path)
		}
	}
}

func TestValidateCensusSourceAssemblyRunRejectsTextOutsideRequest(t *testing.T) {
	const misleading = `
package fixture

func run() {
	// Plan: prepared.sourcePlan, WorkflowKey: workflowKey, HeadKey: workflowKey
	// Objects: objects, Heads: heads, PartitionSize: censusRuntimeSourcePartitionSize
	// StepBound: 1, SessionID: prepared.admitted.sessionID
	// ManagerGeneration: prepared.admitted.generation
	_ = "CoordinateForTest: r.sourceCoordinate Discover: func"
	censussourceassembly.Run(ctx, censussourceassembly.Request{Plan: wrongPlan})
}
`
	if err := validateCensusSourceAssemblyRun([]byte(misleading)); err == nil {
		t.Fatal("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST accepted comments and dead text")
	}
}

func TestValidateCensusSourceAssemblyRunAcceptsExactRequest(t *testing.T) {
	const valid = `
package fixture

func run() {
	censussourceassembly.Run(discoveryCtx, censussourceassembly.Request{
		Plan: prepared.sourcePlan,
		WorkflowKey: workflowKey,
		HeadKey: workflowKey,
		Objects: objects,
		Heads: heads,
		PartitionSize: censusRuntimeSourcePartitionSize,
		StepBound: 1,
		Identity: censussourceassembly.Identity{
			SessionID: prepared.admitted.sessionID,
			ManagerGeneration: prepared.admitted.generation,
		},
		Discover: func() {},
		CoordinateForTest: r.sourceCoordinate,
	})
}
`
	if err := validateCensusSourceAssemblyRun([]byte(valid)); err != nil {
		t.Fatalf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST valid fixture: %v", err)
	}
}

func validateCensusSourceAssemblyRun(raw []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "census_completion.go", raw, 0)
	if err != nil {
		return err
	}

	var requests []*ast.CompositeLit
	calls := 0
	obsoleteDirectCalls := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isSelector(call.Fun, "censussourcedriver", "Step") {
			obsoleteDirectCalls++
		}
		if !isSelector(call.Fun, "censussourceassembly", "Run") {
			return true
		}
		calls++
		if len(call.Args) != 2 {
			return true
		}
		request, ok := call.Args[1].(*ast.CompositeLit)
		if ok && isSelector(request.Type, "censussourceassembly", "Request") {
			requests = append(requests, request)
		}
		return true
	})
	if obsoleteDirectCalls != 0 {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_NO_DIRECT_SOURCE_COMPOSITION calls=%d", obsoleteDirectCalls)
	}
	if calls != 1 {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_EXACTLY_ONE_ASSEMBLY_RUN calls=%d", calls)
	}
	if len(requests) != 1 {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST request_literals=%d", len(requests))
	}

	expected := map[string]string{
		"Plan":              "prepared.sourcePlan",
		"WorkflowKey":       "workflowKey",
		"HeadKey":           "workflowKey",
		"Objects":           "objects",
		"Heads":             "heads",
		"PartitionSize":     "censusRuntimeSourcePartitionSize",
		"StepBound":         "1",
		"CoordinateForTest": "r.sourceCoordinate",
	}
	fields, err := keyedFields(requests[0], expectedRequestFields(expected))
	if err != nil {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST: %w", err)
	}
	for name, want := range expected {
		if got := formatExpr(fields[name]); got != want {
			return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST field=%s got=%q want=%q", name, got, want)
		}
	}
	if _, ok := fields["Discover"].(*ast.FuncLit); !ok {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST field=Discover got=%T want=*ast.FuncLit", fields["Discover"])
	}
	identity, ok := fields["Identity"].(*ast.CompositeLit)
	if !ok || !isSelector(identity.Type, "censussourceassembly", "Identity") {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST field=Identity got=%q", formatExpr(fields["Identity"]))
	}
	identityExpected := map[string]string{
		"SessionID":         "prepared.admitted.sessionID",
		"ManagerGeneration": "prepared.admitted.generation",
	}
	identityFields, err := keyedFields(identity, expectedRequestFields(identityExpected))
	if err != nil {
		return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST field=Identity: %w", err)
	}
	for name, want := range identityExpected {
		if got := formatExpr(identityFields[name]); got != want {
			return fmt.Errorf("ASSERT_COMMAND_RUNTIME_ASSEMBLY_REQUEST field=Identity.%s got=%q want=%q", name, got, want)
		}
	}
	return nil
}

func expectedRequestFields(expressions map[string]string) map[string]struct{} {
	fields := make(map[string]struct{}, len(expressions)+2)
	for name := range expressions {
		fields[name] = struct{}{}
	}
	if _, ok := expressions["Plan"]; ok {
		fields["Identity"] = struct{}{}
		fields["Discover"] = struct{}{}
	}
	return fields
}

func keyedFields(lit *ast.CompositeLit, expected map[string]struct{}) (map[string]ast.Expr, error) {
	fields := make(map[string]ast.Expr, len(lit.Elts))
	for _, element := range lit.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("unkeyed binding %q", formatExpr(element))
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("non-identifier binding %q", formatExpr(pair.Key))
		}
		if _, ok := expected[key.Name]; !ok {
			return nil, fmt.Errorf("unexpected binding %q", key.Name)
		}
		if _, duplicate := fields[key.Name]; duplicate {
			return nil, fmt.Errorf("duplicate binding %q", key.Name)
		}
		fields[key.Name] = pair.Value
	}
	for name := range expected {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("missing binding %q", name)
		}
	}
	return fields, nil
}

func isSelector(expr ast.Expr, packageName, selectorName string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == packageName
}

func formatExpr(expr ast.Expr) string {
	if expr == nil {
		return "<nil>"
	}
	var formatted bytes.Buffer
	if err := format.Node(&formatted, token.NewFileSet(), expr); err != nil {
		return fmt.Sprintf("<format error: %v>", err)
	}
	return formatted.String()
}
