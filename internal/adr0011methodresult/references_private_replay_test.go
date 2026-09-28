package adr0011methodresult

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
)

func TestReplayPrivateAttachedJournal(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	for _, raw := range []string{"null", "[]", "[" + attachedLocation + "]"} {
		t.Run(raw, func(t *testing.T) {
			root, dir := journalRoot(t)
			identity := privateIdentity(key, []byte(raw))
			_, final, e := RunPrivateAttachedReferences(root, key, json.RawMessage(raw), identity)
			if e != nil {
				t.Fatal(e)
			}
			if e = ReplayPrivateAttachedJournal(root, []byte(raw), key, identity, final); e != nil {
				t.Fatal("ASSERT_PURE_REPLAY", e)
			}
			before, e := os.ReadDir(dir)
			if e != nil {
				t.Fatal(e)
			}
			for name, changed := range map[string]string{"duplicate": `[{"uri":"file:///a","uri":"file:///a","range":{}}]`, "invalid-location": `[{"uri":"file:///a"}]`, "different-count": "[" + attachedLocation + "," + attachedLocation + "]", "wrong-shape": "{}", "overflow": "[" + strings.Repeat("null,", 1000) + "null]"} {
				t.Run(name, func(t *testing.T) {
					other := identity
					other.RawDigest = chainDigest([]byte(changed))
					if ReplayPrivateAttachedJournal(root, []byte(changed), key, other, final) == nil {
						t.Fatal("ASSERT_REPLAY_REJECT")
					}
				})
			}
			after, e := os.ReadDir(dir)
			if e != nil || len(after) != len(before) {
				t.Fatal("ASSERT_REPLAY_NO_WRITES")
			}
			if len(before) > 1 {
				if e = os.Remove(filepath.Join(dir, before[0].Name())); e != nil {
					t.Fatal(e)
				}
				if ReplayPrivateAttachedJournal(root, []byte(raw), key, identity, final) == nil {
					t.Fatal("ASSERT_REPLAY_MISSING_PREFIX")
				}
			}
		})
	}
}
