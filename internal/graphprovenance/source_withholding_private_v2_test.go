//go:build darwin || linux

package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
)

func withholdingFixturePrivateV2(t *testing.T, nom, con int) (acquisition.Result, SourcePolicyPrivateV2, map[string][]byte, string) {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	root := t.TempDir()
	workspace := (&url.URL{Scheme: "file", Path: root}).String()
	p := SourcePolicyPrivateV2{SchemaVersion: VersionSourcePolicyPrivateV2, WorkspaceURI: workspace, SessionID: "owned-v2", Generation: 1, Members: []SourceMemberPrivateV2{}}
	expected := map[string][]byte{}
	for i := 0; i < nom+con; i++ {
		path := filepath.Join(root, fmt.Sprintf("member-%02d.go", i))
		body := []byte(fmt.Sprintf("package p\n// owned %d\n", i))
		if err = os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		uri := (&url.URL{Scheme: "file", Path: path}).String()
		role := "CONTEXT_ONLY"
		if i < nom {
			role = "NOMINATION"
		}
		p.Members = append(p.Members, SourceMemberPrivateV2{uri, sourceHashPrivateV1(body), role})
		expected[uri] = body
	}
	forbidden := filepath.Join(root, "forbidden.go")
	if err = os.WriteFile(forbidden, []byte("never read"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := (&url.URL{Scheme: "file", Path: forbidden}).String()
	zero := uint32(0)
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: p.Members[0].URI, Line: &zero, Character: &zero}, UpDepth: 1}
	req := acquisition.Request{Mode: acquisition.Incoming, Context: acquisition.AcquisitionContext{ID: "v2", SessionID: p.SessionID, Generation: 1, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 64, MaxRequests: 64, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	for i, m := range p.Members[1:] {
		if m.Role != "NOMINATION" {
			continue
		}
		extra := target
		extra.ID = fmt.Sprintf("target-%d", i)
		extra.Locator.URI = m.URI
		req.RequiredTargets = append(req.RequiredTargets, extra)
	}
	d, err := acquisition.NewFrozenQueryDeclarationPrivateV2(p.queryMembers())
	if err != nil {
		t.Fatal(err)
	}
	req, err = req.WithFrozenQueryDeclarationPrivateV2(d)
	if err != nil {
		t.Fatal(err)
	}
	item := func(uri string) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: "Owned", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Character: 1}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}
	}
	client := acquisition.NewWireClient(func(_ context.Context, w acquisition.WireRequest) (json.RawMessage, error) {
		if w.Method == "textDocument/prepareCallHierarchy" {
			var params lsp.PrepareCallHierarchyParams
			_ = json.Unmarshal(w.Params, &params)
			return json.Marshal([]lsp.CallHierarchyItem{item(params.TextDocument.URI)})
		}
		rows := []lsp.CallHierarchyIncomingCall{{From: item(outside), FromRanges: []lsp.Range{}}, {From: item("file:///outside-owned-v2.go"), FromRanges: []lsp.Range{}}}
		for _, m := range p.Members {
			if m.Role == "CONTEXT_ONLY" {
				rows = append(rows, lsp.CallHierarchyIncomingCall{From: item(m.URI), FromRanges: []lsp.Range{}})
			}
		}
		return json.Marshal(rows)
	})
	r, err := acquisition.Acquire(context.Background(), client, req)
	if err != nil {
		t.Fatal(err)
	}
	return r, p, expected, root
}
func pHashPrivateV2(p SourcePolicyPrivateV2, uri string) string {
	for _, m := range p.Members {
		if m.URI == uri {
			return m.SHA256
		}
	}
	return ""
}
func TestSourceWithholdingPrivateV2Capture(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}, {5, 2}, {8, 8}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			r, p, expected, _ := withholdingFixturePrivateV2(t, counts[0], counts[1])
			reads := map[string]int{}
			raw, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, func(c context.Context, root *sourceRootPrivateV1, name string, n int64) ([]byte, error) {
				reads[name]++
				return readSourcePrivateV1(c, root, name, n)
			})
			if err != nil {
				t.Fatal(err)
			}
			e, err := ParseSourceWithholdingPrivateV2(raw)
			if err != nil {
				t.Fatal(err)
			}
			if reads["forbidden.go"] != 0 || len(reads) != len(p.Members) || e.WithheldCount != 1 || len(e.Evidence.Acquisition.Graph.Edges) != counts[0]*(2+counts[1]) {
				t.Fatal("ASSERT_V2_WITHHELD_ZERO_IO", reads, e.WithheldCount)
			}
			for _, row := range e.Evidence.Captures {
				if row.URI == "file:///outside-owned-v2.go" && (row.Status != "OUTSIDE_SCOPE" || row.Content != nil) {
					t.Fatal("ASSERT_V2_OUTSIDE_WORKSPACE")
				}
				if row.Status == "READABLE" && pHashPrivateV2(p, row.URI) != sourceHashPrivateV1(row.Content) {
					t.Fatal("ASSERT_V2_READABLE_HASH")
				}
			}
			if _, err = ParseSourceWithholdingPrivateV1(raw); err == nil {
				t.Fatal("ASSERT_V1_REFUSES_V2")
			}
			if err = ValidateV2(e.Evidence); err == nil {
				t.Fatal("ASSERT_ORDINARY_REFUSES_V2")
			}
			if _, err = ValidateFor(raw, Family, "v2"); err == nil {
				t.Fatal("ASSERT_SCHEMA_REFUSES_V2")
			}
			if !e.FrozenQueryScope || e.Accepted || e.Authority != 0 || e.ReplayAdmitted || !e.Partial {
				t.Fatal("ASSERT_V2_CEILING")
			}
			if raw1, err := captureSourceWithholdingPrivateV1(context.Background(), r, SourcePolicyPrivateV1{}, expected, readSourcePrivateV1); err == nil || raw1 != nil {
				t.Fatal("ASSERT_V1_NO_RELABEL")
			}
		})
	}
}
func TestSourceWithholdingPrivateV2ClosedWire(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV2(t, 2, 1)
	raw, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*SourceWithholdingPrivateV2){
		"query-required": func(e *SourceWithholdingPrivateV2) { e.FrozenQueryScope = false },
		"native-forgery": func(e *SourceWithholdingPrivateV2) { e.Evidence.GraphBytes = []byte(`{"full_definition":true}`) },
		"capture-budget": func(e *SourceWithholdingPrivateV2) { e.Evidence.CaptureBudget.Attempts++ },
		"hash-case": func(e *SourceWithholdingPrivateV2) {
			e.Policy.Members[0].SHA256 = strings.ToUpper(e.Policy.Members[0].SHA256)
		},
		"member-alias": func(e *SourceWithholdingPrivateV2) { e.Policy.Members[0].URI += "?alias" },
		"source-claim": func(e *SourceWithholdingPrivateV2) {
			for i := range e.Evidence.Captures {
				if e.Evidence.Captures[i].Status == PolicyWithheldPrivateV2 {
					e.Evidence.Captures[i].CanonicalReceipt = []byte(`{}`)
				}
			}
		},
		"count": func(e *SourceWithholdingPrivateV2) { e.WithheldCount++ },
		"hash":  func(e *SourceWithholdingPrivateV2) { e.Policy.Members[0].SHA256 = sourceHashPrivateV1([]byte("drift")) },
		"nom-zero": func(e *SourceWithholdingPrivateV2) {
			for i := range e.Policy.Members {
				e.Policy.Members[i].Role = "CONTEXT_ONLY"
			}
		},
		"nil":       func(e *SourceWithholdingPrivateV2) { e.Policy.Members = nil },
		"empty":     func(e *SourceWithholdingPrivateV2) { e.Policy.Members = []SourceMemberPrivateV2{} },
		"authority": func(e *SourceWithholdingPrivateV2) { e.Authority = 1 },
		"session":   func(e *SourceWithholdingPrivateV2) { e.Policy.SessionID = "wrong" },
		"withheld-source": func(e *SourceWithholdingPrivateV2) {
			for i := range e.Evidence.Captures {
				if e.Evidence.Captures[i].Status == PolicyWithheldPrivateV2 {
					e.Evidence.Captures[i].Content = []byte{}
				}
			}
		},
		"digest-v1": func(e *SourceWithholdingPrivateV2) {
			for i := range e.Evidence.Captures {
				row := &e.Evidence.Captures[i]
				if row.Status == PolicyWithheldPrivateV2 {
					*row = withheldPrivateV1(e.Evidence.WorkspaceURI, row.URI)
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			e, err := ParseSourceWithholdingPrivateV2(raw)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&e)
			bad, _ := json.Marshal(e)
			if _, err = ParseSourceWithholdingPrivateV2(bad); err == nil {
				t.Fatal("ASSERT_V2_WIRE_REFUSAL", name)
			}
		})
	}
	for _, tc := range [][2]string{{`"members":`, `"Members":`}, {`"source_policy":{`, `"source_policy":{"unknown":[],`}, {`"members":[{`, `"members":[{"URI":"x",`}, {`"authority":0`, `"authority":0,"authority":0`}, {`"frozen_query_scope":true`, `"frozen_query_scope":null`}} {
		bad := bytes.Replace(raw, []byte(tc[0]), []byte(tc[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("mutation absent")
		}
		if _, err = ParseSourceWithholdingPrivateV2(bad); err == nil {
			t.Fatal("ASSERT_V2_KEY_REFUSAL", tc)
		}
	}
	if _, err = ParseSourceWithholdingPrivateV2(bytes.Repeat([]byte(" "), MaxSourceWithholdingPrivateV2+1)); err == nil {
		t.Fatal("ASSERT_V2_4M_CEILING")
	}
	if err = preflightSourceArraysPrivate([]byte(`{"source_policy":{"members":[`+strings.Repeat(`{},`, 16)+`{}]}}`), 16); err == nil {
		t.Fatal("ASSERT_V2_MEMBER_PREFLIGHT")
	}
}
func TestSourceWithholdingPrivateV2NoV1RelabelWithoutFrontier(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	var p SourcePolicyPrivateV1
	expected := map[string][]byte{}
	r, _ := coordinatorV2Fixture(t, func(req *acquisition.Request) {
		u, _ := url.Parse(req.Root.Locator.URI)
		root := filepath.Dir(u.Path)
		p = SourcePolicyPrivateV1{SchemaVersion: VersionSourcePolicyPrivateV1, WorkspaceURI: (&url.URL{Scheme: "file", Path: root}).String(), SessionID: req.Context.SessionID, Generation: req.Context.Generation}
		members := []string{}
		for i, name := range []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go"} {
			body := []byte("package p\n")
			path := filepath.Join(root, name)
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			role := "CONTEXT_ONLY"
			if i < 4 {
				role = "NOMINATION"
			}
			p.Members = append(p.Members, SourceMemberPrivateV1{uri, sourceHashPrivateV1(body), role})
			expected[uri] = body
			members = append(members, uri)
		}
		d, err := acquisition.NewFrozenQueryDeclarationPrivateV2(members)
		if err != nil {
			t.Fatal(err)
		}
		*req, err = req.WithFrozenQueryDeclarationPrivateV2(d)
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(r.Graph.Frontier) != 0 {
		t.Fatal("fixture must have no external frontier")
	}
	reads := 0
	out, err := captureSourceWithholdingPrivateV1(context.Background(), r, p, expected, func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) { reads++; return nil, nil })
	if err == nil || err.Error() != "private V1 source refuses V2 query metadata" || out != nil || reads != 0 {
		t.Fatal("ASSERT_V1_NO_FRONTIER_RELABEL", err, reads)
	}
}

func TestSourceWithholdingPrivateV2ReadFaults(t *testing.T) {
	for _, kind := range []string{"close", "cancel", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			r, p, expected, root := withholdingFixturePrivateV2(t, 2, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "symlink" {
				path, _ := url.Parse(p.Members[0].URI)
				if err := os.Remove(path.Path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "forbidden.go"), path.Path); err != nil {
					t.Fatal(err)
				}
			}
			reads := 0
			raw, err := captureSourceWithholdingPrivateV2(ctx, r, p, expected, func(c context.Context, root *sourceRootPrivateV1, name string, n int64) ([]byte, error) {
				root.ops.read = func(f *os.File, b []byte) (int, error) {
					if f.Name() == "forbidden.go" {
						t.Fatal("ASSERT_V2_FORBIDDEN_FD")
					}
					reads++
					return f.Read(b)
				}
				body, err := readSourcePrivateV1(c, root, name, n)
				if kind == "close" {
					root.ops.close = func(f *os.File) error { return errors.Join(f.Close(), errors.New("owned injected close failure")) }
				}
				if kind == "cancel" {
					cancel()
				}
				return body, err
			})
			if kind == "cancel" {
				if raw != nil || !errors.Is(err, context.Canceled) {
					t.Fatal("ASSERT_V2_CANCEL_NO_OUTPUT", err)
				}
				return
			}
			if kind == "close" {
				if err == nil || raw != nil {
					t.Fatal("ASSERT_V2_CLOSE_NO_PUBLICATION")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			e, err := ParseSourceWithholdingPrivateV2(raw)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				found := false
				for _, row := range e.Evidence.Captures {
					if row.URI == p.Members[0].URI {
						found = true
						if row.Status == "READABLE" || row.Content != nil {
							t.Fatal("ASSERT_V2_SYMLINK_ZERO_READ")
						}
					}
				}
				if !found || reads == 0 {
					t.Fatal("ASSERT_V2_SYMLINK_ROWS")
				}
			}
		})
	}
}

