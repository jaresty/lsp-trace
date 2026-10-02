package adr0011acquisition

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// Private, unregistered custody only. No terminal, provider receipt, or public admission.
var (
	errProductionAttemptCustody          = errors.New("production attempt custody unavailable")
	errProductionAttemptWriteUnavailable = errors.New("production attempt exact completed write frame unavailable")
)

const (
	productionAttemptLimit        = 4194304
	productionAttemptSchema       = "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/"
	productionAttemptSchemaDigest = "sha256:2f32bbca88585a7a5238f1c6875abe2607519551eb505e5ecd646a647f76f879"
	custodyRole                   = "ADR0011_ATTEMPT_HOST_CUSTODY_V1"
	keyObservationRole            = "ADR0011_ATTEMPT_REQUEST_KEY_OBSERVATION_V1"
)

type productionAttemptRef struct {
	Role          string `json:"role"`
	SchemaVersion string `json:"schema_version"`
	Selector      string `json:"selector"`
	Digest        string `json:"digest"`
	ByteLength    int    `json:"byte_length"`
}
type productionAttemptPrewrite struct {
	PolicyRef           productionAttemptRef
	DeclarationRef      productionAttemptRef
	PolicyOriginal      []byte
	DeclarationOriginal []byte
	SchemaOriginal      []byte // Host-supplied complete contract; exact digest is pinned below.
	SessionID           string
	Generation          uint64
	Invocation          string
	// No request-key field exists before invocation.
}

// Caller must retain this separately from the claimant's mutable prewrite input.
// A test-created value demonstrates substitution resistance, not production authority.
type productionAttemptHostSelection struct {
	Selected         productionAttemptPrewrite
	ExpectedOriginal []byte
	ExpectedRef      productionAttemptRef
}

func productionAttemptHostMatches(root *publication.Root, claimed productionAttemptPrewrite, host productionAttemptHostSelection, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	selected, err := productionAttemptExpected(root, host.Selected, read)
	if err != nil || !bytes.Equal(selected, host.ExpectedOriginal) {
		return false
	}
	claimedBytes, err := productionAttemptExpected(root, claimed, read)
	if err != nil || !bytes.Equal(claimedBytes, host.ExpectedOriginal) || !bytes.Equal(claimed.PolicyOriginal, host.Selected.PolicyOriginal) || !bytes.Equal(claimed.DeclarationOriginal, host.Selected.DeclarationOriginal) || !bytes.Equal(claimed.SchemaOriginal, host.Selected.SchemaOriginal) {
		return false
	}
	ref := host.ExpectedRef
	return ref.Role == "attempt_host_custody" && ref.SchemaVersion == productionAttemptSchema+"attemptHostCustody" && ref.Selector == "adr0011-attempt_host_custody-v2-"+strings.TrimPrefix(privateDigest(host.ExpectedOriginal), "sha256:")+".json" && ref.Digest == privateDigest(host.ExpectedOriginal) && ref.ByteLength == len(host.ExpectedOriginal)
}

type productionAttemptPublication struct {
	selector, digest, stage string
	byteCount               int
}

