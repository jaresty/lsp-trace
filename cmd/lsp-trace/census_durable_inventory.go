//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/publication"
)

const censusDurableStoreEnv = "LSP_TRACE_CENSUS_STORE"

func censusTaskStoreContext(task ai.DurableTask, explicit string) ai.DurableTask {
	if explicit != "" {
		for name, argv := range task.Contract.Operations {
			task.Contract.Operations[name] = append(argv, "--publication-root", explicit)
		}
	}
	return task
}

func openDefaultCensusStore(workspace string) (*publication.Root, string, error) {
	return openSelectedCensusStore(workspace, "")
}

// Explicit public selection wins; the environment override is compatibility only.
func openSelectedCensusStore(workspace, explicit string) (root *publication.Root, directory string, err error) {
	defer func() { err = censusStoreIOError(err) }()
	path := explicit
	if path == "" {
		path = os.Getenv(censusDurableStoreEnv)
	}
	if path == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			return nil, "", e
		}
		base, e = censusDefaultConfigDirectory(base, workspace)
		if e != nil {
			return nil, "", e
		}
		for _, name := range []string{"lsp-trace", "census", "durable"} {
			base, e = censusOwnedDirectory(base, name, false)
			if e != nil {
				return nil, "", e
			}
		}
		path = base
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, "", censusStoreError(censusresult.StorePathInvalid, nil)
	}
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil {
		// ENOENT alone is not evidence of an alias (including dangling ancestry).
		if errors.Is(e, os.ErrNotExist) {
			return nil, "", censusStoreError(censusresult.StoreRootMissing, e)
		}
		return nil, "", e
	}
	if canonical != path {
		return nil, "", censusStoreError(censusresult.StoreAliasRefused, nil)
	}
	if workspace != "" && censusPathWithin(workspace, path) {
		return nil, "", censusStoreError(censusresult.StoreSourceOverlap, nil)
	}
	info, e := os.Lstat(path)
	if e != nil {
		return nil, "", e
	}
	if !info.IsDir() {
		return nil, "", censusStoreError(censusresult.StorePathInvalid, nil)
	}
	root, e = publication.OpenRoot(path)
	if e != nil {
		return nil, "", e
	}
	if e = root.ValidatePrivate(); e != nil {
		_ = root.Close()
		return nil, "", e
	}
	return root, path, nil
}
func preflightCensusInventoryStore(workspace, explicit string) (string, string, error) {
	path := ""
	if explicit != "" {
		root, _, e := openSelectedCensusStore(workspace, explicit)
		if e != nil {
			return "", "", e
		}
		if e = root.Close(); e != nil {
			return "", "", e
		}
		path, e = censusOwnedDirectory(explicit, "durable", false)
		if e != nil {
			return "", "", e
		}
	}
	root, dir, e := openSelectedCensusStore(workspace, path)
	if e != nil {
		return "", "", e
	}
	e = root.Close()
	base := explicit
	if base == "" {
		base = filepath.Dir(dir)
		if os.Getenv(censusDurableStoreEnv) != "" {
			base = dir
		}
	}
	return dir, base, e
}

