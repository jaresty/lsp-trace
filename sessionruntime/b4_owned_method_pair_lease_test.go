package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func runOwnedPairDeadline(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("ASSERT_C15_PAIR_DEADLINE\n%s", buf[:n])
	}
}

const callHierarchyLayer1CorpusDir = "testdata/callhierarchy-layer1-red-v1"

// sourceHead is the accepted fixture provenance.  It intentionally does not
// track the checkout HEAD: later test evolution consumes, but does not rewrite,
// the immutable corpus authored at this revision.
const (
	callHierarchyLayer1CorpusSchema     = "adr0011.callhierarchy.layer1.red-oracle-corpus/v1"
	callHierarchyLayer1CorpusSourceHead = "4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9"
	callHierarchyLayer1CorpusBytes      = 431974
	callHierarchyLayer1CorpusSHA256     = "7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84"
)

type callHierarchyLayer1CorpusManifest struct {
	Schema     string                             `json:"schema"`
	SourceHead string                             `json:"source_head"`
	Aggregate  callHierarchyLayer1CorpusAggregate `json:"aggregate"`
	Worlds     []callHierarchyLayer1CorpusEntry   `json:"worlds"`
}

type callHierarchyLayer1CorpusAggregate struct {
	ByteLength int      `json:"byte_length"`
	Order      []string `json:"order"`
	SHA256     string   `json:"sha256"`
}

