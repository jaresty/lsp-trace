package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/source"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
	"lsp-trace/sessionruntime"
)

type contextConfig struct {
	workspace, server, profile, configPath, languageID, at, file, symbol string
	serverArgs, serverEnv                                                stringsFlag
	machine                                                              bool
	line, character                                                      uint32
	upDepth, downDepth, maxNodes, maxMessages                            int
	maxBytes                                                             int64
	timeout, requestTimeout                                              time.Duration
	analysis                                                             transientstructural.AnalysisRequest
}

func parseContext(args []string) (contextConfig, error) {
	c := contextConfig{upDepth: 2, downDepth: 2, maxNodes: 100, maxMessages: int(transientstructuralresult.DefaultMaxMessages), maxBytes: int64(transientstructuralresult.DefaultMaxBytes), timeout: 5 * time.Second, requestTimeout: time.Second, analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisNeighborhood}}
	var analysis, direction string
	var analysisDepth int
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&c.machine, "machine", false, "emit closed machine JSON")
	fs.StringVar(&c.workspace, "workspace", "", "workspace path")
	fs.StringVar(&c.server, "server", "", "language server command")
	fs.StringVar(&c.profile, "profile", "", "named server profile")
	fs.StringVar(&c.configPath, "config", "", "profile config path")
	fs.Var(&c.serverArgs, "server-arg", "repeatable server argument")
	fs.Var(&c.serverEnv, "server-env", "repeatable KEY=VALUE")
	fs.StringVar(&c.languageID, "language-id", "", "document language id")
	fs.StringVar(&c.at, "at", "", "target PATH:LINE:COLUMN (one-based)")
	fs.StringVar(&c.file, "file", "", "target file")
	fs.StringVar(&c.symbol, "symbol", "", "exact symbol")
	fs.IntVar(&c.upDepth, "up-depth", c.upDepth, "incoming traversal depth")
	fs.IntVar(&c.downDepth, "down-depth", c.downDepth, "outgoing traversal depth")
	fs.IntVar(&c.maxNodes, "max-nodes", c.maxNodes, "maximum nodes")
	fs.IntVar(&c.maxMessages, "max-messages", c.maxMessages, "maximum protocol messages")
	fs.Int64Var(&c.maxBytes, "max-bytes", c.maxBytes, "maximum response bytes")
	fs.DurationVar(&c.timeout, "timeout", c.timeout, "global timeout")
	fs.DurationVar(&c.requestTimeout, "request-timeout", c.requestTimeout, "request timeout")
	fs.StringVar(&analysis, "analysis", "neighborhood", "neighborhood or impact")
	fs.StringVar(&direction, "direction", "", "incoming or outgoing for impact")
	fs.IntVar(&analysisDepth, "analysis-depth", 0, "directed impact depth")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() != 0 || !c.machine {
		return c, errors.New("context requires --machine and no positional arguments")
	}
	if c.workspace == "" || (c.server == "" && c.profile == "") || (c.server != "" && c.profile != "") {
		return c, errors.New("context requires --workspace and exactly one of --server or --profile")
	}
	if c.configPath != "" && c.profile == "" {
		return c, errors.New("--config requires --profile")
	}
	position := c.at != "" && c.file == "" && c.symbol == ""
	named := c.at == "" && c.file != "" && c.symbol != ""
	if !position && !named {
		return c, errors.New("context requires exactly one target: --at or --file with --symbol")
	}
	if position {
		p, l, col, err := parseAt(c.at)
		if err != nil {
			return c, err
		}
		c.file = p
		c.line = uint32(l - 1)
		c.character = uint32(col - 1)
	}
	switch strings.ToLower(analysis) {
	case "neighborhood":
		if direction != "" || analysisDepth != 0 {
			return c, errors.New("neighborhood forbids direction and analysis depth")
		}
	case "impact":
		c.analysis.Kind = transientstructural.AnalysisImpact
		c.analysis.MaxDepth = analysisDepth
		switch strings.ToLower(direction) {
		case "incoming":
			c.analysis.Direction = transientstructural.DirectionIncoming
		case "outgoing":
			c.analysis.Direction = transientstructural.DirectionOutgoing
		default:
			return c, errors.New("impact requires --direction incoming|outgoing")
		}
	default:
		return c, errors.New("invalid analysis")
	}
	q := transientstructuralresult.Request{Generation: 1, DownDepth: uint64(c.downDepth), UpDepth: uint64(c.upDepth), MaxNodes: uint64(c.maxNodes), TimeoutMS: uint64(c.timeout.Milliseconds()), RequestTimeoutMS: uint64(c.requestTimeout.Milliseconds()), MaxMessages: uint64(c.maxMessages), MaxBytes: uint64(c.maxBytes), Analysis: transientstructuralresult.AnalysisRequest{Kind: string(c.analysis.Kind), Direction: string(c.analysis.Direction), Depth: uint64(c.analysis.MaxDepth)}}
	if _, err := transientstructuralresult.NormalizeRequest(q); err != nil {
		return c, fmt.Errorf("%w: request=%+v", err, q)
	}
	return c, nil
}

