package acquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/lsp"
	"reflect"
	"sort"
	"testing"
)

// Retained independent review oracle: all-pairs distance closure, not the
// coordinator's target queues or validation interpreter.
func TestIndependentThreeNodeOracle(t *testing.T) {
	items := []lsp.CallHierarchyItem{item("a", 0), item("b", 1), item("c", 2)}
	cases := 0
	for mask := 0; mask < 512; mask++ {
		var dist [3][3]int
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				dist[i][j] = 99
				if i == j {
					dist[i][j] = 0
				}
				if mask&(1<<(i*3+j)) != 0 && i != j {
					dist[i][j] = 1
				}
			}
		}
		for k := 0; k < 3; k++ {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					if dist[i][k]+dist[k][j] < dist[i][j] {
						dist[i][j] = dist[i][k] + dist[k][j]
					}
				}
			}
		}
		for _, mode := range []Mode{Slice, Incoming} {
			for limit := 0; limit < 4; limit++ {
				cases++
				r := request(items[0], items[1], items[0])
				r.RequiredTargets[1].ID = "alias"
				r.Mode = mode
				r.Root.DownDepth = limit
				r.Root.UpDepth = 3 - limit
				for i := range r.RequiredTargets {
					r.RequiredTargets[i].DownDepth = limit
					r.RequiredTargets[i].UpDepth = 3 - limit
				}
				actualCalls := map[string]int{}
				client := NewWireClient(func(_ context.Context, w WireRequest) (json.RawMessage, error) {
					if w.Method == "textDocument/prepareCallHierarchy" {
						var p lsp.PrepareCallHierarchyParams
						_ = json.Unmarshal(w.Params, &p)
						actualCalls[fmt.Sprintf("prepare:%d", p.Position.Line)]++
						return json.Marshal([]lsp.CallHierarchyItem{items[p.Position.Line]})
					}
					var p struct {
						Item lsp.CallHierarchyItem `json:"item"`
					}
					_ = json.Unmarshal(w.Params, &p)
					n := int(p.Item.Range.Start.Line)
					actualCalls[fmt.Sprintf("%s:%d", w.Method, n)]++
					if w.Method == "callHierarchy/outgoingCalls" {
						out := []lsp.CallHierarchyOutgoingCall{}
						for j := 0; j < 3; j++ {
							if mask&(1<<(n*3+j)) != 0 {
								out = append(out, lsp.CallHierarchyOutgoingCall{To: items[j], FromRanges: []lsp.Range{items[n].SelectionRange}})
							}
						}
						return json.Marshal(out)
					}
					out := []lsp.CallHierarchyIncomingCall{}
					for j := 0; j < 3; j++ {
						if mask&(1<<(j*3+n)) != 0 {
							out = append(out, lsp.CallHierarchyIncomingCall{From: items[j], FromRanges: []lsp.Range{items[j].SelectionRange}})
						}
					}
					return json.Marshal(out)
				})
				got, err := Acquire(context.Background(), client, r)
				if err != nil {
					t.Fatalf("mask=%d mode=%s depth=%d: %v", mask, mode, limit, err)
				}
				wantCalls := map[string]int{"prepare:0": 1, "prepare:1": 1}
				for ti, root := range []int{0, 1, 0} {
					expectedNodes := map[string]bool{node(items[root]).ID: true}
					expectedEdges := map[string]bool{}
					starts := []int{root}
					check := func(d DirectionResult, depths [3]int, lim int, outgoing bool) {
						expectedDepths := map[string]int{}
						for n, depth := range depths {
							if depth <= lim {
								expectedDepths[node(items[n]).ID] = depth
								if depth < lim {
									method := "callHierarchy/incomingCalls"
									if outgoing {
										method = "callHierarchy/outgoingCalls"
									}
									wantCalls[fmt.Sprintf("%s:%d", method, n)] = 1
									for j := 0; j < 3; j++ {
										a, b := j, n
										if outgoing {
											a, b = n, j
										}
										if mask&(1<<(a*3+b)) != 0 {
											expectedNodes[node(items[a]).ID] = true
											expectedNodes[node(items[b]).ID] = true
											expectedEdges[node(items[a]).ID+"->"+node(items[b]).ID] = true
										}
									}
								}
							}
						}
						actualDepths := map[string]int{}
						for _, e := range d.Expansions {
							actualDepths[e.NodeID] = e.Depth
						}
						if !reflect.DeepEqual(actualDepths, expectedDepths) {
							t.Fatalf("depth mask=%d mode=%s lim=%d target=%d outgoing=%v actual=%v expected=%v", mask, mode, limit, ti, outgoing, actualDepths, expectedDepths)
						}
					}
					if mode == Slice {
						check(got.Targets[ti].Outgoing, dist[root], limit, true)
						starts = nil
						for n, depth := range dist[root] {
							if depth > limit {
								continue
							}
							degree := 0
							for j := 0; j < 3; j++ {
								if mask&(1<<(n*3+j)) != 0 {
									degree++
								}
							}
							if depth == limit || degree == 0 {
								starts = append(starts, n)
							}
						}
					}
					depths := [3]int{99, 99, 99}
					for n := 0; n < 3; n++ {
						for _, s := range starts {
							if dist[n][s] < depths[n] {
								depths[n] = dist[n][s]
							}
						}
					}
					check(got.Targets[ti].Incoming, depths, 3-limit, false)
					for _, s := range got.Graph.Seeds {
						if s.Label != got.Targets[ti].Requested.ID {
							continue
						}
						if !reflect.DeepEqual(s.ReachedNodeIDs, keys(expectedNodes)) {
							t.Fatalf("node membership mask=%d target=%d", mask, ti)
						}
						actual := []string{}
						for _, id := range s.ReachedRelationIDs {
							for _, e := range got.Graph.Edges {
								if e.RelationID == id {
									actual = append(actual, e.CallerNodeID+"->"+e.CalleeNodeID)
								}
							}
						}
						sort.Strings(actual)
						if !reflect.DeepEqual(actual, keys(expectedEdges)) {
							t.Fatalf("edge membership mask=%d target=%d", mask, ti)
						}
					}
				}
				if !reflect.DeepEqual(actualCalls, wantCalls) {
					t.Fatalf("query sharing mask=%d mode=%s depth=%d: got %v want %v", mask, mode, limit, actualCalls, wantCalls)
				}
			}
		}
	}
	t.Logf("independent all-pairs oracle passed %d cases (512 directed graphs, loops included, 2 modes, 4 depth splits, 3 target rows with alias)", cases)
}