type callHierarchyLayer1CorpusEntry struct {
	ByteLength int    `json:"byte_length"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
}

type callHierarchyLayer1CorpusWorld struct {
	Provenance   callHierarchyLayer1CorpusProvenance `json:"provenance"`
	Record       callHierarchyLayer1CorpusRecord     `json:"record"`
	RootIdentity string                              `json:"root_identity"`
}

type callHierarchyLayer1CorpusProvenance struct {
	Schema     string `json:"schema"`
	SourceHead string `json:"source_head"`
}

type callHierarchyLayer1CorpusRecord struct {
	Accounting      callHierarchyLayer1CorpusAccounting      `json:"accounting"`
	Commitments     callHierarchyLayer1CorpusCommitments     `json:"commitments"`
	Comparison      callHierarchyLayer1CorpusComparison      `json:"comparison"`
	Concurrency     callHierarchyLayer1CorpusConcurrency     `json:"concurrency"`
	Control         callHierarchyLayer1CorpusControl         `json:"control"`
	Error           callHierarchyLayer1CorpusError           `json:"error"`
	External        callHierarchyLayer1CorpusExternal        `json:"external"`
	Frames          callHierarchyLayer1CorpusFrames          `json:"frames"`
	Identity        callHierarchyLayer1CorpusIdentity        `json:"identity"`
	Lifecycle       callHierarchyLayer1CorpusLifecycle       `json:"lifecycle"`
	Name            string                                   `json:"name"`
	Refusal         callHierarchyLayer1CorpusRefusal         `json:"refusal"`
	Result          callHierarchyLayer1CorpusResult          `json:"result"`
	RoundTripLedger callHierarchyLayer1CorpusRoundTripLedger `json:"round_trip_ledger"`
	Terminal        callHierarchyLayer1CorpusAvailableString `json:"terminal"`
	Transport       callHierarchyLayer1CorpusTransport       `json:"transport"`
}

type callHierarchyLayer1CorpusAvailableString struct {
	Available bool    `json:"available"`
	Value     *string `json:"value"`
}

type callHierarchyLayer1CorpusAvailableUint64 struct {
	Available bool    `json:"available"`
	KnownZero bool    `json:"known_zero"`
	Value     *uint64 `json:"value"`
}

type callHierarchyLayer1CorpusGeneration struct {
	Available bool   `json:"available"`
	Value     uint64 `json:"value"`
}

type callHierarchyLayer1CorpusRequestIdentity struct {
	Generation        uint64 `json:"generation"`
	RequestKey        uint64 `json:"request_key"`
	Session           string `json:"session"`
	WorkspaceNonceHex string `json:"workspace_nonce_hex"`
}

type callHierarchyLayer1CorpusTransaction struct {
	Available bool   `json:"available"`
	LocalHex  string `json:"local_hex"`
}

type callHierarchyLayer1CorpusIdentity struct {
	Binding         callHierarchyLayer1CorpusAvailableString `json:"binding"`
	Generation      callHierarchyLayer1CorpusGeneration      `json:"generation"`
	Method          callHierarchyLayer1CorpusAvailableString `json:"method"`
	Profile         callHierarchyLayer1CorpusAvailableString `json:"profile"`
	RequestIdentity callHierarchyLayer1CorpusRequestIdentity `json:"request_identity"`
	Transaction     callHierarchyLayer1CorpusTransaction     `json:"transaction"`
}

type callHierarchyLayer1CorpusCommitment struct {
	Available bool    `json:"available"`
	Identity  *string `json:"identity"`
	Length    *uint64 `json:"length"`
	SHA256    *string `json:"sha256"`
}

type callHierarchyLayer1CorpusCommitments struct {
	Handoff      callHierarchyLayer1CorpusCommitment   `json:"handoff"`
	Predecessors []callHierarchyLayer1CorpusCommitment `json:"predecessors"`
	Selected     callHierarchyLayer1CorpusCommitment   `json:"selected"`
}

type callHierarchyLayer1CorpusControl struct {
	AttachmentAvailable  bool    `json:"attachment_available"`
	Enabled              bool    `json:"enabled"`
	ObservationAvailable bool    `json:"observation_available"`
	PairedWorld          *string `json:"paired_world"`
	Reason               *string `json:"reason"`
}

type callHierarchyLayer1CorpusError struct {
	Available     bool    `json:"available"`
	Class         string  `json:"class"`
	Code          *int64  `json:"code"`
	DataBase64    *string `json:"data_base64"`
	DataForm      string  `json:"data_form"`
	Message       *string `json:"message"`
	Present       bool    `json:"present"`
	SerializedHex string  `json:"serialized_hex"`
}

type callHierarchyLayer1CorpusExternal struct {
	Available         bool    `json:"available"`
	ExecutionExpected bool    `json:"execution_expected"`
	Failure           *string `json:"failure"`
	NotificationsHex  string  `json:"notifications_hex"`
	ResponsesHex      string  `json:"responses_hex"`
}

type callHierarchyLayer1CorpusSpan struct {
	Available bool    `json:"available"`
	BytesHex  *string `json:"bytes_hex"`
	Length    *uint64 `json:"length"`
	SHA256    *string `json:"sha256"`
}

type callHierarchyLayer1CorpusFrame struct {
	Available   bool                          `json:"available"`
	Body        callHierarchyLayer1CorpusSpan `json:"body"`
	FrameHex    string                        `json:"frame_hex"`
	FrameLength uint64                        `json:"frame_length"`
	FrameSHA256 string                        `json:"frame_sha256"`
	Payload     callHierarchyLayer1CorpusSpan `json:"payload"`
}

type callHierarchyLayer1CorpusFrames struct {
	Auxiliary []struct{}                     `json:"auxiliary"`
	Request   callHierarchyLayer1CorpusFrame `json:"request"`
	Response  callHierarchyLayer1CorpusFrame `json:"response"`
}

type callHierarchyLayer1CorpusResult struct {
	Available bool    `json:"available"`
	BytesHex  *string `json:"bytes_hex"`
	Form      string  `json:"form"`
}

type callHierarchyLayer1CorpusRefusal struct {
	Reason  *string `json:"reason"`
	Refused bool    `json:"refused"`
}

type callHierarchyLayer1CorpusConcurrency struct {
	Available bool                              `json:"available"`
	Workers   []callHierarchyLayer1CorpusRecord `json:"workers"`
}

type callHierarchyLayer1CorpusPair struct {
	Disabled       callHierarchyLayer1CorpusRecord `json:"disabled"`
	Enabled        callHierarchyLayer1CorpusRecord `json:"enabled"`
	ExternalEqual  bool                            `json:"external_equal"`
	TransportEqual bool                            `json:"transport_equal"`
}

type callHierarchyLayer1CorpusMutationValue struct {
	BytesHex string `json:"bytes_hex"`
	SHA256   string `json:"sha256"`
	Value    string `json:"value"`
}

type callHierarchyLayer1CorpusComparison struct {
	Available           bool                                    `json:"available"`
	Executions          []callHierarchyLayer1CorpusRecord       `json:"executions"`
	Pairs               []callHierarchyLayer1CorpusPair         `json:"pairs"`
	NoAlias             bool                                    `json:"no_alias,omitempty"`
	DistinctMethod      bool                                    `json:"distinct_method,omitempty"`
	EqualExternalResult bool                                    `json:"equal_external_result,omitempty"`
	LeftWorld           string                                  `json:"left_world,omitempty"`
	RightWorld          string                                  `json:"right_world,omitempty"`
	RetainedPost        *callHierarchyLayer1CorpusMutationValue `json:"retained_post,omitempty"`
	RetainedPre         *callHierarchyLayer1CorpusMutationValue `json:"retained_pre,omitempty"`
	SourcePost          *callHierarchyLayer1CorpusMutationValue `json:"source_post,omitempty"`
	SourcePre           *callHierarchyLayer1CorpusMutationValue `json:"source_pre,omitempty"`
}

type callHierarchyLayer1CorpusAccounting struct {
	Bytes       callHierarchyLayer1CorpusAvailableUint64 `json:"bytes"`
	Diagnostics callHierarchyLayer1CorpusAvailableUint64 `json:"diagnostics"`
	Objects     callHierarchyLayer1CorpusAvailableUint64 `json:"objects"`
	Work        callHierarchyLayer1CorpusAvailableUint64 `json:"work"`
}

type callHierarchyLayer1CorpusLifecycle struct {
	Acquired  bool     `json:"acquired"`
	Completed bool     `json:"completed"`
	Released  bool     `json:"released"`
	Sequence  []string `json:"sequence"`
}

type callHierarchyLayer1CorpusLedgerDimension struct {
	After     *uint64 `json:"after"`
	Available bool    `json:"available"`
	Before    *uint64 `json:"before"`
	Delta     *uint64 `json:"delta"`
	KnownZero bool    `json:"known_zero"`
}

type callHierarchyLayer1CorpusLedgerDimensions struct {
	Callbacks         callHierarchyLayer1CorpusLedgerDimension `json:"callbacks"`
	Diagnostics       callHierarchyLayer1CorpusLedgerDimension `json:"diagnostics"`
	ExternalBytes     callHierarchyLayer1CorpusLedgerDimension `json:"external_bytes"`
	Leases            callHierarchyLayer1CorpusLedgerDimension `json:"leases"`
	Reservations      callHierarchyLayer1CorpusLedgerDimension `json:"reservations"`
	TransportBytes    callHierarchyLayer1CorpusLedgerDimension `json:"transport_bytes"`
	TransportMessages callHierarchyLayer1CorpusLedgerDimension `json:"transport_messages"`
}

type callHierarchyLayer1CorpusLedgerTerminal struct {
	After  callHierarchyLayer1CorpusAvailableString `json:"after"`
	Before callHierarchyLayer1CorpusAvailableString `json:"before"`
}

type callHierarchyLayer1CorpusRoundTripLedger struct {
	Dimensions callHierarchyLayer1CorpusLedgerDimensions `json:"dimensions"`
	Terminal   callHierarchyLayer1CorpusLedgerTerminal   `json:"terminal"`
}

type callHierarchyLayer1CorpusTransport struct {
	Bytes           callHierarchyLayer1CorpusAvailableUint64 `json:"bytes"`
	Messages        callHierarchyLayer1CorpusAvailableUint64 `json:"messages"`
	RequestBytes    callHierarchyLayer1CorpusAvailableUint64 `json:"request_bytes"`
	RequestMessages callHierarchyLayer1CorpusAvailableUint64 `json:"request_messages"`
}

type callHierarchyLayer1LoadedWorld struct {
	entry callHierarchyLayer1CorpusEntry
	raw   []byte
	world callHierarchyLayer1CorpusWorld
}

func callHierarchyLayer1DecodeStrictForTest(t *testing.T, name string, data []byte, dst any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		t.Fatalf("ASSERT_CH_CORPUS_STRICT_DECODE_%s: %v", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("ASSERT_CH_CORPUS_SINGLE_JSON_VALUE_%s: %v", name, err)
	}
}

func callHierarchyLayer1LoadCorpusForTest(t *testing.T) []callHierarchyLayer1LoadedWorld {
	t.Helper()
	manifestBytes, err := os.ReadFile(filepath.Join(callHierarchyLayer1CorpusDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest callHierarchyLayer1CorpusManifest
	callHierarchyLayer1DecodeStrictForTest(t, "manifest", manifestBytes, &manifest)
	if manifest.Schema != callHierarchyLayer1CorpusSchema || manifest.SourceHead != callHierarchyLayer1CorpusSourceHead || manifest.Aggregate.ByteLength != callHierarchyLayer1CorpusBytes || manifest.Aggregate.SHA256 != callHierarchyLayer1CorpusSHA256 || len(manifest.Worlds) != 25 || len(manifest.Aggregate.Order) != 25 {
		t.Fatalf("ASSERT_CH_CORPUS_CANONICAL_MANIFEST_SOURCE_HEAD_4F2B: %+v", manifest)
	}
	worlds := make([]callHierarchyLayer1LoadedWorld, 0, 25)
	aggregate := sha256.New()
	for i, entry := range manifest.Worlds {
		wantPath := fmt.Sprintf("worlds/world-%02d.json", i+1)
		wantName := fmt.Sprintf("world-%02d", i+1)
		if manifest.Aggregate.Order[i] != wantPath || entry.Path != wantPath || entry.Name != wantName {
			t.Fatalf("ASSERT_CH_CORPUS_ORDER_%02d: order=%q entry=%+v", i+1, manifest.Aggregate.Order[i], entry)
		}
		raw, err := os.ReadFile(filepath.Join(callHierarchyLayer1CorpusDir, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatal(err)
		}
		gotHash := fmt.Sprintf("%x", sha256.Sum256(raw))
		if len(raw) != entry.ByteLength || gotHash != entry.SHA256 {
			t.Fatalf("ASSERT_CH_CORPUS_CANONICAL_WORLD_%02d: bytes=%d/%d sha256=%s/%s", i+1, len(raw), entry.ByteLength, gotHash, entry.SHA256)
		}
		aggregate.Write(raw)
		var world callHierarchyLayer1CorpusWorld
		callHierarchyLayer1DecodeStrictForTest(t, entry.Name, raw, &world)
		if world.Record.Name != entry.Name || world.RootIdentity != entry.Name || world.RootIdentity != world.Record.Name {
			t.Errorf("ASSERT_CH_CORPUS_WORLD_ROOT_IDENTITY_EXACT_%02d: entry=%q record=%q root=%q", i+1, entry.Name, world.Record.Name, world.RootIdentity)
		}
		worlds = append(worlds, callHierarchyLayer1LoadedWorld{entry: entry, raw: raw, world: world})
	}
	if got := fmt.Sprintf("%x", aggregate.Sum(nil)); got != manifest.Aggregate.SHA256 {
		t.Fatalf("ASSERT_CH_CORPUS_CANONICAL_AGGREGATE: %s", got)
	}
	return worlds
}

func callHierarchyLayer1StringForTest(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func callHierarchyLayer1Uint64ForTest(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

func callHierarchyLayer1BytesForTest(t *testing.T, name string, encoded *string) []byte {
	t.Helper()
	value, err := hex.DecodeString(callHierarchyLayer1StringForTest(encoded))
	if err != nil {
		t.Errorf("ASSERT_CH_CORPUS_HEX_%s: %v", name, err)
	}
	return value
}

type callHierarchyLayer1ExternalSnapshotForTest struct {
	result          []byte
	serverError     []byte
	failure         session.Failure
	messages        int
	bytes           int64
	requestMessages int
	requestBytes    int64
	notifications   []byte
	responses       []byte
}

func callHierarchyLayer1ExternalForTest(t *testing.T, result RoundTripResult) callHierarchyLayer1ExternalSnapshotForTest {
	t.Helper()
	serverError, err := json.Marshal(result.ServerError)
	if err != nil {
		t.Errorf("ASSERT_CH_LAYER1_SERVER_ERROR_SNAPSHOT: %v", err)
	}
	notifications, err := json.Marshal(result.Notifications)
	if err != nil {
		t.Errorf("ASSERT_CH_LAYER1_NOTIFICATIONS_SNAPSHOT: %v", err)
	}
	responses, err := json.Marshal(result.Responses)
	if err != nil {
		t.Errorf("ASSERT_CH_LAYER1_RESPONSES_SNAPSHOT: %v", err)
	}
	return callHierarchyLayer1ExternalSnapshotForTest{
		result: append([]byte(nil), result.Result...), serverError: serverError,
		failure: result.Failure, messages: result.Messages, bytes: result.Bytes,
		requestMessages: result.RequestMessages, requestBytes: result.RequestBytes,
		notifications: notifications, responses: responses,
	}
}

func callHierarchyLayer1RequestForTest(started StartResult, method string, params []byte, selected, handoff []byte) RoundTripRequest {
	return RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: method,
		Params: append(json.RawMessage(nil), params...), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096,
		callHierarchyLayer1: callHierarchyLayer1RequestAttachment{
			selected: callHierarchyOpaqueCommitmentMetadata{identity: "opaque-selected", sha256: sha256.Sum256(selected), length: uint64(len(selected)), available: true},
			handoff:  callHierarchyOpaqueCommitmentMetadata{identity: "opaque-handoff", sha256: sha256.Sum256(handoff), length: uint64(len(handoff)), available: true},
		},
	}
}

func callHierarchyLayer1ModeForCorpusRecord(record callHierarchyLayer1CorpusRecord) string {
	switch record.Error.Class {
	case "protocol":
		return "server-error"
	case "malformed":
		return "malformed"
	case "cancellation", "deadline":
		return "hang"
	}
	switch record.Result.Form {
	case "nonempty":
		return "references-repeated"
	case "null":
		return "references-null"
	default:
		return "references-empty"
	}
}

func callHierarchyLayer1CommitmentForCorpusRecord(t *testing.T, name string, value callHierarchyLayer1CorpusCommitment) callHierarchyOpaqueCommitmentMetadata {
	t.Helper()
	var digest [32]byte
	decoded := callHierarchyLayer1BytesForTest(t, name, value.SHA256)
	if len(decoded) == len(digest) {
		copy(digest[:], decoded)
	}
	return callHierarchyOpaqueCommitmentMetadata{identity: callHierarchyLayer1StringForTest(value.Identity), sha256: digest, length: callHierarchyLayer1Uint64ForTest(value.Length), available: value.Available}
}

type callHierarchyLayer1OrdinarySpecForTest struct {
	mode, errorClass, method string
	params                   json.RawMessage
	cancel, shutdown         bool
	deadline                 time.Duration
	maxMessages              int
	maxBytes                 int64
}

func callHierarchyLayer1OrdinaryModeForCorpusRecordForTest(t *testing.T, record callHierarchyLayer1CorpusRecord) string {
	t.Helper()
	if record.Error.Class != "none" {
		return callHierarchyLayer1ModeForCorpusRecord(record)
	}
	encoded := callHierarchyLayer1StringForTest(&record.External.ResponsesHex)
	responseBytes, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatalf("ASSERT_CH_CURRENT_%s field=external.responses_hex got=%q: %v", record.Name, encoded, err)
	}
	var response struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("ASSERT_CH_CURRENT_%s field=external.responses got=%q: %v", record.Name, responseBytes, err)
	}
	switch callHierarchyLayer1ResultFormForTest(response.Result) {
	case "null":
		return "references-null"
	case "nonempty":
		return "references-repeated"
	default:
		return "references-empty"
	}
}

func callHierarchyLayer1OrdinarySpecFromCorpusForTest(t *testing.T, record callHierarchyLayer1CorpusRecord) callHierarchyLayer1OrdinarySpecForTest {
	t.Helper()
	method := callHierarchyLayer1StringForTest(record.Identity.Method.Value)
	if !record.Identity.Method.Available {
		method = ""
	}
	deadline := time.Second
	if record.Error.Class == "deadline" {
		deadline = 30 * time.Millisecond
	}
	return callHierarchyLayer1OrdinarySpecForTest{
		mode: callHierarchyLayer1OrdinaryModeForCorpusRecordForTest(t, record), errorClass: record.Error.Class, method: method,
		params: append(json.RawMessage(nil), callHierarchyLayer1BytesForTest(t, record.Name+"-params", record.Frames.Request.Payload.BytesHex)...),
		cancel: record.Error.Class == "cancellation", shutdown: record.Error.Class == "shutdown" || record.Error.Class == "transport-unavailable",
		deadline: deadline, maxMessages: 3, maxBytes: 4096,
	}
}

func callHierarchyLayer1RequestFromSpecForTest(started StartResult, spec callHierarchyLayer1OrdinarySpecForTest, attachment callHierarchyLayer1RequestAttachment) RoundTripRequest {
	return RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: spec.method,
		Params: append(json.RawMessage(nil), spec.params...), Deadline: time.Now().Add(spec.deadline),
		MaxMessages: spec.maxMessages, MaxBytes: spec.maxBytes, callHierarchyLayer1: attachment}
}

func callHierarchyLayer1ExecuteSpecForTest(t *testing.T, name string, spec callHierarchyLayer1OrdinarySpecForTest, attachment callHierarchyLayer1RequestAttachment) (RoundTripResult, RoundTripRequest) {
	t.Helper()
	manager, started, _ := roundTripManager(t, spec.mode)
	if spec.shutdown {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatalf("ASSERT_CH_CURRENT_%s field=manager.shutdown got=%v", name, err)
		}
	}
	ctx := context.Background()
	if spec.cancel {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		cancel()
	}
	request := callHierarchyLayer1RequestFromSpecForTest(started, spec, attachment)
	return manager.RoundTrip(ctx, request), request
}

func callHierarchyLayer1ExecuteCorpusRecord(t *testing.T, record callHierarchyLayer1CorpusRecord) (RoundTripResult, callHierarchyLayer1ExternalSnapshotForTest, RoundTripRequest) {
	t.Helper()
	spec := callHierarchyLayer1OrdinarySpecFromCorpusForTest(t, record)
	attachment := callHierarchyLayer1RequestAttachment{}
	if record.Control.Enabled && record.Control.AttachmentAvailable {
		attachment.selected = callHierarchyLayer1CommitmentForCorpusRecord(t, record.Name+"-selected", record.Commitments.Selected)
		attachment.handoff = callHierarchyLayer1CommitmentForCorpusRecord(t, record.Name+"-handoff", record.Commitments.Handoff)
	}
	result, request := callHierarchyLayer1ExecuteSpecForTest(t, record.Name, spec, attachment)
	return result, callHierarchyLayer1ExternalForTest(t, result), request
}

type callHierarchyLayer1ExpectedFieldForTest struct {
	root reflect.Type
	path string
	want reflect.Type
}

func callHierarchyLayer1AssertFieldPathForTest(t *testing.T, world string, expected callHierarchyLayer1ExpectedFieldForTest) {
	t.Helper()
	typ := expected.root
	for _, part := range strings.Split(expected.path, ".") {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		field, ok := typ.FieldByName(part)
		if !ok {
			t.Errorf("ASSERT_CH_API_GAP_%s path=%s expected=%v got=absent", world, expected.path, expected.want)
			return
		}
		typ = field.Type
	}
	if typ != expected.want {
		t.Errorf("ASSERT_CH_API_GAP_%s path=%s expected=%v got=%v", world, expected.path, expected.want, typ)
	}
}

func callHierarchyLayer1AssertScaffoldShapeForTest(t *testing.T, world string) {
	t.Helper()
	view := reflect.TypeOf(callHierarchyObservationView{})
	result := reflect.TypeOf(callHierarchyLayer1ResultAttachment{})
	metadata := reflect.TypeOf(callHierarchyOpaqueCommitmentMetadata{})
	wantMetadata := []struct {
		name string
		typ  reflect.Type
	}{{"identity", reflect.TypeOf("")}, {"sha256", reflect.TypeOf([32]byte{})}, {"length", reflect.TypeOf(uint64(0))}, {"available", reflect.TypeOf(false)}}
	if metadata.NumField() != len(wantMetadata) {
		t.Errorf("ASSERT_CH_OPAQUE_METADATA_SHAPE_%s fields=%d want=%d", world, metadata.NumField(), len(wantMetadata))
	} else {
		for i, want := range wantMetadata {
			field := metadata.Field(i)
			if field.Name != want.name || field.Type != want.typ {
				t.Errorf("ASSERT_CH_OPAQUE_METADATA_SHAPE_%s field[%d]=%s/%v want=%s/%v", world, i, field.Name, field.Type, want.name, want.typ)
			}
		}
	}
	fields := []callHierarchyLayer1ExpectedFieldForTest{
		{view, "identity.request.workspace.managerNonce", reflect.TypeOf([32]byte{})},
		{view, "identity.request.session", reflect.TypeOf(callHierarchySessionIdentity(""))},
		{view, "identity.request.generation", reflect.TypeOf(callHierarchyGeneration(0))},
		{view, "identity.request.requestKey", reflect.TypeOf(callHierarchyJSONRPCRequestKey{})},
		{view, "identity.transaction.local", reflect.TypeOf([32]byte{})},
		{view, "request.frameBytes", reflect.TypeOf([]byte{})},
		{view, "request.bodyOffset", reflect.TypeOf(uint64(0))},
		{view, "request.bodyLength", reflect.TypeOf(uint64(0))},
		{view, "request.bodySHA256", reflect.TypeOf([32]byte{})},
		{view, "request.paramsOffset", reflect.TypeOf(uint64(0))},
		{view, "request.paramsLength", reflect.TypeOf(uint64(0))},
		{view, "request.paramsSHA256", reflect.TypeOf([32]byte{})},
		{view, "response.frameBytes", reflect.TypeOf([]byte{})},
		{view, "response.bodyOffset", reflect.TypeOf(uint64(0))},
		{view, "response.resultOffset", reflect.TypeOf(uint64(0))},
		{result, "snapshot.notificationsBytes", reflect.TypeOf([]byte{})},
		{result, "snapshot.responsesBytes", reflect.TypeOf([]byte{})},
		{result, "snapshot.failure", reflect.TypeOf(session.Failure(""))},
		{result, "snapshot.requestMessages", reflect.TypeOf(callHierarchyMeasure{})},
		{result, "snapshot.requestBytes", reflect.TypeOf(callHierarchyMeasure{})},
		{result, "ledger.terminalBefore", reflect.TypeOf(callHierarchyTransportTerminalSnapshot{})},
		{result, "ledger.terminalAfter", reflect.TypeOf(callHierarchyTransportTerminalSnapshot{})},
	}
	for _, dimension := range []string{"transportMessages", "transportBytes", "externalBytes", "reservations", "leases", "callbacks", "diagnostics"} {
		fields = append(fields,
			callHierarchyLayer1ExpectedFieldForTest{result, "ledger." + dimension + ".knownZero", reflect.TypeOf(false)},
			callHierarchyLayer1ExpectedFieldForTest{result, "ledger." + dimension + ".delta", reflect.TypeOf(uint64(0))},
		)
	}
	for _, expected := range fields {
		callHierarchyLayer1AssertFieldPathForTest(t, world, expected)
	}
	for _, path := range []string{"accounting.owners", "accounting.accountingTransfers", "transportPublication", "transportRetention"} {
		callHierarchyLayer1AssertFieldPathForTest(t, world, callHierarchyLayer1ExpectedFieldForTest{root: view, path: path, want: reflect.TypeOf(callHierarchyMeasuredDelta{})})
	}
}

func callHierarchyLayer1MeasureFromCorpus(value callHierarchyLayer1CorpusAvailableUint64) callHierarchyMeasure {
	return callHierarchyMeasure{available: value.Available, value: callHierarchyLayer1Uint64ForTest(value.Value)}
}

func callHierarchyLayer1AssertReflectedValueForTest(t *testing.T, world, path string, owner any, fieldName string, expected any) {
	t.Helper()
	value := reflect.ValueOf(owner)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			t.Errorf("ASSERT_CH_API_GAP_%s path=%s expected=%v got=absent", world, path, expected)
			return
		}
		value = value.Elem()
	}
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		t.Errorf("ASSERT_CH_API_GAP_%s path=%s expected=%v got=absent", world, path, expected)
		return
	}
	want := reflect.ValueOf(expected)
	if !want.IsValid() || field.Type() != want.Type() {
		t.Errorf("ASSERT_CH_API_GAP_%s path=%s expected=%v expectedType=%T gotType=%v", world, path, expected, expected, field.Type())
		return
	}
	if !reflect.DeepEqual(field.Interface(), want.Interface()) {
		t.Errorf("ASSERT_CH_CORPUS_EXACT_%s path=%s got=%v expected=%v", world, path, field.Interface(), expected)
	}
}

func callHierarchyLayer1AssertMeasureForTest(t *testing.T, world, path string, expected callHierarchyLayer1CorpusAvailableUint64, actual callHierarchyMeasure) {
	t.Helper()
	if actual != callHierarchyLayer1MeasureFromCorpus(expected) {
		t.Errorf("ASSERT_CH_CORPUS_MEASURE_%s path=%s got=%+v expected=%+v", world, path, actual, expected)
	}
	callHierarchyLayer1AssertReflectedValueForTest(t, world, path+".knownZero", actual, "knownZero", expected.KnownZero)
}

func sha256ArrayForHexForTest(t *testing.T, name, encoded string) [32]byte {
	t.Helper()
	var value [32]byte
	decoded := callHierarchyLayer1BytesForTest(t, name, &encoded)
	copy(value[:], decoded)
	return value
}

func frameSliceForTest(frame []byte, offset, length uint64) []byte {
	if offset > uint64(len(frame)) || length > uint64(len(frame))-offset {
		return nil
	}
	return frame[offset : offset+length]
}

func callHierarchyLayer1AssertLedgerDimensionForTest(t *testing.T, world, name string, expected callHierarchyLayer1CorpusLedgerDimension, actual callHierarchyMeasuredDelta) {
	t.Helper()
	before := callHierarchyLayer1Uint64ForTest(expected.Before)
	after := callHierarchyLayer1Uint64ForTest(expected.After)
	if actual.before.available != expected.Available || actual.after.available != expected.Available || actual.before.value != before || actual.after.value != after {
		t.Errorf("ASSERT_CH_CORPUS_LEDGER_%s_%s availability=%v before=%v after=%v actual=%+v", world, name, expected.Available, expected.Before, expected.After, actual)
	}
	callHierarchyLayer1AssertReflectedValueForTest(t, world, "RoundTripResult.callHierarchyLayer1.ledger."+name+".delta", actual, "delta", callHierarchyLayer1Uint64ForTest(expected.Delta))
	callHierarchyLayer1AssertReflectedValueForTest(t, world, "RoundTripResult.callHierarchyLayer1.ledger."+name+".knownZero", actual, "knownZero", expected.KnownZero)
}

func callHierarchyLayer1AssertLayer1ResultForTest(t *testing.T, expected callHierarchyLayer1CorpusRecord, actual callHierarchyLayer1ResultAttachment) {
	t.Helper()
	wantResult := callHierarchyLayer1BytesForTest(t, expected.Name+"-layer-result", expected.Result.BytesHex)
	wantError := callHierarchyLayer1BytesForTest(t, expected.Name+"-layer-error", &expected.Error.SerializedHex)
	wantData := []byte(nil)
	if expected.Error.DataBase64 != nil {
		var err error
		wantData, err = base64.StdEncoding.DecodeString(*expected.Error.DataBase64)
		if err != nil {
			t.Errorf("ASSERT_CH_CORPUS_ERROR_DATA_BASE64_%s: %v", expected.Name, err)
		}
	}
	if actual.snapshot.resultAvailable != expected.Result.Available || !reflect.DeepEqual(actual.snapshot.resultBytes, wantResult) {
		t.Errorf("ASSERT_CH_CORPUS_LAYER_RESULT_%s availability/form/bytes got=%+v want=%+v", expected.Name, actual.snapshot, expected.Result)
	}
	if actual.snapshot.errorAvailable != expected.Error.Available || actual.snapshot.error.presentAvailable != expected.Error.Available || actual.snapshot.error.present != expected.Error.Present || actual.snapshot.error.classAvailable != expected.Error.Available || actual.snapshot.error.class != expected.Error.Class || actual.snapshot.error.messageAvailable != (expected.Error.Message != nil) || actual.snapshot.error.message != callHierarchyLayer1StringForTest(expected.Error.Message) || actual.snapshot.error.dataAvailable != (expected.Error.DataBase64 != nil) || !reflect.DeepEqual(actual.snapshot.error.data, wantData) || actual.snapshot.error.codeAvailable != (expected.Error.Code != nil) || (expected.Error.Code != nil && actual.snapshot.error.code != *expected.Error.Code) || actual.snapshot.error.serializedAvailable != expected.Error.Available || !reflect.DeepEqual(actual.snapshot.error.serializedBytes, wantError) {
		t.Errorf("ASSERT_CH_CORPUS_LAYER_ERROR_ALL_FIELDS_FORMS_SERIALIZED_%s got=%+v want=%+v", expected.Name, actual.snapshot.error, expected.Error)
	}
	if actual.snapshot.terminal.available != expected.Terminal.Available || actual.snapshot.terminal.value != callHierarchyLayer1StringForTest(expected.Terminal.Value) || actual.snapshot.externalAvailable != expected.External.Available {
		t.Errorf("ASSERT_CH_CORPUS_LAYER_TERMINAL_EXTERNAL_%s got=%+v wantTerminal=%+v", expected.Name, actual.snapshot, expected.Terminal)
	}
	callHierarchyLayer1AssertMeasureForTest(t, expected.Name, "RoundTripResult.callHierarchyLayer1.snapshot.transportMessages", expected.Transport.Messages, actual.snapshot.transportMessages)
	callHierarchyLayer1AssertMeasureForTest(t, expected.Name, "RoundTripResult.callHierarchyLayer1.snapshot.transportBytes", expected.Transport.Bytes, actual.snapshot.transportBytes)
	callHierarchyLayer1AssertReflectedValueForTest(t, expected.Name, "RoundTripResult.RequestMessagesKnownZero", actual, "requestMessagesKnownZero", expected.Transport.RequestMessages.KnownZero)
	callHierarchyLayer1AssertReflectedValueForTest(t, expected.Name, "RoundTripResult.RequestBytesKnownZero", actual, "requestBytesKnownZero", expected.Transport.RequestBytes.KnownZero)
	d := expected.RoundTripLedger.Dimensions
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "transport_messages", d.TransportMessages, actual.ledger.transportMessages)
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "transport_bytes", d.TransportBytes, actual.ledger.transportBytes)
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "external_bytes", d.ExternalBytes, actual.ledger.externalBytes)
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "reservations", d.Reservations, actual.ledger.reservations)
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "leases", d.Leases, actual.ledger.leases)
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "callbacks", d.Callbacks, actual.ledger.callbacks)
	callHierarchyLayer1AssertLedgerDimensionForTest(t, expected.Name, "diagnostics", d.Diagnostics, actual.ledger.diagnostics)
	if actual.ledger.terminalBefore.available != expected.RoundTripLedger.Terminal.Before.Available || actual.ledger.terminalBefore.value != callHierarchyLayer1StringForTest(expected.RoundTripLedger.Terminal.Before.Value) || actual.ledger.terminalAfter.available != expected.RoundTripLedger.Terminal.After.Available || actual.ledger.terminalAfter.value != callHierarchyLayer1StringForTest(expected.RoundTripLedger.Terminal.After.Value) {
		t.Errorf("ASSERT_CH_CORPUS_LEDGER_TERMINAL_BEFORE_AFTER_%s got=%+v/%+v want=%+v", expected.Name, actual.ledger.terminalBefore, actual.ledger.terminalAfter, expected.RoundTripLedger.Terminal)
	}
}

func callHierarchyLayer1ResultFormForTest(value []byte) string {
	if value == nil {
		return "unavailable"
	}
	if bytes.Equal(value, []byte("null")) {
		return "null"
	}
	if len(value) == 0 || bytes.Equal(value, []byte("[]")) {
		return "empty"
	}
	return "nonempty"
}

func callHierarchyLayer1EmitObservationGapsForTest(t *testing.T, expected callHierarchyLayer1CorpusRecord) {
	t.Helper()
	sections := []struct {
		path  string
		value any
	}{
		{"identity", expected.Identity}, {"frames", expected.Frames}, {"commitments", expected.Commitments},
		{"accounting", expected.Accounting}, {"lifecycle", expected.Lifecycle}, {"refusal", expected.Refusal},
		{"result", expected.Result}, {"error", expected.Error}, {"terminal", expected.Terminal},
		{"transport", expected.Transport}, {"round_trip_ledger", expected.RoundTripLedger},
	}
	var emit func(string, reflect.Value)
	emit = func(path string, value reflect.Value) {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				t.Errorf("ASSERT_CH_API_GAP_%s path=callHierarchyObservation.%s expected=<nil> got=absent", expected.Name, path)
				return
			}
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Struct:
			typ := value.Type()
			for i := 0; i < value.NumField(); i++ {
				name := typ.Field(i).Tag.Get("json")
				name = strings.Split(name, ",")[0]
				if name == "" || name == "-" {
					name = typ.Field(i).Name
				}
				emit(path+"."+name, value.Field(i))
			}
		case reflect.Slice, reflect.Array:
			if value.Len() == 0 {
				t.Errorf("ASSERT_CH_API_GAP_%s path=callHierarchyObservation.%s expected=%v got=absent", expected.Name, path, value.Interface())
				return
			}
			for i := 0; i < value.Len(); i++ {
				emit(fmt.Sprintf("%s[%d]", path, i), value.Index(i))
			}
		default:
			t.Errorf("ASSERT_CH_API_GAP_%s path=callHierarchyObservation.%s expected=%v got=absent", expected.Name, path, value.Interface())
		}
	}
	for _, section := range sections {
		emit(section.path, reflect.ValueOf(section.value))
	}
}

func callHierarchyLayer1SuccessfulResultFormForTest(mode string) string {
	switch mode {
	case "references-null":
		return "null"
	case "references-repeated":
		return "nonempty"
	default:
		return "empty"
	}
}

func callHierarchyLayer1AssertCurrentRoundTripForTest(t *testing.T, world string, spec callHierarchyLayer1OrdinarySpecForTest, result RoundTripResult) {
	t.Helper()
	form := callHierarchyLayer1ResultFormForTest(result.Result)
	if spec.cancel && result.Failure != session.RequestCancelled {
		t.Errorf("ASSERT_CH_CURRENT_%s field=Failure got=%q want=%q", world, result.Failure, session.RequestCancelled)
	}
	switch result.Failure {
	case "":
		if result.ServerError == nil {
			if want := callHierarchyLayer1SuccessfulResultFormForTest(spec.mode); form != want {
				t.Errorf("ASSERT_CH_CURRENT_%s field=Result.form got=%q want=%q result=%q", world, form, want, result.Result)
			}
		} else if form != "unavailable" || result.ServerError.Code == 0 || result.ServerError.Message == "" {
			t.Errorf("ASSERT_CH_CURRENT_%s field=ServerError got=%+v resultForm=%q want=typed-code-message/no-result", world, result.ServerError, form)
		}
	case session.RequestCancelled:
		if !spec.cancel {
			t.Errorf("ASSERT_CH_CURRENT_%s field=Failure got=%q without cancelled context", world, result.Failure)
		}
	case session.RequestTimeout:
		if spec.mode != "hang" || spec.deadline <= 0 {
			t.Errorf("ASSERT_CH_CURRENT_%s field=Failure got=%q without executor deadline", world, result.Failure)
		}
	case session.SessionPoisoned:
		if !callHierarchyLayer1PoisonPermittedForTest(spec) {
			t.Errorf("ASSERT_CH_CURRENT_%s field=Failure got=%q fixture=%+v want=poisoning-permitted-fixture", world, result.Failure, spec)
		}
	default:
		t.Errorf("ASSERT_CH_CURRENT_%s field=Failure got=%q want=current round-trip vocabulary", world, result.Failure)
	}
	if result.Failure != "" && (form != "unavailable" || result.ServerError != nil) {
		t.Errorf("ASSERT_CH_CURRENT_%s field=failure-representation failure=%q resultForm=%q serverError=%+v", world, result.Failure, form, result.ServerError)
	}
	if result.Messages < 0 || result.Bytes < 0 || result.RequestMessages < 0 || result.RequestBytes < 0 || (result.Messages == 0) != (result.Bytes == 0) || result.Messages > spec.maxMessages || result.Bytes > spec.maxBytes {
		t.Errorf("ASSERT_CH_CURRENT_%s field=accounting messages=%d bytes=%d requestMessages=%d requestBytes=%d limits=%d/%d", world, result.Messages, result.Bytes, result.RequestMessages, result.RequestBytes, spec.maxMessages, spec.maxBytes)
	}
	for i, notification := range result.Notifications {
		if notification.Method == "" || notification.ID != nil {
			t.Errorf("ASSERT_CH_CURRENT_%s field=Notifications[%d] got=%+v", world, i, notification)
		}
	}
	for i, response := range result.Responses {
		if response.ID == nil || response.Method != "" {
			t.Errorf("ASSERT_CH_CURRENT_%s field=Responses[%d] got=%+v", world, i, response)
		}
	}
}

func callHierarchyLayer1AssertOrdinaryCorpusRecordForTest(t *testing.T, expected callHierarchyLayer1CorpusRecord, result RoundTripResult, request RoundTripRequest) {
	t.Helper()
	actualAttachmentAvailable := request.callHierarchyLayer1 != (callHierarchyLayer1RequestAttachment{})
	if expected.Control.Enabled != actualAttachmentAvailable || expected.Control.AttachmentAvailable != actualAttachmentAvailable {
		t.Errorf("ASSERT_CH_CURRENT_%s field=request.callHierarchyLayer1 enabled=%v attachmentAvailable=%v got=%v", expected.Name, expected.Control.Enabled, expected.Control.AttachmentAvailable, actualAttachmentAvailable)
	}
	if !actualAttachmentAvailable && expected.Control.Reason != nil {
		t.Errorf("ASSERT_CH_CURRENT_%s field=control.reason got=unexposed want=nil corpusReason=%q", expected.Name, *expected.Control.Reason)
	}
	callHierarchyLayer1AssertCurrentRoundTripForTest(t, expected.Name, callHierarchyLayer1OrdinarySpecFromCorpusForTest(t, expected), result)
}

func callHierarchyLayer1AssertCorpusRecordForTest(t *testing.T, expected callHierarchyLayer1CorpusRecord, result RoundTripResult, external callHierarchyLayer1ExternalSnapshotForTest, request RoundTripRequest) {
	t.Helper()
	callHierarchyLayer1AssertScaffoldShapeForTest(t, expected.Name)
	callHierarchyLayer1AssertOrdinaryCorpusRecordForTest(t, expected, result, request)
	lease, ok := result.privateCallHierarchyObservationLease()
	expectedObservation := expected.Control.AttachmentAvailable && expected.Control.ObservationAvailable && !expected.Refusal.Refused
	if !expectedObservation {
		if ok {
			t.Errorf("ASSERT_CH_CORPUS_OBSERVATION_EXPECTED_ABSENT_%s reason=%q", expected.Name, callHierarchyLayer1StringForTest(expected.Refusal.Reason))
			lease.release()
		}
		return
	}
	if !ok {
		t.Errorf("ASSERT_CH_CORPUS_PROPAGATION_MISSING_%s path=RoundTripResult.callHierarchyObservation expected=true", expected.Name)
		callHierarchyLayer1EmitObservationGapsForTest(t, expected)
		return
	}
	callHierarchyLayer1AssertLayer1ResultForTest(t, expected, result.callHierarchyLayer1)
	var view callHierarchyObservationView
	completed := false
	if err := lease.withObservation(func(got callHierarchyObservationView) error { view = got; completed = true; return nil }); err != nil {
		t.Errorf("ASSERT_CH_CORPUS_COMPLETE_PRIVATE_OBSERVATION_%s: %v", expected.Name, err)
	}
	wantWorkspace := sha256ArrayForHexForTest(t, expected.Name+"-workspace", expected.Identity.RequestIdentity.WorkspaceNonceHex)
	wantTransaction := sha256ArrayForHexForTest(t, expected.Name+"-transaction", expected.Identity.Transaction.LocalHex)
	wantRequestKey := []byte(strconv.FormatUint(expected.Identity.RequestIdentity.RequestKey, 10))
	if view.identity.request.workspace.managerNonce != wantWorkspace || view.identity.request.session != callHierarchySessionIdentity(expected.Identity.RequestIdentity.Session) || view.identity.request.generation != callHierarchyGeneration(expected.Identity.RequestIdentity.Generation) || view.identity.request.requestKey.kind != callHierarchyJSONRPCRequestKeyNumber || view.identity.request.requestKey.stringValue != "" || !bytes.Equal(view.identity.request.requestKey.numberBytes, wantRequestKey) || !reflect.DeepEqual(view.identity.transaction.request, view.identity.request) || view.identity.transaction.local != wantTransaction {
		t.Errorf("ASSERT_CH_CORPUS_IDENTITY_COMPLETE_%s: got=%+v want=%+v", expected.Name, view.identity, expected.Identity)
	}
	if view.identity.method != callHierarchyMethodIdentity(callHierarchyLayer1StringForTest(expected.Identity.Method.Value)) || view.identity.binding != callHierarchyBindingIdentity(callHierarchyLayer1StringForTest(expected.Identity.Binding.Value)) || view.identity.profile != callHierarchyProfileIdentity(callHierarchyLayer1StringForTest(expected.Identity.Profile.Value)) {
		t.Errorf("ASSERT_CH_CORPUS_IDENTITY_PROFILE_BINDING_METHOD_%s: got=%+v want=%+v", expected.Name, view.identity, expected.Identity)
	}
	wantRequestFrame := callHierarchyLayer1BytesForTest(t, expected.Name+"-request-frame", &expected.Frames.Request.FrameHex)
	wantResponseFrame := callHierarchyLayer1BytesForTest(t, expected.Name+"-response-frame", &expected.Frames.Response.FrameHex)
	wantRequestBody := callHierarchyLayer1BytesForTest(t, expected.Name+"-request-body", expected.Frames.Request.Body.BytesHex)
	wantRequestPayload := callHierarchyLayer1BytesForTest(t, expected.Name+"-request-payload", expected.Frames.Request.Payload.BytesHex)
	wantResponseBody := callHierarchyLayer1BytesForTest(t, expected.Name+"-response-body", expected.Frames.Response.Body.BytesHex)
	wantResponsePayload := callHierarchyLayer1BytesForTest(t, expected.Name+"-response-payload", expected.Frames.Response.Payload.BytesHex)
	var requestBodySlice, requestPayloadSlice []byte
	if view.request != nil {
		requestBodySlice = frameSliceForTest(view.request.frameBytes, view.request.bodyOffset, view.request.bodyLength)
		requestPayloadSlice = frameSliceForTest(view.request.frameBytes, view.request.paramsOffset, view.request.paramsLength)
	}
	if view.request == nil || !expected.Frames.Request.Available || !expected.Frames.Request.Body.Available || !expected.Frames.Request.Payload.Available || !reflect.DeepEqual(view.request.frameBytes, wantRequestFrame) || view.request.frameLength != expected.Frames.Request.FrameLength || view.request.frameSHA256 != sha256ArrayForHexForTest(t, expected.Name+"-request-frame-declared", expected.Frames.Request.FrameSHA256) || view.request.frameSHA256 != sha256.Sum256(view.request.frameBytes) || view.request.bodyLength != callHierarchyLayer1Uint64ForTest(expected.Frames.Request.Body.Length) || view.request.bodySHA256 != sha256ArrayForHexForTest(t, expected.Name+"-request-body-declared", callHierarchyLayer1StringForTest(expected.Frames.Request.Body.SHA256)) || view.request.bodySHA256 != sha256.Sum256(requestBodySlice) || !reflect.DeepEqual(requestBodySlice, wantRequestBody) || view.request.paramsLength != callHierarchyLayer1Uint64ForTest(expected.Frames.Request.Payload.Length) || view.request.paramsSHA256 != sha256ArrayForHexForTest(t, expected.Name+"-request-payload-declared", callHierarchyLayer1StringForTest(expected.Frames.Request.Payload.SHA256)) || view.request.paramsSHA256 != sha256.Sum256(requestPayloadSlice) || !reflect.DeepEqual(requestPayloadSlice, wantRequestPayload) {
		t.Errorf("ASSERT_CH_CORPUS_REQUEST_FRAME_EXACT_%s", expected.Name)
	}
	var responseBodySlice, responsePayloadSlice []byte
	if view.response != nil {
		responseBodySlice = frameSliceForTest(view.response.frameBytes, view.response.bodyOffset, view.response.bodyLength)
		responsePayloadSlice = frameSliceForTest(view.response.frameBytes, view.response.resultOffset, view.response.resultLength)
	}
	if view.response == nil || !expected.Frames.Response.Available || !expected.Frames.Response.Body.Available || !expected.Frames.Response.Payload.Available || !reflect.DeepEqual(view.response.frameBytes, wantResponseFrame) || view.response.frameLength != expected.Frames.Response.FrameLength || view.response.frameSHA256 != sha256ArrayForHexForTest(t, expected.Name+"-response-frame-declared", expected.Frames.Response.FrameSHA256) || view.response.frameSHA256 != sha256.Sum256(view.response.frameBytes) || view.response.bodyLength != callHierarchyLayer1Uint64ForTest(expected.Frames.Response.Body.Length) || view.response.bodySHA256 != sha256ArrayForHexForTest(t, expected.Name+"-response-body-declared", callHierarchyLayer1StringForTest(expected.Frames.Response.Body.SHA256)) || view.response.bodySHA256 != sha256.Sum256(responseBodySlice) || !reflect.DeepEqual(responseBodySlice, wantResponseBody) || view.response.resultLength != callHierarchyLayer1Uint64ForTest(expected.Frames.Response.Payload.Length) || view.response.resultSHA256 != sha256ArrayForHexForTest(t, expected.Name+"-response-payload-declared", callHierarchyLayer1StringForTest(expected.Frames.Response.Payload.SHA256)) || view.response.resultSHA256 != sha256.Sum256(responsePayloadSlice) || !reflect.DeepEqual(responsePayloadSlice, wantResponsePayload) {
		t.Errorf("ASSERT_CH_CORPUS_RESPONSE_FRAME_EXACT_%s", expected.Name)
	}
	selected := callHierarchyLayer1CommitmentForCorpusRecord(t, expected.Name+"-selected-view", expected.Commitments.Selected)
	handoff := callHierarchyLayer1CommitmentForCorpusRecord(t, expected.Name+"-handoff-view", expected.Commitments.Handoff)
	if request.callHierarchyLayer1.selected != selected || request.callHierarchyLayer1.handoff != handoff {
		t.Errorf("ASSERT_CH_CORPUS_REQUEST_COMMITMENT_METADATA_%s got=%+v expectedSelected=%+v expectedHandoff=%+v", expected.Name, request.callHierarchyLayer1, selected, handoff)
	}
	if view.selected == nil || view.selected.exactSHA256 != selected.sha256 || view.selected.sourceLength != selected.length || view.handoff == nil || view.handoff.successorSHA256 != handoff.sha256 || view.handoff.successorLength != handoff.length || len(view.predecessors) != len(expected.Commitments.Predecessors) {
		t.Errorf("ASSERT_CH_CORPUS_COMMITMENTS_AVAILABILITY_IDENTITY_LENGTH_DIGEST_%s", expected.Name)
	}
	for i, predecessor := range expected.Commitments.Predecessors {
		want := callHierarchyLayer1CommitmentForCorpusRecord(t, fmt.Sprintf("%s-predecessor-%d", expected.Name, i), predecessor)
		if i >= len(view.predecessors) || string(view.predecessors[i].identity) != want.identity || view.predecessors[i].commitmentSHA256 != want.sha256 {
			t.Errorf("ASSERT_CH_CORPUS_PREDECESSOR_CARDINALITY_CONTENT_%s_%d", expected.Name, i)
		}
	}
	callHierarchyLayer1AssertMeasureForTest(t, expected.Name, "callHierarchyObservation.accounting.bytes", expected.Accounting.Bytes, view.accounting.bytes)
	callHierarchyLayer1AssertMeasureForTest(t, expected.Name, "callHierarchyObservation.accounting.objects", expected.Accounting.Objects, view.accounting.objects)
	callHierarchyLayer1AssertMeasureForTest(t, expected.Name, "callHierarchyObservation.accounting.work", expected.Accounting.Work, view.accounting.work)
	callHierarchyLayer1AssertMeasureForTest(t, expected.Name, "callHierarchyObservation.accounting.diagnostics", expected.Accounting.Diagnostics, view.accounting.diagnostics)
	released := false
	if ok {
		released = lease.release()
		if !released || lease.release() {
			t.Errorf("ASSERT_CH_CORPUS_LIFECYCLE_EXACT_ONCE_%s", expected.Name)
		}
	}
	if expected.Lifecycle.Acquired != ok || expected.Lifecycle.Completed != completed || expected.Lifecycle.Released != released {
		t.Errorf("ASSERT_CH_CORPUS_LIFECYCLE_BOOLEANS_%s: got=%v/%v/%v want=%+v", expected.Name, ok, completed, released, expected.Lifecycle)
	}
	callHierarchyLayer1AssertReflectedValueForTest(t, expected.Name, "callHierarchyObservation.lifecycle.sequence", view, "lifecycleSequence", expected.Lifecycle.Sequence)
	if expected.Refusal.Refused != !ok {
		t.Errorf("ASSERT_CH_CORPUS_REFUSAL_%s gotRefused=%v expected=%v", expected.Name, !ok, expected.Refusal.Refused)
	}
	if expected.Refusal.Reason != nil {
		callHierarchyLayer1AssertReflectedValueForTest(t, expected.Name, "RoundTripResult.callHierarchyLayer1.refusal.reason", result.callHierarchyLayer1, "refusalReason", callHierarchyLayer1StringForTest(expected.Refusal.Reason))
	}
	callHierarchyLayer1AssertReflectedValueForTest(t, expected.Name, "RoundTripResult.callHierarchyLayer1.commitments", result.callHierarchyLayer1, "commitments", expected.Commitments)
}

func callHierarchyLayer1AssertGapForTest(t *testing.T, world string, enabled bool, result RoundTripResult) {
	t.Helper()
	if lease, ok := result.privateCallHierarchyObservationLease(); ok {
		lease.release()
		t.Fatalf("ASSERT_CH_CURRENT_%s field=proposal.lease got=present want=absent enabled=%v", world, enabled)
	}
	if !reflect.DeepEqual(result.callHierarchyLayer1, callHierarchyLayer1ResultAttachment{}) {
		t.Fatalf("ASSERT_CH_CURRENT_%s field=proposal.attachment got=%+v want=absent enabled=%v", world, result.callHierarchyLayer1, enabled)
	}
}

func callHierarchyLayer1PoisonPermittedForTest(spec callHierarchyLayer1OrdinarySpecForTest) bool {
	if spec.cancel {
		return false
	}
	switch spec.errorClass {
	case "protocol", "malformed", "shutdown", "transport-unavailable":
		return true
	case "deadline":
		return spec.mode == "hang" && spec.deadline > 0
	case "none":
		return spec.mode == "references-repeated" && len(spec.params) != 0 && !json.Valid(spec.params)
	default:
		return false
	}
}

func callHierarchyLayer1AssertOrdinaryInputsEqualForTest(t *testing.T, disabledName, enabledName string, disabledSpec, enabledSpec callHierarchyLayer1OrdinarySpecForTest, disabled, enabled RoundTripRequest, disabledCtx, enabledCtx context.Context) {
	t.Helper()
	assertEqual := func(field string, disabled, enabled any) {
		t.Helper()
		if !reflect.DeepEqual(disabled, enabled) {
			t.Fatalf("ASSERT_CH_PAIR_INPUT_%s_%s field=%s disabled=%+v enabled=%+v", disabledName, enabledName, field, disabled, enabled)
		}
	}
	assertEqual("fixture.mode", disabledSpec.mode, enabledSpec.mode)
	assertEqual("fixture.errorClass", disabledSpec.errorClass, enabledSpec.errorClass)
	assertEqual("fixture.method", disabledSpec.method, enabledSpec.method)
	assertEqual("fixture.params", disabledSpec.params, enabledSpec.params)
	assertEqual("fixture.cancel", disabledSpec.cancel, enabledSpec.cancel)
	assertEqual("fixture.shutdown", disabledSpec.shutdown, enabledSpec.shutdown)
	assertEqual("fixture.deadline", disabledSpec.deadline, enabledSpec.deadline)
	assertEqual("fixture.maxMessages", disabledSpec.maxMessages, enabledSpec.maxMessages)
	assertEqual("fixture.maxBytes", disabledSpec.maxBytes, enabledSpec.maxBytes)
	assertEqual("fixture.context-cancelled", errors.Is(disabledCtx.Err(), context.Canceled), errors.Is(enabledCtx.Err(), context.Canceled))
	if errors.Is(disabledCtx.Err(), context.Canceled) != disabledSpec.cancel || errors.Is(enabledCtx.Err(), context.Canceled) != enabledSpec.cancel {
		t.Fatalf("ASSERT_CH_PAIR_INPUT_%s_%s field=fixture.context-cancelled-vs-spec disabled=%v/%v enabled=%v/%v", disabledName, enabledName, errors.Is(disabledCtx.Err(), context.Canceled), disabledSpec.cancel, errors.Is(enabledCtx.Err(), context.Canceled), enabledSpec.cancel)
	}

	disabledAttachment, enabledAttachment := disabled.callHierarchyLayer1, enabled.callHierarchyLayer1
	assertEqual("request.Generation", disabled.Generation, enabled.Generation)
	assertEqual("request.Method", disabled.Method, enabled.Method)
	assertEqual("request.Params", disabled.Params, enabled.Params)
	assertEqual("request.deadline-duration", disabledSpec.deadline, enabledSpec.deadline)
	assertEqual("request.MaxMessages", disabled.MaxMessages, enabled.MaxMessages)
	assertEqual("request.MaxBytes", disabled.MaxBytes, enabled.MaxBytes)
	if disabled.SessionID == "" || enabled.SessionID == "" {
		t.Fatalf("ASSERT_CH_PAIR_INPUT_%s_%s field=request.SessionID disabled=%q enabled=%q", disabledName, enabledName, disabled.SessionID, enabled.SessionID)
	}
	if disabled.Deadline.IsZero() || enabled.Deadline.IsZero() {
		t.Fatalf("ASSERT_CH_PAIR_INPUT_%s_%s field=request.Deadline disabled=%v enabled=%v", disabledName, enabledName, disabled.Deadline, enabled.Deadline)
	}
	if disabledAttachment != (callHierarchyLayer1RequestAttachment{}) || enabledAttachment == (callHierarchyLayer1RequestAttachment{}) {
		t.Fatalf("ASSERT_CH_PAIR_INPUT_%s_%s field=private-attachment disabled=%+v enabled=%+v", disabledName, enabledName, disabledAttachment, enabledAttachment)
	}
	if disabledSpec.mode == "" || disabledSpec.deadline <= 0 || disabledSpec.maxMessages <= 0 || disabledSpec.maxBytes <= 0 || enabledSpec.mode == "" || enabledSpec.deadline <= 0 || enabledSpec.maxMessages <= 0 || enabledSpec.maxBytes <= 0 {
		t.Fatalf("ASSERT_CH_PAIR_INPUT_%s_%s field=fixture-spec disabled=%+v enabled=%+v", disabledName, enabledName, disabledSpec, enabledSpec)
	}
}

func callHierarchyLayer1AssertPairResultsEqualForTest(t *testing.T, disabledName, enabledName string, disabled, enabled RoundTripResult) {
	t.Helper()
	if !bytes.Equal(disabled.Result, enabled.Result) || !reflect.DeepEqual(disabled.ServerError, enabled.ServerError) || disabled.Failure != enabled.Failure ||
		disabled.Messages != enabled.Messages || disabled.Bytes != enabled.Bytes || disabled.RequestMessages != enabled.RequestMessages || disabled.RequestBytes != enabled.RequestBytes ||
		!reflect.DeepEqual(disabled.Notifications, enabled.Notifications) || !reflect.DeepEqual(disabled.Responses, enabled.Responses) {
		t.Errorf("ASSERT_CH_PAIR_RESULT_%s_%s field=current-roundtrip disabled=%+v enabled=%+v", disabledName, enabledName, disabled, enabled)
	}
}

func TestCallHierarchyLayer1AcceptedCorpusRED(t *testing.T) {
	worlds := callHierarchyLayer1LoadCorpusForTest(t)
	collect := func(record callHierarchyLayer1CorpusRecord) RoundTripResult {
		result, _, request := callHierarchyLayer1ExecuteCorpusRecord(t, record)
		callHierarchyLayer1AssertOrdinaryCorpusRecordForTest(t, record, result, request)
		callHierarchyLayer1AssertGapForTest(t, record.Name, record.Control.Enabled, result)
		return result
	}
	for _, loaded := range worlds {
		loaded := loaded
		t.Run(loaded.entry.Name, func(t *testing.T) { collect(loaded.world.Record) })
	}
	w24 := worlds[23].world.Record
	if !w24.Concurrency.Available || len(w24.Concurrency.Workers) != 16 {
		t.Errorf("ASSERT_CH_CORPUS_W24_16_WORKERS: %+v", w24.Concurrency)
	}
	t.Run("world-24-workers", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range w24.Concurrency.Workers {
			i := i
			wg.Add(1)
			go func() { defer wg.Done(); collect(w24.Concurrency.Workers[i]) }()
		}
		wg.Wait()
	})
	w25 := worlds[24].world.Record
	if len(w25.Comparison.Executions) != 22 || len(w25.Comparison.Pairs) != 11 {
		t.Fatalf("ASSERT_CH_CORPUS_W25_22_EXECUTIONS_11_PAIRS: %+v", w25.Comparison)
	}
	for _, pair := range w25.Comparison.Pairs {
		pair := pair
		t.Run("pair-"+pair.Disabled.Name+"-"+pair.Enabled.Name, func(t *testing.T) {
			disabledSpec := callHierarchyLayer1OrdinarySpecFromCorpusForTest(t, pair.Disabled)
			enabledSpec := callHierarchyLayer1OrdinarySpecFromCorpusForTest(t, pair.Enabled)
			disabledManager, disabledStarted, _ := roundTripManager(t, disabledSpec.mode)
			enabledManager, enabledStarted, _ := roundTripManager(t, enabledSpec.mode)
			if disabledSpec.shutdown {
				if err := disabledManager.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if enabledSpec.shutdown {
				if err := enabledManager.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			attachment := callHierarchyLayer1RequestAttachment{
				selected: callHierarchyLayer1CommitmentForCorpusRecord(t, pair.Enabled.Name+"-selected", pair.Enabled.Commitments.Selected),
				handoff:  callHierarchyLayer1CommitmentForCorpusRecord(t, pair.Enabled.Name+"-handoff", pair.Enabled.Commitments.Handoff),
			}
			disabledRequest := callHierarchyLayer1RequestFromSpecForTest(disabledStarted, disabledSpec, callHierarchyLayer1RequestAttachment{})
			enabledRequest := callHierarchyLayer1RequestFromSpecForTest(enabledStarted, enabledSpec, attachment)
			disabledCtx, enabledCtx := context.Background(), context.Background()
			if disabledSpec.cancel {
				var disabledCancel context.CancelFunc
				disabledCtx, disabledCancel = context.WithCancel(disabledCtx)
				disabledCancel()
			}
			if enabledSpec.cancel {
				var enabledCancel context.CancelFunc
				enabledCtx, enabledCancel = context.WithCancel(enabledCtx)
				enabledCancel()
			}
			callHierarchyLayer1AssertOrdinaryInputsEqualForTest(t, pair.Disabled.Name, pair.Enabled.Name, disabledSpec, enabledSpec, disabledRequest, enabledRequest, disabledCtx, enabledCtx)
			disabledResult := disabledManager.RoundTrip(disabledCtx, disabledRequest)
			enabledResult := enabledManager.RoundTrip(enabledCtx, enabledRequest)
			callHierarchyLayer1AssertCurrentRoundTripForTest(t, pair.Disabled.Name, disabledSpec, disabledResult)
			callHierarchyLayer1AssertCurrentRoundTripForTest(t, pair.Enabled.Name, enabledSpec, enabledResult)
			callHierarchyLayer1AssertPairResultsEqualForTest(t, pair.Disabled.Name, pair.Enabled.Name, disabledResult, enabledResult)
			callHierarchyLayer1AssertGapForTest(t, pair.Disabled.Name, false, disabledResult)
			callHierarchyLayer1AssertGapForTest(t, pair.Enabled.Name, true, enabledResult)
		})
	}
}

func TestCallHierarchyLayer1UnexpectedPoisoningRejectedForOrdinarySuccess(t *testing.T) {
	for _, mode := range []string{"references-empty", "references-null", "references-repeated"} {
		spec := callHierarchyLayer1OrdinarySpecForTest{mode: mode, errorClass: "none", params: json.RawMessage(`{"item":{}}`), deadline: time.Second, maxMessages: 3, maxBytes: 4096}
		if callHierarchyLayer1PoisonPermittedForTest(spec) {
			t.Fatalf("ASSERT_CH_CURRENT_UNEXPECTED_POISONING_ORDINARY_SUCCESS mode=%q", mode)
		}
	}
	for _, errorClass := range []string{"protocol", "malformed", "shutdown", "transport-unavailable"} {
		spec := callHierarchyLayer1OrdinarySpecForTest{mode: "references-empty", errorClass: errorClass, deadline: time.Second, maxMessages: 3, maxBytes: 4096}
		if !callHierarchyLayer1PoisonPermittedForTest(spec) {
			t.Fatalf("ASSERT_CH_CURRENT_EXPECTED_POISONING_CONDITION errorClass=%q", errorClass)
		}
	}
	if !callHierarchyLayer1PoisonPermittedForTest(callHierarchyLayer1OrdinarySpecForTest{mode: "hang", errorClass: "deadline", deadline: 30 * time.Millisecond}) ||
		!callHierarchyLayer1PoisonPermittedForTest(callHierarchyLayer1OrdinarySpecForTest{mode: "references-repeated", errorClass: "none", params: json.RawMessage("fixed-payload"), deadline: time.Second}) ||
		callHierarchyLayer1PoisonPermittedForTest(callHierarchyLayer1OrdinarySpecForTest{mode: "hang", errorClass: "cancellation", cancel: true, deadline: time.Second}) {
		t.Fatal("ASSERT_CH_CURRENT_POISONING_DEADLINE_CANCELLATION_BOUNDARY")
	}
}

func TestCallHierarchyLayer1ObservationDoesNotControlOrdinaryRoundTrip(t *testing.T) {
	worlds := callHierarchyLayer1LoadCorpusForTest(t)
	pair := worlds[24].world.Record.Comparison.Pairs[0]
	spec := callHierarchyLayer1OrdinarySpecFromCorpusForTest(t, pair.Enabled)
	attachment := callHierarchyLayer1RequestAttachment{
		selected: callHierarchyLayer1CommitmentForCorpusRecord(t, pair.Enabled.Name+"-selected", pair.Enabled.Commitments.Selected),
		handoff:  callHierarchyLayer1CommitmentForCorpusRecord(t, pair.Enabled.Name+"-handoff", pair.Enabled.Commitments.Handoff),
	}
	disabled, _ := callHierarchyLayer1ExecuteSpecForTest(t, pair.Disabled.Name, spec, callHierarchyLayer1RequestAttachment{})
	enabled, _ := callHierarchyLayer1ExecuteSpecForTest(t, pair.Enabled.Name, spec, attachment)
	callHierarchyLayer1AssertPairResultsEqualForTest(t, pair.Disabled.Name, pair.Enabled.Name, disabled, enabled)
	callHierarchyLayer1AssertGapForTest(t, pair.Disabled.Name, false, disabled)
	callHierarchyLayer1AssertGapForTest(t, pair.Enabled.Name, true, enabled)
}

func TestCallHierarchyLayer1MutationPayloadAndPropagationRED(t *testing.T) {
	var gaps []string
	collect := func(name string, result RoundTripResult) {
		if lease, ok := result.privateCallHierarchyObservationLease(); ok {
			lease.release()
			t.Fatalf("ASSERT_CH_LAYER1_EXPECTED_PROPAGATION_GAP_DISAPPEARED name=%s", name)
		}
		if !reflect.DeepEqual(result.callHierarchyLayer1, callHierarchyLayer1ResultAttachment{}) {
			t.Fatalf("ASSERT_CH_LAYER1_PROPAGATION_GAP_CLASSIFICATION_CHANGED name=%s attachment=%+v", name, result.callHierarchyLayer1)
		}
		gaps = append(gaps, name+"=proposal-surface-unavailable")
	}
	for _, payload := range []struct {
		name string
		data []byte
	}{{"nil", nil}, {"empty", []byte{}}, {"nonempty", []byte(`{"item":{}}`)}} {
		t.Run("payload-"+payload.name, func(t *testing.T) {
			manager, started, _ := roundTripManager(t, "references-empty")
			original := append([]byte(nil), payload.data...)
			request := callHierarchyLayer1RequestForTest(started, "callHierarchy/incomingCalls", payload.data, []byte("selected"), []byte("handoff"))
			if len(payload.data) != 0 {
				payload.data[0] ^= 1
			}
			result := manager.RoundTrip(context.Background(), request)
			if !bytes.Equal(request.Params, original) || (original == nil) != (request.Params == nil) {
				t.Errorf("ASSERT_CH_LAYER1_PAYLOAD_TRI_STATE_COPY_%s got=%q want=%q nil=%v/%v", payload.name, request.Params, original, request.Params == nil, original == nil)
			}
			collect("payload-"+payload.name, result)
			external := callHierarchyLayer1ExternalForTest(t, result)
			retained := append([]byte(nil), external.result...)
			if len(result.Result) != 0 {
				result.Result[0] ^= 1
			}
			if !reflect.DeepEqual(external.result, retained) {
				t.Errorf("ASSERT_CH_LAYER1_MUTATION_AFTER_NO_ALIAS_%s", payload.name)
			}
		})
	}
	manager, started, _ := roundTripManager(t, "references-empty")
	prepare := callHierarchyLayer1RequestForTest(started, "callHierarchy/prepare", []byte(`{"item":{}}`), []byte("prepare-selected"), []byte("prepare-handoff"))
	outgoing := callHierarchyLayer1RequestForTest(started, "callHierarchy/outgoingCalls", []byte(`{"item":{}}`), []byte("outgoing-selected"), []byte("outgoing-handoff"))
	prepareResult := manager.RoundTrip(context.Background(), prepare)
	outgoingResult := manager.RoundTrip(context.Background(), outgoing)
	if prepare.callHierarchyLayer1.selected == outgoing.callHierarchyLayer1.selected || prepare.callHierarchyLayer1.handoff == outgoing.callHierarchyLayer1.handoff {
		t.Error("ASSERT_CH_LAYER1_PREPARE_OUTGOING_DISTINCT_OPAQUE_COMMITMENTS")
	}
	collect("prepare", prepareResult)
	collect("outgoing", outgoingResult)
	if len(gaps) != 5 {
		t.Fatalf("ASSERT_CH_LAYER1_EXPECTED_PROPAGATION_GAP_COUNT got=%d want=5 gaps=%v", len(gaps), gaps)
	}
}

/* Superseded hand-authored C17 expectations retained in the uncommitted history only.
func TestCallHierarchyLayer1C15V2ControlAndReflectionExclusion(t *testing.T) {
	const immutableC15V2Expected = 3847
	if privateB4C15V2FinalCount != immutableC15V2Expected {
		t.Errorf("ASSERT_CH_LAYER1_C15_V2_CONTROL_3847: got=%d want=%d", privateB4C15V2FinalCount, immutableC15V2Expected)
	}
	req := callHierarchyLayer1RequestForTest(StartResult{}, "callHierarchy/incomingCalls", nil, []byte("selected"), []byte("handoff"))
	callHierarchyLayer1AssertOpaqueRequestForTest(t, req, []byte("selected"), []byte("handoff"))
}

func TestCallHierarchyLayer1TypedAttachmentShape(t *testing.T) {
	request := RoundTripRequest{}
	request.callHierarchyLayer1.selected.identity = "selected"
	request.callHierarchyLayer1.selected.sha256 = [32]byte{1}
	request.callHierarchyLayer1.selected.length = 7
	request.callHierarchyLayer1.selected.available = true
	request.callHierarchyLayer1.handoff.identity = "handoff"
	request.callHierarchyLayer1.ledger.transportMessages.after = callHierarchyMeasure{available: true}
	if request.callHierarchyLayer1.selected.identity != "selected" || request.callHierarchyLayer1.selected.length != 7 || !request.callHierarchyLayer1.selected.available {
		t.Error("layer 1 request attachment fields are unavailable")
	}
	if request.callHierarchyLayer1.ledger.transportMessages.before.available || !request.callHierarchyLayer1.ledger.transportMessages.after.available {
		t.Error("layer 1 ledger aliases unavailable and known zero")
	}

	result := RoundTripResult{}
	result.callHierarchyLayer1.snapshot.resultAvailable = true
	result.callHierarchyLayer1.snapshot.resultBytes = []byte{}
	result.callHierarchyLayer1.snapshot.error.presentAvailable = true
	result.callHierarchyLayer1.snapshot.error.codeAvailable = true
	result.callHierarchyLayer1.snapshot.externalAvailable = true
	if !result.callHierarchyLayer1.snapshot.resultAvailable || result.callHierarchyLayer1.snapshot.resultBytes == nil || !result.callHierarchyLayer1.snapshot.error.presentAvailable || !result.callHierarchyLayer1.snapshot.error.codeAvailable || !result.callHierarchyLayer1.snapshot.externalAvailable {
		t.Error("layer 1 result attachment fields are unavailable")
	}
}

type callHierarchyLayer1ExternalSnapshotForTest struct {
	result          []byte
	serverError     []byte
	failure         session.Failure
	messages        int
	bytes           int64
	requestMessages int
	requestBytes    int64
	notifications   []byte
	responses       []byte
}

func callHierarchyLayer1ExternalForTest(t *testing.T, result RoundTripResult) callHierarchyLayer1ExternalSnapshotForTest {
	t.Helper()
	serverError, err := json.Marshal(result.ServerError)
	if err != nil {
		t.Errorf("ASSERT_CH_LAYER1_SERVER_ERROR_SNAPSHOT: %v", err)
	}
	notifications, err := json.Marshal(result.Notifications)
	if err != nil {
		t.Errorf("ASSERT_CH_LAYER1_NOTIFICATIONS_SNAPSHOT: %v", err)
	}
	responses, err := json.Marshal(result.Responses)
	if err != nil {
		t.Errorf("ASSERT_CH_LAYER1_RESPONSES_SNAPSHOT: %v", err)
	}
	return callHierarchyLayer1ExternalSnapshotForTest{
		result: append([]byte(nil), result.Result...), serverError: serverError,
		failure: result.Failure, messages: result.Messages, bytes: result.Bytes,
		requestMessages: result.RequestMessages, requestBytes: result.RequestBytes,
		notifications: notifications, responses: responses,
	}
}

func callHierarchyLayer1RequestForTest(started StartResult, method string, params []byte, selected, handoff []byte) RoundTripRequest {
	return RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: method,
		Params: append(json.RawMessage(nil), params...), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096,
		callHierarchyLayer1: callHierarchyLayer1RequestAttachment{
			selected: callHierarchyOpaqueCommitmentMetadata{identity: "opaque-selected", sha256: sha256.Sum256(selected), length: uint64(len(selected)), available: true},
			handoff:  callHierarchyOpaqueCommitmentMetadata{identity: "opaque-handoff", sha256: sha256.Sum256(handoff), length: uint64(len(handoff)), available: true},
		},
	}
}

func callHierarchyLayer1MeasureEqualForTest(a, b callHierarchyMeasure) bool {
	return a.available == b.available && (!a.available || a.value == b.value)
}

func callHierarchyLayer1DeltaExactForTest(delta callHierarchyMeasuredDelta, want uint64) bool {
	return delta.before.available && delta.after.available && delta.after.value == delta.before.value+want
}

func callHierarchyLayer1AssertOpaqueRequestForTest(t *testing.T, request RoundTripRequest, selected, handoff []byte) {
	t.Helper()
	if request.callHierarchyLayer1.selected != (callHierarchyOpaqueCommitmentMetadata{identity: "opaque-selected", sha256: sha256.Sum256(selected), length: uint64(len(selected)), available: true}) ||
		request.callHierarchyLayer1.handoff != (callHierarchyOpaqueCommitmentMetadata{identity: "opaque-handoff", sha256: sha256.Sum256(handoff), length: uint64(len(handoff)), available: true}) {
		t.Error("ASSERT_CH_LAYER1_REQUEST_OPAQUE_IDENTITY_DIGEST_LENGTH")
	}
	metadata := reflect.TypeOf(callHierarchyOpaqueCommitmentMetadata{})
	wantFields := []struct {
		name   string
		typeOf reflect.Type
	}{{"identity", reflect.TypeOf("")}, {"sha256", reflect.TypeOf([32]byte{})}, {"length", reflect.TypeOf(uint64(0))}, {"available", reflect.TypeOf(false)}}
	if metadata.NumField() != len(wantFields) {
		t.Errorf("ASSERT_CH_LAYER1_REQUEST_NO_DIRECTIONAL_SEMANTIC_RAW_FIELDS: fields=%d", metadata.NumField())
	}
	for i, want := range wantFields {
		field := metadata.Field(i)
		if field.Name != want.name || field.Type != want.typeOf {
			t.Errorf("ASSERT_CH_LAYER1_REQUEST_ONLY_OPAQUE_SHA256_LENGTH_AVAILABILITY: field[%d]=%s/%v", i, field.Name, field.Type)
		}
	}
}

func callHierarchyLayer1AssertResultForTest(t *testing.T, name string, got callHierarchyLayer1ResultAttachment, external callHierarchyLayer1ExternalSnapshotForTest) {
	t.Helper()
	if !got.snapshot.resultAvailable || !reflect.DeepEqual(got.snapshot.resultBytes, external.result) {
		t.Errorf("ASSERT_CH_LAYER1_RESULT_BYTES_%s: available=%v got=%q want=%q", name, got.snapshot.resultAvailable, got.snapshot.resultBytes, external.result)
	}
	if !got.snapshot.errorAvailable || !got.snapshot.error.presentAvailable || got.snapshot.error.present != (string(external.serverError) != "null") ||
		!got.snapshot.error.serializedAvailable || !reflect.DeepEqual(got.snapshot.error.serializedBytes, external.serverError) {
		t.Errorf("ASSERT_CH_LAYER1_ERROR_EXACT_%s: snapshot=%+v external=%s", name, got.snapshot.error, external.serverError)
	}
	if !got.snapshot.terminal.available || !got.snapshot.transportMessages.available || !got.snapshot.transportBytes.available || !got.snapshot.externalAvailable {
		t.Errorf("ASSERT_CH_LAYER1_EXTERNAL_AVAILABILITY_%s: snapshot=%+v", name, got.snapshot)
	}
	if got.snapshot.transportMessages.value != uint64(external.messages) || got.snapshot.transportBytes.value != uint64(external.bytes) {
		t.Errorf("ASSERT_CH_LAYER1_TRANSPORT_ACCOUNTING_%s: snapshot=%+v external=%+v", name, got.snapshot, external)
	}
	if !callHierarchyLayer1DeltaExactForTest(got.ledger.transportMessages, uint64(external.messages)) ||
		!callHierarchyLayer1DeltaExactForTest(got.ledger.transportBytes, uint64(external.bytes)) ||
		!callHierarchyLayer1DeltaExactForTest(got.ledger.externalBytes, uint64(len(got.snapshot.externalBytes))) ||
		!callHierarchyLayer1DeltaExactForTest(got.ledger.reservations, 0) ||
		!callHierarchyLayer1DeltaExactForTest(got.ledger.leases, 0) ||
		!callHierarchyLayer1DeltaExactForTest(got.ledger.callbacks, 0) ||
		!callHierarchyLayer1DeltaExactForTest(got.ledger.diagnostics, 0) {
		t.Errorf("ASSERT_CH_LAYER1_COMPLETE_EXACT_LEDGER_%s: ledger=%+v", name, got.ledger)
	}
	if !got.ledger.terminalBefore.available || !got.ledger.terminalAfter.available {
		t.Errorf("ASSERT_CH_LAYER1_TERMINAL_LEDGER_%s: ledger=%+v", name, got.ledger)
	}
}

func TestCallHierarchyLayer1ManagerSharedCausalRED(t *testing.T) {
	params := []byte(`{"item":{"name":"Target","kind":12,"uri":"file:///fixture/main.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":6}},"selectionRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":6}}}}`)

	worlds := []struct {
		name, mode, method string
		params             []byte
		deadline           time.Duration
		cancel             bool
		shutdown           bool
	}{
		{name: "incoming-success-empty-array", mode: "references-empty", method: "callHierarchy/incomingCalls", params: params},
		{name: "outgoing-success-empty-array", mode: "references-empty", method: "callHierarchy/outgoingCalls", params: params},
		{name: "outgoing-success-top-level-null", mode: "references-null", method: "callHierarchy/outgoingCalls", params: params},
		{name: "incoming-success-nonempty", mode: "references-repeated", method: "callHierarchy/incomingCalls", params: params},
		{name: "outgoing-success-nonempty", mode: "references-repeated", method: "callHierarchy/outgoingCalls", params: params},
		{name: "protocol-error", mode: "server-error", method: "callHierarchy/incomingCalls", params: params},
		{name: "malformed-error-response", mode: "malformed", method: "callHierarchy/outgoingCalls", params: params},
		{name: "cancellation", mode: "hang", method: "callHierarchy/incomingCalls", params: params, cancel: true},
		{name: "deadline", mode: "hang", method: "callHierarchy/outgoingCalls", params: params, deadline: 30 * time.Millisecond},
		{name: "shutdown", mode: "references-empty", method: "callHierarchy/incomingCalls", params: params, shutdown: true},
		{name: "transport-unavailable", mode: "references-empty", method: "callHierarchy/outgoingCalls", params: params, shutdown: true},
	}
	for _, world := range worlds {
		t.Run(world.name, func(t *testing.T) {
			run := func(enabled bool) (RoundTripResult, callHierarchyLayer1ExternalSnapshotForTest) {
				manager, sessionStarted, _ := roundTripManager(t, world.mode)
				if world.shutdown {
					if err := manager.Shutdown(context.Background()); err != nil {
						t.Errorf("ASSERT_CH_API_GAP_%s path=manager.shutdown expected=success got=%v", world.name, err)
					}
				}
				ctx := context.Background()
				if world.cancel {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				req := callHierarchyLayer1RequestForTest(sessionStarted, world.method, world.params, []byte("selected"), []byte("handoff"))
				if world.deadline != 0 {
					req.Deadline = time.Now().Add(world.deadline)
				}
				if !enabled {
					req.callHierarchyLayer1 = callHierarchyLayer1RequestAttachment{}
				}
				result := manager.RoundTrip(ctx, req)
				return result, callHierarchyLayer1ExternalForTest(t, result)
			}
			disabledResult, disabled := run(false)
			enabledResult, enabled := run(true)
			if !reflect.DeepEqual(enabled, disabled) {
				t.Errorf("ASSERT_CH_LAYER1_ENABLED_DISABLED_EXTERNAL_EQUAL_%s: enabled=%+v disabled=%+v", world.name, enabled, disabled)
			}
			if !enabledResult.callHierarchyLayer1.snapshot.resultAvailable {
				t.Errorf("ASSERT_CH_LAYER1_PROPAGATION_%s: Manager did not publish the world attachment", world.name)
			}
			callHierarchyLayer1AssertResultForTest(t, world.name, enabledResult.callHierarchyLayer1, enabled)
			if !callHierarchyLayer1MeasureEqualForTest(enabledResult.callHierarchyLayer1.snapshot.transportMessages, callHierarchyMeasure{available: true, value: uint64(enabledResult.Messages)}) {
				t.Errorf("ASSERT_CH_LAYER1_TRANSPORT_EFFECT_EQUAL_%s", world.name)
			}
			if !reflect.DeepEqual(disabledResult.callHierarchyLayer1, callHierarchyLayer1ResultAttachment{}) {
				t.Errorf("ASSERT_CH_LAYER1_DISABLED_ZERO_ATTACHMENT_%s: %+v", world.name, disabledResult.callHierarchyLayer1)
			}
		})
	}

	for _, payload := range []struct {
		name string
		data []byte
	}{
		{name: "nil", data: nil},
		{name: "empty", data: []byte{}},
		{name: "nonempty", data: []byte(`{"item":{}}`)},
	} {
		t.Run("payload-"+payload.name, func(t *testing.T) {
			manager, sessionStarted, _ := roundTripManager(t, "references-empty")
			original := append([]byte(nil), payload.data...)
			req := callHierarchyLayer1RequestForTest(sessionStarted, "callHierarchy/incomingCalls", payload.data, []byte("selected"), []byte("handoff"))
			if payload.data != nil && len(payload.data) != 0 {
				payload.data[0] ^= 1
			}
			result := manager.RoundTrip(context.Background(), req)
			if !result.callHierarchyLayer1.snapshot.resultAvailable {
				t.Errorf("ASSERT_CH_LAYER1_PROPAGATION_PAYLOAD_%s", payload.name)
			}
			if !reflect.DeepEqual(req.Params, original) || (original == nil) != (req.Params == nil) {
				t.Errorf("ASSERT_CH_LAYER1_REQUEST_PAYLOAD_COPY_%s: got=%q want=%q", payload.name, req.Params, original)
			}
			before := append([]byte(nil), result.callHierarchyLayer1.snapshot.resultBytes...)
			if len(result.Result) != 0 {
				result.Result[0] ^= 1
			}
			if !reflect.DeepEqual(result.callHierarchyLayer1.snapshot.resultBytes, before) {
				t.Errorf("ASSERT_CH_LAYER1_RESULT_DEFENSIVE_COPY_%s", payload.name)
			}
		})
	}

	for _, refusal := range []struct {
		name   string
		mutate func(*RoundTripRequest)
	}{
		{name: "selected-unavailable", mutate: func(r *RoundTripRequest) { r.callHierarchyLayer1.selected = callHierarchyOpaqueCommitmentMetadata{} }},
		{name: "handoff-unavailable", mutate: func(r *RoundTripRequest) { r.callHierarchyLayer1.handoff = callHierarchyOpaqueCommitmentMetadata{} }},
		{name: "selected-zero-identity", mutate: func(r *RoundTripRequest) { r.callHierarchyLayer1.selected.identity = "" }},
		{name: "handoff-zero-identity", mutate: func(r *RoundTripRequest) { r.callHierarchyLayer1.handoff.identity = "" }},
		{name: "selected-zero-digest", mutate: func(r *RoundTripRequest) { r.callHierarchyLayer1.selected.sha256 = [32]byte{} }},
		{name: "handoff-zero-digest", mutate: func(r *RoundTripRequest) { r.callHierarchyLayer1.handoff.sha256 = [32]byte{} }},
		{name: "method-unavailable", mutate: func(r *RoundTripRequest) { r.Method = "" }},
	} {
		t.Run("refusal-"+refusal.name, func(t *testing.T) {
			manager, sessionStarted, _ := roundTripManager(t, "references-empty")
			req := callHierarchyLayer1RequestForTest(sessionStarted, "callHierarchy/incomingCalls", params, []byte("selected"), []byte("handoff"))
			refusal.mutate(&req)
			result := manager.RoundTrip(context.Background(), req)
			ledger := result.callHierarchyLayer1.ledger
			if !ledger.transportMessages.before.available {
				t.Errorf("ASSERT_CH_LAYER1_REFUSAL_PROPAGATION_%s", refusal.name)
			}
			if !ledger.transportMessages.before.available || !ledger.transportMessages.after.available ||
				!ledger.transportBytes.before.available || !ledger.transportBytes.after.available ||
				!ledger.reservations.before.available || !ledger.reservations.after.available ||
				!ledger.leases.before.available || !ledger.leases.after.available ||
				!ledger.callbacks.before.available || !ledger.callbacks.after.available ||
				!ledger.diagnostics.before.available || !ledger.diagnostics.after.available {
				t.Errorf("ASSERT_CH_LAYER1_REFUSAL_CROSSES_MANAGER_%s: ledger=%+v result=%+v", refusal.name, ledger, result)
			}
			if ledger.reservations.before.value != ledger.reservations.after.value || ledger.leases.before.value != ledger.leases.after.value || ledger.callbacks.before.value != ledger.callbacks.after.value {
				t.Errorf("ASSERT_CH_LAYER1_REFUSAL_ZERO_LEAK_%s: ledger=%+v", refusal.name, ledger)
			}
		})
	}

	t.Run("mutation-defensive-copies-before-and-after", func(t *testing.T) {
		manager, sessionStarted, _ := roundTripManager(t, "references-repeated")
		requestPayload := append([]byte(nil), params...)
		selectedPayload := []byte("selected-before")
		handoffPayload := []byte("handoff-before")
		req := callHierarchyLayer1RequestForTest(sessionStarted, "callHierarchy/incomingCalls", requestPayload, selectedPayload, handoffPayload)
		wantRequest := append([]byte(nil), req.Params...)
		wantSelected := req.callHierarchyLayer1.selected
		wantHandoff := req.callHierarchyLayer1.handoff
		requestPayload[0] ^= 1
		selectedPayload[0] ^= 1
		handoffPayload[0] ^= 1
		result := manager.RoundTrip(context.Background(), req)
		if !result.callHierarchyLayer1.snapshot.resultAvailable {
			t.Error("ASSERT_CH_LAYER1_PROPAGATION_MUTATION_DEFENSIVE_COPIES")
		}
		if !reflect.DeepEqual(req.Params, wantRequest) || req.callHierarchyLayer1.selected != wantSelected || req.callHierarchyLayer1.handoff != wantHandoff {
			t.Error("ASSERT_CH_LAYER1_MUTATION_BEFORE_ROUNDTRIP_DEFENSIVE_COPY")
		}
		retained := append([]byte(nil), result.callHierarchyLayer1.snapshot.resultBytes...)
		if len(result.Result) != 0 {
			result.Result[0] ^= 1
		}
		if !reflect.DeepEqual(result.callHierarchyLayer1.snapshot.resultBytes, retained) {
			t.Error("ASSERT_CH_LAYER1_MUTATION_AFTER_ROUNDTRIP_DEFENSIVE_COPY")
		}
	})

	t.Run("prepare-outgoing-separation", func(t *testing.T) {
		manager, sessionStarted, _ := roundTripManager(t, "references-empty")
		prepare := callHierarchyLayer1RequestForTest(sessionStarted, "callHierarchy/prepare", params, []byte("prepare-selected"), []byte("prepare-handoff"))
		prepareResult := manager.RoundTrip(context.Background(), prepare)
		outgoing := callHierarchyLayer1RequestForTest(sessionStarted, "callHierarchy/outgoingCalls", params, []byte("outgoing-selected"), []byte("outgoing-handoff"))
		outgoingResult := manager.RoundTrip(context.Background(), outgoing)
		if !prepareResult.callHierarchyLayer1.snapshot.resultAvailable || !outgoingResult.callHierarchyLayer1.snapshot.resultAvailable {
			t.Error("ASSERT_CH_LAYER1_PROPAGATION_PREPARE_OUTGOING_SEPARATION")
		}
		if prepare.callHierarchyLayer1.selected == outgoing.callHierarchyLayer1.selected || prepare.callHierarchyLayer1.handoff == outgoing.callHierarchyLayer1.handoff {
			t.Error("ASSERT_CH_LAYER1_PREPARE_OUTGOING_DISTINCT_COMMITMENTS")
		}
	})

	t.Run("manager-produced-concurrent-lifecycle", func(t *testing.T) {
		manager, sessionStarted, _ := roundTripManager(t, "references-empty")
		const workers = 16
		var wg sync.WaitGroup
		results := make(chan RoundTripResult, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := callHierarchyLayer1RequestForTest(sessionStarted, "callHierarchy/outgoingCalls", params, []byte("selected"), []byte("handoff"))
				results <- manager.RoundTrip(context.Background(), req)
			}()
		}
		wg.Wait()
		close(results)
		index := 0
		for result := range results {
			if !result.callHierarchyLayer1.snapshot.resultAvailable {
				t.Errorf("ASSERT_CH_LAYER1_PROPAGATION_CONCURRENT_LIFECYCLE_%02d", index)
				index++
				continue
			}
			if result.callHierarchyLayer1.ledger.leases.before.value != result.callHierarchyLayer1.ledger.leases.after.value || result.callHierarchyLayer1.ledger.callbacks.before.value != result.callHierarchyLayer1.ledger.callbacks.after.value {
				t.Errorf("ASSERT_CH_LAYER1_CONCURRENT_LIFECYCLE_NO_LEAK_%02d: result=%+v attachment=%+v", index, result, result.callHierarchyLayer1)
			}
			index++
		}
	})
}

*/

func TestCallHierarchyObservationScaffoldZeroLeaseAndAttachments(t *testing.T) {
	lease, ok := (RoundTripResult{}).privateCallHierarchyObservationLease()
	if ok || lease != (callHierarchyObservationLease{}) {
		t.Fatal("zero result exposed a scaffold lease")
	}
	called := false
	if err := lease.withObservation(func(callHierarchyObservationView) error {
		called = true
		return nil
	}); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) || called {
		t.Fatal("zero lease admitted an observation callback")
	}
	if lease.release() {
		t.Fatal("zero lease released an owner")
	}

	requestField, ok := reflect.TypeOf(RoundTripRequest{}).FieldByName("callHierarchyObservation")
	if !ok || requestField.Tag.Get("json") != "-" {
		t.Fatal("request scaffold attachment is not private JSON-ignored state")
	}
	resultField, ok := reflect.TypeOf(RoundTripResult{}).FieldByName("callHierarchyObservation")
	if !ok || resultField.Tag.Get("json") != "-" {
		t.Fatal("result scaffold attachment is not private JSON-ignored state")
	}
}

func callHierarchyObservationOwnerFixture() callHierarchyObservation {
	return callHierarchyObservation{
		predecessors: callHierarchyPredecessorChain{commitments: []callHierarchyPredecessorCommitment{{identity: "predecessor"}}},
		request:      &callHierarchyOriginalRequestFrameObservation{frameBytes: []byte("request")},
		response:     &callHierarchyOriginalResponseFrameObservation{frameBytes: []byte("response")},
		selected:     &callHierarchySelectedItemCommitment{exactBytes: []byte("selected")},
		handoff:      &callHierarchyHandoffCommitment{selected: callHierarchySelectedItemCommitment{exactBytes: []byte("handoff-selected")}, successorBytes: []byte("successor")},
	}
}

func TestCallHierarchyObservationOwnerCopiesAndContainsCallbacks(t *testing.T) {
	source := callHierarchyObservationOwnerFixture()
	owner := newCallHierarchyObservationOwner(source)
	result := RoundTripResult{callHierarchyObservation: owner}
	lease, ok := result.privateCallHierarchyObservationLease()
	if !ok || lease.owner != owner {
		t.Fatal("ASSERT_CH_OWNER_TYPED_RESULT_LEASE")
	}
	source.request.frameBytes[0] = 'x'
	callbackFailure := errors.New("callback failure")
	if err := lease.withObservation(func(view callHierarchyObservationView) error {
		if string(view.request.frameBytes) != "request" || string(view.response.frameBytes) != "response" || len(view.predecessors) != 1 {
			return errors.New("wrong copied view")
		}
		view.request.frameBytes[0] = 'y'
		view.predecessors[0].identity = "mutated"
		return callbackFailure
	}); !errors.Is(err, callbackFailure) {
		t.Fatalf("ASSERT_CH_OWNER_CALLBACK_FAILURE_CONTAINED: %v", err)
	}
	if err := lease.withObservation(func(view callHierarchyObservationView) error {
		if string(view.request.frameBytes) != "request" || view.predecessors[0].identity != "predecessor" {
			return errors.New("callback view aliased owner")
		}
		panic("contained")
	}); err == nil {
		t.Fatal("ASSERT_CH_OWNER_CALLBACK_PANIC_CONTAINED")
	}
	if err := lease.withObservation(func(callHierarchyObservationView) error { return nil }); err != nil {
		t.Fatalf("ASSERT_CH_OWNER_LIVE_AFTER_CALLBACK_FAILURE: %v", err)
	}
	if !lease.release() || lease.release() {
		t.Fatal("ASSERT_CH_OWNER_EXACT_ONCE_RELEASE")
	}
	if _, ok := result.privateCallHierarchyObservationLease(); ok {
		t.Fatal("ASSERT_CH_OWNER_RELEASED_RESULT_LEASE")
	}
}

func TestCallHierarchyObservationOwnerTerminalDrainsCallback(t *testing.T) {
	owner := newCallHierarchyObservationOwner(callHierarchyObservationOwnerFixture())
	lease := callHierarchyObservationLease{owner: owner}
	entered := make(chan struct{})
	leave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.withObservation(func(callHierarchyObservationView) error {
			close(entered)
			<-leave
			return nil
		})
	}()
	<-entered
	owner.requestTerminal()
	if err := lease.withObservation(func(callHierarchyObservationView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatalf("ASSERT_CH_OWNER_TERMINAL_ADMISSION: %v", err)
	}
	released := make(chan bool, 1)
	go func() { released <- lease.release() }()
	select {
	case <-released:
		t.Fatal("ASSERT_CH_OWNER_RELEASE_BEFORE_CALLBACK_DRAIN")
	default:
	}
	close(leave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-released {
		t.Fatal("ASSERT_CH_OWNER_TERMINAL_RELEASE")
	}
}

func TestCallHierarchyObservationOwnerReleaseRaceOneWinner(t *testing.T) {
	lease := callHierarchyObservationLease{owner: newCallHierarchyObservationOwner(callHierarchyObservationOwnerFixture())}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(copyLease callHierarchyObservationLease) {
			defer wg.Done()
			if copyLease.release() {
				winners.Add(1)
			}
		}(lease)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("ASSERT_CH_OWNER_RELEASE_WINNERS: %d", winners.Load())
	}
}

func TestCallHierarchyObservationAttachmentIgnoredByOrdinaryRoundTrip(t *testing.T) {
	m, started, _ := roundTripManager(t, "references-empty")
	result := m.RoundTrip(context.Background(), RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation,
		Method: "textDocument/references", Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0},"context":{"includeDeclaration":false}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096,
		callHierarchyObservation: &callHierarchyObservationRequest{},
	})
	if result.Failure != "" {
		t.Fatalf("ASSERT_CH_OWNER_ORDINARY_ROUND_TRIP: %v", result.Failure)
	}
	if lease, ok := result.privateCallHierarchyObservationLease(); ok || lease != (callHierarchyObservationLease{}) {
		t.Fatal("ASSERT_CH_OWNER_ORDINARY_ROUND_TRIP_IGNORES_ATTACHMENT")
	}
}

/* Superseded hand-authored C17 semantic expectations; accepted corpus above is the sole oracle.
func TestCallHierarchyManagerRoundTripPublishesSemanticObservationLease(t *testing.T) {
	incoming := []callHierarchyMethodIdentity{"textDocument/documentSymbol", "callHierarchy/prepare", "callHierarchy/incomingCalls"}
	outgoing := []callHierarchyMethodIdentity{"textDocument/documentSymbol", "callHierarchy/prepare", "callHierarchy/outgoingCalls"}
	for _, tc := range []struct {
		name    string
		method  string
		binding callHierarchyBindingIdentity
		methods []callHierarchyMethodIdentity
	}{
		{name: "incoming", method: "callHierarchy/incomingCalls", binding: "opaque-incoming", methods: incoming},
		{name: "outgoing", method: "callHierarchy/outgoingCalls", binding: "opaque-outgoing", methods: outgoing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, "references-empty")
			params := json.RawMessage(`{"item":{"name":"Target","kind":12,"uri":"file:///fixture/main.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":6}},"selectionRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":6}}}}`)
			predecessorIdentity := callHierarchyRequestIdentity{session: callHierarchySessionIdentity(started.SessionID), generation: callHierarchyGeneration(started.Generation), requestKey: callHierarchyJSONRPCRequestKey{kind: callHierarchyJSONRPCRequestKeyString, stringValue: "document-symbol-1"}}
			predecessorTransaction := callHierarchyTransactionIdentity{request: predecessorIdentity, local: [32]byte{0x31}}
			predecessor := callHierarchyPredecessorCommitment{identity: "document-symbol", transaction: predecessorTransaction, requestIdentity: predecessorIdentity, requestKey: predecessorIdentity.requestKey, method: "textDocument/documentSymbol", requestOrdinal: 1, commitmentSHA256: callHierarchyDigest(t, "0c213d284fe5eb086a1e37a1d7955bb658e83c1d97aa6f87cb4ea54ad9233d32")}
			selectedBytes := []byte(`{"name":"Target","kind":12,"uri":"file:///fixture/main.go"}`)
			selected := callHierarchySelectedItemCommitment{ordinal: 2, sourceRequest: predecessorIdentity, sourceFrame: predecessorTransaction, sourceOffset: 17, sourceLength: uint64(len(selectedBytes)), exactBytes: append([]byte(nil), selectedBytes...), exactSHA256: callHierarchyDigest(t, "d6ac6a7d5a4839f9cf7f1f7f7eecb17aeea5333b1fea3c6dc767380bc2d11a66")}
			handoffBytes := []byte(`{"item":{"name":"Target"}}`)
			handoff := callHierarchyHandoffCommitment{selected: selected, successorRequest: predecessorIdentity, successor: callHierarchyTransactionIdentity{request: predecessorIdentity, local: [32]byte{0x32}}, successorOffset: 23, successorLength: uint64(len(handoffBytes)), successorBytes: append([]byte(nil), handoffBytes...), successorSHA256: callHierarchyDigest(t, "1b1cbcaae5e63c4e18f3065a498ca96276b9f0a4c46adc59db0feebf32055531")}
			limits := callHierarchyObservationLimits{maxBytes: 4096, maxObjects: 3, maxWork: 8, maxDiagnostics: 2}
			attachment := &callHierarchyObservationRequest{
				identity:     callHierarchyObservationIdentity{profile: "go", binding: tc.binding, transaction: handoff.successor, method: callHierarchyMethodIdentity(tc.method)},
				allowlist:    callHierarchyMethodAllowlist{methods: append([]callHierarchyMethodIdentity(nil), tc.methods...)},
				predecessors: callHierarchyPredecessorChain{commitments: []callHierarchyPredecessorCommitment{predecessor}},
				selected:     &selected, handoff: &handoff, limits: limits,
			}
			result := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: tc.method, Params: params, Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, callHierarchyObservation: attachment})
			if result.Failure != "" || result.ServerError != nil || result.Key.ID == 0 {
				t.Fatalf("ASSERT_CH_MANAGER_ROUNDTRIP_EXTERNAL: result=%+v", result)
			}
			lease, ok := result.privateCallHierarchyObservationLease()
			if !ok {
				t.Errorf("ASSERT_CH_MANAGER_PROPAGATES_RETAINED_OWNER_%s: missing result observation lease", tc.name)
				t.Errorf("ASSERT_CH_MANAGER_CALLBACK_ERROR_TRANSPARENT_%s: observation unavailable", tc.name)
				t.Errorf("ASSERT_CH_MANAGER_CALLBACK_PANIC_TRANSPARENT_%s: observation unavailable", tc.name)
				t.Errorf("ASSERT_CH_MANAGER_RELEASE_EXACT_ONCE_%s: observation unavailable", tc.name)
			}
			callbackFailure := errors.New("callback-transparent")
			if err := lease.withObservation(func(view callHierarchyObservationView) error {
				if view.identity.request.workspace.managerNonce == ([32]byte{}) || view.identity.request.session != callHierarchySessionIdentity(started.SessionID) || view.identity.request.generation != callHierarchyGeneration(started.Generation) || view.identity.request.requestKey.kind == callHierarchyJSONRPCRequestKeyUnavailable {
					return errors.New("manager custody absent")
				}
				if view.identity.binding != tc.binding || view.identity.method != callHierarchyMethodIdentity(tc.method) || len(view.predecessors) != 1 || !reflect.DeepEqual(view.predecessors[0], predecessor) || view.selected == nil || view.selected.ordinal != 2 || view.handoff == nil || view.terminal != callHierarchyObservationTerminalSuccessfulEmpty {
					return errors.New("semantic commitments changed")
				}
				if view.request == nil || view.response == nil || !reflect.DeepEqual(view.request.requestIdentity, view.identity.request) || !reflect.DeepEqual(view.response.requestIdentity, view.identity.request) || view.request.frameLength != uint64(len(view.request.frameBytes)) || view.response.frameLength != uint64(len(view.response.frameBytes)) || view.request.frameSHA256 != sha256.Sum256(view.request.frameBytes) || view.response.frameSHA256 != sha256.Sum256(view.response.frameBytes) || view.response.resultForm != callHierarchyJSONTokenFormEmptyArray || string(view.response.copyResultBytes()) != "[]" {
					return errors.New("frame bytes spans digests or request identity changed")
				}
				if view.accounting.bytes != (callHierarchyMeasure{available: true, value: view.request.frameLength + view.response.frameLength}) || !view.accounting.objects.available || !view.accounting.work.available || !view.accounting.diagnostics.available {
					return errors.New("accounting dimensions unavailable")
				}
				view.predecessors[0].identity = "mutated"
				view.request.frameBytes[0] ^= 1
				view.selected.exactBytes[0] ^= 1
				return callbackFailure
			}); !errors.Is(err, callbackFailure) {
				t.Fatalf("ASSERT_CH_MANAGER_CALLBACK_ERROR_TRANSPARENT_%s: %v", tc.name, err)
			}
			if err := lease.withObservation(func(view callHierarchyObservationView) error {
				if !reflect.DeepEqual(view.predecessors[0], predecessor) || view.request.frameSHA256 != sha256.Sum256(view.request.frameBytes) || string(view.selected.exactBytes) != string(selectedBytes) {
					return errors.New("manager owner did not defensively copy callback attachments")
				}
				panic("callback-panic")
			}); err == nil {
				t.Fatalf("ASSERT_CH_MANAGER_CALLBACK_PANIC_TRANSPARENT_%s", tc.name)
			}
			if !lease.release() || lease.release() {
				t.Fatalf("ASSERT_CH_MANAGER_RELEASE_EXACT_ONCE_%s", tc.name)
			}
		})
	}

	validIncoming := []callHierarchyMethodIdentity{"textDocument/documentSymbol", "callHierarchy/prepare", "callHierarchy/incomingCalls"}
	for _, tc := range []struct {
		name       string
		method     string
		methods    []callHierarchyMethodIdentity
		session    callHierarchySessionIdentity
		generation callHierarchyGeneration
	}{
		{name: "arbitrary", method: "callHierarchy/incomingCalls", methods: []callHierarchyMethodIdentity{"arbitrary"}},
		{name: "mixed", method: "callHierarchy/incomingCalls", methods: []callHierarchyMethodIdentity{validIncoming[0], validIncoming[1], "callHierarchy/outgoingCalls"}},
		{name: "reordered", method: "callHierarchy/incomingCalls", methods: []callHierarchyMethodIdentity{validIncoming[1], validIncoming[0], validIncoming[2]}},
		{name: "duplicate", method: "callHierarchy/incomingCalls", methods: []callHierarchyMethodIdentity{validIncoming[0], validIncoming[1], validIncoming[1]}},
		{name: "stale", method: "callHierarchy/incomingCalls", methods: validIncoming, generation: 999},
		{name: "foreign", method: "callHierarchy/incomingCalls", methods: validIncoming, session: "foreign-session"},
		{name: "method-mismatch", method: "callHierarchy/outgoingCalls", methods: validIncoming},
	} {
		t.Run("refuses-"+tc.name, func(t *testing.T) {
			m, started, _ := roundTripManager(t, "references-empty")
			identity := callHierarchyRequestIdentity{session: callHierarchySessionIdentity(started.SessionID), generation: callHierarchyGeneration(started.Generation), requestKey: callHierarchyJSONRPCRequestKey{kind: callHierarchyJSONRPCRequestKeyString, stringValue: "request-1"}}
			if tc.session != "" {
				identity.session = tc.session
			}
			if tc.generation != 0 {
				identity.generation = tc.generation
			}
			attachment := &callHierarchyObservationRequest{identity: callHierarchyObservationIdentity{request: identity, binding: "opaque-incoming", method: callHierarchyMethodIdentity(tc.method)}, allowlist: callHierarchyMethodAllowlist{methods: append([]callHierarchyMethodIdentity(nil), tc.methods...)}, limits: callHierarchyObservationLimits{maxBytes: 4096, maxObjects: 3, maxWork: 8, maxDiagnostics: 2}}
			result := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: tc.method, Params: json.RawMessage(`{"item":{"name":"Target"}}`), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, callHierarchyObservation: attachment})
			if result.Failure != "" || result.ServerError != nil || result.Key.ID == 0 {
				t.Fatalf("ASSERT_CH_MANAGER_REFUSAL_EXTERNAL_%s: result=%+v", tc.name, result)
			}
			if lease, ok := result.privateCallHierarchyObservationLease(); ok || lease != (callHierarchyObservationLease{}) {
				t.Fatalf("ASSERT_CH_MANAGER_REFUSAL_ZERO_EFFECT_%s: lease=%+v ok=%v", tc.name, lease, ok)
			}
		})
	}
}

*/

func TestPrivateB4OwnedMethodPairAtomicBatchCausalFixture(t *testing.T) {
	runOwnedPairDeadline(t, func() {
		account := mustPrivateB4ByteAccountV2(t)
		type contextSnapshot struct {
			account                privateB4ByteLedgerSnapshot
			sequence, nextObjectID uint64
			objects                map[uint64]privateB4C15ObjectState
		}
		snapshot := func() contextSnapshot {
			ledger := account.snapshot()
			account.mu.Lock()
			defer account.mu.Unlock()
			objects := make(map[uint64]privateB4C15ObjectState, len(account.c15.objects))
			for id, state := range account.c15.objects {
				objects[id] = state
			}
			return contextSnapshot{account: ledger, sequence: account.c15.sequence, nextObjectID: account.c15.nextObjectID, objects: objects}
		}

		privateC15InstallObserver(t, account, func(privateB4C15Event) error { return nil })
		before := snapshot()
		seen := 0
		privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
			if event.SiteID == privateC15SiteC15Copy34OwnedPairParams || event.SiteID == privateC15SiteC15Copy35OwnedPairResult {
				seen++
				if event.SiteID == privateC15SiteC15Copy35OwnedPairResult {
					return errors.New("refuse second copy")
				}
			}
			return nil
		})
		if owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")}); failure != session.ResourceExhausted || owner != nil {
			t.Fatalf("ASSERT_C15_PAIR_BATCH_REFUSAL failure=%q owner=%v", failure, owner)
		}
		afterRefusal := snapshot()
		if seen != 2 || !reflect.DeepEqual(afterRefusal, before) {
			t.Fatalf("ASSERT_C15_PAIR_BATCH_REFUSAL_ATOMIC seen=%d before=%+v after=%+v", seen, before, afterRefusal)
		}

		var events []privateB4C15Event
		privateC15InstallObserver(t, account, func(event privateB4C15Event) error { events = append(events, event); return nil })
		owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
		if failure != "" || owner == nil {
			t.Fatalf("ASSERT_C15_PAIR_BATCH_SUCCESS failure=%q", failure)
		}
		if len(events) != 2 || events[0].SiteID != privateC15SiteC15Copy34OwnedPairParams || events[1].SiteID != privateC15SiteC15Copy35OwnedPairResult ||
			events[1].Sequence != events[0].Sequence+1 || events[0].ObjectID == events[1].ObjectID {
			t.Fatalf("ASSERT_C15_PAIR_BATCH_CAUSAL events=%+v", events)
		}
		lease := OwnedMethodPairLease{owner: owner}
		if err := lease.WithPair(func(v OwnedMethodPairView) error {
			if string(v.Params) != "params" || string(v.Result) != "result" {
				return errors.New("semantic output changed")
			}
			return nil
		}); err != nil {
			t.Fatalf("ASSERT_C15_PAIR_BATCH_OUTPUT err=%v", err)
		}
		if !lease.Release() || lease.Release() {
			t.Fatal("ASSERT_C15_PAIR_BATCH_EXACT_ONCE_OWNER_RELEASE")
		}
		terminal := map[string]int{}
		for _, event := range events {
			if event.Terminal {
				terminal[event.ObjectID]++
			}
		}
		if terminal[events[0].ObjectID] != 1 || terminal[events[1].ObjectID] != 1 {
			t.Fatalf("ASSERT_C15_PAIR_BATCH_EXACT_ONCE_OBJECT_RELEASE terminal=%v events=%+v", terminal, events)
		}
	})
}

func TestPrivateB4OwnedMethodPairLeaseLifecycleTimeoutReleaseRetry(t *testing.T) {
	f := stageBControls(t)
	m, child, _, key, result := stageBStopSelectedWithPair(t, f, true)
	lease, ok := result.PrivateOwnedMethodPairLease()
	if !ok {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_LEASE")
	}
	served := make(chan error, 1)
	go func() {
		reader := lspwire.NewReader(child.input, lspwire.DefaultLimits())
		message, err := reader.Read()
		if err != nil || message.Method != "shutdown" {
			served <- errors.New("shutdown")
			return
		}
		if err = lspwire.NewWriter(child.output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage("null")}); err != nil {
			served <- err
			return
		}
		message, err = reader.Read()
		if err != nil || message.Method != "exit" {
			served <- errors.New("exit")
			return
		}
		served <- nil
	}()
	intent := m.Stop(context.Background(), key.SessionID, "pair-stop")
	if intent.Failure != "" || intent.IntentID == "" {
		t.Fatalf("ASSERT_C15_PAIR_LIFECYCLE_INTENT %+v", intent)
	}
	lease.owner.mu.Lock()
	for !lease.owner.admissionClosed {
		lease.owner.cond.Wait()
	}
	lease.owner.mu.Unlock()
	timedResults := make(chan session.LifecycleResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			timedResults <- m.Stop(ctx, key.SessionID, "pair-stop")
		}()
	}
	for i := 0; i < 2; i++ {
		if timed := <-timedResults; timed.Failure != session.RequestTimeout || timed.IntentID != intent.IntentID {
			t.Fatalf("ASSERT_C15_PAIR_LIFECYCLE_TIMEOUT %+v", timed)
		}
	}
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_TERMINAL_ADMISSION")
	}
	if lease.owner.charge.account.snapshot().Live == 0 || !lease.Release() {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_VALID_AFTER_TIMEOUT")
	}
	lifecycleWait(t, m, intent.IntentID)
	m.mu.Lock()
	deleted := m.sessions[key.SessionID] == nil
	m.mu.Unlock()
	if !deleted {
		t.Fatal("ASSERT_C15_PAIR_LIFECYCLE_SESSION_NOT_DELETED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	completed := m.Stop(ctx, key.SessionID, "pair-stop")
	cancel()
	if completed.Failure != "" || completed.Outcome != session.OutcomeComplete {
		t.Fatalf("ASSERT_C15_PAIR_LIFECYCLE_RETRY %+v", completed)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestPrivateB4OwnedMethodPairLeaseProductionPlusOneRefusesAtomically(t *testing.T) {
	f := newFullP1Fixture(t)
	f.req.CaptureOwnedMethodPair = true
	var account *privateB4ByteAccountV2
	var filler privateB4ByteLeaseV2
	var before privateB4ByteLedgerSnapshot
	afterOwned := 0
	f.m.b4OwnedPairHooks.beforeReserve = func(a *privateB4ByteAccountV2, required uint64) {
		account = a
		remaining := privateB4MaxOwnedBytes - a.snapshot().Live
		var failure session.Failure
		filler, failure = a.reserve(remaining - required + 1)
		if failure != "" {
			t.Fatal(failure)
		}
		before = a.snapshot()
	}
	f.m.b4OwnedPairHooks.afterOwned = func(*privateB4ByteAccountV2, uint64) { afterOwned++ }
	f.m.b4OwnedPairHooks.refused = func(a *privateB4ByteAccountV2, _ uint64) {
		if a.snapshot() != before {
			t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_MUTATION before=%+v after=%+v", before, a.snapshot())
		}
	}
	result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) || afterOwned != 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE failure=%q lease=%+v owned=%d", result.Failure, managerLease, afterOwned)
	}
	if _, ok := result.PrivateOwnedMethodPairLease(); ok {
		t.Fatal("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_LEASE")
	}
	if _, ok := result.CompletedOwnedMethodPair(); ok {
		t.Fatal("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_LEGACY")
	}
	filler.release()
	if account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_PLUS_ONE_CLEANUP %+v", account.snapshot())
	}
}

func TestPrivateB4OwnedMethodPairLeaseProductionPublication(t *testing.T) {
	f := newFullP1Fixture(t)
	f.req.CaptureOwnedMethodPair = true
	var account *privateB4ByteAccountV2
	f.m.b4ParamsHook = func(a *privateB4ByteAccountV2) { account = a }
	result, managerLease, selection := f.transact(t)
	lease, ok := result.PrivateOwnedMethodPairLease()
	if !ok {
		t.Fatal("ASSERT_C15_PAIR_PRODUCTION_LEASE")
	}
	if _, legacy := result.CompletedOwnedMethodPair(); legacy {
		t.Fatal("ASSERT_C15_PAIR_PRIVATE_LEGACY_PUBLICATION")
	}
	if err := lease.WithPair(func(v OwnedMethodPairView) error {
		if v.Method != f.req.Method || len(v.Params) == 0 || len(v.Result) == 0 {
			return errors.New("wrong production view")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	requestLease, _ := result.PrivateB4RequestFrameLease()
	responseLease, _ := result.PrivateB4ResponseFrameLease()
	resultLease, _ := result.PrivateB4ResultLease()
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_CONSUME status=%s", status)
	}
	if !requestLease.Release() || !responseLease.Release() || !resultLease.Release() || account.snapshot().Live == 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_RETAINED snapshot=%+v", account.snapshot())
	}
	if !lease.Release() || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PAIR_PRODUCTION_RELEASE snapshot=%+v", account.snapshot())
	}
}

func TestPrivateOwnedMethodPairLeaseCopiedHandlesDrainAndRelease(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	before := a.snapshot()
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
	if failure != "" {
		t.Fatal(failure)
	}
	want := uint64(len("params") + len("result"))
	if got := a.snapshot(); got.Live != before.Live+want || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_PAIR_OWNED snapshot=%+v", got)
	}
	lease := OwnedMethodPairLease{owner: owner}
	copyLease := lease
	entered := make(chan struct{})
	leave := make(chan struct{})
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- lease.WithPair(func(v OwnedMethodPairView) error {
			if string(v.Params) != "params" || string(v.Result) != "result" {
				return errors.New("wrong view")
			}
			close(entered)
			<-leave
			return nil
		})
	}()
	<-entered
	releaseDone := make(chan bool, 1)
	go func() { releaseDone <- copyLease.Release() }()
	owner.mu.Lock()
	for !owner.releaseStarted {
		owner.cond.Wait()
	}
	owner.mu.Unlock()
	select {
	case <-releaseDone:
		t.Fatal("ASSERT_C15_PAIR_RELEASE_BEFORE_CALLBACK_DRAIN")
	default:
	}
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatalf("ASSERT_C15_PAIR_POST_CLOSE_ADMISSION err=%v", err)
	}
	close(leave)
	if err := <-callbackDone; err != nil {
		t.Fatal(err)
	}
	if !<-releaseDone || lease.Release() || copyLease.Release() {
		t.Fatal("ASSERT_C15_PAIR_EXACT_ONCE_RELEASE")
	}
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) {
		t.Fatal("ASSERT_C15_PAIR_STALE_ACCESS")
	}
	a.requestTerminalRelease()
	if got := a.snapshot(); got.Live != 0 || got.Cumulative != before.Cumulative+want {
		t.Fatalf("ASSERT_C15_PAIR_QUIESCENCE snapshot=%+v", got)
	}
}

func TestPrivateOwnedMethodPairLeaseCallbackReentersManagerAndAccount(t *testing.T) {
	m := newC15DiagnosticManager(t)
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure != "" {
		t.Fatal(failure)
	}
	lease := OwnedMethodPairLease{owner: owner}
	if err := lease.WithPair(func(OwnedMethodPairView) error {
		_ = m.Census()
		if a.snapshot().Live == 0 {
			return errors.New("missing charge")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !lease.Release() {
		t.Fatal("ASSERT_C15_PAIR_CALLBACK_REENTRY_RELEASE")
	}
	a.requestTerminalRelease()
}

func TestPrivateOwnedMethodPairLeaseTerminalClosesAdmissionAndDrainsCallback(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure != "" {
		t.Fatal(failure)
	}
	lease := OwnedMethodPairLease{owner: owner}
	entered := make(chan struct{})
	leave := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- lease.WithPair(func(OwnedMethodPairView) error { close(entered); <-leave; return nil })
	}()
	<-entered
	before := a.snapshot()
	owner.requestTerminal()
	if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); !errors.Is(err, ErrOwnedMethodPairLeaseClosed) || a.snapshot() != before {
		t.Fatalf("ASSERT_C15_PAIR_TERMINAL_ADMISSION err=%v before=%+v after=%+v", err, before, a.snapshot())
	}
	released := make(chan bool, 1)
	go func() { released <- lease.Release() }()
	select {
	case <-released:
		t.Fatal("ASSERT_C15_PAIR_TERMINAL_CALLBACK_EARLY_RELEASE")
	default:
	}
	close(leave)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !<-released {
		t.Fatal("ASSERT_C15_PAIR_TERMINAL_RELEASE")
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_TERMINAL_QUIESCENCE")
	}
}

func TestPrivateOwnedMethodPairLeasePostAccountTerminalRefusesAtomically(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	a.requestTerminalRelease()
	before := a.snapshot()
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure == "" || owner != nil || a.snapshot() != before {
		t.Fatalf("ASSERT_C15_PAIR_POST_TERMINAL_REFUSAL failure=%q owner=%v before=%+v after=%+v", failure, owner, before, a.snapshot())
	}
}

func TestPrivateOwnedMethodPairLeaseEqualityAndActualPlusOne(t *testing.T) {
	pair := OwnedMethodPair{Params: []byte("params"), Result: []byte("result")}
	required := uint64(len(pair.Params) + len(pair.Result))
	a := mustPrivateB4ByteAccountV2(t)
	before := a.snapshot()
	filler, failure := a.reserve(privateB4MaxOwnedBytes - before.Live - required)
	if failure != "" {
		t.Fatal(failure)
	}
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, pair)
	if failure != "" || a.snapshot().Live != privateB4MaxOwnedBytes {
		t.Fatalf("ASSERT_C15_PAIR_EQUALITY failure=%q snapshot=%+v", failure, a.snapshot())
	}
	if !((OwnedMethodPairLease{owner: owner}).Release()) {
		t.Fatal("ASSERT_C15_PAIR_EQUALITY_RELEASE")
	}
	filler.release()
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_EQUALITY_QUIESCENCE")
	}

	b := mustPrivateB4ByteAccountV2(t)
	start := b.snapshot()
	fill, failure := b.reserve(privateB4MaxOwnedBytes - start.Live - required + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	blocked := b.snapshot()
	if got, failure := newPrivateB4OwnedMethodPairOwner(b, pair); failure == "" || got != nil || b.snapshot() != blocked {
		t.Fatalf("ASSERT_C15_PAIR_PLUS_ONE failure=%q owner=%v before=%+v after=%+v", failure, got, blocked, b.snapshot())
	}
	fill.release()
	b.requestTerminalRelease()
	if b.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_PLUS_ONE_QUIESCENCE")
	}
}

func TestPrivateOwnedMethodPairLeaseReleaseRaceOneWinner(t *testing.T) {
	a := mustPrivateB4ByteAccountV2(t)
	owner, failure := newPrivateB4OwnedMethodPairOwner(a, OwnedMethodPair{Params: []byte("p"), Result: []byte("r")})
	if failure != "" {
		t.Fatal(failure)
	}
	lease := OwnedMethodPairLease{owner: owner}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(l OwnedMethodPairLease) {
			defer wg.Done()
			if l.Release() {
				winners.Add(1)
			}
		}(lease)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("ASSERT_C15_PAIR_RELEASE_WINNERS %d", winners.Load())
	}
	a.requestTerminalRelease()
	if a.snapshot().Live != 0 {
		t.Fatal("ASSERT_C15_PAIR_RELEASE_RACE_QUIESCENCE")
	}
}