func runContext(args []string, stdout, stderr io.Writer) int {
	c, err := parseContext(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	profile, err := loadRequestedProfile(c.workspace, profileFlags{Name: c.profile, ConfigPath: c.configPath})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	command := c.server
	if command == "" {
		command = profile.Command
	}
	command, err = exec.LookPath(command)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	command, err = filepath.Abs(command)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	workspace, err := filepath.Abs(c.workspace)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	_, uri, _, err := source.ResolveTarget(workspace, c.file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "transient-context", Workspace: workspace, Profile: "cli", EnvironmentReference: "cli"})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	parent, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()
	started := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: firstNonempty(c.languageID, profile.LanguageID), Process: managedprocess.Spec{Path: command, Args: append(append([]string{}, profile.Args...), c.serverArgs...), Dir: workspace, Env: append(append(os.Environ(), profile.Environment...), c.serverEnv...)}})
	if started.Failure != "" {
		fmt.Fprintln(stderr, "managed startup:", started.Failure)
		return 1
	}
	defer func() {
		cleanup, x := context.WithTimeout(context.Background(), 3*time.Second)
		defer x()
		manager.Stop(cleanup, started.SessionID, "context-cli")
		_ = manager.Shutdown(cleanup)
	}()
	pending := manager.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(c.requestTimeout))
	ready, ok := manager.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		fmt.Fprintln(stderr, "managed readiness failed")
		return 1
	}
	request := transientstructural.Request{SessionID: started.SessionID, Generation: started.Generation, LanguageID: firstNonempty(c.languageID, profile.LanguageID), Target: transientstructural.Target{URI: uri, Symbol: c.symbol}, UpDepth: c.upDepth, DownDepth: c.downDepth, MaxNodes: c.maxNodes, TimeoutMS: c.timeout.Milliseconds(), RequestTimeoutMS: c.requestTimeout.Milliseconds(), MaxMessages: c.maxMessages, MaxBytes: c.maxBytes, Analysis: c.analysis}
	if c.symbol == "" {
		request.Target.Line = &c.line
		request.Target.Character = &c.character
	}
	result, failure := transientstructural.Execute(ctx, manager, request)
	if failure != nil {
		raw, _ := json.Marshal(failure)
		fmt.Fprintln(stderr, string(raw))
		return 2
	}
	transientID, identityFailure := manager.TransientSessionIdentity(started.SessionID, started.Generation)
	if identityFailure != "" {
		fmt.Fprintln(stderr, "transient identity:", identityFailure)
		return 1
	}
	normalized, err := transientstructuralresult.Project(result, request, transientID)
	if err != nil {
		fmt.Fprintln(stderr, "context normalization:", err)
		return 1
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = stdout.Write(append(raw, '\n'))
	return 0
}

