package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

func membersPrivateV2(n int) []string {
	m := make([]string, n)
	for i := range m {
		m[i] = fmt.Sprintf("file:///%02d.go", i)
	}
	return m
}
func declarationPrivateV2(t *testing.T, m []string) FrozenQueryDeclarationPrivateV2 {
	t.Helper()
	d, err := NewFrozenQueryDeclarationPrivateV2(m)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func bindPrivateV2(t *testing.T, r Request, m []string) Request {
	t.Helper()
	r, err := r.WithFrozenQueryDeclarationPrivateV2(declarationPrivateV2(t, m))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFrozenQueryPrivateV2Scheduling(t *testing.T) {
	for _, n := range []int{1, 3, 7, 16} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			m := membersPrivateV2(n)
			a, b := item("A", 0), item("B", 4)
			a.URI, b.URI = m[0], "file:///outside.go"
			f := fixture()
			f.add(a)
			f.edge(a, b)
			r := request(a)
			r.Root.DownDepth, r.Root.UpDepth = 1, 1
			r = bindPrivateV2(t, r, m)
			got := run(t, f, r)
			for _, call := range f.calls {
				if call == "in:B" || call == "out:B" || call == "in:A" {
					t.Fatalf("ASSERT_V2_NO_EXTERNAL_OR_RESEED: %s", call)
				}
			}
			if len(got.Requests) != 2 || len(got.Graph.Edges) != 1 || len(got.Graph.Frontier) != 2 || got.Graph.Summary.Complete || len(got.Omissions) != 0 {
				t.Fatal("ASSERT_V2_RETAINED_PARTIAL")
			}
			incoming := got.Targets[0].Incoming
			if !reflect.DeepEqual(incoming.StartIDs, []string{node(b).ID}) || len(incoming.Expansions) != 1 || incoming.Expansions[0].RequestID != "" || incoming.Expansions[0].Status != Frontier || incoming.Expansions[0].Reason != FrozenQueryStopPrivate {
				t.Fatal("ASSERT_V2_NO_FAKE_RESPONSE")
			}
			for _, boundary := range got.Graph.Frontier {
				if boundary.Reason != graph.ExternalURI || !strings.Contains(boundary.Message, "originating_observations=request-000002") {
					t.Fatal("ASSERT_V2_ORIGIN")
				}
			}
			for _, mutation := range []string{"missing", "origin", "direction", "inside"} {
				bad := got
				bad.Graph.Frontier = append([]graph.Boundary(nil), got.Graph.Frontier...)
				switch mutation {
				case "missing":
					bad.Request.frozenQueryDeclarationPrivateV2 = nil
				case "origin":
					bad.Graph.Frontier[0].Message = "external-to-frozen-queryset direction=INCOMING originating_observations=request-FAKE"
				case "direction":
					bad.Graph.Frontier[0].Message = "external-to-frozen-queryset direction=FAKE"
				case "inside":
					bad.Request = bindPrivateV2(t, request(a), []string{a.URI, b.URI})
					bad.Request.Root.DownDepth, bad.Request.Root.UpDepth = 1, 1
				}
				if ValidateResult(bad) == nil {
					t.Fatalf("ASSERT_V2_FRONTIER_CONSISTENCY: %s", mutation)
				}
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var detached Result
			if err = json.Unmarshal(raw, &detached); err != nil {
				t.Fatal(err)
			}
			if detached.Request.frozenQueryDeclarationPrivateV2 != nil || ValidateResult(detached) == nil {
				t.Fatal("ASSERT_V2_DETACHED_RESULT_REFUSED")
			}
			t.Log("ASSERT_V2_NO_EXTERNAL_OR_RESEED: PASS")
		})
	}
}

func TestFrozenQueryPrivateV2InsideContextAndBudgets(t *testing.T) {
	for _, n := range []int{3, 7, 16} {
		t.Run(fmt.Sprintf("context-%d", n), func(t *testing.T) {
			m := membersPrivateV2(n)
			a, b := item("A", 0), item("B", 4)
			a.URI, b.URI = m[0], m[n-1]
			f := fixture()
			f.add(a)
			f.edge(a, b)
			r := request(a)
			r.Root.DownDepth, r.Root.UpDepth = 1, 1
			r = bindPrivateV2(t, r, m)
			got := run(t, f, r)
			if len(got.Graph.Frontier) != 0 || !reflect.DeepEqual(got.Targets[0].Incoming.StartIDs, []string{node(b).ID}) {
				t.Fatal("ASSERT_V2_CONTEXT_START")
			}
			if len(f.calls) != 3 || f.calls[1] != "out:A" || f.calls[2] != "in:B" {
				t.Fatalf("ASSERT_V2_CONTEXT_QUERIED: %v", f.calls)
			}
		})
	}
	for _, n := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("budget-%d", n), func(t *testing.T) {
			a, b := item("A", 0), item("B", 4)
			b.URI = "file:///outside.go"
			f := fixture()
			f.add(a)
			f.edge(a, b)
			r := request(a)
			r.Root.DownDepth, r.Root.UpDepth = 1, 1
			r.Limits.MaxRequests = n
			r = bindPrivateV2(t, r, []string{a.URI})
			got := run(t, f, r)
			if got.Usage.Requests > n || len(f.calls) > n || got.AcquisitionComplete {
				t.Fatal("ASSERT_V2_FINITE_BUDGET")
			}
		})
	}
}

