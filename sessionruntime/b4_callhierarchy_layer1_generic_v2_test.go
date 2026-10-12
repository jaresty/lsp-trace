package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

const (
	callHierarchyLayer1GenericV2Dir    = "testdata/callhierarchy-layer1-red-v2"
	callHierarchyLayer1GenericV2Schema = "adr0011.callhierarchy.layer1.red-oracle-corpus/v2"
)

type callHierarchyLayer1GenericV2Manifest struct {
	Aggregate  callHierarchyLayer1CorpusAggregate `json:"aggregate"`
	Schema     string                             `json:"schema"`
	SourceHead string                             `json:"source_head"`
	Worlds     []callHierarchyLayer1CorpusEntry   `json:"worlds"`
}

type callHierarchyLayer1GenericV2Envelope struct {
	Driver   callHierarchyLayer1GenericV2Driver   `json:"driver"`
	Expected callHierarchyLayer1GenericV2Expected `json:"expected"`
}

type callHierarchyLayer1GenericV2Driver struct {
	Derived   callHierarchyLayer1GenericV2DriverDerived   `json:"derived"`
	Request   callHierarchyLayer1GenericV2DriverRequest   `json:"request"`
	Transport callHierarchyLayer1GenericV2DriverTransport `json:"transport"`
}

type callHierarchyLayer1GenericV2DriverDerived struct {
	External callHierarchyLayer1GenericV2DriverExternal `json:"external"`
}

type callHierarchyLayer1GenericV2DriverExternal struct {
	NotificationsBytes *string `json:"notifications_bytes"`
	ResponsesBytes     *string `json:"responses_bytes"`
	ResultBytes        *string `json:"result_bytes"`
	ServerErrorBytes   *string `json:"server_error_bytes"`
}

type callHierarchyLayer1GenericV2DriverRequest struct {
	DeadlineAfterNS int64   `json:"deadline_after_ns"`
	Generation      uint64  `json:"generation"`
	MaxBytes        int64   `json:"max_bytes"`
	MaxMessages     int     `json:"max_messages"`
	Method          string  `json:"method"`
	ParamsHex       *string `json:"params_hex"`
	ParamsState     string  `json:"params_state"`
	RequestID       uint64  `json:"request_id"`
	Session         string  `json:"session"`
}

type callHierarchyLayer1GenericV2DriverTransport struct {
	Events []callHierarchyLayer1GenericV2DriverEvent `json:"events"`
}

type callHierarchyLayer1GenericV2DriverEvent struct {
	AtOffsetNS int64   `json:"at_offset_ns"`
	Class      *string `json:"class,omitempty"`
	FrameHex   *string `json:"frame_hex,omitempty"`
	Kind       string  `json:"kind"`
	Message    *string `json:"message,omitempty"`
}

type callHierarchyLayer1GenericV2Frame struct {
	Available   bool   `json:"available"`
	FrameHex    string `json:"frame_hex"`
	FrameLength uint64 `json:"frame_length"`
	FrameSHA256 string `json:"frame_sha256"`
}

type callHierarchyLayer1GenericV2Frames struct {
	Auxiliary []struct{}                        `json:"auxiliary"`
	Request   callHierarchyLayer1GenericV2Frame `json:"request"`
	Response  callHierarchyLayer1GenericV2Frame `json:"response"`
}

type callHierarchyLayer1GenericV2Result struct {
	Available bool    `json:"available"`
	BytesHex  *string `json:"bytes_hex"`
}

type callHierarchyLayer1GenericV2Derived struct {
	Control   callHierarchyLayer1GenericV2DerivedControl   `json:"control"`
	External  callHierarchyLayer1GenericV2DerivedExternal  `json:"external"`
	Identity  callHierarchyLayer1GenericV2DerivedIdentity  `json:"identity"`
	Lifecycle callHierarchyLayer1GenericV2DerivedLifecycle `json:"lifecycle"`
}

type callHierarchyLayer1GenericV2DerivedControl struct {
	ActualAttachmentAvailable bool   `json:"actual_attachment_available"`
	ActualReason              string `json:"actual_reason"`
}

type callHierarchyLayer1GenericV2DerivedExternal struct {
	Bytes              int64   `json:"bytes"`
	Failure            *string `json:"failure"`
	Messages           int     `json:"messages"`
	NotificationsBytes *string `json:"notifications_bytes"`
	RequestBytes       int64   `json:"request_bytes"`
	RequestMessages    int     `json:"request_messages"`
	ResponsesBytes     *string `json:"responses_bytes"`
	ResultBytes        *string `json:"result_bytes"`
	ServerErrorBytes   *string `json:"server_error_bytes"`
}

type callHierarchyLayer1GenericV2DerivedIdentity struct {
	TransactionRequestEqualsRequestIdentity bool `json:"transaction_request_equals_request_identity"`
}

type callHierarchyLayer1GenericV2DerivedLifecycle struct {
	CallbackCompleted    bool `json:"callback_completed"`
	ObservationAvailable bool `json:"observation_available"`
	ReleaseFirst         bool `json:"release_first"`
	ReleaseSecond        bool `json:"release_second"`
}

type callHierarchyLayer1GenericV2Comparison struct {
	Available           bool                                    `json:"available"`
	DistinctMethod      bool                                    `json:"distinct_method,omitempty"`
	EqualExternalResult bool                                    `json:"equal_external_result,omitempty"`
	Executions          []callHierarchyLayer1GenericV2Envelope  `json:"executions,omitempty"`
	ExternalEqual       bool                                    `json:"external_equal,omitempty"`
	LeftWorld           string                                  `json:"left_world,omitempty"`
	NoAlias             bool                                    `json:"no_alias,omitempty"`
	Pairs               []callHierarchyLayer1GenericV2Pair      `json:"pairs,omitempty"`
	RetainedPost        *callHierarchyLayer1CorpusMutationValue `json:"retained_post,omitempty"`
	RetainedPre         *callHierarchyLayer1CorpusMutationValue `json:"retained_pre,omitempty"`
	RightWorld          string                                  `json:"right_world,omitempty"`
	SourcePost          *callHierarchyLayer1CorpusMutationValue `json:"source_post,omitempty"`
	SourcePre           *callHierarchyLayer1CorpusMutationValue `json:"source_pre,omitempty"`
	TransportEqual      bool                                    `json:"transport_equal,omitempty"`
}

type callHierarchyLayer1GenericV2Pair struct {
	Disabled       callHierarchyLayer1GenericV2Envelope `json:"disabled"`
	Enabled        callHierarchyLayer1GenericV2Envelope `json:"enabled"`
	ExternalEqual  bool                                 `json:"external_equal"`
	TransportEqual bool                                 `json:"transport_equal"`
}

type callHierarchyLayer1GenericV2Concurrency struct {
	Available bool                                   `json:"available"`
	Workers   []callHierarchyLayer1GenericV2Envelope `json:"workers"`
}

