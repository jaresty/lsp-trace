package describeworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/targetpacket"
)

const (
	ResponseVersionV1 = "V1"
	ResponseVersionV2 = "V2"
)

type RunResultV2 struct {
	Invocation InvocationRecordV2
	Response   ResponseRecordV2
}

type workerResultV2 struct {
	Status        string  `json:"status"`
	Text          string  `json:"text,omitempty"`
	Tokens        int     `json:"tokens"`
	LoadMS        float64 `json:"load_ms"`
	RunMS         float64 `json:"run_ms"`
	Cancelled     bool    `json:"cancelled"`
	DecodeStatus  int32   `json:"decode_status"`
	Error         string  `json:"error,omitempty"`
	Grammar       string  `json:"grammar"`
	ContextTokens int     `json:"context_tokens"`
}

const (
	subcodeOuterProtocolV2      = "OUTER_PROTOCOL_OR_CANONICALITY"
	subcodeSemanticSyntaxV2     = "SEMANTIC_SYNTAX"
	subcodeSemanticSchemaV2     = "SEMANTIC_SCHEMA_MISMATCH"
	subcodeSemanticValueV2      = "SEMANTIC_VALUE_INVALID"
	subcodeHostBindingV2        = "HOST_BINDING_MISMATCH"
	subcodeConsumerResolutionV2 = "CONSUMER_RESOLUTION_MISMATCH"
	subcodeInternalValidationV2 = "INTERNAL_VALIDATION"
)

var (
	errHostBindingV2        = errors.New("response v2 host binding mismatch")
	errConsumerResolutionV2 = errors.New("response v2 consumer resolution mismatch")
)

