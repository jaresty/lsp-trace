package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/internal/lspwire"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"
)

type captureTestSink struct {
	b                           bytes.Buffer
	short, writeFail, closeFail bool
	writes                      int
	onClose                     func()
}

func (s *captureTestSink) Write(p []byte) (int, error) {
	s.writes++
	if s.writeFail {
		return 0, errors.New("injected")
	}
	if s.short {
		return len(p) - 1, nil
	}
	return s.b.Write(p)
}
func (s *captureTestSink) Close() error {
	if s.onClose != nil {
		s.onClose()
	}
	if s.closeFail {
		return errors.New("injected")
	}
	return nil
}
func captureTestState(s *captureTestSink) *privateDiscoveryCapture {
	return &privateDiscoveryCapture{open: func(int) (io.WriteCloser, error) { return s, nil }}
}
func TestCapture88PrefixAndQuota(t *testing.T) {
	for _, n := range []int{2, 8192, 8193} {
		s := new(captureTestSink)
		c := captureTestState(s)
		raw := bytes.Repeat([]byte{'x'}, n+10)
		raw[5] = '{'
		raw[5+n-1] = '}'
		r := c.capture(context.Background(), raw, 5, 5+n)
		want := CaptureComplete
		if n > 8192 {
			want = CaptureTruncated
		}
		if r.Status != want || int(r.Bytes) != min(n, 8192) || r.OriginalBytes != int64(n) || !r.Valid() {
			t.Fatal("ASSERT_CAPTURE88_PREFIX")
		}
		sum := sha256.Sum256(raw[5 : 5+min(n, 8192)])
		if sum != r.PrefixSHA256 || !bytes.Equal(s.b.Bytes(), raw[5:5+min(n, 8192)]) {
			t.Fatal("ASSERT_CAPTURE88_PREFIX_DIGEST")
		}
		raw[5] = 'z'
		if s.b.Bytes()[0] != '{' {
			t.Fatal("ASSERT_CAPTURE88_OWNERSHIP")
		}
	}
	s := new(captureTestSink)
	c := captureTestState(s)
	for i := 0; i < 2; i++ {
		if c.capture(context.Background(), []byte("{}"), 0, 2).Status != CaptureComplete {
			t.Fatal("ASSERT_CAPTURE88_TWO")
		}
	}
	if r := c.capture(context.Background(), []byte("{}"), 0, 2); r.Status != CaptureUnavailable || r.Reason != CaptureQuotaExhausted || s.writes != 2 {
		t.Fatal("ASSERT_CAPTURE88_QUOTA")
	}
}
func TestCapture88SinkFailuresAndDeadline(t *testing.T) {
	for _, s := range []*captureTestSink{{short: true}, {writeFail: true}, {closeFail: true}} {
		c := captureTestState(s)
		r := c.capture(context.Background(), []byte("{}"), 0, 2)
		if r.Status != CaptureFailed || !r.Valid() {
			t.Fatal("ASSERT_CAPTURE88_SINK_FAILURE")
		}
		if s.closeFail && !c.uncertain {
			t.Fatal("ASSERT_CAPTURE88_CLOSE_UNCERTAIN")
		}
	}
	s := new(captureTestSink)
	c := captureTestState(s)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if r := c.capture(ctx, []byte("{}"), 0, 2); r.Status != CaptureFailed || r.Reason != CaptureDeadline || s.writes != 0 {
		t.Fatal("ASSERT_CAPTURE88_DEADLINE")
	}
}
func TestCapture88OpenFailure(t *testing.T) {
	for _, fault := range []error{errors.New("injected open failure"), errPrivateCaptureCloseUncertain} {
		w, c, _ := discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":0}}`))
		w.capture = &privateDiscoveryCapture{open: func(int) (io.WriteCloser, error) { return nil, fault }}
		w.prepayResponseFacts()
		m := response76Runtime(w, c)
		before := w.refusal
		result := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
		record := w.responseFactsSnapshot().Records[0].Capture
		if result.Failure != "" || result.ServerError == nil || result.ServerError.Code != 0 || w.failed || w.refusal != before || record.Status != CaptureFailed || record.BytesAvailable || record.DigestAvailable || !record.Valid() {
			t.Fatal("ASSERT_CAPTURE88_OPEN_PRIMARY")
		}
		if errors.Is(fault, errPrivateCaptureCloseUncertain) && w.capture.close(context.Background()) {
			t.Fatal("ASSERT_CAPTURE88_OPEN_CLOSE_UNCERTAIN")
		}
	}
}

