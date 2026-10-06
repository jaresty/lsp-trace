package lspwire

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

const (
	testSuccessorHeaderLimit  = uint64(65536)
	testSuccessorFrameLimit   = uint64(2097152)
	testSuccessorConsumeLimit = uint64(8388608)
	testSuccessorAcquireLimit = uint64(8392705)
)

func testSuccessorOptions() SuccessorIngressOptions {
	return SuccessorIngressOptions{
		OwnerID: "unit1-owner",
		Limits: SuccessorIngressLimits{
			HeaderBytes: testSuccessorHeaderLimit, FrameBytes: testSuccessorFrameLimit,
			ConsumptionBytes: testSuccessorConsumeLimit, AcquisitionBytes: testSuccessorAcquireLimit,
			MaxReadBytes: 4096, PrefetchBytes: 4096,
			HistoryAcquiredBytes: 8388608, HistoryOutstandingBytes: 8388608,
		},
	}
}

func successorMessage() []byte     { return []byte(`{"jsonrpc":"2.0","method":"x"}`) }
func successorHeader(n int) []byte { return []byte("Content-Length: " + itoa(n) + "\r\n\r\n") }
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type successorAllocationLeaseProbe struct{ releases int }

func (l *successorAllocationLeaseProbe) Release() { l.releases++ }

type successorAllocationOwnerProbe struct {
	role       SuccessorAllocationRole
	capacity   uint64
	lease      *successorAllocationLeaseProbe
	roles      []SuccessorAllocationRole
	capacities []uint64
	leases     []*successorAllocationLeaseProbe
	reject     bool
	rejectRole SuccessorAllocationRole
}

func (o *successorAllocationOwnerProbe) ReserveSuccessorAllocation(role SuccessorAllocationRole, capacity uint64) (SuccessorAllocationLease, error) {
	o.role, o.capacity = role, capacity
	o.roles = append(o.roles, role)
	o.capacities = append(o.capacities, capacity)
	if o.reject || o.rejectRole == role {
		return nil, errors.New("refused")
	}
	o.lease = &successorAllocationLeaseProbe{}
	o.leases = append(o.leases, o.lease)
	return o.lease, nil
}

func TestSuccessorIngressPrefetchAllocationLease(t *testing.T) {
	owner := &successorAllocationOwnerProbe{}
	opts := testSuccessorOptions()
	opts.AllocationOwner = owner
	r, err := NewSuccessorIngressReader(bytes.NewReader(nil), opts)
	if err != nil || owner.role != SuccessorAllocationPrefetch || owner.capacity != 4096 || owner.lease == nil {
		t.Fatalf("prefetch reservation: reader=%v err=%v owner=%+v", r, err, owner)
	}
	r.Close()
	r.Close()
	if owner.lease.releases != 1 {
		t.Fatalf("prefetch release count=%d", owner.lease.releases)
	}

	reject := &successorAllocationOwnerProbe{reject: true}
	opts.AllocationOwner = reject
	if got, err := NewSuccessorIngressReader(bytes.NewReader(nil), opts); got != nil || !errors.Is(err, errIngressReserve) {
		t.Fatalf("prefetch refusal: reader=%v err=%v", got, err)
	}
}

