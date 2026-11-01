//go:build darwin || linux

package retainedpartialprogramc

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/programc"
)

// Detached diagnostic data only. This does not construct a ProjectionPrivateV3,
// assessment seal, source admission, original capture, or provider observation.
func detachedAssessmentReportPrivateV3(nodes []string, seeds []uint64) InstabilityReportPrivateV3 {
	s := SourceBindingPrivateV3{Admission: pilot.SelectedAdmissionProfilePrivateV3, Partial: true, Profile: ProfilePrivateV3, ProfileSHA256: strings.Repeat("a", 64), ClaimCeiling: ClaimCeilingPrivateV3, PageRevisionSHA256: strings.Repeat("b", 64), DerivedCanonicalV5SHA256: strings.Repeat("c", 64), FullBatchCount: 1, FullTargetCount: 1, BatchTargetCount: 1, SourceMemberCount: 1, SessionID: "detached-not-an-owner", WorkspaceURI: "file:///detached", ProviderCommand: "controlled-not-live", Generation: 1, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	a := assessmentMathPrivateV3(s, seeds)
	a.Outcome = "STABLE"
	a.CompleteRuns, a.CompletePairs = a.ExpectedRuns, a.ExpectedPairs
	for i := range a.Runs {
		members := nodes
		if nodes != nil {
			members = append([]string{}, nodes...)
		}
		a.Runs[i].Status = "COMPLETE"
		a.Runs[i].Partition = programc.PartitionMathPrivate{Outcome: "COMPLETE", Algorithm: "DETACHED_TEST_PARTITION", LogicalDigest: strings.Repeat("d", 64), Resolution: 1, Seed: seeds[i], Communities: []programc.Community{{Members: members}}}
	}
	for i := range a.Pairs {
		a.Pairs[i].Status = "COMPLETE"
		a.Pairs[i].Metrics = programc.PureInstabilityMetrics{Outcome: "STABLE", Smaller: 1, Larger: 1, Work: 2, States: 2, WorkRepresentable: true, NodeDenominator: len(nodes), CommunityDenominator: 1, MatchedDenominator: 1, Minimum: 1, Median: 1, Mean: 1}
	}
	return InstabilityReportPrivateV3{s, a}
}

func TestAssessmentSerializationPrivateV3OversizeBeforeMarshal(t *testing.T) {
	// Six repeated partition identities exceed the report cap, despite one node
	// per run (well below 10,000) and no serialized occurrence collection. Only
	// the input string is allocated here; the spy refuses any attempted marshal.
	identity := strings.Repeat("x", assessmentReportBytesPrivateV3/6+1)
	r := detachedAssessmentReportPrivateV3([]string{identity}, []uint64{1, 2, 3, 4, 5, 6})
	if len(r.Math.Runs) != 6 || len(r.Math.Pairs) != 15 || len(r.Math.Runs[0].Partition.Communities[0].Members) != 1 || len(identity) <= assessmentReportBytesPrivateV3/6 {
		t.Fatal("detached fixture setup")
	}
	calls := 0
	spyErr := errors.New("marshal boundary reached")
	raw, err := marshalAssessmentPrivateV3(context.Background(), r, func() error { calls++; return spyErr })
	if !errors.Is(err, pilot.ErrPagesUnitPrivateV3) || calls != 0 || raw != nil {
		t.Fatalf("ASSERT_V3_ASSESSMENT_OVERSIZE_BEFORE_MARSHAL: calls=%d err=%v bytes=%d", calls, err, len(raw))
	}
	t.Log("ASSERT_V3_ASSESSMENT_OVERSIZE_BEFORE_MARSHAL: PASS; detached report only, not an oversized public-owner path")
}

func TestAssessmentSerializationPrivateV3CanceledBeforeMarshal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	raw, err := marshalAssessmentPrivateV3(ctx, detachedAssessmentReportPrivateV3([]string{"a"}, []uint64{3, 7}), func() error { calls++; return errors.New("marshal boundary reached") })
	if !errors.Is(err, context.Canceled) || calls != 0 || raw != nil {
		t.Fatalf("ASSERT_V3_ASSESSMENT_CANCEL_BEFORE_MARSHAL: calls=%d err=%v bytes=%d", calls, err, len(raw))
	}
	t.Log("ASSERT_V3_ASSESSMENT_CANCEL_BEFORE_MARSHAL: PASS")
}