func TestCapture88AdmissionAndDisabled(t *testing.T) {
	w, _, inert := discoveryWindow(nil)
	before := w.remaining
	if !w.prepayCapture(nil, func(PrivateDiscoveryCaptureQuote) error { t.Fatal("ASSERT_CAPTURE88_DISABLED_ADMISSION"); return nil }) || w.capture != nil || w.remaining != before || inert.reads != 0 {
		t.Fatal("ASSERT_CAPTURE88_DISABLED")
	}
	if testing.AllocsPerRun(32, func() {
		if !w.prepayCapture(nil, nil) {
			t.Fatal("ASSERT_CAPTURE88_DISABLED")
		}
	}) != 0 {
		t.Fatal("ASSERT_CAPTURE88_DISABLED_ALLOCATIONS")
	}
	options := &PrivateDiscoveryCaptureOptions{Permission: true, Destination: "/not-opened"}
	q, ok := PrivateDiscoveryCaptureQuoteForOptions(options)
	if !ok {
		t.Fatal("quote")
	}
	n := uint64(unsafe.Sizeof(privateDiscoveryCapture{})) + 16384
	for _, currency := range []string{"retained", "work"} {
		w, _, inert = discoveryWindow(nil)
		if currency == "retained" {
			w.remaining.RetainedBytes = n - 1
		} else {
			w.remaining.Work = q.Work - 1
		}
		if w.prepayCapture(options, func(PrivateDiscoveryCaptureQuote) error { return nil }) || w.capture != nil || inert.reads != 0 {
			t.Fatal("ASSERT_CAPTURE88_PREPAY")
		}
	}
	w, _, _ = discoveryWindow(nil)
	if w.prepayCapture(options, func(PrivateDiscoveryCaptureQuote) error { return errors.New("refused") }) || w.capture != nil {
		t.Fatal("ASSERT_CAPTURE88_FUNDING")
	}
	if _, ok := PrivateDiscoveryCaptureQuoteForOptions(&PrivateDiscoveryCaptureOptions{Destination: "/not-opened"}); ok {
		t.Fatal("ASSERT_CAPTURE88_PERMISSION")
	}
}
func TestCapture88BeforeConstructorAndFinalize(t *testing.T) {
	w, _, _ := discoveryWindow(nil)
	calls := 0
	c := PrivateDiscoveryConstruction{NewStarter: func() (Starter, error) { calls++; return nil, errors.New("must not construct") }, Limits: w.limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "inert"}}, FailureCapture: &PrivateDiscoveryCaptureOptions{Permission: false, Destination: "/not-opened"}}
	if _, e := RunPrivateDiscovery(context.Background(), c, func(context.Context, *PrivateDiscoverySession) error { return nil }); e == nil || calls != 0 {
		t.Fatal("ASSERT_CAPTURE88_BEFORE_CONSTRUCTOR")
	}
	ctx, cancel := context.WithCancel(context.Background())
	sink := &captureTestSink{onClose: cancel}
	capture := captureTestState(sink)
	r := capture.capture(ctx, []byte("{}"), 0, 2)
	if r.Status != CaptureFailed || r.Reason != CaptureDeadline || !capture.uncertain {
		t.Fatal("ASSERT_CAPTURE88_FINALIZATION_DEADLINE")
	}
	t.Logf("compiler capture backing=%d response record=%d store charge=%d", unsafe.Sizeof(privateDiscoveryCapture{}), unsafe.Sizeof(PrivateDiscoveryResponseRecord{}), privateResponseFactsCharge)
}

