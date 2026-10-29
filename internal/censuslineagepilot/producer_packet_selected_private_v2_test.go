//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func selectedFixturePrivateV2(t *testing.T, members int) (DerivedProducerPacketPrivateV2, ProducerPacketRootsPrivateV2, producerPacketValidationCountsPrivateV2) {
	t.Helper()
	f, o, captures, _ := packetFixturePrivateV2(t, members, 0)
	roots, _, _ := packetRootsPrivateV2(t, f)
	retained, err := RetainCompleteScopePrivateV2(context.Background(), f, o, captures, roots)
	if err != nil {
		t.Fatal(err)
	}
	q, err := QualifyProducerPacketPrivateV2(context.Background(), retained.VerifiedPair())
	if err != nil {
		t.Fatal(err)
	}
	var observed producerPacketValidationCountsPrivateV2
	d, err := deriveProducerPacketPrivateV2(context.Background(), q, &observed)
	if err != nil {
		t.Fatal(err)
	}
	return d, roots, observed
}

func TestSelectedProducerSnapshotPrivateV2DetachedAndWork(t *testing.T) {
	d, _, admission := selectedFixturePrivateV2(t, 5)
	if admission.wholePacket != 1 || admission.sourceArtifact != 2*len(d.state.batches) {
		t.Fatalf("ASSERT_ALL_BATCH_ADMISSION: %+v batches=%d", admission, len(d.state.batches))
	}
	before, err := ReadDerivedProducerPacketPrivateV2(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		for i := range d.state.batches {
			observed := producerPacketValidationCountsPrivateV2{-1, -1}
			selected, err := readSelectedDerivedProducerPacketPrivateV2(context.Background(), d, i, &observed)
			if err != nil {
				t.Fatal(err)
			}
			if observed.wholePacket != 0 || observed.sourceArtifact != 0 {
				t.Fatalf("ASSERT_SELECTED_NO_WHOLE_REVALIDATION: %+v", observed)
			}
			original, err := selected.ReadPrivateV2(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			expected, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := selected.ViewPrivateV2()
			if err != nil {
				t.Fatal(err)
			}
			mutateSelectedMetadataPrivateV2(reflect.ValueOf(&changed).Elem())
			again, err := selected.ReadPrivateV2(context.Background())
			if err != nil {
				t.Fatal("ASSERT_SELECTED_DEEP_DETACH", err)
			}
			actual, err := json.Marshal(again)
			if err != nil || !bytes.Equal(expected, actual) || !reflect.DeepEqual(original, again) {
				t.Fatal("ASSERT_SELECTED_DEEP_DETACH")
			}
			if !bytes.Equal(again.Packet, before.Packet) || !bytes.Equal(again.Batch.SourceArtifact, before.Batches[i].SourceArtifact) || !bytes.Equal(again.Batch.NativeGraph, before.Batches[i].NativeGraph) {
				t.Fatal("ASSERT_SELECTED_ORIGINAL_BYTES")
			}
			members, err := again.Source.Evidence.Acquisition.Request.FrozenQueryMembersPrivateV2()
			if err != nil || len(members) == 0 {
				t.Fatal("ASSERT_SELECTED_IMMUTABLE_QUERY_METADATA", members, err)
			}
		}
	}
	after, err := ReadDerivedProducerPacketPrivateV2(context.Background(), d)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("ASSERT_SELECTED_LEGACY_UNCHANGED", err)
	}
	t.Logf("ASSERT_ALL_BATCH_ADMISSION: PASS whole=%d source=%d batches=%d", admission.wholePacket, admission.sourceArtifact, len(d.state.batches))
	t.Log("ASSERT_SELECTED_NO_WHOLE_REVALIDATION: PASS; ASSERT_SELECTED_DEEP_DETACH: PASS; ASSERT_SELECTED_ORIGINAL_BYTES: PASS")
}

// Change every exported leaf, including nested Data, request RawMessages,
// graph/locator pointers, nil-vs-empty containers and all returned byte arrays.
func mutateSelectedMetadataPrivateV2(v reflect.Value) {
	if !v.CanSet() {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).PkgPath == "" {
				mutateSelectedMetadataPrivateV2(v.Field(i))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			mutateSelectedMetadataPrivateV2(v.Index(i))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			mutateSelectedMetadataPrivateV2(v.Elem())
		}
	case reflect.Map:
		v.Clear()
	case reflect.Interface:
		if !v.IsNil() {
			v.Set(reflect.Zero(v.Type()))
		}
	case reflect.String:
		v.SetString("MUTATED")
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(v.Int() + 1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(v.Uint() + 1)
	}
}

func TestSelectedProducerSnapshotPrivateV2Refusals(t *testing.T) {
	for _, kind := range []string{"zero", "negative", "N", "copied-derived", "copied-qualified", "copied-snapshot", "changed-report", "changed-packet", "foreign-origin", "closed-output", "closed-source", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			d, roots, _ := selectedFixturePrivateV2(t, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ordinal := 0
			switch kind {
			case "zero":
				d = DerivedProducerPacketPrivateV2{}
			case "negative":
				ordinal = -1
			case "N":
				ordinal = len(d.state.batches)
			case "copied-derived":
				c := *d.state
				d.state = &c
			case "copied-qualified":
				c := *d.state.qualified.state
				d.state.qualified.state = &c
			case "copied-snapshot":
				c := *d.state.snapshot
				d.state.snapshot = &c
			case "changed-report":
				d.state.qualified.state.report[0] ^= 1
			case "changed-packet":
				d.state.qualified.state.raw[0] ^= 1
			case "foreign-origin":
				c := *d.state.qualified.state.pair.state.origin.state
				d.state.qualified.state.pair.state.origin.state = &c
			case "closed-output":
				if err := roots.state.outputs[1].Close(); err != nil {
					t.Fatal(err)
				}
			case "closed-source":
				if err := roots.state.source.Close(); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				cancel()
			}
			if _, err := ReadSelectedDerivedProducerPacketPrivateV2(ctx, d, ordinal); err == nil {
				t.Fatal("ASSERT_SELECTED_ACTUAL_CUSTODY_REFUSAL", kind)
			}
		})
	}
	if _, err := (ValidatedBatchPrivateV2{}).ViewPrivateV2(); err == nil {
		t.Fatal("ASSERT_SELECTED_ZERO_REFUSAL")
	}
}