// RunV2 executes the same pinned, sandboxed acquisition contract as Run and
// emits only additive V2 terminal records.
func (r *Runner) RunV2(ctx context.Context, request describerequest.Record, packet targetpacket.Packet, attemptID string) (RunResultV2, error) {
	if r == nil || r.manager == nil || r.validated.config.ResponseVersion != ResponseVersionV2 {
		return RunResultV2{}, failure(StageStart, CodePolicyMismatch)
	}
	if err := describerequest.ValidateRecord(request); err != nil || attemptID == "" || len(attemptID) > 256 {
		return RunResultV2{}, failure(StagePreflight, CodePolicyMismatch)
	}
	c := r.validated.config
	if err := verifyConfigPins(c); err != nil {
		return RunResultV2{}, failureWithSubcode(StagePreflight, CodeModelUnavailable, err.Error())
	}
	dir, err := os.MkdirTemp("", "lsp-trace-describe-")
	if err != nil {
		return RunResultV2{}, failure(StageStart, CodeBackendFailure)
	}
	defer os.RemoveAll(dir)
	if err = os.Chmod(dir, 0700); err != nil {
		return RunResultV2{}, failure(StageStart, CodeBackendFailure)
	}
	prompt := []byte(request.Envelope.Prompt)
	if len(prompt) > c.Limits.TempBytes {
		return RunResultV2{}, failure(StageRun, CodeResourceLimit)
	}
	promptPath := filepath.Join(dir, "prompt.txt")
	if err = os.WriteFile(promptPath, prompt, 0600); err != nil {
		return RunResultV2{}, failure(StageStart, CodeBackendFailure)
	}
	args := []string{"-f", c.SandboxProfile.Path, c.Worker.Path, "-model", c.Model.Path, "-lib", filepath.Dir(c.Library.Path), "-prompt-file", promptPath, "-grammar-file", c.Grammar.Path, "-timeout", (time.Duration(c.Limits.TimeoutMS) * time.Millisecond).String(), "-max-tokens", strconv.Itoa(c.Limits.MaxTokens), "-context-tokens", strconv.Itoa(c.Limits.ContextTokens)}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(c.Limits.TimeoutMS)*time.Millisecond)
	defer cancel()
	process, start := r.manager.Start(runCtx, managedprocess.Spec{Path: c.SandboxExecutable.Path, Args: args, Dir: dir, Env: []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "TMPDIR=" + dir, "LANG=C", "LC_ALL=C"}})
	if start.Kind != managedprocess.StartStarted {
		return RunResultV2{}, failure(StageStart, CodeBackendFailure)
	}
	receipt := ResourceReceipt{Started: true, TerminalOutcomes: 1}
	var out limitedCapture
	out.limit = c.Limits.StdoutBytes
	readDone := make(chan error, 1)
	go func() { _, e := io.Copy(&out, process.Stdout()); readDone <- e }()
	deathDone := make(chan managedprocess.DeathObservation, 1)
	go func() { deathDone <- process.Wait() }()
	var death managedprocess.DeathObservation
	select {
	case death = <-deathDone:
	case <-runCtx.Done():
		receipt.Teardown = true
		td := process.Teardown(context.Background())
		death = td.Death
		receipt.Reaped = td.Death.Reap.Kind == managedprocess.ReapComplete
	}
	_ = process.Close()
	<-readDone
	receipt.StdoutBytes = len(out.bytes)
	receipt.StdoutTruncated = out.truncated
	receipt.StderrBytes = len(death.Stderr.Bytes)
	receipt.StderrTruncated = death.Stderr.Truncated
	if death.Reap.Kind == managedprocess.ReapComplete {
		receipt.Reaped = true
	}
	status, code := StatusSucceeded, Code("")
	if ctx.Err() != nil {
		status, code = StatusCancelled, CodeCancelled
	} else if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		status, code = StatusTimeout, CodeTimeout
	} else if out.truncated || death.Stderr.Truncated {
		status, code = StatusResourceLimit, CodeResourceLimit
	} else if death.Kind != managedprocess.DeathExited || death.ExitCode != 0 {
		status, code = StatusBackendFailure, CodeBackendFailure
	}
	binding := bindingFrom(c, request, attemptID, prompt)
	terminal := func(status TerminalStatus, subcode string, response ResponseRecordV2) (InvocationRecordV2, error) {
		return newInvocationRecordV2(binding, status, receipt, out.bytes, death.Stderr.Bytes, subcode, response)
	}
	if code != "" {
		subcode := ""
		if code == CodeBackendFailure {
			subcode = classifyBackendExit(death)
		}
		invocation, _ := terminal(status, subcode, ResponseRecordV2{})
		result := RunResultV2{Invocation: invocation}
		if code == CodeBackendFailure {
			return result, backendFailure(subcode)
		}
		return result, failure(StageRun, code)
	}
	response, err := r.RecordV2(out.bytes, request, packet, attemptID)
	if err != nil {
		subcode := subcodeInternalValidationV2
		var f *Failure
		if AsFailure(err, &f) && f.Subcode() != "" {
			subcode = f.Subcode()
		}
		invocation, _ := terminal(StatusOutputInvalid, subcode, ResponseRecordV2{})
		return RunResultV2{Invocation: invocation}, err
	}
	invocation, err := terminal(StatusSucceeded, "", response)
	if err != nil {
		return RunResultV2{}, failure(StageRecord, CodeBackendFailure)
	}
	return RunResultV2{Invocation: invocation, Response: response}, nil
}

