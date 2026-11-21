package publication

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Cancellation is synchronous with an actual descriptor close, not a timer or
// an assertion that a blocked syscall can be preempted.
type checkedCleanupCancelCloser struct {
	io.Closer
	cancel context.CancelFunc
}

func (c checkedCleanupCancelCloser) Close() error {
	err := c.Closer.Close()
	c.cancel()
	return err
}

type checkedCleanupCancelData struct {
	*checkedReadFaultFile
	cancel context.CancelFunc
}

func (f checkedCleanupCancelData) Close() error {
	err := f.checkedReadFaultFile.Close()
	f.cancel()
	return err
}

func TestReadVerifiedBoundFileCheckedCleanupContext(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		cancelAt                           string
		readFailure, closeFailures, noData bool
	}{
		{name: "data-close", cancelAt: "data"},
		{name: "parent-close", cancelAt: "parent0"},
		{name: "inner-parent-close", cancelAt: "parent1"},
		{name: "data-close-errors", cancelAt: "data", closeFailures: true},
		{name: "parent-close-errors", cancelAt: "parent0", closeFailures: true},
		{name: "read-error-and-cancel", cancelAt: "parent0", readFailure: true},
		{name: "read-and-close-errors-and-cancel", cancelAt: "data", readFailure: true, closeFailures: true},
		{name: "parent-only-error-and-cancel", cancelAt: "parent0", closeFailures: true, noData: true},
		{name: "success-control"},
		{name: "read-error-control", readFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, root := checkedReadRoot(t)
			checkedReadPublish(t, root, "keep", []byte("borrowed-root-live"))
			checkedReadPublish(t, root, "a/b/file", []byte("contents"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			callback := func(name string) context.CancelFunc {
				if name == tc.cancelAt {
					return cancel
				}
				return func() {}
			}
			order := []string{}
			h := &boundFileReadHandles{}
			// Safety cleanup is a no-op after the reader relinquishes ownership.
			t.Cleanup(func() { var raw []byte; var err error; h.finish(&raw, &err) })
			var parents []*checkedReadFaultParent
			var causes []error
			for i, name := range []string{"parent0", "parent1"} {
				path := filepath.Join(dir, "a")
				if i == 1 {
					path = filepath.Join(path, "b")
				}
				f, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				p := &checkedReadFaultParent{Closer: f, name: name, order: &order}
				if tc.closeFailures {
					p.failure = errors.New(name + " close")
					causes = append(causes, p.failure)
				}
				parents = append(parents, p)
				h.parents = append(h.parents, checkedCleanupCancelCloser{p, callback(name)})
			}
			var data *checkedReadFaultFile
			var before os.FileInfo
			if tc.noData {
				causes = append(causes, ErrBoundFilePolicy)
			} else {
				f, err := os.Open(filepath.Join(dir, "a/b/file"))
				if err != nil {
					t.Fatal(err)
				}
				data = &checkedReadFaultFile{File: f, order: &order}
				h.data = checkedCleanupCancelData{data, callback("data")}
				before, err = f.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if tc.readFailure {
					data.readErr = errors.New("earlier read error")
					causes = append(causes, data.readErr)
				}
				if tc.closeFailures {
					data.closeErr = errors.New("data close")
					causes = append(causes, data.closeErr)
				}
			}
			got, err := h.readContext(ctx, root.info, before, 64)
			if tc.cancelAt != "" {
				if !errors.Is(err, context.Canceled) || got != nil {
					t.Errorf("ASSERT_CLEANUP_CANCEL: bytes=%q err=%v", got, err)
				}
			}
			for _, cause := range causes {
				if !errors.Is(err, cause) {
					t.Errorf("ASSERT_CLEANUP_CAUSES: lost %v in %v", cause, err)
				}
			}
			if tc.cancelAt == "" && len(causes) == 0 {
				if err != nil || string(got) != "contents" {
					t.Errorf("ASSERT_CLEANUP_SUCCESS: bytes=%q err=%v", got, err)
				}
			} else if got != nil {
				t.Errorf("ASSERT_CLEANUP_NO_BYTES: %q", got)
			}
			wantOrder := []string{"parent1", "parent0"}
			if data != nil {
				wantOrder = append([]string{"data"}, wantOrder...)
				if data.closes != 1 {
					t.Errorf("ASSERT_CLEANUP_ONCE: data=%d", data.closes)
				}
			}
			for _, p := range parents {
				if p.closes != 1 {
					t.Errorf("ASSERT_CLEANUP_ONCE: %s=%d", p.name, p.closes)
				}
			}
			if !reflect.DeepEqual(order, wantOrder) {
				t.Errorf("cleanup order: %v want %v", order, wantOrder)
			}
			if h.data != nil || len(h.parents) != 0 {
				t.Error("ownership not relinquished")
			}
			checkedReadRootLive(t, root)
		})
	}
}
