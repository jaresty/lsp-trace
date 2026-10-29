//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func TestProducerPacketPrivateV2ActualNoncallableDiscovery(t *testing.T) {
	spec, m, id, child := producerFixturePrivateV2(t, 2, 1)
	child.mu.Lock()
	normal := child.response
	child.response = func(msg lspwire.Message) json.RawMessage {
		raw := normal(msg)
		if msg.Method != "textDocument/documentSymbol" {
			return raw
		}
		var symbols []lsp.DocumentSymbol
		if json.Unmarshal(raw, &symbols) != nil {
			t.Fatal("owned symbol decode")
		}
		symbols = append(symbols, lsp.DocumentSymbol{Name: "OwnedNonCallable", Kind: 13, Range: lsp.Range{Start: lsp.Position{Line: 6}, End: lsp.Position{Line: 7}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 6}, End: lsp.Position{Line: 6, Character: 1}}})
		raw, _ = json.Marshal(symbols)
		return raw
	}
	child.mu.Unlock()
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	var origin EnumerationOriginPrivateV2
	var captures []OpaqueSuccessfulV2Capture
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		var e error
		origin, _, e = DiscoverScopePrivateV2(ctx, f, m, id)
		if e != nil {
			return e
		}
		for i := 0; i < origin.BatchCount(); i++ {
			b, e := origin.Batch(i)
			if e != nil {
				return e
			}
			c, e := AcquireScopeBatchPrivateV2(ctx, m, origin, b)
			if e != nil {
				return e
			}
			captures = append(captures, c)
		}
		return nil
	})
	if err = m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	roots, _, _ := packetRootsPrivateV2(t, f)
	out, err := RetainCompleteScopePrivateV2(context.Background(), f, origin, captures, roots)
	if err != nil {
		t.Fatal(err)
	}
	q, err := QualifyProducerPacketPrivateV2(context.Background(), out.VerifiedPair())
	if err != nil {
		t.Fatal("ASSERT_PACKET_ACTUAL_NONCALLABLE_ACCOUNTING", err)
	}
	d, err := DeriveProducerPacketPrivateV2(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadDerivedProducerPacketPrivateV2(context.Background(), d); err != nil {
		t.Fatal(err)
	}
}
func TestProducerPacketPrivateV2DerivedLineageAndCapabilityPins(t *testing.T) {
	for _, kind := range []string{"copied-qualified", "changed-report", "copied-derived", "closed-source", "closed-output", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			f, o, captures, _ := packetFixturePrivateV2(t, 1, 0)
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "copied-qualified":
				copy := *q.state
				d.state.qualified = QualifiedProducerPacketPrivateV2{&copy}
			case "changed-report":
				q.state.report[0] ^= 1
			case "copied-derived":
				copy := *d.state
				d = DerivedProducerPacketPrivateV2{&copy}
			case "closed-source":
				if err = roots.state.source.Close(); err != nil {
					t.Fatal(err)
				}
			case "closed-output":
				if err = roots.state.outputs[1].Close(); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				cancel()
			}
			if _, err = ReadDerivedProducerPacketPrivateV2(ctx, d); err == nil {
				t.Fatal("ASSERT_PACKET_DERIVED_REQUIRES_QUALIFIED_LINEAGE_AND_PINS", kind)
			}
		})
	}
}