// RecordV2 is the explicit additive V2 branch. Run remains the historical V1 API.
// The adapter envelope remains canonical NDJSON; only its model text permits JSON whitespace.
func (r *Runner) RecordV2(adapterLine []byte, request describerequest.Record, packet targetpacket.Packet, attemptID string) (ResponseRecordV2, error) {
	if r == nil || r.validated.config.ResponseVersion != ResponseVersionV2 {
		return ResponseRecordV2{}, failure(StagePreflight, CodePolicyMismatch)
	}
	if len(adapterLine) < 2 || adapterLine[len(adapterLine)-1] != '\n' || adapterLine[len(adapterLine)-2] == '\n' {
		return ResponseRecordV2{}, failureWithSubcode(StageOutput, CodeOutputInvalid, subcodeOuterProtocolV2)
	}
	var outer workerResultV2
	if err := strictCanonical(adapterLine[:len(adapterLine)-1], &outer); err != nil || outer.Status != "COMPLETE" || outer.Text == "" || outer.Error != "" {
		return ResponseRecordV2{}, failureWithSubcode(StageOutput, CodeOutputInvalid, subcodeOuterProtocolV2)
	}
	host, err := responseHostV2(r.validated.config, request, packet, attemptID)
	if err != nil {
		return ResponseRecordV2{}, failureWithSubcode(StageRecord, CodeOutputInvalid, recordV2Subcode(err))
	}
	record, err := NewResponseRecordV2([]byte(outer.Text), host)
	if err != nil {
		return ResponseRecordV2{}, failureWithSubcode(StageOutput, CodeOutputInvalid, recordV2Subcode(err))
	}
	return record, nil
}

func recordV2Subcode(err error) string {
	switch {
	case errors.Is(err, errSemanticSyntaxV2):
		return subcodeSemanticSyntaxV2
	case errors.Is(err, errSemanticSchemaV2):
		return subcodeSemanticSchemaV2
	case errors.Is(err, errSemanticValueV2):
		return subcodeSemanticValueV2
	case errors.Is(err, errHostBindingV2):
		return subcodeHostBindingV2
	case errors.Is(err, errConsumerResolutionV2):
		return subcodeConsumerResolutionV2
	default:
		return subcodeInternalValidationV2
	}
}

func responseHostV2(c Config, request describerequest.Record, packet targetpacket.Packet, attemptID string) (ResponseHostV2, error) {
	if attemptID == "" || request.RecordID == "" || request.Lineage.PacketID != packet.PacketID {
		return ResponseHostV2{}, errHostBindingV2
	}
	if request.Lineage.ConsumerResolution != string(packet.ConsumerResolution) {
		return ResponseHostV2{}, errConsumerResolutionV2
	}
	consumer := ConsumerIdentityV2{Resolution: ConsumerResolvedV2, AlternativeID: request.Lineage.AlternativeID, AlternativeOrdinal: request.Lineage.AlternativeOrdinal}
	if packet.ConsumerResolution == targetpacket.ConsumerUnresolved {
		consumer.Resolution = ConsumerUnresolvedV2
		if len(packet.ConsumerAlternatives) != 0 || consumer.AlternativeID != "OUTWARD_CONSUMER_UNRESOLVED" || consumer.AlternativeOrdinal != 0 {
			return ResponseHostV2{}, errConsumerResolutionV2
		}
	} else {
		if consumer.AlternativeOrdinal < 0 || consumer.AlternativeOrdinal >= len(packet.ConsumerAlternatives) || packet.ConsumerAlternatives[consumer.AlternativeOrdinal].ReconciliationID != consumer.AlternativeID {
			return ResponseHostV2{}, errConsumerResolutionV2
		}
	}
	promptSum := sha256.Sum256([]byte(request.Envelope.Prompt))
	return ResponseHostV2{
		RequestRecordID: request.RecordID, MessageID: request.Envelope.MessageID, AttemptID: attemptID, Consumer: consumer,
		Pins:       HostPinsV2{WorkerSHA256: c.Worker.SHA256, ModelSHA256: c.Model.SHA256, GrammarSHA256: c.Grammar.SHA256, PromptSHA256: "sha256:" + hex.EncodeToString(promptSum[:])},
		Provenance: HostProvenanceV2{PacketID: packet.PacketID, RequestLineageIdentity: request.Lineage.LineageIdentity},
		Custody:    HostCustodyV2{GraphDigest: packet.Custody.GraphDigest, CaptureID: packet.Custody.CaptureID},
	}, nil
}

func EncodeAdapterResultV2(text string) []byte {
	outer := workerResultV2{Status: "COMPLETE", Text: text}
	raw, _ := json.Marshal(outer)
	return append(raw, '\n')
}