type callHierarchyLayer1GenericV2Expected struct {
	Accounting      callHierarchyLayer1CorpusAccounting      `json:"accounting"`
	Commitments     callHierarchyLayer1CorpusCommitments     `json:"commitments"`
	Comparison      *callHierarchyLayer1GenericV2Comparison  `json:"comparison,omitempty"`
	Concurrency     *callHierarchyLayer1GenericV2Concurrency `json:"concurrency,omitempty"`
	Control         *callHierarchyLayer1CorpusControl        `json:"control,omitempty"`
	Derived         callHierarchyLayer1GenericV2Derived      `json:"derived"`
	Error           callHierarchyLayer1CorpusError           `json:"error"`
	External        callHierarchyLayer1CorpusExternal        `json:"external"`
	Frames          callHierarchyLayer1GenericV2Frames       `json:"frames"`
	Identity        callHierarchyLayer1CorpusIdentity        `json:"identity"`
	Lifecycle       callHierarchyLayer1CorpusLifecycle       `json:"lifecycle"`
	Name            string                                   `json:"name"`
	Refusal         callHierarchyLayer1CorpusRefusal         `json:"refusal"`
	Result          callHierarchyLayer1GenericV2Result       `json:"result"`
	RoundTripLedger callHierarchyLayer1CorpusRoundTripLedger `json:"round_trip_ledger"`
	Terminal        callHierarchyLayer1CorpusAvailableString `json:"terminal"`
	Transport       callHierarchyLayer1CorpusTransport       `json:"transport"`
}

type callHierarchyLayer1GenericV2Loaded struct {
	entry    callHierarchyLayer1CorpusEntry
	envelope callHierarchyLayer1GenericV2Envelope
	raw      []byte
}

func callHierarchyLayer1GenericV2DecodeStrict(t *testing.T, name string, raw []byte, dst any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V2_STRICT_%s: %v", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("ASSERT_CH_GENERIC_V2_SINGLE_VALUE_%s: %v", name, err)
	}
	canonical, err := json.Marshal(dst)
	if err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V2_CANONICAL_MARSHAL_%s: %v", name, err)
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		at := 0
		for at < len(raw) && at < len(canonical) && raw[at] == canonical[at] {
			at++
		}
		rawTail, canonicalTail := raw[at:], canonical[at:]
		if len(rawTail) > 80 {
			rawTail = rawTail[:80]
		}
		if len(canonicalTail) > 80 {
			canonicalTail = canonicalTail[:80]
		}
		t.Fatalf("ASSERT_CH_GENERIC_V2_CANONICAL_BYTES_%s offset=%d lengths=%d/%d raw=%q canonical=%q", name, at, len(raw), len(canonical), rawTail, canonicalTail)
	}
}

func callHierarchyLayer1GenericV2Hex(t *testing.T, path string, value *string) []byte {
	t.Helper()
	if value == nil {
		return nil
	}
	if *value != strings.ToLower(*value) || strings.HasPrefix(*value, "0x") || len(*value)%2 != 0 {
		t.Fatalf("ASSERT_CH_GENERIC_V2_DRIVER_HEX_%s: %q", path, *value)
	}
	decoded, err := hex.DecodeString(*value)
	if err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V2_DRIVER_HEX_%s: %v", path, err)
	}
	return decoded
}

func callHierarchyLayer1GenericV2ValidateDriver(t *testing.T, e callHierarchyLayer1GenericV2Envelope) {
	t.Helper()
	d := e.Driver
	params := callHierarchyLayer1GenericV2Hex(t, e.Expected.Name+".request.params_hex", d.Request.ParamsHex)
	switch d.Request.ParamsState {
	case "nil":
		if d.Request.ParamsHex != nil {
			t.Fatalf("ASSERT_CH_GENERIC_V2_PARAMS_NIL_%s", e.Expected.Name)
		}
	case "empty":
		if d.Request.ParamsHex == nil || len(params) != 0 {
			t.Fatalf("ASSERT_CH_GENERIC_V2_PARAMS_EMPTY_%s", e.Expected.Name)
		}
	case "present":
		if d.Request.ParamsHex == nil || len(params) == 0 {
			t.Fatalf("ASSERT_CH_GENERIC_V2_PARAMS_PRESENT_%s", e.Expected.Name)
		}
	default:
		t.Fatalf("ASSERT_CH_GENERIC_V2_PARAMS_STATE_%s: %q", e.Expected.Name, d.Request.ParamsState)
	}
	allowed := map[string]bool{"response-frame": true, "transport-error": true, "cancel": true, "deadline": true, "shutdown": true, "unavailable": true}
	for i, event := range d.Transport.Events {
		if !allowed[event.Kind] {
			t.Fatalf("ASSERT_CH_GENERIC_V2_EVENT_KIND_%s_%d: %q", e.Expected.Name, i, event.Kind)
		}
		switch event.Kind {
		case "response-frame":
			if event.FrameHex == nil || event.Class != nil || event.Message != nil {
				t.Fatalf("ASSERT_CH_GENERIC_V2_EVENT_SHAPE_%s_%d", e.Expected.Name, i)
			}
			callHierarchyLayer1GenericV2Hex(t, fmt.Sprintf("%s.event[%d].frame_hex", e.Expected.Name, i), event.FrameHex)
		case "transport-error":
			if event.FrameHex != nil || event.Class == nil || event.Message == nil {
				t.Fatalf("ASSERT_CH_GENERIC_V2_EVENT_SHAPE_%s_%d", e.Expected.Name, i)
			}
		default:
			if event.FrameHex != nil || event.Class != nil || event.Message != nil {
				t.Fatalf("ASSERT_CH_GENERIC_V2_EVENT_SHAPE_%s_%d", e.Expected.Name, i)
			}
		}
	}
}

func callHierarchyLayer1GenericV2SchemaPaths(typ reflect.Type, prefix string, out map[string]bool) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Slice {
		if typ.Elem().Kind() == reflect.Struct && typ.Elem().NumField() == 0 {
			out[prefix] = true
			return
		}
		callHierarchyLayer1GenericV2SchemaPaths(typ.Elem(), prefix+"[*]", out)
		return
	}
	if typ.Kind() != reflect.Struct {
		out[prefix] = true
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if (prefix == "comparison" && name != "available") || (prefix == "concurrency" && name == "workers") {
			continue
		}
		callHierarchyLayer1GenericV2SchemaPaths(field.Type, path, out)
	}
}

