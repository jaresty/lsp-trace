package censusacquisition

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
)

func TestDiscoveryClosureDistinguishesSuccessfulEmptyFromUnavailable(t *testing.T) {
	workspace := t.TempDir()
	file := testSource(workspace, "empty.go")
	empty, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: &fakeDiscoveryClient{documentSupported: true, callSupported: true}}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || empty.Complete || empty.Closure != DiscoveryClosureSuccessful || len(empty.Targets) != 0 {
		t.Fatalf("ASSERT_DISCOVERY_SUCCESSFUL_EMPTY_CLOSURE discovery=%+v err=%v", empty, err)
	}
	file.Disposition = census.FileUnreadable
	unavailable, err := (DiscoveryAdapter{Workspace: workspace, Files: StaticFiles{file}, Supplier: &fakeSupplier{errors: map[string]error{}}, Client: &fakeDiscoveryClient{documentSupported: true, callSupported: true}}).Discover(context.Background(), SessionIdentity{"s", 1})
	if err != nil || unavailable.Closure != DiscoveryClosureUnknown || len(unavailable.ProviderFailures) != 1 {
		t.Fatalf("ASSERT_DISCOVERY_UNAVAILABLE_NOT_SUCCESSFUL_EMPTY discovery=%+v err=%v", unavailable, err)
	}
}

func TestCloneDiscoverySliceNilnessMatrixAndDeepAntiAlias(t *testing.T) {
	type sliceCase struct {
		name  string
		set   func(*Discovery, bool)
		isNil func(Discovery) bool
	}
	cases := []sliceCase{
		{"Accounting.Files", func(d *Discovery, empty bool) {
			if empty {
				d.Accounting.Files = make([]census.FileEntry, 0)
			}
		}, func(d Discovery) bool { return d.Accounting.Files == nil }},
		{"Accounting.Symbols", func(d *Discovery, empty bool) {
			if empty {
				d.Accounting.Symbols = make([]census.SymbolEntry, 0)
			}
		}, func(d Discovery) bool { return d.Accounting.Symbols == nil }},
		{"FileLedger.Entries", func(d *Discovery, empty bool) {
			if empty {
				d.FileLedger.Entries = make([]captureset.LedgerEntry, 0)
			}
		}, func(d Discovery) bool { return d.FileLedger.Entries == nil }},
		{"SymbolLedger.Entries", func(d *Discovery, empty bool) {
			if empty {
				d.SymbolLedger.Entries = make([]captureset.LedgerEntry, 0)
			}
		}, func(d Discovery) bool { return d.SymbolLedger.Entries == nil }},
		{"Targets", func(d *Discovery, empty bool) {
			if empty {
				d.Targets = make([]PreparedTarget, 0)
			}
		}, func(d Discovery) bool { return d.Targets == nil }},
		{"EnumerationSymbols", func(d *Discovery, empty bool) {
			if empty {
				d.EnumerationSymbols = make([]EnumerationSymbol, 0)
			}
		}, func(d Discovery) bool { return d.EnumerationSymbols == nil }},
		{"ProviderFailures", func(d *Discovery, empty bool) {
			if empty {
				d.ProviderFailures = make([]ProviderFailure, 0)
			}
		}, func(d Discovery) bool { return d.ProviderFailures == nil }},
		{"IncompleteCauses", func(d *Discovery, empty bool) {
			if empty {
				d.IncompleteCauses = make([]IncompleteCause, 0)
			}
		}, func(d Discovery) bool { return d.IncompleteCauses == nil }},
		{"Observations", func(d *Discovery, empty bool) {
			if empty {
				d.Observations = make([]DiscoveryObservation, 0)
			}
		}, func(d Discovery) bool { return d.Observations == nil }},
	}
	for _, tc := range cases {
		for _, empty := range []bool{false, true} {
			name := "nil"
			if empty {
				name = "non-nil-empty"
			}
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				var original Discovery
				tc.set(&original, empty)
				clone := cloneDiscovery(original)
				if gotNil := tc.isNil(clone); gotNil == empty {
					t.Fatalf("ASSERT_DISCOVERY_CLONE_SLICE_NILNESS field=%s empty=%t got_nil=%t", tc.name, empty, gotNil)
				}
			})
		}
	}

	for _, seed := range []struct {
		name  string
		value []byte
	}{
		{name: "nil"},
		{name: "non-nil-empty", value: make([]byte, 0)},
	} {
		t.Run("CanonicalSeedV2/"+seed.name, func(t *testing.T) {
			clone := cloneDiscovery(Discovery{Targets: []PreparedTarget{{CanonicalSeedV2: seed.value}}})
			if gotNil := clone.Targets[0].CanonicalSeedV2 == nil; gotNil != (seed.value == nil) {
				t.Fatalf("ASSERT_DISCOVERY_CLONE_CANONICAL_SEED_NILNESS input_nil=%t output_nil=%t", seed.value == nil, gotNil)
			}
		})
	}

	fileOrdinal, symbolOrdinal := 1, 2
	observed, omitted := uint64(3), uint64(4)
	original := Discovery{
		Targets:          []PreparedTarget{{CanonicalSeedV2: []byte("seed")}},
		ProviderFailures: []ProviderFailure{{FileOrdinal: &fileOrdinal}},
		IncompleteCauses: []IncompleteCause{{SymbolOrdinal: &symbolOrdinal}},
		Observations:     []DiscoveryObservation{{Observed: &observed, Omitted: &omitted}},
	}
	clone := cloneDiscovery(original)
	original.Targets[0].CanonicalSeedV2[0] = 'X'
	*original.ProviderFailures[0].FileOrdinal = 9
	*original.IncompleteCauses[0].SymbolOrdinal = 9
	*original.Observations[0].Observed = 9
	*original.Observations[0].Omitted = 9
	if string(clone.Targets[0].CanonicalSeedV2) != "seed" || *clone.ProviderFailures[0].FileOrdinal != 1 || *clone.IncompleteCauses[0].SymbolOrdinal != 2 || *clone.Observations[0].Observed != 3 || *clone.Observations[0].Omitted != 4 {
		t.Fatal("ASSERT_DISCOVERY_CLONE_NESTED_REFERENCE_ANTI_ALIAS")
	}
}

func TestCloneDiscoveryReferenceFieldCoverage(t *testing.T) {
	listed := []string{"Accounting", "EnumerationSymbols", "FileLedger", "IncompleteCauses", "Observations", "ProviderFailures", "SymbolLedger", "Targets"}
	var got []string
	typ := reflect.TypeOf(Discovery{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if typeContainsMutableReference(field.Type, map[reflect.Type]bool{}) {
			got = append(got, field.Name)
		}
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, listed) {
		t.Fatalf("ASSERT_DISCOVERY_CLONE_REFERENCE_FIELD_COVERAGE got=%v want=%v", got, listed)
	}
}

func typeContainsMutableReference(typ reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[typ] {
		return false
	}
	seen[typ] = true
	switch typ.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer:
		return true
	case reflect.Array:
		return typeContainsMutableReference(typ.Elem(), seen)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			if typeContainsMutableReference(typ.Field(i).Type, seen) {
				return true
			}
		}
	}
	return false
}
