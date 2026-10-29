//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

func producerPolicyPrivateV2() ScopeEffectivePolicyPrivateV2 {
	return ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 8, MaxContextFiles: 8, MaxMembers: 16, FileBytes: 2 << 20, TotalBytes: 8 << 20, MaxPathDepth: 32, Discovery: censusacquisition.DiscoveryLimits{MaxFiles: 8, MaxSymbols: 256, MaxDepth: 64, MaxStringBytes: 1 << 20}, MaxTargets: 63, Planning: censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 2}, MaxNodes: 128, MaxRequests: 256, MaxEvidenceBytes: 4 << 20, MaxPathWork: 100000, TimeoutMS: 30000, RequestTimeoutMS: 5000, MaxResponseBytes: 2 << 20, MaxMessages: 64}
}
func producerSpecPrivateV2(t *testing.T, root string, nom, con int) ScopeSpecPrivateV2 {
	t.Helper()
	s := ScopeSpecPrivateV2{Root: root, RevisionAssertion: strings.Repeat("a", 40), PartitionSize: 1, EffectivePolicy: producerPolicyPrivateV2()}
	if nom > 1 {
		s.PartitionSize = 2
	}
	for i := 0; i < nom+con; i++ {
		role := "NOMINATION"
		if i >= nom {
			role = "CONTEXT_ONLY"
		}
		rel := fmt.Sprintf("owned-v2/%s-%02d.go", strings.ToLower(role), i)
		body := []byte(fmt.Sprintf("package owned\n// private case %d/%d member %d\nfunc A%d() {}\n\nfunc B%d() {}\n", nom, con, i, i, i))
		file := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, body, 0600); err != nil {
			t.Fatal(err)
		}
		s.Members = append(s.Members, ScopeMemberPrivateV2{CanonicalRelativePath: rel, Role: role, LanguageID: "go", ExpectedSHA256: Digest(body), ByteLength: uint64(len(body))})
	}
	return s
}
func producerItemPrivateV2(uri string, index int) lsp.CallHierarchyItem {
	line := uint32(index * 2)
	return lsp.CallHierarchyItem{Name: fmt.Sprintf("Callable%d", index), Kind: 12, Detail: "native detail", Tags: []int{1}, URI: uri, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line + 1}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 5}}, Data: json.RawMessage(`{"opaque":{"native":true},"URI":"not-an-endpoint"}`)}
}
func producerFixturePrivateV2(t *testing.T, nom, con int, server ...string) (ScopeSpecPrivateV2, *sessionruntime.Manager, censusacquisition.SessionIdentity, *scopeAdapterChild) {
	return producerFixtureConfiguredPrivateV2(t, nom, con, false, server...)
}