func firstNonempty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func normalizeContextResult(in transientstructural.Result, q transientstructural.Request, transientID string) (transientstructuralresult.Result, error) {
	nodeIDs := make(map[string]string, len(in.Analysis.Nodes))
	nodes := make([]transientstructuralresult.Node, len(in.Analysis.Nodes))
	for i, n := range in.Analysis.Nodes {
		id, err := transientstructuralresult.NodeID(transientID, q.Generation, n.ID)
		if err != nil {
			return transientstructuralresult.Result{}, errors.New("context normalization failed")
		}
		nodeIDs[n.ID] = id
		nodes[i] = transientstructuralresult.Node{ID: id}
	}
	target, ok := nodeIDs[in.TargetID]
	if !ok {
		return transientstructuralresult.Result{}, errors.New("context normalization failed")
	}
	edges := make([]transientstructuralresult.Edge, len(in.Analysis.Occurrences))
	for i, e := range in.Analysis.Occurrences {
		caller, callerOK := nodeIDs[e.CallerID]
		callee, calleeOK := nodeIDs[e.CalleeID]
		id, err := transientstructuralresult.EdgeID(transientID, q.Generation, e.ID, caller, callee)
		if err != nil || !callerOK || !calleeOK {
			return transientstructuralresult.Result{}, errors.New("context normalization failed")
		}
		edges[i] = transientstructuralresult.Edge{ID: id, CallerNodeID: caller, CalleeNodeID: callee}
	}
	var incoming, outgoing uint64
	for _, edge := range edges {
		if edge.CalleeNodeID == target {
			incoming++
		}
		if edge.CallerNodeID == target {
			outgoing++
		}
	}
	analysis := any(transientstructuralresult.NeighborhoodResult{RootNodeID: target, Nodes: nodes, Edges: edges, IncomingCount: incoming, OutgoingCount: outgoing})
	if in.Analysis.Kind == transientstructural.AnalysisImpact {
		reachable := []string{}
		for _, n := range nodes {
			if n.ID != target {
				reachable = append(reachable, n.ID)
			}
		}
		witnesses := []string{}
		for _, e := range edges {
			witnesses = append(witnesses, e.ID)
		}
		analysis = transientstructuralresult.ImpactResult{RootNodeID: target, Direction: string(q.Analysis.Direction), Depth: uint64(q.Analysis.MaxDepth), Nodes: nodes, Edges: edges, ReachableNodeIDs: reachable, WitnessEdgeIDs: witnesses}
	}
	a := transientstructuralresult.Accounting{
		RequestAttempted: uint64(in.Accounting.Requests.Attempted), RequestSucceeded: uint64(in.Accounting.Requests.Succeeded), RequestFailed: uint64(in.Accounting.Requests.Failed), RequestCancelled: uint64(in.Accounting.Requests.Cancelled),
		PreparedAttempted: uint64(in.Accounting.Preparation.Attempted), PreparedReturned: uint64(in.Accounting.Preparation.Returned), PreparedEmpty: uint64(in.Accounting.Preparation.Empty), PreparedFailed: uint64(in.Accounting.Preparation.Failed),
		NodeObserved: uint64(in.Accounting.Nodes.Observed), NodeAdmitted: uint64(in.Accounting.Nodes.Admitted), NodeRejected: uint64(in.Accounting.Nodes.Rejected), NodeOmitted: uint64(in.Accounting.Nodes.Omitted),
		OccurrenceObserved: uint64(in.Accounting.Occurrences.Observed), OccurrenceAdmitted: uint64(in.Accounting.Occurrences.Admitted), OccurrenceRejected: uint64(in.Accounting.Occurrences.Rejected), OccurrenceOmitted: uint64(in.Accounting.Occurrences.Omitted),
		FrontierObserved: uint64(in.Accounting.Frontier.Observed), FrontierExpanded: uint64(in.Accounting.Frontier.Expanded), FrontierUnexpanded: uint64(in.Accounting.Frontier.Unexpanded),
		RequestOmissionReasons: transientstructuralresult.EmptyReasonMap(), NodeOmissionReasons: transientstructuralresult.EmptyReasonMap(), OccurrenceOmissionReasons: transientstructuralresult.EmptyReasonMap(), FrontierOmissionReasons: transientstructuralresult.EmptyReasonMap(),
	}
	if a.RequestFailed != 0 || a.RequestCancelled != 0 {
		return transientstructuralresult.Result{}, errors.New("context normalization failed")
	}
	if a.NodeOmitted > 0 {
		a.NodeOmissionReasons[transientstructuralresult.Deduplication] = a.NodeOmitted
		a.DeduplicatedNodes = a.NodeOmitted
	}
	if a.OccurrenceOmitted > 0 {
		a.OccurrenceOmissionReasons[transientstructuralresult.Deduplication] = a.OccurrenceOmitted
		a.DeduplicatedOccurrences = a.OccurrenceOmitted
	}
	for _, omission := range in.Accounting.Omissions {
		reason := transientstructuralresult.OmissionReason(omission.Reason)
		if omission.Count < 0 {
			return transientstructuralresult.Result{}, errors.New("context normalization failed")
		}
		count := uint64(omission.Count)
		if reason == transientstructuralresult.Deduplication {
			deduplicated := a.NodeOmitted + a.OccurrenceOmitted
			if count < deduplicated {
				return transientstructuralresult.Result{}, errors.New("context normalization failed")
			}
			count -= deduplicated
		}
		if count > 0 {
			if _, known := a.FrontierOmissionReasons[reason]; !known {
				return transientstructuralresult.Result{}, errors.New("context normalization failed")
			}
			a.FrontierOmissionReasons[reason] += count
		}
	}
	nq := transientstructuralresult.Request{Generation: q.Generation, DownDepth: uint64(q.DownDepth), UpDepth: uint64(q.UpDepth), MaxNodes: uint64(q.MaxNodes), TimeoutMS: uint64(q.TimeoutMS), RequestTimeoutMS: uint64(q.RequestTimeoutMS), MaxMessages: uint64(q.MaxMessages), MaxBytes: uint64(q.MaxBytes), Analysis: transientstructuralresult.AnalysisRequest{Kind: string(q.Analysis.Kind), Direction: string(q.Analysis.Direction), Depth: uint64(q.Analysis.MaxDepth)}}
	out := transientstructuralresult.NewResult(transientID, q.Generation, target, in.Qualification.PositionEncoding, nq, a, analysis)
	if err := out.Validate(); err != nil {
		return transientstructuralresult.Result{}, errors.New("context normalization failed")
	}
	return out, nil
}