func callHierarchyLayer1GenericV2ValidateExpectedSchema(t *testing.T) []string {
	t.Helper()
	paths := map[string]bool{}
	callHierarchyLayer1GenericV2SchemaPaths(reflect.TypeOf(callHierarchyLayer1GenericV2Expected{}), "", paths)
	if len(paths) != 145 {
		t.Fatalf("ASSERT_CH_GENERIC_V2_EXPECTED_PATHS got=%d want=145", len(paths))
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	return ordered
}

func callHierarchyLayer1GenericV2Walk(e callHierarchyLayer1GenericV2Envelope, fn func(callHierarchyLayer1GenericV2Envelope)) {
	fn(e)
	if e.Expected.Concurrency != nil {
		for _, worker := range e.Expected.Concurrency.Workers {
			callHierarchyLayer1GenericV2Walk(worker, fn)
		}
	}
	if e.Expected.Comparison != nil {
		for _, execution := range e.Expected.Comparison.Executions {
			callHierarchyLayer1GenericV2Walk(execution, fn)
		}
		for _, pair := range e.Expected.Comparison.Pairs {
			callHierarchyLayer1GenericV2Walk(pair.Disabled, fn)
			callHierarchyLayer1GenericV2Walk(pair.Enabled, fn)
		}
	}
}

func callHierarchyLayer1GenericV2Load(t *testing.T) []callHierarchyLayer1GenericV2Loaded {
	t.Helper()
	manifestRaw, err := os.ReadFile(filepath.Join(callHierarchyLayer1GenericV2Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest callHierarchyLayer1GenericV2Manifest
	callHierarchyLayer1GenericV2DecodeStrict(t, "manifest", manifestRaw, &manifest)
	if manifest.Schema != callHierarchyLayer1GenericV2Schema || manifest.SourceHead == "" || manifest.Aggregate.ByteLength <= 0 || len(manifest.Aggregate.SHA256) != sha256.Size*2 || len(manifest.Worlds) != 25 || len(manifest.Aggregate.Order) != len(manifest.Worlds) {
		t.Fatalf("ASSERT_CH_GENERIC_V2_MANIFEST: %+v", manifest)
	}
	listed := map[string]bool{}
	aggregate := sha256.New()
	loaded := make([]callHierarchyLayer1GenericV2Loaded, 0, 25)
	for i, entry := range manifest.Worlds {
		path := fmt.Sprintf("worlds/world-%02d.json", i+1)
		name := fmt.Sprintf("world-%02d", i+1)
		if entry.Path != path || entry.Name != name || manifest.Aggregate.Order[i] != path || filepath.IsAbs(entry.Path) || filepath.ToSlash(filepath.Clean(entry.Path)) != entry.Path || strings.Contains(entry.Path, "..") || listed[entry.Path] {
			t.Fatalf("ASSERT_CH_GENERIC_V2_PATH_%02d: %+v", i+1, entry)
		}
		listed[entry.Path] = true
		raw, err := os.ReadFile(filepath.Join(callHierarchyLayer1GenericV2Dir, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(raw))
		if len(raw) != entry.ByteLength || hash != entry.SHA256 {
			t.Fatalf("ASSERT_CH_GENERIC_V2_WORLD_HASH_%02d", i+1)
		}
		aggregate.Write(raw)
		var envelope callHierarchyLayer1GenericV2Envelope
		callHierarchyLayer1GenericV2DecodeStrict(t, name, raw, &envelope)
		if envelope.Expected.Name != name {
			t.Fatalf("ASSERT_CH_GENERIC_V2_ROOT_NAME_%02d: %q", i+1, envelope.Expected.Name)
		}
		callHierarchyLayer1GenericV2Walk(envelope, func(record callHierarchyLayer1GenericV2Envelope) {
			callHierarchyLayer1GenericV2ValidateDriver(t, record)
		})
		loaded = append(loaded, callHierarchyLayer1GenericV2Loaded{entry: entry, envelope: envelope, raw: raw})
	}
	if got := fmt.Sprintf("%x", aggregate.Sum(nil)); got != manifest.Aggregate.SHA256 {
		t.Fatalf("ASSERT_CH_GENERIC_V2_AGGREGATE: %s", got)
	}
	entries, err := os.ReadDir(filepath.Join(callHierarchyLayer1GenericV2Dir, "worlds"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(listed) {
		t.Fatalf("ASSERT_CH_GENERIC_V2_EXTRA_WORLD_FILES got=%d want=%d", len(entries), len(listed))
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !listed["worlds/"+entry.Name()] {
			t.Fatalf("ASSERT_CH_GENERIC_V2_UNLISTED_WORLD_%s", entry.Name())
		}
	}
	return loaded
}

type callHierarchyLayer1GenericV2Actual struct {
	result        RoundTripResult
	request       RoundTripRequest
	requestFrame  []byte
	responseFrame []byte
	external      callHierarchyLayer1GenericV2ExternalActual
}

type callHierarchyLayer1GenericV2ExternalActual struct {
	requestFrame, responseFrame []byte
	result                      json.RawMessage
	serverError                 *lspwire.RPCError
	notifications               []lspwire.Message
	responses                   []lspwire.Message
	failure                     session.Failure
	messages                    int
	bytes                       int64
	requestMessages             int
	requestBytes                int64
}

type callHierarchyLayer1GenericV2Observation struct {
	attachment, owner, view, lease reflect.Value
	available                      bool
	reason                         string
}

const (
	callHierarchyLayer1GenericV2AttachmentField = "callHierarchyLayer1Generic"
	callHierarchyLayer1GenericV2AcquireMethod   = "privateCallHierarchyLayer1GenericLease"
	callHierarchyLayer1GenericV2ViewMethod      = "View"
	callHierarchyLayer1GenericV2CallbackMethod  = "Callback"
	callHierarchyLayer1GenericV2ReleaseMethod   = "Release"
)

type callHierarchyLayer1GenericV2Counters struct {
	roots, workers, executions, pairs atomic.Int64
}

type callHierarchyLayer1GenericV2WriteCapture struct {
	mu    sync.Mutex
	frame []byte
	wrote chan struct{}
	once  sync.Once
}

func (w *callHierarchyLayer1GenericV2WriteCapture) Write(p []byte) (int, error) {
	n := len(p)
	w.mu.Lock()
	w.frame = append(w.frame, p[:n]...)
	w.mu.Unlock()
	w.once.Do(func() { close(w.wrote) })
	return n, nil
}
func (w *callHierarchyLayer1GenericV2WriteCapture) Close() error {
	w.once.Do(func() { close(w.wrote) })
	return nil
}
func (w *callHierarchyLayer1GenericV2WriteCapture) snapshot() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.frame...)
}

type callHierarchyLayer1GenericV2Transport struct {
	stdin     *callHierarchyLayer1GenericV2WriteCapture
	output    *io.PipeWriter
	stdout    *io.PipeReader
	events    []callHierarchyLayer1GenericV2DriverEvent
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	response  []byte
	requestID uint64
	deadline  time.Duration
}

func callHierarchyLayer1GenericV2NewTransport(events []callHierarchyLayer1GenericV2DriverEvent, cancel context.CancelFunc, requestID uint64, deadline time.Duration) *callHierarchyLayer1GenericV2Transport {
	stdout, output := io.Pipe()
	c := &callHierarchyLayer1GenericV2Transport{stdin: &callHierarchyLayer1GenericV2WriteCapture{wrote: make(chan struct{})}, output: output, stdout: stdout, events: events, cancel: cancel, done: make(chan struct{}), requestID: requestID, deadline: deadline}
	go c.replay()
	return c
}

func (c *callHierarchyLayer1GenericV2Transport) replay() {
	defer close(c.done)
	<-c.stdin.wrote
	started := time.Now()
	for _, event := range c.events {
		if wait := time.Duration(event.AtOffsetNS) - time.Since(started); wait > 0 {
			time.Sleep(wait)
		}
		switch event.Kind {
		case "response-frame":
			frame := callHierarchyLayer1GenericV2HexNoTest(event.FrameHex)
			n, err := c.output.Write(frame)
			if n > 0 {
				c.mu.Lock()
				c.response = append(c.response, frame[:n]...)
				c.mu.Unlock()
			}
			if err != nil {
				return
			}
		case "transport-error":
			_ = c.output.CloseWithError(errors.New(stringValue(event.Message)))
			return
		case "cancel", "deadline":
			c.cancel()
			return
		case "shutdown", "unavailable":
			_ = c.output.Close()
			return
		}
	}
}

func (c *callHierarchyLayer1GenericV2Transport) Stdin() io.WriteCloser { return c.stdin }
func (c *callHierarchyLayer1GenericV2Transport) Stdout() io.ReadCloser { return c.stdout }
func (c *callHierarchyLayer1GenericV2Transport) Teardown(context.Context) managedprocess.TeardownObservation {
	c.closeOnce.Do(func() { _ = c.stdin.Close(); _ = c.output.Close() })
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *callHierarchyLayer1GenericV2Transport) Close() managedprocess.ResourceObservation {
	c.closeOnce.Do(func() { _ = c.stdin.Close(); _ = c.output.Close() })
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *callHierarchyLayer1GenericV2Transport) responseSnapshot() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.response...)
}

func callHierarchyLayer1GenericV2HexNoTest(value *string) []byte {
	if value == nil {
		return nil
	}
	decoded, _ := hex.DecodeString(*value)
	return decoded
}

func callHierarchyLayer1GenericV2Commitment(t *testing.T, path string, value callHierarchyLayer1CorpusCommitment) callHierarchyOpaqueCommitmentMetadata {
	t.Helper()
	var digest [32]byte
	copy(digest[:], callHierarchyLayer1GenericV2Hex(t, path+".sha256", value.SHA256))
	identity := ""
	if value.Identity != nil {
		identity = *value.Identity
	}
	length := uint64(0)
	if value.Length != nil {
		length = *value.Length
	}
	return callHierarchyOpaqueCommitmentMetadata{identity: identity, sha256: digest, length: length, available: value.Available}
}

func callHierarchyLayer1GenericV2Execute(t *testing.T, record callHierarchyLayer1GenericV2Envelope) callHierarchyLayer1GenericV2Actual {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	transport := callHierarchyLayer1GenericV2NewTransport(record.Driver.Transport.Events, cancel, record.Driver.Request.RequestID, time.Duration(record.Driver.Request.DeadlineAfterNS))
	manager, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 1, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{transport}})
	if err != nil {
		t.Fatal(err)
	}
	pending := lspwire.NewPending(0)
	for id := uint64(1); id < record.Driver.Request.RequestID; id++ {
		key := pending.Begin(record.Driver.Request.Generation)
		if got := pending.Accept(lspwire.ResponseKey(key)); got != lspwire.ResponseAccepted {
			t.Fatalf("ASSERT_CH_GENERIC_V2_PENDING_ADVANCE_%s_%d", record.Expected.Name, id)
		}
	}
	manager.mu.Lock()
	manager.sessions[record.Driver.Request.Session] = &runtimeSession{record: Record{SessionID: record.Driver.Request.Session, Generation: record.Driver.Request.Generation, State: session.Ready}, process: transport, pending: pending, requests: make(map[lspwire.RequestKey]*Request)}
	manager.mu.Unlock()
	t.Cleanup(func() { cancel(); _ = transport.Teardown(context.Background()); _ = transport.Close() })
	params := callHierarchyLayer1GenericV2Hex(t, record.Expected.Name+".driver.request.params_hex", record.Driver.Request.ParamsHex)
	var wireParams json.RawMessage
	if record.Driver.Request.ParamsHex != nil {
		wireParams = json.RawMessage(append([]byte(nil), params...))
	}
	paramsSource := params
	request := RoundTripRequest{
		SessionID: record.Driver.Request.Session, Generation: record.Driver.Request.Generation, Method: record.Driver.Request.Method,
		Params: wireParams, Deadline: time.Now().Add(time.Duration(record.Driver.Request.DeadlineAfterNS)),
		MaxMessages: record.Driver.Request.MaxMessages, MaxBytes: record.Driver.Request.MaxBytes,
		callHierarchyLayer1: callHierarchyLayer1RequestAttachment{
			selected: callHierarchyLayer1GenericV2Commitment(t, record.Expected.Name+".selected", record.Expected.Commitments.Selected),
			handoff:  callHierarchyLayer1GenericV2Commitment(t, record.Expected.Name+".handoff", record.Expected.Commitments.Handoff),
		},
	}
	result := manager.RoundTrip(ctx, request)
	if len(paramsSource) > 0 {
		paramsSource[0] ^= 0xff
	}
	select {
	case <-transport.done:
	case <-time.After(time.Second):
		cancel()
		_ = transport.output.Close()
		<-transport.done
	}
	cancel()
	_ = transport.stdin.Close()
	_ = transport.output.Close()
	if result.Key.ID != record.Driver.Request.RequestID {
		t.Errorf("ASSERT_CH_GENERIC_V2_REQUEST_ID_%s got=%d want=%d", record.Expected.Name, result.Key.ID, record.Driver.Request.RequestID)
	}
	requestFrame := transport.stdin.snapshot()
	responseFrame := transport.responseSnapshot()
	external := callHierarchyLayer1GenericV2ExternalFromResult(result, requestFrame, responseFrame)
	return callHierarchyLayer1GenericV2Actual{result: result, request: request, requestFrame: external.requestFrame, responseFrame: external.responseFrame, external: external}
}

