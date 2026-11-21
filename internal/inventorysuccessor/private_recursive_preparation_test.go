//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

func TestPreparationTwoBatchesOneCursorV1ReadbackControls(t *testing.T) {
	for _, fault := range []string{"RESERVE", "OBSERVE", "replay"} {
		t.Run(fault, func(t *testing.T) {
			ctx := context.Background()
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := publication.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			j, a, d := fixtureV2(t)
			charge := countersV2{Calls: 1, OutputBytes: 1024}
			a.MaxTranche = charge
			a.Ceiling = countersV2{Calls: 3, OutputBytes: 3072}
			a.MaxWindows = 3
			a.MaxWindowNS = uint64(10 * time.Second)
			deadline := time.Now().Add(5 * time.Second)
			a.Expires = deadline.UnixNano()
			sealTestV2(t, &a)
			d.Authorization = a.Identity
			l := verificationLimitsV2{128, 512 << 10, 128}
			k := &kernelV2{root: root}
			initial, err := k.initialize(ctx, j, a, d, time.Now().UnixNano(), l)
			if err != nil {
				t.Fatal(err)
			}
			writes := 0
			k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				receipt, e := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
				if e != nil {
					return receipt, e
				}
				writes++
				var h resourceHeadV2
				if e := parseV2(raw, &h); e != nil {
					t.Fatal(e)
				}
				if h.Event == fault {
					// Return successful CAS unchanged, but make independent verification fail.
					if e := os.Remove(filepath.Join(dir, s)); e != nil {
						t.Fatal(e)
					}
				}
				return receipt, nil
			}
			reads := 0
			read := func(context.Context) ([]source.Record, bool, error) {
				reads++
				return []source.Record{{Name: "witness"}}, true, nil
			}
			run := func(head, id string) ([]source.Record, bool, string, error) {
				return readFundedPreparationBatch(ctx, k, j, head, id, charge, time.Now(), deadline, l, read)
			}
			id := hashLabelV2("first")
			batch, done, next, err := run(initial.Identity, id)
			if fault != "replay" {
				want := 0
				if fault == "OBSERVE" {
					want = 1
				}
				if err == nil || batch != nil || done || next != "" || reads != want || writes != want+1 {
					t.Fatalf("ASSERT_PREPARATION_INDEPENDENT_%s: reads=%d writes=%d result=%v/%v/%s err=%v", fault, reads, writes, batch, done, next, err)
				}
				t.Logf("ASSERT_PREPARATION_INDEPENDENT_%s: PASS", fault)
				return
			}
			if err != nil || len(batch) != 1 || !done || next == "" {
				t.Fatalf("positive control: %v %v %s %v", batch, done, next, err)
			}
			if result, done, replayHead, err := run(next, id); err == nil || result != nil || done || replayHead != "" || reads != 1 || writes != 2 {
				t.Fatalf("ASSERT_PREPARATION_REPLAY_INERT: reads=%d writes=%d err=%v", reads, writes, err)
			}
			if _, _, fresh, err := run(next, hashLabelV2("fresh")); err != nil || fresh == "" || reads != 2 || writes != 4 {
				t.Fatalf("ASSERT_PREPARATION_FRESH_AFFORDABLE: reads=%d writes=%d err=%v", reads, writes, err)
			}
			t.Log("ASSERT_PREPARATION_REPLAY_INERT: PASS; ASSERT_PREPARATION_FRESH_AFFORDABLE: PASS")
		})
	}
}
