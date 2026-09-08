package main

import (
	"bytes"
	"context"
	"encoding/json"
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

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

// Version dispatch is explicit. Without the flag, legacy parsing and bytes are
// untouched. Duplicate flags fail closed, including conflicting v1/v2 values.
func acquisitionVersion(args []string) (string, []string, error) {
	version := ""
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") || arg == "-" {
			out = append(out, args[i:]...)
			break
		}
		name := strings.TrimPrefix(arg, "-")
		name = strings.TrimPrefix(name, "-")
		if name != "acquisition-version" && !strings.HasPrefix(name, "acquisition-version=") {
			out = append(out, arg)
			// All existing non-boolean flags consume one following value.
			// Do not mistake a server argument or filename for our selector.
			boolean := name == "pretty" || name == "graph-provenance" || name == "expand-topmost-siblings" || name == "expand-dispatch-family" || name == "help" || name == "h"
			if !boolean && !strings.Contains(name, "=") && i+1 < len(args) {
				i++
				out = append(out, args[i])
			}
			continue
		}
		if version != "" {
			return "", nil, fmt.Errorf("duplicate acquisition-version")
		}
		if _, v, ok := strings.Cut(name, "="); ok {
			version = v
		} else {
			i++
			if i >= len(args) {
				return "", nil, fmt.Errorf("acquisition-version requires a value")
			}
			version = args[i]
		}
		if version != "v1" && version != "v2" {
			return "", nil, fmt.Errorf("unsupported acquisition version %q", version)
		}
	}
	return version, out, nil
}

func admitAcquisitionV2(data []byte) error {
	_, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v2")
	return err
}

func runAcquisitionV2(mode string, args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	fs := flag.NewFlagSet(mode+" v2", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c sliceConfig
	var profile profileFlags
	var manifestPath string
	fs.StringVar(&c.workspace, "workspace", "", "host workspace path")
	fs.StringVar(&c.command, "server", "", "trusted language server executable")
	fs.StringVar(&profile.Name, "profile", "", "named server profile")
	fs.StringVar(&profile.ConfigPath, "config", "", "profile configuration")
	fs.Var(&c.args, "server-arg", "server argument")
	fs.Var(&c.env, "server-env", "server environment KEY=VALUE")
	fs.StringVar(&c.languageID, "language-id", "", "runtime default document language")
	fs.StringVar(&manifestPath, "seed-manifest", "", "versioned v2 seed manifest file; no inline selector/limit overrides")
	fs.StringVar(&c.output, "output", "", "immutable output selector")
	fs.BoolVar(&c.pretty, "pretty", false, "indent outer JSON without changing embedded graph bytes")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 || c.workspace == "" || manifestPath == "" {
		return fail(fmt.Errorf("v2 requires --workspace and --seed-manifest with no positional arguments"))
	}
	if profile.ConfigPath != "" && profile.Name == "" {
		return fail(fmt.Errorf("--config requires --profile"))
	}
	op := acquisitionops.Slice
	if mode == "incoming" {
		op = acquisitionops.Incoming
	}
	// Bounded regular-file input: no FIFO or unbounded ReadAll, no MCP path analogue.
	manifestRoot, err := os.OpenRoot(filepath.Dir(manifestPath))
	if err != nil {
		return fail(err)
	}
	raw, err := source.ReadRegularInputBounded(manifestRoot, filepath.Base(manifestPath), acquisitionops.MaxInputBytes)
	manifestRoot.Close()
	if err != nil {
		return fail(err)
	}
	manifest, err := acquisitionops.DecodeManifest(raw, op)
	if err != nil {
		return fail(err)
	}
	effective, err := manifest.Request(op)
	if err != nil {
		return fail(err)
	}
	if err := applySliceProfile(&c, profile, explicitServerFields(fs)); err != nil {
		return fail(err)
	}
	if c.command == "" {
		return fail(fmt.Errorf("--server or --profile is required"))
	}
	for _, entry := range c.env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			return fail(fmt.Errorf("invalid --server-env"))
		}
	}
	workspace, err := filepath.Abs(c.workspace)
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
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "public-acquisition", Workspace: workspace, Profile: "cli", EnvironmentReference: "cli"})
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, deadlineCancel := context.WithTimeout(ctx, effective.Limits.Timeout)
	defer deadlineCancel()
	started := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.languageID, Process: managedprocess.Spec{Path: command, Args: c.args, Dir: workspace, Env: append(os.Environ(), c.env...)}})
	if started.Failure != "" {
		return fail(fmt.Errorf("managed startup: %s", started.Failure))
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		stopped := manager.Stop(cleanup, started.SessionID, "public-acquisition-cli")
		if stopped.Failure != "" {
			fmt.Fprintln(stderr, "managed cleanup:", stopped.Failure)
		}
		if err := manager.Shutdown(cleanup); err != nil {
			fmt.Fprintln(stderr, "managed cleanup:", err)
		}
	}()
	pending := manager.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(effective.Limits.RequestTimeout))
	ready, ok := manager.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		return fail(fmt.Errorf("managed readiness: %s", ready.Failure))
	}
	input, _ := json.Marshal(acquisitionops.Input{SessionID: started.SessionID, Generation: started.Generation, SeedManifest: manifest})
	result, failed := acquisitionops.NewExecutor(manager).Execute(ctx, operation.Request{Name: op, Input: input})
	if failed != nil {
		return fail(failed)
	}
	data := result.Artifact
	if c.pretty {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			return fail(err)
		}
		data = append(pretty.Bytes(), '\n')
	}
	if c.output != "" {
		err = publishValidatedBundle(c.output, data, admitAcquisitionV2)
	} else {
		_, err = stdout.Write(data)
	}
	if err != nil {
		return fail(err)
	}
	return 0 // Every structurally valid acquisition retains partial outcomes.
}