func callHierarchyLayer1GenericV2ExternalFromResult(result RoundTripResult, requestFrame, responseFrame []byte) callHierarchyLayer1GenericV2ExternalActual {
	return callHierarchyLayer1GenericV2ExternalActual{
		requestFrame: append([]byte(nil), requestFrame...), responseFrame: append([]byte(nil), responseFrame...),
		result: append(json.RawMessage(nil), result.Result...), serverError: result.ServerError,
		notifications: append([]lspwire.Message(nil), result.Notifications...), responses: append([]lspwire.Message(nil), result.Responses...),
		failure: result.Failure, messages: result.Messages, bytes: result.Bytes,
		requestMessages: result.RequestMessages, requestBytes: result.RequestBytes,
	}
}

func callHierarchyLayer1GenericV2Equal(t *testing.T, world, path string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ASSERT_CH_GENERIC_V2_EXACT_%s path=%s got=%v expected=%v", world, path, callHierarchyLayer1GenericV2Scalar(reflect.ValueOf(got)), callHierarchyLayer1GenericV2Scalar(reflect.ValueOf(want)))
	}
}

func callHierarchyLayer1GenericV2Observe(result *RoundTripResult) callHierarchyLayer1GenericV2Observation {
	root := reflect.ValueOf(result)
	if root.Kind() != reflect.Pointer || root.IsNil() {
		return callHierarchyLayer1GenericV2Observation{reason: "round-trip-result-pointer"}
	}
	elem := root.Elem()
	field, ok := elem.Type().FieldByName(callHierarchyLayer1GenericV2AttachmentField)
	if !ok {
		return callHierarchyLayer1GenericV2Observation{reason: "field:" + callHierarchyLayer1GenericV2AttachmentField}
	}
	attachment := elem.FieldByIndex(field.Index)
	if (attachment.Kind() == reflect.Pointer || attachment.Kind() == reflect.Interface) && attachment.IsNil() {
		return callHierarchyLayer1GenericV2Observation{attachment: attachment, reason: "owner:nil"}
	}
	acquire := root.MethodByName(callHierarchyLayer1GenericV2AcquireMethod)
	if !acquire.IsValid() || acquire.Type().NumIn() != 0 || acquire.Type().NumOut() != 2 || acquire.Type().Out(1).Kind() != reflect.Bool {
		return callHierarchyLayer1GenericV2Observation{attachment: attachment, owner: attachment, reason: "method:" + callHierarchyLayer1GenericV2AcquireMethod + "() (lease,bool)"}
	}
	out := acquire.Call(nil)
	if !out[1].Bool() {
		return callHierarchyLayer1GenericV2Observation{attachment: attachment, owner: attachment, reason: "acquire:false"}
	}
	lease := out[0]
	viewMethod := lease.MethodByName(callHierarchyLayer1GenericV2ViewMethod)
	if !viewMethod.IsValid() || viewMethod.Type().NumIn() != 0 || viewMethod.Type().NumOut() != 2 || viewMethod.Type().Out(1).Kind() != reflect.Bool {
		return callHierarchyLayer1GenericV2Observation{attachment: attachment, owner: attachment, lease: lease, reason: "method:View() (view,bool)"}
	}
	viewOut := viewMethod.Call(nil)
	if !viewOut[1].Bool() {
		return callHierarchyLayer1GenericV2Observation{attachment: attachment, owner: attachment, lease: lease, reason: "view:false"}
	}
	return callHierarchyLayer1GenericV2Observation{attachment: attachment, owner: attachment, lease: lease, view: viewOut[0], available: true}
}

