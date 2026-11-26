//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticPublicationGates(t *testing.T) {
	for _, kind := range []string{"pair", "same-root", "foreign-protected", "mode"} {
		t.Run(kind, func(t *testing.T) {
			outs, ins := postPublicationRoots(t)
			_, otherIns := postPublicationRoots(t)
			ins = append(ins, otherIns[0])
			h := automaticTestOwner(t, 1)
			for _, root := range ins {
				path, id, e := root.CanonicalIdentityPrivateV2()
				if e != nil {
					t.Fatal(e)
				}
				h.roots = append(h.roots, producerRootBindingPrivateV2{path, id.Device, id.Inode})
			}
			i, v := automaticTestPair()
			a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "same-root" {
				outs[1] = outs[0]
			}
			if kind == "foreign-protected" {
				ins = ins[:1]
			}
			if kind == "mode" {
				path, _, e := ins[0].CanonicalIdentityPrivateV2()
				if e != nil {
					t.Fatal(e)
				}
				if e = os.Chmod(filepath.Join(filepath.Dir(path), "primary"), 0755); e != nil {
					t.Fatal(e)
				}
			}
			out, e := RetainAutomaticPrivateV2(context.Background(), h, a, outs, ins)
			if kind != "pair" {
				if e == nil {
					t.Fatal("ASSERT_AUTO_PUBLICATION_GATE: FAIL", kind)
				}
				t.Log("ASSERT_AUTO_PUBLICATION_GATE: PASS")
				return
			}
			if e != nil || out.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" {
				t.Fatal("ASSERT_AUTO_PAIRED_BYTES: FAIL", e)
			}
			for _, root := range outs {
				b, e := root.ReadPostPreparationPrivateV2(context.Background(), out.EnvelopeSHA256, int64(out.ByteLength))
				if e != nil || !bytes.Equal(b, out.EnvelopeBytes()) {
					t.Fatal("ASSERT_AUTO_PAIRED_BYTES: FAIL", e)
				}
			}
			var envelope postPublicationEnvelopePrivateV2
			if json.Unmarshal(out.EnvelopeBytes(), &envelope) != nil || envelope.Inventory == nil || envelope.HumanInventory != a.Human() || len(envelope.Ledger.Leads) != 1 {
				t.Fatal("ASSERT_AUTO_PAIRED_BYTES: FAIL")
			}
			t.Log("ASSERT_AUTO_PAIRED_BYTES: PASS")
		})
	}
}