func (p productionAttemptPublication) ref(role, schema string) productionAttemptRef {
	return productionAttemptRef{role, schema, p.selector, p.digest, p.byteCount}
}
func exactProductionRef(root *publication.Root, ref productionAttemptRef, role, schema string, original []byte, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	if root == nil || ref.Role != role || ref.SchemaVersion != schema || ref.Selector == "" || ref.Digest != privateDigest(original) || ref.ByteLength != len(original) || len(original) == 0 || len(original) > productionAttemptLimit {
		return false
	}
	got, err := read(root, ref.Selector, productionAttemptLimit)
	return err == nil && bytes.Equal(got, original) && privateDigest(got) == ref.Digest
}
func productionAttemptRead(read func(*publication.Root, string, int64) ([]byte, error)) func(*publication.Root, string, int64) ([]byte, error) {
	if read != nil {
		return read
	}
	return publication.ReadVerifiedBoundFile
}
func validProductionOriginal(schemaBytes, original []byte, definition string) bool {
	if privateDigest(schemaBytes) != productionAttemptSchemaDigest || strictjson.RejectDuplicates(schemaBytes) != nil || strictjson.RejectDuplicates(original) != nil {
		return false
	}
	var schema any
	if json.Unmarshal(schemaBytes, &schema) != nil {
		return false
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if compiler.AddResource(strings.TrimSuffix(productionAttemptSchema, "#/$defs/"), schema) != nil {
		return false
	}
	shape, err := compiler.Compile(productionAttemptSchema + definition)
	if err != nil {
		return false
	}
	var value any
	return json.Unmarshal(original, &value) == nil && shape.Validate(value) == nil
}
func productionAttemptExpected(root *publication.Root, pre productionAttemptPrewrite, read func(*publication.Root, string, int64) ([]byte, error)) ([]byte, error) {
	read = productionAttemptRead(read)
	if len(pre.Invocation) != len("sha256:")+64 || !strings.HasPrefix(pre.Invocation, "sha256:") {
		return nil, errProductionAttemptCustody
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(pre.Invocation, "sha256:")); err != nil {
		return nil, errProductionAttemptCustody
	}
	if !validProductionOriginal(pre.SchemaOriginal, pre.PolicyOriginal, "attemptWitnessPolicy") || !validProductionOriginal(pre.SchemaOriginal, pre.DeclarationOriginal, "productionQueryDeclaration") || pre.SessionID == "" || pre.Generation == 0 || pre.Invocation == "" || len(pre.Invocation) != len("sha256:")+64 || !strings.HasPrefix(pre.Invocation, "sha256:") || !exactProductionRef(root, pre.PolicyRef, "attempt_witness_policy", productionAttemptSchema+"attemptWitnessPolicy", pre.PolicyOriginal, read) || !exactProductionRef(root, pre.DeclarationRef, "production_query_declaration", productionAttemptSchema+"productionQueryDeclaration", pre.DeclarationOriginal, read) {
		return nil, errProductionAttemptCustody
	}
	// Bind the independently selected policy's declaration and declaration's session.
	var policy struct {
		DeclarationRef productionAttemptRef `json:"declaration_ref"`
		SelectedRoles  []string             `json:"selected_roles"`
	}
	var declaration struct {
		SessionID       string `json:"session_id"`
		Generation      uint64 `json:"generation"`
		InvocationNonce string `json:"invocation_nonce"`
	}
	if json.Unmarshal(pre.PolicyOriginal, &policy) != nil || json.Unmarshal(pre.DeclarationOriginal, &declaration) != nil || policy.DeclarationRef != pre.DeclarationRef || declaration.SessionID != pre.SessionID || declaration.Generation != pre.Generation || declaration.InvocationNonce == "" {
		return nil, errProductionAttemptCustody
	}
	// The selected originals, not an independently supplied label, determine each digest.
	return json.Marshal(struct {
		Role                      string               `json:"role"`
		Version                   int                  `json:"version"`
		PolicyRef                 productionAttemptRef `json:"policy_ref"`
		DeclarationRef            productionAttemptRef `json:"declaration_ref"`
		InvocationID              string               `json:"invocation_id"`
		HostSessionID             string               `json:"host_session_id"`
		HostGeneration            uint64               `json:"host_generation"`
		SelectionPhase            string               `json:"selection_phase"`
		PolicyReadbackDigest      string               `json:"policy_readback_digest"`
		DeclarationReadbackDigest string               `json:"declaration_readback_digest"`
	}{custodyRole, 1, pre.PolicyRef, pre.DeclarationRef, pre.Invocation, pre.SessionID, pre.Generation, "PRE_INVOCATION", privateDigest(pre.PolicyOriginal), privateDigest(pre.DeclarationOriginal)})
}
func publishProductionAttemptBytes(root *publication.Root, role string, raw []byte, read func(*publication.Root, string, int64) ([]byte, error)) (productionAttemptPublication, error) {
	out := productionAttemptPublication{stage: "ABSENT"}
	if root == nil || (role != "attempt_host_custody" && role != "attempt_request_key_observation" && role != "attempt_write_frame") || len(raw) == 0 || len(raw) > productionAttemptLimit {
		return out, errProductionAttemptCustody
	}
	digest := privateDigest(raw)
	extension := ".json"
	if role == "attempt_write_frame" {
		extension = ".bin"
	}
	selector := "adr0011-" + role + "-v2-" + strings.TrimPrefix(digest, "sha256:") + extension
	installed := false
	trace := func(e publication.BoundFileTraceEvent) {
		if e.Stage == "HARDLINK" && e.Result == "INSTALLED" && e.OK {
			installed = true
		}
	}
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errProductionAttemptCustody
		}
		return nil
	}, trace)
	if receipt != nil || installed {
		out = productionAttemptPublication{selector: selector, digest: digest, byteCount: len(raw), stage: "COMMITTED_UNVERIFIED"}
	}
	if err != nil || receipt == nil {
		return out, errProductionAttemptCustody
	}
	if receipt.VerificationStatus != "VERIFIED" || receipt.Mechanism != publication.BoundFileMechanism || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.FinalSelector != selector || receipt.Digest != digest || receipt.ByteLength != uint64(len(raw)) || !receipt.NamespaceAtomic {
		return out, errProductionAttemptCustody
	}
	got, err := productionAttemptRead(read)(root, selector, productionAttemptLimit)
	if err != nil || !bytes.Equal(got, raw) || privateDigest(got) != digest {
		return out, errProductionAttemptCustody
	}
	out.stage = "VERIFIED"
	return out, nil
}
func publishProductionAttemptPrewrite(root *publication.Root, pre productionAttemptPrewrite, host productionAttemptHostSelection, read func(*publication.Root, string, int64) ([]byte, error)) (productionAttemptPublication, error) {
	if !productionAttemptHostMatches(root, pre, host, read) {
		return productionAttemptPublication{stage: "ABSENT"}, errProductionAttemptCustody
	}
	return publishProductionAttemptBytes(root, "attempt_host_custody", host.ExpectedOriginal, read)
}

