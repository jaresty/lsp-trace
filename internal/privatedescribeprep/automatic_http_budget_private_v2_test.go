package privatedescribeprep

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// This observer delegates to the real approved transport. It counts bytes
// delivered by Response.Body.Read, not TLS traffic, buffering, headers or heap.
type automaticConsumedHTTP struct {
	transport              http.RoundTripper
	mu                     sync.Mutex
	reads                  []int
	requests, requestBytes int
}

func (o *automaticConsumedHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := o.transport.RoundTrip(r)
	if err != nil {
		return resp, err
	}
	o.mu.Lock()
	slot := len(o.reads)
	o.reads = append(o.reads, 0)
	o.mu.Unlock()
	resp.Body = &automaticConsumedBody{ReadCloser: resp.Body, observe: func(n int) { o.mu.Lock(); o.reads[slot] += n; o.mu.Unlock() }}
	return resp, nil
}

type automaticConsumedBody struct {
	io.ReadCloser
	observe func(int)
}

func (b *automaticConsumedBody) Read(p []byte) (int, error) {
	n, e := b.ReadCloser.Read(p)
	b.observe(n)
	return n, e
}
func (o *automaticConsumedHTTP) snapshot() (int, int, []int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.requests, o.requestBytes, append([]int(nil), o.reads...)
}
func automaticConsumedAssert(t *testing.T, name string, ok bool, detail any) {
	t.Helper()
	if !ok {
		t.Errorf("%s: FAIL %v", name, detail)
	} else {
		t.Logf("%s: PASS %v", name, detail)
	}
}
func automaticConsumedTLS(t *testing.T, l AutomaticLimitsPrivateV2) (*automaticHTTPBackend, *automaticHTTPBackend, *automaticConsumedHTTP) {
	t.Helper()
	o := &automaticConsumedHTTP{}
	// Always oversized, including the final aggregate-limited exchange.
	payload := strings.Repeat("x", 2<<20)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, e := io.ReadAll(r.Body)
		if e != nil {
			t.Errorf("owned request read: %v", e)
		}
		o.mu.Lock()
		o.requests++
		o.requestBytes += len(raw)
		o.mu.Unlock()
		_, _ = io.WriteString(w, payload) // Early client close is expected on overflow.
	}))
	t.Cleanup(srv.Close)
	c := AutomaticHTTPConfigPrivateV2{Enabled: true, Endpoint: srv.URL, InterpreterModel: "owned-i", ReviewerModel: "owned-v", ScopeSHA256: pilot.Digest([]byte("owned consumed-byte fixture")), TimeoutMS: 5000, Limits: l}
	c.ApprovalSHA256 = AutomaticHTTPApprovalPrivateV2(c)
	i, v, e := NewAutomaticHTTPPrivateV2(c, c.ScopeSHA256, nil)
	if e != nil {
		t.Fatal(e)
	}
	ib, vb := i.(*automaticHTTPBackend), v.(*automaticHTTPBackend)
	tr := ib.client.Transport.(*http.Transport)
	tr.TLSClientConfig = srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	o.transport = tr
	// Instrumentation observes the existing transport; no policy is replaced.
	ib.client.Transport = o
	if vb.client != ib.client {
		t.Fatal("owned HTTP pair must share client")
	}
	return ib, vb, o
}
func TestAutomaticHTTPConsumedBudgetFourTargets(t *testing.T) {
	t.Log("procedure: four tiny prepared own targets -> AutomatePrivateV2 with actual owned TLS HTTP oversized bodies -> independently report consumed bytes, send count and complete target dispositions")
	l := automaticTestLimits()
	l.MaxResponseBytes = 1 << 20
	l.MaxTotalBytes = 3 << 20
	i, v, o := automaticConsumedTLS(t, l)
	h := automaticTestOwner(t, 4)
	a, e := AutomatePrivateV2(context.Background(), h, l, i, v)
	got := a.Inventory()
	calls, requests, reads := o.snapshot()
	consumed := requests
	for _, n := range reads {
		consumed += n
	}
	automaticConsumedAssert(t, "ASSERT_AUTO_HTTP_AGGREGATE_CHARGE", e == nil && got.Bytes == l.MaxTotalBytes && consumed == got.Bytes+1, fmt.Sprint("bytes=", got.Bytes, " request=", requests, " reads=", reads, " error=", e))
	automaticConsumedAssert(t, "ASSERT_AUTO_HTTP_AGGREGATE_NO_FOURTH", calls == 3 && got.Calls == calls && len(reads) == 3, fmt.Sprint("HTTP=", calls, " reported=", got.Calls, " reads=", reads))
	complete := e == nil && got.Status == "PROVISIONAL_WITH_UNRESOLVED" && len(got.Rows) == 4 && len(got.Candidates) == 0
	for n, row := range got.Rows {
		complete = complete && row.Target == fmt.Sprintf("target-%d", n) && row.Status == "BUDGET_EXHAUSTED" && len(row.Revisions) == 1
	}
	automaticConsumedAssert(t, "ASSERT_AUTO_HTTP_SPENT_TARGETS_ACCOUNTED", complete, got.Rows)
	automaticConsumedAssert(t, "ASSERT_AUTO_HTTP_TINY_REQUESTS", requests > 0 && requests < 4*(100<<10), requests)
}
func TestAutomaticHTTPConsumedBudgetPerResponse(t *testing.T) {
	t.Log("procedure: two actual owned TLS oversized HTTP reads under an unspent 3MiB aggregate -> exact request plus consumed-body charge and permitted continuation")
	l := automaticTestLimits()
	l.MaxResponseBytes = 1 << 20
	l.MaxTotalBytes = 3 << 20
	i, _, o := automaticConsumedTLS(t, l)
	b := automaticBudget{limits: l}
	req := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE"}
	for n := 1; n <= 2; n++ {
		_, err := b.call(context.Background(), i, req)
		calls, q, reads := o.snapshot()
		consumed := q
		for _, r := range reads {
			consumed += r
		}
		ok := err != nil && automaticClass(err) == "BUDGET_EXHAUSTED" && calls == n && b.calls == n && len(reads) == n && b.bytes == consumed && b.bytes < l.MaxTotalBytes
		for _, r := range reads {
			ok = ok && r == l.MaxResponseBytes+1
		}
		automaticConsumedAssert(t, fmt.Sprintf("ASSERT_AUTO_HTTP_RESPONSE_PREFIX_%d", n), ok, fmt.Sprint("charged=", b.bytes, " request=", q, " reads=", reads, " calls=", calls, " error=", err))
	}
}

