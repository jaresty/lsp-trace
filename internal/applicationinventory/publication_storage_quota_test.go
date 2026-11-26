//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"errors"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicationStorageQuota(t *testing.T) {
	for _, mode := range []string{"sufficient", "capacity", "cleanup", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			path, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if e = os.Chmod(path, 0700); e != nil {
				t.Fatal(e)
			}
			root, e := publication.OpenRoot(path)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			limit := uint64(1 << 20)
			if mode == "capacity" {
				limit = 128 << 10
			}
			if e = publication.ConfigureStorage(context.Background(), root, limit); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			budget, e := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 8<<20)
			if e != nil {
				t.Fatal(e)
			}
			fault := func(stage string) error {
				if stage == "verified:a.bin" {
					if mode == "cleanup" {
						return errors.New("controlled interruption after materialized artifact")
					}
					if mode == "cancel" {
						cancel()
					}
				}
				return nil
			}
			destination := filepath.Join(path, "publication")
			start := time.Now()
			result, e := publishV3(ctx, destination, []ArtifactV3{{"a.bin", bytes.Repeat([]byte("a"), 100<<10)}, {"b.bin", bytes.Repeat([]byte("b"), 100<<10)}}, budget, fault)
			status, se := publication.ReadStorageStatus(context.Background(), root)
			if se != nil {
				t.Fatal(se)
			}
			t.Logf("PUBLICATION_QUOTA mode=%s elapsed_ns=%d file_bytes=%d allocated_sum=%d uncertain=%d sampled_peak=%d", mode, time.Since(start).Nanoseconds(), status.FileBytes, status.AllocatedBytes, status.UncertainReservedBytes, status.SampledPeakFileBytes)
			if mode == "sufficient" {
				if e != nil || len(result.Artifacts) != 2 || status.UncertainReservedBytes != 0 {
					t.Fatal(e, result, status)
				}
			} else {
				if e == nil || result.Version != "" {
					t.Fatal("PARTIAL_SUCCESS", e, result)
				}
				if _, e = os.Stat(destination); !os.IsNotExist(e) {
					t.Fatal("CLEANUP_FAILED", e)
				}
				if status.FileBytes != 4096 {
					t.Fatal("unremoved artifact", status)
				}
				if mode == "capacity" {
					if status.UncertainReservedBytes != 0 {
						t.Fatal("early refusal charged")
					}
				} else if status.UncertainReservedBytes != 216<<10 {
					t.Fatal("UNSAFE_FAILURE_REFUND", status)
				}
			}
		})
	}
}
