//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// Actual Manager/fake-pipe transport, owned unrelated source paths only. This
// proves the guard/query/capture connection, not a production scope freezer.
func TestSourceWithholdingPrivateV2ManagerConnection(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}, {5, 2}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			var workspace string
			members := []sessionruntime.RetainedScopeMemberPrivate{}
			restart, m, id, child, _, _ := adapterFixtureWithRestartPathsAndServerPrivate(t, "", func(root string) {
				workspace = root
				if err := os.MkdirAll(filepath.Join(root, "unrelated-v2"), 0700); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < counts[0]+counts[1]; i++ {
					path := filepath.Join(root, "unrelated-v2", fmt.Sprintf("owned-%02d.go", i))
					body := []byte(fmt.Sprintf("package owned\n// member %d\n", i))
					if err := os.WriteFile(path, body, 0600); err != nil {
						t.Fatal(err)
					}
					role := sessionruntime.RetainedScopeRolePrivate("CONTEXT_ONLY")
					if i < counts[0] {
						role = sessionruntime.RetainedScopeNominationPrivate
					}
					members = append(members, sessionruntime.RetainedScopeMemberPrivate{URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go", Expected: body, Role: role})
				}
				if err := os.WriteFile(filepath.Join(root, "unrelated-v2", "withheld.go"), []byte("never read"), 0600); err != nil {
					t.Fatal(err)
				}
			})
			outside := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "unrelated-v2", "withheld.go")}).String()
			child.mu.Lock()
			child.response = func(msg lspwire.Message) json.RawMessage {
				switch msg.Method {
				case "textDocument/prepareCallHierarchy":
					var p lsp.PrepareCallHierarchyParams
					_ = json.Unmarshal(msg.Params, &p)
					raw, _ := json.Marshal([]lsp.CallHierarchyItem{scopeAdapterItem(p.TextDocument.URI)})
					return raw
				case "callHierarchy/incomingCalls":
					rows := []lsp.CallHierarchyIncomingCall{{From: scopeAdapterItem(outside), FromRanges: []lsp.Range{}}}
					for _, member := range members {
						if member.Role == sessionruntime.RetainedScopeRolePrivate("CONTEXT_ONLY") {
							rows = append(rows, lsp.CallHierarchyIncomingCall{From: scopeAdapterItem(member.URI), FromRanges: []lsp.Range{}})
						}
					}
					raw, _ := json.Marshal(rows)
					return raw
				}
				return json.RawMessage(`[]`)
			}
			child.mu.Unlock()
			var bound *graphprovenance.BoundSourcePrivateV2
			var result acquisition.Result
			err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: restart.Scope().Selection().Workspace}, func(ctx context.Context) error {
				for _, member := range members {
					doc := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: id.SessionID, Generation: id.Generation, URI: member.URI, LanguageID: "go", CaptureSupply: true})
					if doc.Failure != "" || doc.Supply == nil {
						return fmt.Errorf("prepare: %+v", doc)
					}
				}
				guard, failure := m.BindRetainedScopePrivateV2(ctx, id.SessionID, id.Generation, members)
				if failure != "" {
					return fmt.Errorf("bind: %s", failure)
				}
				var err error
				bound, err = graphprovenance.BindSourcePrivateV2(ctx, m, guard, id.SessionID, id.Generation, workspace)
				if err != nil {
					return err
				}
				if _, err = graphprovenance.BindSourcePrivateV2(ctx, m, &sessionruntime.RetainedScopeGuardPrivateV2{}, id.SessionID, id.Generation, workspace); err == nil {
					t.Fatal("ASSERT_V2_FORGED_GUARD")
				}
				copiedGuard := *guard
				if _, err = graphprovenance.BindSourcePrivateV2(ctx, m, &copiedGuard, id.SessionID, id.Generation, workspace); err == nil {
					t.Fatal("ASSERT_V2_COPIED_GUARD")
				}
				if _, err = graphprovenance.BindSourcePrivateV2(ctx, &sessionruntime.Manager{}, guard, id.SessionID, id.Generation, workspace); err == nil {
					t.Fatal("ASSERT_V2_FOREIGN_MANAGER")
				}
				declaration, err := bound.FrozenQueryDeclarationPrivateV2(ctx)
				if err != nil {
					return err
				}
				want := []string{}
				for _, member := range members {
					want = append(want, member.URI)
				}
				if !reflect.DeepEqual(declaration.MembersPrivateV2(), want) {
					t.Fatal("ASSERT_V2_BOUND_QUERY_EXACT")
				}
				zero := uint32(0)
				target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: members[0].URI, LanguageID: "go", Line: &zero, Character: &zero}, UpDepth: 1}
				req := acquisition.Request{Mode: acquisition.Incoming, Context: acquisition.AcquisitionContext{ID: "owned-v2", SessionID: id.SessionID, Generation: id.Generation, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 64, MaxRequests: 64, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
				for i, member := range members[1:] {
					if member.Role != sessionruntime.RetainedScopeNominationPrivate {
						continue
					}
					extra := target
					extra.ID = fmt.Sprintf("member-%d", i)
					extra.Locator.URI = member.URI
					req.RequiredTargets = append(req.RequiredTargets, extra)
				}
				req, err = req.WithFrozenQueryDeclarationPrivateV2(declaration)
				if err != nil {
					return err
				}
				calls := 0
				client := acquisition.NewWireClient(func(c context.Context, w acquisition.WireRequest) (json.RawMessage, error) {
					calls++
					response := m.RoundTrip(c, sessionruntime.RoundTripRequest{SessionID: w.Context.SessionID, Generation: w.Context.Generation, RetainedScopePrivateV2: guard, Method: w.Method, Params: w.Params, Deadline: time.Now().Add(time.Second), MaxMessages: w.MaxMessages, MaxBytes: int64(w.MaxBytes)})
					if response.Failure != "" || response.ServerError != nil {
						return nil, fmt.Errorf("wire failure %s / %v", response.Failure, response.ServerError)
					}
					return response.Result, nil
				})
				result, err = acquisition.Acquire(ctx, client, req)
				if err != nil {
					return err
				}
				if calls != 2*counts[0] || 1+len(req.RequiredTargets) != counts[0] {
					t.Fatal("ASSERT_V2_ACTUAL_WIRE_BINDING", calls)
				}
				if counts[1] > 0 {
					t.Run("context-seed-refused", func(t *testing.T) {
						badReq := req
						badReq.RequiredTargets = nil
						badReq.Root.Locator.URI = members[len(members)-1].URI
						badResult, err := acquisition.Acquire(ctx, client, badReq)
						if err != nil {
							t.Fatal(err)
						}
						out, err := graphprovenance.CaptureSourceWithholdingPrivateV2(ctx, badResult, bound)
						if out != nil || err == nil || err.Error() != "private V2 source seed must be NOMINATION" {
							t.Fatal("ASSERT_V2_PUBLIC_CONTEXT_SEED", err)
						}
					})
				}
				raw, err := graphprovenance.CaptureSourceWithholdingPrivateV2(ctx, result, bound)
				if err != nil {
					return err
				}
				e, err := graphprovenance.ParseSourceWithholdingPrivateV2(raw)
				if err != nil {
					return err
				}
				if e.WithheldCount != 1 || len(e.Policy.Members) != len(members) || e.Evidence.CaptureBudget.Attempts != len(members) {
					t.Fatal("ASSERT_V2_VARIABLE_CAPTURE", e.WithheldCount, e.Evidence.CaptureBudget)
				}
				if _, err = graphprovenance.ParseSourceWithholdingPrivateV1(raw); err == nil {
					t.Fatal("ASSERT_V2_LEGACY_REFUSAL")
				}
				if err = graphprovenance.ValidateV2(e.Evidence); err == nil {
					t.Fatal("ASSERT_V2_ORDINARY_REFUSAL")
				}
				copied := *bound
				if out, err := graphprovenance.CaptureSourceWithholdingPrivateV2(ctx, result, &copied); err == nil || out != nil {
					t.Fatal("ASSERT_V2_COPIED_SOURCE")
				}
				if again, err := graphprovenance.CaptureSourceWithholdingPrivateV2(ctx, result, bound); err != nil || !bytes.Equal(raw, again) {
					t.Fatal("ASSERT_V2_ALIAS", err)
				}
				path, _ := url.Parse(members[0].URI)
				if err = os.WriteFile(path.Path, []byte("drift"), 0600); err != nil {
					return err
				}
				if out, err := graphprovenance.CaptureSourceWithholdingPrivateV2(ctx, result, bound); err == nil || out != nil {
					t.Fatal("ASSERT_V2_SOURCE_DRIFT")
				}
				if err = os.WriteFile(path.Path, members[0].Expected, 0600); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = bound.FrozenQueryDeclarationPrivateV2(context.Background()); err == nil {
				t.Fatal("ASSERT_V2_EXPIRED_QUERY_LEASE")
			}
			if out, err := graphprovenance.CaptureSourceWithholdingPrivateV2(context.Background(), result, bound); err == nil || out != nil {
				t.Fatal("ASSERT_V2_EXPIRED_CAPTURE_LEASE")
			}
		})
	}
}
