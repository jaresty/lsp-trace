package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

//go:embed bootstrap.example.v1.json
var publicBootstrapExample []byte

type bootstrapConfig struct {
	Version   int                            `json:"version"`
	Processes []bootstrapProcessConfig       `json:"processes,omitempty"`
	Providers []bootstrapProviderDeclaration `json:"providers,omitempty"`
}

type bootstrapProcessConfig struct {
	Alias               string                    `json:"alias,omitempty"`
	LanguageID          string                    `json:"language_id,omitempty"`
	Profile             bootstrapProfileIdentity  `json:"profile"`
	Execution           managedExecutionAuthority `json:"execution"`
	SeedBinding         *seedbinding.Manifest     `json:"seed_binding,omitempty"`
	SeedCustodySelector string                    `json:"seed_custody_selector,omitempty"`
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
	Alias             string
	SessionID         string
	Generation        uint64
	RepositoryRoot    string
	GitCommit         string
	CustodyProvenance seedbinding.CustodyMode
}

func loadBootstrapConfig(path string) (bootstrapConfig, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config path: %w", err)
	}
	absolutePath = filepath.Clean(absolutePath)
	file, err := os.Open(absolutePath)
	if err != nil {
		return bootstrapConfig{}, err
	}
	defer file.Close()
	const maxBootstrapBytes = 4 << 20
	raw, err := io.ReadAll(io.LimitReader(file, maxBootstrapBytes+1))
	if err != nil || len(raw) > maxBootstrapBytes {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config unavailable or exceeds byte limit")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var config bootstrapConfig
	if err := decoder.Decode(&config); err != nil {
		return bootstrapConfig{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config must contain one JSON value")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return bootstrapConfig{}, err
	}
	if config.Version != 1 || len(config.Processes)+len(config.Providers) == 0 {
		return bootstrapConfig{}, fmt.Errorf("bootstrap config requires version 1 and at least one process or provider")
	}
	base := filepath.Dir(absolutePath)
	resolve := func(value string) string {
		if value == "" {
			return ""
		}
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Clean(filepath.Join(base, value))
	}
	for i := range config.Processes {
		process := &config.Processes[i]
		process.Profile.Workspace = resolve(process.Profile.Workspace)
		process.Execution.Path = resolve(process.Execution.Path)
		process.Execution.Directory = resolve(process.Execution.Directory)
		if process.Profile.Workspace == "." || process.Execution.Path == "." || process.Execution.Directory == "." {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d requires nonempty workspace, execution path and directory", i)
		}
		if strings.TrimSpace(process.LanguageID) != process.LanguageID {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d language_id is not canonical", i)
		}
	}
	for i := range config.Providers {
		provider := &config.Providers[i]
		provider.Execution.Path = resolve(provider.Execution.Path)
		provider.Execution.Directory = resolve(provider.Execution.Directory)
	}
	if err := validateBootstrapProviders(config); err != nil {
		return bootstrapConfig{}, err
	}
	for i, process := range config.Processes {
		if process.SeedBinding == nil {
			if process.SeedCustodySelector != "" {
				return bootstrapConfig{}, fmt.Errorf("bootstrap process %d custody selector requires seed binding", i)
			}
			continue
		}
		manifest := process.SeedBinding
		if manifest.SchemaVersion != seedbinding.VersionV2 && manifest.SchemaVersion != seedbinding.VersionV3 {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d seed binding version invalid", i)
		}
		mode := manifest.CustodyMode
		if manifest.SchemaVersion == seedbinding.VersionV2 {
			mode = seedbinding.VerifiedHost
		}
		if mode == seedbinding.VerifiedHost && process.SeedCustodySelector == "" {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d VERIFIED_HOST seed binding requires custody selector", i)
		}
		if mode == seedbinding.CallerAssertedLocal && process.SeedCustodySelector != "" {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d CALLER_ASSERTED_LOCAL forbids custody selector", i)
		}
		if mode != seedbinding.VerifiedHost && mode != seedbinding.CallerAssertedLocal {
			return bootstrapConfig{}, fmt.Errorf("bootstrap process %d seed custody mode invalid", i)
		}
		if process.SeedCustodySelector != "" {
			decoded, err := hex.DecodeString(process.SeedCustodySelector)
			if err != nil || len(decoded) != sha256.Size || process.SeedCustodySelector != strings.ToLower(process.SeedCustodySelector) {
				return bootstrapConfig{}, fmt.Errorf("bootstrap process %d seed custody selector is not a canonical sha256 digest", i)
			}
		}
	}
	return config, nil
}

type preparedBootstrap struct {
	alias            string
	languageID       string
	profile          runtimeprofile.Profile
	process          managedprocess.Spec
	seedBinding      *seedbinding.Manifest
	providerIdentity seedbinding.ProviderIdentity
}

func prepareBootstrap(config bootstrapConfig) ([]preparedBootstrap, error) {
	prepared := make([]preparedBootstrap, 0, len(config.Processes))
	seen := make(map[string]struct{}, len(config.Processes))
	seenAliases := make(map[string]struct{}, len(config.Processes))
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
		if process.Alias != "" {
			if _, duplicate := seenAliases[process.Alias]; duplicate {
				return nil, fmt.Errorf("bootstrap process %d duplicates alias %q", i, process.Alias)
			}
			seenAliases[process.Alias] = struct{}{}
		}
		id := profile.SessionKey().String()
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("bootstrap process %d duplicates session identity %s", i, id)
		}
		seen[id] = struct{}{}
		spec := managedprocess.Spec{Path: process.Execution.Path, Args: append([]string(nil), process.Execution.Arguments...), Dir: process.Execution.Directory, Env: append([]string(nil), process.Execution.Environment...)}
		identity := seedbinding.ProviderIdentity{}
		if process.SeedBinding != nil {
			payload, err := os.ReadFile(spec.Path)
			if err != nil {
				return nil, fmt.Errorf("bootstrap process %d provider payload unavailable: %w", i, err)
			}
			payloadSum, pathSum := sha256.Sum256(payload), sha256.Sum256([]byte(filepath.Clean(spec.Path)))
			configBytes, _ := json.Marshal(struct {
				Args []string
				Dir  string
				Env  []string
			}{spec.Args, filepath.Clean(spec.Dir), spec.Env})
			configSum := sha256.Sum256(configBytes)
			v := process.SeedBinding.Validator
			identity = seedbinding.ProviderIdentity{Class: v.Class, Authority: v.Authority, Name: v.Name, Version: v.Version, ExecutableSHA256: "sha256:" + hex.EncodeToString(pathSum[:]), PayloadSHA256: "sha256:" + hex.EncodeToString(payloadSum[:]), ConfigSHA256: "sha256:" + hex.EncodeToString(configSum[:])}
			want := seedbinding.ProviderIdentity{Class: v.Class, Authority: v.Authority, Name: v.Name, Version: v.Version, ExecutableSHA256: v.ExecutableSHA256, PayloadSHA256: v.PayloadSHA256, ConfigSHA256: v.ConfigSHA256}
			if err := seedbinding.VerifyProviderIdentity(want, identity); err != nil {
				return nil, fmt.Errorf("bootstrap process %d: %w", i, err)
			}
		}
		prepared = append(prepared, preparedBootstrap{alias: process.Alias, languageID: process.LanguageID, profile: profile, process: spec, seedBinding: process.SeedBinding, providerIdentity: identity})
	}
	for i, process := range prepared {
		if process.alias != "" {
			if _, collision := seen[process.alias]; collision {
				return nil, fmt.Errorf("bootstrap process %d alias %q collides with a session identity", i, process.alias)
			}
		}
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
			Profile:          process.profile,
			Process:          process.process,
			LanguageID:       process.languageID,
			SeedBinding:      process.seedBinding,
			ProviderIdentity: process.providerIdentity,
		})
		if result.Failure != "" {
			rollback()
			return nil, fmt.Errorf("bootstrap process %d start: %s", i, result.Failure)
		}
		repositoryRoot, gitCommit := pinnedGitMetadata(process.process.Dir)
		session := bootstrapSession{Alias: process.alias, SessionID: result.SessionID, Generation: result.Generation, RepositoryRoot: repositoryRoot, GitCommit: gitCommit, CustodyProvenance: result.CustodyProvenance}
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

