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
	callHierarchyLayer1GenericV3Dir    = "testdata/callhierarchy-layer1-red-v3"
	callHierarchyLayer1GenericV3Schema = "adr0011.callhierarchy.layer1.red-oracle-corpus/v3"
)

type callHierarchyLayer1GenericV3Manifest struct {
	Aggregate callHierarchyLayer1CorpusAggregate `json:"aggregate"`
	Schema    string                             `json:"schema"`
	Source    struct {
		Registry struct {
			ByteLength int    `json:"byte_length"`
			Path       string `json:"path"`
			SHA256     string `json:"sha256"`
		} `json:"registry"`
		V2 struct {
			Commit string `json:"commit"`
			Path   string `json:"path"`
		} `json:"v2"`
	} `json:"source"`
	Worlds []callHierarchyLayer1CorpusEntry `json:"worlds"`
}

type callHierarchyLayer1GenericV3Envelope struct {
	Driver   callHierarchyLayer1GenericV3Driver   `json:"driver"`
	Expected callHierarchyLayer1GenericV3Expected `json:"expected"`
}

type callHierarchyLayer1GenericV3CommitmentInput struct {
	Available bool    `json:"available"`
	BytesHex  *string `json:"bytes_hex"`
	Identity  string  `json:"identity"`
	Length    uint64  `json:"length"`
	SHA256    string  `json:"sha256"`
}

type callHierarchyLayer1GenericV3CommitmentInputs struct {
	Handoff      callHierarchyLayer1GenericV3CommitmentInput   `json:"handoff"`
	Predecessors []callHierarchyLayer1GenericV3CommitmentInput `json:"predecessors"`
	Selected     callHierarchyLayer1GenericV3CommitmentInput   `json:"selected"`
}

type callHierarchyLayer1GenericV3Driver struct {
	CommitmentInputs callHierarchyLayer1GenericV3CommitmentInputs `json:"commitment_inputs"`
	Derived          callHierarchyLayer1GenericV3DriverDerived    `json:"derived"`
	Request          callHierarchyLayer1GenericV3DriverRequest    `json:"request"`
	Transport        callHierarchyLayer1GenericV3DriverTransport  `json:"transport"`
}

type callHierarchyLayer1GenericV3DriverDerived struct {
	External callHierarchyLayer1GenericV3DriverExternal `json:"external"`
}

type callHierarchyLayer1GenericV3DriverExternal struct {
	NotificationsBytes *string `json:"notifications_bytes"`
	ResponsesBytes     *string `json:"responses_bytes"`
	ResultBytes        *string `json:"result_bytes"`
	ServerErrorBytes   *string `json:"server_error_bytes"`
}

type callHierarchyLayer1GenericV3DriverCausalOutcome struct {
	Class         *string `json:"class"`
	Terminal      *string `json:"terminal,omitempty"`
	RefusalClass  *string `json:"refusal_class,omitempty"`
	RefusalReason *string `json:"refusal_reason,omitempty"`
}

func (outcome *callHierarchyLayer1GenericV3DriverCausalOutcome) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("causal outcome must be an object")
	}
	seen := make(map[string]bool, 4)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return fmt.Errorf("causal outcome member name must be a string")
		}
		if seen[name] {
			return fmt.Errorf("duplicate causal outcome member %q", name)
		}
		seen[name] = true
		switch name {
		case "class":
			err = decoder.Decode(&outcome.Class)
		case "terminal":
			err = decoder.Decode(&outcome.Terminal)
		case "refusal_class":
			err = decoder.Decode(&outcome.RefusalClass)
		case "refusal_reason":
			err = decoder.Decode(&outcome.RefusalReason)
		default:
			return fmt.Errorf("unknown causal outcome member %q", name)
		}
		if err != nil {
			return fmt.Errorf("causal outcome member %q: %w", name, err)
		}
	}
	if _, err = decoder.Token(); err != nil {
		return err
	}
	if err = decoder.Decode(&token); !errors.Is(err, io.EOF) {
		return fmt.Errorf("causal outcome trailing value: %v", err)
	}
	return nil
}

func callHierarchyLayer1GenericV3CausalOutcomeValues(outcome *callHierarchyLayer1GenericV3DriverCausalOutcome) (class, terminal, refusalClass, refusalReason string, present bool, err error) {
	if outcome == nil {
		return "", "", "", "", false, nil
	}
	if outcome.Class == nil || *outcome.Class == "" {
		return "", "", "", "", true, errors.New("causal outcome requires class")
	}
	class = *outcome.Class
	switch class {
	case "accepted-terminal":
		if outcome.Terminal == nil || *outcome.Terminal != "successful-empty" || outcome.RefusalClass != nil || outcome.RefusalReason != nil {
			return "", "", "", "", true, errors.New("accepted-terminal requires only terminal successful-empty")
		}
		return class, *outcome.Terminal, "", "", true, nil
	case "pre-effect-refusal":
		if outcome.Terminal != nil || outcome.RefusalClass == nil || *outcome.RefusalClass != "method-unavailable" || outcome.RefusalReason == nil || *outcome.RefusalReason != "method unavailable" {
			return "", "", "", "", true, errors.New("pre-effect-refusal requires method-unavailable and method unavailable")
		}
		return class, "", *outcome.RefusalClass, *outcome.RefusalReason, true, nil
	default:
		return "", "", "", "", true, fmt.Errorf("invalid causal outcome class %q", class)
	}
}

func callHierarchyLayer1GenericV3TypedCausalOutcome(outcome *callHierarchyLayer1GenericV3DriverCausalOutcome) (callHierarchyLayer1GenericCausalOutcome, error) {
	class, terminal, _, refusalReason, present, err := callHierarchyLayer1GenericV3CausalOutcomeValues(outcome)
	if err != nil || !present {
		return callHierarchyLayer1GenericCausalOutcome{}, err
	}
	switch class {
	case "accepted-terminal":
		return callHierarchyLayer1GenericCausalOutcome{
			class:    callHierarchyLayer1GenericCausalOutcomeAcceptedTerminal,
			terminal: terminal,
		}, nil
	case "pre-effect-refusal":
		return callHierarchyLayer1GenericCausalOutcome{
			class:         callHierarchyLayer1GenericCausalOutcomePreEffectRefusal,
			refusalType:   callHierarchyLayer1GenericRefusalMethodUnavailable,
			refusalReason: session.Failure(refusalReason),
		}, nil
	default:
		return callHierarchyLayer1GenericCausalOutcome{}, fmt.Errorf("invalid causal outcome class %q", class)
	}
}

type callHierarchyLayer1GenericV3DriverIdentity struct {
	Binding           string `json:"binding"`
	Profile           string `json:"profile"`
	WorkspaceNonceHex string `json:"workspace_nonce_hex"`
}

func (identity *callHierarchyLayer1GenericV3DriverIdentity) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("identity must be an object")
	}
	seen := make(map[string]bool, 3)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return fmt.Errorf("identity member name must be a string")
		}
		if seen[name] {
			return fmt.Errorf("duplicate identity member %q", name)
		}
		seen[name] = true
		switch name {
		case "binding":
			err = decoder.Decode(&identity.Binding)
		case "profile":
			err = decoder.Decode(&identity.Profile)
		case "workspace_nonce_hex":
			err = decoder.Decode(&identity.WorkspaceNonceHex)
		default:
			return fmt.Errorf("unknown identity member %q", name)
		}
		if err != nil {
			return fmt.Errorf("identity member %q: %w", name, err)
		}
	}
	if _, err = decoder.Token(); err != nil {
		return err
	}
	if len(seen) != 3 || !seen["binding"] || !seen["profile"] || !seen["workspace_nonce_hex"] {
		return fmt.Errorf("identity requires exactly binding, profile, and workspace_nonce_hex")
	}
	if err = decoder.Decode(&token); !errors.Is(err, io.EOF) {
		return fmt.Errorf("identity trailing value: %v", err)
	}
	return nil
}

func callHierarchyLayer1GenericV3IdentityValues(identity *callHierarchyLayer1GenericV3DriverIdentity) (callHierarchyProfileIdentity, callHierarchyBindingIdentity, callHierarchyWorkspaceCustody, error) {
	if identity == nil {
		return "", "", callHierarchyWorkspaceCustody{}, errors.New("missing identity")
	}
	if identity.Profile == "" || identity.Binding == "" {
		return "", "", callHierarchyWorkspaceCustody{}, errors.New("empty profile or binding")
	}
	if len(identity.WorkspaceNonceHex) != sha256.Size*2 || identity.WorkspaceNonceHex != strings.ToLower(identity.WorkspaceNonceHex) || strings.HasPrefix(identity.WorkspaceNonceHex, "0x") {
		return "", "", callHierarchyWorkspaceCustody{}, errors.New("workspace nonce is not exactly 64 lowercase hexadecimal characters")
	}
	workspaceBytes, err := hex.DecodeString(identity.WorkspaceNonceHex)
	if err != nil || len(workspaceBytes) != sha256.Size {
		return "", "", callHierarchyWorkspaceCustody{}, errors.New("workspace nonce does not decode to 32 bytes")
	}
	var nonce [32]byte
	copy(nonce[:], workspaceBytes)
	if nonce == [32]byte{} {
		return "", "", callHierarchyWorkspaceCustody{}, errors.New("workspace nonce is zero")
	}
	return callHierarchyProfileIdentity(identity.Profile), callHierarchyBindingIdentity(identity.Binding), callHierarchyWorkspaceCustody{managerNonce: nonce}, nil
}

type callHierarchyLayer1GenericV3DriverRequest struct {
	AttachmentEnabled bool                                             `json:"attachment_enabled"`
	CausalOutcome     *callHierarchyLayer1GenericV3DriverCausalOutcome `json:"causal_outcome,omitempty"`
	DeadlineAfterNS   int64                                            `json:"deadline_after_ns"`
	Generation        uint64                                           `json:"generation"`
	Identity          *callHierarchyLayer1GenericV3DriverIdentity      `json:"identity"`
	MaxBytes          int64                                            `json:"max_bytes"`
	MaxMessages       int                                              `json:"max_messages"`
	Method            string                                           `json:"method"`
	ParamsHex         *string                                          `json:"params_hex"`
	ParamsState       string                                           `json:"params_state"`
	RequestID         uint64                                           `json:"request_id"`
	Session           string                                           `json:"session"`
}

func (request *callHierarchyLayer1GenericV3DriverRequest) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("driver request must be an object")
	}
	seen := make(map[string]bool, 12)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return fmt.Errorf("driver request member name must be a string")
		}
		if seen[name] {
			return fmt.Errorf("duplicate driver request member %q", name)
		}
		seen[name] = true
		var discard json.RawMessage
		if err := decoder.Decode(&discard); err != nil {
			return err
		}
		if name == "causal_outcome" && bytes.Equal(bytes.TrimSpace(discard), []byte("null")) {
			return errors.New("causal_outcome must not be null")
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	type plain callHierarchyLayer1GenericV3DriverRequest
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode((*plain)(request)); err != nil {
		return err
	}
	if (len(seen) != 11 && len(seen) != 12) || !seen["identity"] || (len(seen) == 12 && !seen["causal_outcome"]) {
		return fmt.Errorf("driver request requires exactly 11 members plus optional causal_outcome")
	}
	if err := strict.Decode(&token); !errors.Is(err, io.EOF) {
		return fmt.Errorf("driver request trailing value: %v", err)
	}
	return nil
}

type callHierarchyLayer1GenericV3DriverTransport struct {
	Events []callHierarchyLayer1GenericV3DriverEvent `json:"events"`
}

type callHierarchyLayer1GenericV3DriverEvent struct {
	AtOffsetNS int64   `json:"at_offset_ns"`
	Class      *string `json:"class,omitempty"`
	FrameHex   *string `json:"frame_hex,omitempty"`
	Kind       string  `json:"kind"`
	Message    *string `json:"message,omitempty"`
}

type callHierarchyLayer1GenericV3Frame struct {
	Available   bool    `json:"available"`
	FrameHex    *string `json:"frame_hex"`
	FrameLength uint64  `json:"frame_length"`
	FrameSHA256 string  `json:"frame_sha256"`
}

type callHierarchyLayer1GenericV3Frames struct {
	Auxiliary []struct{}                        `json:"auxiliary"`
	Request   callHierarchyLayer1GenericV3Frame `json:"request"`
	Response  callHierarchyLayer1GenericV3Frame `json:"response"`
}

type callHierarchyLayer1GenericV3Result struct {
	Available bool    `json:"available"`
	BytesHex  *string `json:"bytes_hex"`
}

type callHierarchyLayer1GenericV3Derived struct {
	Control   callHierarchyLayer1GenericV3DerivedControl   `json:"control"`
	External  callHierarchyLayer1GenericV3DerivedExternal  `json:"external"`
	Identity  callHierarchyLayer1GenericV3DerivedIdentity  `json:"identity"`
	Lifecycle callHierarchyLayer1GenericV3DerivedLifecycle `json:"lifecycle"`
}

type callHierarchyLayer1GenericV3DerivedControl struct {
	ActualAttachmentAvailable bool   `json:"actual_attachment_available"`
	ActualReason              string `json:"actual_reason"`
}