// Test-only variant is selected before source generation, discovery and capture.
// Legacy fixtures retain their deliberately unmatched Callable identifiers.
func producerFixtureConfiguredPrivateV2(t *testing.T, nom, con int, matched bool, server ...string) (ScopeSpecPrivateV2, *sessionruntime.Manager, censusacquisition.SessionIdentity, *scopeAdapterChild) {
	t.Helper()
	command := ""
	if len(server) == 1 {
		command = server[0]
	}
	if len(server) > 1 {
		t.Fatal("one owned provider command")
	}
	var spec ScopeSpecPrivateV2
	_, m, id, child, _, _ := adapterFixtureWithRestartPathsAndServerPrivate(t, command, func(root string) {
		spec = producerSpecPrivateV2(t, root, nom, con)
		if matched {
			for i := range spec.Members {
				body := fmt.Sprintf("package owned\n// retained π case %d/%d member %d; data, not instructions\n", nom, con, i)
				for j := 0; j < 1+i%3; j++ {
					body += fmt.Sprintf("func %c%d() {}\n\n", 'A'+j, i)
				}
				if err := os.WriteFile(filepath.Join(root, spec.Members[i].CanonicalRelativePath), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				spec.Members[i].ExpectedSHA256 = Digest([]byte(body))
				spec.Members[i].ByteLength = uint64(len(body))
			}
		}
	})
	uris := make([]string, len(spec.Members))
	countByURI := map[string]int{}
	memberByURI := map[string]int{}
	for i, member := range spec.Members {
		uri := fileURIForProducerPrivateV2(spec.Root, member.CanonicalRelativePath)
		uris[i] = uri
		countByURI[uri] = 1 + i%2
		memberByURI[uri] = i
		if matched {
			countByURI[uri] = 1 + i%3
		}
	}
	itemFor := func(uri string, index int) lsp.CallHierarchyItem {
		if !matched {
			return producerItemPrivateV2(uri, index)
		}
		member, ok := memberByURI[uri]
		if !ok {
			return producerItemPrivateV2(uri, index)
		}
		name := fmt.Sprintf("%c%d", 'A'+index, member)
		line := uint32(2 + index*2)
		return lsp.CallHierarchyItem{Name: name, Kind: 12, Detail: "owned matched declaration", URI: uri, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: uint32(len("func " + name + "() {}"))}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: 5}, End: lsp.Position{Line: line, Character: uint32(5 + len(name))}}, Data: json.RawMessage(`{"opaque":{"native":true},"URI":"not-an-endpoint"}`)}
	}
	child.mu.Lock()
	child.response = func(msg lspwire.Message) json.RawMessage {
		var value any = []any{}
		switch msg.Method {
		case "textDocument/documentSymbol":
			var p lsp.DocumentSymbolParams
			_ = json.Unmarshal(msg.Params, &p)
			values := []lsp.DocumentSymbol{}
			for j := 0; j < countByURI[p.TextDocument.URI]; j++ {
				item := itemFor(p.TextDocument.URI, j)
				values = append(values, lsp.DocumentSymbol{Name: item.Name, Kind: item.Kind, Detail: item.Detail, Range: item.Range, SelectionRange: item.SelectionRange})
			}
			value = values
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			_ = json.Unmarshal(msg.Params, &p)
			index := int(p.Position.Line / 2)
			if matched {
				index = int(p.Position.Line-2) / 2
			}
			value = []lsp.CallHierarchyItem{itemFor(p.TextDocument.URI, index)}
		case "callHierarchy/incomingCalls":
			var p lsp.CallHierarchyIncomingCallsParams
			_ = json.Unmarshal(msg.Params, &p)
			if (!matched && p.Item.SelectionRange.Start.Line == 0) || (matched && p.Item.SelectionRange.Start.Line == 2) {
				return json.RawMessage("null")
			}
			value = []lsp.CallHierarchyIncomingCall{}
		case "callHierarchy/outgoingCalls":
			var p lsp.CallHierarchyOutgoingCallsParams
			_ = json.Unmarshal(msg.Params, &p)
			outside := producerItemPrivateV2("file:///withheld-outside-private-v2.go", 0)
			calls := []lsp.CallHierarchyOutgoingCall{{To: outside, FromRanges: []lsp.Range{}}}
			if con > 0 && p.Item.URI != uris[nom] {
				c := lsp.CallHierarchyOutgoingCall{To: itemFor(uris[nom], 0), FromRanges: []lsp.Range{{Start: lsp.Position{}, End: lsp.Position{Character: 1}}}}
				calls = append(calls, c, c)
			}
			value = calls
		}
		raw, _ := json.Marshal(value)
		return raw
	}
	child.mu.Unlock()
	return spec, m, id, child
}
func fileURIForProducerPrivateV2(root, rel string) string {
	// Use the exact neutral selection canonicalizer rather than manual URI escaping.
	// The paths in these owned fixtures are ASCII and require no escaping.
	return "file://" + filepath.ToSlash(filepath.Join(root, rel))
}
func producerLeasePrivateV2(t *testing.T, m *sessionruntime.Manager, id censusacquisition.SessionIdentity, root string, fn func(context.Context) error) {
	t.Helper()
	if err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: root}, fn); err != nil {
		t.Fatal(err)
	}
}
func TestScopeProducerPrivateV2Connected(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}, {5, 2}, {8, 8}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			spec, m, id, child := producerFixturePrivateV2(t, counts[0], counts[1])
			frozen, err := FreezeScopePrivateV2(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			// Mutating caller-owned DTO and detached bytes cannot change frozen pins.
			detached := frozen.SpecBytes()
			detached[0] = '!'
			spec.Members[0].ExpectedSHA256 = Digest([]byte("wrong caller mutation"))
			var origin EnumerationOriginPrivateV2
			producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
				var report ScopeDiscoveryReportPrivateV2
				origin, report, err = DiscoverScopePrivateV2(ctx, frozen, m, id)
				if err != nil {
					return err
				}
				var d censusacquisition.Discovery
				if json.Unmarshal(report.Bytes(), &d) != nil || d.Accounting.FileDenominator != counts[0] || !d.Complete {
					t.Fatal("ASSERT_V2_ACTUAL_ENUMERATION")
				}
				selection, e := ParseSourceSelection(frozen.SelectionBytes())
				if e != nil {
					return e
				}
				filePlan, e := ParsePartitionPlan(frozen.FilePlanBytes(), selection)
				if e != nil {
					return e
				}
				wantFilePartitions := (counts[0] + frozen.state.spec.PartitionSize - 1) / frozen.state.spec.PartitionSize
				if len(filePlan.Plan.Partitions) != wantFilePartitions {
					t.Fatal("ASSERT_V2_FILE_PARTITION_MAPPING")
				}
				if counts[0] > 1 && len(filePlan.Plan.Partitions) == origin.BatchCount() {
					t.Fatal("ASSERT_V2_FILE_PARTITION_NOT_TARGET_BATCH")
				}
				wantTargets := counts[0] + counts[0]/2
				if len(d.Targets) != wantTargets || origin.BatchCount() != (wantTargets+1)/2 {
					t.Fatal("ASSERT_V2_TARGET_BATCH_MAPPING", len(d.Targets), origin.BatchCount())
				}
				var frame originFramePrivateV2
				if json.Unmarshal(origin.Bytes(), &frame) != nil || frame.SpecSHA256 != Digest(frozen.SpecBytes()) || frame.FilePlanSHA256 != Digest(frozen.FilePlanBytes()) || frame.Authority != 0 || frame.Accepted || frame.Source != "UNKNOWN" || frame.Absence != "PROHIBITED" {
					t.Fatal("ASSERT_V2_ORIGIN_PINS")
				}
				wantQueryURIs := make([]string, len(frozen.state.files))
				for i, file := range frozen.state.files {
					wantQueryURIs[i] = file.URI
				}
				sort.Strings(wantQueryURIs)
				if !reflect.DeepEqual(frame.CanonicalQueryURIs, wantQueryURIs) {
					t.Fatal("ASSERT_V2_SORTED_QUERY_VS_ORDERED_PINS")
				}
				a := origin.state.adapter
				if a.guard != nil || a.guardV2 == nil || len(a.guardV2.MembersPrivate()) != counts[0]+counts[1] {
					t.Fatal("ASSERT_V2_REAL_GUARD")
				}
				if counts[1] > 0 {
					member := frozen.state.files[counts[0]]
					if a.Supply(ctx, censusacquisition.SourceFile{Path: member.Path, URI: member.URI, LanguageID: "go"}) == nil {
						t.Fatal("ASSERT_V2_CONTEXT_NEVER_NOMINATION")
					}
				}
				// Nomination-only discovery has completed before any directional graph query.
				messages := child.snapshot()
				prepared, symbols := 0, 0
				firstSymbols := -1
				for i, msg := range messages {
					if msg.Method == "textDocument/didOpen" || msg.Method == "textDocument/didChange" {
						prepared++
					}
					if msg.Method == "textDocument/documentSymbol" {
						symbols++
						if firstSymbols < 0 {
							firstSymbols = i
						}
						if prepared != counts[0]+counts[1] {
							t.Fatal("ASSERT_V2_ALL_SUPPLY_BEFORE_DISCOVERY")
						}
					}
					if strings.HasPrefix(msg.Method, "callHierarchy/") {
						t.Fatal("ASSERT_V2_NO_GRAPH_BEFORE_DISCOVERY")
					}
				}
				if firstSymbols < 0 || symbols != counts[0] {
					t.Fatal("ASSERT_V2_NOMINATION_DENOMINATOR")
				}
				for i := 0; i < origin.BatchCount(); i++ {
					batch, e := origin.Batch(i)
					if e != nil {
						return e
					}
					pair, e := AcquireScopeBatchPrivateV2(ctx, m, origin, batch)
					if e != nil {
						return fmt.Errorf("batch %d: %w", i, e)
					}
					parsed, e := graphprovenance.ParseSourceWithholdingPrivateV2(pair.ArtifactBytes())
					if e != nil {
						return e
					}
					actualQuery, e := parsed.Evidence.Acquisition.Request.FrozenQueryMembersPrivateV2()
					if e != nil || !reflect.DeepEqual(actualQuery, wantQueryURIs) {
						t.Fatal("ASSERT_V2_CAPTURED_QUERY_DECLARATION", e)
					}
					if len(parsed.Policy.Members) != counts[0]+counts[1] || pair.ArtifactDigest() != Digest(pair.ArtifactBytes()) || pair.ArtifactByteLength() != uint64(len(pair.ArtifactBytes())) {
						t.Fatal("ASSERT_V2_CAPTURE_PINS")
					}
					for _, member := range parsed.Policy.Members {
						found := false
						for _, file := range frozen.state.files {
							if member.URI == file.URI && member.SHA256 == file.SHA256 && member.Role == file.Role {
								found = true
							}
						}
						if !found {
							t.Fatal("ASSERT_V2_EXACT_POLICY_MEMBERSHIP")
						}
					}
					contextCaptured, outsideWithheld := false, false
					for _, row := range parsed.Evidence.Captures {
						if row.URI == "file:///withheld-outside-private-v2.go" {
							outsideWithheld = row.Status == "OUTSIDE_SCOPE" && len(row.Content) == 0 && row.Path == ""
						}
						if counts[1] > 0 && row.URI == frozen.state.files[counts[0]].URI {
							contextCaptured = row.Status == "READABLE" && bytes.Equal(row.Content, frozen.state.files[counts[0]].Bytes)
						}
					}
					if !outsideWithheld || (counts[1] > 0 && !contextCaptured) {
						t.Fatal("ASSERT_V2_CONTEXT_FROM_RELATION", contextCaptured, outsideWithheld)
					}
					for _, target := range parsed.Evidence.Acquisition.Targets {
						item := target.Resolution.Prepared
						if item == nil {
							t.Fatal("ASSERT_V2_PREPARED_ITEM_PRESERVED")
						}
						want := producerItemPrivateV2(item.URI, int(item.SelectionRange.Start.Line/2))
						if !reflect.DeepEqual(*item, want) {
							t.Fatal("ASSERT_V2_PREPARED_METADATA_PRESERVED")
						}
					}
					for _, node := range parsed.Evidence.Acquisition.Graph.Nodes {
						want := producerItemPrivateV2(node.URI, int(node.SelectionRange.Start.Line/2))
						if node.Name != want.Name || node.Detail != want.Detail || !bytes.Equal(node.Data, want.Data) {
							t.Fatal("ASSERT_V2_NATIVE_METADATA_PRESERVED")
						}
					}
					var captureFrame captureOriginFramePrivateV2
					if json.Unmarshal(pair.OriginBytes(), &captureFrame) != nil || captureFrame.ArtifactSHA256 != pair.ArtifactDigest() || captureFrame.ArtifactByteLength != pair.ArtifactByteLength() || captureFrame.EnumerationOriginSHA256 != Digest(origin.Bytes()) {
						t.Fatal("ASSERT_V2_DETACHED_PAIR_BINDING")
					}
					copied := batch
					if _, e = AcquireScopeBatchPrivateV2(ctx, m, origin, copied); e == nil {
						t.Fatal("ASSERT_V2_SHARED_SPEND_REPLAY")
					}
					if !batch.state.spent {
						t.Fatal("ASSERT_V2_SHARED_SPEND_STATE")
					}
				}
				if _, _, e := DiscoverScopePrivateV2(ctx, frozen, m, id); e == nil {
					t.Fatal("ASSERT_V2_NO_RESCAN")
				}
				return nil
			})
			if origin.state.adapter.check(context.Background()) == nil {
				t.Fatal("ASSERT_V2_LEASE_EXPIRED")
			}
		})
	}
}
func TestScopeProducerPrivateV2SpecRefusals(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	base := producerSpecPrivateV2(t, root, 2, 1)
	cases := map[string]func(*ScopeSpecPrivateV2){
		"nom9": func(s *ScopeSpecPrivateV2) {
			for i := 0; i < 7; i++ {
				v := s.Members[0]
				v.CanonicalRelativePath = fmt.Sprintf("new%d.go", i)
				s.Members = append(s.Members, v)
			}
		},
		"context9": func(s *ScopeSpecPrivateV2) {
			for i := 0; i < 8; i++ {
				v := s.Members[2]
				v.CanonicalRelativePath = fmt.Sprintf("ctx%d.go", i)
				s.Members = append(s.Members, v)
			}
		},
		"role": func(s *ScopeSpecPrivateV2) { s.Members[0].Role = "BOTH" }, "language": func(s *ScopeSpecPrivateV2) { s.Members[0].LanguageID = "" }, "long-language": func(s *ScopeSpecPrivateV2) { s.Members[0].LanguageID = strings.Repeat("x", 257) },
		"path": func(s *ScopeSpecPrivateV2) { s.Members[0].CanonicalRelativePath = "../outside.go" }, "empty-component": func(s *ScopeSpecPrivateV2) { s.Members[0].CanonicalRelativePath = "a//b.go" }, "backslash": func(s *ScopeSpecPrivateV2) { s.Members[0].CanonicalRelativePath = `a\b.go` }, "absolute": func(s *ScopeSpecPrivateV2) { s.Members[0].CanonicalRelativePath = "/a.go" }, "depth": func(s *ScopeSpecPrivateV2) { s.Members[0].CanonicalRelativePath = strings.Repeat("a/", 33) + "a.go" },
		"hash": func(s *ScopeSpecPrivateV2) { s.Members[0].ExpectedSHA256 = "bad" }, "length": func(s *ScopeSpecPrivateV2) { s.Members[0].ByteLength = 0 }, "declared-budget": func(s *ScopeSpecPrivateV2) { s.Members[0].ByteLength = 2<<20 + 1 }, "duplicate": func(s *ScopeSpecPrivateV2) { s.Members[1] = s.Members[0] },
		"zero": func(s *ScopeSpecPrivateV2) { s.EffectivePolicy.MaxRequests = 0 }, "unbounded": func(s *ScopeSpecPrivateV2) { s.EffectivePolicy.Discovery.MaxSymbols = 0 }, "ceiling": func(s *ScopeSpecPrivateV2) { s.EffectivePolicy.MaxTargets = 64 }, "partition": func(s *ScopeSpecPrivateV2) { s.PartitionSize = 3 }, "revision": func(s *ScopeSpecPrivateV2) { s.RevisionAssertion = strings.Repeat("A", 40) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := base
			s.Members = append([]ScopeMemberPrivateV2(nil), base.Members...)
			mutate(&s)
			reads := 0
			f, err := freezeScopePrivateV2(context.Background(), s, scopeReadOps{hook: func(string, string) error { reads++; return nil }})
			if err == nil || f.valid() || reads != 0 {
				t.Fatal("ASSERT_V2_PREFLIGHT_BEFORE_IO", reads, err)
			}
		})
	}
	for _, kind := range []string{"digest-mismatch", "length-mismatch", "close", "cancel", "hardlink", "symlink-final", "symlink-ancestor"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			root, _ = filepath.EvalSymlinks(root)
			s := producerSpecPrivateV2(t, root, 2, 0)
			ops := scopeReadOps{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reads := 0
			ops.hook = func(stage, p string) error {
				if stage == "file-read" {
					reads++
				}
				return nil
			}
			switch kind {
			case "digest-mismatch":
				s.Members[0].ExpectedSHA256 = Digest([]byte("mismatch"))
			case "length-mismatch":
				s.Members[0].ByteLength++
			case "close":
				ops.close = func(f *os.File) error { _ = f.Close(); return errors.New("owned close fault") }
			case "cancel":
				ops.hook = func(stage, p string) error {
					if stage == "file-read" {
						cancel()
					}
					return nil
				}
			case "hardlink":
				_ = os.Remove(filepath.Join(root, s.Members[1].CanonicalRelativePath))
				if err := os.Link(filepath.Join(root, s.Members[0].CanonicalRelativePath), filepath.Join(root, s.Members[1].CanonicalRelativePath)); err != nil {
					t.Fatal(err)
				}
			case "symlink-final":
				p := filepath.Join(root, s.Members[0].CanonicalRelativePath)
				_ = os.Rename(p, p+".target")
				_ = os.Symlink(p+".target", p)
			case "symlink-ancestor":
				p := filepath.Join(root, "owned-v2")
				_ = os.Rename(p, p+"-target")
				_ = os.Symlink(p+"-target", p)
			}
			f, err := freezeScopePrivateV2(ctx, s, ops)
			if err == nil || f.valid() {
				t.Fatal("ASSERT_V2_IO_FAILURE_NIL")
			}
			if kind == "hardlink" && reads != 1 {
				t.Fatal("ASSERT_V2_ALIAS_BEFORE_SECOND_READ", reads)
			}
		})
	}
}
func TestScopeProducerPrivateV2IdentitySubstitution(t *testing.T) {
	for _, kind := range []string{"root", "file", "changed"} {
		t.Run(kind, func(t *testing.T) {
			spec, m, id, _ := producerFixturePrivateV2(t, 1, 0)
			f, err := FreezeScopePrivateV2(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath)
			switch kind {
			case "root":
				if err = os.Rename(spec.Root, spec.Root+"-old"); err != nil {
					t.Fatal(err)
				}
				_ = os.Mkdir(spec.Root, 0700)
				for _, file := range f.state.files {
					q := filepath.Join(spec.Root, file.Path)
					_ = os.MkdirAll(filepath.Dir(q), 0700)
					_ = os.WriteFile(q, file.Bytes, 0600)
				}
			case "file":
				_ = os.Remove(p)
				_ = os.WriteFile(p, f.state.files[0].Bytes, 0600)
			case "changed":
				_ = os.WriteFile(p, []byte("package changed\n"), 0600)
			}
			producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
				if _, err := newScopeRuntimeAdapterPrivateV2(ctx, f, m, id); err == nil {
					t.Fatal("ASSERT_V2_SUBSTITUTION_REJECTED")
				}
				return nil
			})
		})
	}
}
func TestScopeProducerPrivateV2IncompleteAndEmpty(t *testing.T) {
	for _, kind := range []string{"empty", "symbols", "targets"} {
		t.Run(kind, func(t *testing.T) {
			spec, m, id, child := producerFixturePrivateV2(t, 2, 0)
			switch kind {
			case "empty":
				child.mu.Lock()
				child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage("[]") }
				child.mu.Unlock()
			case "symbols":
				spec.EffectivePolicy.Discovery.MaxSymbols = 1
			case "targets":
				spec.EffectivePolicy.MaxTargets = 1
				spec.EffectivePolicy.Planning.MaxBatchTargets = 1
			}
			f, err := FreezeScopePrivateV2(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
				o, r, err := DiscoverScopePrivateV2(ctx, f, m, id)
				if err == nil || o.valid() || o.BatchCount() != 0 || len(r.Bytes()) == 0 {
					t.Fatal("ASSERT_V2_NO_INCOMPLETE_OR_EMPTY_ORIGIN", err)
				}
				return nil
			})
		})
	}
}
func TestScopeProducerPrivateV2SingleSpendAndPreflight(t *testing.T) {
	spec, m, id, _ := producerFixturePrivateV2(t, 2, 1)
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		o, _, err := DiscoverScopePrivateV2(ctx, f, m, id)
		if err != nil {
			return err
		}
		b, _ := o.Batch(0)
		req, err := scopeBatchRequestPrivateV2(o.state.adapter, b.state.binding)
		if err != nil {
			return err
		}
		wrong := b.state.binding
		wrong.RequestID += "-foreign"
		if _, err = acquireScopeBatchPrivateV2(ctx, m, o, b, wrong, req); err == nil || b.state.spent {
			t.Fatal("ASSERT_V2_BAD_BINDING_UNSPENT")
		}
		if _, err = AcquireScopeBatchPrivateV2(ctx, &sessionruntime.Manager{}, o, b); err == nil || b.state.spent {
			t.Fatal("ASSERT_V2_FOREIGN_MANAGER_UNSPENT")
		}
		nested := directionalraw.NewReservation("nested")
		defer nested.Release()
		if _, err = AcquireScopeBatchPrivateV2(nested.Context(ctx), m, o, b); err == nil || b.state.spent {
			t.Fatal("ASSERT_V2_NESTED_UNSPENT")
		}
		wrongReq := req
		wrongReq.Context.Generation++
		if _, err = acquireScopeBatchPrivateV2(ctx, m, o, b, b.state.binding, wrongReq); err == nil || b.state.spent {
			t.Fatal("ASSERT_V2_BAD_REQUEST_UNSPENT")
		}
		var wg sync.WaitGroup
		var successes atomic.Int32
		for i := 0; i < 3; i++ {
			copy := b
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, e := AcquireScopeBatchPrivateV2(ctx, m, o, copy); e == nil {
					successes.Add(1)
				}
			}()
		}
		wg.Wait()
		if successes.Load() != 1 {
			t.Fatal("ASSERT_V2_CONCURRENT_SINGLE_SPEND", successes.Load())
		}
		t.Log("ASSERT_V2_CONCURRENT_SINGLE_SPEND: PASS")
		return nil
	})
}
func TestScopeProducerPrivateV2PostConsumeFailureExhausted(t *testing.T) {
	for _, kind := range []string{"grant", "capture"} {
		t.Run(kind, func(t *testing.T) {
			spec, m, id, _ := producerFixturePrivateV2(t, 1, 0)
			f, err := FreezeScopePrivateV2(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
				o, _, err := DiscoverScopePrivateV2(ctx, f, m, id)
				if err != nil {
					return err
				}
				b, _ := o.Batch(0)
				if kind == "grant" {
					b.state.authority, err = acquisitionauthority.MintSeedAuthority(b.state.binding, b.state.seeds, seedbinding.CallerAssertedLocal, true, "", true)
					if err != nil {
						return err
					}
				} else {
					o.state.adapter.beforePartialCapturePrivateTest = func(r *acquisition.Result) { r.Policy = "owned-invalid-capture-policy" }
				}
				pair, err := AcquireScopeBatchPrivateV2(ctx, m, o, b)
				if err == nil || pair.valid() || !b.state.spent {
					t.Fatal("ASSERT_V2_POST_CONSUME_EXHAUSTED", err, b.state.spent)
				}
				if _, err = AcquireScopeBatchPrivateV2(ctx, m, o, b); err == nil {
					t.Fatal("ASSERT_V2_POST_CONSUME_REPLAY")
				}
				t.Log("ASSERT_V2_POST_CONSUME_EXHAUSTED: PASS")
				return nil
			})
		})
	}
}
func TestScopeProducerPrivateV2OpaqueJSON(t *testing.T) {
	for _, value := range []any{FrozenScopePrivateV2{}, EnumerationOriginPrivateV2{}, ScopeBatchSelectionPrivateV2{}, OpaqueSuccessfulV2Capture{}} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatal("ASSERT_V2_NO_CAPABILITY_JSON")
		}
	}
	var o EnumerationOriginPrivateV2
	_ = json.Unmarshal([]byte(`{"state":{"accepted":true}}`), &o)
	if o.valid() || o.BatchCount() != 0 {
		t.Fatal("ASSERT_V2_JSON_NO_ORIGIN")
	}
	var result OpaqueSuccessfulV2Capture
	_ = json.Unmarshal([]byte(`{"origin":"consistent","artifact":"consistent"}`), &result)
	if result.valid() || result.ArtifactBytes() != nil {
		t.Fatal("ASSERT_V2_JSON_NO_PRODUCER")
	}
}
