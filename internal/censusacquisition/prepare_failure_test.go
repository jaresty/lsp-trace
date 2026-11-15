package censusacquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/census"
	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/lsp"
	"testing"
	"time"
)

type prepare66Client struct {
	err   error
	empty bool
}

func (prepare66Client) SupportsDocumentSymbols() bool { return true }
func (prepare66Client) SupportsCallHierarchy() bool   { return true }
func (prepare66Client) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	p := lsp.Position{Line: 1, Character: 1}
	r := lsp.Range{Start: p, End: lsp.Position{Line: 1, Character: 2}}
	return []lsp.DocumentSymbol{{Name: "Run", Kind: 12, Range: r, SelectionRange: r}}, nil
}
func (c prepare66Client) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return nil, c.err
}

type prepare66Supplier struct{}

func (prepare66Supplier) Supply(context.Context, SourceFile) error { return nil }

type prepare66Typed struct{ cause discoveryfailure.Cause }

func (prepare66Typed) Error() string                                 { panic("raw error must not be formatted") }
func (e prepare66Typed) PrepareFailureCause() discoveryfailure.Cause { return e.cause }

func TestPrepare66RetainedCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want discoveryfailure.Cause
		code string
	}{
		{"rpc", prepare66Typed{discoveryfailure.RPCServer}, discoveryfailure.RPCServer, "REQUEST_FAILED"},
		{"budget", prepare66Typed{discoveryfailure.EnclosureBudget}, discoveryfailure.EnclosureBudget, "REQUEST_FAILED"},
		{"cancel", context.Canceled, discoveryfailure.Canceled, "REQUEST_FAILED"},
		{"deadline", context.DeadlineExceeded, discoveryfailure.Deadline, "REQUEST_FAILED"},
		{"opaque", errors.New("secret://source/credential"), discoveryfailure.Unknown, "REQUEST_FAILED"},
		{"invalidtyped", prepare66Typed{"secret://source"}, discoveryfailure.Unknown, "REQUEST_FAILED"},
		{"empty", nil, discoveryfailure.Unknown, "UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const workspace = "/offline"
			file := SourceFile{Path: "a.go", URI: "file:///offline/a.go", LanguageID: "go"}
			d, e := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: prepare66Supplier{}, Client: prepare66Client{err: tc.err}}).Discover(context.Background(), SessionIdentity{"offline", 1})
			if e != nil || len(d.ProviderFailures) != 1 {
				t.Fatal("PREPARE_FAILURE_RETAINED")
			}
			f := d.ProviderFailures[0]
			if f.Code != tc.code || f.PrepareCause != tc.want {
				t.Fatalf("PREPARE_CAUSE_PROPAGATED: got=%q want=%q code=%s", f.PrepareCause, tc.want, f.Code)
			}
			raw, e := json.Marshal(d)
			if e != nil || bytes.Contains(raw, []byte("secret")) {
				t.Fatal("NO_RAW_PREPARE_PAYLOAD", e)
			}
			var decoded Discovery
			if json.Unmarshal(raw, &decoded) != nil {
				t.Fatal("CAUSE_DECODE")
			}
			if decoded.ProviderFailures[0].PrepareCause != tc.want {
				t.Fatal("CAUSE_ROUNDTRIP")
			}
			if d.Closure == DiscoveryClosureSuccessful || TerminalDiscoveryOutcome(d) != "UNKNOWN" {
				t.Fatal("FAILURE_NOT_EMPTY_OR_COMPLETE")
			}
			if e := ValidateTerminalDiscovery(decoded, "UNKNOWN"); e != nil {
				t.Fatal("RETAINED_METADATA_VALID", e)
			}
			bad := cloneDiscovery(decoded)
			bad.ProviderFailures[0].PrepareCause = discoveryfailure.RPCServer
			bad.ProviderFailures[0].Stage = "DOCUMENT_SYMBOL"
			if ValidateTerminalDiscovery(bad, "UNKNOWN") == nil {
				t.Fatal("PREPARE_CAUSE_SCOPE_VALIDATED")
			}
			if tc.want == discoveryfailure.Unknown && bytes.Contains(raw, []byte("PrepareCause")) {
				t.Fatal("LEGACY_UNKNOWN_OMITTED")
			}
		})
	}
}

// Switch to an already-terminal context only inside prepare. This exercises the
// real adapter cancellation branch without timers, scheduling races, or native work.
type prepare68BoundaryContext struct{ context.Context }
type prepare68BoundaryClient struct {
	prepare66Client
	boundary *prepare68BoundaryContext
	terminal context.Context
}

func (c prepare68BoundaryClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	c.boundary.Context = c.terminal
	return nil, c.err
}

