package censuslineagepilot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type acquisitionCountingRoot struct {
	preflightRoot
	reads  *int
	closed *bool
}

func (r acquisitionCountingRoot) ReadContentObserved(c context.Context, id string, n int64) ([]byte, error) {
	*r.reads++
	return r.preflightRoot.ReadContentObserved(c, id, n)
}
func (r acquisitionCountingRoot) Close() error { *r.closed = true; return r.preflightRoot.Close() }

// This is initial unauthorized substitution, not rename after successful
// descriptor admission. Both destinations contain genuinely valid canonical
// inputs so a parser/digest guard cannot accidentally mask the acquisition bug.
func TestPreflightRejectsAncestorSwapBeforeAcquisition(t *testing.T) {
	for _, privateTmp := range []bool{false, true} {
		name := "other-private-root"
		if privateTmp {
			name = "alias-to-private-tmp"
		}
		t.Run(name, func(t *testing.T) {
			req, sb, pb := preflightFixture(t)
			outer := req.RootPath
			ancestor := filepath.Join(outer, "ancestor")
			initial := filepath.Join(ancestor, "root")
			if err := os.MkdirAll(initial, 0700); err != nil {
				t.Fatal(err)
			}
			targetBase, err := os.MkdirTemp(outer, "destination-")
			if privateTmp {
				targetBase, err = os.MkdirTemp("/private/tmp", "lsp-trace-acquisition-test-")
			}
			if err != nil {
				t.Fatal(err)
			}
			// Only this test-created directory is removed, never a supplied durable root.
			if privateTmp {
				t.Cleanup(func() {
					if err := os.RemoveAll(targetBase); err != nil {
						t.Error(err)
					}
				})
			}
			target := filepath.Join(targetBase, "root")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{initial, target} {
				for i, raw := range [][]byte{sb, pb} {
					if err := os.WriteFile(filepath.Join(dir, req.Inputs[i].Selector), raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			req.RootPath = initial
			if err := ValidateDurableRoot(initial); err != nil {
				t.Fatal("initial root not admitted", err)
			}
			reads := 0
			closed := false
			acquired := false
			gapReached := false
			result, err := preflightWithOpen(context.Background(), req, func(path string) (preflightRoot, error) {
				gapReached = true
				// This callback executes after ValidateDurableRoot, immediately before the
				// actual production acquisition function. Redirect only a t-owned ancestor.
				if err := os.Rename(ancestor, ancestor+"-saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(targetBase, ancestor); err != nil {
					t.Fatal(err)
				}
				r, e := openPreflightRoot(context.Background(), path)
				if e != nil {
					return r, e
				}
				acquired = true
				return acquisitionCountingRoot{r, &reads, &closed}, nil
			})
			if !gapReached {
				t.Fatal("regression never reached validation/acquisition gap")
			}
			if reads != 0 {
				t.Errorf("unauthorized content reads: %d", reads)
			}
			if err == nil || result.state != nil {
				t.Error("ancestor substitution admitted")
			}
			if acquired && !closed {
				t.Error("acquired root was not closed")
			}
		})
	}
}