func TestSourceWithholdingPrivateV2PolicyBounds(t *testing.T) {
	_, p, _, _ := withholdingFixturePrivateV2(t, 8, 8)
	for _, kind := range []string{"zero", "seventeen", "nom9", "ctx9", "session", "uri", "hash"} {
		t.Run(kind, func(t *testing.T) {
			q := p
			q.Members = append([]SourceMemberPrivateV2(nil), p.Members...)
			switch kind {
			case "zero":
				q.Members = nil
			case "seventeen":
				q.Members = append(q.Members, q.Members[15])
			case "nom9":
				q.Members[8].Role = "NOMINATION"
			case "ctx9":
				q.Members[7].Role = "CONTEXT_ONLY"
			case "session":
				q.SessionID = strings.Repeat("x", 1025)
			case "uri":
				q.Members[0].URI = strings.Repeat("x", 8193)
			case "hash":
				q.Members[0].SHA256 = "sha256:" + strings.Repeat("A", 64)
			}
			if q.validate() == nil {
				t.Fatal("ASSERT_V2_POLICY_BOUNDS", kind)
			}
		})
	}
	r, v1, expected, _ := withholdingFixturePrivateV1(t)
	raw, err := captureSourceWithholdingPrivateV1(context.Background(), r, v1, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseSourceWithholdingPrivateV2(raw); err == nil {
		t.Fatal("ASSERT_V2_REFUSES_FIXED_V1")
	}
	for _, field := range []string{"uri", "workspace_uri", "session_id"} {
		limit := 8192
		if field == "session_id" {
			limit = 1024
		}
		raw := []byte(fmt.Sprintf(`{"source_policy":{"members":[{"%s":%q}]}}`, field, strings.Repeat("x", limit+1)))
		if preflightSourceArraysPrivate(raw, 16) == nil {
			t.Fatal("ASSERT_V2_EARLY_METADATA_BOUND", field)
		}
	}
}

func TestSourceWithholdingPrivateV2QueryRefusal(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV2(t, 1, 0)
	raw, _ := json.Marshal(r)
	var fresh acquisition.Result
	if err := json.Unmarshal(raw, &fresh); err != nil {
		t.Fatal(err)
	}
	reads := 0
	reader := func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) { reads++; return nil, nil }
	if out, err := captureSourceWithholdingPrivateV2(context.Background(), fresh, p, expected, reader); err == nil || out != nil || reads != 0 {
		t.Fatal("ASSERT_V2_NIL_QUERY_ZERO_IO")
	}
	d, err := acquisition.NewFrozenQueryDeclarationPrivateV2(append(p.queryMembers(), p.WorkspaceURI+"/z.go"))
	if err != nil {
		t.Fatal(err)
	}
	r.Request, err = r.Request.WithFrozenQueryDeclarationPrivateV2(d)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, reader); err == nil || out != nil || reads != 0 {
		t.Fatal("ASSERT_V2_CHANGED_QUERY_ZERO_IO")
	}
}
