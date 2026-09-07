package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
	"lsp-trace/sliceops"
)

// This additive adapter delegates all traversal and provenance semantics to the
// same managed operation as MCP. It does not modify the legacy CLI pipeline.
func runGraphProvenanceSlice(ctx context.Context, c sliceConfig, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	workspace, err := filepath.Abs(c.workspace)
	if err != nil {
		return fail(err)
	}
	file, line, column, err := parseAt(c.ats[0])
	if err != nil {
		return fail(err)
	}
	_, uri, _, err := source.ResolveTarget(workspace, file)
	if err != nil {
		return fail(err)
	}
	command, err := exec.LookPath(c.command)
	if err != nil {
		return fail(err)
	}
	command, err = filepath.Abs(command)
	if err != nil {
		return fail(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "graph-provenance", Workspace: workspace, Profile: "cli", EnvironmentReference: "cli"})
	if err != nil {
		return fail(err)
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		return fail(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		return fail(err)
	}
	started := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.languageID, Process: managedprocess.Spec{Path: command, Args: append([]string(nil), c.args...), Dir: workspace, Env: append(os.Environ(), c.env...)}})
	if started.Failure != "" {
		return fail(fmt.Errorf("managed startup: %s", started.Failure))
	}
	fmt.Fprintln(stderr, "graph provenance: LOCAL_DARWIN_SUPERVISION_ONLY; analyzed source remains unverified")
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		stopped := manager.Stop(cleanup, started.SessionID, "graph-provenance-cli")
		if stopped.Failure != "" {
			fmt.Fprintln(stderr, "managed cleanup:", stopped.Failure)
		}
		if err := manager.Shutdown(cleanup); err != nil {
			fmt.Fprintln(stderr, "managed cleanup:", err)
		}
	}()
	pending := manager.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(c.requestTimeout))
	ready, ok := manager.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		return fail(fmt.Errorf("managed readiness: %s", ready.Failure))
	}
	input, err := json.Marshal(map[string]any{"session_id": started.SessionID, "generation": started.Generation, "start_mode": "at", "uri": uri, "line": line - 1, "character": column - 1, "language_id": c.languageID, "down_depth": c.downDepth, "up_depth": c.upDepth, "max_nodes": c.maxNodes, "timeout_ms": c.timeout.Milliseconds(), "request_timeout_ms": c.requestTimeout.Milliseconds(), "graph_provenance": true})
	if err != nil {
		return fail(err)
	}
	result, failed := sliceops.NewExecutor(manager).Execute(ctx, operation.Request{Name: sliceops.OperationSlice, Input: input})
	if failed != nil {
		return fail(failed)
	}
	data := result.Artifact
	if c.pretty {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			return fail(err)
		}
		data = pretty.Bytes()
	}
	if c.output != "" {
		err = publishBundle(c.output, data)
	} else {
		_, err = stdout.Write(data)
	}
	if err != nil {
		return fail(err)
	}
	return 0 // The wrapper publishes bounded partial graphs; inspect embedded summary.
}