func TestFrozenQueryPrivateV2Constructor(t *testing.T) {
	for name, m := range map[string][]string{
		"zero": nil, "seventeen": membersPrivateV2(17), "duplicate": {"file:///a.go", "file:///a.go"},
		"unsorted": {"file:///b.go", "file:///a.go"}, "invalid-utf8": {"file:///a\xff"},
		"scheme-alias": {"FILE:///a.go"}, "host-alias": {"file://HOST/a.go"},
		"path-alias": {"file:///a/../b.go"}, "slash-alias": {"file:///a//b.go"},
		"escape-alias": {"file:///%61.go"}, "relative": {"a.go"}, "fragment": {"file:///a.go#x"}, "query": {"file:///a.go?q=x"},
		"per-uri-over": {"file:///" + strings.Repeat("a", 8192-len("file:///")+1)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewFrozenQueryDeclarationPrivateV2(m); err == nil {
				t.Fatal("ASSERT_V2_INVALID_METADATA")
			}
		})
	}
	// Path case is not normalized: both canonical spellings are distinct members.
	d := declarationPrivateV2(t, []string{"file:///A.go", "file:///a.go"})
	if !d.contains("file:///A.go") || !d.contains("file:///a.go") || d.contains("file:///a.GO") {
		t.Fatal("ASSERT_V2_EXACT_CASE")
	}
	max := make([]string, 16)
	for i := range max {
		max[i] = "file:///" + fmt.Sprintf("%02d", i) + strings.Repeat("a", 8192-len("file:///")-2)
	}
	d = declarationPrivateV2(t, max)
	if len(d.MembersPrivateV2()) != 16 {
		t.Fatal("ASSERT_V2_AGGREGATE_EQUALITY")
	}
	// At 16 * 8192 the aggregate is exactly 128 KiB. +1 necessarily also
	// exceeds a per-URI cap; an aggregate-only violating state is unreachable.
	max[15] += "a"
	if _, err := NewFrozenQueryDeclarationPrivateV2(max); err == nil {
		t.Fatal("ASSERT_V2_AGGREGATE_PLUS_ONE")
	}
	huge := []string{"file:///" + strings.Repeat("a", 1<<20)}
	alloc := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := NewFrozenQueryDeclarationPrivateV2(huge); err == nil {
				b.Fatal("ASSERT_V2_OVERSIZE")
			}
		}
	})
	if alloc.AllocedBytesPerOp() > 512 {
		t.Fatalf("ASSERT_V2_EARLY_BOUNDED_ALLOCATION: %d", alloc.AllocedBytesPerOp())
	}
}

