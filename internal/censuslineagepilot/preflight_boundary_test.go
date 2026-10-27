package censuslineagepilot

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/publication"
)

func TestPreflightPrecancelNoOpen(t *testing.T) {
	req, _, _ := preflightFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := preflightWithOpen(ctx, req, func(string) (preflightRoot, error) { t.Fatal("opened after pre-cancel"); return nil, nil })
	if !errors.Is(err, context.Canceled) || got.state != nil {
		t.Fatal("cancellation lost", err)
	}
}
func TestPreflightOpenFailureClosesAcquiredRoot(t *testing.T) {
	req, _, _ := preflightFixture(t)
	r, err := publication.OpenRoot(req.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	f := &faultPreflightRoot{preflightRoot: r, closeErr: errors.New("close")}
	defer func() {
		if !f.closed {
			r.Close()
		}
	}()
	failed := errors.New("open")
	got, err := preflightWithOpen(context.Background(), req, func(string) (preflightRoot, error) { return f, failed })
	if got.state != nil || !f.closed || !errors.Is(err, failed) || !errors.Is(err, f.closeErr) {
		t.Fatal("open/close causes or handle lost", err)
	}
}

type cancelAfterReadRoot struct {
	preflightRoot
	cancel context.CancelFunc
}

func (r cancelAfterReadRoot) ReadContentObserved(c context.Context, id string, n int64) ([]byte, error) {
	raw, err := r.preflightRoot.ReadContentObserved(c, id, n)
	r.cancel()
	return raw, err
}
func TestPreflightCancelAfterRead(t *testing.T) {
	req, _, _ := preflightFixture(t)
	r, err := publication.OpenRoot(req.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	f := &faultPreflightRoot{preflightRoot: r}
	defer func() {
		if !f.closed {
			r.Close()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := preflightWithOpen(ctx, req, func(string) (preflightRoot, error) { return cancelAfterReadRoot{f, cancel}, nil })
	if got.state != nil || !f.closed || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled bytes escaped", err)
	}
}
func TestPreflightFixedReferenceSetBeforeOpen(t *testing.T) {
	req, _, _ := preflightFixture(t)
	cases := [][]Artifact{nil, req.Inputs[:1], append(append([]Artifact{}, req.Inputs...), req.Inputs[0])}
	sameID := append([]Artifact{}, req.Inputs...)
	sameID[1].Selector = sameID[0].Selector
	sameID[1].SHA256 = sameID[0].SHA256
	cases = append(cases, sameID)
	oversized := append([]Artifact{}, req.Inputs...)
	oversized[0].ByteLength = uint64(MaxSelectionInputBytes + 1)
	cases = append(cases, oversized)
	for _, refs := range cases {
		req.Inputs = refs
		got, err := preflightWithOpen(context.Background(), req, func(string) (preflightRoot, error) { t.Fatal("invalid references opened root"); return nil, nil })
		if err == nil || got.state != nil {
			t.Fatal("invalid reference set admitted")
		}
	}
}
