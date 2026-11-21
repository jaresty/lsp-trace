//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/scopejob"
)

func runCensusScopeJob(o censusCLIOptions, out, diagnostic io.Writer) int {
	fail := func(e error) int {
		if errors.Is(e, errCensusEligibilityUnknown) {
			d, _ := buildCensusCLIDiagnostic(censusStageConfig, nil)
			raw, _ := marshalCensusCLIDiagnostic(d.WithDetail(errCensusEligibilityUnknown.Error()))
			_, _ = diagnostic.Write(raw)
			return 1
		}
		return writeCensusFailure(diagnostic, censusStageConfig, true, e)
	}
	workspace, e := filepath.Abs(o.Workspace)
	if e != nil {
		return fail(scopejob.ErrInvalid)
	}
	budget := scopejob.DefaultBudget()
	if o.ScopeJobBudgetPath != "" {
		info, e := os.Lstat(o.ScopeJobBudgetPath)
		if e != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
			return fail(scopejob.ErrInvalid)
		}
		f, e := os.Open(o.ScopeJobBudgetPath)
		if e != nil {
			return fail(scopejob.ErrInvalid)
		}
		opened, se := f.Stat()
		if se != nil || !os.SameFile(info, opened) {
			_ = f.Close()
			return fail(scopejob.ErrInvalid)
		}
		raw, readErr := io.ReadAll(io.LimitReader(f, 16385))
		e = errors.Join(readErr, f.Close())
		if e == nil {
			budget, e = decodeScopeJobBudget(raw)
		}
		if e != nil {
			return fail(scopejob.ErrInvalid)
		}
	}
	profile, selection, e := freezeCensusSelection(workspace, o.Profile)
	if e != nil {
		return fail(e)
	}
	config, e := scopejob.Canonical(scopejob.Config{Workspace: workspace, Roots: o.Sources, Includes: o.Includes, Excludes: o.Excludes, Prunes: o.PrunedDirectories, Profile: profile, Language: selection.DefaultLanguage, Selection: selection, Budget: budget})
	if e != nil {
		return fail(e)
	}
	// Source validation, including implicit Go-root evidence, is paid after the
	// inventory claim. No native server, caller IPC or model API is reached.
	root, _, e := openSelectedCensusStore(workspace, o.PublicationRoot)
	if e != nil {
		return fail(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ownedConfig, e := ai.EnumerationConfig(config)
	if e != nil {
		_ = root.Close()
		return fail(e)
	}
	ownedConfig.EnumerationRequireGoRoot = o.Profile == ""
	store := ai.DurableStore{Root: root, Directory: root.Path()}
	result, e := store.Create(ctx, ownedConfig)
	if result.RunID != "" {
		id := result.RunID
		operation := []string{"census", "job-status", id}
		if o.PublicationRoot != "" {
			operation = append(operation, "--publication-root", o.PublicationRoot)
		}
		// Emit the durable selector before enumeration, including for a later kill.
		receiptErr := json.NewEncoder(diagnostic).Encode(struct {
			Schema          string   `json:"schema"`
			JobID           string   `json:"job_id"`
			StatusOperation []string `json:"status_operation"`
		}{"lsp-trace.inventory-job-start.v1", id, operation})
		e = errors.Join(e, receiptErr)
	}
	if e == nil {
		result, e = store.Enumerate(ctx, result.RunID, result.Head)
	}
	e = errors.Join(e, root.Close(), ctx.Err())
	if e != nil {
		if result.RunID != "" {
			_ = json.NewEncoder(diagnostic).Encode(struct{ Schema, RunID, Phase, Status string }{"lsp-trace.inventory-job-failure.v1", result.RunID, result.Phase, result.Status})
		}
		return fail(e)
	}
	if writeInventoryEnumerationStatus(out, result, o.PublicationRoot) != nil {
		return 1
	}
	return 0
}

// The pre-funded status surface is capped independently of source scope size.
// The status selector is the only CLI continuation; acquisition is unavailable.
func writeInventoryEnumerationStatus(out io.Writer, status ai.InventoryJobStatus, explicit string) error {
	operation := []string{"census", "job-status", status.RunID}
	if explicit != "" {
		operation = append(operation, "--publication-root", explicit)
	}
	raw, err := json.Marshal(struct {
		ai.InventoryJobStatus
		StatusOperation []string `json:"status_operation"`
	}{status, operation})
	if err != nil || len(raw) > 32768 {
		return ai.ErrDurableInventory
	}
	n, err := out.Write(append(raw, '\n'))
	if err == nil && n != len(raw)+1 {
		return io.ErrShortWrite
	}
	return err
}

// Budget is a flat numeric object. Validate its raw key stream before assigning
// any Go field: encoding/json alone accepts duplicates and case aliases. Derive
// exact keys from Budget's own tags, not a second unrelated schema descriptor.
func decodeScopeJobBudget(raw []byte) (scopejob.Budget, error) {
	var budget scopejob.Budget
	if len(raw) > 16384 {
		return budget, scopejob.ErrInvalid
	}
	t := reflect.TypeOf(budget)
	keys := make(map[string]bool, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		keys[t.Field(i).Tag.Get("json")] = false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	start, e := d.Token()
	if e != nil || start != json.Delim('{') {
		return budget, scopejob.ErrInvalid
	}
	count := 0
	for d.More() {
		key, e := d.Token()
		name, ok := key.(string)
		seen, known := keys[name]
		if e != nil || !ok || !known || seen {
			return budget, scopejob.ErrInvalid
		}
		keys[name] = true
		value, e := d.Token()
		if _, ok := value.(json.Number); e != nil || !ok {
			return budget, scopejob.ErrInvalid
		}
		count++
	}
	end, e := d.Token()
	if e != nil || end != json.Delim('}') || count != len(keys) {
		return budget, scopejob.ErrInvalid
	}
	if _, e = d.Token(); e != io.EOF {
		return budget, scopejob.ErrInvalid
	}
	// Range, integer and signedness checks remain with the typed JSON decoder.
	if json.Unmarshal(raw, &budget) != nil {
		return scopejob.Budget{}, scopejob.ErrInvalid
	}
	return budget, nil
}

func runCensusScopeJobStatus(args []string, out, diagnostic io.Writer) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runCensusScopeJobStatusContext(ctx, args, out, diagnostic)
}

func runCensusScopeJobStatusContext(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	fail := func(e error) int { return writeCensusFailure(diagnostic, censusStageConfig, true, e) }
	if len(args) != 1 && len(args) != 3 {
		return fail(errors.New("usage: census job-status JOB_ID [--publication-root PATH]"))
	}
	explicit := ""
	if len(args) == 3 {
		if args[1] != "--publication-root" {
			return fail(scopejob.ErrInvalid)
		}
		explicit = args[2]
	}
	root, _, e := openSelectedCensusStore("", explicit)
	if e != nil {
		return fail(e)
	}
	// Inventory IDs are exact lowercase 32-byte hex; legacy scope IDs use
	// v2- plus 16-byte hex or the original 16-byte hex layout. Never scan stores.
	if len(args[0]) == 64 {
		store := ai.DurableStore{Root: root, Directory: root.Path()}
		result, err := store.JobStatus(ctx, args[0])
		err = errors.Join(err, root.Close(), ctx.Err())
		if err != nil {
			return fail(err)
		}
		if writeInventoryEnumerationStatus(out, result, explicit) != nil {
			return 1
		}
		return 0
	}
	result, e := scopejob.InspectContext(ctx, root, args[0], time.Now())
	e = errors.Join(e, root.Close(), ctx.Err())
	if e != nil {
		return fail(e)
	}
	if explicit != "" {
		result.StatusOperation = append(result.StatusOperation, "--publication-root", explicit)
	}
	if json.NewEncoder(out).Encode(result) != nil {
		return 1
	}
	return 0
}
