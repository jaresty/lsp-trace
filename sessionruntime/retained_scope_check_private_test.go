package sessionruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestCheckRetainedScopePrivateAllMembers(t *testing.T) {
	for i := 0; i < 6; i++ {
		for _, kind := range []string{"version", "language", "generation"} {
			t.Run(fmt.Sprintf("%d/%s", i, kind), func(t *testing.T) {
				m, s, c, members := retainedScopeFixture(t)
				t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
				g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
				if f != "" {
					t.Fatal(f)
				}
				if _, err := json.Marshal(g); err == nil {
					t.Fatal("guard serialized")
				}
				if f = m.CheckRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g); f != "" {
					t.Fatal(f)
				}
				before, _ := c.snapshot()
				m.mu.Lock()
				r := m.sessions[s.SessionID]
				d := r.documents[members[i].URI]
				switch kind {
				case "version":
					d.version++
				case "language":
					d.languageID = "other"
				case "generation":
					d.supply.Generation++
				}
				r.documents[members[i].URI] = d
				m.mu.Unlock()
				if f = m.CheckRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g); f == "" {
					t.Fatal("drift accepted")
				}
				after, _ := c.snapshot()
				if len(after) != len(before) {
					t.Fatal("read-only check wrote")
				}
			})
		}
	}
}