func callHierarchyLayer1GenericV2Field(value reflect.Value, path string) (any, bool) {
	return callHierarchyLayer1GenericV2Project(value, strings.Split(path, "."))
}

func callHierarchyLayer1GenericV2Project(value reflect.Value, parts []string) (any, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			if len(parts) == 0 {
				return nil, true
			}
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil, len(parts) == 0
	}
	if len(parts) == 0 {
		return callHierarchyLayer1GenericV2Scalar(value), true
	}

	part := parts[0]
	wildcard := strings.HasSuffix(part, "[*]")
	name := strings.TrimSuffix(part, "[*]")
	if value.Kind() != reflect.Struct {
		return nil, false
	}
	var fieldValue reflect.Value
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		fieldName := strings.Split(field.Tag.Get("json"), ",")[0]
		if fieldName == "" {
			fieldName = field.Name
		}
		if fieldName == name {
			fieldValue = value.Field(i)
			break
		}
	}
	if !fieldValue.IsValid() {
		return nil, false
	}
	if !wildcard {
		return callHierarchyLayer1GenericV2Project(fieldValue, parts[1:])
	}
	for fieldValue.IsValid() && (fieldValue.Kind() == reflect.Pointer || fieldValue.Kind() == reflect.Interface) {
		if fieldValue.IsNil() {
			return nil, false
		}
		fieldValue = fieldValue.Elem()
	}
	if !fieldValue.IsValid() || (fieldValue.Kind() != reflect.Slice && fieldValue.Kind() != reflect.Array) {
		return nil, false
	}
	if fieldValue.Kind() == reflect.Slice && fieldValue.Type().Elem().Kind() == reflect.Uint8 {
		if len(parts) != 1 {
			return nil, false
		}
		return callHierarchyLayer1GenericV2Scalar(fieldValue), true
	}
	projected := make([]any, fieldValue.Len())
	if fieldValue.Kind() == reflect.Slice && fieldValue.IsNil() {
		projected = nil
	}
	for i := 0; i < fieldValue.Len(); i++ {
		item, ok := callHierarchyLayer1GenericV2Project(fieldValue.Index(i), parts[1:])
		if !ok {
			return nil, false
		}
		projected[i] = item
	}
	return projected, true
}

func callHierarchyLayer1GenericV2Scalar(value reflect.Value) any {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Bool:
		return value.Bool()
	case reflect.String:
		return value.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint()
	case reflect.Float32, reflect.Float64:
		return value.Float()
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			if value.IsNil() {
				return []byte(nil)
			}
			copied := make([]byte, value.Len())
			copy(copied, value.Bytes())
			return copied
		}
		if value.IsNil() {
			return []any(nil)
		}
		values := make([]any, value.Len())
		for i := range values {
			values[i] = callHierarchyLayer1GenericV2Scalar(value.Index(i))
		}
		return values
	case reflect.Array:
		values := make([]any, value.Len())
		for i := range values {
			values[i] = callHierarchyLayer1GenericV2Scalar(value.Index(i))
		}
		return values
	case reflect.Struct:
		values := make(map[string]any, value.NumField())
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			values[name] = callHierarchyLayer1GenericV2Scalar(value.Field(i))
		}
		return values
	}
	if value.CanInterface() {
		return value.Interface()
	}
	return fmt.Sprint(value)
}

func callHierarchyLayer1GenericV2AssertExternal(t *testing.T, record callHierarchyLayer1GenericV2Envelope, actual callHierarchyLayer1GenericV2Actual) {
	t.Helper()
	d, request := record.Driver.Request, actual.request
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "request.session", request.SessionID, d.Session)
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "request.generation", request.Generation, d.Generation)
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "request.method", request.Method, d.Method)
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "request.params", []byte(request.Params), callHierarchyLayer1GenericV2Hex(t, record.Expected.Name+".request.params_hex", d.ParamsHex))
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "request.max_messages", request.MaxMessages, d.MaxMessages)
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "request.max_bytes", request.MaxBytes, d.MaxBytes)
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "result.key.generation", actual.result.Key.Generation, d.Generation)
	callHierarchyLayer1GenericV2Equal(t, record.Expected.Name, "result.key.id", actual.result.Key.ID, d.RequestID)
}

