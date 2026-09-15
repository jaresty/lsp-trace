package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"lsp-trace/internal/operation"
	tsr "lsp-trace/internal/transientstructuralresult"
	"lsp-trace/internal/vcssymbolsidecar"
)

type historicalLSPProfile struct {
	command   string
	args, env []string
}
type contextSymbolChurnExecutor struct {
	profiles map[string]historicalLSPProfile
}
type contextSymbolChurnInput struct {
	Input            string `json:"input"`
	Workspace        string `json:"workspace"`
	FromRevision     string `json:"from_revision"`
	ToRevision       string `json:"to_revision"`
	Profile          string `json:"profile"`
	LanguageID       string `json:"language_id"`
	TimeoutMS        int    `json:"timeout_ms,omitempty"`
	RequestTimeoutMS int    `json:"request_timeout_ms,omitempty"`
}

func newContextSymbolChurnExecutor(config bootstrapConfig) (*contextSymbolChurnExecutor, error) {
	prepared, err := prepareBootstrap(config)
	if err != nil {
		return nil, err
	}
	profiles := map[string]historicalLSPProfile{}
	for i, p := range prepared {
		keys := []string{p.alias, config.Processes[i].Profile.Profile}
		for _, key := range keys {
			if key == "" {
				continue
			}
			if _, exists := profiles[key]; exists {
				return nil, errors.New("historical LSP profile name is ambiguous")
			}
			profiles[key] = historicalLSPProfile{command: p.process.Path, args: append([]string(nil), p.process.Args...), env: append([]string(nil), p.process.Env...)}
		}
	}
	return &contextSymbolChurnExecutor{profiles: profiles}, nil
}
func (e *contextSymbolChurnExecutor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if op.Name != operation.Name("context_symbol_churn") {
		return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
	}
	var in contextSymbolChurnInput
	if err := json.Unmarshal(op.Input, &in); err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: err}
	}
	profile, ok := e.profiles[in.Profile]
	if !ok {
		names := make([]string, 0, len(e.profiles))
		for name := range e.profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		message := "historical LSP profile unavailable"
		if len(names) > 0 {
			message += "; available profiles: " + strings.Join(names, ", ")
		}
		return operation.Result{}, &operation.Failure{Code: "PROFILE_UNAVAILABLE", Err: errors.New(message)}
	}
	raw := []byte(in.Input)
	artifact, err := tsr.DecodeV2Artifact(raw)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: err}
	}
	set := map[string]struct{}{}
	for _, n := range artifact.Nodes {
		set[n.Path] = struct{}{}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	timeout := 60 * time.Second
	if in.TimeoutMS > 0 {
		timeout = time.Duration(in.TimeoutMS) * time.Millisecond
	}
	requestTimeout := 30 * time.Second
	if in.RequestTimeoutMS > 0 {
		requestTimeout = time.Duration(in.RequestTimeoutMS) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	result, err := vcssymbolsidecar.BuildV2(ctx, raw, vcssymbolsidecar.BuildRequest{Repository: in.Workspace, FromRevision: in.FromRevision, ToRevision: in.ToRevision, Paths: paths, LanguageID: in.LanguageID}, vcssymbolsidecar.GitDiff{Timeout: timeout}, vcssymbolsidecar.GitWorktreeProvider{Timeout: timeout}, vcssymbolsidecar.LSPProcessProvider{Command: profile.command, Args: profile.args, Env: profile.env, RequestTimeout: requestTimeout})
	if err != nil {
		code := "ACQUISITION_FAILED"
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), strings.Contains(strings.ToLower(err.Error()), "timeout"):
			code = "TIMEOUT"
		case strings.Contains(err.Error(), "limit"):
			code = "RESOURCE_LIMIT"
		}
		return operation.Result{}, &operation.Failure{Code: code, Err: err}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: err}
	}
	return operation.Result{Artifact: encoded}, nil
}
