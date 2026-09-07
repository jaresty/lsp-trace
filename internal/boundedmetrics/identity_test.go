package boundedmetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
)

func TestMetricsFixedIndependentPythonVectors(t *testing.T) {
	// Literal Python hashlib/json oracle vectors; these inputs test only identity,
	// not admission. Result vector is the empty table result with this exact input.
	for _, v := range []struct{ raw, basis, result string }{
		{"{}", "sha256:77e1dbe10b282153b7f3bc2bef100a066fd5e4a8efda97dd023c8556f7f7225d", "sha256:dab8cd93b4e369ed0859472283e9bc887416674b950f5178e5d72003799aca94"},
		{"{}\n", "sha256:d208fc883d9ef769f83c9a90943cd5341502ddc2cf6061f82760d80fe0409899", "sha256:1bbb6fc9d0122834878321f908db2bf97609e00d56684e3abb415aba6e22551f"},
	} {
		if got := basis([]byte(v.raw)); got != v.basis {
			t.Fatal("ASSERT_FIXED_BASIS", got)
		}
		e, err := compute(context.Background(), []byte(v.raw), table(0, nil))
		if err != nil {
			t.Fatal(err)
		}
		if got := seal(e); got != v.result {
			t.Fatal("ASSERT_FIXED_RESULT", got)
		}
	}
}
func TestMetricsIndependentPythonAdmittedOracle(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable; fixed independently computed vectors still enforced")
	}
	for _, f := range []struct {
		n int
		p [][2]int
	}{{0, nil}, {1, nil}, {2, nil}, {3, [][2]int{{0, 1}, {1, 1}}}, {3, [][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {2, 0}, {2, 1}, {2, 2}}}} {
		raw := fixture(t, f.n, f.p)
		got := admitted(t, raw)
		cmd := exec.Command(python, "testdata/oracle.py")
		cmd.Stdin = bytes.NewReader(raw)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("oracle: %v %s", err, out)
		}
		if !bytes.Equal(canonical(got), canonical(json.RawMessage(out))) {
			t.Fatalf("ASSERT_PYTHON_FULL_ARTIFACT_ORACLE\nGo: %s\nPython: %s", canonical(got), out)
		}
	}
}
