//go:build darwin || linux

package graphprovenance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceWithholdingPrivateV1SymlinkCapture(t *testing.T) {
	r, p, expected, workspace := withholdingFixturePrivateV1(t)
	target := filepath.Join(workspace, "z.go")
	if err := os.WriteFile(target, []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(workspace, "a.go")
	if err := os.Remove(allowed); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("z.go", allowed); err != nil {
		t.Fatal(err)
	}
	raw, err := captureSourceWithholdingPrivateV1(context.Background(), r, p, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ParseSourceWithholdingPrivateV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range e.Evidence.Captures {
		if row.URI == p.Members[0].URI {
			found = true
			if row.Status == "READABLE" || row.Content != nil {
				t.Fatal("ASSERT_PRIVATE_CAPTURE_REFUSES_EQUAL_SYMLINK: forbidden equal-byte target captured")
			}
		}
	}
	if !found {
		t.Fatal("capture row omitted")
	}
	// The historical V2 source path deliberately keeps its prior symlink behavior.
	oldRaw, err := CaptureV2(context.Background(), r, workspace)
	if err != nil {
		t.Fatal(err)
	}
	var old EvidenceV2
	if err = old.UnmarshalJSON(oldRaw); err != nil {
		t.Fatal(err)
	}
	for _, row := range old.Captures {
		if row.URI == p.Members[0].URI && row.Status != "READABLE" {
			t.Fatal("ASSERT_HISTORICAL_V2_SYMLINK_CHANGED")
		}
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "package p\n" {
		t.Fatal("forbidden target mutated", err)
	}
	t.Log("ASSERT_PRIVATE_CAPTURE_REFUSES_EQUAL_SYMLINK: PASS")
}