func TestAssessmentSerializationPrivateV3ExactJSON(t *testing.T) {
	for _, nodes := range [][]string{{"one"}, {"a", "b", "c"}, nil, {}} {
		r := detachedAssessmentReportPrivateV3(nodes, []uint64{0, math.MaxUint64})
		r.Source.ProviderCommand = "&<>\x00\x01\b\f\n\r\t\\\"π\u2028\u2029\xff"
		r.Source.Generation = math.MaxUint64
		r.Math.Authority = math.MinInt
		r.Math.Pairs[0].Metrics.VI = -math.MaxFloat64
		want, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		got, err := marshalAssessmentPrivateV3(context.Background(), r, func() error { calls++; return nil })
		if err != nil || calls != 1 || !bytes.Equal(got, want) {
			t.Fatalf("ASSERT_V3_ASSESSMENT_EXACT_JSON: nodes=%d calls=%d err=%v", len(nodes), calls, err)
		}
	}
	t.Log("ASSERT_V3_ASSESSMENT_EXACT_JSON: PASS; trivial/multiple members, escaping, invalid UTF-8 replacement, numeric widths, nil/empty member lists")
}

func TestAssessmentSerializationPrivateV3MultiNodeMath(t *testing.T) {
	ctx := context.Background()
	seeds := []uint64{3, 7}
	r := detachedAssessmentReportPrivateV3([]string{"a", "b"}, seeds)
	m := programc.CallsMathPrivate{NodeIdentities: []string{"a", "b"}, Occurrences: []programc.Occurrence{{Identity: "o", RelationID: "r", Provider: "detached-not-a-capture", From: 0, To: 1, Weight: 1}}, PairWeights: map[programc.Pair]float64{{From: 0, To: 1}: 1}}
	a := assessmentMathPrivateV3(r.Source, seeds)
	for i, seed := range seeds {
		partition, failure := programc.PartitionCallsMathPrivate(m, seed)
		if failure != nil {
			t.Fatal("multi-node math fixture", failure)
		}
		a.Runs[i].Partition, a.Runs[i].Status = partition, partition.Outcome
	}
	assessed, err := assessPrivateMath(ctx, m, a, seeds, a.Runs)
	if err != nil {
		t.Fatal("multi-node assessment fixture", err)
	}
	r.Math = assessed
	want, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := assessmentEncodedBoundPrivateV3(ctx, r)
	got, serializeErr := marshalAssessmentPrivateV3(ctx, r, nil)
	if err != nil || serializeErr != nil || !bytes.Equal(got, want) || len(got) > bound || r.Math.CompleteRuns != 2 || r.Math.CompletePairs != 1 {
		t.Fatal("ASSERT_V3_ASSESSMENT_MULTI_NODE_MATH", err, serializeErr, r.Math.CompleteRuns, r.Math.CompletePairs)
	}
	t.Log("ASSERT_V3_ASSESSMENT_MULTI_NODE_MATH: PASS; unchanged neutral two-node computation/assessment, detached serializer input, no source authority")
}

func TestAssessmentSerializationPrivateV3ObserveRefuseOnly(t *testing.T) {
	r := detachedAssessmentReportPrivateV3([]string{"a"}, []uint64{3, 7})
	refusal := errors.New("observer refusal")
	if raw, err := marshalAssessmentPrivateV3(context.Background(), r, func() error { return refusal }); raw != nil || !errors.Is(err, refusal) {
		t.Fatal("ASSERT_V3_ASSESSMENT_OBSERVER_REFUSAL", err)
	}
	r.Math.Runs[0].Partition.Resolution = math.NaN()
	if raw, err := marshalAssessmentPrivateV3(context.Background(), r, nil); raw != nil || err == nil {
		t.Fatal("ASSERT_V3_ASSESSMENT_JSON_ERROR", err)
	}
	t.Log("ASSERT_V3_ASSESSMENT_OBSERVER_REFUSAL: PASS; hook cannot supply bytes")
}

