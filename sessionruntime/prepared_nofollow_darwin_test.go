//go:build darwin

package sessionruntime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/session"
)

func TestPreparedNoFollowRejectsSeedAndStaleGeneration(t *testing.T) {
	m, req, _, writer := supplyFixture(t, []byte("package fixture\n"))
	req.PreparedNoFollow = true
	m.mu.Lock()
	m.sessions[req.SessionID].seedSources[req.URI] = []byte("package substituted\n")
	m.mu.Unlock()
	before := writer.Len()
	got := m.PrepareDocument(context.Background(), req)
	if got.Failure != DocumentSupplyUnavailable || got.Supply != nil || writer.Len() != before {
		t.Fatalf("ASSERT_NOFOLLOW_SEED_CUSTODY_REJECT: failure=%v supply=%t", got.Failure, got.Supply != nil)
	}
	req.Generation++
	got = m.PrepareDocument(context.Background(), req)
	if got.Failure != session.StaleGeneration || got.Supply != nil || writer.Len() != before {
		t.Fatalf("ASSERT_NOFOLLOW_GENERATION_REJECT: failure=%v supply=%t", got.Failure, got.Supply != nil)
	}
}

func TestPreparedNoFollowDescriptorWalk(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(filepath.Join(root, "parent"), 0700); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(root, "parent", "source.go")
	if err := os.WriteFile(leaf, []byte("package x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := readPreparedNoFollow(root, leaf); err != nil || string(got) != "package x\n" {
		t.Fatalf("ASSERT_NOFOLLOW_SAME_LEAF: %q %v", got, err)
	}
	for _, tc := range []struct{ name, root, path string }{
		{"root", filepath.Join(base, "root-link"), filepath.Join(base, "root-link", "parent", "source.go")},
		{"parent", root, filepath.Join(root, "parent-link", "source.go")},
		{"leaf", root, filepath.Join(root, "parent", "leaf-link.go")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var target string
			switch tc.name {
			case "root":
				target = root
			case "parent":
				target = filepath.Join(root, "parent")
			default:
				target = leaf
			}
			var link string
			switch tc.name {
			case "root":
				link = tc.root
			case "parent":
				link = filepath.Join(root, "parent-link")
			default:
				link = tc.path
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			if _, err := readPreparedNoFollow(tc.root, tc.path); err == nil {
				t.Fatal("ASSERT_NOFOLLOW_SYMLINK_REJECT")
			}
		})
	}
	if err := os.WriteFile(filepath.Join(root, "empty.go"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPreparedNoFollow(root, filepath.Join(root, "empty.go")); err == nil {
		t.Fatal("ASSERT_NOFOLLOW_ZERO_REJECT")
	}
	if _, err := readPreparedNoFollow(root, filepath.Join(base, "other.go")); err == nil {
		t.Fatal("ASSERT_NOFOLLOW_OUTSIDE_REJECT")
	}
}