func pinnedGitMetadata(directory string) (string, string) {
	rootCommand := exec.Command("git", "-C", directory, "rev-parse", "--show-toplevel")
	rootBytes, err := rootCommand.Output()
	if err != nil {
		return "", ""
	}
	root := strings.TrimSpace(string(rootBytes))
	if root == "" || !filepath.IsAbs(root) {
		return "", ""
	}
	commitCommand := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD^{commit}")
	commitBytes, err := commitCommand.Output()
	if err != nil {
		return "", ""
	}
	return filepath.Clean(root), strings.TrimSpace(string(commitBytes))
}

type hostSeedTrustBundle struct {
	RootVersion uint32
	RootID      string
	Policy      seedbinding.HostPreparedPolicy
	Receipt     seedbinding.HostCustodyReceipt
}

type hostSeedBootstrapAuthority interface {
	AuthenticatedRootSet() (seedbinding.HostRootSet, error)
	AuthenticatedBundles() ([]hostSeedTrustBundle, error)
}

// officialHostSeedBootstrapAuthority is deliberately not configurable through
// CLI or MCP input. Official builds fail closed until the embedding host wires
// an already-authenticated bootstrap authority. Tests inject their authority
// directly into loadBootstrapSeedTrust and cannot expose it through run.
func officialHostSeedBootstrapAuthority() hostSeedBootstrapAuthority { return nil }