type callHierarchyLayer1GenericV3DerivedExternal struct {
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

type callHierarchyLayer1GenericV3DerivedIdentity struct {
	TransactionRequestEqualsRequestIdentity bool `json:"transaction_request_equals_request_identity"`
}

type callHierarchyLayer1GenericV3DerivedLifecycle struct {
	CallbackCompleted    bool `json:"callback_completed"`
	ObservationAvailable bool `json:"observation_available"`
	ReleaseFirst         bool `json:"release_first"`
	ReleaseSecond        bool `json:"release_second"`
}

type callHierarchyLayer1GenericV3Comparison struct {
	Available           bool                                    `json:"available"`
	DistinctMethod      bool                                    `json:"distinct_method,omitempty"`
	EqualExternalResult bool                                    `json:"equal_external_result,omitempty"`
	Executions          []callHierarchyLayer1GenericV3Envelope  `json:"executions,omitempty"`
	ExternalEqual       bool                                    `json:"external_equal,omitempty"`
	LeftWorld           string                                  `json:"left_world,omitempty"`
	NoAlias             bool                                    `json:"no_alias,omitempty"`
	Pairs               []callHierarchyLayer1GenericV3Pair      `json:"pairs,omitempty"`
	RetainedPost        *callHierarchyLayer1CorpusMutationValue `json:"retained_post,omitempty"`
	RetainedPre         *callHierarchyLayer1CorpusMutationValue `json:"retained_pre,omitempty"`
	RightWorld          string                                  `json:"right_world,omitempty"`
	SourcePost          *callHierarchyLayer1CorpusMutationValue `json:"source_post,omitempty"`
	SourcePre           *callHierarchyLayer1CorpusMutationValue `json:"source_pre,omitempty"`
	TransportEqual      bool                                    `json:"transport_equal,omitempty"`
}

type callHierarchyLayer1GenericV3Pair struct {
	Disabled       callHierarchyLayer1GenericV3Envelope `json:"disabled"`
	Enabled        callHierarchyLayer1GenericV3Envelope `json:"enabled"`
	ExternalEqual  bool                                 `json:"external_equal"`
	TransportEqual bool                                 `json:"transport_equal"`
}

type callHierarchyLayer1GenericV3Concurrency struct {
	Available bool                                   `json:"available"`
	Workers   []callHierarchyLayer1GenericV3Envelope `json:"workers"`
}

type callHierarchyLayer1GenericV3Error struct {
	Available     bool    `json:"available"`
	Class         string  `json:"class"`
	Code          *int64  `json:"code"`
	DataBase64    *string `json:"data_base64"`
	DataForm      string  `json:"data_form"`
	Message       *string `json:"message"`
	Present       bool    `json:"present"`
	SerializedHex *string `json:"serialized_hex"`
}

type callHierarchyLayer1GenericV3Expected struct {
	Accounting      callHierarchyLayer1CorpusAccounting      `json:"accounting"`
	Commitments     callHierarchyLayer1CorpusCommitments     `json:"commitments"`
	Comparison      *callHierarchyLayer1GenericV3Comparison  `json:"comparison,omitempty"`
	Concurrency     *callHierarchyLayer1GenericV3Concurrency `json:"concurrency,omitempty"`
	Control         *callHierarchyLayer1CorpusControl        `json:"control,omitempty"`
	Derived         callHierarchyLayer1GenericV3Derived      `json:"derived"`
	Error           callHierarchyLayer1GenericV3Error        `json:"error"`
	External        callHierarchyLayer1CorpusExternal        `json:"external"`
	Frames          callHierarchyLayer1GenericV3Frames       `json:"frames"`
	Identity        callHierarchyLayer1CorpusIdentity        `json:"identity"`
	Lifecycle       callHierarchyLayer1CorpusLifecycle       `json:"lifecycle"`
	Name            string                                   `json:"name"`
	Refusal         callHierarchyLayer1CorpusRefusal         `json:"refusal"`
	Result          callHierarchyLayer1GenericV3Result       `json:"result"`
	RoundTripLedger callHierarchyLayer1CorpusRoundTripLedger `json:"round_trip_ledger"`
	Terminal        callHierarchyLayer1CorpusAvailableString `json:"terminal"`
	Transport       callHierarchyLayer1CorpusTransport       `json:"transport"`
}

type callHierarchyLayer1GenericV3Loaded struct {
	entry    callHierarchyLayer1CorpusEntry
	envelope callHierarchyLayer1GenericV3Envelope
	raw      []byte
}

func callHierarchyLayer1GenericV3DecodeStrict(t *testing.T, name string, raw []byte, dst any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V3_STRICT_%s: %v", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("ASSERT_CH_GENERIC_V3_SINGLE_VALUE_%s: %v", name, err)
	}
	canonical, err := json.Marshal(dst)
	if err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V3_CANONICAL_MARSHAL_%s: %v", name, err)
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
		t.Fatalf("ASSERT_CH_GENERIC_V3_CANONICAL_BYTES_%s offset=%d lengths=%d/%d raw=%q canonical=%q", name, at, len(raw), len(canonical), rawTail, canonicalTail)
	}
}

func callHierarchyLayer1GenericV3Hex(t *testing.T, path string, value *string) []byte {
	t.Helper()
	if value == nil {
		return nil
	}
	if *value != strings.ToLower(*value) || strings.HasPrefix(*value, "0x") || len(*value)%2 != 0 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_DRIVER_HEX_%s: %q", path, *value)
	}
	decoded, err := hex.DecodeString(*value)
	if err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V3_DRIVER_HEX_%s: %v", path, err)
	}
	return decoded
}

func callHierarchyLayer1GenericV3ValidateDriver(t *testing.T, e callHierarchyLayer1GenericV3Envelope) {
	t.Helper()
	d := e.Driver
	validateCommitment := func(path string, input callHierarchyLayer1GenericV3CommitmentInput, expected callHierarchyLayer1CorpusCommitment) {
		actual := callHierarchyLayer1GenericV3Commitment(t, path, input)
		want := callHierarchyLayer1GenericV3CommitmentExpected(t, path+".expected", expected)
		if actual != want {
			t.Fatalf("ASSERT_CH_GENERIC_V3_COMMITMENT_DERIVATION_%s", path)
		}
	}
	validateCommitment("driver.commitment_inputs.selected", d.CommitmentInputs.Selected, e.Expected.Commitments.Selected)
	validateCommitment("driver.commitment_inputs.handoff", d.CommitmentInputs.Handoff, e.Expected.Commitments.Handoff)
	if len(d.CommitmentInputs.Predecessors) != len(e.Expected.Commitments.Predecessors) {
		t.Fatalf("ASSERT_CH_GENERIC_V3_PREDECESSOR_CARDINALITY")
	}
	for i := range d.CommitmentInputs.Predecessors {
		validateCommitment(fmt.Sprintf("driver.commitment_inputs.predecessors[%d]", i), d.CommitmentInputs.Predecessors[i], e.Expected.Commitments.Predecessors[i])
	}
	params := callHierarchyLayer1GenericV3Hex(t, e.Expected.Name+".request.params_hex", d.Request.ParamsHex)
	switch d.Request.ParamsState {
	case "nil":
		if d.Request.ParamsHex != nil {
			t.Fatalf("ASSERT_CH_GENERIC_V3_PARAMS_NIL_%s", e.Expected.Name)
		}
	case "empty":
		if d.Request.ParamsHex == nil || len(params) != 0 {
			t.Fatalf("ASSERT_CH_GENERIC_V3_PARAMS_EMPTY_%s", e.Expected.Name)
		}
	case "present":
		if d.Request.ParamsHex == nil || len(params) == 0 {
			t.Fatalf("ASSERT_CH_GENERIC_V3_PARAMS_PRESENT_%s", e.Expected.Name)
		}
	default:
		t.Fatalf("ASSERT_CH_GENERIC_V3_PARAMS_STATE_%s: %q", e.Expected.Name, d.Request.ParamsState)
	}
	if _, _, _, err := callHierarchyLayer1GenericV3IdentityValues(d.Request.Identity); err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V3_DRIVER_IDENTITY_%s: %v", e.Expected.Name, err)
	}
	if _, _, _, _, _, err := callHierarchyLayer1GenericV3CausalOutcomeValues(d.Request.CausalOutcome); err != nil {
		t.Fatalf("ASSERT_CH_GENERIC_V3_DRIVER_CAUSAL_OUTCOME_%s: %v", e.Expected.Name, err)
	}
	allowed := map[string]bool{"response-frame": true, "transport-error": true, "cancel": true, "deadline": true, "shutdown": true, "unavailable": true}
	for i, event := range d.Transport.Events {
		if !allowed[event.Kind] {
			t.Fatalf("ASSERT_CH_GENERIC_V3_EVENT_KIND_%s_%d: %q", e.Expected.Name, i, event.Kind)
		}
		switch event.Kind {
		case "response-frame":
			if event.FrameHex == nil || event.Class != nil || event.Message != nil {
				t.Fatalf("ASSERT_CH_GENERIC_V3_EVENT_SHAPE_%s_%d", e.Expected.Name, i)
			}
			callHierarchyLayer1GenericV3Hex(t, fmt.Sprintf("%s.event[%d].frame_hex", e.Expected.Name, i), event.FrameHex)
		case "transport-error":
			if event.FrameHex != nil || event.Class == nil || event.Message == nil {
				t.Fatalf("ASSERT_CH_GENERIC_V3_EVENT_SHAPE_%s_%d", e.Expected.Name, i)
			}
		default:
			if event.FrameHex != nil || event.Class != nil || event.Message != nil {
				t.Fatalf("ASSERT_CH_GENERIC_V3_EVENT_SHAPE_%s_%d", e.Expected.Name, i)
			}
		}
	}
}

func callHierarchyLayer1GenericV3SchemaPaths(typ reflect.Type, prefix string, out map[string]bool) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Slice {
		if typ.Elem().Kind() == reflect.Struct && typ.Elem().NumField() == 0 {
			out[prefix] = true
			return
		}
		callHierarchyLayer1GenericV3SchemaPaths(typ.Elem(), prefix+"[*]", out)
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
		callHierarchyLayer1GenericV3SchemaPaths(field.Type, path, out)
	}
}

func callHierarchyLayer1GenericV3ValidateExpectedSchema(t *testing.T) []string {
	t.Helper()
	paths := map[string]bool{}
	callHierarchyLayer1GenericV3SchemaPaths(reflect.TypeOf(callHierarchyLayer1GenericV3Expected{}), "", paths)
	if len(paths) != 145 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_EXPECTED_PATHS got=%d want=145", len(paths))
	}
	removed := []string{
		"frames.request.body.available", "frames.request.body.bytes_hex", "frames.request.body.length", "frames.request.body.sha256",
		"frames.request.payload.available", "frames.request.payload.bytes_hex", "frames.request.payload.length", "frames.request.payload.sha256",
		"frames.response.body.available", "frames.response.body.bytes_hex", "frames.response.body.length", "frames.response.body.sha256",
		"frames.response.payload.available", "frames.response.payload.bytes_hex", "frames.response.payload.length", "frames.response.payload.sha256",
		"result.form",
	}
	if len(removed) != 17 {
		t.Fatal("ASSERT_CH_GENERIC_V3_REMOVED_CARDINALITY")
	}
	for _, path := range removed {
		if paths[path] {
			t.Fatalf("ASSERT_CH_GENERIC_V3_REMOVED_PRESENT_%s", path)
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	return ordered
}

func callHierarchyLayer1GenericV3Partition(t *testing.T, paths []string) ([]string, []string) {
	t.Helper()
	b := map[string]bool{
		"lifecycle.acquired": true, "lifecycle.completed": true, "lifecycle.released": true, "lifecycle.sequence[*]": true,
		"derived.control.actual_attachment_available": true, "derived.control.actual_reason": true,
		"derived.external.result_bytes": true, "derived.external.server_error_bytes": true, "derived.external.notifications_bytes": true,
		"derived.external.responses_bytes": true, "derived.external.failure": true, "derived.external.messages": true,
		"derived.external.bytes": true, "derived.external.request_messages": true, "derived.external.request_bytes": true,
		"derived.lifecycle.observation_available": true, "derived.lifecycle.callback_completed": true,
		"derived.lifecycle.release_first": true, "derived.lifecycle.release_second": true,
		"derived.identity.transaction_request_equals_request_identity": true,
	}
	c := map[string]bool{
		"comparison.available": true, "concurrency.available": true, "control.attachment_available": true,
		"control.enabled": true, "control.observation_available": true, "control.paired_world": true,
		"control.reason": true, "name": true, "external.execution_expected": true,
	}
	if len(b) != 20 || len(c) != 9 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_ORDINARY_OWNERSHIP B=%d C=%d", len(b), len(c))
	}
	ordinaryA := make([]string, 0, 116)
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			t.Fatalf("ASSERT_CH_GENERIC_V3_PARTITION_DUPLICATE_%s", path)
		}
		seen[path] = true
		switch {
		case b[path]:
			delete(b, path)
		case c[path]:
			delete(c, path)
		default:
			ordinaryA = append(ordinaryA, path)
		}
	}
	if len(ordinaryA) != 116 || len(b) != 0 || len(c) != 0 || len(paths) != 145 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_PARTITION A=%d B_missing=%d C_missing=%d total=%d", len(ordinaryA), len(b), len(c), len(paths))
	}
	world24A := make([]string, 0, 81)
	world24LedgerB := 0
	for _, path := range ordinaryA {
		if strings.HasPrefix(path, "round_trip_ledger.dimensions.") {
			world24LedgerB++
			continue
		}
		world24A = append(world24A, path)
	}
	if len(world24A) != 81 || world24LedgerB != 35 || len(world24A)+20+world24LedgerB+9 != 145 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_WORLD24_OWNERSHIP A=%d ordinary_B=%d ledger_B=%d C=%d total=%d", len(world24A), 20, world24LedgerB, 9, len(world24A)+20+world24LedgerB+9)
	}
	return ordinaryA, world24A
}

