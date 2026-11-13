package applicationinventory

import (
	"context"
	"errors"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplicationInventoryV3PublicationFailures(t *testing.T) {
	for _, phase := range []string{"write:catalog.json", "readback:catalog.json", "verified:catalog.json", "write:manifest.json", "readback:manifest.json", "cleanup", "cancel", "corrupt-readback"} {
		t.Run(phase, func(t *testing.T) {
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			destination := filepath.Join(root, "new-owned")
			b, _ := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 1<<20)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fault := func(p string) error {
				if phase == "corrupt-readback" && p == "readback:catalog.json" {
					return os.WriteFile(filepath.Join(destination, "catalog.json"), []byte("corrupted"), 0600)
				}
				if phase == "cancel" && p == "verified:catalog.json" {
					cancel()
				}
				if p == phase {
					return errors.New("controlled publication failure")
				}
				return nil
			}
			out, e := publishV3(ctx, destination, []ArtifactV3{{"catalog.json", []byte(`{"fixture":true}`)}}, b, fault)
			if e == nil || out.Version != "" {
				t.Fatal("ASSERT_V3_PUBLICATION_NO_PARTIAL_SUCCESS", phase, e)
			}
			if _, e = os.Stat(destination); !os.IsNotExist(e) {
				t.Fatal("ASSERT_V3_PUBLICATION_FAILURE_CLEANUP", phase, e)
			}
			t.Log("ASSERT_V3_PUBLICATION_NO_PARTIAL_SUCCESS: PASS; ASSERT_V3_PUBLICATION_FAILURE_CLEANUP: PASS")
		})
	}
}
func TestApplicationInventoryV3PublicationSafeModesAndPredecessor(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	destination := filepath.Join(root, "publication")
	b, _ := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 4<<20)
	out, e := PublishV3(context.Background(), destination, []ArtifactV3{{"catalog.json", []byte("{}")}}, b)
	if e != nil || len(out.Artifacts) != 1 {
		t.Fatal("ASSERT_V3_PUBLICATION_SUCCESS", e)
	}
	for p, mode := range map[string]os.FileMode{destination: 0700, filepath.Join(destination, "catalog.json"): 0600, filepath.Join(destination, "manifest.json"): 0600} {
		i, e := os.Stat(p)
		if e != nil || i.Mode().Perm() != mode {
			t.Fatal("ASSERT_V3_EVIDENCE_MODES", p, e)
		}
	}
	original, _ := os.ReadFile(filepath.Join(destination, "catalog.json"))
	if _, e = PublishV3(context.Background(), destination, []ArtifactV3{{"catalog.json", []byte("changed")}}, b); e == nil {
		t.Fatal("ASSERT_V3_PREDECESSOR_NOT_OVERWRITTEN")
	}
	after, _ := os.ReadFile(filepath.Join(destination, "catalog.json"))
	if string(after) != string(original) {
		t.Fatal("ASSERT_V3_PREDECESSOR_NOT_OVERWRITTEN")
	}
	if _, e = PublishV3(context.Background(), filepath.Join(root, "unsafe"), []ArtifactV3{{"../escape", []byte("{}")}}, b); e == nil {
		t.Fatal("ASSERT_V3_UNSAFE_PATH_REFUSED")
	}
	t.Log("ASSERT_V3_PUBLICATION_SUCCESS: PASS; ASSERT_V3_EVIDENCE_MODES: PASS; ASSERT_V3_PREDECESSOR_NOT_OVERWRITTEN: PASS; ASSERT_V3_UNSAFE_PATH_REFUSED: PASS")
}
