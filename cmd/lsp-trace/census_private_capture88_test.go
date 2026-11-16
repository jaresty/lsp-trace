package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
	"os"
	"testing"
)

func TestCapture88CLIClosedProjection(t *testing.T) {
	facts := sessionruntime.PrivateDiscoveryResponseFacts{Available: true, Count: 128}
	for i := range facts.Records {
		facts.Records[i].Ordinal = uint16(i + 1)
	}
	d, e := censusInventoryStoppedDiagnosticResponsesValue(censussourcedriver.StopRetained, nil, facts, true)
	if e != nil || d.PrivateDiscoveryResponses == nil || d.Retry || d.Code != censusresult.CodeDiscoveryRetained {
		t.Fatal("ASSERT_CAPTURE88_CLI_WIRING")
	}
	raw, e := censusresult.MarshalDiagnostic(d)
	if e != nil || len(raw) > 48000 || !bytes.Contains(raw, []byte(`"retained_total":128`)) || !bytes.Contains(raw, []byte(`"returned":2`)) || !bytes.Contains(raw, []byte(`"truncated":true`)) {
		t.Fatal("ASSERT_CAPTURE88_CLI_BOUND_COUNTS")
	}
	t.Logf("CLI structural bytes=%d retained_total=%d returned=%d omitted_retained=%d", len(raw), d.PrivateDiscoveryResponses.Total, d.PrivateDiscoveryResponses.Returned, d.PrivateDiscoveryResponses.Omitted)
	if !bytes.Contains(raw, []byte(`"omitted_retained":126`)) || !bytes.Contains(raw, []byte(`"retention_truncated":false`)) {
		t.Fatal("ASSERT_CAPTURE88_CLI_DISCLOSURE")
	}
	if _, e := censusresult.DecodeDiagnostic(raw); e != nil {
		t.Fatal("ASSERT_CAPTURE88_CLI_ROUNDTRIP")
	}
	d, e = censusInventoryStoppedDiagnostic(censussourcedriver.StopIndeterminateExecution, errors.New("PRIVATE_PAYLOAD_PATH_SENTINEL"))
	if e != nil || d.Retry || d.Code != censusresult.CodeDiscoveryIndeterminate {
		t.Fatal("ASSERT_CAPTURE88_CLI_PRIMARY")
	}
	raw, e = censusresult.MarshalDiagnostic(d)
	if e != nil || bytes.Contains(raw, []byte("PRIVATE_PAYLOAD_PATH_SENTINEL")) {
		t.Fatal("ASSERT_CAPTURE88_CLI_NO_ERROR_TEXT")
	}
	d, e = censusInventoryStoppedDiagnosticResponsesValue(censussourcedriver.StopRetained, nil, facts, true)
	if e != nil {
		t.Fatal("fixture")
	}
	ref := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoveryRead, Currency: sessionruntime.PrivateDiscoveryResponseBytes, Requested: 1, AmountsAvailable: true, CleanupUncertain: true}
	for _, stop := range []censussourcedriver.Stop{censussourcedriver.StopRetained, censussourcedriver.StopIndeterminateExecution} {
		value, err := censusInventoryStoppedDiagnosticResponsesValue(stop, &ref, facts, true)
		if err != nil || value.Retry || value.PrivateDiscoveryRefusal == nil || *value.PrivateDiscoveryRefusal != ref || value.PrivateDiscoveryResponses == nil {
			t.Fatal("ASSERT_CAPTURE88_CLI_FIRST_REFUSAL")
		}
	}
	ordinary, err := censusInventoryStoppedDiagnosticResponsesValue(censussourcedriver.StopFailed, nil, facts, true)
	if err != nil || ordinary.PrivateDiscoveryResponses != nil || !ordinary.Retry || ordinary.Code != censusresult.CodeInvalidConfig {
		t.Fatal("ASSERT_CAPTURE88_ORDINARY_FALLBACK_UNCHANGED")
	}
	d.Detail = "PRIVATE_PAYLOAD_PATH_SENTINEL"
	if _, e := censusresult.MarshalDiagnostic(d); e == nil {
		t.Fatal("ASSERT_CAPTURE88_CLI_INVALID_GUARD")
	}
}

