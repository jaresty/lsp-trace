//go:build darwin || linux

package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPostPreparationTopologyPrivate(t *testing.T) {
	for _, kind := range []string{"disjoint", "equal-output", "equal-input", "output-ancestor", "input-ancestor", "input-input-ancestor", "closed", "cancel", "close-fault", "acquire-error", "alias", "replaced", "empty", "over-count"} {
		t.Run(kind, func(t *testing.T) {
			old, ins, paths := retentionTopologyFixturePrivate(t)
			outs := [2]*ContentRetentionRoot{old[0], old[1]}
			protected := []*ObservedRoot{ins[0], ins[1], ins[2]}
			openInput := func(path string) *ObservedRoot {
				r, e := OpenObservedRoot(context.Background(), path)
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() {
					if e := r.Close(); e != nil {
						t.Error(e)
					}
				})
				return r
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ops, log := trackingObservedRootOps(t)
			fault := errors.New("post topology injected fault")
			switch kind {
			case "equal-output":
				outs[1] = outs[0]
			case "equal-input":
				protected[0] = openInput(paths[0])
			case "output-ancestor":
				p := filepath.Join(paths[0], "child")
				if e := os.Mkdir(p, 0700); e != nil {
					t.Fatal(e)
				}
				protected[0] = openInput(p)
			case "input-ancestor":
				protected[0] = openInput(filepath.Dir(paths[0]))
			case "input-input-ancestor":
				p := filepath.Join(ins[0].root.path, "child")
				if e := os.Mkdir(p, 0700); e != nil {
					t.Fatal(e)
				}
				protected = append(protected, openInput(p))
			case "closed":
				if e := ins[0].Close(); e != nil {
					t.Fatal(e)
				}
			case "cancel":
				cancel()
			case "empty":
				protected = nil
			case "over-count":
				for len(protected) < 17 {
					protected = append(protected, ins[0])
				}
			case "close-fault":
				close := ops.closeFile
				ops.closeFile = func(f *os.File) error { return errors.Join(close(f), fault) }
			case "acquire-error":
				open := ops.openDir
				ops.openDir = func(p *os.File, n string) (*os.File, error) { f, e := open(p, n); return f, errors.Join(e, fault) }
			case "alias":
				alias := paths[0] + "-alias"
				if e := os.Symlink(paths[0], alias); e != nil {
					t.Fatal(e)
				}
				r, e := OpenContentRetentionRoot(ctx, alias)
				if e != nil {
					log.assertClosed(t)
					return
				}
				outs[1] = r
				t.Cleanup(func() {
					if e := r.Close(); e != nil {
						t.Error(e)
					}
				})
			case "replaced":
				open := ops.openDir
				done := false
				ops.openDir = func(p *os.File, n string) (*os.File, error) {
					f, e := open(p, n)
					if n == "out0" && !done && e == nil {
						done = true
						if e := os.Rename(paths[0], paths[0]+"-moved"); e != nil {
							t.Fatal(e)
						}
						if e := os.Mkdir(paths[0], 0700); e != nil {
							t.Fatal(e)
						}
					}
					return f, e
				}
			}
			e := checkPostPreparationRetentionTopologyV1Private(ctx, outs, protected, ops)
			good := kind == "disjoint" || kind == "input-input-ancestor"
			if good && e != nil {
				t.Fatal(e)
			}
			if !good && e == nil {
				t.Fatal("unsafe topology admitted")
			}
			if (kind == "close-fault" || kind == "acquire-error") && !errors.Is(e, fault) {
				t.Fatal("fault lost", e)
			}
			log.assertClosed(t)
		})
	}
}

func TestPostPreparationTopologyPrivateTemporaryOutputs(t *testing.T) {
	old, ins, _ := retentionTopologyFixturePrivate(t)
	// Linux can admit /tmp as a root, but the output topology still cannot.
	r, e := OpenContentRetentionRoot(context.Background(), "/tmp")
	if e != nil {
		return
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if e = CheckPostPreparationRetentionTopologyV1Private(context.Background(), [2]*ContentRetentionRoot{old[0], r}, ins[:]); e == nil {
		t.Fatal("tmp output admitted")
	}
}
