package censuscontinuation

import (
	"encoding/json"
	"errors"

	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

const ContractSchema = "lsp-trace.census-continuation-contract.v1"
const MaxCaptureBytes = 64 << 20

func ProductionCaptureLimits() v5sourcesnapshotv3.Limits {
	return v5sourcesnapshotv3.Limits{MaxArtifactBytes: MaxCaptureBytes, MaxParentBytes: MaxCaptureBytes, MaxReceipts: 1000, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}
}

type RetryPolicy struct {
	MaxAttemptsPerRequest int `json:"max_attempts_per_request"`
}

type SchemaVersions struct {
	Handoff   string `json:"handoff"`
	ProgramC  string `json:"program_c"`
	Snapshots string `json:"snapshots"`
	Packets   string `json:"packets"`
	Requests  string `json:"requests"`
	Worker    string `json:"worker"`
	Catalog   string `json:"catalog"`
}

type ContractInput struct {
	ContinuationID         string
	ProfileID              string
	WorkerPinID            string
	ProgramCSeed           uint64
	CaptureLimits          v5sourcesnapshotv3.Limits
	PacketSourcePolicy     sourceprojection.Policy
	PacketResolveLimits    retainedprojection.ResolveLimits
	PacketMaxResponseBytes int
	DescribeDeadlineMS     int
	SchemaVersions         SchemaVersions
	RetryPolicy            RetryPolicy
	ResponseVersion        string
}

type contractWire struct {
	SchemaVersion          string                           `json:"schema_version"`
	ContractID             string                           `json:"contract_id"`
	ContinuationID         string                           `json:"continuation_id"`
	ProfileID              string                           `json:"profile_id"`
	WorkerPinID            string                           `json:"worker_pin_id"`
	ProgramCSeed           uint64                           `json:"program_c_seed"`
	CaptureLimits          v5sourcesnapshotv3.Limits        `json:"capture_limits"`
	PacketSourcePolicy     sourceprojection.Policy          `json:"packet_source_policy"`
	PacketResolveLimits    retainedprojection.ResolveLimits `json:"packet_resolve_limits"`
	PacketMaxResponseBytes int                              `json:"packet_max_response_bytes"`
	DescribeDeadlineMS     int                              `json:"describe_deadline_ms"`
	SchemaVersions         SchemaVersions                   `json:"schema_versions"`
	RetryPolicy            RetryPolicy                      `json:"retry_policy"`
	ResponseVersion        string                           `json:"response_version,omitempty"`
}

type ContinuationContract struct{ wire contractWire }

func NewContinuationContract(in ContractInput) (ContinuationContract, error) {
	w := contractWire{SchemaVersion: ContractSchema, ContinuationID: in.ContinuationID, ProfileID: in.ProfileID, WorkerPinID: in.WorkerPinID, ProgramCSeed: in.ProgramCSeed, CaptureLimits: in.CaptureLimits, PacketSourcePolicy: in.PacketSourcePolicy, PacketResolveLimits: in.PacketResolveLimits, PacketMaxResponseBytes: in.PacketMaxResponseBytes, DescribeDeadlineMS: in.DescribeDeadlineMS, SchemaVersions: in.SchemaVersions, RetryPolicy: in.RetryPolicy, ResponseVersion: in.ResponseVersion}
	w.ContractID = identityJSON("lsp-trace:census-continuation-contract:v1", w, func(v *contractWire) { v.ContractID = "" })
	c := ContinuationContract{wire: w}
	return c, c.Validate()
}

func (c ContinuationContract) ID() string                               { return c.wire.ContractID }
func (c ContinuationContract) ProfileID() string                        { return c.wire.ProfileID }
func (c ContinuationContract) ProgramCSeed() uint64                     { return c.wire.ProgramCSeed }
func (c ContinuationContract) CaptureLimits() v5sourcesnapshotv3.Limits { return c.wire.CaptureLimits }
func (c ContinuationContract) PacketSourcePolicy() sourceprojection.Policy {
	return c.wire.PacketSourcePolicy
}
func (c ContinuationContract) PacketResolveLimits() retainedprojection.ResolveLimits {
	return c.wire.PacketResolveLimits
}
func (c ContinuationContract) PacketMaxResponseBytes() int { return c.wire.PacketMaxResponseBytes }
func (c ContinuationContract) DescribeDeadlineMS() int     { return c.wire.DescribeDeadlineMS }
func (c ContinuationContract) MaxAttempts() int            { return c.wire.RetryPolicy.MaxAttemptsPerRequest }
func (c ContinuationContract) ResponseVersion() string     { return c.wire.ResponseVersion }
func (c ContinuationContract) Bytes() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c.wire)
}
func ParseContinuationContract(b []byte) (ContinuationContract, error) {
	var w contractWire
	if err := strictCanonical(b, &w); err != nil {
		return ContinuationContract{}, err
	}
	c := ContinuationContract{w}
	return c, c.Validate()
}
func (c ContinuationContract) Validate() error {
	w := c.wire
	if w.SchemaVersion != ContractSchema || (w.ResponseVersion != "" && w.ResponseVersion != "V1" && w.ResponseVersion != "V2") || !validDigest(w.ContractID) || !validDigest(w.ContinuationID) || !validDigest(w.ProfileID) || !validDigest(w.WorkerPinID) || w.PacketMaxResponseBytes <= 0 || w.DescribeDeadlineMS <= 0 || w.RetryPolicy.MaxAttemptsPerRequest <= 0 || w.CaptureLimits.MaxArtifactBytes > MaxCaptureBytes || w.CaptureLimits.MaxParentBytes > MaxCaptureBytes {
		return errors.New("continuation contract integrity mismatch")
	}
	if identityJSON("lsp-trace:census-continuation-contract:v1", w, func(v *contractWire) { v.ContractID = "" }) != w.ContractID {
		return errors.New("continuation contract identity mismatch")
	}
	return nil
}
