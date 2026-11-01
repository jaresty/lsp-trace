//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Swap the path only AFTER validating the opened root. Both bundle families
// must read that root, including registered metadata, parts and reply.
func TestPinRepairRecoveryOpenedRoot(t *testing.T) {
	for _, nav := range []bool{false, true} {
		t.Run(map[bool]string{false: "cohort", true: "navigation"}[nav], func(t *testing.T) {
			c, items := cohortInputs(t)
			m := repairMailbox(t)
			m.ExchangeTimeout = time.Millisecond
			var claim []byte
			if nav {
				claim = mustJSON(bindNavigation(c, nil, nil, "interpreter", 1))
			} else {
				claim = mustJSON(bindCohortTask(c, items[:1], "interpreter", "PROPOSE", 1))
			}
			if _, e := m.ExchangeCohort(context.Background(), claim); e != context.DeadlineExceeded {
				t.Fatal("publish fixture", e)
			}
			original := []byte("original physical bytes")
			if e := os.WriteFile(filepath.Join(m.Directory, "reply-00000001.json"), original, 0600); e != nil {
				t.Fatal(e)
			}
			root, e := mailboxStore(m.Directory)
			if e != nil {
				t.Fatal(e)
			}
			defer root.close()
			pin, e := mailboxPinRoot(c, m.Directory, root)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.Rename(m.Directory, m.Directory+"-original"); e != nil {
				t.Fatal(e)
			}
			if e = PrepareMailbox(m.Directory); e != nil {
				t.Fatal(e)
			}
			for _, role := range []string{"interpreter", "reviewer"} {
				if e = MarkCohortReady(m.Directory, Ready{CohortProtocol, role, "replacement-" + role, hash([]byte(CohortRolePrompt(role)))}); e != nil {
					t.Fatal(e)
				}
			}
			entries, e := os.ReadDir(m.Directory + "-original")
			if e != nil {
				t.Fatal(e)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "request-") || strings.HasPrefix(entry.Name(), "part-") {
					raw, e := os.ReadFile(filepath.Join(m.Directory+"-original", entry.Name()))
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(filepath.Join(m.Directory, entry.Name()), raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			if e = os.WriteFile(filepath.Join(m.Directory, "reply-00000001.json"), []byte("foreign"), 0600); e != nil {
				t.Fatal(e)
			}
			s, e := openCohortStore(c)
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			reply, e := observedCohortReplyRoot(s, c, 1, claim, nil, os.ErrNotExist, pin, root)
			if e != nil || !bytes.Equal(reply, original) {
				t.Fatalf("ASSERT_PIN_RECOVERY_VALIDATED_ROOT_ONLY: reply=%q err=%v", reply, e)
			}
		})
	}
}

type pinCloseBackend struct {
	CohortMailbox
	seen **store
	mode string
}

func (b pinCloseBackend) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	bound, ok := ctx.Value(cohortPinnedMailboxKey{}).(*cohortPinnedMailbox)
	if !ok {
		return nil, ErrInvalid
	}
	*b.seen = bound.root
	if b.mode == "panic" {
		panic("controlled process loss")
	}
	if b.mode == "cancelled" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		cancel()
	}
	return b.CohortMailbox.ExchangeCohort(ctx, raw)
}
func TestPinRepairRootLifetime(t *testing.T) {
	for _, mode := range []string{"deadline", "cancelled", "panic"} {
		t.Run(mode, func(t *testing.T) {
			c, items := cohortInputs(t)
			m := repairMailbox(t)
			m.ExchangeTimeout = time.Millisecond
			var root *store
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			func() {
				defer func() {
					if v := recover(); v != nil && mode != "panic" {
						t.Fatal(v)
					}
				}()
				RunCohort(ctx, c, items, pinCloseBackend{m, &root, mode})
			}()
			if root == nil {
				t.Fatal("root lifetime fixture did not exchange")
			}
			if _, e := root.root.Stat("."); e == nil {
				t.Fatal("ASSERT_PIN_OPENED_ROOT_CLOSED")
			}
		})
	}
}
