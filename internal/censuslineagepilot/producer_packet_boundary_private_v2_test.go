//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func TestProducerPacketPrivateV2ForeignSameBytes(t *testing.T) {
	f, o, captures, _ := packetFixturePrivateV2(t, 1, 0)
	_, other, foreign, _ := packetFixturePrivateV2(t, 1, 0)
	// Owned negative laundering intervention on an ACTUAL independently minted
	// success. Byte equality must not erase the distinct original origin pointer.
	other.state.raw = bytes.Clone(o.state.raw)
	foreign[0].state.origin = bytes.Clone(captures[0].state.origin)
	foreign[0].state.seeds = bytes.Clone(captures[0].state.seeds)
	foreign[0].state.artifact = bytes.Clone(captures[0].state.artifact)
	if !other.valid() || !bytes.Equal(foreign[0].OriginBytes(), captures[0].OriginBytes()) || !bytes.Equal(foreign[0].ArtifactBytes(), captures[0].ArtifactBytes()) {
		t.Fatal("ASSERT_PACKET_FOREIGN_SAME_BYTE_FIXTURE")
	}
	roots, _, _ := packetRootsPrivateV2(t, f)
	calls := 0
	ops := producerPacketRetentionOpsPrivateV2()
	ops.read = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
		calls++
		return nil, os.ErrNotExist
	}
	out, err := retainCompleteScopePrivateV2(context.Background(), f, o, foreign, roots, nil, ops)
	if err == nil || out.VerifiedPair().valid() || calls != 0 {
		t.Fatal("ASSERT_PACKET_FOREIGN_SAME_BYTES_REFUSED_BEFORE_STORAGE", err, calls)
	}
}
func TestProducerPacketPrivateV2ArtifactBoundBeforeStorage(t *testing.T) {
	f, o, captures, _ := packetFixturePrivateV2(t, 1, 0)
	roots, _, _ := packetRootsPrivateV2(t, f)
	original := captures[0].state.artifact
	defer func() { captures[0].state.artifact = original }()
	captures[0].state.artifact = make([]byte, producerPacketArtifactBytesPrivateV2+1)
	calls := 0
	ops := producerPacketRetentionOpsPrivateV2()
	ops.read = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
		calls++
		return nil, os.ErrNotExist
	}
	out, err := retainCompleteScopePrivateV2(context.Background(), f, o, captures, roots, nil, ops)
	if err != ErrProducerPacketOverBoundPrivateV2 || out.Phase() != "NOT_RETAINED_OVER_BOUND" || out.VerifiedPair().valid() || calls != 0 {
		t.Fatal("ASSERT_PACKET_ARTIFACT_ONE_OVER_PREFLIGHT", err, calls, out.Phase())
	}
}
func TestProducerPacketPrivateV2EncodedSizeEquality(t *testing.T) {
	p := ProducerPacketInspectionPrivateV2{FrozenSpec: []byte{}, Files: []ProducerPacketFilePrivateV2{}, Batches: []ProducerPacketBatchPrivateV2{}}
	// Align the metadata length so a base64 payload lands EXACTLY at 16 MiB.
	for i := 0; i < 4; i++ {
		p.Profile = strings.Repeat("x", i)
		size, e := producerPacketEncodedLengthPrivateV2(p)
		if e != nil {
			t.Fatal(e)
		}
		if (producerPacketBytesPrivateV2-size)%4 == 0 {
			p.FrozenSpec = make([]byte, (producerPacketBytesPrivateV2-size)/4*3)
			break
		}
	}
	size, err := producerPacketEncodedLengthPrivateV2(p)
	if err != nil || size != producerPacketBytesPrivateV2 {
		t.Fatal("ASSERT_PACKET_EXACT_ENCODED_SIZE", size, err)
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) != size {
		t.Fatal("ASSERT_PACKET_SIZE_MATCHES_ACTUAL_BASE64_JSON", len(raw), err)
	}
	p.Profile += "x"
	if _, err = producerPacketEncodedLengthPrivateV2(p); err != ErrProducerPacketOverBoundPrivateV2 {
		t.Fatal("ASSERT_PACKET_ONE_OVER_INCLUDES_JSON_OVERHEAD", err)
	}
}
func TestProducerPacketPrivateV2WrapperArrayPreflight(t *testing.T) {
	for _, field := range []string{"files", "batches", "ordered_roles"} {
		t.Run(field, func(t *testing.T) {
			// A compact hostile array must not allocate an arbitrarily large typed slice.
			raw := []byte(`{"` + field + `":[` + strings.Repeat(`{},`, 300) + `{}]}`)
			if _, err := PureParseProducerPacketPrivateV2(raw); err != ErrProducerPacketOverBoundPrivateV2 {
				t.Fatal("ASSERT_PACKET_WRAPPER_ARRAY_PREFLIGHT", field, err)
			}
		})
	}
}