func TestCapture88SelectedIntegration(t *testing.T) {
	for _, tc := range []struct {
		body    string
		want    PrivateCaptureStatus
		written string
	}{
		{`{"jsonrpc":"2.0","id":1,"error":{"code":0}}`, CaptureComplete, `{"code":0}`},
		{`{"jsonrpc":"2.0","id":1,"error":{}}`, CaptureComplete, `{}`},
		{`{"jsonrpc":"2.0","id":1,"result":[]}`, CaptureAbsent, ""},
		{`{"jsonrpc":"2.0","id":1,"result":[],"error":null}`, CaptureAbsent, ""},
		{`{"jsonrpc":"2.0","id":1,"result":["PRIVATE_RESULT"],"error":{"code":0}}`, CaptureNotCaptured, ""},
		{`{"jsonrpc":"2.0","id":1,"error":{"code":true}}`, CaptureNotCaptured, ""},
	} {
		w, c, _ := discoveryWindow(discoveryWire(tc.body))
		sink := new(captureTestSink)
		w.capture = captureTestState(sink)
		if !w.prepayResponseFacts() {
			t.Fatal("prepay")
		}
		m := response76Runtime(w, c)
		m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
		d := w.responseFactsSnapshot()
		if !d.Valid() || d.Count != 1 || d.Records[0].Capture.Status != tc.want || sink.b.String() != tc.written {
			t.Fatal("ASSERT_CAPTURE88_SELECTED_OBJECT_ONLY")
		}
	}
	wire := append(discoveryWire(`{"jsonrpc":"2.0","id":9,"error":{"code":0}}`), discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":0}}`)...)
	w, c, _ := discoveryWindow(wire)
	sink := new(captureTestSink)
	w.capture = captureTestState(sink)
	if !w.prepayResponseFacts() {
		t.Fatal("prepay")
	}
	m := response76Runtime(w, c)
	m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
	if w.capture.count != 1 || sink.writes != 1 || w.responseFactsSnapshot().Records[0].ReadAttempts != 2 {
		t.Fatal("ASSERT_CAPTURE88_UNMATCHED_QUOTA")
	}
	w, c, _ = discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":7,"error":{"code":0}}`))
	w.capture = captureTestState(new(captureTestSink))
	w.prepayResponseFacts()
	i, _ := w.beginResponseFacts()
	f, e := w.frameRecord(c, 7)
	if e != nil {
		t.Fatal("fixture")
	}
	for _, disp := range []PrivateDiscoveryResponseDisposition{PrivateResponseBoundary, PrivateResponseLate, PrivateResponseJoined} {
		w.observeResponseFacts(i, f, disp)
	}
	if w.capture.count != 0 {
		t.Fatal("ASSERT_CAPTURE88_LATE_JOINED")
	}
	if f.facts.Association != lspwire.ResponseIDMatched {
		t.Fatal("fixture association")
	}
}
func TestCapture88CancellationEnabled(t *testing.T) {
	w, c, _ := discoveryWindow(nil)
	pipe, writer := io.Pipe()
	defer writer.Close()
	inert := &response76BlockingChild{in: pipe, started: make(chan struct{})}
	c.wireChild = inert
	sink := new(captureTestSink)
	w.capture = captureTestState(sink)
	w.prepayResponseFacts()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.ctx = ctx
	m := response76Runtime(w, c)
	done := make(chan struct{})
	go func() {
		m.RoundTrip(ctx, RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
		close(done)
	}()
	<-inert.started
	cancel()
	<-done
	snapshot := w.responseFactsSnapshot()
	if !snapshot.Valid() || snapshot.Count != 1 || snapshot.Records[0].Selected != (lspwire.ResponseFacts{}) || w.capture.count != 0 || sink.writes != 0 {
		t.Fatal("ASSERT_CAPTURE88_CANCEL_NO_CAPTURE")
	}
}

func TestCapture88Summary(t *testing.T) {
	w, _, _ := discoveryWindow(nil)
	w.prepayResponseFacts()
	for i := 0; i < 128; i++ {
		w.beginResponseFacts(PrivateDiscoveryCorrelation{Method: PrivateMethodPrepareCallHierarchy})
	}
	d := w.responseFactsSnapshot()
	s, e := PrivateDiscoverySummarize(d)
	if e != nil || !s.Valid() || s.Total != 128 || s.Returned != 2 || s.Omitted != 126 || s.RetentionTruncated || !s.Truncated || s.Records[0].Ordinal != 128 {
		t.Fatal("ASSERT_CAPTURE88_SUMMARY_COUNTS")
	}
	raw, e := json.Marshal(s)
	if e != nil || len(raw) > 47000 || bytes.Contains(raw, []byte("PRIVATE_RESULT")) {
		t.Fatal("ASSERT_CAPTURE88_SUMMARY_BOUND")
	}
	var decoded PrivateDiscoveryResponseSummary
	for _, bad := range [][]byte{bytes.Replace(raw, []byte(`"returned":2`), []byte(`"returned":3`), 1), bytes.Replace(raw, []byte(`"omitted_retained":126`), []byte(`"omitted_retained":0`), 1), []byte(`{"available":true,"retained_total":3,"returned":2,"omitted_retained":1,"truncated":true,"records":[{},{},{}]}`)} {
		if json.Unmarshal(bad, &decoded) == nil {
			t.Fatal("ASSERT_CAPTURE88_SUMMARY_DECODE_GUARD")
		}
	}
	d.Truncated = true
	limited, e := PrivateDiscoverySummarize(d)
	if e != nil || !limited.RetentionTruncated || !limited.Truncated || !limited.Valid() {
		t.Fatal("ASSERT_CAPTURE88_RETAINED_TOTAL_NOT_FULL_ARCHIVE")
	}
	s.Truncated = false
	if s.Valid() {
		t.Fatal("ASSERT_CAPTURE88_SUMMARY_GUARD")
	}
}
func TestCapture88AggregateAndPrimary(t *testing.T) {
	object := `{"code":0,"message":"` + strings.Repeat("x", 8193-len(`{"code":0,"message":""}`)) + `"}`
	var wire []byte
	for i := 1; i <= 3; i++ {
		wire = append(wire, discoveryWire(`{"jsonrpc":"2.0","id":`+strconv.Itoa(i)+`,"error":`+object+`}`)...)
	}
	w, c, _ := discoveryWindow(wire)
	sink := new(captureTestSink)
	w.capture = captureTestState(sink)
	w.prepayResponseFacts()
	m := response76Runtime(w, c)
	for i := 0; i < 3; i++ {
		r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
		if r.Failure != "" || r.ServerError == nil || r.ServerError.Code != 0 {
			t.Fatal("ASSERT_CAPTURE88_PRIMARY")
		}
	}
	d := w.responseFactsSnapshot()
	if len(sink.b.Bytes()) != 16384 || sink.writes != 2 || d.Records[2].Capture.Status != CaptureUnavailable || !d.Valid() {
		t.Fatal("ASSERT_CAPTURE88_AGGREGATE")
	}
	summary, e := PrivateDiscoverySummarize(d)
	if e != nil || summary.Returned != 2 || summary.Omitted != 1 || summary.Records[0].Ordinal != 1 || summary.Records[1].Ordinal != 2 || summary.Records[0].Capture.Artifact != 1 || summary.Records[1].Capture.Artifact != 2 {
		t.Fatal("ASSERT_CAPTURE88_CAPTURE_REFERENCE_PRIORITY")
	}
	d.Records[1].Capture.Artifact = 1
	if d.Valid() {
		t.Fatal("ASSERT_CAPTURE88_UNIQUE_ARTIFACTS")
	}
	for _, bad := range []*captureTestSink{{short: true}, {writeFail: true}, {closeFail: true}} {
		w, c, _ = discoveryWindow(discoveryWire(`{"jsonrpc":"2.0","id":1,"error":{"code":0}}`))
		w.capture = captureTestState(bad)
		w.prepayResponseFacts()
		m = response76Runtime(w, c)
		before := w.refusal
		r := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: "inert", Generation: 1, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{}`), MaxMessages: 16, MaxBytes: 1 << 20})
		if r.Failure != "" || r.ServerError == nil || r.ServerError.Code != 0 || w.failed || w.refusal != before || w.responseFactsSnapshot().Records[0].Capture.Status != CaptureFailed {
			t.Fatal("ASSERT_CAPTURE88_SINK_PRIMARY_IMMUTABLE")
		}
	}
}

// capture88PrivateDirectory owns only its fresh namespace, never HOME or its
// ancestors. The real opener must admit the complete canonical path: a private
// leaf below a writable ancestor is not an acceptable positive fixture.
func capture88PrivateDirectory(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal("fixture HOME", err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil || !filepath.IsAbs(home) {
		t.Fatal("fixture canonical HOME", err)
	}
	root, err := os.MkdirTemp(home, ".capture88-owned-")
	if err != nil {
		t.Fatal("fixture private namespace", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error("fixture cleanup", err)
		}
	})
	f, err := openPrivateCaptureDirectory(context.Background(), root)
	if err != nil {
		t.Fatal("fixture ancestor admission", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal("fixture descriptor close", err)
	}
	return root
}

func TestCapture88PrivateFiles(t *testing.T) {
	path := capture88PrivateDirectory(t)
	w, _, _ := discoveryWindow(nil)
	o := &PrivateDiscoveryCaptureOptions{Permission: true, Destination: path}
	admissions := 0
	if !w.prepayCapture(o, func(q PrivateDiscoveryCaptureQuote) error { admissions++; return nil }) || admissions != 1 {
		t.Fatal("ASSERT_CAPTURE88_FILE_PREFLIGHT")
	}
	r := w.capture.capture(context.Background(), []byte("{}"), 0, 2)
	if r.Status != CaptureComplete {
		t.Fatal("ASSERT_CAPTURE88_FILE_WRITE")
	}
	name := filepath.Join(path, "error-01.private.json")
	st, e := os.Stat(name)
	if e != nil || st.Mode().Perm() != 0600 || st.Size() != 2 {
		t.Fatal("ASSERT_CAPTURE88_FILE_MODE")
	}
	target := filepath.Join(path, "target")
	if os.WriteFile(target, []byte("unchanged"), 0600) != nil || os.Symlink(target, filepath.Join(path, "error-02.private.json")) != nil {
		t.Fatal("fixture")
	}
	if w.capture.capture(context.Background(), []byte("{}"), 0, 2).Status != CaptureFailed {
		t.Fatal("ASSERT_CAPTURE88_FILE_NOFOLLOW")
	}
	b, e := os.ReadFile(target)
	if e != nil || string(b) != "unchanged" {
		t.Fatal("ASSERT_CAPTURE88_NO_OVERWRITE")
	}
	if !w.capture.close(context.Background()) {
		t.Fatal("ASSERT_CAPTURE88_FINALIZE")
	}
	root, e := openPrivateCaptureDirectory(context.Background(), path)
	if e != nil {
		t.Fatal("fixture")
	}
	other := &privateDiscoveryCapture{root: root}
	if other.capture(context.Background(), []byte("{}"), 0, 2).Status != CaptureFailed {
		t.Fatal("ASSERT_CAPTURE88_EXCLUSIVE")
	}
	other.close(context.Background())
}
func TestCapture88DirectoryAndSchema(t *testing.T) {
	root := capture88PrivateDirectory(t)
	f, e := openPrivateCaptureDirectory(context.Background(), root)
	if e != nil {
		t.Fatal("ASSERT_CAPTURE88_SAFE_DIRECTORY")
	}
	f.Close()
	link := filepath.Join(root, "link")
	if os.Symlink(root, link) != nil {
		t.Fatal("fixture setup")
	}
	if f, e := openPrivateCaptureDirectory(context.Background(), link); e == nil {
		f.Close()
		t.Fatal("ASSERT_CAPTURE88_NOFOLLOW")
	}
	if os.Mkdir(filepath.Join(root, "child"), 0700) != nil {
		t.Fatal("fixture setup")
	}
	if f, e := openPrivateCaptureDirectory(context.Background(), filepath.Join(link, "child")); e == nil {
		f.Close()
		t.Fatal("ASSERT_CAPTURE88_PARENT_NOFOLLOW")
	}
	unsafeParent := filepath.Join(root, "unsafe")
	if os.Mkdir(unsafeParent, 0700) != nil || os.Mkdir(filepath.Join(unsafeParent, "child"), 0700) != nil || os.Chmod(unsafeParent, 0777) != nil {
		t.Fatal("fixture setup")
	}
	if f, e := openPrivateCaptureDirectory(context.Background(), filepath.Join(unsafeParent, "child")); e == nil {
		f.Close()
		t.Fatal("ASSERT_CAPTURE88_WRITABLE_PARENT")
	}
	if os.Chmod(root, 0755) != nil {
		t.Fatal("fixture setup")
	}
	if f, e := openPrivateCaptureDirectory(context.Background(), root); e == nil {
		f.Close()
		t.Fatal("ASSERT_CAPTURE88_MODE")
	}
	for _, r := range []PrivateCaptureRecord{{Status: 99}, {Status: CaptureComplete}, {Status: CaptureDisabled, Artifact: 1}, {Status: CaptureUnavailable}, {Status: CaptureNotCaptured, Reason: CaptureSinkFailure}} {
		if r.Valid() {
			t.Fatal("ASSERT_CAPTURE88_SCHEMA")
		}
		if _, e := json.Marshal(r); e == nil {
			t.Fatal("ASSERT_CAPTURE88_JSON_GUARD")
		}
	}
}

// These guards require the retained external syscall overlay. Normal selections
// skip them explicitly; the repair runner arms and executes them separately.
func repair90FaultContext(t *testing.T, mode int) (context.Context, *[]int) {
	t.Helper()
	if os.Getenv("LSP_TRACE_REPAIR90_FAULT") != "1" {
		t.Skip("requires external real-close/EINTR overlay")
	}
	calls := new([]int)
	ctx := context.WithValue(context.Background(), "repair90.close.mode", mode)
	ctx = context.WithValue(ctx, "repair90.close.calls", calls)
	if mode >= 6 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		t.Cleanup(cancel)
		ctx = context.WithValue(ctx, "repair90.close.cancel", cancel)
	}
	return ctx, calls
}

func TestReview89DirectoryCloseUncertainty(t *testing.T) {
	ctx, calls := repair90FaultContext(t, 1)
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal("fixture")
	}
	f, e := openPrivateCaptureDirectory(ctx, dir)
	if f != nil {
		f.Close()
	}
	if !errors.Is(e, errPrivateCaptureCloseUncertain) {
		t.Fatal("REVIEW89_DIRECTORY_CLOSE_UNCERTAINTY_LOST")
	}
	if !errors.Is(e, ErrPrivateDiscovery) || len(*calls) != 2 || (*calls)[0] == (*calls)[1] {
		t.Fatal("REPAIR90_PARENT_CHILD_OWNERSHIP")
	}
}
func TestReview89DirectoryCloseCleanup(t *testing.T) {
	ctx, calls := repair90FaultContext(t, 1)
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal("fixture")
	}
	w, _, _ := discoveryWindow(nil)
	starts := 0
	c := PrivateDiscoveryConstruction{NewStarter: func() (Starter, error) { starts++; return nil, errors.New("inert") }, Limits: w.limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "inert"}}, FailureCapture: &PrivateDiscoveryCaptureOptions{Permission: true, Destination: dir}, CaptureAdmission: func(PrivateDiscoveryCaptureQuote) error { return nil }}
	observation, e := RunPrivateDiscovery(ctx, c, func(context.Context, *PrivateDiscoverySession) error { return nil })
	if e == nil || starts != 0 || !observation.Failed || observation.CleanupJoined {
		t.Fatal("REVIEW89_PRESTART_REFUSAL")
	}
	d, known := PrivateDiscoveryRefusalFromError(e)
	if !known || !d.CleanupUncertain {
		t.Fatal("REVIEW89_DIRECTORY_CLOSE_CLEANUP_FLAG_LOST")
	}
	if d.Stage != PrivateDiscoveryUnknownStage || d.AmountsAvailable || d.Currency != PrivateDiscoveryUnknownCurrency || len(*calls) != 2 {
		t.Fatal("REPAIR90_PREPARATION_UNKNOWN_NOT_BUDGET")
	}
	if !observation.ResponseFacts.Available || observation.ResponseFacts.Count != 0 || !observation.ResponseFacts.Valid() {
		t.Fatal("REPAIR90_PRESTART_ZERO_NOT_UNKNOWN")
	}
}

func TestRepair90DirectoryFaultBranches(t *testing.T) {
	for _, mode := range []int{2, 3, 4, 5, 6, 7, 8} {
		t.Run(strconv.Itoa(mode), func(t *testing.T) {
			ctx, calls := repair90FaultContext(t, mode)
			dir, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal("fixture")
			}
			w, _, _ := discoveryWindow(nil)
			starts := 0
			c := PrivateDiscoveryConstruction{NewStarter: func() (Starter, error) { starts++; return nil, errors.New("inert") }, Limits: w.limits, Deadline: time.Now().Add(time.Second), Members: []PrivateDiscoveryMember{{URI: "inert"}}, FailureCapture: &PrivateDiscoveryCaptureOptions{Permission: true, Destination: dir}, CaptureAdmission: func(PrivateDiscoveryCaptureQuote) error { return nil }}
			observation, e := RunPrivateDiscovery(ctx, c, func(context.Context, *PrivateDiscoverySession) error { return nil })
			d, known := PrivateDiscoveryRefusalFromError(e)
			if !known || !d.Valid() || !d.CleanupUncertain || d.Stage != PrivateDiscoveryUnknownStage || d.AmountsAvailable || starts != 0 || !observation.Failed || observation.CleanupJoined {
				t.Fatal("REPAIR90_ALL_SETUP_CLOSE_UNCERTAINTY_REACHES_RUNNER")
			}
			want := 2
			if mode == 4 || mode == 7 {
				want = 1
			}
			if mode == 5 || mode == 8 {
				want = strings.Count(dir, "/") + 1
			}
			if len(*calls) != want {
				t.Fatalf("REPAIR90_NO_RETRY_NO_DOUBLE_CLOSE got=%d want=%d", len(*calls), want)
			}
			for _, fd := range *calls {
				if fd < 0 {
					t.Fatal("REPAIR90_MUST_NOT_CLOSE_UNOWNED_CHILD")
				}
			}
		})
	}
}

func TestRepair90SetupDoesNotPoisonOrFabricateOwner(t *testing.T) {
	ctx, _ := repair90FaultContext(t, 1)
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal("fixture")
	}
	w, _, _ := discoveryWindow(nil)
	w.ctx = ctx
	ref := PrivateDiscoveryRefusal{Stage: PrivateDiscoveryRead, Currency: PrivateDiscoveryResponseBytes, Requested: 9, Remaining: 3, AmountsAvailable: true}
	w.mu.Lock()
	w.recordRefusalLocked(ref)
	w.mu.Unlock()
	if w.prepayCapture(&PrivateDiscoveryCaptureOptions{Permission: true, Destination: dir}, func(PrivateDiscoveryCaptureQuote) error { return nil }) || w.capture != nil || w.failed {
		t.Fatal("REPAIR90_PREPARATION_NOT_POISON_OR_FAKE_OWNER")
	}
	got, ok := PrivateDiscoveryRefusalFromError(w.refusalErrorFrom(nil, PrivateDiscoveryUnknownStage, true))
	ref.CleanupUncertain = true
	if !ok || got != ref {
		t.Fatal("REPAIR90_FIRST_NUMERIC_REFUSAL_MONOTONIC")
	}
}