type automaticPrefixReadError struct{ left, read int }

func (b *automaticPrefixReadError) Read(p []byte) (int, error) {
	if b.left == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	n := min(len(p), b.left)
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}
	b.left -= n
	b.read += n
	if b.left == 0 {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}
func (*automaticPrefixReadError) Close() error { return nil }
func TestAutomaticHTTPConsumedBudgetReadError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix int
	}{{"within-cap", 8192}, {"overflow-prefix", 65537}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("procedure: actual HTTP adapter ReadAll/LimitReader consumes a non-empty fault-body prefix, then read error; budget must charge the independently counted prefix")
			body := &automaticPrefixReadError{left: tc.prefix}
			calls := 0
			backend := &automaticHTTPBackend{endpoint: "https://owned.invalid", model: "owned", timeout: time.Second, requestCap: 1 << 20, tokens: 256, client: &http.Client{Transport: automaticTestRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: r}, nil
			})}}
			l := automaticTestLimits()
			l.MaxResponseBytes = 65536
			l.MaxTotalBytes = 1 << 20
			req := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE"}
			q, err := backend.wireSize(req, l.MaxResponseBytes)
			if err != nil {
				t.Fatal(err)
			}
			b := automaticBudget{limits: l}
			_, err = b.call(context.Background(), backend, req)
			wantClass := "UNAVAILABLE"
			if tc.prefix > l.MaxResponseBytes {
				wantClass = "BUDGET_EXHAUSTED"
			}
			automaticConsumedAssert(t, "ASSERT_AUTO_HTTP_ERROR_PREFIX_CHARGED", err != nil && automaticClass(err) == wantClass && body.read == tc.prefix && backend.lastResponse == body.read && b.bytes == q+body.read && calls == 1 && b.calls == 1, fmt.Sprint("charged=", b.bytes, " request=", q, " read=", body.read, " class=", automaticClass(err)))
		})
	}
}

// Supplementary in-process metadata defense, not the HTTP resource witness.
type automaticBudgetNumericBackend struct {
	request, response, calls int
}

func (*automaticBudgetNumericBackend) Identity() string { return "owned-numeric-metadata" }
func (b *automaticBudgetNumericBackend) wireSize(AutomaticRequestPrivateV2, int) (int, error) {
	return b.request, nil
}
func (b *automaticBudgetNumericBackend) wireResponseBytes() int { return b.response }
func (b *automaticBudgetNumericBackend) Exchange(context.Context, AutomaticRequestPrivateV2, int) ([]byte, error) {
	b.calls++
	return nil, nil
}
func TestAutomaticHTTPConsumedBudgetNumericBounds(t *testing.T) {
	for _, tc := range []struct {
		name              string
		request, response int
		class             string
		charged, calls    int
	}{
		{"negative-request", -1, 0, "MALFORMED", 0, 0},
		{"negative-response", 1, -1, "MALFORMED", 1, 1},
		{"large-response", 1, int(^uint(0) >> 1), "BUDGET_EXHAUSTED", 3 << 20, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("procedure: supplementary in-process byte-metadata bounds -> charge never wraps or decreases")
			backend := &automaticBudgetNumericBackend{request: tc.request, response: tc.response}
			l := automaticTestLimits()
			l.MaxTotalBytes = 3 << 20
			b := automaticBudget{limits: l}
			_, e := b.call(context.Background(), backend, AutomaticRequestPrivateV2{})
			automaticConsumedAssert(t, "ASSERT_AUTO_BUDGET_NUMERIC_BOUNDS", e != nil && automaticClass(e) == tc.class && b.bytes == tc.charged && b.bytes >= 0 && b.bytes <= l.MaxTotalBytes && b.calls == tc.calls && backend.calls == tc.calls, fmt.Sprint("bytes=", b.bytes, " class=", automaticClass(e), " calls=", b.calls))
		})
	}
}
func TestAutomaticHTTPConsumedBudgetNoResponseAllowance(t *testing.T) {
	t.Log("procedure: exact remaining request cost leaves zero response allowance -> no HTTP exchange, no consumed bytes, no reported exchange")
	l := automaticTestLimits()
	i, _, o := automaticConsumedTLS(t, l)
	req := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE"}
	q, e := i.wireSize(req, l.MaxResponseBytes)
	if e != nil {
		t.Fatal(e)
	}
	l.MaxTotalBytes = q
	b := automaticBudget{limits: l}
	_, e = b.call(context.Background(), i, req)
	calls, _, reads := o.snapshot()
	automaticConsumedAssert(t, "ASSERT_AUTO_HTTP_ZERO_ALLOWANCE_NO_SEND", e != nil && automaticClass(e) == "BUDGET_EXHAUSTED" && calls == 0 && len(reads) == 0 && b.calls == 0 && b.bytes == 0, fmt.Sprint("HTTP=", calls, " reported=", b.calls, " charged=", b.bytes))
}