func TestFrozenQueryPrivateV2ImmutabilityAndWire(t *testing.T) {
	a := item("A", 0)
	input := []string{a.URI, "file:///b.go"}
	d := declarationPrivateV2(t, input)
	r, err := request(a).WithFrozenQueryDeclarationPrivateV2(d)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = "file:///outside.go"
	detached := d.MembersPrivateV2()
	detached[0] = "file:///outside.go"
	copied := r
	if !reflect.DeepEqual(d.MembersPrivateV2(), []string{a.URI, "file:///b.go"}) || !copied.queryAllowedPrivate(a.URI) || copied.queryAllowedPrivate(input[0]) {
		t.Fatal("ASSERT_V2_IMMUTABLE")
	}
	// A copy of the bound Request shares only immutable declaration state.
	gotMembers := copied.frozenQueryDeclarationPrivateV2.MembersPrivateV2()
	gotMembers[0] = input[0]
	if !r.queryAllowedPrivate(a.URI) {
		t.Fatal("ASSERT_V2_REQUEST_COPY")
	}
	old := request(a)
	raw, _ := json.Marshal(r)
	oldRaw, _ := json.Marshal(old)
	if !bytes.Equal(raw, oldRaw) {
		t.Fatalf("ASSERT_V2_WIRE_UNCHANGED: %s != %s", raw, oldRaw)
	}
	var fresh Request
	if err := json.Unmarshal(raw, &fresh); err != nil {
		t.Fatal(err)
	}
	if fresh.frozenQueryPrivate() || fresh.frozenQueryDeclarationPrivateV2 != nil || fresh.FrozenQueryURIsPrivate != [6]string{} {
		t.Fatal("ASSERT_V2_FRESH_JSON")
	}
	// Unknown public names cannot install the unexported field.
	if err := json.Unmarshal([]byte(`{"frozenQueryDeclarationPrivateV2":{},"FrozenQueryDeclarationPrivateV2":{}}`), &fresh); err != nil || fresh.frozenQueryPrivate() {
		t.Fatal("ASSERT_V2_JSON_CANNOT_POPULATE")
	}
	populated := r
	if err := json.Unmarshal(raw, &populated); err != nil || populated.frozenQueryDeclarationPrivateV2 != r.frozenQueryDeclarationPrivateV2 {
		t.Fatal("ASSERT_V2_PREPOPULATED_RETENTION")
	}
	legacy := old
	legacy.FrozenQueryURIsPrivate = [6]string{"file:///a.go", "file:///b.go", "file:///c.go", "file:///d.go", "file:///e.go", "file:///f.go"}
	legacyRaw, _ := json.Marshal(legacy)
	if !bytes.Equal(raw, legacyRaw) {
		t.Fatal("ASSERT_V2_V1_WIRE_UNCHANGED")
	}
}

func TestFrozenQueryPrivateV2RefusalBeforeTraffic(t *testing.T) {
	a := item("A", 0)
	d := declarationPrivateV2(t, []string{a.URI})
	for name, decl := range map[string]FrozenQueryDeclarationPrivateV2{
		"zero": {}, "forged": {state: &frozenQueryStatePrivateV2{members: []string{a.URI}}},
		"forged-seal":     {state: &frozenQueryStatePrivateV2{members: []string{a.URI}, seal: &frozenQuerySealPrivateV2{marker: 1}}},
		"invalid-members": {state: &frozenQueryStatePrivateV2{seal: frozenQuerySealV2}},
	} {
		t.Run(name, func(t *testing.T) {
			plain := request(a)
			if _, err := plain.WithFrozenQueryDeclarationPrivateV2(decl); err == nil || decl.MembersPrivateV2() != nil {
				t.Fatal("ASSERT_V2_INVALID_DECLARATION")
			}
			plain.frozenQueryDeclarationPrivateV2 = &decl
			f := fixture()
			f.add(a)
			if !plain.frozenQueryPrivate() || plain.queryAllowedPrivate(a.URI) {
				t.Fatal("ASSERT_V2_FAIL_CLOSED")
			}
			if _, err := Acquire(context.Background(), f, plain); err == nil || len(f.calls) != 0 {
				t.Fatal("ASSERT_V2_PREFLIGHT")
			}
		})
	}
	for _, name := range []string{"mixed", "root-outside", "required-outside"} {
		t.Run(name, func(t *testing.T) {
			r := request(a)
			switch name {
			case "mixed":
				r.FrozenQueryURIsPrivate = [6]string{"file:///a.go", "file:///b.go", "file:///c.go", "file:///d.go", "file:///e.go", "file:///f.go"}
			case "root-outside":
				r.Root.Locator.URI = "file:///outside.go"
			case "required-outside":
				b := item("B", 4)
				b.URI = "file:///outside.go"
				r = request(a, b)
			}
			if _, err := r.WithFrozenQueryDeclarationPrivateV2(d); err == nil {
				t.Fatal("ASSERT_V2_BIND_REFUSAL")
			}
			r.frozenQueryDeclarationPrivateV2 = &d
			f := fixture()
			f.add(a)
			if _, err := Acquire(context.Background(), f, r); err == nil || len(f.calls) != 0 {
				t.Fatal("ASSERT_V2_PREFLIGHT")
			}
		})
	}
	// Even identical membership across versions cannot bypass mixed refusal.
	six := []string{"file:///a.go", "file:///b.go", "file:///c.go", "file:///d.go", "file:///e.go", "file:///f.go"}
	r := request(a)
	copy(r.FrozenQueryURIsPrivate[:], six)
	d = declarationPrivateV2(t, six)
	r.frozenQueryDeclarationPrivateV2 = &d
	f := fixture()
	f.add(a)
	if _, err := Acquire(context.Background(), f, r); err == nil || len(f.calls) != 0 {
		t.Fatal("ASSERT_V2_IDENTICAL_MIXED")
	}
}
