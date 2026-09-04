package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type bootstrapConfig struct {
	Version   int                      `json:"version"`
	Processes []bootstrapProcessConfig `json:"processes"`
}

type bootstrapProcessConfig struct {
	Profile   bootstrapProfileIdentity  `json:"profile"`
	Execution managedExecutionAuthority `json:"execution"`
}

type bootstrapProfileIdentity struct {
	TrustDomain          string                  `json:"trust_domain"`
	Workspace            string                  `json:"workspace"`
	Profile              string                  `json:"profile"`
	EnvironmentReference string                  `json:"environment_reference"`
	Options              []runtimeprofile.Option `json:"options,omitempty"`
}

// managedExecutionAuthority is host-owned process authority. It is deliberately
// distinct from runtimeprofile.Profile, which contains identity only.
type managedExecutionAuthority struct {
	Path        string   `json:"path"`
	Arguments   []string `json:"arguments,omitempty"`
	Directory   string   `json:"directory"`
	Environment []string `json:"environment,omitempty"`
}

type bootstrapSession struct {
	SessionID  string
	Generation uint64
}

func loadBootstrapConfig(path string) (bootstrapConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return bootstrapConfig{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config bootstrapConfig
	if err := decoder.Decode(&config); err != nil {
		return bootstrapConfig{}, err
	}
	if config.Version != 1 || len(config.Processes) == 0 {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config requires version 1 and at least one process")
	}
	for i, process := range config.Processes {
		if process.Execution.Path == "" || process.Execution.Directory == "" {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d execution path and directory are required", i)
		}
	}
	return config, nil
}

func startBootstrap(ctx context.Context, manager *sessionruntime.Manager, config bootstrapConfig, timeout time.Duration) ([]bootstrapSession, error) {
	started := make([]bootstrapSession, 0, len(config.Processes))
	rollback := func() { _ = stopBootstrap(context.Background(), manager, started) }
	for i, process := range config.Processes {
		validated, err := runtimeprofile.Validate(runtimeprofile.Selector{
			TrustDomain: process.Profile.TrustDomain, Workspace: process.Profile.Workspace,
			Profile: process.Profile.Profile, EnvironmentReference: process.Profile.EnvironmentReference,
			Options: process.Profile.Options,
		})
		if err != nil {
			rollback()
			return nil, fmt.Errorf("bootstrap process %d profile: %w", i, err)
		}
		result := manager.Start(ctx, sessionruntime.StartRequest{
			Profile: runtimeprofile.Resolve(validated),
			Process: managedprocess.Spec{Path: process.Execution.Path, Args: append([]string(nil), process.Execution.Arguments...), Dir: process.Execution.Directory, Env: append([]string(nil), process.Execution.Environment...)},
		})
		if result.Failure != "" {
			rollback()
			return nil, fmt.Errorf("bootstrap process %d start: %s", i, result.Failure)
		}
		session := bootstrapSession{SessionID: result.SessionID, Generation: result.Generation}
		started = append(started, session)
		deadline := time.Now().Add(timeout)
		pending := manager.BeginReadiness(ctx, session.SessionID, session.Generation, deadline)
		ready, found := manager.WaitReadiness(ctx, pending.ID)
		if !found || ready.SessionID != session.SessionID || ready.Generation != session.Generation || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
			rollback()
			return nil, fmt.Errorf("bootstrap process %d readiness failed: found=%t state=%s failure=%s", i, found, ready.State, ready.Failure)
		}
	}
	return started, nil
}

func stopBootstrap(ctx context.Context, manager *sessionruntime.Manager, sessions []bootstrapSession) error {
	for i := len(sessions) - 1; i >= 0; i-- {
		result := manager.Stop(ctx, sessions[i].SessionID, "production-bootstrap")
		if result.Failure != "" {
			return fmt.Errorf("stop bootstrap session %s: %s", sessions[i].SessionID, result.Failure)
		}
	}
	return manager.Shutdown(ctx)
}