func TestAssessmentSerializationPrivateV3ComputeValidateDependency(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "private_v3.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "ComputeInstabilityPrivateV3" && fn.Name.Name != "ValidateInstabilityPrivateV3") {
			continue
		}
		found++
		bounded, direct := 0, 0
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "marshalAssessmentPrivateV3" {
				bounded++
				if len(call.Args) != 3 {
					t.Fatal("serializer argument shape", fn.Name.Name)
				}
				id, ok := call.Args[2].(*ast.Ident)
				if !ok || id.Name != "nil" {
					t.Fatal("production observer must be nil", fn.Name.Name)
				}
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "json" && sel.Sel.Name == "Marshal" {
					direct++
				}
			}
			return true
		})
		if bounded != 1 || direct != 0 {
			t.Fatalf("ASSERT_V3_ASSESSMENT_PRODUCTION_DEPENDENCY: %s bounded=%d direct=%d", fn.Name.Name, bounded, direct)
		}
	}
	if found != 2 {
		t.Fatal("ASSERT_V3_ASSESSMENT_PRODUCTION_DEPENDENCY: zero/missing function match", found)
	}
	t.Log("ASSERT_V3_ASSESSMENT_PRODUCTION_DEPENDENCY: PASS; Compute and Validate call the bounded serializer, no direct json.Marshal")
}

// Reflection is test-only: force any future JSON field/type addition through
// these coverage/budget checks rather than silently trusting the concrete walk.
func TestAssessmentSerializationPrivateV3ConcreteSchemaBudgets(t *testing.T) {
	jsonMarshaler := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshaler := reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	lists := map[string]bool{
		"Source.SelectedObjectSHA256":             false,
		"Math.Seeds":                              false,
		"Math.Runs":                               false,
		"Math.Runs.Partition.Communities":         false,
		"Math.Runs.Partition.Communities.Members": false,
		"Math.Pairs":                              false,
	}
	var inspect func(reflect.Type, string, int)
	inspect = func(typ reflect.Type, path string, depth int) {
		if depth > 8 {
			t.Fatal("unexpected recursive report type", path)
		}
		if typ.Implements(jsonMarshaler) || reflect.PointerTo(typ).Implements(jsonMarshaler) || typ.Implements(textMarshaler) || reflect.PointerTo(typ).Implements(textMarshaler) {
			t.Fatal("custom encoding requires a new bound", path)
		}
		switch typ.Kind() {
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if f.PkgPath != "" {
					continue
				} // json ignores unexported retained bytes.
				if f.Anonymous || f.Tag.Get("json") != "" {
					t.Fatal("new JSON schema behavior requires review", path, f.Name)
				}
				next := f.Name
				if path != "" {
					next = path + "." + next
				}
				inspect(f.Type, next, depth+1)
			}
		case reflect.Array, reflect.Slice:
			if _, ok := lists[path]; !ok {
				t.Fatal("new list requires concrete traversal", path)
			}
			lists[path] = true
			if typ.Kind() == reflect.Array && (path != "Source.SelectedObjectSHA256" || typ.Len() != 4) {
				t.Fatal("array shape changed", path)
			}
			inspect(typ.Elem(), path, depth+1)
		case reflect.String, reflect.Bool, reflect.Int, reflect.Uint64, reflect.Float64:
		default:
			t.Fatal("unsupported/new report type requires concrete bound", path, typ)
		}
	}
	inspect(reflect.TypeOf(InstabilityReportPrivateV3{}), "", 0)
	for path, seen := range lists {
		if !seen {
			t.Fatal("missing expected list", path)
		}
	}

	// Upper-bound key/punctuation/scalar cost independently from production.
	// String values and list contents are excluded: the concrete walker counts
	// those dynamically. Include nested fixed struct keys (run/partition, metrics).
	var fixed func(reflect.Type) int
	fixed = func(typ reflect.Type) int {
		size, fields := 2, 0
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.PkgPath != "" {
				continue
			}
			key, err := json.Marshal(f.Name)
			if err != nil {
				t.Fatal(err)
			}
			size += len(key) + 1 // colon
			if fields > 0 {
				size++
			}
			fields++
			switch f.Type.Kind() {
			case reflect.Struct:
				size += fixed(f.Type)
			case reflect.String, reflect.Array, reflect.Slice: // dynamic value separately counted
			default:
				size += 32 // conservative scalar width, including bool
			}
		}
		return size
	}
	cases := []struct {
		value  any
		budget int
	}{
		{SourceBindingPrivateV3{}, 4096}, {InstabilityPrivate{}, 4096},
		{InstabilityRunPrivate{}, 1024}, {InstabilityPairPrivate{}, 2048}, {programc.Community{}, 64},
	}
	for _, c := range cases {
		cost := fixed(reflect.TypeOf(c.value))
		if cost > c.budget {
			t.Fatalf("ASSERT_V3_ASSESSMENT_FIXED_BUDGET: %T cost=%d budget=%d", c.value, cost, c.budget)
		}
	}
	envelope := fixed(reflect.TypeOf(InstabilityReportPrivateV3{})) - fixed(reflect.TypeOf(SourceBindingPrivateV3{})) - fixed(reflect.TypeOf(InstabilityPrivate{}))
	if envelope > 64 {
		t.Fatal("envelope budget", envelope)
	}
	t.Log("ASSERT_V3_ASSESSMENT_FIXED_BUDGET: PASS; concrete keys/scalars and exact dynamic list schema, no custom marshalers")
}

