package describeworker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
)

const InvocationSchemaV2 = "lsp-trace.describe-invocation.v2"

type invocationReceiptV2 struct {
	ResourceReceipt
	StdoutSHA256 string `json:"stdout_sha256"`
	StderrSHA256 string `json:"stderr_sha256"`
}

type invocationWireV2 struct {
	SchemaVersion  string              `json:"schema_version"`
	InvocationID   string              `json:"invocation_id"`
	Binding        IdentityBinding     `json:"binding"`
	TerminalStatus TerminalStatus      `json:"terminal_status"`
	Receipt        invocationReceiptV2 `json:"receipt"`
	FailureSubcode string              `json:"failure_subcode"`
	Response       *responseWireV2     `json:"response"`
	Authority      int                 `json:"authority"`
	Accepted       bool                `json:"accepted"`
	Completeness   string              `json:"completeness"`
}

type InvocationRecordV2 struct{ wire invocationWireV2 }

func NewInvocationRecordV2(binding IdentityBinding, status TerminalStatus, receipt ResourceReceipt, response ResponseRecordV2) (InvocationRecordV2, error) {
	record, err := newInvocationRecordV2(binding, status, receipt, nil, nil, "", response)
	if err != nil || response.ID() == "" {
		return record, err
	}
	record.wire.Receipt.StdoutSHA256 = response.RawStdoutDigest()
	record.wire.InvocationID = identity("lsp-trace.describe-invocation.identity.v2", record.wire, func(v *invocationWireV2) { v.InvocationID = "" })
	return record, record.Validate()
}

func NewFailedInvocationRecordV2(binding IdentityBinding, status TerminalStatus, receipt ResourceReceipt, stdout, stderr []byte, subcode string) (InvocationRecordV2, error) {
	if status == StatusSucceeded {
		return InvocationRecordV2{}, errors.New("failed invocation v2 status required")
	}
	return newInvocationRecordV2(binding, status, receipt, stdout, stderr, subcode, ResponseRecordV2{})
}

func newInvocationRecordV2(binding IdentityBinding, status TerminalStatus, receipt ResourceReceipt, stdout, stderr []byte, subcode string, response ResponseRecordV2) (InvocationRecordV2, error) {
	stdoutSum, stderrSum := sha256.Sum256(stdout), sha256.Sum256(stderr)
	wire := invocationWireV2{
		SchemaVersion: InvocationSchemaV2, Binding: cloneBinding(binding), TerminalStatus: status,
		Receipt:        invocationReceiptV2{ResourceReceipt: receipt, StdoutSHA256: "sha256:" + hex.EncodeToString(stdoutSum[:]), StderrSHA256: "sha256:" + hex.EncodeToString(stderrSum[:])},
		FailureSubcode: subcode, Completeness: CompletenessUnknown,
	}
	if response.ID() != "" {
		copy := response.wire
		wire.Response = &copy
	}
	wire.InvocationID = identity("lsp-trace.describe-invocation.identity.v2", wire, func(v *invocationWireV2) { v.InvocationID = "" })
	record := InvocationRecordV2{wire: wire}
	return record, record.Validate()
}

func ParseInvocationRecordV2(raw []byte) (InvocationRecordV2, error) {
	var wire invocationWireV2
	if err := strictCanonical(raw, &wire); err != nil {
		return InvocationRecordV2{}, err
	}
	record := InvocationRecordV2{wire: wire}
	return record, record.Validate()
}

var invocationDigestV2 = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func (r InvocationRecordV2) Validate() error {
	wire := r.wire
	want := identity("lsp-trace.describe-invocation.identity.v2", wire, func(v *invocationWireV2) { v.InvocationID = "" })
	if wire.SchemaVersion != InvocationSchemaV2 || wire.InvocationID != want || wire.Authority != 0 || wire.Accepted || wire.Completeness != CompletenessUnknown || !validStatus(wire.TerminalStatus) || wire.Receipt.TerminalOutcomes != 1 || !invocationDigestV2.MatchString(wire.Receipt.StdoutSHA256) || !invocationDigestV2.MatchString(wire.Receipt.StderrSHA256) {
		return errors.New("describe invocation v2 integrity mismatch")
	}
	if wire.Receipt.StdoutBytes == 0 && wire.Receipt.StdoutSHA256 != emptyStreamSHA256V2 || wire.Receipt.StderrBytes == 0 && wire.Receipt.StderrSHA256 != emptyStreamSHA256V2 {
		return errors.New("describe invocation v2 stream custody mismatch")
	}
	if !validInvocationSubcodeV2(wire.TerminalStatus, wire.FailureSubcode) {
		return errors.New("describe invocation v2 failure subcode mismatch")
	}
	if err := validateBinding(wire.Binding, true); err != nil {
		return err
	}
	if wire.TerminalStatus == StatusSucceeded {
		if wire.Response == nil {
			return errors.New("successful invocation v2 response missing")
		}
		response := ResponseRecordV2{wire: *wire.Response}
		if err := response.Validate(); err != nil {
			return err
		}
		host := response.Host()
		if host.RequestRecordID != wire.Binding.RequestRecordID || host.MessageID != wire.Binding.MessageID || host.AttemptID != wire.Binding.AttemptID || host.Pins.WorkerSHA256 != wire.Binding.WorkerSHA256 || host.Pins.ModelSHA256 != wire.Binding.ModelSHA256 || host.Pins.GrammarSHA256 != wire.Binding.GrammarSHA256 || host.Pins.PromptSHA256 != wire.Binding.PromptSHA256 {
			return errors.New("invocation v2 response binding mismatch")
		}
		return nil
	}
	if wire.Response != nil {
		return errors.New("failed invocation v2 must not contain response")
	}
	return nil
}

const emptyStreamSHA256V2 = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func validInvocationSubcodeV2(status TerminalStatus, subcode string) bool {
	if status == StatusSucceeded || status == StatusCancelled || status == StatusTimeout || status == StatusResourceLimit {
		return subcode == ""
	}
	allowed := map[string]bool{
		subcodeOuterProtocolV2: true, subcodeSemanticSyntaxV2: true, subcodeSemanticSchemaV2: true,
		subcodeSemanticValueV2: true, subcodeHostBindingV2: true, subcodeConsumerResolutionV2: true,
		subcodeInternalValidationV2: true, "SUPERVISION": true, "ADAPTER_PROTOCOL": true, "MODEL_LOAD": true,
		"PROMPT_LIMIT": true, "GRAMMAR": true, "OUTPUT_PARSE": true,
	}
	return allowed[subcode]
}

func (r InvocationRecordV2) ID() string               { return r.wire.InvocationID }
func (r InvocationRecordV2) Status() TerminalStatus   { return r.wire.TerminalStatus }
func (r InvocationRecordV2) Receipt() ResourceReceipt { return r.wire.Receipt.ResourceReceipt }
func (r InvocationRecordV2) FailureSubcode() string   { return r.wire.FailureSubcode }
func (r InvocationRecordV2) StdoutSHA256() string     { return r.wire.Receipt.StdoutSHA256 }
func (r InvocationRecordV2) StderrSHA256() string     { return r.wire.Receipt.StderrSHA256 }
func (r InvocationRecordV2) Binding() IdentityBinding { return cloneBinding(r.wire.Binding) }
func (r InvocationRecordV2) Response() ResponseRecordV2 {
	if r.wire.Response == nil {
		return ResponseRecordV2{}
	}
	return ResponseRecordV2{wire: *r.wire.Response}
}
func (r InvocationRecordV2) Bytes() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r.wire)
}
