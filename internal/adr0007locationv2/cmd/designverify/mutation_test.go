package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestV2MutationMatrixFailsClosed(t *testing.T) {
	repo := filepath.Clean("../../../../")
	tmp := t.TempDir()
	gen := filepath.Join(tmp, "gen")
	verify := filepath.Join(tmp, "verify")
	run(t, repo, "go", "build", "-o", gen, "./internal/adr0007locationv2/cmd/designgen")
	run(t, repo, "go", "build", "-o", verify, "./internal/adr0007locationv2/cmd/designverify")
	base := filepath.Join(tmp, "base")
	run(t, repo, gen, "-root", base, "-freeze")
	if out, e := exec.Command(verify, "-root", base).CombinedOutput(); e != nil {
		t.Fatalf("base: %v %s", e, out)
	}
	case1 := filepath.Join("cases", "01-exact-file-intersects")
	muts := []struct{ name, file, old, new string }{
		{"request-enum", filepath.Join(case1, "REQUEST.json"), `"relation":"INTERSECTS"`, `"relation":"UNKNOWN"`}, {"condition", filepath.Join(case1, "CONDITION.json"), `"caseId":"01"`, `"caseId":"99"`}, {"binding", filepath.Join(case1, "SOURCE_ADMISSION.json"), `"admissionDigest":"sha256:`, `"admissionDigest":"sha256:f`}, {"object-digest", filepath.Join(case1, "SOURCE_ADMISSION.json"), `"objectDigest":"sha256:`, `"objectDigest":"sha256:f`}, {"candidate", filepath.Join(case1, "REQUEST.json"), `"path":"src/a.go"`, `"path":"src/z.go"`}, {"member-row", filepath.Join(case1, "EXPECTED_RESULT.json"), `"memberId":"m1"`, `"memberId":"substitute"`}, {"witness", filepath.Join(case1, "EXPECTED_RESULT.json"), `"intersection":{"start"`, `"intersection":{"start":{"line":99,"character":0},"end":{"line":100,"character":0}},"unused":{"start"`}, {"result-enum", filepath.Join(case1, "EXPECTED_RESULT.json"), `"outcome":"COMPLETE"`, `"outcome":"UNKNOWN"`}, {"accounting", filepath.Join(case1, "EXPECTED_RESULT.json"), `"input":1`, `"input":2`}, {"attempt", filepath.Join(case1, "EXPECTED_ATTEMPTS.json"), `"rawByteLength":`, `"rawByteLength":999,"unused":`}, {"raw", filepath.Join(case1, "EXPECTED_RAW.json"), `"byteLength":`, `"byteLength":999,"unused":`}, {"conflict", filepath.Join(case1, "EXPECTED_CONFLICTS.json"), `[]`, `null`}, {"review", filepath.Join(case1, "EXPECTED_REVIEW.json"), `"verdict":true`, `"verdict":false`}, {"account", filepath.Join(case1, "EXPECTED_ACCOUNT.json"), `"producerAttempts":1`, `"producerAttempts":2`}, {"fixture-envelope", filepath.Join(case1, "FIXTURE.json"), `"ordinal":1`, `"ordinal":2`}, {"freeze-entry", "FREEZE.json", `"bytes":`, `"bytes":999,"unused":`}}
	for _, m := range muts {
		t.Run(m.name, func(t *testing.T) {
			root := filepath.Join(tmp, m.name)
			copyTree(t, base, root)
			replace(t, filepath.Join(root, m.file), m.old, m.new)
			expectFail(t, verify, root)
		})
	}
	t.Run("missing-case", func(t *testing.T) {
		root := filepath.Join(tmp, "missing")
		copyTree(t, base, root)
		os.RemoveAll(filepath.Join(root, case1))
		expectFail(t, verify, root)
	})
	t.Run("extra-case", func(t *testing.T) {
		root := filepath.Join(tmp, "extra")
		copyTree(t, base, root)
		copyTree(t, filepath.Join(root, case1), filepath.Join(root, "cases", "25-extra"))
		expectFail(t, verify, root)
	})
	t.Run("reordered-case", func(t *testing.T) {
		root := filepath.Join(tmp, "reordered")
		copyTree(t, base, root)
		mustT(t, os.Rename(filepath.Join(root, case1), filepath.Join(root, "cases", "00-reordered")))
		expectFail(t, verify, root)
	})
}
func expectFail(t *testing.T, v, r string) {
	t.Helper()
	if out, e := exec.Command(v, "-root", r).CombinedOutput(); e == nil {
		t.Fatalf("mutation accepted: %s", out)
	}
}
func replace(t *testing.T, p, old, new string) {
	t.Helper()
	b := readT(t, p)
	if !bytes.Contains(b, []byte(old)) {
		t.Fatalf("missing mutation target %q", old)
	}
	b = bytes.Replace(b, []byte(old), []byte(new), 1)
	mustT(t, os.WriteFile(p, b, 0644))
}
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	mustT(t, filepath.Walk(src, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(src, p)
		q := filepath.Join(dst, rel)
		if i.IsDir() {
			return os.MkdirAll(q, 0755)
		}
		return os.WriteFile(q, readT(t, p), 0644)
	}))
}
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command(args[0], args[1:]...)
	c.Dir = dir
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("%v: %v %s", args, e, out)
	}
}
func readT(t *testing.T, p string) []byte { t.Helper(); b, e := os.ReadFile(p); mustT(t, e); return b }
func mustT(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
