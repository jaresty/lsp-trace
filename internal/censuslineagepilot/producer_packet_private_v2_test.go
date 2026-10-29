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
	"testing"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/publication"
)

func packetFixturePrivateV2(t *testing.T, nom, con int, server ...string) (FrozenScopePrivateV2, EnumerationOriginPrivateV2, []OpaqueSuccessfulV2Capture, func() int) {
	t.Helper()
	return packetFixtureConfiguredPrivateV2(t, nom, con, false, server...)
}

func packetFixtureConfiguredPrivateV2(t *testing.T, nom, con int, matched bool, server ...string) (FrozenScopePrivateV2, EnumerationOriginPrivateV2, []OpaqueSuccessfulV2Capture, func() int) {
	t.Helper()
	spec, m, id, child := producerFixtureConfiguredPrivateV2(t, nom, con, matched, server...)
	if err := os.Chmod(spec.Root, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	var o EnumerationOriginPrivateV2
	var captures []OpaqueSuccessfulV2Capture
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		var e error
		o, _, e = DiscoverScopePrivateV2(ctx, f, m, id)
		if e != nil {
			return e
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
	return f, o, captures, func() int { return len(child.snapshot()) }
}
func packetRootsPrivateV2(t *testing.T, f FrozenScopePrivateV2) (ProducerPacketRootsPrivateV2, string, string) {
	t.Helper()
	p, b, pp, bp := retentionRoots(t)
	source, err := publication.OpenSourceProtectionRootPrivateV2(context.Background(), f.state.spec.Root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := source.Close(); e != nil {
			t.Error(e)
		}
	})
	roots, err := NewProducerPacketRootsPrivateV2(context.Background(), [2]*publication.ContentRetentionRoot{p, b}, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	return roots, pp, bp
}
func TestProducerPacketPrivateV2Connected(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}, {5, 2}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			f, o, captures, traffic := packetFixturePrivateV2(t, counts[0], counts[1])
			before := traffic()
			roots, _, _ := packetRootsPrivateV2(t, f)
			ops := producerPacketRetentionOpsPrivateV2()
			read, publish := ops.read, ops.publish
			reads, writes := 0, 0
			ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
				reads++
				return read(ctx, r, id, n)
			}
			ops.publish = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
				writes++
				return publish(ctx, r, id, raw)
			}
			out, err := retainCompleteScopePrivateV2(context.Background(), f, o, captures, roots, nil, ops)
			if err != nil {
				t.Fatal("retain", err)
			}
			if !out.VerifiedPair().valid() || writes != 2 || reads != 6 {
				t.Fatal("ASSERT_PACKET_EXACT_PAIR_PROTOCOL", writes, reads, out.Phase())
			}
			q, err := qualifyProducerPacketPrivateV2(context.Background(), out.VerifiedPair(), ops)
			if err != nil {
				t.Fatal("qualify", err)
			}
			if reads != 8 || writes != 2 {
				t.Fatal("ASSERT_PACKET_QUALIFIER_INDEPENDENT_READS", reads, writes)
			}
			derived, err := DeriveProducerPacketPrivateV2(context.Background(), q)
			if err != nil {
				t.Fatal("derive", err)
			}
			view, err := ReadDerivedProducerPacketPrivateV2(context.Background(), derived)
			if err != nil {
				t.Fatal("read", err)
			}
			p, err := PureParseProducerPacketPrivateV2(view.Packet)
			if err != nil || len(p.Batches) != o.BatchCount() || len(p.Files) != counts[0]+counts[1] {
				t.Fatal("ASSERT_PACKET_COMPLETE_ORIGIN", err)
			}
			var report ProducerPacketQualificationReportPrivateV2
			if e := json.Unmarshal(view.QualificationReport, &report); e != nil || report.Authority != 0 || report.Accepted || report.Feature != "UNRESOLVED" || report.Completeness != "UNKNOWN" || report.Absence != "PROHIBITED" || report.Status != "PRIVATE_PACKET_QUALIFIED_ONLY" || report.BatchCount != o.BatchCount() || report.PacketSHA256 != Digest(view.Packet) || report.PolicySHA256 != Digest([]byte(report.ProfilePolicy)) {
				t.Fatal("ASSERT_PACKET_REPORT_PRIVATE_ONLY", e)
			}
			if report.CopyObservations[0].RootPath == report.CopyObservations[1].RootPath || report.CopyObservations[0].ObjectByteLength != uint64(len(view.Packet)) || report.CopyObservations[1].ObjectByteLength != uint64(len(view.Packet)) || report.CopyObservations[0].ObjectDevice == report.CopyObservations[1].ObjectDevice && report.CopyObservations[0].ObjectInode == report.CopyObservations[1].ObjectInode {
				t.Fatal("ASSERT_PACKET_REPORT_ACTUAL_DISTINCT_ROOT_COPY_OBSERVATIONS")
			}
			if spent := out.BudgetSpent(); spent.LogicalBytes == 0 || spent.LogicalBytes > producerPacketLogicalPrivateV2 || spent.ReadHashBytes > producerPacketReadHashPrivateV2 || spent.ValidationWork > producerPacketWorkPrivateV2 {
				t.Fatal("ASSERT_PACKET_FINITE_RESERVATION_REPORT", spent)
			}
			for _, capture := range captures {
				i := capture.state.ordinal
				original, e := graphprovenance.ParseSourceWithholdingPrivateV2(capture.ArtifactBytes())
				if e != nil || !bytes.Equal(original.Evidence.GraphBytes, view.Batches[i].NativeGraph) {
					t.Fatal("ASSERT_PACKET_NATIVE_GRAPH_BYTES_UNCHANGED", e)
				}
				if !bytes.Equal(p.Batches[i].SourceArtifact, capture.ArtifactBytes()) || !bytes.Equal(p.Batches[i].Origin, capture.OriginBytes()) {
					t.Fatal("ASSERT_PACKET_ORIGINAL_BYTES")
				}
			}
			if traffic() != before {
				t.Fatal("ASSERT_PACKET_NO_PROVIDER_AFTER_CAPTURE")
			}
			detached := out.CopyReceipts()
			detached[0].Receipt.Digest = "wrong"
			if out.CopyReceipts()[0].Receipt.Digest == "wrong" {
				t.Fatal("ASSERT_PACKET_DETACHED_RECEIPTS")
			}
			view.Packet[0] = '!'
			view.Batches[0].NativeGraph[0] = '!'
			again, err := ReadDerivedProducerPacketPrivateV2(context.Background(), derived)
			if err != nil || again.Packet[0] == '!' || again.Batches[0].NativeGraph[0] == '!' {
				t.Fatal("ASSERT_PACKET_DETACHED_DERIVED", err)
			}
			t.Log("ASSERT_PACKET_CONNECTED_OFFLINE: PASS")
		})
	}
}
func TestProducerPacketPrivateV2Preflight(t *testing.T) {
	f, o, captures, _ := packetFixturePrivateV2(t, 2, 1)
	for _, kind := range []string{"missing", "duplicate", "extra", "foreign-pointer", "wrong-seeds", "unreleased", "zero-frozen", "zero-origin", "budget", "work", "wrong-source"} {
		t.Run(kind, func(t *testing.T) {
			roots, pp, bp := packetRootsPrivateV2(t, f)
			cc := append([]OpaqueSuccessfulV2Capture(nil), captures...)
			ff, oo := f, o
			var limits []ProducerPacketLimitsPrivateV2
			switch kind {
			case "missing":
				cc = cc[:len(cc)-1]
			case "duplicate":
				cc[1] = cc[0]
			case "extra":
				cc = append(cc, cc[0])
			case "foreign-pointer":
				copyState := *cc[0].state
				copyState.enumeration = &enumerationOriginStatePrivateV2{}
				copyState.self = &copyState
				cc[0] = OpaqueSuccessfulV2Capture{&copyState}
			case "wrong-seeds":
				copyState := *cc[0].state
				copyState.seeds = []byte("wrong")
				copyState.self = &copyState
				cc[0] = OpaqueSuccessfulV2Capture{&copyState}
			case "unreleased":
				copyState := *cc[0].state
				copyState.releasedObserved = false
				copyState.self = &copyState
				cc[0] = OpaqueSuccessfulV2Capture{&copyState}
			case "zero-frozen":
				ff = FrozenScopePrivateV2{}
			case "zero-origin":
				oo = EnumerationOriginPrivateV2{}
			case "budget":
				limits = []ProducerPacketLimitsPrivateV2{{LogicalBytes: 1}}
			case "work":
				limits = []ProducerPacketLimitsPrivateV2{{ValidationWork: 1}}
			case "wrong-source":
				other := t.TempDir()
				other, _ = filepath.EvalSymlinks(other)
				source, e := publication.OpenSourceProtectionRootPrivateV2(context.Background(), other)
				if e != nil {
					t.Fatal(e)
				}
				defer source.Close()
				var e2 error
				roots, e2 = NewProducerPacketRootsPrivateV2(context.Background(), roots.state.outputs, source, nil)
				if e2 != nil {
					t.Fatal(e2)
				}
			}
			calls := 0
			ops := producerPacketRetentionOpsPrivateV2()
			ops.read = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
				calls++
				return nil, errors.New("unexpected read")
			}
			ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
				calls++
				return nil, errors.New("unexpected publish")
			}
			out, err := retainCompleteScopePrivateV2(context.Background(), ff, oo, cc, roots, limits, ops)
			if err == nil || out.VerifiedPair().valid() || calls != 0 {
				t.Fatal("ASSERT_PACKET_PREFLIGHT_NO_STORAGE", kind, err, calls)
			}
			for _, path := range []string{pp, bp} {
				entries, e := os.ReadDir(path)
				if e != nil || len(entries) != 0 {
					t.Fatal("ASSERT_PACKET_NO_PREFLIGHT_COPIES", e)
				}
			}
		})
	}
}
func TestProducerPacketPrivateV2PureParserNeverSeal(t *testing.T) {
	for _, value := range []any{ProducerPacketRootsPrivateV2{}, VerifiedProducerPacketPairPrivateV2{}, QualifiedProducerPacketPrivateV2{}, DerivedProducerPacketPrivateV2{}} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatal("ASSERT_PACKET_OPAQUE_NO_JSON")
		}
	}
	if _, err := QualifyProducerPacketPrivateV2(context.Background(), VerifiedProducerPacketPairPrivateV2{}); err == nil {
		t.Fatal("ASSERT_PACKET_ZERO_PAIR_REFUSED")
	}
	if _, err := DeriveProducerPacketPrivateV2(context.Background(), QualifiedProducerPacketPrivateV2{}); err == nil {
		t.Fatal("ASSERT_PACKET_ZERO_QUALIFIED_REFUSED")
	}
	if _, err := ReadDerivedProducerPacketPrivateV2(context.Background(), DerivedProducerPacketPrivateV2{}); err == nil {
		t.Fatal("ASSERT_PACKET_ZERO_DERIVED_REFUSED")
	}
	if _, err := PureParseProducerPacketPrivateV2([]byte(`{}`)); err == nil {
		t.Fatal("ASSERT_PACKET_DTO_NOT_QUALIFICATION")
	}
}