func callHierarchyLayer1GenericV3Walk(e callHierarchyLayer1GenericV3Envelope, fn func(callHierarchyLayer1GenericV3Envelope)) {
	fn(e)
	if e.Expected.Concurrency != nil {
		for _, worker := range e.Expected.Concurrency.Workers {
			callHierarchyLayer1GenericV3Walk(worker, fn)
		}
	}
	if e.Expected.Comparison != nil {
		for _, execution := range e.Expected.Comparison.Executions {
			callHierarchyLayer1GenericV3Walk(execution, fn)
		}
		for _, pair := range e.Expected.Comparison.Pairs {
			callHierarchyLayer1GenericV3Walk(pair.Disabled, fn)
			callHierarchyLayer1GenericV3Walk(pair.Enabled, fn)
		}
	}
}

func callHierarchyLayer1GenericV3Load(t *testing.T) []callHierarchyLayer1GenericV3Loaded {
	t.Helper()
	manifestRaw, err := os.ReadFile(filepath.Join(callHierarchyLayer1GenericV3Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest callHierarchyLayer1GenericV3Manifest
	callHierarchyLayer1GenericV3DecodeStrict(t, "manifest", manifestRaw, &manifest)
	if manifest.Schema != callHierarchyLayer1GenericV3Schema || manifest.Source.Registry.ByteLength != 51098 || manifest.Source.Registry.Path != "docs/qualification/adr0011-callhierarchy-layer1-generic-v3-causal-input-registry.json" || len(manifest.Source.Registry.SHA256) != sha256.Size*2 || manifest.Source.V2.Commit == "" || manifest.Source.V2.Path != "sessionruntime/testdata/callhierarchy-layer1-red-v2" || manifest.Aggregate.ByteLength <= 0 || len(manifest.Aggregate.SHA256) != sha256.Size*2 || len(manifest.Worlds) != 25 || len(manifest.Aggregate.Order) != len(manifest.Worlds) {
		t.Fatalf("ASSERT_CH_GENERIC_V3_MANIFEST: %+v", manifest)
	}
	listed := map[string]bool{}
	aggregate := sha256.New()
	loaded := make([]callHierarchyLayer1GenericV3Loaded, 0, 25)
	for i, entry := range manifest.Worlds {
		path := fmt.Sprintf("worlds/world-%02d.json", i+1)
		name := fmt.Sprintf("world-%02d", i+1)
		if entry.Path != path || entry.Name != name || manifest.Aggregate.Order[i] != path || filepath.IsAbs(entry.Path) || filepath.ToSlash(filepath.Clean(entry.Path)) != entry.Path || strings.Contains(entry.Path, "..") || listed[entry.Path] {
			t.Fatalf("ASSERT_CH_GENERIC_V3_PATH_%02d: %+v", i+1, entry)
		}
		listed[entry.Path] = true
		raw, err := os.ReadFile(filepath.Join(callHierarchyLayer1GenericV3Dir, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(raw))
		if len(raw) != entry.ByteLength || hash != entry.SHA256 {
			t.Fatalf("ASSERT_CH_GENERIC_V3_WORLD_HASH_%02d", i+1)
		}
		aggregate.Write(raw)
		var envelope callHierarchyLayer1GenericV3Envelope
		callHierarchyLayer1GenericV3DecodeStrict(t, name, raw, &envelope)
		if envelope.Expected.Name != name {
			t.Fatalf("ASSERT_CH_GENERIC_V3_ROOT_NAME_%02d: %q", i+1, envelope.Expected.Name)
		}
		callHierarchyLayer1GenericV3Walk(envelope, func(record callHierarchyLayer1GenericV3Envelope) {
			callHierarchyLayer1GenericV3ValidateDriver(t, record)
		})
		loaded = append(loaded, callHierarchyLayer1GenericV3Loaded{entry: entry, envelope: envelope, raw: raw})
	}
	if got := fmt.Sprintf("%x", aggregate.Sum(nil)); got != manifest.Aggregate.SHA256 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_AGGREGATE: %s", got)
	}
	entries, err := os.ReadDir(filepath.Join(callHierarchyLayer1GenericV3Dir, "worlds"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(listed) {
		t.Fatalf("ASSERT_CH_GENERIC_V3_EXTRA_WORLD_FILES got=%d want=%d", len(entries), len(listed))
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !listed["worlds/"+entry.Name()] {
			t.Fatalf("ASSERT_CH_GENERIC_V3_UNLISTED_WORLD_%s", entry.Name())
		}
	}
	return loaded
}

type callHierarchyLayer1GenericV3Actual struct {
	result           RoundTripResult
	request          RoundTripRequest
	requestFrame     []byte
	responseFrame    []byte
	external         callHierarchyLayer1GenericV3ExternalActual
	attempted        bool
	executed         bool
	preEffectRefusal string
}

type callHierarchyLayer1GenericV3IdentityAttachment interface {
	privateCallHierarchyLayer1WithIdentity(callHierarchyProfileIdentity, callHierarchyBindingIdentity, callHierarchyWorkspaceCustody) callHierarchyLayer1RequestAttachment
}

type callHierarchyLayer1GenericV3EnabledAttachment interface {
	privateCallHierarchyLayer1WithEnabled(bool) callHierarchyLayer1RequestAttachment
}

type callHierarchyLayer1GenericV3CausalOutcomeAttachment interface {
	privateCallHierarchyLayer1WithCausalOutcome(callHierarchyLayer1GenericCausalOutcome) callHierarchyLayer1RequestAttachment
}

type callHierarchyLayer1GenericV3PredecessorAttachment interface {
	privateCallHierarchyLayer1WithPredecessors([]callHierarchyOpaqueCommitmentMetadata) callHierarchyLayer1RequestAttachment
}

type callHierarchyLayer1GenericV3LeaseAcquirer interface {
	privateCallHierarchyLayer1GenericLease() (any, bool)
}

type callHierarchyLayer1GenericV3ExternalActual struct {
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

type callHierarchyLayer1GenericV3Observation struct {
	attachment, owner, view, lease reflect.Value
	available                      bool
	reason                         string
}

const (
	callHierarchyLayer1GenericV3AttachmentField = "callHierarchyLayer1Generic"
	callHierarchyLayer1GenericV3AcquireMethod   = "privateCallHierarchyLayer1GenericLease"
	callHierarchyLayer1GenericV3ViewMethod      = "View"
	callHierarchyLayer1GenericV3CallbackMethod  = "Callback"
	callHierarchyLayer1GenericV3ReleaseMethod   = "Release"
)

type callHierarchyLayer1GenericV3Counters struct {
	roots, workers, executions, pairs atomic.Int64
	attempted, executed, refused      atomic.Int64
}

type callHierarchyLayer1GenericV3WriteCapture struct {
	mu    sync.Mutex
	frame []byte
	wrote chan struct{}
	once  sync.Once
}

func (w *callHierarchyLayer1GenericV3WriteCapture) Write(p []byte) (int, error) {
	n := len(p)
	w.mu.Lock()
	w.frame = append(w.frame, p[:n]...)
	w.mu.Unlock()
	w.once.Do(func() { close(w.wrote) })
	return n, nil
}
func (w *callHierarchyLayer1GenericV3WriteCapture) Close() error {
	w.once.Do(func() { close(w.wrote) })
	return nil
}
func (w *callHierarchyLayer1GenericV3WriteCapture) snapshot() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.frame...)
}

type callHierarchyLayer1GenericV3Transport struct {
	stdin     *callHierarchyLayer1GenericV3WriteCapture
	output    *io.PipeWriter
	stdout    *io.PipeReader
	events    []callHierarchyLayer1GenericV3DriverEvent
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	response  []byte
	requestID uint64
	deadline  time.Duration
}

func callHierarchyLayer1GenericV3NewTransport(events []callHierarchyLayer1GenericV3DriverEvent, cancel context.CancelFunc, requestID uint64, deadline time.Duration) *callHierarchyLayer1GenericV3Transport {
	stdout, output := io.Pipe()
	c := &callHierarchyLayer1GenericV3Transport{stdin: &callHierarchyLayer1GenericV3WriteCapture{wrote: make(chan struct{})}, output: output, stdout: stdout, events: events, cancel: cancel, done: make(chan struct{}), requestID: requestID, deadline: deadline}
	go c.replay()
	return c
}

func (c *callHierarchyLayer1GenericV3Transport) replay() {
	defer close(c.done)
	<-c.stdin.wrote
	started := time.Now()
	for _, event := range c.events {
		if wait := time.Duration(event.AtOffsetNS) - time.Since(started); wait > 0 {
			time.Sleep(wait)
		}
		switch event.Kind {
		case "response-frame":
			frame := callHierarchyLayer1GenericV3HexNoTest(event.FrameHex)
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
			_ = c.output.CloseWithError(errors.New(callHierarchyLayer1GenericV3StringValue(event.Message)))
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

func (c *callHierarchyLayer1GenericV3Transport) Stdin() io.WriteCloser { return c.stdin }
func (c *callHierarchyLayer1GenericV3Transport) Stdout() io.ReadCloser { return c.stdout }
func (c *callHierarchyLayer1GenericV3Transport) Teardown(context.Context) managedprocess.TeardownObservation {
	c.closeOnce.Do(func() { _ = c.stdin.Close(); _ = c.output.Close() })
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *callHierarchyLayer1GenericV3Transport) Close() managedprocess.ResourceObservation {
	c.closeOnce.Do(func() { _ = c.stdin.Close(); _ = c.output.Close() })
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *callHierarchyLayer1GenericV3Transport) responseSnapshot() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.response...)
}

func callHierarchyLayer1GenericV3HexNoTest(value *string) []byte {
	if value == nil {
		return nil
	}
	decoded, _ := hex.DecodeString(*value)
	return decoded
}

func callHierarchyLayer1GenericV3CommitmentExpected(t *testing.T, path string, value callHierarchyLayer1CorpusCommitment) callHierarchyOpaqueCommitmentMetadata {
	t.Helper()
	var digest [32]byte
	copy(digest[:], callHierarchyLayer1GenericV3Hex(t, path+".sha256", value.SHA256))
	identity, length := "", uint64(0)
	if value.Identity != nil {
		identity = *value.Identity
	}
	if value.Length != nil {
		length = *value.Length
	}
	return callHierarchyOpaqueCommitmentMetadata{identity: identity, sha256: digest, length: length, available: value.Available}
}

func callHierarchyLayer1GenericV3Commitment(t *testing.T, path string, value callHierarchyLayer1GenericV3CommitmentInput) callHierarchyOpaqueCommitmentMetadata {
	t.Helper()
	digestText := value.SHA256
	var digest [32]byte
	copy(digest[:], callHierarchyLayer1GenericV3Hex(t, path+".sha256", &digestText))
	material := callHierarchyLayer1GenericV3Hex(t, path+".bytes_hex", value.BytesHex)
	if value.Available {
		if value.BytesHex == nil || uint64(len(material)) != value.Length || sha256.Sum256(material) != digest || value.Identity == "" {
			t.Fatalf("ASSERT_CH_GENERIC_V3_DRIVER_COMMITMENT_%s", path)
		}
	} else if value.BytesHex != nil || value.Identity != "" || value.Length != 0 || digest != [32]byte{} {
		t.Fatalf("ASSERT_CH_GENERIC_V3_DRIVER_COMMITMENT_UNAVAILABLE_%s", path)
	}
	return callHierarchyOpaqueCommitmentMetadata{identity: value.Identity, sha256: digest, length: value.Length, available: value.Available}
}

func callHierarchyLayer1GenericV3ApplyIdentity(candidate any, identity *callHierarchyLayer1GenericV3DriverIdentity) (callHierarchyLayer1RequestAttachment, bool) {
	setter, ok := candidate.(callHierarchyLayer1GenericV3IdentityAttachment)
	if !ok {
		return callHierarchyLayer1RequestAttachment{}, false
	}
	profile, binding, workspace, err := callHierarchyLayer1GenericV3IdentityValues(identity)
	if err != nil {
		return callHierarchyLayer1RequestAttachment{}, false
	}
	return setter.privateCallHierarchyLayer1WithIdentity(profile, binding, workspace), true
}

func callHierarchyLayer1GenericV3Execute(t *testing.T, record callHierarchyLayer1GenericV3Envelope) callHierarchyLayer1GenericV3Actual {
	t.Helper()
	params := callHierarchyLayer1GenericV3Hex(t, record.Expected.Name+".driver.request.params_hex", record.Driver.Request.ParamsHex)
	var wireParams json.RawMessage
	if record.Driver.Request.ParamsHex != nil {
		wireParams = json.RawMessage(append([]byte(nil), params...))
	}
	paramsSource := params
	attachment := callHierarchyLayer1RequestAttachment{
		selected: callHierarchyLayer1GenericV3Commitment(t, "driver.commitment_inputs.selected", record.Driver.CommitmentInputs.Selected),
		handoff:  callHierarchyLayer1GenericV3Commitment(t, "driver.commitment_inputs.handoff", record.Driver.CommitmentInputs.Handoff),
	}
	identityAttachment, ok := callHierarchyLayer1GenericV3ApplyIdentity(any(attachment), record.Driver.Request.Identity)
	request := RoundTripRequest{
		SessionID: record.Driver.Request.Session, Generation: record.Driver.Request.Generation, Method: record.Driver.Request.Method,
		Params: wireParams, Deadline: time.Now().Add(time.Duration(record.Driver.Request.DeadlineAfterNS)),
		MaxMessages: record.Driver.Request.MaxMessages, MaxBytes: record.Driver.Request.MaxBytes,
		callHierarchyLayer1: attachment,
	}
	if !ok {
		return callHierarchyLayer1GenericV3Actual{request: request, attempted: true, preEffectRefusal: "request-attachment-method:privateCallHierarchyLayer1WithIdentity"}
	}
	attachment = identityAttachment
	enabledSetter, ok := any(attachment).(callHierarchyLayer1GenericV3EnabledAttachment)
	if !ok {
		enabledSetter, ok = any(&attachment).(callHierarchyLayer1GenericV3EnabledAttachment)
	}
	request.callHierarchyLayer1 = attachment
	if !ok {
		return callHierarchyLayer1GenericV3Actual{request: request, attempted: true, preEffectRefusal: "request-attachment-method:privateCallHierarchyLayer1WithEnabled"}
	}
	attachment = enabledSetter.privateCallHierarchyLayer1WithEnabled(record.Driver.Request.AttachmentEnabled)
	causalSetter, ok := any(attachment).(callHierarchyLayer1GenericV3CausalOutcomeAttachment)
	if !ok {
		causalSetter, ok = any(&attachment).(callHierarchyLayer1GenericV3CausalOutcomeAttachment)
	}
	request.callHierarchyLayer1 = attachment
	if !ok {
		return callHierarchyLayer1GenericV3Actual{request: request, attempted: true, preEffectRefusal: "request-attachment-method:privateCallHierarchyLayer1WithCausalOutcome"}
	}
	causalOutcome, err := callHierarchyLayer1GenericV3TypedCausalOutcome(record.Driver.Request.CausalOutcome)
	if err != nil {
		return callHierarchyLayer1GenericV3Actual{request: request, attempted: true, preEffectRefusal: "driver-causal-outcome:" + err.Error()}
	}
	attachment = causalSetter.privateCallHierarchyLayer1WithCausalOutcome(causalOutcome)
	if causalOutcome.class == callHierarchyLayer1GenericCausalOutcomePreEffectRefusal {
		return callHierarchyLayer1GenericV3Actual{request: request, attempted: true, preEffectRefusal: string(causalOutcome.refusalReason)}
	}
	predecessorSetter, ok := any(attachment).(callHierarchyLayer1GenericV3PredecessorAttachment)
	if !ok {
		predecessorSetter, ok = any(&attachment).(callHierarchyLayer1GenericV3PredecessorAttachment)
	}
	request.callHierarchyLayer1 = attachment
	if !ok {
		return callHierarchyLayer1GenericV3Actual{request: request, attempted: true, preEffectRefusal: "request-attachment-method:privateCallHierarchyLayer1WithPredecessors"}
	}
	predecessors := make([]callHierarchyOpaqueCommitmentMetadata, len(record.Driver.CommitmentInputs.Predecessors))
	for i, predecessor := range record.Driver.CommitmentInputs.Predecessors {
		predecessors[i] = callHierarchyLayer1GenericV3Commitment(t, fmt.Sprintf("driver.commitment_inputs.predecessors[%d]", i), predecessor)
	}
	request.callHierarchyLayer1 = predecessorSetter.privateCallHierarchyLayer1WithPredecessors(predecessors)

	ctx, cancel := context.WithCancel(context.Background())
	transport := callHierarchyLayer1GenericV3NewTransport(record.Driver.Transport.Events, cancel, record.Driver.Request.RequestID, time.Duration(record.Driver.Request.DeadlineAfterNS))
	manager, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 1, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{transport}})
	if err != nil {
		t.Fatal(err)
	}
	pending := lspwire.NewPending(0)
	manager.mu.Lock()
	manager.sessions[record.Driver.Request.Session] = &runtimeSession{record: Record{SessionID: record.Driver.Request.Session, Generation: record.Driver.Request.Generation, State: session.Ready}, process: transport, pending: pending, requests: make(map[lspwire.RequestKey]*Request)}
	manager.mu.Unlock()
	t.Cleanup(func() { cancel(); _ = transport.Teardown(context.Background()); _ = transport.Close() })
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
		t.Errorf("ASSERT_CH_GENERIC_V3_REQUEST_ID_%s got=%d want=%d", record.Expected.Name, result.Key.ID, record.Driver.Request.RequestID)
	}
	requestFrame := transport.stdin.snapshot()
	responseFrame := transport.responseSnapshot()
	external := callHierarchyLayer1GenericV3ExternalFromResult(result, requestFrame, responseFrame)
	return callHierarchyLayer1GenericV3Actual{result: result, request: request, requestFrame: external.requestFrame, responseFrame: external.responseFrame, external: external, attempted: true, executed: true}
}

func callHierarchyLayer1GenericV3ExternalFromResult(result RoundTripResult, requestFrame, responseFrame []byte) callHierarchyLayer1GenericV3ExternalActual {
	return callHierarchyLayer1GenericV3ExternalActual{
		requestFrame: append([]byte(nil), requestFrame...), responseFrame: append([]byte(nil), responseFrame...),
		result: append(json.RawMessage(nil), result.Result...), serverError: result.ServerError,
		notifications: append([]lspwire.Message(nil), result.Notifications...), responses: append([]lspwire.Message(nil), result.Responses...),
		failure: result.Failure, messages: result.Messages, bytes: result.Bytes,
		requestMessages: result.RequestMessages, requestBytes: result.RequestBytes,
	}
}

func callHierarchyLayer1GenericV3Equal(t *testing.T, world, path string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ASSERT_CH_GENERIC_V3_EXACT_%s path=%s got=%v expected=%v", world, path, callHierarchyLayer1GenericV3Scalar(reflect.ValueOf(got)), callHierarchyLayer1GenericV3Scalar(reflect.ValueOf(want)))
	}
}

