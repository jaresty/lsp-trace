//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"fmt"
	"testing"
)

func TestScopedExecutionTopologyPrivateRetiredCapabilities(t *testing.T) {
	for _, phase := range []string{"before-store", "before-operation"} {
		for role := 0; role < 6; role++ {
			t.Run(fmt.Sprintf("%s/root-%d", phase, role), func(t *testing.T) {
				f := executionClaimFixture(t)
				o := f.owner(t)
				p := executionTopologyProofPrivate(t, f)
				var store *scopedExecutionStorePrivate
				if phase == "before-operation" {
					var err error
					store, err = newScopedExecutionStorePrivate(context.Background(), o, f.roots, p)
					if err != nil {
						t.Fatal(err)
					}
					defer store.Close()
				}
				topologyUnchangedPrivate(t, f, func() {
					var err error
					if role < 3 {
						err = p.state.inputs[role].Close()
					} else {
						err = f.roots[role-3].Close()
					}
					if err != nil {
						t.Fatal(err)
					}
					if phase == "before-store" {
						s, e := newScopedExecutionStorePrivate(context.Background(), o, f.roots, p)
						if e == nil || s != nil {
							t.Fatal("retired capability reached store")
						}
					} else {
						if _, e := store.Load(context.Background(), o.selector); e == nil {
							t.Fatal("retired capability reached Load")
						}
						if r, e := store.CompareAbsent(context.Background(), o.selector, o.raw); e == nil || r != nil {
							t.Fatal("retired capability reached CAS")
						}
					}
				})
			})
		}
	}
}
