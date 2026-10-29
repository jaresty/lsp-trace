//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func TestProducerPacketPrivateV2NativeCodecProcessAndOccurrences(t *testing.T) {
	withProducerPacketCodecPrivateV2(t, "", nil)
}
func withProducerPacketCodecPrivateV2(t *testing.T, command string, fn func(DerivedProducerPacketPrivateV2, [][]byte, func() int)) {
	t.Helper()
	spec, m, id, child := producerFixturePrivateV2(t, 2, 1, command)
	var incoming atomic.Int32
	child.mu.Lock()
	normal := child.response
	child.response = func(msg lspwire.Message) json.RawMessage {
		if msg.Method == "callHierarchy/incomingCalls" && incoming.Add(1)%2 == 0 {
			return json.RawMessage("[]")
		}
		return normal(msg)
	}
	child.mu.Unlock()
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	var o EnumerationOriginPrivateV2
	var captures []OpaqueSuccessfulV2Capture
	var originals [][]byte
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		var e error
		o, _, e = DiscoverScopePrivateV2(ctx, f, m, id)
		if e != nil {
			return e
		}
		// Owned process-metadata intervention before the ACTUAL producer captures and
		// releases. This tests exact codec custody, not authenticated runtime history.
		o.state.adapter.beforePartialCapturePrivateTest = func(r *acquisition.Result) {
			r.Graph.Invocation.Server.Environment = map[string]string{"PACKET_FIXTURE_TOKEN": "not-a-real-secret"}
			r.Graph.Invocation.EffectiveEnvironment = []string{"PACKET_FIXTURE_TOKEN=not-a-real-secret", "PATH=/owned/frozen-bin"}
			r.Graph.Invocation.WorkingDirectory = "/owned/original-packet-cwd"
			raw, err := json.Marshal(r.Graph)
			if err != nil {
				t.Error(err)
			}
			originals = append(originals, raw)
		}
		for i := 0; i < o.BatchCount(); i++ {
			b, e := o.Batch(i)
			if e != nil {
				return e
			}
			capture, e := AcquireScopeBatchPrivateV2(ctx, m, o, b)
			if e != nil {
				return e
			}
			captures = append(captures, capture)
		}
		return nil
	})
	if err = m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	roots, _, _ := packetRootsPrivateV2(t, f)
	out, err := RetainCompleteScopePrivateV2(context.Background(), f, o, captures, roots)
	if err != nil {
		t.Fatal(err)
	}
	q, err := QualifyProducerPacketPrivateV2(context.Background(), out.VerifiedPair())
	if err != nil {
		t.Fatal(err)
	}
	d, err := DeriveProducerPacketPrivateV2(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	view, err := ReadDerivedProducerPacketPrivateV2(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	traffic := func() int { return len(child.snapshot()) }
	before := traffic()
	if fn != nil {
		fn(d, originals, traffic)
	}
	if traffic() != before {
		t.Fatal("ASSERT_V2_CODEC_NO_PROVIDER_AFTER_CAPTURE")
	}
	sawNull, sawEmpty, sawDuplicate := false, false, false
	for i, batch := range view.Batches {
		if !bytes.Equal(batch.NativeGraph, originals[i]) {
			t.Fatal("ASSERT_PACKET_PROCESS_METADATA_EXACT_NOT_NORMALIZED")
		}
		evidence, e := graphprovenance.ParseSourceWithholdingPrivateV2(batch.SourceArtifact)
		if e != nil {
			t.Fatal(e)
		}
		for _, request := range evidence.Evidence.Acquisition.Requests {
			if request.Method == "callHierarchy/incomingCalls" {
				sawNull = sawNull || bytes.Equal(request.Response, []byte("null"))
				sawEmpty = sawEmpty || bytes.Equal(request.Response, []byte("[]"))
			}
			if request.Method == "callHierarchy/outgoingCalls" {
				var calls []lsp.CallHierarchyOutgoingCall
				if json.Unmarshal(request.Response, &calls) != nil {
					t.Fatal("owned outgoing decode")
				}
				for j := 1; j < len(calls); j++ {
					if reflect.DeepEqual(calls[j-1], calls[j]) {
						sawDuplicate = true
					}
				}
			}
		}
	}
	if !sawNull || !sawEmpty || !sawDuplicate {
		t.Fatal("ASSERT_PACKET_DIRECTIONAL_NULL_EMPTY_DUPLICATE_OCCURRENCES_PRESENT", sawNull, sawEmpty, sawDuplicate)
	}
}