func callHierarchyLayer1GenericV3Observe(result *RoundTripResult) callHierarchyLayer1GenericV3Observation {
	root := reflect.ValueOf(result)
	if root.Kind() != reflect.Pointer || root.IsNil() {
		return callHierarchyLayer1GenericV3Observation{reason: "round-trip-result-pointer"}
	}
	elem := root.Elem()
	field, ok := elem.Type().FieldByName(callHierarchyLayer1GenericV3AttachmentField)
	if !ok {
		return callHierarchyLayer1GenericV3Observation{reason: "field:" + callHierarchyLayer1GenericV3AttachmentField}
	}
	attachment := elem.FieldByIndex(field.Index)
	if (attachment.Kind() == reflect.Pointer || attachment.Kind() == reflect.Interface) && attachment.IsNil() {
		return callHierarchyLayer1GenericV3Observation{attachment: attachment, reason: "owner:nil"}
	}
	acquirer, ok := any(result).(callHierarchyLayer1GenericV3LeaseAcquirer)
	if !ok {
		acquirer, ok = any(*result).(callHierarchyLayer1GenericV3LeaseAcquirer)
	}
	if !ok {
		return callHierarchyLayer1GenericV3Observation{attachment: attachment, owner: attachment, reason: "interface:" + callHierarchyLayer1GenericV3AcquireMethod + "() (any,bool)"}
	}
	leaseValue, ok := acquirer.privateCallHierarchyLayer1GenericLease()
	if !ok {
		return callHierarchyLayer1GenericV3Observation{attachment: attachment, owner: attachment, reason: "acquire:false"}
	}
	lease := reflect.ValueOf(leaseValue)
	viewMethod := lease.MethodByName(callHierarchyLayer1GenericV3ViewMethod)
	if !viewMethod.IsValid() || viewMethod.Type().NumIn() != 0 || viewMethod.Type().NumOut() != 2 || viewMethod.Type().Out(1).Kind() != reflect.Bool {
		return callHierarchyLayer1GenericV3Observation{attachment: attachment, owner: attachment, lease: lease, reason: "method:View() (view,bool)"}
	}
	viewOut := viewMethod.Call(nil)
	if !viewOut[1].Bool() {
		return callHierarchyLayer1GenericV3Observation{attachment: attachment, owner: attachment, lease: lease, reason: "view:false"}
	}
	return callHierarchyLayer1GenericV3Observation{attachment: attachment, owner: attachment, lease: lease, view: viewOut[0], available: true}
}

func callHierarchyLayer1GenericV3Field(value reflect.Value, path string) (any, bool) {
	return callHierarchyLayer1GenericV3Project(value, strings.Split(path, "."))
}

func callHierarchyLayer1GenericV3Project(value reflect.Value, parts []string) (any, bool) {
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
		return callHierarchyLayer1GenericV3Scalar(value), true
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
		return callHierarchyLayer1GenericV3Project(fieldValue, parts[1:])
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
		return callHierarchyLayer1GenericV3Scalar(fieldValue), true
	}
	projected := make([]any, fieldValue.Len())
	if fieldValue.Kind() == reflect.Slice && fieldValue.IsNil() {
		projected = nil
	}
	for i := 0; i < fieldValue.Len(); i++ {
		item, ok := callHierarchyLayer1GenericV3Project(fieldValue.Index(i), parts[1:])
		if !ok {
			return nil, false
		}
		projected[i] = item
	}
	return projected, true
}

func callHierarchyLayer1GenericV3Scalar(value reflect.Value) any {
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
			values[i] = callHierarchyLayer1GenericV3Scalar(value.Index(i))
		}
		return values
	case reflect.Array:
		values := make([]any, value.Len())
		for i := range values {
			values[i] = callHierarchyLayer1GenericV3Scalar(value.Index(i))
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
			values[name] = callHierarchyLayer1GenericV3Scalar(value.Field(i))
		}
		return values
	}
	if value.CanInterface() {
		return value.Interface()
	}
	return fmt.Sprint(value)
}

func callHierarchyLayer1GenericV3AssertExternal(t *testing.T, record callHierarchyLayer1GenericV3Envelope, actual callHierarchyLayer1GenericV3Actual) {
	t.Helper()
	d, request := record.Driver.Request, actual.request
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "request.session", request.SessionID, d.Session)
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "request.generation", request.Generation, d.Generation)
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "request.method", request.Method, d.Method)
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "request.params", []byte(request.Params), callHierarchyLayer1GenericV3Hex(t, record.Expected.Name+".request.params_hex", d.ParamsHex))
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "request.max_messages", request.MaxMessages, d.MaxMessages)
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "request.max_bytes", request.MaxBytes, d.MaxBytes)
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "result.key.generation", actual.result.Key.Generation, d.Generation)
	callHierarchyLayer1GenericV3Equal(t, record.Expected.Name, "result.key.id", actual.result.Key.ID, d.RequestID)
}

func callHierarchyLayer1GenericV3AssertActual(t *testing.T, record callHierarchyLayer1GenericV3Envelope, actual callHierarchyLayer1GenericV3Actual, expectedPaths []string) callHierarchyLayer1GenericV3Observation {
	t.Helper()
	e := record.Expected
	callHierarchyLayer1GenericV3AssertExternal(t, record, actual)
	observation := callHierarchyLayer1GenericV3Observe(&actual.result)
	if !observation.available {
		return observation
	}
	compared, skipped, fieldErrors := 0, 0, 0
	for _, path := range expectedPaths {
		want, wantOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(e), path)
		got, gotOK := callHierarchyLayer1GenericV3Field(observation.view, path)
		if !wantOK || !gotOK {
			fieldErrors++
			t.Errorf("ASSERT_CH_GENERIC_V3_FIELD_%s path=%s expected_path=%t actual_path=%t", e.Name, path, wantOK, gotOK)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			fieldErrors++
			t.Errorf("ASSERT_CH_GENERIC_V3_EXACT_%s path=%s got=%v expected=%v", e.Name, path, got, want)
			continue
		}
		compared++
	}
	wantCompared := len(expectedPaths)
	if compared+skipped+fieldErrors != wantCompared {
		t.Errorf("ASSERT_CH_GENERIC_V3_A_ACCOUNTING_%s registry=%d compared=%d skipped=%d errors=%d want=%d", e.Name, len(expectedPaths), compared, skipped, fieldErrors, wantCompared)
	}
	if compared != wantCompared || fieldErrors != 0 {
		t.Errorf("ASSERT_CH_GENERIC_V3_A_COMPLETENESS_%s compared=%d errors=%d want_compared=%d want_errors=0", e.Name, compared, fieldErrors, wantCompared)
	}
	t.Logf("ASSERT_CH_GENERIC_V3_PATH_COUNTS world=%s registry=%d compared=%d skipped=%d errors=%d", e.Name, len(expectedPaths), compared, skipped, fieldErrors)
	return observation
}

func callHierarchyLayer1GenericV3AggregateDimension(workers []callHierarchyLayer1CorpusLedgerDimension) (callHierarchyLayer1CorpusLedgerDimension, error) {
	var before, after uint64
	for i, worker := range workers {
		if !worker.Available {
			return callHierarchyLayer1CorpusLedgerDimension{}, nil
		}
		if worker.Before == nil || worker.After == nil || worker.Delta == nil {
			return callHierarchyLayer1CorpusLedgerDimension{}, fmt.Errorf("worker %d has incomplete available dimension", i)
		}
		if *worker.After < *worker.Before {
			return callHierarchyLayer1CorpusLedgerDimension{}, fmt.Errorf("worker %d has after %d before before %d", i, *worker.After, *worker.Before)
		}
		before += *worker.Before
		after += *worker.After
	}
	delta := after - before
	return callHierarchyLayer1CorpusLedgerDimension{
		After:     &after,
		Available: true,
		Before:    &before,
		Delta:     &delta,
		KnownZero: delta == 0,
	}, nil
}

func callHierarchyLayer1GenericV3EqualDimension(got, want callHierarchyLayer1CorpusLedgerDimension) bool {
	return reflect.DeepEqual(got, want)
}

