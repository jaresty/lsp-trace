//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"reflect"
	"sync"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

func TestSelectedProjectionPreparationPrivateV2OwnedMetadata(t *testing.T) {
	pilot.WithOwnedMatchedProducerPacketPrivateV2(t, 2, 1, false, func(proof pilot.DerivedProducerPacketPrivateV2, traffic func() int) {
		ctx := context.Background()
		before := traffic()
		p, f := derived.ProjectPrivateV2(ctx, proof, 0)
		if f != nil {
			t.Fatal(f)
		}
		p2, f := derived.ProjectPrivateV2(ctx, proof, 0)
		if f != nil {
			t.Fatal(f)
		}
		e, err := p.PreparationEvidencePrivateV2()
		if err != nil {
			t.Fatal(err)
		}
		e2, err := p2.PreparationEvidencePrivateV2()
		if err != nil || !reflect.DeepEqual(e, e2) {
			t.Fatal("ASSERT_SAME_BYTE_FOREIGN_PROJECTION_FIXTURE", err)
		}
		a, _, err := derived.InstabilityComputePrivateV2(ctx, p, []uint64{42, 43})
		if err != nil {
			t.Fatal(err)
		}
		in := []prep.InputPrivateV2{{Projection2: p, Assessment2: a}}
		original, err := prep.PreparePrivateV2(ctx, in, prepLimitsV2())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := prep.PreparePrivateV2(ctx, []prep.InputPrivateV2{{Projection2: p2, Assessment2: a}}, prepLimitsV2()); err == nil {
			t.Fatal("ASSERT_SAME_BYTE_FOREIGN_PROJECTION_ASSESSMENT_REFUSED")
		}
		meta, err := p.ValidatedPreparationMetadataPrivateV2()
		if err != nil {
			t.Fatal(err)
		}
		meta.Packet[0] ^= 1
		meta.QualificationReport[0] ^= 1
		meta.Batch.SourceArtifact[0] ^= 1
		meta.Source.Policy.Members[0].Role = "CONTEXT_ONLY"
		meta.Source.Evidence.Captures[0].Content[0] ^= 1
		meta.Source.Evidence.Acquisition.Graph.Nodes[0].Name = "changed"
		meta.Source.Evidence.Acquisition.Request.Root.ID = "changed"
		meta.Discovery.Targets[0].Name = "changed"
		meta.BatchDiscovery[0].Name = "changed"
		meta.Files[0].Body[0] ^= 1
		e.Packet[0] ^= 1
		e.SourceArtifact[0] ^= 1
		e.OriginalNativeV3[0] ^= 1
		again, err := p.ValidatedPreparationMetadataPrivateV2()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(meta.Packet, again.Packet) || again.Source.Evidence.Acquisition.Graph.Nodes[0].Name == "changed" {
			t.Fatal("ASSERT_PREPARATION_METADATA_DETACHED")
		}
		after, err := prep.PreparePrivateV2(ctx, in, prepLimitsV2())
		if err != nil || !bytes.Equal(original.CanonicalBytes(), after.CanonicalBytes()) {
			t.Fatal("ASSERT_PREPARATION_METADATA_MUTATION_NO_EFFECT", err)
		}
		// Independent detached views can be mutated concurrently; the shared validated
		// snapshot is immutable and contains no caller-exposed mutable containers.
		var wg sync.WaitGroup
		for j := 0; j < 4; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 3; k++ {
					m, err := p.ValidatedPreparationMetadataPrivateV2()
					if err != nil {
						t.Error(err)
						return
					}
					m.Packet[0] ^= 1
					m.Source.Evidence.Acquisition.Graph.Nodes[0].Name = "concurrent"
				}
			}()
		}
		wg.Wait()
		if traffic() != before {
			t.Fatal("ASSERT_SELECTED_NO_PROVIDER_IO")
		}
		t.Log("ASSERT_SAME_BYTE_FOREIGN_PROJECTION_ASSESSMENT_REFUSED: PASS; ASSERT_PREPARATION_METADATA_MUTATION_NO_EFFECT: PASS; ASSERT_SELECTED_NO_PROVIDER_IO: PASS")
	})
}
