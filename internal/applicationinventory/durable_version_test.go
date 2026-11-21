//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionDispatchConservative(t *testing.T) {
	raw, e := os.ReadFile("testdata/stored-v1/pending.json")
	if e != nil {
		t.Fatal(e)
	}
	var env inventoryHeadEnvelope
	if json.Unmarshal(raw, &env) != nil {
		t.Fatal("fixture")
	}
	var original durableInventoryHead
	if strictInventory(env.Content, &original) != nil {
		t.Fatal("fixture shape")
	}
	for _, tc := range []struct {
		name string
		edit func(*durableInventoryHead)
		want error
	}{
		{"supported", func(h *durableInventoryHead) {
			h.Schema = durableHeadVersion
			h.Completion.Version = durableQuoteVersion
		}, nil},
		{"old-plan", func(h *durableInventoryHead) { h.Schema = durableHeadVersion }, ErrUnsupportedStoredVersion},
		{"unknown-head", func(h *durableInventoryHead) { h.Schema = "lsp-trace.durable-inventory-head.v999" }, ErrDurableInventory},
		{"unknown-plan", func(h *durableInventoryHead) { h.Schema = durableHeadVersion; h.Completion.Version = "unknown" }, ErrDurableInventory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := original
			tc.edit(&h)
			encoded, _ := encodeInventoryHead(h)
			var got durableInventoryHead
			if e := decodeInventoryHead(encoded, &got); !errors.Is(e, tc.want) {
				t.Fatalf("version dispatch got=%v want=%v", e, tc.want)
			}
		})
	}
	for _, content := range [][]byte{[]byte(`{"Schema":"lsp-trace.durable-inventory-head.v1"}`), append(append([]byte(nil), env.Content...), []byte(`{}`)...), bytes.Replace(env.Content, []byte(`"Sequence":1`), []byte(`"Sequence":1,"Sequence":1`), 1)} {
		encoded, _ := json.Marshal(inventoryHeadEnvelope{Digest: cat.NativeDigestV3(content), Content: content})
		var h durableInventoryHead
		if e := decodeInventoryHead(encoded, &h); !errors.Is(e, ErrDurableInventory) {
			t.Fatal("malformed claimed unsupported", e)
		}
	}
	corrupt := append([]byte(nil), raw...)
	corrupt[20] ^= 1
	var h durableInventoryHead
	if e := decodeInventoryHead(corrupt, &h); !errors.Is(e, ErrDurableInventory) {
		t.Fatal("corrupt claimed unsupported", e)
	}
	// Frozen pre-chunk shape is recognized only; it is never converted to a head.
	var old inventoryHeadV1
	old.Schema = original.Schema
	old.RunID = original.RunID
	old.TaskID = original.TaskID
	old.State = original.State
	old.Input = original.Input
	old.Completion.Version = original.Completion.Version
	content, _ := json.Marshal(old)
	encoded, _ := json.Marshal(inventoryHeadEnvelope{Digest: cat.NativeDigestV3(content), Content: content})
	if e := decodeInventoryHead(encoded, &h); !errors.Is(e, ErrUnsupportedStoredVersion) {
		t.Fatal("pre-chunk recognition", e)
	}
}

func TestStoredV1RefusedWithoutEffects(t *testing.T) {
	for _, name := range []string{"pending", "published"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/stored-v1/" + name + ".json")
			if e != nil {
				t.Fatal(e)
			}
			var env struct {
				Content struct{ RunID, TaskID, Role string }
			}
			if json.Unmarshal(raw, &env) != nil {
				t.Fatal("fixture")
			}
			dir := t.TempDir()
			if e = os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(dir, inventoryHeadName(env.Content.RunID))
			if e = os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			root, e := publication.OpenRoot(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			s := DurableStore{Root: root, Directory: dir}
			ctx := context.Background()
			h := env.Content
			checks := []func() error{
				func() error { _, e := s.Task(ctx, h.RunID, ""); return e },
				func() error { _, e := s.Read(ctx, h.RunID, h.TaskID, "input", 0, 32768); return e },
				func() error { _, e := s.Submit(ctx, h.RunID, h.TaskID, h.Role, []byte("invalid")); return e },
				func() error { _, e := s.Result(ctx, h.RunID, h.TaskID); return e },
			}
			before, _ := os.ReadDir(dir)
			for i, f := range checks {
				if e = f(); !errors.Is(e, ErrUnsupportedStoredVersion) {
					t.Fatalf("ASSERT_STORED_VERSION operation=%d: %v", i, e)
				}
				after, e := os.ReadFile(path)
				if e != nil || !bytes.Equal(raw, after) {
					t.Fatal("ASSERT_STORED_BYTES_UNCHANGED", e)
				}
				files, _ := os.ReadDir(dir)
				if len(files) != len(before) {
					t.Fatal("ASSERT_NO_VERSION_EFFECTS")
				}
			}
			t.Log("ASSERT_STORED_VERSION: PASS; all four operations, exact bytes and no new artifacts")
		})
	}
}