func callHierarchyLayer1GenericV3AssertWorld24Aggregate(t *testing.T, root callHierarchyLayer1GenericV3Envelope, workers []callHierarchyLayer1GenericV3Envelope, actuals []callHierarchyLayer1GenericV3Actual) {
	t.Helper()
	if len(workers) != 16 || len(actuals) != len(workers) {
		t.Fatalf("ASSERT_CH_GENERIC_V3_WORLD24_ACTUAL_WORKER_LEDGERS workers=%d actuals=%d want=16", len(workers), len(actuals))
	}
	observations := make([]callHierarchyLayer1GenericV3Observation, len(actuals))
	for i := range actuals {
		observations[i] = callHierarchyLayer1GenericV3Observe(&actuals[i].result)
	}
	dimensions := []struct {
		name string
		want callHierarchyLayer1CorpusLedgerDimension
	}{
		{"callbacks", root.Expected.RoundTripLedger.Dimensions.Callbacks},
		{"diagnostics", root.Expected.RoundTripLedger.Dimensions.Diagnostics},
		{"external_bytes", root.Expected.RoundTripLedger.Dimensions.ExternalBytes},
		{"leases", root.Expected.RoundTripLedger.Dimensions.Leases},
		{"reservations", root.Expected.RoundTripLedger.Dimensions.Reservations},
		{"transport_bytes", root.Expected.RoundTripLedger.Dimensions.TransportBytes},
		{"transport_messages", root.Expected.RoundTripLedger.Dimensions.TransportMessages},
	}
	for _, dimension := range dimensions {
		workerDimensions := make([]callHierarchyLayer1CorpusLedgerDimension, len(observations))
		for i, observation := range observations {
			if !observation.available {
				continue
			}
			base := "round_trip_ledger.dimensions." + dimension.name
			before, beforeOK := callHierarchyLayer1GenericV3Field(observation.view, base+".before")
			after, afterOK := callHierarchyLayer1GenericV3Field(observation.view, base+".after")
			delta, deltaOK := callHierarchyLayer1GenericV3Field(observation.view, base+".delta")
			available, availableOK := callHierarchyLayer1GenericV3Field(observation.view, base+".available")
			knownZero, knownZeroOK := callHierarchyLayer1GenericV3Field(observation.view, base+".known_zero")
			beforeValue, beforeValueOK := before.(uint64)
			afterValue, afterValueOK := after.(uint64)
			deltaValue, deltaValueOK := delta.(uint64)
			availableValue, availableValueOK := available.(bool)
			knownZeroValue, knownZeroValueOK := knownZero.(bool)
			if !beforeOK || !afterOK || !deltaOK || !availableOK || !knownZeroOK || !beforeValueOK || !afterValueOK || !deltaValueOK || !availableValueOK || !knownZeroValueOK {
				t.Errorf("ASSERT_CH_GENERIC_V3_WORLD24_WORKER_LEDGER worker=%d dimension=%s observation=%t before=%v/%t after=%v/%t delta=%v/%t available=%v/%t known_zero=%v/%t", i, dimension.name, observation.available, before, beforeOK, after, afterOK, delta, deltaOK, available, availableOK, knownZero, knownZeroOK)
				continue
			}
			workerDimensions[i] = callHierarchyLayer1CorpusLedgerDimension{Before: &beforeValue, After: &afterValue, Delta: &deltaValue, Available: availableValue, KnownZero: knownZeroValue}
		}
		got, err := callHierarchyLayer1GenericV3AggregateDimension(workerDimensions)
		if err != nil {
			t.Errorf("ASSERT_CH_GENERIC_V3_WORLD24_WORKER_LEDGER dimension=%s: %v", dimension.name, err)
			continue
		}
		want := dimension.want
		for _, workerDimension := range workerDimensions {
			if !workerDimension.Available {
				want = callHierarchyLayer1CorpusLedgerDimension{}
				break
			}
		}
		if !callHierarchyLayer1GenericV3EqualDimension(got, want) {
			t.Errorf("ASSERT_CH_GENERIC_V3_WORLD24_AGGREGATE dimension=%s got=%+v want=%+v", dimension.name, got, want)
		}
	}
}

func TestCallHierarchyLayer1GenericV3World24AggregateHelper(t *testing.T) {
	dimension := func(before, after uint64) callHierarchyLayer1CorpusLedgerDimension {
		delta := after - before
		return callHierarchyLayer1CorpusLedgerDimension{Before: &before, After: &after, Delta: &delta, Available: true, KnownZero: delta == 0}
	}

	t.Run("normal-sum", func(t *testing.T) {
		got, err := callHierarchyLayer1GenericV3AggregateDimension([]callHierarchyLayer1CorpusLedgerDimension{dimension(2, 5), dimension(7, 11)})
		want := dimension(9, 16)
		if err != nil || !callHierarchyLayer1GenericV3EqualDimension(got, want) {
			t.Fatalf("normal sum got=%+v want=%+v err=%v", got, want, err)
		}
	})

	t.Run("unavailable-normalized", func(t *testing.T) {
		got, err := callHierarchyLayer1GenericV3AggregateDimension([]callHierarchyLayer1CorpusLedgerDimension{dimension(2, 5), {}})
		want := callHierarchyLayer1CorpusLedgerDimension{}
		if err != nil || !callHierarchyLayer1GenericV3EqualDimension(got, want) {
			t.Fatalf("unavailable aggregate got=%+v want=%+v err=%v", got, want, err)
		}
	})

	t.Run("after-before-detected", func(t *testing.T) {
		before, after, delta := uint64(5), uint64(2), uint64(0)
		invalid := callHierarchyLayer1CorpusLedgerDimension{Before: &before, After: &after, Delta: &delta, Available: true}
		if _, err := callHierarchyLayer1GenericV3AggregateDimension([]callHierarchyLayer1CorpusLedgerDimension{invalid}); err == nil {
			t.Fatal("after < before was accepted")
		}
	})

	t.Run("known-zero", func(t *testing.T) {
		got, err := callHierarchyLayer1GenericV3AggregateDimension([]callHierarchyLayer1CorpusLedgerDimension{dimension(4, 4), dimension(9, 9)})
		want := dimension(13, 13)
		if err != nil || !callHierarchyLayer1GenericV3EqualDimension(got, want) {
			t.Fatalf("known zero got=%+v want=%+v err=%v", got, want, err)
		}
	})

	t.Run("inequality-detected", func(t *testing.T) {
		got, err := callHierarchyLayer1GenericV3AggregateDimension([]callHierarchyLayer1CorpusLedgerDimension{dimension(1, 3)})
		want := dimension(1, 3)
		want.KnownZero = true
		if err != nil || callHierarchyLayer1GenericV3EqualDimension(got, want) {
			t.Fatalf("five-member inequality got=%+v want=%+v err=%v", got, want, err)
		}
	})
}

func TestCallHierarchyLayer1GenericV3ComparatorProjection(t *testing.T) {
	t.Run("predecessors-wildcard-values", func(t *testing.T) {
		type predecessor struct {
			Name string `json:"name"`
		}
		type fixture struct {
			Predecessors []predecessor `json:"predecessors"`
		}
		got, ok := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{Predecessors: []predecessor{{Name: "first"}, {Name: "second"}}}), "predecessors[*].name")
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
		left, leftOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{Lifecycle: lifecycle{Sequence: []string{"acquire", "release"}}}), "lifecycle.sequence[*]")
		right, rightOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{Lifecycle: lifecycle{Sequence: []string{"acquire", "callback"}}}), "lifecycle.sequence[*]")
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
		nilValues, nilOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{}), "values[*]")
		emptyValues, emptyOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{Values: []string{}}), "values[*]")
		ordered, orderedOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{Values: []string{"a", "b"}}), "values[*]")
		reversed, reversedOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{Values: []string{"b", "a"}}), "values[*]")
		nilPointer, pointerOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{}), "pointer")
		nilInterface, interfaceOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{}), "interface")
		_, missingOK := callHierarchyLayer1GenericV3Field(reflect.ValueOf(fixture{}), "missing")
		if !nilOK || !emptyOK || !orderedOK || !reversedOK || !pointerOK || !interfaceOK || missingOK || nilPointer != nil || nilInterface != nil || reflect.DeepEqual(nilValues, emptyValues) || reflect.DeepEqual(ordered, reversed) {
			t.Fatalf("nil=%#v empty=%#v ordered=%#v reversed=%#v pointer=%#v interface=%#v missing_ok=%t", nilValues, emptyValues, ordered, reversed, nilPointer, nilInterface, missingOK)
		}
	})
}

func callHierarchyLayer1GenericV3StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func callHierarchyLayer1GenericV3NullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func callHierarchyLayer1GenericV3MutationChecks(t *testing.T, record callHierarchyLayer1GenericV3Envelope) {
	t.Helper()
	actual := callHierarchyLayer1GenericV3Execute(t, record)
	want := callHierarchyLayer1GenericV3Hex(t, record.Expected.Name+".mutation.params", record.Driver.Request.ParamsHex)
	if !bytes.Equal(actual.request.Params, want) {
		t.Errorf("ASSERT_CH_GENERIC_V3_PARAMS_OPAQUE_%s got=%x want=%x", record.Expected.Name, actual.request.Params, want)
	}
	observation := callHierarchyLayer1GenericV3Observe(&actual.result)
	if !observation.available {
		return
	}
	before := fmt.Sprint(callHierarchyLayer1GenericV3Scalar(observation.view))
	if len(actual.request.Params) > 0 {
		actual.request.Params[0] ^= 0xff
	}
	again := callHierarchyLayer1GenericV3Observe(&actual.result)
	if !again.available || fmt.Sprint(callHierarchyLayer1GenericV3Scalar(again.view)) != before {
		t.Errorf("ASSERT_CH_GENERIC_V3_RETAINED_INPUT_ALIAS_%s", record.Expected.Name)
	}
}

func callHierarchyLayer1GenericV3AttachmentMappingErrors(worlds []callHierarchyLayer1GenericV3Loaded) []string {
	if len(worlds) != 25 || worlds[23].envelope.Expected.Concurrency == nil || worlds[24].envelope.Expected.Comparison == nil {
		return []string{"ASSERT_CH_GENERIC_V3_ATTACHMENT_MAPPING_SHAPE"}
	}
	errors := []string{}
	physicalControls, workers, enabled, disabled := 0, 0, 0, 0
	check := func(category string, record callHierarchyLayer1GenericV3Envelope, wantEnabled bool, wantControl bool) {
		if record.Driver.Request.AttachmentEnabled != wantEnabled {
			errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_ATTACHMENT_MAPPING category=%s got=%t want=%t", category, record.Driver.Request.AttachmentEnabled, wantEnabled))
		}
		if record.Driver.Request.AttachmentEnabled {
			enabled++
		} else {
			disabled++
		}
		if wantControl {
			physicalControls++
			if record.Expected.Control == nil ||
				record.Expected.Control.Enabled != record.Driver.Request.AttachmentEnabled ||
				record.Expected.Control.AttachmentAvailable != record.Driver.Request.AttachmentEnabled ||
				record.Expected.Control.ObservationAvailable != record.Driver.Request.AttachmentEnabled {
				errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_ATTACHMENT_CONTROL category=%s present=%t", category, record.Expected.Control != nil))
			}
		} else {
			workers++
			if record.Expected.Control != nil {
				errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_ATTACHMENT_WORKER_CONTROL_BORROW category=%s", category))
			}
		}
	}
	for i := range worlds {
		check("root", worlds[i].envelope, true, true)
	}
	for i := range worlds[23].envelope.Expected.Concurrency.Workers {
		check("world-24-worker", worlds[23].envelope.Expected.Concurrency.Workers[i], true, false)
	}
	comparison := worlds[24].envelope.Expected.Comparison
	for i := range comparison.Executions {
		check("standalone", comparison.Executions[i], i%2 == 0, true)
	}
	for i := range comparison.Pairs {
		check("pair-disabled", comparison.Pairs[i].Disabled, false, true)
		check("pair-enabled", comparison.Pairs[i].Enabled, true, true)
	}
	if physicalControls != 69 || workers != 16 || disabled != 22 || enabled != 63 {
		errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_ATTACHMENT_MAPPING_COUNTS controls=%d workers=%d disabled=%d enabled=%d", physicalControls, workers, disabled, enabled))
	}
	return errors
}

func callHierarchyLayer1GenericV3ValidateAttachmentMapping(t *testing.T, worlds []callHierarchyLayer1GenericV3Loaded) {
	t.Helper()
	for _, mappingError := range callHierarchyLayer1GenericV3AttachmentMappingErrors(worlds) {
		t.Error(mappingError)
	}
}

