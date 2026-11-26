//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

// Store is local application configuration, never a path from a task/answer.
// Require an existing private directory; setup/provisioning is explicit.
func openCensusDurableStore() (*publication.Root, error) {
	p := os.Getenv(censusDurableStoreEnv)
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return nil, errors.New("LSP_TRACE_CENSUS_STORE must configure an existing absolute private application directory")
	}
	r, e := publication.OpenRoot(p)
	if e != nil {
		return nil, e
	}
	if e = r.ValidatePrivate(); e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}
func censusDurableCaller() (censusCallerHello, error) {
	var h censusCallerHello
	raw := []byte(os.Getenv(censusDurableCallerEnv))
	if len(raw) == 0 || len(raw) > 4096 {
		return h, errors.New("LSP_TRACE_CENSUS_CALLER must contain the bounded caller declaration JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&h) != nil || d.Decode(new(any)) != io.EOF || h.Protocol != censusCallerProtocol || h.Interpreter == "" || h.Reviewer == "" || len(h.Interpreter) > 256 || len(h.Reviewer) > 256 || h.Subagents && h.Interpreter == h.Reviewer || !h.Subagents && h.Interpreter != h.Reviewer || !validApplicationDigest(h.Pins.ModelSHA256) || !validApplicationDigest(h.Pins.WorkerSHA256) || !validApplicationDigest(h.Pins.GrammarSHA256) || h.Pins.PromptSHA256 != "" {
		return h, errors.New("invalid caller declaration; identities/pins are declarations, not authenticated execution")
	}
	return h, nil
}
func preflightCensusDurableDescribe(o censusCLIOptions) error {
	if _, e := censusDurableCaller(); e != nil {
		return e
	}
	workspace, e := filepath.Abs(o.Workspace)
	if e != nil {
		return e
	}
	workspace, e = filepath.EvalSymlinks(workspace)
	if e != nil {
		return e
	}
	store, e := filepath.EvalSymlinks(os.Getenv(censusDurableStoreEnv))
	if e != nil {
		return e
	}
	relative, e := filepath.Rel(workspace, store)
	if e != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("durable store must be outside source workspace")
	}
	r, e := openCensusDurableStore()
	if e != nil {
		return e
	}
	return r.Close()
}

// One-item milestone only. Larger lead sets require the controller's durable
// sequential state-machine extension; do not multiply budgets or omit leads.
func startCensusDurableDescribe(ctx context.Context, o censusCLIOptions, inputs product.InventoryNativeInputsV3, targets []product.InventoryTargetV3, leads []ai.Lead, out io.Writer) (err error) {
	if len(leads) != 1 || len(targets) != 1 {
		return errors.New("DURABLE_DESCRIBE_ONE_ITEM_ONLY: multi-item controller continuation is not implemented")
	}
	hello, e := censusDurableCaller()
	if e != nil {
		return e
	}
	root, e := openCensusDurableStore()
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	prepare := inputs.Preparer(o.Workspace, censusInventorySourceRevision, targets...)
	prepared, e := prepare(ctx, leads[0])
	if e != nil {
		return e
	}
	_, _, request, ok := prepared.Selected.Selected()
	if !ok {
		return sc.ErrSelectedConsumerV3
	}
	// The task's independent semantic interval begins after native preparation,
	// but remains inside the application wall deadline fixed at invocation start.
	// Native time stays spent; process exit never pauses or renews either clock.
	runDeadline := o.durableDeadline
	if runDeadline.IsZero() || !time.Now().Before(runDeadline) {
		return sc.ErrDurableDescribeV1
	}
	budget, e := sc.NewCallerBudgetV3(runDeadline, 64<<20)
	if e != nil {
		return e
	}
	deadline := time.Now().Add(min(o.RequestTimeout, 30*time.Second, time.Until(runDeadline)))
	pins := hello.Pins
	pins.PromptSHA256 = "sha256:" + request.Envelope.InputSHA256
	binding := sc.CallerDescribeBindingV3{AttemptID: digestCatalog(leads[0].ID + "/durable-describe"), Interpreter: hello.Interpreter, Pins: pins, Deadline: deadline, Limits: dw.SuppliedResponseLimits{MaxAdapterBytes: 64 << 10, MaxSemanticBytes: 32 << 10}, Budget: budget}
	mode := cat.ReviewModeSameThreadV3
	if hello.Subagents {
		mode = cat.ReviewModeSubagentsV3
	}
	policy := sc.DurableDescribePolicyV1{ReviewMode: mode, Reviewer: hello.Reviewer, RolePrompts: [2]string{product.InventoryRolePromptV3("interpreter"), product.InventoryRolePromptV3("reviewer")}}
	task, e := (sc.DurableDescribeStoreV1{Root: root}).Issue(ctx, prepared.Selected, binding, runDeadline, policy)
	if e != nil {
		return e
	}
	// This is a pending task, NEVER an inventory terminal/publication.
	return json.NewEncoder(out).Encode(task)
}

