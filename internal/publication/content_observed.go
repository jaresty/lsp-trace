package publication

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
)

var ErrContentObservation = errors.New("content observation rejected")
var ErrContentObservationUnsupported = errors.New("no-follow content observation unsupported")
var observedContentID = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type observedFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

// Per-call operations are private to permit deterministic fault tests without
// mutable package-global hooks or additional caller authority.
type contentReadOps struct {
	lstat func(*Root, string) (os.FileInfo, error)
	open  func(*Root, string) (observedFile, error)
}

// ReadContentObserved borrows the pinned root and observes one root-level content
// ID. It never creates, changes or removes a file. On any error, including close
// failure, no bytes escape. Cancellation is checked at safe read boundaries;
// regular-file reads are byte-bounded, not interruptible arbitrary I/O, and this
// API makes no elapsed-time guarantee. Do not close/use the root concurrently.
func (r *Root) ReadContentObserved(ctx context.Context, contentID string, limit int64) ([]byte, error) {
	return r.readContentObserved(ctx, contentID, limit, contentReadOps{lstat: statObservedContent, open: openObservedContent})
}
func (r *Root) readContentObserved(ctx context.Context, id string, limit int64, ops contentReadOps) ([]byte, error) {
	if !observedContentID.MatchString(id) {
		return nil, ErrContentObservation
	}
	return r.readObservedFile(ctx, id, limit, ops, true)
}

// Internal root-level selector observation; callers must validate the namespace.
func (r *Root) readObservedFile(ctx context.Context, id string, limit int64, ops contentReadOps, contentAddressed bool) (raw []byte, err error) {
	if ctx == nil || limit < 1 || limit == math.MaxInt64 {
		return nil, ErrContentObservation
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = r.ValidatePrivate(); err != nil {
		return nil, errors.Join(ErrContentObservation, err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	before, err := ops.lstat(r, id)
	if err != nil {
		return nil, err
	}
	if !r.validObservedInfo(before) || before.Size() < 0 || before.Size() > limit {
		return nil, ErrContentObservation
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	f, err := ops.open(r, id)
	if f != nil {
		defer func() {
			err = errors.Join(err, f.Close(), ctx.Err())
			if err != nil {
				raw = nil
			}
		}()
	}
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, ErrContentObservation
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !sameObservedInfo(before, opened) || !r.validObservedInfo(opened) {
		return nil, ErrContentObservation
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	raw, err = io.ReadAll(io.LimitReader(contextObservedReader{ctx, f}, limit+1))
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit || int64(len(raw)) != opened.Size() {
		return nil, ErrContentObservation
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !sameObservedInfo(opened, after) || !r.validObservedInfo(after) {
		return nil, ErrContentObservation
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	final, err := ops.lstat(r, id)
	if err != nil {
		return nil, err
	}
	if !sameObservedInfo(after, final) || !r.validObservedInfo(final) {
		return nil, ErrContentObservation
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = r.ValidatePrivate(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if contentAddressed && fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) != id {
		return nil, ErrContentObservation
	}
	return raw, nil
}
func (r *Root) validObservedInfo(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) == 0 && validPublishedMetadata(r.info, info)
}
func sameObservedInfo(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Size() == b.Size() && a.Mode() == b.Mode() && a.ModTime().Equal(b.ModTime())
}

type contextObservedReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextObservedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}