// The manager's request frame is intentionally unavailable after a WRITE without
// successful keyed D/R. Such attempts cannot produce a verified observation here.
func observeProductionAttemptWrite(root *publication.Root, pre productionAttemptPrewrite, host productionAttemptHostSelection, expected productionAttemptPublication, result sessionruntime.RoundTripResult, read func(*publication.Root, string, int64) ([]byte, error)) (productionAttemptPublication, error) {
	absent := productionAttemptPublication{stage: "ABSENT"}
	if expected.stage != "VERIFIED" || !productionAttemptHostMatches(root, pre, host, read) || expected.ref("attempt_host_custody", productionAttemptSchema+"attemptHostCustody") != host.ExpectedRef {
		return absent, errProductionAttemptCustody
	}
	write, ok := result.CompletedRequestWrite()
	if !ok {
		return absent, errProductionAttemptWriteUnavailable
	}
	frame, ok := result.CompletedMethodRequestFrame()
	if !ok {
		return absent, errProductionAttemptWriteUnavailable
	}
	rawPre := host.ExpectedOriginal
	if expected.digest != privateDigest(rawPre) || expected.byteCount != len(rawPre) || expected.selector != "adr0011-attempt_host_custody-v2-"+strings.TrimPrefix(expected.digest, "sha256:")+".json" {
		return absent, errProductionAttemptCustody
	}
	held, err := productionAttemptRead(read)(root, expected.selector, productionAttemptLimit)
	if err != nil || !bytes.Equal(held, rawPre) {
		return absent, errProductionAttemptCustody
	}
	body, ok := exactFrameBody(frame)
	if !ok || strictjson.RejectDuplicates(body) != nil || len(frame) > productionAttemptLimit {
		return absent, errProductionAttemptCustody
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return absent, errProductionAttemptCustody
	}
	var method string
	var wireID uint64
	if json.Unmarshal(fields["method"], &method) != nil || json.Unmarshal(fields["id"], &wireID) != nil || len(fields["params"]) == 0 || method != "textDocument/references" {
		return absent, errProductionAttemptCustody
	}
	if write.SessionID != pre.SessionID || write.Generation != pre.Generation || write.Key != result.Key || write.Key.ID == 0 || write.Key.Generation != pre.Generation || write.Method != method || wireID != write.Key.ID || write.FrameBytes != int64(len(frame)) || write.FrameSHA256 != privateDigest(frame) {
		return absent, errProductionAttemptCustody
	}
	framePublished, err := publishProductionAttemptBytes(root, "attempt_write_frame", frame, read)
	if err != nil || framePublished.stage != "VERIFIED" {
		return absent, errProductionAttemptCustody
	}
	observation, err := json.Marshal(struct {
		Role             string               `json:"role"`
		Version          int                  `json:"version"`
		PolicyRef        productionAttemptRef `json:"policy_ref"`
		DeclarationID    string               `json:"declaration_id"`
		CustodyRef       productionAttemptRef `json:"custody_ref"`
		RequestKey       string               `json:"request_key"`
		InvocationID     string               `json:"invocation_id"`
		HostSessionID    string               `json:"host_session_id"`
		HostGeneration   uint64               `json:"host_generation"`
		WireID           uint64               `json:"wire_id"`
		WriteSelector    string               `json:"write_selector"`
		WriteDigest      string               `json:"write_digest"`
		WriteByteLength  int                  `json:"write_byte_length"`
		ObservationPhase string               `json:"observation_phase"`
	}{keyObservationRole, 1, pre.PolicyRef, pre.DeclarationRef.Digest, expected.ref("attempt_host_custody", productionAttemptSchema+"attemptHostCustody"), adr0011requestkey.Encode(write.Key), pre.Invocation, pre.SessionID, pre.Generation, wireID, framePublished.selector, framePublished.digest, framePublished.byteCount, "COMPLETED_FRAMED_WRITE"})
	if err != nil {
		return absent, errProductionAttemptCustody
	}
	return publishProductionAttemptBytes(root, "attempt_request_key_observation", observation, read)
}