func TestAssessmentSerializationPrivateV3EveryStringField(t *testing.T) {
	r := detachedAssessmentReportPrivateV3([]string{"a", "b"}, []uint64{1, 2, 3, 4, 5, 6})
	oversize := strings.Repeat("x", assessmentReportBytesPrivateV3/6+1)
	fields := 0
	var walk func(reflect.Value, string)
	walk = func(v reflect.Value, path string) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if f.PkgPath == "" {
					walk(v.Field(i), path+"."+f.Name)
				}
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i), path+"["+strconv.Itoa(i)+"]")
			}
		case reflect.String:
			fields++
			t.Run(path, func(t *testing.T) {
				original := v.String()
				defer v.SetString(original)
				v.SetString(oversize)
				calls := 0
				raw, err := marshalAssessmentPrivateV3(context.Background(), r, func() error { calls++; return errors.New("marshal boundary reached") })
				if !errors.Is(err, pilot.ErrPagesUnitPrivateV3) || calls != 0 || raw != nil {
					t.Fatalf("ASSERT_V3_ASSESSMENT_EVERY_STRING: %s calls=%d err=%v", path, calls, err)
				}
			})
		}
	}
	walk(reflect.ValueOf(&r).Elem(), "Report")
	if fields == 0 {
		t.Fatal("zero string-field matches")
	}
	t.Logf("ASSERT_V3_ASSESSMENT_EVERY_STRING: PASS; %d exported string leaves independently oversized", fields)
}

func TestAssessmentSerializationPrivateV3BoundAndBoundary(t *testing.T) {
	ctx := context.Background()
	r := detachedAssessmentReportPrivateV3([]string{"a", "b"}, []uint64{0, math.MaxUint64})
	for _, value := range []string{"", "&<>\x00\x01\b\f\n\r\t\\\"", "π\u2028\u2029\xff"} {
		r.Source.ProviderCommand = value
		bound, err := assessmentEncodedBoundPrivateV3(ctx, r)
		raw, marshalErr := json.Marshal(r)
		if err != nil || marshalErr != nil || len(raw) > bound || bound > assessmentReportBytesPrivateV3 {
			t.Fatalf("ASSERT_V3_ASSESSMENT_ENCODED_BOUND: raw=%d bound=%d err=%v marshal=%v", len(raw), bound, err, marshalErr)
		}
	}
	payload := strings.Repeat("x", assessmentReportBytesPrivateV3/6+1)
	lo, hi := 0, len(payload)
	for lo < hi {
		mid := lo + (hi-lo+1)/2
		r.Source.ProviderCommand = payload[:mid]
		if _, err := assessmentEncodedBoundPrivateV3(ctx, r); err == nil {
			lo = mid
		} else if errors.Is(err, pilot.ErrPagesUnitPrivateV3) {
			hi = mid - 1
		} else {
			t.Fatal(err)
		}
	}
	r.Source.ProviderCommand = payload[:lo]
	bound, err := assessmentEncodedBoundPrivateV3(ctx, r)
	raw, marshalErr := marshalAssessmentPrivateV3(ctx, r, nil)
	if err != nil || marshalErr != nil || len(raw) > bound || bound > assessmentReportBytesPrivateV3 || assessmentReportBytesPrivateV3-bound >= 6 {
		t.Fatalf("ASSERT_V3_ASSESSMENT_BOUNDARY: n=%d raw=%d bound=%d err=%v marshal=%v", lo, len(raw), bound, err, marshalErr)
	}
	r.Source.ProviderCommand = payload[:lo+1]
	calls := 0
	if raw, err := marshalAssessmentPrivateV3(ctx, r, func() error { calls++; return errors.New("boundary marshal") }); !errors.Is(err, pilot.ErrPagesUnitPrivateV3) || calls != 0 || raw != nil {
		t.Fatal("ASSERT_V3_ASSESSMENT_BOUNDARY_NEXT_BYTE", calls, err)
	}
	b := assessmentBoundPrivateV3{ctx: ctx, remaining: assessmentReportBytesPrivateV3, work: 8}
	if err := b.take(assessmentReportBytesPrivateV3); err != nil || b.remaining != 0 {
		t.Fatal("equality boundary", err)
	}
	if err := b.take(1); !errors.Is(err, pilot.ErrPagesUnitPrivateV3) {
		t.Fatal("strict upper boundary", err)
	}
	b = assessmentBoundPrivateV3{ctx: ctx, remaining: assessmentReportBytesPrivateV3, work: 8}
	if err := b.take(math.MaxInt); !errors.Is(err, pilot.ErrPagesUnitPrivateV3) || b.remaining != assessmentReportBytesPrivateV3 {
		t.Fatal("overflow-safe subtraction", err)
	}
	t.Log("ASSERT_V3_ASSESSMENT_ENCODED_BOUND: PASS; escaping upper bound and conservative report boundary")
}