func TestCallHierarchyLayer1GenericV3AttachmentMappingControls(t *testing.T) {
	worlds := callHierarchyLayer1GenericV3Load(t)
	callHierarchyLayer1GenericV3ValidateAttachmentMapping(t, worlds)

	t.Run("missing-physical-control", func(t *testing.T) {
		copyWorlds := append([]callHierarchyLayer1GenericV3Loaded(nil), worlds...)
		copyWorlds[0].envelope.Expected.Control = nil
		if len(callHierarchyLayer1GenericV3AttachmentMappingErrors(copyWorlds)) == 0 {
			t.Fatal("ASSERT_CH_GENERIC_V3_ATTACHMENT_MISSING_CONTROL_ACCEPTED")
		}
	})
	for _, mismatch := range []struct {
		name   string
		mutate func(*callHierarchyLayer1CorpusControl)
	}{
		{"enabled-mismatch", func(control *callHierarchyLayer1CorpusControl) { control.Enabled = !control.Enabled }},
		{"attachment-available-mismatch", func(control *callHierarchyLayer1CorpusControl) {
			control.AttachmentAvailable = !control.AttachmentAvailable
		}},
		{"observation-available-mismatch", func(control *callHierarchyLayer1CorpusControl) {
			control.ObservationAvailable = !control.ObservationAvailable
		}},
	} {
		t.Run(mismatch.name, func(t *testing.T) {
			copyWorlds := append([]callHierarchyLayer1GenericV3Loaded(nil), worlds...)
			control := *copyWorlds[0].envelope.Expected.Control
			mismatch.mutate(&control)
			copyWorlds[0].envelope.Expected.Control = &control
			if len(callHierarchyLayer1GenericV3AttachmentMappingErrors(copyWorlds)) == 0 {
				t.Fatalf("ASSERT_CH_GENERIC_V3_ATTACHMENT_%s_ACCEPTED", mismatch.name)
			}
		})
	}
	t.Run("worker-control-borrow", func(t *testing.T) {
		copyWorlds := append([]callHierarchyLayer1GenericV3Loaded(nil), worlds...)
		workers := append([]callHierarchyLayer1GenericV3Envelope(nil), copyWorlds[23].envelope.Expected.Concurrency.Workers...)
		control := *copyWorlds[0].envelope.Expected.Control
		workers[0].Expected.Control = &control
		concurrency := *copyWorlds[23].envelope.Expected.Concurrency
		concurrency.Workers = workers
		copyWorlds[23].envelope.Expected.Concurrency = &concurrency
		if len(callHierarchyLayer1GenericV3AttachmentMappingErrors(copyWorlds)) == 0 {
			t.Fatal("ASSERT_CH_GENERIC_V3_ATTACHMENT_WORKER_CONTROL_ACCEPTED")
		}
	})
}

type callHierarchyLayer1GenericV3IdentityRecorder struct {
	attachment callHierarchyLayer1RequestAttachment
	profile    callHierarchyProfileIdentity
	binding    callHierarchyBindingIdentity
	workspace  callHierarchyWorkspaceCustody
	calls      int
}

func (recorder *callHierarchyLayer1GenericV3IdentityRecorder) privateCallHierarchyLayer1WithIdentity(profile callHierarchyProfileIdentity, binding callHierarchyBindingIdentity, workspace callHierarchyWorkspaceCustody) callHierarchyLayer1RequestAttachment {
	recorder.calls++
	recorder.profile = profile
	recorder.binding = binding
	recorder.workspace = workspace
	return recorder.attachment
}