func callHierarchyLayer1GenericV2AssertActual(t *testing.T, record callHierarchyLayer1GenericV2Envelope, actual callHierarchyLayer1GenericV2Actual, expectedPaths []string) callHierarchyLayer1GenericV2Observation {
	t.Helper()
	e := record.Expected
	callHierarchyLayer1GenericV2AssertExternal(t, record, actual)
	observation := callHierarchyLayer1GenericV2Observe(&actual.result)
	if !observation.available {
		return observation
	}
	compared, skipped, fieldErrors := 0, 0, 0
	for _, path := range expectedPaths {
		want, wantOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(e), path)
		got, gotOK := callHierarchyLayer1GenericV2Field(observation.view, path)
		if !wantOK || !gotOK {
			fieldErrors++
			t.Errorf("ASSERT_CH_GENERIC_V2_FIELD_%s path=%s expected_path=%t actual_path=%t", e.Name, path, wantOK, gotOK)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			fieldErrors++
			t.Errorf("ASSERT_CH_GENERIC_V2_EXACT_%s path=%s got=%v expected=%v", e.Name, path, got, want)
			continue
		}
		compared++
	}
	if compared+skipped+fieldErrors != 145 || len(expectedPaths) != 145 {
		t.Errorf("ASSERT_CH_GENERIC_V2_PATH_ACCOUNTING_%s registry=%d compared=%d skipped=%d errors=%d want=145", e.Name, len(expectedPaths), compared, skipped, fieldErrors)
	}
	if compared != 145 || fieldErrors != 0 {
		t.Errorf("ASSERT_CH_GENERIC_V2_PATH_COMPLETENESS_%s compared=%d errors=%d want_compared=145 want_errors=0", e.Name, compared, fieldErrors)
	}
	t.Logf("ASSERT_CH_GENERIC_V2_PATH_COUNTS world=%s registry=%d compared=%d skipped=%d errors=%d", e.Name, len(expectedPaths), compared, skipped, fieldErrors)
	return observation
}

func TestCallHierarchyLayer1GenericV2ComparatorProjection(t *testing.T) {
	t.Run("predecessors-wildcard-values", func(t *testing.T) {
		type predecessor struct {
			Name string `json:"name"`
		}
		type fixture struct {
			Predecessors []predecessor `json:"predecessors"`
		}
		got, ok := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{Predecessors: []predecessor{{Name: "first"}, {Name: "second"}}}), "predecessors[*].name")
		if !ok || !reflect.DeepEqual(got, []any{"first", "second"}) {
			t.Fatalf("projection got=%#v ok=%t", got, ok)
		}
	})

	t.Run("lifecycle-sequence-element-inequality", func(t *testing.T) {
		type lifecycle struct {
			Sequence []string `json:"sequence"`
		}
		type fixture struct {
			Lifecycle lifecycle `json:"lifecycle"`
		}
		left, leftOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{Lifecycle: lifecycle{Sequence: []string{"acquire", "release"}}}), "lifecycle.sequence[*]")
		right, rightOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{Lifecycle: lifecycle{Sequence: []string{"acquire", "callback"}}}), "lifecycle.sequence[*]")
		if !leftOK || !rightOK || reflect.DeepEqual(left, right) {
			t.Fatalf("terminal sequence comparison left=%#v right=%#v ok=%t/%t", left, right, leftOK, rightOK)
		}
	})

	t.Run("nil-empty-distinction-and-order", func(t *testing.T) {
		type fixture struct {
			Values    []string `json:"values"`
			Pointer   *string  `json:"pointer"`
			Interface any      `json:"interface"`
		}
		nilValues, nilOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{}), "values[*]")
		emptyValues, emptyOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{Values: []string{}}), "values[*]")
		ordered, orderedOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{Values: []string{"a", "b"}}), "values[*]")
		reversed, reversedOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{Values: []string{"b", "a"}}), "values[*]")
		nilPointer, pointerOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{}), "pointer")
		nilInterface, interfaceOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{}), "interface")
		_, missingOK := callHierarchyLayer1GenericV2Field(reflect.ValueOf(fixture{}), "missing")
		if !nilOK || !emptyOK || !orderedOK || !reversedOK || !pointerOK || !interfaceOK || missingOK || nilPointer != nil || nilInterface != nil || reflect.DeepEqual(nilValues, emptyValues) || reflect.DeepEqual(ordered, reversed) {
			t.Fatalf("nil=%#v empty=%#v ordered=%#v reversed=%#v pointer=%#v interface=%#v missing_ok=%t", nilValues, emptyValues, ordered, reversed, nilPointer, nilInterface, missingOK)
		}
	})
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func callHierarchyLayer1GenericV2MutationChecks(t *testing.T, record callHierarchyLayer1GenericV2Envelope) {
	t.Helper()
	actual := callHierarchyLayer1GenericV2Execute(t, record)
	want := callHierarchyLayer1GenericV2Hex(t, record.Expected.Name+".mutation.params", record.Driver.Request.ParamsHex)
	if !bytes.Equal(actual.request.Params, want) {
		t.Errorf("ASSERT_CH_GENERIC_V2_PARAMS_OPAQUE_%s got=%x want=%x", record.Expected.Name, actual.request.Params, want)
	}
	observation := callHierarchyLayer1GenericV2Observe(&actual.result)
	if !observation.available {
		return
	}
	before := fmt.Sprint(callHierarchyLayer1GenericV2Scalar(observation.view))
	if len(actual.request.Params) > 0 {
		actual.request.Params[0] ^= 0xff
	}
	again := callHierarchyLayer1GenericV2Observe(&actual.result)
	if !again.available || fmt.Sprint(callHierarchyLayer1GenericV2Scalar(again.view)) != before {
		t.Errorf("ASSERT_CH_GENERIC_V2_RETAINED_INPUT_ALIAS_%s", record.Expected.Name)
	}
}

