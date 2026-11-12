package inventorysuccessor

import (
	"context"
	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/sourceprojection"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	pa "lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
)

func TestSelectedV2SevenPolicies(t *testing.T) {
	cases := []struct {
		mode     string
		k        []pa.RelationKind
		included []string
	}{
		{"RD", []pa.RelationKind{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}, []string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}},
		{"C", []pa.RelationKind{pa.TypedCalls}, []string{"CALLS"}},
		{"CRD", []pa.RelationKind{pa.TypedCalls, pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}, []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}},
		{"D", []pa.RelationKind{pa.TypedResolvesToDefinition}, []string{"RESOLVES_TO_DEFINITION"}},
		{"CD", []pa.RelationKind{pa.TypedCalls, pa.TypedResolvesToDefinition}, []string{"CALLS", "RESOLVES_TO_DEFINITION"}},
		{"R", []pa.RelationKind{pa.TypedReferencesSymbol}, []string{"REFERENCES_SYMBOL"}},
		{"CR", []pa.RelationKind{pa.TypedCalls, pa.TypedReferencesSymbol}, []string{"CALLS", "REFERENCES_SYMBOL"}},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			m, ok := selectedModeV2(c.k)
			if !ok || m != c.mode {
				t.Fatal("selected configuration shape")
			}
			p, e := selectedPolicyV2(m)
			if e != nil {
				t.Fatal(e)
			}
			_, actual, e := rg.ReadAllModePolicyMetadataPrivate(p)
			if e != nil || len(actual) != len(c.included) {
				t.Fatal("selected actual policy kinds", actual, e)
			}
			for i, k := range actual {
				if k != c.included[i] {
					t.Fatal("selected actual policy kind", actual)
				}
			}
		})
	}
	for _, k := range [][]pa.RelationKind{nil, {pa.TypedCalls, pa.TypedCalls}, {pa.RelationKind("UNKNOWN")}} {
		if _, ok := selectedModeV2(k); ok {
			t.Fatal("invalid kinds accepted")
		}
	}
}

func TestSelectedV2QuoteAlgebra(t *testing.T) {
	now := time.Unix(100, 0)
	deadline := now.Add(15 * time.Second)
	previous := countersV2{}
	for _, o := range []uint64{1, 2, 73, 74, 1024} {
		c, e := selectedSourceChargeV2(o, 8<<20, 64<<20, now, deadline)
		if e != nil {
			t.Fatal(e)
		}
		if !previous.within(c) {
			t.Fatal("nonmonotone source quote")
		}
		previous = c
		if c.ProviderRequests != 0 || c.ProviderMessages != 0 || c.RequestBytes != 0 || c.ResponseBytes != 0 || c.WireBytes != 0 {
			t.Fatal("source provider coordinate nonzero")
		}
		if c.ActiveNS != uint64(15*time.Second) || c.WriteBytes != 7*(8<<20)+6*(recordLimitV2+2*headLimitV2) {
			t.Fatal("source store/native quote")
		}
	}
	for _, o := range []uint64{0, 1025, math.MaxUint64} {
		if _, e := selectedSourceChargeV2(o, 8<<20, 64<<20, now, deadline); e == nil {
			t.Fatal("source count bound")
		}
	}
	if _, e := selectedSourceChargeV2(1, 8<<20, 64<<20, now, now.Add(15*time.Second+1)); e == nil {
		t.Fatal("source deadline bound")
	}
	if _, e := selectedSourceChargeV2(1, 8<<20, 64<<20, deadline, deadline); e == nil {
		t.Fatal("expired quote")
	}
	if _, e := selectedMulV2(math.MaxUint64, 2); e == nil {
		t.Fatal("checked multiply")
	}
	if _, e := selectedAddV2(math.MaxUint64, 1); e == nil {
		t.Fatal("checked addition")
	}
	m := rg.AllModeMetadataPrivate{Input: rg.CompositionCommitment{ByteLength: 1 << 20}, Contributions: 1024, Custody: 2048, Predecessors: 1024}
	p, e := selectedPreparationChargeV2(m, now, deadline)
	if e != nil {
		t.Fatal(e)
	}
	if p.ProviderRequests != 0 || p.ProviderMessages != 0 || p.RequestBytes != 0 || p.ResponseBytes != 0 || p.WireBytes != 0 {
		t.Fatal("preparation provider coordinate")
	}
	m.Custody++
	if _, e := selectedPreparationChargeV2(m, now, deadline); e == nil {
		t.Fatal("preparation metadata bound")
	}
	t.Log("ASSERT_SELECTED_QUOTE: PASS")
}

// Independent fixed oracle for O=2, artifact=256KiB, parent=1MiB,
// original remaining deadline=10s; derived from the published component formula.
func TestSelectedV2QuoteExactVector(t *testing.T) {
	now := time.Unix(100, 0)
	c, e := selectedSourceChargeV2(2, 256<<10, 1<<20, now, now.Add(10*time.Second))
	want := [14]uint64{9037261824, 9326144512, 2719744, 66718, 9305697280, 0, 0, 9041814110, 10000000000, 132254, 0, 0, 0, 35389440}
	if e != nil || c.vector() != want {
		t.Fatalf("ASSERT_SELECTED_QUOTE_VECTOR: got %v want %v error %v", c.vector(), want, e)
	}
	for u := uint64(0); u <= 10000; u++ {
		if 34*u+9 <= 10000 && u > 293 {
			t.Fatal("upper reconstruction bound")
		}
	}
	for _, v := range [][3]int{{2, 1, 2}, {2048, 1024, 2048}} {
		if !selectedProjectionWithinQuoteV2(v[0], v[1], v[2]) {
			t.Fatal("dense kernel equality guard")
		}
	}
	for _, v := range [][3]int{{3, 1, 2}, {2, 1, 3}, {0, 0, 0}, {2048, 1025, 2048}, {-1, 1, 0}} {
		if selectedProjectionWithinQuoteV2(v[0], v[1], v[2]) {
			t.Fatal("dense kernel unquoted envelope")
		}
	}
	t.Log("ASSERT_SELECTED_QUOTE_VECTOR: PASS")
}

