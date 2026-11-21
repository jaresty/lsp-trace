package inventorysuccessor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// Accounting-only construction. The real normal-route test supplies original
// controlled acquisition; this fixture does not claim native custody evidence.
func TestFolderControllerAdmission(t *testing.T) {
	for _, kind := range []string{"one-shot", "expired", "over-budget", "closed-root"} {
		t.Run(kind, func(t *testing.T) {
			originalPath, rootPath := t.TempDir(), t.TempDir()
			if err := os.Chmod(originalPath, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(rootPath, 0700); err != nil {
				t.Fatal(err)
			}
			original, e := publication.OpenRoot(originalPath)
			if e != nil {
				t.Fatal(e)
			}
			defer original.Close()
			root, e := publication.OpenRoot(rootPath)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			deadline := time.Now().Add(5 * time.Second)
			if kind == "expired" {
				deadline = time.Now().Add(-time.Second)
			}
			s := &privateDiscoveryOperationV2{owner: &PrivateDiscoveryOwnerV2{k: &kernelV2{root: original}}, completed: true, attempt: hashLabelV2("test"), binding: PrivateDiscoveryBindingV2{Revision: "fixture", DenominatorDigest: hashLabelV2("denominator"), Deadline: deadline}, observation: sessionruntime.PrivateDiscoveryObservation{CleanupJoined: true}}
			op := PrivateDiscoveryOperationV2{state: s}
			b := countersV2{Calls: 2, ActiveNS: uint64(time.Second)}.vector()
			if kind == "over-budget" {
				b[9] = 17
			}
			if kind == "closed-root" {
				root.Close()
			}
			calls := 0
			consume := func(got [14]uint64, producer string) error {
				calls++
				if got != b || !hashV2(producer) {
					t.Fatal("ASSERT_ADMITTED_BINDING", got, producer)
				}
				return nil
			}
			err := WithLocalFolderControllerV1(context.Background(), root, op, b, consume)
			if kind == "one-shot" {
				if err != nil || calls != 1 {
					t.Fatal("ASSERT_CONTROLLER_ADMISSION", err, calls)
				}
				retryPath := t.TempDir()
				if err := os.Chmod(retryPath, 0700); err != nil {
					t.Fatal(err)
				}
				retryRoot, e := publication.OpenRoot(retryPath)
				if e != nil {
					t.Fatal(e)
				}
				defer retryRoot.Close()
				if WithLocalFolderControllerV1(context.Background(), retryRoot, op, b, consume) == nil || calls != 1 {
					t.Fatal("ASSERT_CONTROLLER_NO_REPLAY")
				}
			} else if err == nil || calls != 0 {
				t.Fatal("ASSERT_PRE_EFFECT_REFUSAL", kind, err, calls)
			}
			t.Log("ASSERT_CONTROLLER_ADMISSION: PASS", kind)
		})
	}
}
func TestFolderControllerExecutableDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "producer")
	raw := []byte("actual producer bytes")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := folderControllerExecutableDigest(context.Background(), path, int64(len(raw)))
	if err != nil || digest != hashLabelV2(string(raw)) {
		t.Fatal("ASSERT_PRODUCER_BYTES", digest, err)
	}
	if _, err := folderControllerExecutableDigest(context.Background(), path, int64(len(raw)-1)); err == nil {
		t.Fatal("ASSERT_PRODUCER_BOUND")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := folderControllerExecutableDigest(ctx, path, int64(len(raw))); err == nil {
		t.Fatal("ASSERT_PRODUCER_CANCEL")
	}
	if _, err := folderControllerExecutableDigest(context.Background(), filepath.Dir(path), 64<<20); err == nil {
		t.Fatal("ASSERT_PRODUCER_REGULAR_FILE")
	}
	t.Log("ASSERT_PRODUCER_BYTES: PASS exact hash, finite bound, cancellation and non-file refusal")
}
