package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/lspwire"
)

func framed(t *testing.T, messages ...lspwire.Message) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	w := lspwire.NewWriter(&b, fixtureLimits)
	for _, m := range messages {
		if err := w.Write(m); err != nil {
			t.Fatal(err)
		}
	}
	return &b
}

func request(id, method, params string) lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: method, Params: json.RawMessage(params)}
}
func notification(method, params string) lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, Method: method, Params: json.RawMessage(params)}
}
func readAll(t *testing.T, b *bytes.Buffer) []lspwire.Message {
	t.Helper()
	r := lspwire.NewReader(b, fixtureLimits)
	var out []lspwire.Message
	for {
		m, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
}

func TestLifecycleRepliesAndBarriersUseRealFramingDeterministically(t *testing.T) {
	input := framed(t,
		request("1", "initialize", `{}`),
		notification("initialized", `{}`),
		request("2", "fixture/reply", `{"result":{"ok":true}}`),
		request("3", "fixture/barrier", `{"label":"ready"}`),
	)
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	got := readAll(t, &stdout)
	if len(got) != 3 {
		t.Fatalf("reply count = %d, want 3", len(got))
	}
	if string(got[0].ID) != "1" || !bytes.Contains(got[0].Result, []byte(`"name":"fake-lsp-fixture"`)) {
		t.Fatalf("initialize = %+v", got[0])
	}
	if string(got[1].Result) != `{"ok":true}` {
		t.Fatalf("reply result = %s", got[1].Result)
	}
	if string(got[2].Result) != `{"barrier":"ready"}` {
		t.Fatalf("barrier result = %s", got[2].Result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHangCancellationObservationAndLateReply(t *testing.T) {
	input := framed(t,
		request("7", "fixture/hang", `{}`),
		notification("$/cancelRequest", `{"id":7}`),
		notification("fixture/lateReply", `{"id":7,"result":{"late":true}}`),
	)
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	got := readAll(t, &stdout)
	if len(got) != 2 {
		t.Fatalf("message count = %d, want 2", len(got))
	}
	if got[0].Method != "fixture/cancelObserved" || string(got[0].Params) != `{"id":7}` {
		t.Fatalf("cancel observation = %+v", got[0])
	}
	if string(got[1].ID) != "7" || string(got[1].Result) != `{"late":true}` {
		t.Fatalf("late reply = %+v", got[1])
	}
}

func TestMalformedOutputCrashAndBoundedStderr(t *testing.T) {
	input := framed(t, notification("fixture/malformed", `{}`))
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("malformed exit code = %d", code)
	}
	if stdout.String() != "Content-Length: nope\r\n\r\n{}" {
		t.Fatalf("malformed output = %q", stdout.String())
	}

	input = framed(t, notification("fixture/stderr", `{"text":"`+strings.Repeat("x", 5000)+`"}`), notification("fixture/crash", `{}`))
	stdout.Reset()
	stderr.Reset()
	if code := run(input, &stdout, &stderr); code != fixtureCrashCode {
		t.Fatalf("crash exit code = %d", code)
	}
	if stderr.Len() != maxStderrBytes {
		t.Fatalf("stderr bytes = %d, want %d", stderr.Len(), maxStderrBytes)
	}
}

func TestMalformedInboundFrameIsRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(strings.NewReader("Content-Length: nope\r\n\r\n{}"), &stdout, &stderr); code != fixtureInputErrorCode {
		t.Fatalf("input error code = %d", code)
	}
	if stderr.Len() == 0 || stderr.Len() > maxStderrBytes {
		t.Fatalf("stderr bytes = %d", stderr.Len())
	}
}

const processTargetTimeout = 2 * time.Second

type processTargetSession struct {
	in       *io.PipeWriter
	out      *io.PipeReader
	wire     *lspwire.Writer
	reader   *lspwire.Reader
	done     chan struct{}
	exitCode int
	stderr   bytes.Buffer
}

func (s *processTargetSession) closeAndWait(t *testing.T) {
	t.Helper()
	_ = s.in.Close()
	_ = s.out.Close()
	select {
	case <-s.done:
		if s.exitCode != 0 {
			t.Errorf("run exit code = %d; stderr = %q", s.exitCode, s.stderr.String())
		}
	case <-time.After(processTargetTimeout):
		t.Fatalf("run did not exit after both pipe ends closed")
	}
}

func startProcessTargetSession(t *testing.T) *processTargetSession {
	t.Helper()
	t.Setenv("LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL", "process-target")
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	s := &processTargetSession{in: inWriter, out: outReader, wire: lspwire.NewWriter(inWriter, fixtureLimits), reader: lspwire.NewReader(outReader, fixtureLimits), done: make(chan struct{})}
	go func() {
		s.exitCode = run(inReader, outWriter, &s.stderr)
		close(s.done)
	}()
	t.Cleanup(func() {
		s.closeAndWait(t)
	})
	return s
}

func (s *processTargetSession) send(t *testing.T, message lspwire.Message) lspwire.Message {
	t.Helper()
	result := make(chan struct {
		message lspwire.Message
		err     error
	}, 1)
	go func() {
		if err := s.wire.Write(message); err != nil {
			result <- struct {
				message lspwire.Message
				err     error
			}{err: err}
			return
		}
		got, err := s.reader.Read()
		result <- struct {
			message lspwire.Message
			err     error
		}{message: got, err: err}
	}()
	select {
	case outcome := <-result:
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		return outcome.message
	case <-time.After(processTargetTimeout):
		_ = s.in.Close()
		_ = s.out.Close()
		select {
		case <-s.done:
			t.Fatalf("operation timed out; run exit code = %d; stderr = %q", s.exitCode, s.stderr.String())
		case <-time.After(processTargetTimeout):
			t.Fatalf("operation timed out and run did not terminate")
		}
		return lspwire.Message{}
	}
}

func marshalParams(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func didOpen(t *testing.T, uri, source string) lspwire.Message {
	t.Helper()
	return notification("textDocument/didOpen", string(marshalParams(t, map[string]any{"textDocument": map[string]any{"uri": uri, "text": source}})))
}

func processTargetReplies(t *testing.T, requests ...lspwire.Message) []lspwire.Message {
	t.Helper()
	s := startProcessTargetSession(t)
	var replies []lspwire.Message
	for _, request := range requests {
		if request.Method == "textDocument/didOpen" {
			if err := s.wire.Write(request); err != nil {
				t.Fatal(err)
			}
			continue
		}
		replies = append(replies, s.send(t, request))
	}
	_ = s.in.Close()
	return replies
}

type testPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type testRange struct {
	Start testPosition `json:"start"`
	End   testPosition `json:"end"`
}

type testDocumentSymbol struct {
	Name           string    `json:"name"`
	Kind           int       `json:"kind"`
	Range          testRange `json:"range"`
	SelectionRange testRange `json:"selectionRange"`
}

type testPrepareItem struct {
	Name           string            `json:"name"`
	Kind           int               `json:"kind"`
	URI            string            `json:"uri"`
	Range          testRange         `json:"range"`
	SelectionRange testRange         `json:"selectionRange"`
	Data           map[string]string `json:"data"`
}

type testOutgoingCall struct {
	To         testPrepareItem `json:"to"`
	FromRanges []testRange     `json:"fromRanges"`
}

func decodeTestResult[T any](t *testing.T, message lspwire.Message) []T {
	t.Helper()
	var result []T
	if err := json.Unmarshal(message.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return result
}

func assertPosition(t *testing.T, got, want testPosition, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %+v, want %+v", label, got, want)
	}
}

func assertRange(t *testing.T, got, want testRange, label string) {
	t.Helper()
	assertPosition(t, got.Start, want.Start, label+" start")
	assertPosition(t, got.End, want.End, label+" end")
}

func utf16ByteOffset(t *testing.T, source string, position testPosition) int {
	t.Helper()
	lines := strings.Split(source, "\n")
	if position.Line < 0 || position.Line >= len(lines) || position.Character < 0 {
		t.Fatalf("invalid source position %+v", position)
	}
	offset := 0
	for i := 0; i < position.Line; i++ {
		offset += len(lines[i]) + 1
	}
	line := lines[position.Line]
	units := 0
	for i := 0; i < len(line); {
		r, size := rune(line[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(line[i:])
		}
		next := units + 1
		if r > 0xffff {
			next = units + 2
		}
		if position.Character == units {
			return offset + i
		}
		if position.Character < next {
			t.Fatalf("position splits UTF-16 surrogate at %+v", position)
		}
		units = next
		i += size
	}
	if position.Character != units {
		t.Fatalf("character out of bounds at %+v", position)
	}
	return offset + len(line)
}

func extractSourceRange(t *testing.T, source string, r testRange) string {
	t.Helper()
	start := utf16ByteOffset(t, source, r.Start)
	end := utf16ByteOffset(t, source, r.End)
	if end < start {
		t.Fatalf("reversed range %+v", r)
	}
	return source[start:end]
}

func TestProcessTargetCleanupObservationIsReusable(t *testing.T) {
	s := startProcessTargetSession(t)
	_ = s.in.Close()
	s.closeAndWait(t)
	s.closeAndWait(t)
}

func TestProcessTarget(t *testing.T) {
	mainURI := "file:///workspace/target/main.go"
	peerURI := "file:///workspace/target/peer.go"
	targetSource := "package fixture\n\nfunc Target() {\n\tPeer()\n}\n"
	peerSource := "package fixture\n\nfunc Peer() {}\n"
	t.Run("documentSymbol Target decodes identity and full source extent", func(t *testing.T) {
		got := processTargetReplies(t,
			didOpen(t, peerURI, peerSource),
			didOpen(t, mainURI, targetSource),
			request("1", "textDocument/documentSymbol", `{"textDocument":{"uri":"`+mainURI+`"}}`),
		)
		if len(got) != 1 {
			t.Fatalf("Target documentSymbol reply count = %d", len(got))
		}
		symbols := decodeTestResult[testDocumentSymbol](t, got[0])
		if len(symbols) != 1 {
			t.Fatalf("Target documentSymbol count = %d", len(symbols))
		}
		if symbols[0].Name != "Target" || symbols[0].Kind != 12 {
			t.Fatalf("Target identity = %+v", symbols[0])
		}
		assertRange(t, symbols[0].Range, testRange{Start: testPosition{2, 0}, End: testPosition{4, 1}}, "Target range")
		assertRange(t, symbols[0].SelectionRange, testRange{Start: testPosition{2, 5}, End: testPosition{2, 11}}, "Target selection")
		if gotSource := extractSourceRange(t, targetSource, symbols[0].Range); gotSource != "func Target() {\n\tPeer()\n}" {
			t.Fatalf("Target extracted source = %q", gotSource)
		}
		if gotName := extractSourceRange(t, targetSource, symbols[0].SelectionRange); gotName != "Target" {
			t.Fatalf("Target extracted name = %q", gotName)
		}
	})
	t.Run("documentSymbol Peer decodes identity and exact extent", func(t *testing.T) {
		got := processTargetReplies(t,
			didOpen(t, mainURI, targetSource),
			didOpen(t, peerURI, peerSource),
			request("1", "textDocument/documentSymbol", `{"textDocument":{"uri":"`+peerURI+`"}}`),
		)
		if len(got) != 1 {
			t.Fatalf("Peer documentSymbol reply count = %d", len(got))
		}
		symbols := decodeTestResult[testDocumentSymbol](t, got[0])
		if len(symbols) != 1 || symbols[0].Name != "Peer" || symbols[0].Kind != 12 {
			t.Fatalf("Peer identity = %+v", symbols)
		}
		assertRange(t, symbols[0].Range, testRange{Start: testPosition{2, 0}, End: testPosition{2, 14}}, "Peer range")
		assertRange(t, symbols[0].SelectionRange, testRange{Start: testPosition{2, 5}, End: testPosition{2, 9}}, "Peer selection")
		if gotSource := extractSourceRange(t, peerSource, symbols[0].Range); gotSource != "func Peer() {}" {
			t.Fatalf("Peer extracted source = %q", gotSource)
		}
		if gotName := extractSourceRange(t, peerSource, symbols[0].SelectionRange); gotName != "Peer" {
			t.Fatalf("Peer extracted name = %q", gotName)
		}
	})
	t.Run("prepare Target decodes exact item after reverse didOpen order", func(t *testing.T) {
		got := processTargetReplies(t,
			didOpen(t, peerURI, peerSource),
			didOpen(t, mainURI, targetSource),
			request("1", "textDocument/prepareCallHierarchy", `{"textDocument":{"uri":"`+mainURI+`"},"position":{"line":2,"character":5}}`),
		)
		if len(got) != 1 {
			t.Fatalf("Target prepare reply count = %d", len(got))
		}
		items := decodeTestResult[testPrepareItem](t, got[0])
		if len(items) != 1 || items[0].Name != "Target" || items[0].Kind != 12 || items[0].URI != mainURI || items[0].Data["fixture"] != "Target" {
			t.Fatalf("Target prepare identity = %+v", items)
		}
		assertRange(t, items[0].Range, testRange{Start: testPosition{2, 0}, End: testPosition{4, 1}}, "Target prepare range")
		assertRange(t, items[0].SelectionRange, testRange{Start: testPosition{2, 5}, End: testPosition{2, 11}}, "Target prepare selection")
	})
	t.Run("prepare Peer decodes exact item independently", func(t *testing.T) {
		got := processTargetReplies(t,
			didOpen(t, mainURI, targetSource),
			didOpen(t, peerURI, peerSource),
			request("1", "textDocument/prepareCallHierarchy", `{"textDocument":{"uri":"`+peerURI+`"},"position":{"line":2,"character":5}}`),
		)
		if len(got) != 1 {
			t.Fatalf("Peer prepare reply count = %d", len(got))
		}
		items := decodeTestResult[testPrepareItem](t, got[0])
		if len(items) != 1 || items[0].Name != "Peer" || items[0].Kind != 12 || items[0].URI != peerURI || items[0].Data["fixture"] != "Peer" {
			t.Fatalf("Peer prepare identity = %+v", items)
		}
		assertRange(t, items[0].Range, testRange{Start: testPosition{2, 0}, End: testPosition{2, 14}}, "Peer prepare range")
		assertRange(t, items[0].SelectionRange, testRange{Start: testPosition{2, 5}, End: testPosition{2, 9}}, "Peer prepare selection")
	})
	t.Run("outgoing Target uses returned prepare item and Peer is empty", func(t *testing.T) {
		s := startProcessTargetSession(t)
		for _, open := range []lspwire.Message{didOpen(t, peerURI, peerSource), didOpen(t, mainURI, targetSource)} {
			if err := s.wire.Write(open); err != nil {
				t.Fatal(err)
			}
		}
		preparedReply := s.send(t, request("1", "textDocument/prepareCallHierarchy", `{"textDocument":{"uri":"`+mainURI+`"},"position":{"line":2,"character":5}}`))
		items := decodeTestResult[testPrepareItem](t, preparedReply)
		if len(items) != 1 {
			t.Fatalf("prepared item count = %d", len(items))
		}
		if err := s.wire.Write(didOpen(t, "file:///workspace/unrelated/last.go", "package fixture\n\nfunc Last() {}\n")); err != nil {
			t.Fatal(err)
		}
		itemJSON, err := json.Marshal(items[0])
		if err != nil {
			t.Fatal(err)
		}
		got := []lspwire.Message{s.send(t, request("1", "callHierarchy/outgoingCalls", `{"item":`+string(itemJSON)+`}`))}
		if len(got) != 1 {
			t.Fatalf("Target outgoing reply count = %d", len(got))
		}
		calls := decodeTestResult[testOutgoingCall](t, got[0])
		if len(calls) != 1 || calls[0].To.Name != "Peer" || calls[0].To.Kind != 12 || calls[0].To.URI != peerURI {
			t.Fatalf("Target outgoing identity = %+v", calls)
		}
		if gotSource := extractSourceRange(t, peerSource, calls[0].To.Range); gotSource != "func Peer() {}" {
			t.Fatalf("Target outgoing Peer extracted source = %q", gotSource)
		}
		if gotName := extractSourceRange(t, peerSource, calls[0].To.SelectionRange); gotName != "Peer" {
			t.Fatalf("Target outgoing Peer extracted name = %q", gotName)
		}
		if len(calls[0].FromRanges) != 1 {
			t.Fatalf("Target outgoing fromRanges = %+v", calls[0].FromRanges)
		}
		if gotToken := extractSourceRange(t, targetSource, calls[0].FromRanges[0]); gotToken != "Peer" {
			t.Fatalf("Target outgoing fromRange extracted token = %q", gotToken)
		}

		peerReply := s.send(t, request("2", "textDocument/prepareCallHierarchy", `{"textDocument":{"uri":"`+peerURI+`"},"position":{"line":2,"character":5}}`))
		peerItems := decodeTestResult[testPrepareItem](t, peerReply)
		if len(peerItems) != 1 {
			t.Fatalf("Peer prepared item count = %d", len(peerItems))
		}
		peer := peerItems[0]
		to := calls[0].To
		toData, toHasData := to.Data["fixture"]
		peerData, peerHasData := peer.Data["fixture"]
		if !toHasData || !peerHasData || toData != peerData {
			t.Fatalf("ASSERT_OUTGOING_PEER_DATA_EQUALS_PREPARED outgoing=%+v prepared=%+v", to.Data, peer.Data)
		}
		if to.Name != peer.Name || to.Kind != peer.Kind || to.URI != peer.URI || to.Range != peer.Range || to.SelectionRange != peer.SelectionRange {
			t.Fatalf("Target outgoing Peer differs from returned Peer prepare item: to=%+v peer=%+v", to, peer)
		}
		peerJSON, err := json.Marshal(peer)
		if err != nil {
			t.Fatal(err)
		}
		peerOutgoing := s.send(t, request("2", "callHierarchy/outgoingCalls", `{"item":`+string(peerJSON)+`}`))
		peerCalls := decodeTestResult[testOutgoingCall](t, peerOutgoing)
		if len(peerCalls) != 0 {
			t.Fatalf("Peer outgoing count = %d, want 0", len(peerCalls))
		}
	})
}

func TestFixtureHasNoDescendantControl(t *testing.T) {
	input := framed(t, request("9", "fixture/descendant", `{}`))
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	got := readAll(t, &stdout)
	if len(got) != 1 || got[0].Error == nil || got[0].Error.Code != methodNotFoundCode {
		t.Fatalf("descendant response = %+v", got)
	}
}