func TestSuccessorIngressHeaderAllocationLease(t *testing.T) {
	body := successorMessage()
	frame := append(successorHeader(len(body)), body...)
	owner := &successorAllocationOwnerProbe{}
	opts := testSuccessorOptions()
	opts.AllocationOwner = owner
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if len(owner.roles) != 5 || owner.roles[1] != SuccessorAllocationHeader || owner.capacities[1] != testSuccessorHeaderLimit+1 ||
		owner.roles[2] != SuccessorAllocationConsumeScratch || owner.capacities[2] != uint64(len(successorHeader(len(body)))) ||
		owner.roles[3] != SuccessorAllocationBody || owner.capacities[3] != uint64(len(body)) ||
		owner.roles[4] != SuccessorAllocationConsumeScratch || owner.capacities[4] != uint64(len(body)) {
		t.Fatalf("frame reservation roles=%v capacities=%v", owner.roles, owner.capacities)
	}
	for i, lease := range owner.leases {
		want := 1
		if i == 0 {
			want = 0
		}
		if lease.releases != want {
			t.Fatalf("frame lifetime lease[%d]=%d want=%d", i, lease.releases, want)
		}
	}
	r.Close()
	for i, lease := range owner.leases {
		if lease.releases != 1 {
			t.Fatalf("close lifetime lease[%d]=%d", i, lease.releases)
		}
	}

	reject := &successorAllocationOwnerProbe{rejectRole: SuccessorAllocationHeader}
	opts.AllocationOwner = reject
	r, err = NewSuccessorIngressReader(bytes.NewReader(frame), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); !errors.Is(err, errIngressReserve) {
		t.Fatalf("header refusal err=%v", err)
	}
	r.Close()
	if len(reject.leases) != 1 || reject.leases[0].releases != 1 {
		t.Fatalf("header refusal prefetch cleanup leases=%v", reject.leases)
	}

	reject = &successorAllocationOwnerProbe{rejectRole: SuccessorAllocationBody}
	opts.AllocationOwner = reject
	r, err = NewSuccessorIngressReader(bytes.NewReader(frame), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); !errors.Is(err, errIngressReserve) {
		t.Fatalf("body refusal err=%v", err)
	}
	r.Close()
	if len(reject.leases) != 3 {
		t.Fatalf("body refusal cleanup leases=%v", reject.leases)
	}
	for i, lease := range reject.leases {
		if lease.releases != 1 {
			t.Fatalf("body refusal lease[%d]=%d", i, lease.releases)
		}
	}
}

func TestSuccessorIngressConsumeScratchPartialAndRefusal(t *testing.T) {
	owner := &successorAllocationOwnerProbe{}
	r := &SuccessorIngressReader{
		options:  SuccessorIngressOptions{AllocationOwner: owner, Limits: SuccessorIngressLimits{ConsumptionBytes: 1}},
		prefetch: []byte("abc"),
	}
	var dst []byte
	if err := r.consumeCopy(&dst, 3); !errors.Is(err, errIngressConsume) {
		t.Fatalf("partial consume err=%v", err)
	}
	if string(dst) != "ab" || string(r.prefetch) != "c" || len(owner.roles) != 1 || owner.roles[0] != SuccessorAllocationConsumeScratch || owner.capacities[0] != 3 || owner.leases[0].releases != 1 {
		t.Fatalf("partial consume dst=%q prefetch=%q roles=%v capacities=%v", dst, r.prefetch, owner.roles, owner.capacities)
	}

	reject := &successorAllocationOwnerProbe{rejectRole: SuccessorAllocationConsumeScratch}
	r = &SuccessorIngressReader{
		options:  SuccessorIngressOptions{AllocationOwner: reject, Limits: SuccessorIngressLimits{ConsumptionBytes: 10}},
		prefetch: []byte("xyz"),
	}
	dst = []byte("keep")
	if err := r.consumeCopy(&dst, 9); !errors.Is(err, errIngressReserve) {
		t.Fatalf("scratch refusal err=%v", err)
	}
	if string(dst) != "keep" || string(r.prefetch) != "xyz" || r.consumed != 0 || len(reject.roles) != 1 || reject.capacities[0] != 3 {
		t.Fatalf("scratch refusal dst=%q prefetch=%q consumed=%d roles=%v capacities=%v", dst, r.prefetch, r.consumed, reject.roles, reject.capacities)
	}
}

type successorCapture struct{ frames [][]byte }

func (c *successorCapture) ObserveOriginalFrame(p []byte) {
	c.frames = append(c.frames, append([]byte(nil), p...))
}

type successorScriptRead struct {
	data []byte
	err  error
	want int
}
type successorScriptReader struct {
	t     *testing.T
	reads []successorScriptRead
	calls int
}

