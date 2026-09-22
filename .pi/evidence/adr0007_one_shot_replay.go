package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
)

const authorizedRequest = "c3d9892a7cbbda802f9feea1dd5299e382c6854c9733bb272a8c6fd0820bedb8"
const failedCheckpoint = "sha256:c8635b72e56fa5a7636eaa09d3c607863a6de4d2a4d6548303ca17007fc317b9"

type hostConfig struct {
	Continuation struct {
		PublicationRoot string `json:"publication_root"`
		MaxObjectBytes  int64  `json:"max_object_bytes"`
		Worker          struct {
			Worker            describeworker.FilePin `json:"worker"`
			Model             describeworker.FilePin `json:"model"`
			Library           describeworker.FilePin `json:"library"`
			SandboxExecutable describeworker.FilePin `json:"sandbox_executable"`
			SandboxProfile    describeworker.FilePin `json:"sandbox_profile"`
			Grammar           describeworker.FilePin `json:"grammar"`
			RuntimeIdentity   string                 `json:"runtime_identity"`
			AdapterIdentity   string                 `json:"adapter_identity"`
			ModelIdentity     string                 `json:"model_identity"`
			Limits            describeworker.Limits  `json:"limits"`
		} `json:"worker"`
	} `json:"continuation"`
}

type oneShot struct {
	runner *describeworker.Runner
	cancel context.CancelFunc
	calls  int
	result describeworker.RunResult
	err    error
}

func (w *oneShot) Run(ctx context.Context, r describerequest.Record, attempt string) (describeworker.RunResult, error) {
	if w.calls != 0 {
		return describeworker.RunResult{}, errors.New("ONE_SHOT_SECOND_CALL_REJECTED")
	}
	if r.RecordID != authorizedRequest {
		return describeworker.RunResult{}, fmt.Errorf("ONE_SHOT_REQUEST_MISMATCH")
	}
	w.calls++
	w.result, w.err = w.runner.Run(ctx, r, attempt)
	w.cancel()
	return w.result, w.err
}

type output struct {
	Calls                  int      `json:"calls"`
	RequestID              string   `json:"request_id"`
	InvocationID           string   `json:"invocation_id,omitempty"`
	ResponseID             string   `json:"response_id,omitempty"`
	InvocationStatus       string   `json:"invocation_status,omitempty"`
	InvocationStdoutBytes  int      `json:"invocation_stdout_bytes,omitempty"`
	InvocationStderrBytes  int      `json:"invocation_stderr_bytes,omitempty"`
	ResponseStrictValid    bool     `json:"response_strict_valid"`
	CheckpointSelector     string   `json:"checkpoint_selector,omitempty"`
	ContinuationStatus     string   `json:"continuation_status,omitempty"`
	Stage                  string   `json:"stage,omitempty"`
	Code                   string   `json:"code,omitempty"`
	Subcode                string   `json:"subcode,omitempty"`
	Verdict                string   `json:"verdict,omitempty"`
	TargetRole             string   `json:"target_role,omitempty"`
	NearestOutwardConsumer string   `json:"nearest_outward_consumer,omitempty"`
	ConsumerNeed           string   `json:"consumer_need,omitempty"`
	ProvidedBehavior       string   `json:"provided_behavior,omitempty"`
	BoundaryContribution   string   `json:"boundary_contribution,omitempty"`
	Limitations            []string `json:"limitations,omitempty"`
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: replay HOST_CONFIG")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var h hostConfig
	if err := json.Unmarshal(raw, &h); err != nil {
		panic(err)
	}
	w := h.Continuation.Worker
	cfg := describeworker.Config{Worker: w.Worker, Model: w.Model, Library: w.Library, SandboxExecutable: w.SandboxExecutable, SandboxProfile: w.SandboxProfile, Grammar: w.Grammar, RuntimeIdentity: w.RuntimeIdentity, AdapterIdentity: w.AdapterIdentity, ModelIdentity: w.ModelIdentity, Limits: w.Limits}
	root, err := publication.OpenRoot(h.Continuation.PublicationRoot)
	if err != nil {
		panic(err)
	}
	defer root.Close()
	bundle, err := continuationhost.New(continuationhost.Config{Root: root, MaxObjectBytes: h.Continuation.MaxObjectBytes, Worker: cfg}, continuationhost.Primitives{})
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker := &oneShot{runner: bundle.Runner, cancel: cancel}
	res := censuscontinuation.Resume(ctx, censuscontinuation.ResumeRequest{Selector: failedCheckpoint, Store: bundle.Store, Worker: worker})
	o := output{Calls: worker.calls, RequestID: authorizedRequest, CheckpointSelector: res.CheckpointID, ContinuationStatus: string(res.Status)}
	if worker.result.Invocation.ID() != "" {
		o.InvocationID = worker.result.Invocation.ID()
		o.InvocationStatus = string(worker.result.Invocation.Status())
		receipt := worker.result.Invocation.Receipt()
		o.InvocationStdoutBytes = receipt.StdoutBytes
		o.InvocationStderrBytes = receipt.StderrBytes
		if b, e := worker.result.Invocation.Bytes(); e == nil {
			if _, e = describeworker.ParseInvocationRecord(b); e != nil {
				panic(e)
			}
		}
	}
	if worker.result.Response.ID() != "" {
		o.ResponseID = worker.result.Response.ID()
		b, e := worker.result.Response.Bytes()
		if e != nil {
			panic(e)
		}
		parsed, e := describeworker.ParseResponseRecord(b)
		if e != nil {
			panic(e)
		}
		if e = parsed.Validate(); e != nil {
			panic(e)
		}
		o.ResponseStrictValid = true
		s := parsed.Response()
		o.Verdict = s.Verdict
		o.TargetRole = s.TargetRole
		o.NearestOutwardConsumer = s.NearestOutwardConsumer
		o.ConsumerNeed = s.ConsumerNeed
		o.ProvidedBehavior = s.ProvidedBehavior
		o.BoundaryContribution = s.BoundaryContribution
		o.Limitations = s.Limitations
	}
	var failure *describeworker.Failure
	if describeworker.AsFailure(worker.err, &failure) {
		o.Stage = string(failure.Stage())
		o.Code = string(failure.Code())
		o.Subcode = failure.Subcode()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(o); err != nil {
		panic(err)
	}
	if worker.calls != 1 {
		os.Exit(2)
	}
}
