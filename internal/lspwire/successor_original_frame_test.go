package lspwire

import (
	"bytes"
	"errors"
	"testing"
)

type successorOwnedFrameCapture struct {
	frame []byte
	lease SuccessorAllocationLease
	calls int
}

func (c *successorOwnedFrameCapture) ObserveOriginalFrame([]byte) {
	panic("legacy frame capture used")
}

func (c *successorOwnedFrameCapture) ObserveOriginalFrameOwned(frame []byte, lease SuccessorAllocationLease) {
	c.calls++
	c.frame, c.lease = frame, lease
}

func TestSuccessorIngressOriginalFrameLeaseTransfersExactBacking(t *testing.T) {
	body := successorMessage()
	frame := append(successorHeader(len(body)), body...)
	owner := &successorAllocationOwnerProbe{}
	capture := &successorOwnedFrameCapture{}
	opts := testSuccessorOptions()
	opts.AllocationOwner, opts.Capture = owner, capture
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if capture.calls != 1 || !bytes.Equal(capture.frame, frame) || capture.lease == nil {
		t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_FRAME_TRANSFER capture=%+v", capture)
	}
	var frameLease *successorAllocationLeaseProbe
	for i, role := range owner.roles {
		if role == SuccessorAllocationOriginalFrame {
			if owner.capacities[i] != uint64(len(frame)) {
				t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_FRAME_EXACT capacity=%d want=%d", owner.capacities[i], len(frame))
			}
			frameLease = owner.leases[i]
		}
	}
	if frameLease == nil || frameLease.releases != 0 {
		t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_FRAME_OWNED lease=%+v", frameLease)
	}
	capture.lease.Release()
	if frameLease.releases != 1 {
		t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_FRAME_RELEASE releases=%d", frameLease.releases)
	}
	r.Close()
}

func TestSuccessorIngressOriginalFrameRefusalPrecedesCapture(t *testing.T) {
	body := successorMessage()
	frame := append(successorHeader(len(body)), body...)
	owner := &successorAllocationOwnerProbe{rejectRole: SuccessorAllocationOriginalFrame}
	capture := &successorOwnedFrameCapture{}
	opts := testSuccessorOptions()
	opts.AllocationOwner, opts.Capture = owner, capture
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); !errors.Is(err, errIngressReserve) {
		t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_FRAME_REFUSAL err=%v", err)
	}
	if capture.calls != 0 || capture.frame != nil || capture.lease != nil {
		t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_FRAME_REFUSAL_ZERO_CAPTURE capture=%+v", capture)
	}
	r.Close()
}
