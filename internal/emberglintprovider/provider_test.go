package emberglintprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type analyzerStub struct {
	kinds  []RelationKind
	result AnalysisResult
	err    error
	calls  int
}

func (a *analyzerStub) QualifiedRelationKinds() []RelationKind {
	return append([]RelationKind(nil), a.kinds...)
}
func (a *analyzerStub) Analyze(context.Context, AnalysisRequest) (AnalysisResult, error) {
	a.calls++
	return a.result, a.err
}

type custodyStub struct {
	docs  []Document
	err   error
	calls int
}

func (c *custodyStub) Resolve(context.Context, CustodyRequest) ([]Document, error) {
	c.calls++
	return append([]Document(nil), c.docs...), c.err
}

func request(relations ...string) []byte {
	v := map[string]any{
		"schema_version": RequestSchema, "provider_id": ProviderID, "adapter_id": "qualified-adapter@1",
		"session":          map[string]any{"session_id": "s", "generation": 1},
		"seed":             map[string]any{"uri": "file:///workspace/app.gts", "line": 0, "character": 0},
		"relations":        relations,
		"document_custody": map[string]any{"original_uri": "file:///workspace/app.gts", "workspace_revision": "rev-1", "fail_on_unknown_revision": true},
		"limits":           map[string]any{"max_nodes": 10, "max_messages": 1, "max_bytes": 4096, "timeout_ms": 1000, "request_timeout_ms": 1000},
	}
	b, _ := json.Marshal(v)
	return b
}
func frame(body []byte) []byte {
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}
func responseBody(t *testing.T, b []byte) []byte {
	t.Helper()
	parts := bytes.SplitN(b, []byte("\r\n\r\n"), 2)
	if len(parts) != 2 {
		t.Fatalf("response is not one Content-Length frame: %q", b)
	}
	var n int
	if _, err := fmt.Sscanf(string(parts[0]), "Content-Length: %d", &n); err != nil || n != len(parts[1]) {
		t.Fatalf("bad response length: %q", parts[0])
	}
	return parts[1]
}
func run(t *testing.T, p Provider, input []byte) (Envelope, []byte, error) {
	t.Helper()
	var out bytes.Buffer
	err := p.Serve(context.Background(), bytes.NewReader(input), &out)
	if err != nil {
		return Envelope{}, out.Bytes(), err
	}
	body := responseBody(t, out.Bytes())
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	return env, out.Bytes(), nil
}

func TestServeEmptyIsDeterministicAndOrchestratesNarrowInterfaces(t *testing.T) {
	doc := Document{"document_id": "doc", "uri": "file:///workspace/app.gts", "revision": "rev-1", "blob": "sha256:x", "range": map[string]any{}}
	a := &analyzerStub{kinds: []RelationKind{RendersFrom}, result: AnalysisResult{Status: StatusEmpty}}
	c := &custodyStub{docs: []Document{doc}}
	p := Provider{Analyzer: a, Custody: c, MaxRequestBytes: 8192, MaxResponseBytes: 8192}
	first, b1, err := run(t, p, frame(request(string(RendersFrom))))
	if err != nil {
		t.Fatal(err)
	}
	_, b2, err := run(t, p, frame(request(string(RendersFrom))))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatal("canonical response bytes changed for identical input")
	}
	if first.Provider.Name != "ember-glint" || first.Provider.Version != "1" || first.Protocol.Name != "lsp-trace.provider-observations" || first.Protocol.Version != "1" {
		t.Fatalf("identity mismatch: %+v", first)
	}
	if first.Authority != "PROVIDER_REPORTED" || first.Coverage.Status != StatusEmpty || len(first.Observations) != 0 {
		t.Fatalf("unexpected empty envelope: %+v", first)
	}
	if a.calls != 2 || c.calls != 2 {
		t.Fatalf("interfaces not orchestrated: analyzer=%d custody=%d", a.calls, c.calls)
	}
}

func TestServeRejectsIdentityBoundsAndExtraFrames(t *testing.T) {
	p := Provider{Analyzer: &analyzerStub{}, Custody: &custodyStub{}, MaxRequestBytes: 8192, MaxResponseBytes: 8192}
	badID := bytes.Replace(request("RENDERS_FROM"), []byte(ProviderID), []byte("other@1"), 1)
	for name, tc := range map[string]struct {
		input []byte
		want  string
	}{
		"identity":       {frame(badID), "provider identity"},
		"two frames":     {append(frame(request("RENDERS_FROM")), frame(request("RENDERS_FROM"))...), "exactly one"},
		"declared bytes": {frame(bytes.Replace(request("RENDERS_FROM"), []byte(`"max_bytes":4096`), []byte(`"max_bytes":1`), 1)), "max_bytes"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := run(t, p, tc.input)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
		})
	}
}

func TestServeDistinguishesOutcomesAndAdvertisesQualifiedKindsOnly(t *testing.T) {
	doc := Document{"document_id": "doc", "uri": "file:///workspace/app.gts", "revision": "rev-1", "blob": "sha256:x", "range": map[string]any{}}
	for _, tc := range []struct {
		name       string
		kinds      []RelationKind
		result     AnalysisResult
		analyzeErr error
		custodyErr error
		relation   string
		want       Status
	}{
		{"unsupported", nil, AnalysisResult{}, nil, nil, "RENDERS_FROM", StatusUnsupported},
		{"unavailable", []RelationKind{RendersFrom}, AnalysisResult{}, nil, ErrUnavailable, "RENDERS_FROM", StatusUnavailable},
		{"partial", []RelationKind{RendersFrom}, AnalysisResult{Status: StatusPartial, Observations: []Observation{{Kind: RendersFrom, From: map[string]any{"symbol": "source"}, To: map[string]any{"symbol": "target"}, OriginalAnchor: Anchor{DocumentID: "doc", URI: "file:///workspace/app.gts", Revision: "rev-1", Blob: "sha256:x", Range: map[string]any{}}, Supports: []any{"bounded static evidence"}, DoesNotSupport: []any{"runtime execution"}}}}, nil, nil, "RENDERS_FROM", StatusPartial},
		{"empty", []RelationKind{RendersFrom}, AnalysisResult{Status: StatusEmpty}, nil, nil, "RENDERS_FROM", StatusEmpty},
		{"failed", []RelationKind{RendersFrom}, AnalysisResult{}, errors.New("analyzer failed"), nil, "RENDERS_FROM", StatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &analyzerStub{kinds: tc.kinds, result: tc.result, err: tc.analyzeErr}
			c := &custodyStub{docs: []Document{doc}, err: tc.custodyErr}
			env, _, err := run(t, Provider{Analyzer: a, Custody: c, MaxRequestBytes: 8192, MaxResponseBytes: 8192}, frame(request(tc.relation)))
			if err != nil {
				t.Fatal(err)
			}
			if env.Coverage.Status != tc.want {
				t.Fatalf("status=%q want %q", env.Coverage.Status, tc.want)
			}
			if fmt.Sprint(env.Coverage.QualifiedRelations) != fmt.Sprint(tc.kinds) {
				t.Fatalf("advertised=%v qualified=%v", env.Coverage.QualifiedRelations, tc.kinds)
			}
		})
	}
}
