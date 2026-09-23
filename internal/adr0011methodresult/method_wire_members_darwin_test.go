//go:build darwin

package adr0011methodresult

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
)

// Parsed members are test-peer output, not admitted definition/reference edges.
func TestADR0011ManagedMethodWireMembers(t *testing.T) {
	for _, tc := range []struct {
		name, method, raw string
		kind              Kind
		malformed         bool
	}{
		{"D-04", transport.MethodDefinition, `[` + loc + `,` + loc + `]`, Location, false},
		{"D-05", transport.MethodDefinition, `[` + link + `,` + link + `]`, LocationLink, false},
		{"R-05", transport.MethodReferences, `[` + loc + `,` + loc + `]`, Location, false},
		{"D-07", transport.MethodDefinition, `[` + loc + `,` + malformedSecondLocation + `]`, Location, true},
		{"R-07", transport.MethodReferences, `[` + loc + `,` + malformedSecondLocation + `]`, Location, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, trace := startManagedMethodPeer(t, tc.name)
			req := fixtureRequest(tc.method, false)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			req.Deadline = time.Now().Add(5 * time.Second)
			wire := transport.New(manager).Execute(context.Background(), req)
			parsed, failed := Parse(wire, 3)
			obs, present := wire.Observation()
			if wire.Outcome() != transport.OutcomeTransportSuccess || !present || string(wire.Raw()) != tc.raw ||
				!obs.ReportedMethodAdvertised || obs.LocalWriteCorrespondence != transport.LocalWriteMatch ||
				obs.RawResultDisposition != transport.RawResultRetained {
				t.Fatalf("ASSERT_ADR0011_MANAGED_MEMBER_WIRE: case=%s outcome=%s present=%v local=%s rawLength=%d", tc.name, wire.Outcome(), present, obs.LocalWriteCorrespondence, len(wire.Raw()))
			}
			logged, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			wantLine := fmt.Sprintf("%s|sha256:%x", tc.method, sha256.Sum256(req.Params))
			if strings.TrimSpace(string(logged)) != wantLine {
				t.Fatalf("ASSERT_ADR0011_MANAGED_MEMBER_PEER: case=%s method/params digest mismatch", tc.name)
			}
			if tc.method == transport.MethodReferences && (!obs.IncludeDeclarationPresent || obs.DeclaredIncludeDeclaration) {
				t.Fatalf("ASSERT_ADR0011_MANAGED_MEMBER_CONTEXT: case=%s includeDeclaration not false", tc.name)
			}
			if tc.malformed {
				if failed == nil || failed.Code != FailureMalformed || failed.Ordinal != 1 || len(parsed.Items) != 0 || parsed.Null {
					ordinal := -1
					if failed != nil {
						ordinal = failed.Ordinal
					}
					t.Fatalf("ASSERT_ADR0011_MANAGED_MALFORMED_SECOND_NO_PARTIAL: case=%s failure=%v ordinal=%d items=%d null=%v", tc.name, failed, ordinal, len(parsed.Items), parsed.Null)
				}
				return
			}
			if failed != nil || parsed.Null || len(parsed.Items) != 2 {
				t.Fatalf("ASSERT_ADR0011_MANAGED_MEMBER_MULTIPLICITY: case=%s failure=%v items=%d null=%v", tc.name, failed, len(parsed.Items), parsed.Null)
			}
			for ordinal, item := range parsed.Items {
				if item.Ordinal != ordinal || item.Kind != tc.kind || item.URI != "file:///w/a.go" {
					t.Fatalf("ASSERT_ADR0011_MANAGED_MEMBER_ORDINAL: case=%s ordinal=%d item=%+v", tc.name, ordinal, item)
				}
				if tc.kind == LocationLink && (item.Range.Start != (Position{2, 1}) || item.TargetRange == nil || item.TargetRange.Start != (Position{1, 0}) || item.OriginSelectionRange == nil || item.OriginSelectionRange.Start != (Position{0, 1})) {
					t.Fatalf("ASSERT_ADR0011_MANAGED_LINK_RANGES: case=%s ordinal=%d item=%+v", tc.name, ordinal, item)
				}
			}
			if parsed.Items[0].Range != parsed.Items[1].Range || parsed.Items[0].Ordinal == parsed.Items[1].Ordinal {
				t.Fatalf("ASSERT_ADR0011_MANAGED_MEMBER_MULTIPLICITY: case=%s repeated range lost or ordinal collapsed", tc.name)
			}
		})
	}
}
