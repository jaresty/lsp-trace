package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	if !filepath.IsAbs(path) {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config path must be absolute")
	}
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
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config must contain one JSON value")
	}
	if config.Version != 1 || len(config.Processes) == 0 {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config requires version 1 and at least one process")
	}
	for i, process := range config.Processes {
		if !filepath.IsAbs(process.Execution.Path) || !filepath.IsAbs(process.Execution.Directory) {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d execution path and directory must be absolute", i)
		}
	}
	return config, nil
}

type preparedBootstrap struct {
	profile runtimeprofile.Profile
	process managedprocess.Spec
}

func prepareBootstrap(config bootstrapConfig) ([]preparedBootstrap, error) {
	prepared := make([]preparedBootstrap, 0, len(config.Processes))
	seen := make(map[string]struct{}, len(config.Processes))
	for i, process := range config.Processes {
		validated, err := runtimeprofile.Validate(runtimeprofile.Selector{
			TrustDomain: process.Profile.TrustDomain, Workspace: process.Profile.Workspace,
			Profile: process.Profile.Profile, EnvironmentReference: process.Profile.EnvironmentReference,
			Options: process.Profile.Options,
		})
		if err != nil {
			return nil, fmt.Errorf("bootstrap process %d profile: %w", i, err)
		}
		profile := runtimeprofile.Resolve(validated)
		id := profile.SessionKey().String()
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("bootstrap process %d duplicates session identity %s", i, id)
		}
		seen[id] = struct{}{}
		prepared = append(prepared, preparedBootstrap{
			profile: profile,
			process: managedprocess.Spec{Path: process.Execution.Path, Args: append([]string(nil), process.Execution.Arguments...), Dir: process.Execution.Directory, Env: append([]string(nil), process.Execution.Environment...)},
		})
	}
	return prepared, nil
}

func startBootstrap(ctx context.Context, manager *sessionruntime.Manager, config bootstrapConfig, timeout time.Duration) ([]bootstrapSession, error) {
	prepared, err := prepareBootstrap(config)
	if err != nil {
		return nil, err
	}
	started := make([]bootstrapSession, 0, len(prepared))
	rollback := func() {
		rollbackContext, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_ = stopBootstrap(rollbackContext, manager, started)
	}
	for i, process := range prepared {
		result := manager.Start(ctx, sessionruntime.StartRequest{
			Profile: process.profile,
			Process: process.process,
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
	var failures []error
	for i := len(sessions) - 1; i >= 0; i-- {
		result := manager.Stop(ctx, sessions[i].SessionID, "production-bootstrap")
		if result.Failure != "" {
			failures = append(failures, fmt.Errorf("stop bootstrap session %s: %s", sessions[i].SessionID, result.Failure))
		}
	}
	if err := manager.Shutdown(ctx); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}