func (r *successorScriptReader) Read(p []byte) (int, error) {
	r.t.Helper()
	if r.calls >= len(r.reads) {
		r.t.Fatalf("unexpected read %d", r.calls+1)
	}
	x := r.reads[r.calls]
	r.calls++
	if x.want != 0 && len(p) != x.want {
		r.t.Fatalf("read %d request=%d want=%d", r.calls, len(p), x.want)
	}
	if len(p) > 4096 {
		r.t.Fatalf("read %d request=%d exceeds 4096", r.calls, len(p))
	}
	copy(p, x.data)
	return len(x.data), x.err
}

func TestSuccessorIngressHistoryBoundsAndNoRetrocharge(t *testing.T) {
	o := testSuccessorOptions()
	o.History = &ImmutableHistoryMetadata{Identity: "history", Cut: "cut", Entries: 1, CutOrdinal: 1, Acquired: 8388608, Outstanding: 8388608}
	frame := append(successorHeader(30), successorMessage()...)
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), o)
	if err != nil {
		t.Fatalf("at-bound constructor: %v", err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatalf("read: %v", err)
	}
	if r.acquired != 52 || r.consumed != 52 || r.validated != 52 {
		t.Fatalf("A/C/V=%d/%d/%d want 52/52/52", r.acquired, r.consumed, r.validated)
	}
	if o.History.Acquired != 8388608 || o.History.Outstanding != 8388608 {
		t.Fatalf("history mutated: %+v", *o.History)
	}

	o.History = &ImmutableHistoryMetadata{Identity: "history", Cut: "cut", Entries: 1, CutOrdinal: 1, Acquired: 8388609}
	if _, err := NewSuccessorIngressReader(bytes.NewReader(nil), o); err == nil {
		t.Fatal("history acquired over bound accepted")
	}
}

func TestSuccessorIngressFullEOFAndShortEOF(t *testing.T) {
	header, body := successorHeader(30), successorMessage()
	for _, tc := range []struct {
		name    string
		body    []byte
		bodyErr error
		wantErr bool
		a, c, v uint64
	}{
		{"full", body, io.EOF, false, 52, 52, 52},
		{"short", body[:29], io.EOF, true, 51, 51, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &successorScriptReader{t: t, reads: []successorScriptRead{{header, nil, 4096}, {tc.body, tc.bodyErr, 30}}}
			r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
			if err != nil {
				t.Fatal(err)
			}
			_, got := r.ReadFrame()
			if (got != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", got, tc.wantErr)
			}
			if r.acquired != tc.a || r.consumed != tc.c || r.validated != tc.v {
				t.Fatalf("A/C/V=%d/%d/%d want %d/%d/%d", r.acquired, r.consumed, r.validated, tc.a, tc.c, tc.v)
			}
		})
	}
}

func TestSuccessorIngressCarryConsumesBeforeRead(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	first := append(append([]byte(nil), frame...), successorHeader(30)[:5]...)
	second := append(append([]byte(nil), successorHeader(30)[5:]...), successorMessage()...)
	s := &successorScriptReader{t: t, reads: []successorScriptRead{{first, nil, 4096}, {second, nil, 4096}}}
	r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if r.acquired != 57 || r.consumed != 52 || len(r.prefetch) != 5 {
		t.Fatalf("first A/C/prefetch=%d/%d/%d", r.acquired, r.consumed, len(r.prefetch))
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if s.calls != 2 || r.acquired != 104 || r.consumed != 104 || r.validated != 104 || len(r.prefetch) != 0 {
		t.Fatalf("final calls/A/C/V/prefetch=%d/%d/%d/%d/%d", s.calls, r.acquired, r.consumed, r.validated, len(r.prefetch))
	}
}

func TestSuccessorIngressReturnedBytesChargedBeforeTransportError(t *testing.T) {
	errX := errors.New("errX")
	header := successorHeader(30)
	s := &successorScriptReader{t: t, reads: []successorScriptRead{{header, nil, 4096}, {[]byte("xx"), errX, 30}}}
	r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	_, got := r.ReadFrame()
	if !errors.Is(got, errX) {
		t.Fatalf("err=%v want errX", got)
	}
	if r.acquired != 24 || r.consumed != 24 || r.validated != 0 {
		t.Fatalf("A/C/V=%d/%d/%d want 24/24/0", r.acquired, r.consumed, r.validated)
	}
}