// This compiling adapter preserves the same scenario and assertions across the
// old error-only API and its explicitly supplied closed-snapshot extension.
// It never constructs an operation, synthetic operation error, or native grant.
func stopped90WithSnapshot(stop censussourcedriver.Stop, cause error, facts sessionruntime.PrivateDiscoveryResponseFacts) (censusresult.Diagnostic, error) {
	switch f := any(censusInventoryStoppedDiagnostic).(type) {
	case func(censussourcedriver.Stop, error) (censusresult.Diagnostic, error):
		return f(stop, cause)
	case func(censussourcedriver.Stop, error, ...sessionruntime.PrivateDiscoveryResponseFacts) (censusresult.Diagnostic, error):
		return f(stop, cause, facts)
	default:
		panic("unsupported stopped diagnostic signature")
	}
}

func capture90Facts() sessionruntime.PrivateDiscoveryResponseFacts {
	f := sessionruntime.PrivateDiscoveryResponseFacts{Available: true, Count: 1}
	r := &f.Records[0]
	r.Ordinal, r.ReadAttempts, r.LastDisposition = 1, 1, sessionruntime.PrivateResponseSelected
	r.Selected = lspwire.ResponseFacts{State: lspwire.ResponseValidJSON, Envelope: lspwire.ResponseObject, Error: lspwire.ResponseObject, ID: lspwire.ResponseNumber, Association: lspwire.ResponseIDMatched, CodeState: lspwire.ResponseCodeValid}
	r.LastFrame = r.Selected
	r.Capture = sessionruntime.PrivateCaptureRecord{Status: sessionruntime.CaptureComplete, Artifact: 1, Bytes: 10, OriginalBytes: 10, BytesAvailable: true, DigestAvailable: true, PrefixSHA256: sha256.Sum256([]byte(`{"code":0}`))}
	return f
}

func TestReview89RetainedNilErrorLosesSummary(t *testing.T) {
	facts := capture90Facts() // neutral structural fixture, not proof of a file
	if !facts.Valid() {
		t.Fatal("REPAIR90_FIXTURE")
	}
	d, e := stopped90WithSnapshot(censussourcedriver.StopRetained, nil, facts)
	if e != nil || d.Code != censusresult.CodeDiscoveryRetained || d.Retry {
		t.Fatal("REVIEW89_STOP")
	}
	if d.PrivateDiscoveryResponses == nil {
		t.Fatal("REVIEW89_RETAINED_METADATA_MISSING")
	}
	if !d.PrivateDiscoveryResponses.Available || d.PrivateDiscoveryResponses.Returned != 1 || d.PrivateDiscoveryResponses.Records[0].Capture != facts.Records[0].Capture {
		t.Fatal("REPAIR90_CAPTURE_REFERENCE_LOST")
	}
}