func TestCallHierarchyLayer1GenericV2CorpusCausalRED(t *testing.T) {
	expectedPaths := callHierarchyLayer1GenericV2ValidateExpectedSchema(t)
	worlds := callHierarchyLayer1GenericV2Load(t)
	if len(worlds) != 25 {
		t.Fatalf("ASSERT_CH_GENERIC_V2_ROOTS got=%d want=25", len(worlds))
	}
	if worlds[23].envelope.Expected.Concurrency == nil || len(worlds[23].envelope.Expected.Concurrency.Workers) != 16 {
		t.Fatal("ASSERT_CH_GENERIC_V2_W24_WORKERS")
	}
	if worlds[24].envelope.Expected.Comparison == nil || len(worlds[24].envelope.Expected.Comparison.Executions) != 22 || len(worlds[24].envelope.Expected.Comparison.Pairs) != 11 {
		t.Fatal("ASSERT_CH_GENERIC_V2_W25_EXECUTIONS_PAIRS")
	}
	var counters callHierarchyLayer1GenericV2Counters
	rootActuals := make([]callHierarchyLayer1GenericV2Actual, len(worlds))
	prerequisiteReason := ""
	recordPrerequisite := func(observation callHierarchyLayer1GenericV2Observation) {
		if !observation.available && prerequisiteReason == "" {
			prerequisiteReason = observation.reason
		}
	}
	for i, world := range worlds {
		i, world := i, world
		t.Run(world.entry.Name, func(t *testing.T) {
			counters.roots.Add(1)
			rootActuals[i] = callHierarchyLayer1GenericV2Execute(t, world.envelope)
			recordPrerequisite(callHierarchyLayer1GenericV2AssertActual(t, world.envelope, rootActuals[i], expectedPaths))
		})
	}

	w24 := worlds[23].envelope.Expected.Concurrency.Workers
	t.Run("world-24-actual-workers-and-aggregate", func(t *testing.T) {
		var wg sync.WaitGroup
		observed := make([]callHierarchyLayer1GenericV2Actual, len(w24))
		for i := range w24 {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				observed[i] = callHierarchyLayer1GenericV2Execute(t, w24[i])
				counters.workers.Add(1)
			}()
		}
		wg.Wait()
		for i := range w24 {
			recordPrerequisite(callHierarchyLayer1GenericV2AssertActual(t, w24[i], observed[i], expectedPaths))
		}
	})

	w25 := worlds[24].envelope.Expected.Comparison
	t.Run("world-25-actual-executions-and-pairs", func(t *testing.T) {
		for _, execution := range w25.Executions {
			actual := callHierarchyLayer1GenericV2Execute(t, execution)
			recordPrerequisite(callHierarchyLayer1GenericV2AssertActual(t, execution, actual, expectedPaths))
			counters.executions.Add(1)
		}
		for _, pair := range w25.Pairs {
			disabled := callHierarchyLayer1GenericV2Execute(t, pair.Disabled)
			enabled := callHierarchyLayer1GenericV2Execute(t, pair.Enabled)
			recordPrerequisite(callHierarchyLayer1GenericV2AssertActual(t, pair.Disabled, disabled, expectedPaths))
			recordPrerequisite(callHierarchyLayer1GenericV2AssertActual(t, pair.Enabled, enabled, expectedPaths))
			disabledTransport := struct {
				requestFrame, responseFrame []byte
				messages, requestMessages   int
				bytes, requestBytes         int64
			}{disabled.requestFrame, disabled.responseFrame, disabled.external.messages, disabled.external.requestMessages, disabled.external.bytes, disabled.external.requestBytes}
			enabledTransport := struct {
				requestFrame, responseFrame []byte
				messages, requestMessages   int
				bytes, requestBytes         int64
			}{enabled.requestFrame, enabled.responseFrame, enabled.external.messages, enabled.external.requestMessages, enabled.external.bytes, enabled.external.requestBytes}
			transportEqual := reflect.DeepEqual(disabledTransport, enabledTransport)
			callHierarchyLayer1GenericV2Equal(t, "world-25", "comparison.transport_equal", transportEqual, pair.TransportEqual)
			if pair.TransportEqual {
				callHierarchyLayer1GenericV2Equal(t, "world-25", "comparison.transport_equal.actual", disabledTransport, enabledTransport)
			}
			externalEqual := reflect.DeepEqual(disabled.external, enabled.external)
			callHierarchyLayer1GenericV2Equal(t, "world-25", "comparison.external_equal", externalEqual, pair.ExternalEqual)
			if pair.ExternalEqual {
				callHierarchyLayer1GenericV2Equal(t, "world-25", "comparison.external_equal.actual", disabled.external, enabled.external)
			}
			counters.pairs.Add(2)
		}
	})
	total := counters.roots.Load() + counters.workers.Load() + counters.executions.Load() + counters.pairs.Load()
	t.Logf("ASSERT_CH_GENERIC_V2_EXECUTION_COUNTERS roots=%d workers=%d executions=%d pair_members=%d total=%d", counters.roots.Load(), counters.workers.Load(), counters.executions.Load(), counters.pairs.Load(), total)
	if counters.roots.Load() != 25 || counters.workers.Load() != 16 || counters.executions.Load() != 22 || counters.pairs.Load() != 22 || total != 85 {
		t.Fatal("ASSERT_CH_GENERIC_V2_EXECUTION_COUNTER_MISMATCH")
	}
	if prerequisiteReason != "" {
		t.Errorf("ASSERT_CH_GENERIC_V2_ATTACHMENT_PREREQUISITE contract=RoundTripResult.%s/View reason=%s", callHierarchyLayer1GenericV2AcquireMethod, prerequisiteReason)
		return
	}
	callHierarchyLayer1GenericV2AssertLifecycle(t, worlds[0].envelope, &rootActuals[0].result)
}

var callHierarchyLayer1GenericV2ErrorType = reflect.TypeOf((*error)(nil)).Elem()

func callHierarchyLayer1GenericV2LifecycleMethods(observation callHierarchyLayer1GenericV2Observation) (reflect.Value, reflect.Value, bool) {
	callback := observation.lease.MethodByName(callHierarchyLayer1GenericV2CallbackMethod)
	release := observation.lease.MethodByName(callHierarchyLayer1GenericV2ReleaseMethod)
	validCallback := callback.IsValid() && callback.Type().NumIn() == 1 && callback.Type().In(0).Kind() == reflect.Func && callback.Type().NumOut() == 1 && callback.Type().Out(0).Implements(callHierarchyLayer1GenericV2ErrorType)
	validRelease := release.IsValid() && release.Type().NumIn() == 0 && release.Type().NumOut() == 1 && release.Type().Out(0).Kind() == reflect.Bool
	return callback, release, validCallback && validRelease
}

func callHierarchyLayer1GenericV2Callback(t *testing.T, callback reflect.Value, body func([]reflect.Value) error) (returned error, escaped any, calls int) {
	t.Helper()
	fnType := callback.Type().In(0)
	if fnType.NumOut() != 1 || !fnType.Out(0).Implements(callHierarchyLayer1GenericV2ErrorType) {
		t.Fatalf("ASSERT_CH_GENERIC_V2_CALLBACK_SIGNATURE callback=%s", fnType)
	}
	fn := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		calls++
		err := body(args)
		if err == nil {
			return []reflect.Value{reflect.Zero(fnType.Out(0))}
		}
		return []reflect.Value{reflect.ValueOf(err)}
	})
	func() {
		defer func() { escaped = recover() }()
		out := callback.Call([]reflect.Value{fn})
		if !out[0].IsNil() {
			returned = out[0].Interface().(error)
		}
	}()
	return returned, escaped, calls
}

func callHierarchyLayer1GenericV2FirstBytes(value reflect.Value) ([]byte, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil, false
	}
	if value.Kind() == reflect.Slice && value.Type().Elem().Kind() == reflect.Uint8 && value.Len() > 0 {
		return value.Bytes(), true
	}
	if value.Kind() == reflect.Struct {
		for i := 0; i < value.NumField(); i++ {
			if b, ok := callHierarchyLayer1GenericV2FirstBytes(value.Field(i)); ok {
				return b, true
			}
		}
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		for i := 0; i < value.Len(); i++ {
			if b, ok := callHierarchyLayer1GenericV2FirstBytes(value.Index(i)); ok {
				return b, true
			}
		}
	}
	return nil, false
}