func runCensusDurable(args []string, input io.Reader, out, diagnostic io.Writer) int {
	stage := censusStageSyntax
	trustedRun := "" // Set only after an owned-store current-run observation succeeds.
	operation := "unknown"
	if len(args) > 0 {
		switch args[0] {
		case "task", "read", "submit", "result":
			operation = args[0]
		}
	}
	fail := func(err error) int {
		reason := censusresult.InvalidOperation
		if stage == censusStageConfig && operation != "unknown" {
			reason = censusConfigReason(err)
		} else if stage == censusStageAssembly && operation != "unknown" {
			reason = censusresult.RunUnavailable
			if errors.Is(err, ai.ErrUnsupportedStoredVersion) {
				reason = censusresult.UnsupportedStoredVersion
			} else if trustedRun != "" {
				reason = censusresult.OperationUnconfirmed
			}
		}
		return writeCensusOperationFailure(diagnostic, operation, reason, trustedRun)
	}
	// A single explicit store option, repeated on ID-only calls; never import a manifest.
	explicit := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--machine" {
			continue
		}
		if arg == "--publication-root" || strings.HasPrefix(arg, "--publication-root=") {
			if explicit != "" {
				return fail(ai.ErrDurableInventory)
			}
			if arg == "--publication-root" {
				i++
				if i >= len(args) {
					return fail(ai.ErrDurableInventory)
				}
				explicit = args[i]
			} else {
				explicit = strings.TrimPrefix(arg, "--publication-root=")
			}
			if explicit == "" {
				return fail(ai.ErrDurableInventory)
			}
		} else {
			positional = append(positional, arg)
		}
	}
	args = positional
	operation = "unknown"
	if len(args) > 0 {
		switch args[0] {
		case "task", "read", "submit", "result":
			operation = args[0]
		}
	}
	if len(args) < 2 {
		return fail(errors.New("usage: census durable task RUN_ID [TASK_ID] | read|submit|result RUN_ID TASK_ID ..."))
	}
	stage = censusStageConfig
	selected := ""
	if explicit != "" {
		// Validate the public parent too; do not silently canonicalize aliases or '..'.
		parent, _, e := openSelectedCensusStore("", explicit)
		if e != nil {
			return fail(e)
		}
		if e = parent.Close(); e != nil {
			return fail(e)
		}
		selected = filepath.Join(explicit, "durable")
	}
	root, dir, e := openSelectedCensusStore("", selected)
	if e != nil {
		return fail(e)
	}
	// Preserve explicitly configured one-item milestone handles without interpreting
	// a corrupt inventory record as a legacy fallback.
	if len(args[1]) == 64 {
		_, probe := root.ReadSelectorWithAbsence("inventory-run-"+args[1]+".json", 16384)
		if errors.Is(probe, os.ErrNotExist) && explicit == "" && os.Getenv(censusDurableStoreEnv) != "" {
			_ = root.Close()
			return runCensusDurableDescribe(args, input, out, diagnostic)
		}
	}
	stage = censusStageAssembly
	store := ai.DurableStore{Root: root, Directory: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run := args[1]
	if current, lookupErr := store.Task(ctx, run, ""); lookupErr == nil {
		trustedRun = current.RunID
	}
	task := ""
	if len(args) > 2 {
		task = args[2]
	}
	var result any
	switch args[0] {
	case "task":
		if len(args) != 2 && len(args) != 3 {
			e = ai.ErrDurableInventory
		} else {
			result, e = store.Task(ctx, run, task)
		}
	case "read":
		if len(args) != 6 {
			e = ai.ErrDurableInventory
			break
		}
		offset, oe := strconv.Atoi(args[4])
		limit, le := strconv.Atoi(args[5])
		if oe != nil || le != nil {
			e = ai.ErrDurableInventory
			break
		}
		result, e = store.Read(ctx, run, task, args[3], offset, limit)
	case "submit":
		if len(args) != 4 {
			e = ai.ErrDurableInventory
			break
		}
		var pending ai.DurableTask
		pending, e = store.Task(ctx, run, task)
		if e != nil {
			break
		}
		if pending.Status != "PENDING" || args[3] != pending.Role {
			e = ai.ErrDurableInventory
			break
		}
		var stop context.CancelFunc
		ctx, stop = context.WithDeadline(ctx, time.Unix(0, pending.DeadlineUnixNano))
		defer stop()
		var raw []byte
		raw, e = readCensusDurableInput(ctx, input)
		if e == nil {
			result, e = store.Submit(ctx, run, task, args[3], raw)
		}
	case "result":
		if len(args) != 3 {
			e = ai.ErrDurableInventory
		} else {
			result, e = store.Result(ctx, run, task)
		}
	default:
		e = ai.ErrDurableInventory
	}
	e = errors.Join(e, root.Close())
	if e != nil {
		return fail(e)
	}
	if task, ok := result.(ai.DurableTask); ok {
		result = censusTaskStoreContext(task, explicit)
	}
	if e = json.NewEncoder(out).Encode(result); e != nil {
		return fail(e)
	}
	return 0
}
