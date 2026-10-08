package adr0007locationv5

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func freezeRoot(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "docs", "pilot", "adr0007", "experiment", "location-intersection-prospective-v5")
}

func TestBuildFreezeRootCensus(t *testing.T) {
	f, b, err := BuildFreeze(freezeRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != "IMPLEMENTATION_AGREEMENT_GO" || f.Design["accepted"] != false || f.Design["authority"] != float64(0) && f.Design["authority"] != 0 {
		t.Fatalf("bad custody design: %#v", f.Design)
	}
	if f.Custody["spec"] != "637680d2" || f.Custody["integration"] != "a9c82f1f" || f.Custody["independence"] != "procedural-not-cryptographic" {
		t.Fatalf("bad custody: %#v", f.Custody)
	}
	if f.Counts["inputCases"] != 26 || f.Counts["evaluatorResults"] != 26 || f.Counts["oracleResults"] != 26 || f.Counts["oracleDerivations"] != 26 || f.Counts["boundaryBundles"] != 4 {
		t.Fatalf("bad counts: %#v", f.Counts)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatal("freeze must be canonical newline terminated")
	}
	seenFreeze := false
	for _, m := range f.Files {
		if m.Path == "FREEZE.json" {
			seenFreeze = true
			if m.Bytes != 0 || m.SHA256 != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
				t.Fatalf("FREEZE self measurement not zero image: %#v", m)
			}
		}
	}
	if !seenFreeze {
		t.Fatal("FREEZE self measurement missing")
	}
}

func TestGenerateTwiceDeterministicAndVerify(t *testing.T) {
	src := freezeRoot(t)
	a := t.TempDir()
	b := t.TempDir()
	fa, err := CopyTreeAndFreeze(src, a)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := CopyTreeAndFreeze(src, b)
	if err != nil {
		t.Fatal(err)
	}
	if fa.RootIdentity != fb.RootIdentity {
		t.Fatalf("identity mismatch %s %s", fa.RootIdentity, fb.RootIdentity)
	}
	ca, err := Census(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := Census(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(ca) != len(cb) {
		t.Fatalf("census length mismatch")
	}
	for i := range ca {
		if ca[i] != cb[i] {
			t.Fatalf("census mismatch at %d %#v %#v", i, ca[i], cb[i])
		}
	}
	if _, err := VerifyFreeze(a); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyFreeze(b); err != nil {
		t.Fatal(err)
	}
}

func CopyTreeAndFreeze(src, dst string) (Freeze, error) {
	if err := CopyTree(src, dst); err != nil {
		return Freeze{}, err
	}
	return WriteFreezeLast(dst)
}

func TestEvaluatorOracleByteEquality(t *testing.T) {
	root := freezeRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "inputs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		a, err := os.ReadFile(filepath.Join(root, "evaluator-candidate", "cases", id, "RESULT.json"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(root, "oracle-candidate", "cases", id, "RESULT.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("ASSERT evaluator-oracle-byte-equality FAIL case=%s", id)
		}
	}
}

func TestBoundaryOutcomes(t *testing.T) {
	root := freezeRoot(t)
	want := map[string]string{"W": `"outcome":"COMPLETE"`, "B": `"outcome":"COMPLETE"`, "W-1": `"outcome":"RESOURCE_LIMIT","members":[],"ranked":[]`, "B-1": `"outcome":"RESOURCE_LIMIT","members":[],"ranked":[]`}
	for name, fragment := range want {
		b, err := os.ReadFile(filepath.Join(root, "oracle-candidate", "boundaries", name, "RESULT.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(b, []byte(fragment)) {
			t.Fatalf("ASSERT boundary-outcome FAIL %s got=%s", name, b)
		}
	}
}

func TestFreezeFailClosedGuards(t *testing.T) {
	src := freezeRoot(t)
	t.Run("requires-empty-output", func(t *testing.T) {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, "x"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := CopyTree(src, d); err == nil {
			t.Fatal("expected non-empty output rejection")
		}
	})
	t.Run("missing-required-file", func(t *testing.T) {
		d := t.TempDir()
		if _, err := CopyTreeAndFreeze(src, d); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(d, "POLICY.json")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := BuildFreeze(d); err == nil {
			t.Fatal("expected missing normative file rejection")
		}
	})
	t.Run("extra-case", func(t *testing.T) {
		d := t.TempDir()
		if _, err := CopyTreeAndFreeze(src, d); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(d, "inputs", "99-extra"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := BuildFreeze(d); err == nil {
			t.Fatal("expected extra case rejection")
		}
	})
	t.Run("freeze-mutation", func(t *testing.T) {
		d := t.TempDir()
		if _, err := CopyTreeAndFreeze(src, d); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(d, "FREEZE.json")
		old, _ := os.ReadFile(p)
		if err := os.WriteFile(p, bytes.Replace(old, []byte("IMPLEMENTATION_AGREEMENT_GO"), []byte("IMPLEMENTATION_AGREEMENT_NO"), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyFreeze(d); err == nil {
			t.Fatal("expected freeze mismatch")
		}
	})
}