func TestCallHierarchyLayer1GenericV3IdentityCausalGuards(t *testing.T) {
	const valid = `{"binding":"opaque-incoming","profile":"corpus-profile-go","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`
	decode := func(raw string) (*callHierarchyLayer1GenericV3DriverIdentity, error) {
		decoder := json.NewDecoder(strings.NewReader(raw))
		var identity *callHierarchyLayer1GenericV3DriverIdentity
		if err := decoder.Decode(&identity); err != nil {
			return nil, err
		}
		var trailing json.RawMessage
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("identity trailing value: %v", err)
		}
		return identity, nil
	}

	t.Run("strict-malformed-duplicate-missing-zero", func(t *testing.T) {
		var duplicateRequest callHierarchyLayer1GenericV3DriverRequest
		if err := json.Unmarshal([]byte(`{"attachment_enabled":true,"deadline_after_ns":1,"generation":1,"identity":{"binding":"b","profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"},"identity":{"binding":"b","profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"},"max_bytes":1,"max_messages":1,"method":"m","params_hex":null,"params_state":"nil","request_id":1,"session":"s"}`), &duplicateRequest); err == nil {
			t.Fatal("ASSERT_CH_GENERIC_V3_DUPLICATE_IDENTITY_PATH_ACCEPTED")
		}
		invalidJSON := []struct {
			name string
			raw  string
		}{
			{"null-identity", `null`},
			{"empty-object", `{}`},
			{"absent-profile", `{"binding":"b","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"absent-binding", `{"profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"absent-workspace", `{"binding":"b","profile":"p"}`},
			{"null-profile", `{"binding":"b","profile":null,"workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"null-binding", `{"binding":null,"profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"null-workspace", `{"binding":"b","profile":"p","workspace_nonce_hex":null}`},
			{"wrong-type-profile", `{"binding":"b","profile":1,"workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"wrong-type-binding", `{"binding":true,"profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"wrong-type-workspace", `{"binding":"b","profile":"p","workspace_nonce_hex":[]}`},
			{"duplicate-binding", `{"binding":"b","binding":"b2","profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"unknown-member", `{"binding":"b","profile":"p","workspace_nonce_hex":"01","extra":"x"}`},
			{"workspace-0x-prefix", `{"binding":"b","profile":"p","workspace_nonce_hex":"0x0101010101010101010101010101010101010101010101010101010101010101"}`},
			{"trailing-json", valid + ` null`},
		}
		for _, test := range invalidJSON {
			t.Run(test.name, func(t *testing.T) {
				identity, err := decode(test.raw)
				if err == nil {
					_, _, _, err = callHierarchyLayer1GenericV3IdentityValues(identity)
				}
				if err == nil {
					t.Fatalf("ASSERT_CH_GENERIC_V3_IDENTITY_INVALID_ACCEPTED raw=%s", test.raw)
				}
			})
		}
		for _, identity := range []*callHierarchyLayer1GenericV3DriverIdentity{
			{Binding: "", Profile: "p", WorkspaceNonceHex: strings.Repeat("01", 32)},
			{Binding: "b", Profile: "", WorkspaceNonceHex: strings.Repeat("01", 32)},
			{Binding: "b", Profile: "p", WorkspaceNonceHex: strings.Repeat("0", 64)},
			{Binding: "b", Profile: "p", WorkspaceNonceHex: strings.Repeat("A1", 32)},
			{Binding: "b", Profile: "p", WorkspaceNonceHex: strings.Repeat("1", 63)},
			{Binding: "b", Profile: "p", WorkspaceNonceHex: strings.Repeat("1", 65)},
			{Binding: "b", Profile: "p", WorkspaceNonceHex: strings.Repeat("gg", 32)},
		} {
			if _, _, _, err := callHierarchyLayer1GenericV3IdentityValues(identity); err == nil {
				t.Errorf("ASSERT_CH_GENERIC_V3_IDENTITY_VALUE_INVALID_ACCEPTED identity=%+v", identity)
			}
		}
	})

	t.Run("expected-and-method-are-not-input-oracles", func(t *testing.T) {
		identity, err := decode(valid)
		if err != nil {
			t.Fatal(err)
		}
		before := *identity
		record := callHierarchyLayer1GenericV3Envelope{}
		record.Driver.Request.Identity = identity
		record.Driver.Request.Method = "method-that-does-not-match-binding"
		changedExpected := "changed-expected-only"
		record.Expected.Identity.Profile.Value = &changedExpected
		recorder := &callHierarchyLayer1GenericV3IdentityRecorder{}
		if _, ok := callHierarchyLayer1GenericV3ApplyIdentity(recorder, record.Driver.Request.Identity); !ok {
			t.Fatal("ASSERT_CH_GENERIC_V3_IDENTITY_CAUSAL_INPUT_REFUSED")
		}
		if *record.Driver.Request.Identity != before || recorder.profile != callHierarchyProfileIdentity(before.Profile) || recorder.binding != callHierarchyBindingIdentity(before.Binding) || recorder.calls != 1 {
			t.Fatalf("ASSERT_CH_GENERIC_V3_EXPECTED_OR_METHOD_ORACLE driver=%+v actual=%q/%q calls=%d", record.Driver.Request.Identity, recorder.profile, recorder.binding, recorder.calls)
		}
	})

	apply := func(t *testing.T, identity *callHierarchyLayer1GenericV3DriverIdentity) *callHierarchyLayer1GenericV3IdentityRecorder {
		t.Helper()
		recorder := &callHierarchyLayer1GenericV3IdentityRecorder{}
		if _, ok := callHierarchyLayer1GenericV3ApplyIdentity(recorder, identity); !ok {
			t.Fatal("ASSERT_CH_GENERIC_V3_IDENTITY_MUTATION_REFUSED")
		}
		return recorder
	}

	t.Run("profile-mutation-changes-only-profile-argument", func(t *testing.T) {
		first, err := decode(valid)
		if err != nil {
			t.Fatal(err)
		}
		second := *first
		second.Profile = "mutated-profile"
		left, right := apply(t, first), apply(t, &second)
		if left.calls != 1 || right.calls != 1 || left.profile != callHierarchyProfileIdentity(first.Profile) || right.profile != callHierarchyProfileIdentity(second.Profile) || left.binding != right.binding || left.workspace != right.workspace {
			t.Fatalf("ASSERT_CH_GENERIC_V3_IDENTITY_PROFILE_MUTATION_SCOPE left=%+v right=%+v", left, right)
		}
	})

	for _, test := range []struct {
		name              string
		mutate            func(*callHierarchyLayer1GenericV3DriverIdentity)
		wantProfileSame   bool
		wantBindingSame   bool
		wantWorkspaceSame bool
	}{
		{"binding-mutation-changes-only-binding-argument", func(identity *callHierarchyLayer1GenericV3DriverIdentity) { identity.Binding = "mutated-binding" }, true, false, true},
		{"nonce-nibble-mutation-changes-only-workspace-argument", func(identity *callHierarchyLayer1GenericV3DriverIdentity) {
			identity.WorkspaceNonceHex = "11" + identity.WorkspaceNonceHex[2:]
		}, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			first, err := decode(valid)
			if err != nil {
				t.Fatal(err)
			}
			baseline := callHierarchyLayer1GenericV3Envelope{}
			baseline.Driver.Request.Identity = first
			baseline.Driver.Request.Method = "method-is-not-an-identity-source"
			expected := "expected-is-not-an-identity-source"
			baseline.Expected.Identity.Profile.Value = &expected
			mutated := baseline
			second := *first
			test.mutate(&second)
			mutated.Driver.Request.Identity = &second
			left, right := apply(t, baseline.Driver.Request.Identity), apply(t, mutated.Driver.Request.Identity)
			profileSame := left.profile == right.profile
			bindingSame := left.binding == right.binding
			workspaceSame := left.workspace == right.workspace
			if baseline.Driver.Request.Method != mutated.Driver.Request.Method || baseline.Expected.Identity.Profile.Value != mutated.Expected.Identity.Profile.Value || left.calls != 1 || right.calls != 1 || profileSame != test.wantProfileSame || bindingSame != test.wantBindingSame || workspaceSame != test.wantWorkspaceSame {
				t.Fatalf("ASSERT_CH_GENERIC_V3_IDENTITY_ARGUMENT_MUTATION_SCOPE test=%s method=%q/%q expected=%p/%p profile_same=%t binding_same=%t workspace_same=%t left=%+v right=%+v", test.name, baseline.Driver.Request.Method, mutated.Driver.Request.Method, baseline.Expected.Identity.Profile.Value, mutated.Expected.Identity.Profile.Value, profileSame, bindingSame, workspaceSame, left, right)
			}
		})
	}
}

type callHierarchyLayer1GenericV3CausalOutcomeRecorder struct {
	attachment callHierarchyLayer1RequestAttachment
	outcome    callHierarchyLayer1GenericCausalOutcome
	calls      int
}

func (recorder *callHierarchyLayer1GenericV3CausalOutcomeRecorder) privateCallHierarchyLayer1WithCausalOutcome(outcome callHierarchyLayer1GenericCausalOutcome) callHierarchyLayer1RequestAttachment {
	recorder.calls++
	recorder.outcome = outcome
	return recorder.attachment
}

func callHierarchyLayer1GenericV3CausalOutcomePlacementErrors(worlds []callHierarchyLayer1GenericV3Loaded) []string {
	want := map[string]uint64{"W04": 104, "W05": 105, "W23": 123}
	seen := map[string]int{}
	absent := 0
	errors := []string{}
	for _, world := range worlds {
		callHierarchyLayer1GenericV3Walk(world.envelope, func(record callHierarchyLayer1GenericV3Envelope) {
			requestID, scoped := want[record.Expected.Name]
			present := record.Driver.Request.CausalOutcome != nil
			if scoped && record.Driver.Request.RequestID == requestID {
				seen[record.Expected.Name]++
				if !present {
					errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_SCOPED_ABSENT identity=%s/%d", record.Expected.Name, requestID))
				}
				return
			}
			if present {
				errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_UNSCOPED_PRESENT identity=%s/%d", record.Expected.Name, record.Driver.Request.RequestID))
				return
			}
			absent++
		})
	}
	for name := range want {
		if seen[name] != 1 {
			errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_SCOPED_OCCURRENCE identity=%s/%d got=%d want=1", name, want[name], seen[name]))
		}
	}
	if absent != 82 {
		errors = append(errors, fmt.Sprintf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_OTHER_COUNT got=%d want=82", absent))
	}
	return errors
}

func TestCallHierarchyLayer1GenericV3CausalOutcomeGuards(t *testing.T) {
	decode := func(raw string) (*callHierarchyLayer1GenericV3DriverCausalOutcome, error) {
		if strings.TrimSpace(raw) == "null" {
			return nil, errors.New("causal outcome must not be null")
		}
		decoder := json.NewDecoder(strings.NewReader(raw))
		var outcome *callHierarchyLayer1GenericV3DriverCausalOutcome
		if err := decoder.Decode(&outcome); err != nil {
			return nil, err
		}
		var trailing json.RawMessage
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("causal outcome trailing value: %v", err)
		}
		return outcome, nil
	}
	validAccepted := `{"class":"accepted-terminal","terminal":"successful-empty"}`
	validRefusal := `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`
	requestPrefix := `{"attachment_enabled":true,"causal_outcome":`
	requestSuffix := `,"deadline_after_ns":1,"generation":1,"identity":{"binding":"b","profile":"p","workspace_nonce_hex":"0101010101010101010101010101010101010101010101010101010101010101"},"max_bytes":1,"max_messages":1,"method":"m","params_hex":null,"params_state":"nil","request_id":1,"session":"s"}`
	for _, test := range []struct{ name, raw string }{
		{"request-null", requestPrefix + `null` + requestSuffix},
		{"request-duplicate", strings.Replace(requestPrefix+validAccepted+requestSuffix, `"causal_outcome":`+validAccepted, `"causal_outcome":`+validAccepted+`,"causal_outcome":`+validAccepted, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var request callHierarchyLayer1GenericV3DriverRequest
			if err := json.Unmarshal([]byte(test.raw), &request); err == nil {
				t.Fatalf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_REQUEST_INVALID_ACCEPTED raw=%s", test.raw)
			}
		})
	}

	for _, test := range []struct{ name, raw string }{
		{"null", `null`},
		{"array", `[]`},
		{"string-scalar", `"accepted-terminal"`},
		{"number-scalar", `1`},
		{"boolean-scalar", `true`},
		{"empty-object", `{}`},
		{"missing-accepted-class", `{"terminal":"successful-empty"}`},
		{"missing-accepted-terminal", `{"class":"accepted-terminal"}`},
		{"null-accepted-class", `{"class":null,"terminal":"successful-empty"}`},
		{"null-accepted-terminal", `{"class":"accepted-terminal","terminal":null}`},
		{"non-string-accepted-class", `{"class":1,"terminal":"successful-empty"}`},
		{"non-string-accepted-terminal", `{"class":"accepted-terminal","terminal":true}`},
		{"empty-accepted-class", `{"class":"","terminal":"successful-empty"}`},
		{"empty-accepted-terminal", `{"class":"accepted-terminal","terminal":""}`},
		{"case-accepted-class", `{"class":"Accepted-Terminal","terminal":"successful-empty"}`},
		{"case-accepted-terminal", `{"class":"accepted-terminal","terminal":"Successful-Empty"}`},
		{"whitespace-accepted-class", `{"class":" accepted-terminal","terminal":"successful-empty"}`},
		{"whitespace-accepted-terminal", `{"class":"accepted-terminal","terminal":"successful-empty "}`},
		{"unicode-accepted-class", `{"class":"accepted‐terminal","terminal":"successful-empty"}`},
		{"unicode-accepted-terminal", `{"class":"accepted-terminal","terminal":"successful‐empty"}`},
		{"accepted-unknown", `{"class":"accepted-terminal","terminal":"successful-empty","extra":true}`},
		{"accepted-duplicate-class", `{"class":"accepted-terminal","class":"accepted-terminal","terminal":"successful-empty"}`},
		{"accepted-duplicate-terminal", `{"class":"accepted-terminal","terminal":"successful-empty","terminal":"successful-empty"}`},
		{"accepted-conflicting-refusal-class", `{"class":"accepted-terminal","terminal":"successful-empty","refusal_class":"method-unavailable"}`},
		{"accepted-conflicting-refusal-reason", `{"class":"accepted-terminal","terminal":"successful-empty","refusal_reason":"method unavailable"}`},
		{"missing-refusal-class", `{"refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"missing-refusal-type", `{"class":"pre-effect-refusal","refusal_reason":"method unavailable"}`},
		{"missing-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable"}`},
		{"null-refusal-class", `{"class":null,"refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"null-refusal-type", `{"class":"pre-effect-refusal","refusal_class":null,"refusal_reason":"method unavailable"}`},
		{"null-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":null}`},
		{"non-string-refusal-class", `{"class":false,"refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"non-string-refusal-type", `{"class":"pre-effect-refusal","refusal_class":1,"refusal_reason":"method unavailable"}`},
		{"non-string-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":[]}`},
		{"empty-refusal-class", `{"class":"","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"empty-refusal-type", `{"class":"pre-effect-refusal","refusal_class":"","refusal_reason":"method unavailable"}`},
		{"empty-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":""}`},
		{"case-refusal-class", `{"class":"Pre-Effect-Refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"case-refusal-type", `{"class":"pre-effect-refusal","refusal_class":"Method-Unavailable","refusal_reason":"method unavailable"}`},
		{"case-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"Method unavailable"}`},
		{"whitespace-refusal-class", `{"class":"pre-effect-refusal ","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"whitespace-refusal-type", `{"class":"pre-effect-refusal","refusal_class":" method-unavailable","refusal_reason":"method unavailable"}`},
		{"whitespace-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable "}`},
		{"unicode-refusal-class", `{"class":"pre‐effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"unicode-refusal-type", `{"class":"pre-effect-refusal","refusal_class":"method‐unavailable","refusal_reason":"method unavailable"}`},
		{"unicode-refusal-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"refusal-unknown", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable","extra":true}`},
		{"refusal-duplicate-class", `{"class":"pre-effect-refusal","class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"refusal-duplicate-type", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"refusal-duplicate-reason", `{"class":"pre-effect-refusal","refusal_class":"method-unavailable","refusal_reason":"method unavailable","refusal_reason":"method unavailable"}`},
		{"refusal-conflicting-terminal", `{"class":"pre-effect-refusal","terminal":"successful-empty","refusal_class":"method-unavailable","refusal_reason":"method unavailable"}`},
		{"trailing", validAccepted + ` null`},
	} {
		t.Run("strict-"+test.name, func(t *testing.T) {
			outcome, err := decode(test.raw)
			if err == nil {
				_, err = callHierarchyLayer1GenericV3TypedCausalOutcome(outcome)
			}
			if err == nil {
				t.Fatalf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_INVALID_ACCEPTED raw=%s", test.raw)
			}
		})
	}
	for _, test := range []struct {
		name string
		raw  string
		want callHierarchyLayer1GenericCausalOutcome
	}{
		{"accepted-complete", validAccepted, callHierarchyLayer1GenericCausalOutcome{class: callHierarchyLayer1GenericCausalOutcomeAcceptedTerminal, terminal: "successful-empty"}},
		{"refusal-complete", validRefusal, callHierarchyLayer1GenericCausalOutcome{class: callHierarchyLayer1GenericCausalOutcomePreEffectRefusal, refusalType: callHierarchyLayer1GenericRefusalMethodUnavailable, refusalReason: session.Failure("method unavailable")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoded, err := decode(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := callHierarchyLayer1GenericV3TypedCausalOutcome(decoded)
			if err != nil || got != test.want {
				t.Fatalf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_COMPLETE_SHAPE got=%+v want=%+v err=%v", got, test.want, err)
			}
		})
	}

	worlds := callHierarchyLayer1GenericV3Load(t)
	if placementErrors := callHierarchyLayer1GenericV3CausalOutcomePlacementErrors(worlds); len(placementErrors) != 0 {
		t.Fatal(strings.Join(placementErrors, "\n"))
	}
	t.Run("placement-relocation-rejected", func(t *testing.T) {
		copyWorlds := append([]callHierarchyLayer1GenericV3Loaded(nil), worlds...)
		moved := copyWorlds[3].envelope.Driver.Request.CausalOutcome
		copyWorlds[3].envelope.Driver.Request.CausalOutcome = nil
		copyWorlds[0].envelope.Driver.Request.CausalOutcome = moved
		if len(callHierarchyLayer1GenericV3CausalOutcomePlacementErrors(copyWorlds)) == 0 {
			t.Fatal("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_RELOCATION_ACCEPTED")
		}
	})

	apply := func(t *testing.T, outcome *callHierarchyLayer1GenericV3DriverCausalOutcome) (callHierarchyLayer1GenericCausalOutcome, callHierarchyLayer1GenericCausalOutcome) {
		t.Helper()
		decoded, err := callHierarchyLayer1GenericV3TypedCausalOutcome(outcome)
		if err != nil {
			t.Fatal(err)
		}
		recorder := &callHierarchyLayer1GenericV3CausalOutcomeRecorder{}
		var setter callHierarchyLayer1GenericV3CausalOutcomeAttachment = recorder
		setter.privateCallHierarchyLayer1WithCausalOutcome(decoded)
		if recorder.calls != 1 {
			t.Fatalf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_SETTER_CALLS got=%d want=1", recorder.calls)
		}
		return decoded, recorder.outcome
	}
	mutations := []struct {
		name   string
		mutate func(*callHierarchyLayer1GenericV3Envelope)
	}{
		{"id", func(record *callHierarchyLayer1GenericV3Envelope) { record.Driver.Request.RequestID += 9000 }},
		{"address", func(record *callHierarchyLayer1GenericV3Envelope) { record.Driver.Request.Session = "mutated-address" }},
		{"world-label", func(record *callHierarchyLayer1GenericV3Envelope) { record.Expected.Name = "mutated-world" }},
		{"method", func(record *callHierarchyLayer1GenericV3Envelope) { record.Driver.Request.Method = "mutated-method" }},
		{"expected", func(record *callHierarchyLayer1GenericV3Envelope) {
			record.Expected.Terminal.Value = callHierarchyLayer1GenericV3NullableString("mutated-expected")
		}},
		{"response", func(record *callHierarchyLayer1GenericV3Envelope) {
			record.Driver.Derived.External.ResponsesBytes = callHierarchyLayer1GenericV3NullableString("mutated-response")
		}},
		{"error", func(record *callHierarchyLayer1GenericV3Envelope) {
			record.Expected.Error.Available = !record.Expected.Error.Available
		}},
		{"frame", func(record *callHierarchyLayer1GenericV3Envelope) {
			record.Expected.Frames.Response.Available = !record.Expected.Frames.Response.Available
		}},
		{"control", func(record *callHierarchyLayer1GenericV3Envelope) {
			if record.Expected.Control == nil {
				record.Expected.Control = &callHierarchyLayer1CorpusControl{}
			} else {
				control := *record.Expected.Control
				control.Enabled = !control.Enabled
				record.Expected.Control = &control
			}
		}},
		{"external", func(record *callHierarchyLayer1GenericV3Envelope) {
			record.Driver.Derived.External.ResultBytes = callHierarchyLayer1GenericV3NullableString("mutated-external")
		}},
	}
	for _, worldIndex := range []int{3, 4, 22} {
		record := worlds[worldIndex].envelope
		raw, err := json.Marshal(record.Driver.Request.CausalOutcome)
		wantRaw := validAccepted
		if worldIndex == 22 {
			wantRaw = validRefusal
		}
		if err != nil || string(raw) != wantRaw {
			t.Fatalf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_VERBATIM world=%s raw=%s want=%s err=%v", record.Expected.Name, raw, wantRaw, err)
		}
		baselineDecoded, baselineSetter := apply(t, record.Driver.Request.CausalOutcome)
		for _, mutation := range mutations {
			t.Run(record.Expected.Name+"-anti-dispatch-"+mutation.name, func(t *testing.T) {
				mutated := record
				mutation.mutate(&mutated)
				decoded, setter := apply(t, mutated.Driver.Request.CausalOutcome)
				if decoded != baselineDecoded || setter != baselineSetter || decoded != setter {
					t.Fatalf("ASSERT_CH_GENERIC_V3_CAUSAL_OUTCOME_ANTI_DISPATCH world=%s mutation=%s decoded=%+v/%+v setter=%+v/%+v", record.Expected.Name, mutation.name, baselineDecoded, decoded, baselineSetter, setter)
				}
			})
		}
	}
}

func TestCallHierarchyLayer1GenericV3CorpusCausalRED(t *testing.T) {
	expectedPaths := callHierarchyLayer1GenericV3ValidateExpectedSchema(t)
	ordinaryAPaths, world24APaths := callHierarchyLayer1GenericV3Partition(t, expectedPaths)
	worlds := callHierarchyLayer1GenericV3Load(t)
	callHierarchyLayer1GenericV3ValidateAttachmentMapping(t, worlds)
	if len(worlds) != 25 {
		t.Fatalf("ASSERT_CH_GENERIC_V3_ROOTS got=%d want=25", len(worlds))
	}
	if worlds[23].envelope.Expected.Concurrency == nil || len(worlds[23].envelope.Expected.Concurrency.Workers) != 16 {
		t.Fatal("ASSERT_CH_GENERIC_V3_W24_WORKERS")
	}
	if worlds[24].envelope.Expected.Comparison == nil || len(worlds[24].envelope.Expected.Comparison.Executions) != 22 || len(worlds[24].envelope.Expected.Comparison.Pairs) != 11 {
		t.Fatal("ASSERT_CH_GENERIC_V3_W25_EXECUTIONS_PAIRS")
	}
	var counters callHierarchyLayer1GenericV3Counters
	rootActuals := make([]callHierarchyLayer1GenericV3Actual, len(worlds))
	prerequisiteReason := ""
	recordActual := func(actual callHierarchyLayer1GenericV3Actual) bool {
		if actual.attempted {
			counters.attempted.Add(1)
		}
		if actual.executed {
			counters.executed.Add(1)
			return true
		}
		if actual.preEffectRefusal != "" {
			counters.refused.Add(1)
			if prerequisiteReason == "" {
				prerequisiteReason = actual.preEffectRefusal
			}
		}
		return false
	}
	recordPrerequisite := func(observation callHierarchyLayer1GenericV3Observation) {
		if !observation.available && prerequisiteReason == "" {
			prerequisiteReason = observation.reason
		}
	}
	for i, world := range worlds {
		i, world := i, world
		t.Run(world.entry.Name, func(t *testing.T) {
			counters.roots.Add(1)
			rootActuals[i] = callHierarchyLayer1GenericV3Execute(t, world.envelope)
			if recordActual(rootActuals[i]) {
				paths := ordinaryAPaths
				if i == 23 {
					paths = world24APaths
				}
				recordPrerequisite(callHierarchyLayer1GenericV3AssertActual(t, world.envelope, rootActuals[i], paths))
			}
		})
	}

	w24 := worlds[23].envelope.Expected.Concurrency.Workers
	t.Run("world-24-actual-workers-and-aggregate", func(t *testing.T) {
		var wg sync.WaitGroup
		observed := make([]callHierarchyLayer1GenericV3Actual, len(w24))
		for i := range w24 {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				observed[i] = callHierarchyLayer1GenericV3Execute(t, w24[i])
				counters.workers.Add(1)
			}()
		}
		wg.Wait()
		for i := range w24 {
			if recordActual(observed[i]) {
				recordPrerequisite(callHierarchyLayer1GenericV3AssertActual(t, w24[i], observed[i], ordinaryAPaths))
			}
		}
		callHierarchyLayer1GenericV3AssertWorld24Aggregate(t, worlds[23].envelope, w24, observed)
	})

	w25 := worlds[24].envelope.Expected.Comparison
	t.Run("world-25-actual-executions-and-pairs", func(t *testing.T) {
		for _, execution := range w25.Executions {
			actual := callHierarchyLayer1GenericV3Execute(t, execution)
			if recordActual(actual) {
				recordPrerequisite(callHierarchyLayer1GenericV3AssertActual(t, execution, actual, ordinaryAPaths))
			}
			counters.executions.Add(1)
		}
		for _, pair := range w25.Pairs {
			disabled := callHierarchyLayer1GenericV3Execute(t, pair.Disabled)
			enabled := callHierarchyLayer1GenericV3Execute(t, pair.Enabled)
			disabledExecuted, enabledExecuted := recordActual(disabled), recordActual(enabled)
			counters.pairs.Add(2)
			if !disabledExecuted || !enabledExecuted {
				continue
			}
			recordPrerequisite(callHierarchyLayer1GenericV3AssertActual(t, pair.Disabled, disabled, ordinaryAPaths))
			recordPrerequisite(callHierarchyLayer1GenericV3AssertActual(t, pair.Enabled, enabled, ordinaryAPaths))
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
			callHierarchyLayer1GenericV3Equal(t, "world-25", "comparison.transport_equal", transportEqual, pair.TransportEqual)
			if pair.TransportEqual {
				callHierarchyLayer1GenericV3Equal(t, "world-25", "comparison.transport_equal.actual", disabledTransport, enabledTransport)
			}
			externalEqual := reflect.DeepEqual(disabled.external, enabled.external)
			callHierarchyLayer1GenericV3Equal(t, "world-25", "comparison.external_equal", externalEqual, pair.ExternalEqual)
			if pair.ExternalEqual {
				callHierarchyLayer1GenericV3Equal(t, "world-25", "comparison.external_equal.actual", disabled.external, enabled.external)
			}
		}
	})
	total := counters.roots.Load() + counters.workers.Load() + counters.executions.Load() + counters.pairs.Load()
	t.Logf("ASSERT_CH_GENERIC_V3_EXECUTION_COUNTERS roots=%d workers=%d executions=%d pair_members=%d total=%d attempted=%d executed=%d pre_effect_refused=%d", counters.roots.Load(), counters.workers.Load(), counters.executions.Load(), counters.pairs.Load(), total, counters.attempted.Load(), counters.executed.Load(), counters.refused.Load())
	if counters.roots.Load() != 25 || counters.workers.Load() != 16 || counters.executions.Load() != 22 || counters.pairs.Load() != 22 || total != 85 || counters.attempted.Load() != 85 || counters.executed.Load()+counters.refused.Load() != 85 {
		t.Fatal("ASSERT_CH_GENERIC_V3_EXECUTION_COUNTER_MISMATCH")
	}
	if prerequisiteReason != "" {
		if prerequisiteReason != "request-attachment-method:privateCallHierarchyLayer1WithIdentity" || counters.attempted.Load() != 85 || counters.executed.Load() != 0 || counters.refused.Load() != 85 {
			t.Fatalf("ASSERT_CH_GENERIC_V3_ATTACHMENT_PREREQUISITE_CROSS_RULE attempted=%d executed=%d pre_effect_refused=%d reason=%s", counters.attempted.Load(), counters.executed.Load(), counters.refused.Load(), prerequisiteReason)
		}
		t.Errorf("ASSERT_CH_GENERIC_V3_ATTACHMENT_PREREQUISITE attempted=85 executed=0 pre_effect_refused=85 contract=RoundTripRequest.callHierarchyLayer1.privateCallHierarchyLayer1WithIdentity reason=%s", prerequisiteReason)
		return
	}
	callHierarchyLayer1GenericV3AssertLifecycle(t, worlds[0].envelope, &rootActuals[0].result)
}

var callHierarchyLayer1GenericV3ErrorType = reflect.TypeOf((*error)(nil)).Elem()

func callHierarchyLayer1GenericV3LifecycleMethods(observation callHierarchyLayer1GenericV3Observation) (reflect.Value, reflect.Value, bool) {
	callback := observation.lease.MethodByName(callHierarchyLayer1GenericV3CallbackMethod)
	release := observation.lease.MethodByName(callHierarchyLayer1GenericV3ReleaseMethod)
	validCallback := callback.IsValid() && callback.Type().NumIn() == 1 && callback.Type().In(0).Kind() == reflect.Func && callback.Type().NumOut() == 1 && callback.Type().Out(0).Implements(callHierarchyLayer1GenericV3ErrorType)
	validRelease := release.IsValid() && release.Type().NumIn() == 0 && release.Type().NumOut() == 1 && release.Type().Out(0).Kind() == reflect.Bool
	return callback, release, validCallback && validRelease
}

func callHierarchyLayer1GenericV3Callback(t *testing.T, callback reflect.Value, body func([]reflect.Value) error) (returned error, escaped any, calls int) {
	t.Helper()
	fnType := callback.Type().In(0)
	if fnType.NumOut() != 1 || !fnType.Out(0).Implements(callHierarchyLayer1GenericV3ErrorType) {
		t.Fatalf("ASSERT_CH_GENERIC_V3_CALLBACK_SIGNATURE callback=%s", fnType)
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

func callHierarchyLayer1GenericV3FirstBytes(value reflect.Value) ([]byte, bool) {
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
			if b, ok := callHierarchyLayer1GenericV3FirstBytes(value.Field(i)); ok {
				return b, true
			}
		}
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		for i := 0; i < value.Len(); i++ {
			if b, ok := callHierarchyLayer1GenericV3FirstBytes(value.Index(i)); ok {
				return b, true
			}
		}
	}
	return nil, false
}

func callHierarchyLayer1GenericV3FreshLifecycle(t *testing.T, record callHierarchyLayer1GenericV3Envelope) (callHierarchyLayer1GenericV3Actual, callHierarchyLayer1GenericV3Observation, reflect.Value, reflect.Value, bool) {
	t.Helper()
	actual := callHierarchyLayer1GenericV3Execute(t, record)
	observation := callHierarchyLayer1GenericV3Observe(&actual.result)
	if !observation.available {
		return actual, observation, reflect.Value{}, reflect.Value{}, false
	}
	callback, release, ok := callHierarchyLayer1GenericV3LifecycleMethods(observation)
	return actual, observation, callback, release, ok
}

func callHierarchyLayer1GenericV3AssertLifecycle(t *testing.T, record callHierarchyLayer1GenericV3Envelope, result *RoundTripResult) {
	t.Helper()
	observation := callHierarchyLayer1GenericV3Observe(result)
	if !observation.available {
		t.Errorf("ASSERT_CH_GENERIC_V3_LIFECYCLE_PREREQUISITE contract=RoundTripResult.%s/View/Callback/Release reason=%s", callHierarchyLayer1GenericV3AcquireMethod, observation.reason)
		return
	}
	callback, release, ok := callHierarchyLayer1GenericV3LifecycleMethods(observation)
	if !ok {
		t.Errorf("ASSERT_CH_GENERIC_V3_LIFECYCLE_PREREQUISITE contract=Callback(func(view)error)error+Release()bool")
		return
	}

	if source, ok := callHierarchyLayer1GenericV3FirstBytes(observation.view); ok {
		before := append([]byte(nil), source...)
		source[0] ^= 0xff
		again := callHierarchyLayer1GenericV3Observe(result)
		reacquired, reacquiredOK := callHierarchyLayer1GenericV3FirstBytes(again.view)
		if !again.available || !reacquiredOK || !bytes.Equal(reacquired, before) {
			t.Errorf("ASSERT_CH_GENERIC_V3_LIFECYCLE_OUTPUT_COPY_MUTATION_REACQUIRE")
		}
	}

	sentinel := errors.New("generic-v3-callback-error")
	returned, escaped, calls := callHierarchyLayer1GenericV3Callback(t, callback, func([]reflect.Value) error { return nil })
	if escaped != nil || returned != nil || calls != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V3_CALLBACK_SUCCESS returned=%v escaped=%v calls=%d", returned, escaped, calls)
	}
	returned, escaped, calls = callHierarchyLayer1GenericV3Callback(t, callback, func([]reflect.Value) error { return sentinel })
	if escaped != nil || !errors.Is(returned, sentinel) || calls != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V3_CALLBACK_ERROR returned=%v escaped=%v calls=%d", returned, escaped, calls)
	}
	returned, escaped, calls = callHierarchyLayer1GenericV3Callback(t, callback, func([]reflect.Value) error { panic("generic-v3-callback-panic") })
	if escaped != nil || returned == nil || calls != 1 {
		t.Errorf("ASSERT_CH_GENERIC_V3_CALLBACK_PANIC_CONTAINED returned=%v escaped=%v calls=%d", returned, escaped, calls)
	}
	if !release.Call(nil)[0].Bool() || release.Call(nil)[0].Bool() {
		t.Errorf("ASSERT_CH_GENERIC_V3_LIFECYCLE_RELEASE_AFTER_CALLBACKS")
	}
	if callHierarchyLayer1GenericV3Observe(result).available {
		t.Errorf("ASSERT_CH_GENERIC_V3_LIFECYCLE_TERMINAL_AFTER_CALLBACKS")
	}

	sequentialActual, sequential, _, sequentialRelease, sequentialOK := callHierarchyLayer1GenericV3FreshLifecycle(t, record)
	if !sequential.available || !sequentialOK {
		t.Errorf("ASSERT_CH_GENERIC_V3_SEQUENTIAL_RELEASE_PREREQUISITE")
		return
	}
	first, second := sequentialRelease.Call(nil)[0].Bool(), sequentialRelease.Call(nil)[0].Bool()
	if !first || second {
		t.Errorf("ASSERT_CH_GENERIC_V3_SEQUENTIAL_RELEASE first=%t second=%t", first, second)
	}
	if callHierarchyLayer1GenericV3Observe(&sequentialActual.result).available {
		t.Errorf("ASSERT_CH_GENERIC_V3_SEQUENTIAL_RELEASE_TERMINAL")
	}

	concurrentActual, concurrent, concurrentCallback, concurrentRelease, concurrentOK := callHierarchyLayer1GenericV3FreshLifecycle(t, record)
	if !concurrent.available || !concurrentOK {
		t.Errorf("ASSERT_CH_GENERIC_V3_CONCURRENT_RELEASE_PREREQUISITE")
		return
	}
	_, escaped, calls = callHierarchyLayer1GenericV3Callback(t, concurrentCallback, func([]reflect.Value) error { return nil })
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
		t.Errorf("ASSERT_CH_GENERIC_V3_CONCURRENT_RELEASE_EXACT winners=%d losers=%d callback_calls=%d escaped=%v", winners, racers-winners, calls, escaped)
	}
	if callHierarchyLayer1GenericV3Observe(&concurrentActual.result).available {
		t.Errorf("ASSERT_CH_GENERIC_V3_CONCURRENT_RELEASE_TERMINAL")
	}

	drainActual, drain, drainCallback, drainRelease, drainOK := callHierarchyLayer1GenericV3FreshLifecycle(t, record)
	if !drain.available || !drainOK {
		t.Errorf("ASSERT_CH_GENERIC_V3_DRAIN_PREREQUISITE")
		return
	}
	entered, unblock := make(chan struct{}), make(chan struct{})
	callbackDone, releaseDone := make(chan struct{}), make(chan bool, 1)
	go func() {
		defer close(callbackDone)
		returned, escaped, calls := callHierarchyLayer1GenericV3Callback(t, drainCallback, func([]reflect.Value) error { close(entered); <-unblock; return nil })
		if returned != nil || escaped != nil || calls != 1 {
			t.Errorf("ASSERT_CH_GENERIC_V3_DRAIN_CALLBACK returned=%v escaped=%v calls=%d", returned, escaped, calls)
		}
	}()
	<-entered
	go func() { releaseDone <- drainRelease.Call(nil)[0].Bool() }()
	select {
	case won := <-releaseDone:
		t.Errorf("ASSERT_CH_GENERIC_V3_DRAIN_RELEASE_EARLY result=%t", won)
	case <-time.After(20 * time.Millisecond):
	}
	close(unblock)
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("ASSERT_CH_GENERIC_V3_DRAIN_CALLBACK_DEADLOCK")
	}
	select {
	case won := <-releaseDone:
		if !won {
			t.Errorf("ASSERT_CH_GENERIC_V3_DRAIN_RELEASE_RESULT")
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_CH_GENERIC_V3_DRAIN_RELEASE_DEADLOCK")
	}
	if callHierarchyLayer1GenericV3Observe(&drainActual.result).available {
		t.Errorf("ASSERT_CH_GENERIC_V3_DRAIN_TERMINAL")
	}
}
