package sessionruntime

import (
	"context"
	"errors"
	"io"
	"sync"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

// ownedTransport is confined to one exclusively admitted protocol operation.
// Its child is captured at admission, never looked up again during teardown.
// Starter-provided wire children must interrupt outstanding pipe I/O on Close
// or Teardown. The managed process implements this with owned pipes and its
// existing process/group kill-and-reap path. Arbitrary uninterruptible Reader /
// Writer implementations are not supported by this runtime ownership contract.
type ownedTransport struct {
	child     Child
	once      sync.Once
	resources managedprocess.ResourceObservation
	teardown  managedprocess.TeardownObservation
}

func (o *ownedTransport) retire() {
	o.once.Do(func() {
		// Close before reaping: a server need not cooperate, drain input, or exit
		// voluntarily to release the host's pipe operations.
		o.resources = o.child.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Do not spend a graceful-shutdown interval after a deadline.
		o.teardown = o.child.Teardown(ctx)
	})
}

// run joins the owned I/O goroutine on every path. A cancelled/failed framed
// write must retire this owner before its protocol lease can be released.
func (o *ownedTransport) run(ctx context.Context, io func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- io() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		o.retire()
		<-done
		return ctx.Err()
	}
}

// checkedWriter does not treat a short framed write as confirmed supply, even
// when a custom transport violates io.Writer's short-write error convention.
type checkedWriter struct{ io.Writer }

func (w checkedWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func contextFailure(ctx context.Context) session.Failure {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return session.RequestTimeout
	}
	if ctx.Err() != nil {
		return session.RequestCancelled
	}
	return ""
}