func TestSelectedV2ContractBeforeSerialization(t *testing.T) {
	pin := "sha256:" + strings.Repeat("0", 64)
	in := cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, CaptureLimits: cc.ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "CONTROLLED_BOUNDED_CONTRACT"}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(in)
	if e != nil {
		t.Fatal(e)
	}
	c := PrivateSelectedConfigurationV2{Contract: contract, ContractInput: in}
	checked, e := selectedCheckedContractV2(c)
	if e != nil || checked.Contract.ID() != contract.ID() {
		t.Fatal("bounded contract identity", e)
	}
	c.ContractInput.ProgramCSeed = 1
	if _, e := selectedCheckedContractV2(c); e == nil {
		t.Fatal("contract attestation mismatch")
	}
	c.ContractInput = in
	c.ContractInput.SchemaVersions.Handoff = strings.Repeat("x", 4097)
	if _, e := selectedCheckedContractV2(c); e == nil {
		t.Fatal("unbounded schema must reject before any contract serialization")
	}
	c.ContractInput = in
	c.ContractInput.SchemaVersions = cc.SchemaVersions{Handoff: strings.Repeat("x", 256), ProgramC: strings.Repeat("x", 256), Snapshots: strings.Repeat("x", 256), Packets: strings.Repeat("x", 256)}
	if _, e := selectedCheckedContractV2(c); e == nil {
		t.Fatal("aggregate metadata strings unbounded")
	}
	for _, v := range []any{map[string]string{}, [17]string{}, [16][16]uint64{}} {
		dst := reflect.New(reflect.TypeOf(v)).Elem()
		budget := [2]int{128, 1024}
		if selectedContractValueV2(dst, reflect.ValueOf(v), &budget, 0) {
			t.Fatal("metadata shape should refuse")
		}
	}
	// Detached slices preserve canonical null/empty distinctions and custody.
	for _, v := range [][]string{nil, {}, {"bounded"}} {
		var dst []string
		budget := [2]int{128, 1024}
		if !selectedContractValueV2(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(v), &budget, 0) || !reflect.DeepEqual(v, dst) {
			t.Fatal("metadata clone")
		}
		if len(v) > 0 {
			v[0] = "changed"
			if dst[0] != "bounded" {
				t.Fatal("borrowed slice")
			}
		}
	}
	t.Log("ASSERT_SELECTED_CONTRACT_PRE_SERIALIZATION: PASS")
}

func TestSelectedV2OriginalScopeAndConcurrentRetirement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &selectedPlanV2{request: PrivateMixedInputRequestV2{}, owner: &PrivateManagedInputOwnerV2{}}
	p.request.Start.Deadline = time.Now().Add(time.Second)
	p.operation = PrivateMixedInputOperationV2{&privateMixedInputOperationV2{retainedContext: ctx}}
	if !selectedPlanLiveV2(p) {
		t.Fatal("original scope live")
	}
	cancel()
	if selectedPlanLiveV2(p) {
		t.Fatal("original scope cannot be renewed by new caller")
	}
	source := PrivateSelectedSourceOperationV2{&selectedSourceOperationV2{prepared: &preparedSelectedV2{plan: p}, ctx: ctx}}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, _, _, _, e := source.Begin(context.Background()); e == nil {
				t.Error("canceled original source available")
			}
		}()
	}
	wg.Wait()
	if !source.state.used || source.state.begun {
		t.Fatal("failed Begin must remain consumed and unbegun")
	}
	if source.Finish(context.Background(), nil) == nil {
		t.Fatal("failed Begin cannot be relabeled observed")
	}
	captured := PrivateSelectedCapturedV2{&selectedCapturedV2{plan: p}}
	if _, e := p.owner.ReserveSelectedPreparationV2(nil, captured); e == nil || !captured.state.reserved {
		t.Fatal("nil P reserve must consume captured stage")
	}
	if _, e := p.owner.ReserveSelectedPreparationV2(context.Background(), captured); e == nil {
		t.Fatal("captured stage cannot retry")
	}
}

func TestSelectedV2RetireCanceledOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &selectedPlanV2{owner: &PrivateManagedInputOwnerV2{}, request: PrivateMixedInputRequestV2{}}
	source := PrivateSelectedSourceOperationV2{&selectedSourceOperationV2{prepared: &preparedSelectedV2{plan: p}, ctx: ctx}}
	if _, _, _, _, _, e := source.Begin(ctx); e == nil || !source.state.used {
		t.Fatal("canceled source must retire")
	}
	if _, _, _, _, _, e := source.Begin(context.Background()); e == nil {
		t.Fatal("source reused")
	}
	prep := PrivateSelectedPreparationOperationV2{&selectedPreparationOperationV2{captured: &selectedCapturedV2{plan: p}, ctx: ctx}}
	if _, e := ExecuteSelectedPreparationV2(ctx, prep); e == nil || !prep.state.used {
		t.Fatal("canceled preparation must retire")
	}
	if _, e := ExecuteSelectedPreparationV2(context.Background(), prep); e == nil {
		t.Fatal("preparation reused")
	}
	if _, e := ExecuteSelectedMixedInputV2(ctx, PrivateMixedInputOperationV2{}, PrivateMixedInputRequestV2{}, PrivateSelectedPreparationPlanV2{}); e == nil {
		t.Fatal("foreign plan accepted")
	}
}