func TestRepair90SnapshotAvailabilityAndOwnership(t *testing.T) {
	for _, stop := range []censussourcedriver.Stop{censussourcedriver.StopRetained, censussourcedriver.StopIndeterminateExecution} {
		for _, cause := range []error{nil, errors.New("PRIVATE_PAYLOAD_PATH_SENTINEL")} {
			facts := capture90Facts()
			d, e := stopped90WithSnapshot(stop, cause, facts)
			if e != nil || d.PrivateDiscoveryResponses == nil || d.Retry {
				t.Fatal("REPAIR90_STOP_SNAPSHOT")
			}
			want := censusresult.CodeDiscoveryRetained
			if stop == censussourcedriver.StopIndeterminateExecution {
				want = censusresult.CodeDiscoveryIndeterminate
			}
			if d.Code != want {
				t.Fatal("REPAIR90_STOP_CODE")
			}
			facts.Records[0].Capture = sessionruntime.PrivateCaptureRecord{}
			raw, e := censusresult.MarshalDiagnostic(d)
			if e != nil || len(raw) > 48000 || bytes.Contains(raw, []byte("PRIVATE_PAYLOAD_PATH_SENTINEL")) || d.PrivateDiscoveryResponses.Records[0].Capture.Artifact != 1 {
				t.Fatal("REPAIR90_CLOSED_OWNED_BOUND")
			}
			if _, e = censusresult.DecodeDiagnostic(raw); e != nil {
				t.Fatal("REPAIR90_ROUNDTRIP")
			}
		}
		old, e := censusInventoryStoppedDiagnostic(stop, nil)
		if e != nil || old.PrivateDiscoveryResponses != nil || old.Retry {
			t.Fatal("REPAIR90_NO_PHANTOM_FACTS")
		}
		unknown, e := stopped90WithSnapshot(stop, nil, sessionruntime.PrivateDiscoveryResponseFacts{})
		if e != nil || unknown.PrivateDiscoveryResponses != nil {
			t.Fatal("REPAIR90_UNKNOWN_IS_NOT_ZERO")
		}
		zero, e := stopped90WithSnapshot(stop, nil, sessionruntime.PrivateDiscoveryResponseFacts{Available: true})
		if e != nil || zero.PrivateDiscoveryResponses == nil || !zero.PrivateDiscoveryResponses.Available || zero.PrivateDiscoveryResponses.Total != 0 {
			t.Fatal("REPAIR90_AVAILABLE_ZERO")
		}
		bad := capture90Facts()
		bad.Records[0].Capture.Status = 99
		if _, e = stopped90WithSnapshot(stop, nil, bad); e == nil {
			t.Fatal("REPAIR90_INVALID_FACTS_REJECTED")
		}
	}
	facts := capture90Facts()
	ref := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoveryRead, Currency: sessionruntime.PrivateDiscoveryResponseBytes, Requested: 9, Remaining: 3, AmountsAvailable: true, CleanupUncertain: true}
	d, e := censusInventoryStoppedDiagnosticResponsesValue(censussourcedriver.StopRetained, &ref, facts, true)
	if e != nil || d.PrivateDiscoveryRefusal == nil || *d.PrivateDiscoveryRefusal != ref || d.PrivateDiscoveryResponses == nil || d.Retry {
		t.Fatal("REPAIR90_NUMERIC_REFUSAL_PRESERVED")
	}
	d, e = stopped90WithSnapshot(censussourcedriver.StopFailed, nil, facts)
	if e != nil || d.Code != censusresult.CodeInvalidConfig || d.PrivateDiscoveryResponses != nil || !d.Retry {
		t.Fatal("REPAIR90_CONFIG_FALLBACK")
	}
}

// Source structure, not native execution. Merely projecting the value seam is
// insufficient: the actual post-assembly caller and writer must carry it.
func TestRepair90BridgeSnapshotHandoff(t *testing.T) {
	fs := token.NewFileSet()
	bridge, e := parser.ParseFile(fs, "census_inventory_bridge_private.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	var assemblyEnd token.Pos
	var handoff, writer bool
	for _, decl := range bridge.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "runCensusInventoryBridgePrivate" {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := c.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				if id.Name == "runCensusPrivateInventoryPartition" {
					assemblyEnd = c.End()
				}
				if id.Name != "writeCensusInventoryDiscoveryFailure" || len(c.Args) != 5 {
					return true
				}
				get, ok := c.Args[4].(*ast.CallExpr)
				if !ok || len(get.Args) != 0 {
					return true
				}
				sel, ok := get.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "ResponseFacts" {
					return true
				}
				recv, ok := sel.X.(*ast.Ident)
				errSel, errOK := c.Args[2].(*ast.SelectorExpr)
				stopSel, stopOK := c.Args[3].(*ast.SelectorExpr)
				handoff = ok && recv.Name == "op" && assemblyEnd != 0 && c.Pos() > assemblyEnd && errOK && stopOK && errSel.Sel.Name == "Err" && stopSel.Sel.Name == "Stop"
				return true
			})
		}
		if fn.Name.Name == "writeCensusInventoryDiscoveryFailure" {
			last := fn.Type.Params.List[len(fn.Type.Params.List)-1]
			_, variadic := last.Type.(*ast.Ellipsis)
			if !variadic || len(last.Names) != 1 {
				continue
			}
			name := last.Names[0].Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := c.Fun.(*ast.Ident)
				if !ok || id.Name != "censusInventoryStoppedDiagnostic" || len(c.Args) != 3 || !c.Ellipsis.IsValid() {
					return true
				}
				arg, ok := c.Args[2].(*ast.Ident)
				writer = ok && arg.Name == name
				return true
			})
		}
	}
	if !handoff || !writer {
		t.Fatal("REPAIR90_ACTUAL_BRIDGE_SNAPSHOT_HANDOFF_MISSING")
	}
	// Keep the legacy production callsite guard compatible and strengthened.
	if _, e = os.Stat("census_inventory_stopped_diagnostic_test.go"); e != nil {
		t.Fatal(e)
	}
}
