//go:build darwin || linux

package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func retentionTopologyFixturePrivate(t *testing.T) ([3]*ContentRetentionRoot, [3]*ObservedRoot, [3]string) {
	t.Helper()
	path, _, _ := observedRootFixture(t)
	base := filepath.Dir(path)
	var outputs [3]*ContentRetentionRoot
	var inputs [3]*ObservedRoot
	var paths [3]string
	for i := 0; i < 3; i++ {
		paths[i] = filepath.Join(base, "out"+string(rune('0'+i)))
		ip := filepath.Join(base, "in"+string(rune('0'+i)))
		for _, p := range []string{paths[i], ip} {
			if err := os.Mkdir(p, 0700); err != nil {
				t.Fatal(err)
			}
		}
		var err error
		outputs[i], err = OpenContentRetentionRoot(context.Background(), paths[i])
		if err != nil {
			t.Fatal(err)
		}
		inputs[i], err = OpenObservedRoot(context.Background(), ip)
		if err != nil {
			t.Fatal(err)
		}
		a, b := outputs[i], inputs[i]
		t.Cleanup(func() {
			if err := errors.Join(a.Close(), b.Close()); err != nil {
				t.Error(err)
			}
		})
	}
	return outputs, inputs, paths
}
func TestRetentionTopologyPrivatePinnedTraversalAndCloses(t *testing.T) {
	for _, kind := range []string{"disjoint", "close-fault", "moved-during-walk", "symlink-during-walk", "acquire-handle-with-error"} {
		t.Run(kind, func(t *testing.T) {
			outputs, inputs, paths := retentionTopologyFixturePrivate(t)
			ops, log := trackingObservedRootOps(t)
			fault := errors.New("topology component close fault")
			acquireFault := errors.New("topology acquisition fault")
			if kind == "close-fault" {
				closeFile := ops.closeFile
				ops.closeFile = func(f *os.File) error { return errors.Join(closeFile(f), fault) }
			}
			open := ops.openDir
			switched := false
			ops.openDir = func(parent *os.File, name string) (*os.File, error) {
				f, err := open(parent, name)
				if name == "out0" && !switched && err == nil {
					switched = true
					switch kind {
					case "moved-during-walk", "symlink-during-walk":
						moved := paths[0] + "-moved"
						if e := os.Rename(paths[0], moved); e != nil {
							t.Fatal(e)
						}
						if kind == "symlink-during-walk" {
							err = os.Symlink(moved, paths[0])
						} else {
							err = os.Mkdir(paths[0], 0700)
						}
						if err != nil {
							t.Fatal(err)
						}
					case "acquire-handle-with-error":
						err = acquireFault
					}
				}
				return f, err
			}
			err := checkRetentionRootTopologyPrivate(context.Background(), outputs, inputs, ops)
			if kind == "disjoint" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("uncertainty admitted")
			}
			if kind == "close-fault" && !errors.Is(err, fault) {
				t.Fatal("close fault lost", err)
			}
			if kind == "acquire-handle-with-error" && !errors.Is(err, acquireFault) {
				t.Fatal("acquisition fault lost", err)
			}
			log.assertClosed(t)
			// The input capability is read-only and remains live after helper cleanup.
			for _, r := range inputs {
				if _, e := r.DirectoryIdentity(); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestRetentionTopologyPrivateOutputTempPolicy(t *testing.T) {
	// Existing admission still rejects /private/tmp. No test or production escape
	// hatch changes it; all executable topology fixtures above are owned home roots.
	if r, e := OpenObservedRoot(context.Background(), "/private/tmp"); r != nil || e == nil {
		if r != nil {
			_ = r.Close()
		}
		t.Fatal("temporary root admitted")
	}
}
