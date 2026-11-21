package publication

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type cancelReadFile struct {
	*os.File
	cancel        context.CancelFunc
	reads, closes int
	closeErr      error
}

func (f *cancelReadFile) Read(p []byte) (int, error) {
	f.reads++
	n, e := f.File.Read(p[:min(len(p), 1)])
	f.cancel()
	return n, e
}
func (f *cancelReadFile) Close() error { f.closes++; return errors.Join(f.File.Close(), f.closeErr) }
func TestReadVerifiedBoundFileCheckedContext(t *testing.T) {
	dir, root := checkedReadRoot(t)
	checkedReadPublish(t, root, "keep", []byte("borrowed-root-live"))
	for _, kind := range []string{"cancel", "deadline", "success"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			if kind == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
				defer stop()
			}
			got, e := ReadVerifiedBoundFileCheckedContext(ctx, root, "keep", 64)
			if kind == "success" {
				if e != nil || string(got) != "borrowed-root-live" {
					t.Fatal(e, string(got))
				}
			} else {
				want := context.Canceled
				if kind == "deadline" {
					want = context.DeadlineExceeded
				}
				if !errors.Is(e, want) || got != nil {
					t.Fatalf("cancelled read returned bytes: %q %v", got, e)
				}
			}
			checkedReadRootLive(t, root)
		})
	}
	t.Run("between-stream-reads-and-close-error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f, e := os.Open(filepath.Join(dir, "keep"))
		if e != nil {
			t.Fatal(e)
		}
		before, e := f.Stat()
		if e != nil {
			t.Fatal(e)
		}
		closeErr := errors.New("reported close failure")
		data := &cancelReadFile{File: f, cancel: cancel, closeErr: closeErr}
		order := []string{}
		parentFile, e := os.Open(dir)
		if e != nil {
			t.Fatal(e)
		}
		parent := &checkedReadFaultParent{Closer: parentFile, name: "parent", order: &order}
		handles := boundFileReadHandles{data: data, parents: []io.Closer{parent}}
		got, e := handles.readContext(ctx, root.info, before, 64)
		if !errors.Is(e, context.Canceled) || !errors.Is(e, closeErr) || got != nil || data.reads != 1 || data.closes != 1 || parent.closes != 1 {
			t.Fatalf("stream cancellation/cleanup: reads=%d closes=%d/%d bytes=%q err=%v", data.reads, data.closes, parent.closes, got, e)
		}
		checkedReadRootLive(t, root)
	})
}