// Host-neutral request protocol: no host process, Python, Node, Pi or provider.
// Submission is read from stdin, other arguments are IDs and bounded integers.
func runCensusDurableDescribe(args []string, input io.Reader, out, diagnostic io.Writer) int {
	fail := func(e error) int { fmt.Fprintln(diagnostic, e); return 1 }
	if len(args) < 3 {
		return fail(errors.New("usage: census durable task|read|submit|result RUN_ID TASK_ID [ARTIFACT OFFSET LIMIT | ROLE]"))
	}
	root, e := openCensusDurableStore()
	if e != nil {
		return fail(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := sc.DurableDescribeStoreV1{Root: root}
	var result any
	run, task := args[1], args[2]
	switch args[0] {
	case "task":
		if len(args) != 3 {
			e = sc.ErrDurableDescribeV1
			break
		}
		result, e = store.Task(ctx, run, task)
	case "read":
		if len(args) != 6 {
			e = sc.ErrDurableDescribeV1
			break
		}
		offset, oe := strconv.Atoi(args[4])
		limit, le := strconv.Atoi(args[5])
		if oe != nil || le != nil {
			e = sc.ErrDurableDescribeV1
			break
		}
		result, e = store.Read(ctx, run, task, args[3], offset, limit)
	case "submit":
		if len(args) != 4 {
			e = sc.ErrDurableDescribeV1
			break
		}
		// Transport failure does not submit a partial answer. Complete oversized or
		// malformed answers still reach Submit and spend its persistent claim.
		var pending sc.DurableDescribeTaskV1
		pending, e = store.Task(ctx, run, task)
		if e != nil || pending.Status != "PENDING" || args[3] != "interpreter" {
			e = sc.ErrDurableDescribeV1
			break
		}
		var stop context.CancelFunc
		ctx, stop = context.WithDeadline(ctx, time.Unix(0, pending.DeadlineUnixNano))
		defer stop()
		var raw []byte
		raw, e = readCensusDurableInput(ctx, input)
		if e == nil {
			e = store.Submit(ctx, run, task, args[3], raw)
		}
		if e == nil {
			result, e = store.Task(ctx, run, task)
		}
	case "result":
		if len(args) != 3 {
			e = sc.ErrDurableDescribeV1
			break
		}
		var raw []byte
		raw, e = store.Result(ctx, run, task)
		if e == nil {
			result = struct {
				Schema, Status string
				Authority      uint8
				Accepted       bool
				Completeness   string
				Description    json.RawMessage
			}{"lsp-trace.durable-describe-result.v1", "DESCRIBED_NOT_REVIEWED", 0, false, "UNKNOWN", raw}
		}
	default:
		e = sc.ErrDurableDescribeV1
	}
	e = errors.Join(e, root.Close())
	if e != nil {
		return fail(e)
	}
	if e = json.NewEncoder(out).Encode(result); e != nil {
		return fail(e)
	}
	return 0
}