type bootstrapSeedTrust struct {
	receipts map[string]seedbinding.HostReceiptAuthority
}

func loadBootstrapSeedTrust(host hostSeedBootstrapAuthority, now time.Time) (*bootstrapSeedTrust, error) {
	if host == nil {
		return nil, nil
	}
	roots, err := host.AuthenticatedRootSet()
	if err != nil {
		return nil, fmt.Errorf("authenticated host seed roots: %w", err)
	}
	bundles, err := host.AuthenticatedBundles()
	if err != nil {
		return nil, fmt.Errorf("authenticated host seed bundles: %w", err)
	}
	trust := &bootstrapSeedTrust{receipts: map[string]seedbinding.HostReceiptAuthority{}}
	for i, bundle := range bundles {
		if bundle.RootVersion != roots.Version {
			return nil, fmt.Errorf("seed custody bundle %d root version mismatch", i)
		}
		root := roots.Keys[bundle.RootID]
		payload, err := seedbinding.CustodyReceiptSigningBytes(bundle.Receipt)
		if err != nil || len(root) != ed25519.PublicKeySize || !ed25519.Verify(root, payload, bundle.Receipt.Signature) {
			return nil, fmt.Errorf("seed custody receipt %d independent-root signature mismatch", i)
		}
		verified, err := roots.Verify(bundle.RootID, bundle.Policy, bundle.Receipt, now)
		if err != nil {
			return nil, fmt.Errorf("seed custody bundle %d: %w", i, err)
		}
		receipt := bundle.Receipt
		receipt.Authenticated = true
		selector, _ := seedbinding.CustodyReceiptDigest(receipt)
		if _, exists := trust.receipts[selector]; exists {
			return nil, fmt.Errorf("duplicate seed custody receipt")
		}
		trust.receipts[selector] = seedbinding.HostReceiptAuthority{Receipt: receipt, PreparedContext: verified}
	}
	return trust, nil
}

type bootstrapSeedAuthorities []seedbinding.HostReceiptAuthority

func (a bootstrapSeedAuthorities) Verify(ctx context.Context, claim seedbinding.CustodyClaim) error {
	for _, authority := range a {
		if authority.Verify(ctx, claim) == nil {
			return nil
		}
	}
	return fmt.Errorf("no authenticated bootstrap custody receipt matches seed claim")
}

func seedAuthoritiesFromConfig(config bootstrapConfig, trust *bootstrapSeedTrust) seedbinding.RevisionAuthority {
	if trust == nil {
		return nil
	}
	authorities := make(bootstrapSeedAuthorities, 0, len(config.Processes))
	for _, process := range config.Processes {
		if process.SeedBinding == nil || process.SeedBinding.CustodyMode == seedbinding.CallerAssertedLocal {
			continue
		}
		authority, ok := trust.receipts[process.SeedCustodySelector]
		if !ok {
			return nil
		}
		authorities = append(authorities, authority)
	}
	if len(authorities) == 0 {
		return nil
	}
	return authorities
}

func seedRevisionFromConfig(config bootstrapConfig) string {
	var revision string
	for _, process := range config.Processes {
		if process.SeedBinding == nil {
			continue
		}
		if revision == "" {
			revision = process.SeedBinding.SourceRevision
			continue
		}
		if revision != process.SeedBinding.SourceRevision {
			return ""
		}
	}
	return revision
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
