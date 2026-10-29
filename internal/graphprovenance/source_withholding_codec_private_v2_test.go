//go:build darwin || linux

package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceWithholdingPrivateV2RetainedContext(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV2(t, 1, 0)
	r.Graph.Invocation.Server.Environment = map[string]string{"FIXTURE_TOKEN": "not-a-real-secret"}
	r.Graph.Invocation.EffectiveEnvironment = []string{"FIXTURE_TOKEN=not-a-real-secret", "PATH=/fixture/frozen-bin"}
	r.Graph.Invocation.WorkingDirectory = "/fixture/original-cwd"
	original, err := json.Marshal(r.Graph)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal("ASSERT_V2_PROCESS_CAPTURE", err)
	}
	e, err := ParseSourceWithholdingPrivateV2(raw)
	if err != nil {
		t.Fatal("ASSERT_V2_PROCESS_PARSE", err)
	}
	again, err := json.Marshal(e)
	if err != nil || !bytes.Equal(bytes.TrimSpace(raw), again) || !bytes.Equal(original, e.Evidence.GraphBytes) {
		t.Fatal("ASSERT_V2_PROCESS_EXACT", err)
	}
	e.Evidence.Acquisition.Graph.Nodes[0].Name = "forged node name"
	if _, err = json.Marshal(e); err == nil {
		t.Fatal("ASSERT_V2_OTHER_GRAPH_FIELDS_REPROJECTED")
	}
}
func TestSourceWithholdingPrivateV2ArtifactLength(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV2(t, 1, 0)
	raw, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{len(bytes.TrimSpace(raw)) + (5 << 20), 4 << 20, (4 << 20) + 1} {
		padded := append(bytes.TrimSpace(raw), bytes.Repeat([]byte(" "), n-len(bytes.TrimSpace(raw)))...)
		_, err = ParseSourceWithholdingPrivateV2(padded)
		if n == 4<<20 && err != nil {
			t.Fatal("ASSERT_V2_EXACT_ARTIFACT_LENGTH", err)
		}
		if n > 4<<20 && err == nil {
			t.Fatal("ASSERT_V2_OVER_ARTIFACT_LENGTH", n)
		}
	}
}
func TestSourceWithholdingPrivateV2EncodedArtifactLimit(t *testing.T) {
	r, p, expected, root := withholdingFixturePrivateV2(t, 4, 0)
	for i := range p.Members {
		body := bytes.Repeat([]byte("x"), 900<<10)
		expected[p.Members[i].URI] = body
		p.Members[i].SHA256 = sourceHashPrivateV1(body)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("member-%02d.go", i)), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, readSourcePrivateV1)
	if out != nil || err == nil {
		t.Fatal("ASSERT_V2_ENCODED_LIMIT_NO_PUBLICATION", len(out), err)
	}
}

func TestSourceWithholdingPrivateV2ExcludedSymlink(t *testing.T) {
	r, p, expected, root := withholdingFixturePrivateV2(t, 2, 1)
	target := filepath.Join(t.TempDir(), "outside-target.go")
	if err := os.WriteFile(target, []byte("owned forbidden target"), 0600); err != nil {
		t.Fatal(err)
	}
	excluded := filepath.Join(root, "forbidden.go")
	if err := os.Remove(excluded); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, excluded); err != nil {
		t.Fatal(err)
	}
	accesses := 0
	calls := 0
	raw, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, func(c context.Context, root *sourceRootPrivateV1, name string, n int64) ([]byte, error) {
		calls++
		if name == "forbidden.go" {
			accesses++
		}
		root.ops.hook = func(stage, name string, _ os.FileInfo) error {
			if name == "forbidden.go" || name == "outside-target.go" {
				accesses++
			}
			return nil
		}
		root.ops.read = func(f *os.File, b []byte) (int, error) {
			if f.Name() == "outside-target.go" {
				accesses++
			}
			return f.Read(b)
		}
		return readSourcePrivateV1(c, root, name, n)
	})
	if err != nil {
		t.Fatal(err)
	}
	if accesses != 0 || calls != len(p.Members) {
		t.Fatal("ASSERT_V2_EXCLUDED_SYMLINK_ZERO_ACCESS", accesses, calls)
	}
	e, err := ParseSourceWithholdingPrivateV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	view, err := sourceProfileV2Private(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range e.Evidence.Captures {
		if row.URI == p.WorkspaceURI+"/forbidden.go" {
			found = true
			if !view.validWithheld(p.WorkspaceURI, row) || row.Content != nil || row.CanonicalReceipt != nil {
				t.Fatal("ASSERT_V2_EXCLUDED_SYMLINK_METADATA_ONLY")
			}
		}
	}
	if !found {
		t.Fatal("excluded receipt missing")
	}
}

func TestSourceWithholdingPrivateV2NominationSeeds(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV2(t, 2, 1)
	raw, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal(err)
	}
	p.Members[0].Role = "CONTEXT_ONLY"
	p.Members[2].Role = "NOMINATION"
	t.Run("capture", func(t *testing.T) {
		reads := 0
		out, err := captureSourceWithholdingPrivateV2(context.Background(), r, p, expected, func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) {
			reads++
			return expected[p.Members[0].URI], nil
		})
		if err == nil || out != nil || reads != 0 {
			t.Fatal("ASSERT_V2_CONTEXT_SEED_BEFORE_IO", reads, err)
		}
	})
	t.Run("parser", func(t *testing.T) {
		e, err := ParseSourceWithholdingPrivateV2(raw)
		if err != nil {
			t.Fatal(err)
		}
		e.Policy = p
		bad, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = ParseSourceWithholdingPrivateV2(bad); err == nil {
			t.Fatal("ASSERT_V2_CONTEXT_SEED_PARSE")
		}
	})
}

func TestSourceWithholdingPrivateV2CancellationNoOutput(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV2(t, 2, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, err := captureSourceWithholdingPrivateV2(ctx, r, p, expected, func(c context.Context, root *sourceRootPrivateV1, name string, n int64) ([]byte, error) {
		b, err := readSourcePrivateV1(c, root, name, n)
		cancel()
		return b, err
	})
	if out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("ASSERT_V2_CANCEL_NO_OUTPUT", len(out), err)
	}
	// Entry cancellation does not attempt reads.
	out, err = captureSourceWithholdingPrivateV2(ctx, r, p, expected, func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) {
		t.Fatal("cancelled entry read")
		return nil, nil
	})
	if out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("ASSERT_V2_CANCEL_ENTRY")
	}
	// A cancellation does not suppress root-close errors or bypass the close.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	closeErr := errors.New("owned close failure")
	closes, reads := 0, 0
	out, err = captureSourceWithholdingPrivateV2(ctx2, r, p, expected, func(c context.Context, root *sourceRootPrivateV1, name string, n int64) ([]byte, error) {
		reads++
		b, err := readSourcePrivateV1(c, root, name, n)
		root.ops.close = func(f *os.File) error { closes++; return errors.Join(f.Close(), closeErr) }
		cancel2()
		return b, err
	})
	if out != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, closeErr) || closes == 0 || reads != 1 {
		t.Fatal("ASSERT_V2_CANCEL_CLOSE_JOIN", reads, closes, err)
	}
}
