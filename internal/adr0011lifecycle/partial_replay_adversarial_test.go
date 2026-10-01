package adr0011lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fencedPartialFixture(t *testing.T) (*syntheticRoot, []byte, []byte) {
	t.Helper()
	l, schema, policy := partialFixture(t)
	if err := l.provisionPartial(schema, policy); err != nil {
		t.Fatal(err)
	}
	state, err := l.partialSnapshot(schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.partialFence(schema, policy, state); err != nil {
		t.Fatal(err)
	}
	return l, schema, policy
}
func TestPartialReplayRejectsSubstitutedHostSlots(t *testing.T) {
	for _, kind := range []string{"prepare", "commit"} {
		t.Run(kind, func(t *testing.T) {
			l, schema, policy := fencedPartialFixture(t)
			host, err := l.partialEnumerate(l.anchor, true)
			if err != nil {
				t.Fatal(err)
			}
			old := host[kind][2]
			record, err := l.partialRead(l.anchor, old, kind)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(l.anchor.Path(), old), filepath.Join(filepath.Dir(l.anchor.Path()), "saved")); err != nil {
				t.Fatal(err)
			}
			record.Previous = "sha256:substituted"
			if _, err := l.partialPublish(l.anchor, record); err != nil {
				t.Fatal(err)
			}
			if _, err := l.partialSnapshot(schema, policy); err == nil {
				t.Fatal("substituted host selector admitted")
			}
		})
	}
}
func TestPartialReplayRejectsOriginalByteMutation(t *testing.T) {
	l, schema, policy := fencedPartialFixture(t)
	host, err := l.partialEnumerate(l.anchor, true)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(l.anchor.Path(), host["commit"][2])
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 'x'
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.partialSnapshot(schema, policy); err == nil {
		t.Fatal("mutated original host bytes admitted")
	}
}
func TestPartialReplayRejectsRootIdentityReplacement(t *testing.T) {
	l, schema, policy := fencedPartialFixture(t)
	original := l.root.Path()
	moved := original + "-moved"
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := l.partialSnapshot(schema, policy); err == nil || !strings.Contains(err.Error(), "replaced") {
		t.Fatalf("substituted root admitted: %v", err)
	}
}
func TestPartialReplayRejectsOrphanCommit(t *testing.T) {
	l, schema, policy := fencedPartialFixture(t)
	extra, err := l.partialBase("commit", 4, schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.partialPublish(l.anchor, extra); err != nil {
		t.Fatal(err)
	}
	if _, err := l.partialSnapshot(schema, policy); err == nil {
		t.Fatal("orphan commit admitted")
	}
}
func TestPartialReplayRejectsMissingCommitAndFork(t *testing.T) {
	t.Run("missing_commit", func(t *testing.T) {
		l, schema, policy := fencedPartialFixture(t)
		host, err := l.partialEnumerate(l.anchor, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(l.anchor.Path(), host["commit"][2]), filepath.Join(filepath.Dir(l.anchor.Path()), "saved-commit")); err != nil {
			t.Fatal(err)
		}
		if _, err := l.partialSnapshot(schema, policy); err == nil {
			t.Fatal("unverified transition admitted")
		}
	})
	t.Run("fork", func(t *testing.T) {
		l, schema, policy := fencedPartialFixture(t)
		host, err := l.partialEnumerate(l.anchor, true)
		if err != nil {
			t.Fatal(err)
		}
		r, err := l.partialRead(l.anchor, host["prepare"][2], "prepare")
		if err != nil {
			t.Fatal(err)
		}
		r.Previous = "sha256:fork"
		if _, err := l.partialPublish(l.anchor, r); err != nil {
			t.Fatal(err)
		}
		if _, err := l.partialSnapshot(schema, policy); err == nil {
			t.Fatal("forked prepare admitted")
		}
	})
}
func TestPartialPinsRejectDifferentBytes(t *testing.T) {
	l, schema, policy := partialFixture(t)
	schema = append(schema, ' ')
	if err := l.provisionPartial(schema, policy); err == nil {
		t.Fatal("wrong selected schema bytes admitted")
	}
}