func TestPrepare68CancellationCausePrecedence(t *testing.T) {
	for _, boundary := range []string{"canceled", "deadline"} {
		for _, tc := range []struct {
			name  string
			err   error
			known discoveryfailure.Cause
		}{
			{"budget", prepare66Typed{discoveryfailure.EnclosureBudget}, discoveryfailure.EnclosureBudget},
			{"rpc", prepare66Typed{discoveryfailure.RPCServer}, discoveryfailure.RPCServer},
			{"session-unavailable", prepare66Typed{discoveryfailure.SessionUnavailable}, discoveryfailure.SessionUnavailable},
			{"session-crashed", prepare66Typed{discoveryfailure.SessionCrashed}, discoveryfailure.SessionCrashed},
			{"invalid-response", prepare66Typed{discoveryfailure.InvalidResponse}, discoveryfailure.InvalidResponse},
			{"typed-canceled", prepare66Typed{discoveryfailure.Canceled}, discoveryfailure.Canceled},
			{"typed-deadline", prepare66Typed{discoveryfailure.Deadline}, discoveryfailure.Deadline},
			{"generic-canceled", context.Canceled, discoveryfailure.Canceled},
			{"generic-deadline", context.DeadlineExceeded, discoveryfailure.Deadline},
			{"opaque", errors.New("secret://source/credential"), discoveryfailure.Unknown},
			{"invalid-typed", prepare66Typed{"secret://source"}, discoveryfailure.Unknown},
			{"unknown-typed", prepare66Typed{discoveryfailure.Unknown}, discoveryfailure.Unknown},
			{"nil", nil, discoveryfailure.Unknown},
		} {
			t.Run(boundary+"/"+tc.name, func(t *testing.T) {
				terminal, cancel := context.WithCancel(context.Background())
				cancel()
				want := tc.known
				fallback := discoveryfailure.Canceled
				if boundary == "deadline" {
					// A deadline in the past expires synchronously; no timer is started.
					terminal, cancel = context.WithDeadline(context.Background(), time.Unix(0, 0))
					defer cancel()
					fallback = discoveryfailure.Deadline
				}
				if want == discoveryfailure.Unknown {
					want = fallback
				}
				ctx := &prepare68BoundaryContext{context.Background()}
				client := prepare68BoundaryClient{prepare66Client: prepare66Client{err: tc.err}, boundary: ctx, terminal: terminal}
				d, err := (DiscoveryAdapter{Workspace: "/offline", Files: StaticFiles{{Path: "a.go", URI: "file:///offline/a.go", LanguageID: "go"}}, Supplier: prepare66Supplier{}, Client: client}).Discover(ctx, SessionIdentity{"offline", 1})
				if err != nil || len(d.ProviderFailures) != 1 {
					t.Fatal("PREPARE68_BRANCH_REACHED")
				}
				f := d.ProviderFailures[0]
				if f.PrepareCause != want {
					t.Fatalf("CAUSE_PRECEDENCE_PRESERVED: got=%q want=%q", f.PrepareCause, want)
				}
				if f.Stage != "PREPARE_CALL_HIERARCHY" || f.Code != "CANCELED" || f.FileOrdinal != nil || f.SymbolOrdinal == nil || *f.SymbolOrdinal != 0 || d.Complete || d.Closure == DiscoveryClosureSuccessful || len(d.Targets) != 0 || TerminalDiscoveryOutcome(d) != "UNKNOWN" {
					t.Fatal("CANCELLATION_POLICY_PRESERVED")
				}
				if len(d.Accounting.Files) != 1 || d.Accounting.Files[0].Disposition != census.FileIncomplete || len(d.Accounting.Symbols) != 1 || d.Accounting.Symbols[0].Disposition != census.SymbolIncomplete || len(d.IncompleteCauses) != 2 {
					t.Fatal("CANCELLATION_DISPOSITIONS_PRESERVED")
				}
				for _, cause := range d.IncompleteCauses {
					if cause.Stage != "DISCOVERY" || cause.Code != "CANCELED" {
						t.Fatal("CANCELLATION_INCOMPLETE_CAUSES_PRESERVED")
					}
				}
				for _, obs := range d.Observations {
					switch obs.Dimension {
					case DiscoveryTargetSourceOmissions, DiscoveryProviderCompleteness:
						if !obs.Known || obs.Omitted == nil || *obs.Omitted != 1 {
							t.Fatal("CANCELLATION_OMISSIONS_PRESERVED")
						}
					case DiscoveryTimeoutRequestBudget:
						if obs.Known != (boundary == "deadline") {
							t.Fatal("CURRENT_BOUNDARY_TIMEOUT_POLICY_PRESERVED")
						}
					}
				}
				raw, err := json.Marshal(d)
				if err != nil || bytes.Contains(raw, []byte("secret")) {
					t.Fatal("NO_RAW_PREPARE68_PAYLOAD")
				}
				var decoded Discovery
				if json.Unmarshal(raw, &decoded) != nil || ValidateTerminalDiscovery(decoded, "UNKNOWN") != nil || decoded.ProviderFailures[0].PrepareCause != want {
					t.Fatal("PREPARE68_VALID_ROUNDTRIP")
				}
			})
		}
	}
}

func TestPrepare66LegacyBytesAndValidation(t *testing.T) {
	old := []byte(`{"Stage":"PREPARE_CALL_HIERARCHY","Code":"REQUEST_FAILED","FileOrdinal":null,"SymbolOrdinal":0}`)
	var f ProviderFailure
	if e := json.Unmarshal(old, &f); e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(f)
	if e != nil || !bytes.Equal(raw, old) || f.PrepareCause != discoveryfailure.Unknown {
		t.Fatal("OLD_FAILURE_BYTES_PRESERVED", e)
	}
	for _, label := range []string{"FUTURE", "secret://source", ""} {
		bad := bytes.Replace(old, []byte(`"Stage":`), []byte(`"PrepareCause":"`+label+`","Stage":`), 1)
		if json.Unmarshal(bad, &f) == nil {
			t.Fatal("UNKNOWN_LABEL_REJECTED")
		}
	}
	f.PrepareCause = "FUTURE"
	if _, e = json.Marshal(f); e == nil {
		t.Fatal("INVALID_ENUM_MARSHAL_REJECTED")
	}
}
