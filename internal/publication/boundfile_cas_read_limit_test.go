//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareAndReplaceBoundFilePredecessorReadBound(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		size                    int
		absent, grow, excessive bool
		wantLimit               int64
	}{
		{name: "small", size: 17, wantLimit: 17},
		{name: "head-limit", size: 64 << 10, wantLimit: 64 << 10},
		{name: "generic-larger-record", size: 96 << 10, wantLimit: 96 << 10},
		{name: "absent-but-existing", size: 96 << 10, absent: true, wantLimit: 1},
		{name: "replaced-after-preflight", size: 64 << 10, grow: true, wantLimit: 64 << 10},
		{name: "untrusted-large-length", size: 17, excessive: true, wantLimit: 1 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, root := boundRoot(t)
			ctx := context.Background()
			initial := bytes.Repeat([]byte("a"), tc.size)
			if r, err := CompareAndReplaceBoundFile(ctx, root, "current.json", BoundFilePredecessor{Absent: true}, initial, func([]byte) error { return nil }); err != nil || r == nil || !r.Committed {
				t.Fatalf("initial publication: receipt=%+v err=%v", r, err)
			}
			pred := BoundFilePredecessor{Selector: "current.json", Digest: DigestForTest(initial), ByteLength: uint64(len(initial)), Absent: tc.absent}
			visible := initial
			if tc.grow {
				visible = bytes.Repeat([]byte("b"), 96<<10)
				if err := os.WriteFile(filepath.Join(dir, "current.json"), visible, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.excessive {
				pred.ByteLength = math.MaxUint64
			}
			var limit int64
			reads, verified := 0, 0
			replacement := []byte("replacement")
			receipt, err := compareAndReplaceBoundFile(ctx, root, "current.json", pred, replacement, func(raw []byte) error {
				verified++
				if !bytes.Equal(raw, replacement) {
					return errors.New("wrong replacement")
				}
				return nil
			}, func(selector string, max int64) ([]byte, error) {
				reads++
				limit = max
				return root.ReadSelectorWithAbsence(selector, max)
			})
			if reads != 1 || limit != tc.wantLimit {
				t.Fatalf("ASSERT_CAS_PREDECESSOR_READ_BOUND reads=%d got=%d want=%d", reads, limit, tc.wantLimit)
			}
			refused := tc.absent || tc.grow || tc.excessive
			if refused {
				if !errors.Is(err, ErrStalePredecessor) || (receipt != nil && receipt.Committed) || verified != 0 {
					t.Fatalf("stale predecessor progressed: receipt=%+v err=%v verified=%d", receipt, err, verified)
				}
				got, readErr := os.ReadFile(filepath.Join(dir, "current.json"))
				if readErr != nil || !bytes.Equal(got, visible) {
					t.Fatal("refusal changed current record", readErr)
				}
			} else if err != nil || receipt == nil || !receipt.Committed || verified == 0 {
				t.Fatalf("valid CAS refused: receipt=%+v err=%v verified=%d", receipt, err, verified)
			}
		})
	}
}