func callHierarchyLayer1GenericV2FreshLifecycle(t *testing.T, record callHierarchyLayer1GenericV2Envelope) (callHierarchyLayer1GenericV2Actual, callHierarchyLayer1GenericV2Observation, reflect.Value, reflect.Value, bool) {
	t.Helper()
	actual := callHierarchyLayer1GenericV2Execute(t, record)
	observation := callHierarchyLayer1GenericV2Observe(&actual.result)
	if !observation.available {
		return actual, observation, reflect.Value{}, reflect.Value{}, false
	}
	callback, release, ok := callHierarchyLayer1GenericV2LifecycleMethods(observation)
	return actual, observation, callback, release, ok
}

func callHierarchyLayer1GenericV2AssertLifecycle(t *testing.T, record callHierarchyLayer1GenericV2Envelope, result *RoundTripResult) {
	t.Helper()
	observation := callHierarchyLayer1GenericV2Observe(result)
	if !observation.available {
		t.Errorf("ASSERT_CH_GENERIC_V2_LIFECYCLE_PREREQUISITE contract=RoundTripResult.%s/View/Callback/Release reason=%s", callHierarchyLayer1GenericV2AcquireMethod, observation.reason)
		return
	}
	callback, release, ok := callHierarchyLayer1GenericV2LifecycleMethods(observation)
	if !ok {
		t.Errorf("ASSERT_CH_GENERIC_V2_LIFECYCLE_PREREQUISITE contract=Callback(func(view)error)error+Release()bool")
		return
	}

	if source, ok := callHierarchyLayer1GenericV2FirstBytes(observation.view); ok {
		before := append([]byte(nil), source...)
		source[0] ^= 0xff
		again := callHierarchyLayer1GenericV2Observe(result)
		reacquired, reacquiredOK := callHierarchyLayer1GenericV2FirstBytes(again.view)
		if !again.available || !reacquiredOK || !bytes.Equal(reacquired, before) {
			t.Errorf("ASSERT_CH_GENERIC_V2_LIFECYCLE_OUTPUT_COPY_MUTATION_REACQUIRE")
		}
	}

	sentinel := errors.New("generic-v2-callback-error")
	returned, escaped, calls := callHierarchyLayer1GenericV2Callback(t, callback, func([]reflect.Value) error { return nil })
	if escaped != nil || returned != nil || calls != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V2_CALLBACK_SUCCESS returned=%v escaped=%v calls=%d", returned, escaped, calls)
	}
	returned, escaped, calls = callHierarchyLayer1GenericV2Callback(t, callback, func([]reflect.Value) error { return sentinel })
	if escaped != nil || !errors.Is(returned, sentinel) || calls != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V2_CALLBACK_ERROR returned=%v escaped=%v calls=%d", returned, escaped, calls)
	}
	returned, escaped, calls = callHierarchyLayer1GenericV2Callback(t, callback, func([]reflect.Value) error { panic("generic-v2-callback-panic") })
	if escaped != nil || returned == nil || calls != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V2_CALLBACK_PANIC_CONTAINED returned=%v escaped=%v calls=%d", returned, escaped, calls)
	}
	if !release.Call(nil)[0].Bool() || release.Call(nil)[0].Bool() {
		t.Errorf("ASSERT_CH_GENERIC_V2_LIFECYCLE_RELEASE_AFTER_CALLBACKS")
	}
	if callHierarchyLayer1GenericV2Observe(result).available {
		t.Errorf("ASSERT_CH_GENERIC_V2_LIFECYCLE_TERMINAL_AFTER_CALLBACKS")
	}

	sequentialActual, sequential, _, sequentialRelease, sequentialOK := callHierarchyLayer1GenericV2FreshLifecycle(t, record)
	if !sequential.available || !sequentialOK {
		t.Errorf("ASSERT_CH_GENERIC_V2_SEQUENTIAL_RELEASE_PREREQUISITE")
		return
	}
	first, second := sequentialRelease.Call(nil)[0].Bool(), sequentialRelease.Call(nil)[0].Bool()
	if !first || second {
		t.Errorf("ASSERT_CH_GENERIC_V2_SEQUENTIAL_RELEASE first=%t second=%t", first, second)
	}
	if callHierarchyLayer1GenericV2Observe(&sequentialActual.result).available {
		t.Errorf("ASSERT_CH_GENERIC_V2_SEQUENTIAL_RELEASE_TERMINAL")
	}

	concurrentActual, concurrent, concurrentCallback, concurrentRelease, concurrentOK := callHierarchyLayer1GenericV2FreshLifecycle(t, record)
	if !concurrent.available || !concurrentOK {
		t.Errorf("ASSERT_CH_GENERIC_V2_CONCURRENT_RELEASE_PREREQUISITE")
		return
	}
	_, escaped, calls = callHierarchyLayer1GenericV2Callback(t, concurrentCallback, func([]reflect.Value) error { return nil })
	const racers = 16
	start := make(chan struct{})
	results := make(chan bool, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- concurrentRelease.Call(nil)[0].Bool() }()
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for won := range results {
		if won {
			winners++
		}
	}
	if escaped != nil || calls != 1 || winners != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V2_CONCURRENT_RELEASE_EXACT winners=%d losers=%d callback_calls=%d escaped=%v", winners, racers-winners, calls, escaped)
	}
	if callHierarchyLayer1GenericV2Observe(&concurrentActual.result).available {
		t.Errorf("ASSERT_CH_GENERIC_V2_CONCURRENT_RELEASE_TERMINAL")
	}

	drainActual, drain, drainCallback, drainRelease, drainOK := callHierarchyLayer1GenericV2FreshLifecycle(t, record)
	if !drain.available || !drainOK {
		t.Errorf("ASSERT_CH_GENERIC_V2_DRAIN_PREREQUISITE")
		return
	}
	entered, unblock := make(chan struct{}), make(chan struct{})
	callbackDone, releaseDone := make(chan struct{}), make(chan bool, 1)
	go func() {
		defer close(callbackDone)
		returned, escaped, calls := callHierarchyLayer1GenericV2Callback(t, drainCallback, func([]reflect.Value) error { close(entered); <-unblock; return nil })
		if returned != nil || escaped != nil || calls != 1 {
			t.Errorf("ASSERT_CH_GENERIC_V2_DRAIN_CALLBACK returned=%v escaped=%v calls=%d", returned, escaped, calls)
		}
	}()
	<-entered
	go func() { releaseDone <- drainRelease.Call(nil)[0].Bool() }()
	select {
	case won := <-releaseDone:
		t.Errorf("ASSERT_CH_GENERIC_V2_DRAIN_RELEASE_EARLY result=%t", won)
	case <-time.After(20 * time.Millisecond):
	}
	close(unblock)
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("ASSERT_CH_GENERIC_V2_DRAIN_CALLBACK_DEADLOCK")
	}
	select {
	case won := <-releaseDone:
		if !won {
			t.Errorf("ASSERT_CH_GENERIC_V2_DRAIN_RELEASE_RESULT")
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_CH_GENERIC_V2_DRAIN_RELEASE_DEADLOCK")
	}
	if callHierarchyLayer1GenericV2Observe(&drainActual.result).available {
		t.Errorf("ASSERT_CH_GENERIC_V2_DRAIN_TERMINAL")
	}
}