type cancelAssessmentWalkPrivateV3 struct {
	context.Context
	cancel        context.CancelFunc
	checks, after int
}

func (c *cancelAssessmentWalkPrivateV3) Err() error {
	c.checks++
	if c.checks == c.after {
		c.cancel()
	}
	return c.Context.Err()
}

func TestAssessmentSerializationPrivateV3WalkLimits(t *testing.T) {
	r := detachedAssessmentReportPrivateV3(make([]string, 512), []uint64{3, 7})
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &cancelAssessmentWalkPrivateV3{Context: base, cancel: cancel, after: 100}
	calls := 0
	raw, err := marshalAssessmentPrivateV3(ctx, r, func() error { calls++; return errors.New("marshal boundary") })
	if !errors.Is(err, context.Canceled) || calls != 0 || raw != nil || ctx.checks != ctx.after {
		t.Fatal("ASSERT_V3_ASSESSMENT_CANCEL_DURING_WALK", calls, ctx.checks, err)
	}
	if _, err := marshalAssessmentPrivateV3(nil, r, nil); !errors.Is(err, pilot.ErrInvalid) {
		t.Fatal("nil context", err)
	}
	r.Math.Seeds = make([]uint64, assessmentWalkWorkPrivateV3+1)
	calls = 0
	if raw, err := marshalAssessmentPrivateV3(context.Background(), r, func() error { calls++; return errors.New("marshal boundary") }); !errors.Is(err, pilot.ErrPagesUnitPrivateV3) || calls != 0 || raw != nil {
		t.Fatal("ASSERT_V3_ASSESSMENT_WORK_ADMISSION", calls, err)
	}
	b := assessmentBoundPrivateV3{ctx: context.Background(), remaining: assessmentReportBytesPrivateV3, work: 4}
	if err := b.list(2); err != nil || b.work != 0 {
		t.Fatal("list work reservation", b.work, err)
	}
	if err := b.take(0); !errors.Is(err, pilot.ErrPagesUnitPrivateV3) {
		t.Fatal("work cannot reset", err)
	}
	t.Log("ASSERT_V3_ASSESSMENT_WALK_LIMITS: PASS; mid-walk cancellation, nil context, list admission and monotone work")
}

func TestAssessmentSerializationPrivateV3NilEmptyLists(t *testing.T) {
	for _, empty := range []bool{false, true} {
		r := detachedAssessmentReportPrivateV3(nil, []uint64{3, 7})
		r.Math.Seeds, r.Math.Runs, r.Math.Pairs = nil, nil, nil
		if empty {
			r.Math.Seeds = []uint64{}
			r.Math.Runs = []InstabilityRunPrivate{}
			r.Math.Pairs = []InstabilityPairPrivate{}
		}
		bound, err := assessmentEncodedBoundPrivateV3(context.Background(), r)
		want, marshalErr := json.Marshal(r)
		got, serializeErr := marshalAssessmentPrivateV3(context.Background(), r, nil)
		if err != nil || marshalErr != nil || serializeErr != nil || !bytes.Equal(got, want) || len(want) > bound {
			t.Fatal("ASSERT_V3_ASSESSMENT_NIL_EMPTY_LISTS", empty, err, marshalErr, serializeErr)
		}
	}
	t.Log("ASSERT_V3_ASSESSMENT_NIL_EMPTY_LISTS: PASS")
}