func TestIndependentTightCacheMemberships(t *testing.T) {
	a, b, c := item("a", 0), item("b", 1), item("c", 2)
	for _, mode := range []Mode{Slice, Incoming} {
		for _, nodes := range []int{2, 3} {
			f := fixture()
			for _, x := range []lsp.CallHierarchyItem{a, b, c} {
				f.add(x)
			}
			f.edge(a, b)
			f.edge(b, c)
			r := request(a, b, a)
			if mode == Incoming {
				r = request(c, b, c)
			}
			r.RequiredTargets[1].ID = "alias"
			r.Mode = mode
			r.Limits.MaxRequests = 4
			r.Limits.MaxNodes = nodes
			got := run(t, f, r)
			if len(f.calls) != 4 || got.Usage.Requests != 4 {
				t.Fatal("global four-request bound/cache reuse")
			}
			edgeCount := 1
			if nodes == 3 {
				edgeCount = 2
			}
			if len(got.Graph.Edges) != edgeCount {
				t.Fatalf("mode=%s nodes=%d retained edges=%d", mode, nodes, len(got.Graph.Edges))
			}
			for _, s := range got.Graph.Seeds {
				want := nodes
				if s.Label == "b" {
					want = nodes - 1
				}
				if len(s.ReachedNodeIDs) != want {
					t.Fatalf("mode=%s nodes=%d seed=%s got=%d want=%d", mode, nodes, s.Label, len(s.ReachedNodeIDs), want)
				}
			}
			if got.AcquisitionComplete || got.Graph.Summary.Complete {
				t.Fatal("partial acquisition promoted by canonical receipt")
			}
		}
	}
}
func TestIndependentBudgetBoundarySweep(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	count := 0
	for requests := 0; requests <= 8; requests++ {
		for nodes := 0; nodes <= 3; nodes++ {
			for _, bytes := range []int{0, 1, 80, 120, 512, 2048, 8192} {
				r := request(a, b, a)
				r.RequiredTargets[1].ID = "alias"
				r.Limits.MaxRequests = requests
				r.Limits.MaxNodes = nodes
				r.Limits.MaxEvidenceBytes = bytes
				r.Limits.MaxPathWork = 1
				f := fixtureForEdge(a, b)
				got := run(t, f, r)
				count++
				if got.Usage.Requests != len(f.calls) || got.Usage.Requests > requests || got.Usage.Nodes > nodes || got.Usage.EvidenceBytes > bytes || got.Usage.PathWork > 1 || len(got.Targets) != 3 {
					t.Fatalf("budget boundary r=%d n=%d bytes=%d", requests, nodes, bytes)
				}
				if !got.AcquisitionComplete && got.Graph.Summary.Complete {
					t.Fatal("native complete promoted")
				}
			}
		}
	}
	t.Logf("%d request/node/evidence boundary combinations passed", count)
}
