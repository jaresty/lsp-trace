//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestScopeFailureObservationPrivateV2(t *testing.T) {
	for _, kind := range []string{"unleased", "source", "member", "incomplete", "encoding", "success"} {
		t.Run(kind, func(t *testing.T) {
			spec, m, id, child := producerFixturePrivateV2(t, 2, 0)
			if kind == "incomplete" {
				spec.EffectivePolicy.Discovery.MaxSymbols = 1
			}
			f, e := FreezeScopePrivateV2(context.Background(), spec)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "source" {
				if e = os.WriteFile(filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath), []byte("SECRET_DIAGNOSTIC_CANARY_41b"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "encoding" {
				child.mu.Lock()
				child.response = func(msg lspwire.Message) json.RawMessage {
					if msg.Method != "textDocument/documentSymbol" {
						return json.RawMessage(`[]`)
					}
					raw, _ := json.Marshal([]lsp.DocumentSymbol{{Name: strings.Repeat("z", 80000), Kind: 12, Range: lsp.Range{End: lsp.Position{Character: 1}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}})
					return raw
				}
				child.mu.Unlock()
			}
			run := func(ctx context.Context) error {
				if kind == "member" {
					if _, err := newScopeRuntimeAdapterPrivateV2(ctx, f, m, id); err != nil {
						return err
					}
				}
				origin, r, err := DiscoverScopePrivateV2(ctx, f, m, id)
				d, ok := r.FailureObservationPrivateV2()
				if kind == "success" {
					if err != nil || !origin.valid() || ok || len(r.Bytes()) == 0 {
						t.Fatal("ASSERT_SUCCESS_DIAGNOSTIC_ABSENT")
					}
					var out map[string]json.RawMessage
					if json.Unmarshal(r.Bytes(), &out) != nil || out["failure"] != nil || out["Failure"] != nil {
						t.Fatal("ASSERT_SUCCESS_WIRE_UNCHANGED")
					}
					again, e := json.Marshal(out)
					if e != nil || len(again) == 0 {
						t.Fatal("success raw parse")
					}
				} else {
					stage, availability := "ADAPTER_ACTIVE_LEASE", "NOT_REACHED_DISCOVERY"
					switch kind {
					case "source":
						stage = "ADAPTER_SOURCE_REVALIDATION"
					case "member":
						stage = "ADAPTER_MEMBER_PREPARE"
					case "incomplete":
						stage = "DISCOVERY_ACCOUNTING"
						availability = "AVAILABLE_ERROR_REPORT"
					case "encoding":
						stage = "DISCOVERY_SERIALIZATION"
						availability = "DISCOVERY_ENCODING_BOUND"
					}
					if err == nil || origin.valid() || !ok || d.Stage != stage || d.DiscoveryAvailability != availability {
						t.Fatalf("ASSERT_DISCOVERY_FAILURE_BOUNDARY kind=%s observation=%+v has=%t", kind, d, ok)
					}
					if kind == "source" || kind == "encoding" {
						if err != ErrInvalid || !errors.Is(err, ErrInvalid) {
							t.Fatal("ASSERT_ORIGINAL_ERRINVALID_IDENTITY")
						}
					}
					if kind == "incomplete" && !errors.Is(err, ErrScopeDiscoveryIncompletePrivateV2) {
						t.Fatal("ASSERT_ORIGINAL_INCOMPLETE_IDENTITY")
					}
					if kind == "member" {
						if d.MemberOrdinal == nil || *d.MemberOrdinal != 0 {
							t.Fatal("ASSERT_KNOWN_MEMBER_ORDINAL")
						}
						*d.MemberOrdinal = 999
						again, _ := r.FailureObservationPrivateV2()
						if *again.MemberOrdinal != 0 {
							t.Fatal("ASSERT_DETACHED_DIAGNOSTIC_COPY")
						}
					}
					if (len(r.Bytes()) > 0) != (availability == "AVAILABLE_ERROR_REPORT") {
						t.Fatal("ASSERT_DISCOVERY_REPORT_AVAILABILITY")
					}
					raw, _ := json.Marshal(d)
					if bytes.Contains(raw, []byte(spec.Root)) || bytes.Contains(raw, []byte("file://")) || bytes.Contains(raw, []byte("SECRET_DIAGNOSTIC_CANARY_41b")) {
						t.Fatal("ASSERT_CONTENT_FREE_DIAGNOSTIC")
					}
				}
				before := producerSupplyAndQueryTrafficPrivateV2(child)
				_, second, twice := DiscoverScopePrivateV2(ctx, f, m, id)
				if twice != ErrInvalid || producerSupplyAndQueryTrafficPrivateV2(child) != before {
					t.Fatal("ASSERT_ONCE_ONLY_DISCOVERY_UNCHANGED")
				}
				sd, present := second.FailureObservationPrivateV2()
				if !present || sd.Stage != "DISCOVERY_ADMISSION" {
					t.Fatal("ASSERT_REPEAT_ADMISSION_DIAGNOSTIC")
				}
				return nil
			}
			if kind == "unleased" {
				if e = run(context.Background()); e != nil {
					t.Fatal(e)
				}
			} else {
				producerLeasePrivateV2(t, m, id, spec.Root, run)
			}
		})
	}
}

type scopeDiagnosticCyclePrivate struct{}

func (scopeDiagnosticCyclePrivate) Error() string   { panic("must not inspect raw messages") }
func (e scopeDiagnosticCyclePrivate) Unwrap() error { return e }
func TestScopeFailureClassPrivateV2Bounded(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{errors.New("SECRET_DIAGNOSTIC_CANARY_41b"), "UNKNOWN"},
		{scopeDiagnosticCyclePrivate{}, "CAUSE_TRAVERSAL_BOUND"},
		{errors.Join(errors.New("secret"), context.Canceled), "CANCELLED"},
		{scopeRuntimeFailurePrivate{Failure: session.Failure("SECRET_DIAGNOSTIC_CANARY_41b")}, "RUNTIME_FAILURE_UNKNOWN"},
	} {
		if got := ScopeFailureClassPrivateV2(tc.err); got != tc.want {
			t.Fatalf("ASSERT_CLOSED_FAILURE_CLASS got=%s want=%s", got, tc.want)
		}
	}
}
