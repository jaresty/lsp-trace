package acquisitionops

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/operation"
	"strings"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type forbiddenRuntime struct{ calls int }

func (r *forbiddenRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	r.calls++
	return sessionruntime.SessionMetadata{}, session.SessionNotFound
}
func (r *forbiddenRuntime) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.calls++
	return sessionruntime.RoundTripResult{}
}
func (r *forbiddenRuntime) Records() []sessionruntime.Record { r.calls++; return nil }

const validManifest = `{"schema_version":"lsp-trace.seed-manifest.v2","coordinate_convention":"zero-based-session","root":{"id":"root","locator":{"uri":"file:///fixture/a.go","line":0,"character":0}},"required_targets":[]}`

func TestManifestClosedPreflight(t *testing.T) {
	for _, change := range []struct{ name, old, new string }{
		{"empty-symbol-with-position", `"character":0`, `"character":0,"symbol":""`},
		{"empty-language", `"character":0`, `"character":0,"language_id":""`},
		{"unknown-selector", `"character":0`, `"character":0,"name":"a"`},
		{"partial-position", `,"character":0`, ``},
		{"null-coordinate", `"character":0`, `"character":null`},
		{"duplicate-coordinate", `"character":0`, `"character":0,"character":1`},
		{"negative", `"character":0`, `"character":-1`},
		{"fraction", `"character":0`, `"character":0.5`},
		{"overflow", `"character":0`, `"character":4294967296`},
		{"URI", `file:///fixture/a.go`, `file:///fixture/../a.go`},
		{"version", `.v2`, `.v99`},
		{"root-id", `"id":"root"`, `"id":"primary"`},
	} {
		t.Run(change.name, func(t *testing.T) {
			raw := strings.Replace(validManifest, change.old, change.new, 1)
			for _, mode := range []string{"slice_v2", "incoming_v2"} {
				r := &forbiddenRuntime{}
				_, fail := NewExecutor(r).Execute(context.Background(), request(mode, `{"session_id":"s","generation":1,"seed_manifest":`+raw+`}`))
				if fail == nil || fail.Code != "INVALID_INPUT" || r.calls != 0 {
					t.Fatalf("ASSERT_PREFLIGHT_BEFORE_RUNTIME: failure=%v calls=%d", fail, r.calls)
				}
			}
		})
	}
}
func request(mode, raw string) operation.Request {
	return operation.Request{Name: operation.Name(mode), Input: json.RawMessage(raw)}
}

func TestManifestDefaultsAndBounds(t *testing.T) {
	m, err := DecodeManifest([]byte(validManifest), Slice)
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Request(Slice)
	if err != nil || r.Root.DownDepth != 2 || r.Root.UpDepth != 2 || r.Limits.MaxRequests != 1000 {
		t.Fatal("ASSERT_EXPLICIT_DEFAULTS")
	}
	zero := 0
	m.Root.DownDepth = &zero
	m.Root.UpDepth = &zero
	m.Limits.MaxNodes = &zero
	m.Limits.MaxRequests = &zero
	m.Limits.MaxEvidenceBytes = &zero
	m.Limits.MaxPathWork = &zero
	r, err = m.Request(Incoming)
	if err != nil || r.Root.DownDepth != 0 || r.Root.UpDepth != 0 || r.Limits.MaxRequests != 0 || r.Limits.MaxNodes != 0 || r.Limits.MaxPathWork != 0 || r.Limits.MaxEvidenceBytes != 0 {
		t.Fatal("ASSERT_ZERO_NOT_DEFAULT")
	}
	for i := 0; i < 63; i++ {
		target := m.Root
		target.ID = fmt.Sprintf("target-%d", i)
		m.RequiredTargets = append(m.RequiredTargets, target)
	}
	if _, err := m.Request(Slice); err != nil {
		t.Fatal("ASSERT_64_TOTAL_ACCEPTED", err)
	}
	m.RequiredTargets = append(m.RequiredTargets, m.RequiredTargets[0])
	if _, err := m.Request(Slice); err == nil {
		t.Fatal("ASSERT_65_TOTAL_REJECTED")
	}
	m.RequiredTargets = m.RequiredTargets[:63]
	m.RequiredTargets[1].ID = m.RequiredTargets[0].ID
	if _, err := m.Request(Slice); err == nil {
		t.Fatal("ASSERT_DUPLICATE_IDS_REJECTED")
	}
}
