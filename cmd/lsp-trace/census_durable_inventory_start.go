//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"time"
)

func defaultCensusDeclaration() (censusCallerHello, error) {
	if os.Getenv(censusDurableCallerEnv) != "" {
		return censusDurableCaller()
	}
	// These are explicit UNSPECIFIED caller declarations, not model execution pins.
	pin := cat.NativeDigestV3([]byte("UNSPECIFIED_CALLER_DECLARATION_NO_EXECUTION_AUTHENTICATION"))
	return censusCallerHello{Protocol: censusCallerProtocol, Interpreter: "main", Reviewer: "main", Pins: dw.HostPinsV2{ModelSHA256: pin, WorkerSHA256: pin, GrammarSHA256: pin}}, nil
}
func startCensusDurableInventory(ctx context.Context, o censusCLIOptions, inputs product.InventoryNativeInputsV3, targets []product.InventoryTargetV3, leads []ai.Lead, out io.Writer) (err error) {
	hello, e := defaultCensusDeclaration()
	if e != nil {
		return e
	}
	root, dir, e := openSelectedCensusStore(o.Workspace, o.durableStorePath)
	if e != nil {
		return e
	}
	mode := cat.ReviewModeSameThreadV3
	if hello.Subagents {
		mode = cat.ReviewModeSubagentsV3
	}
	policy := sc.DurableDescribePolicyV1{ReviewMode: mode, Reviewer: hello.Reviewer, RolePrompts: [2]string{product.InventoryRolePromptV3("interpreter"), product.InventoryRolePromptV3("reviewer")}}
	revision := censusInventorySourceRevision
	if len(targets) > 0 {
		revision = targets[0].Revision
	}
	q := ai.Request{Manifest: ai.Manifest{Scope: o.Workspace, Provenance: "bounded normal-folder paid acquisition; original D/R and both CALLS retained", Leads: leads, UntouchedGaps: []string{"bounded acquisition; unselected source and whole-application completeness unknown"}}, Preparer: inputs.Preparer(o.Workspace, revision, targets...), Interpreter: hello.Interpreter, ReviewerIdentity: hello.Reviewer, Pins: hello.Pins, MaxNativeAttempts: len(leads), NativeRetainedUpper: 8 << 20, BudgetBytes: 64 << 20, HostWindow: min(o.RequestTimeout, 30*time.Second), ReviewMode: mode}
	task, e := (ai.DurableStore{Root: root, Directory: dir}).Start(ctx, q, policy, o.durableDeadline)
	e = errors.Join(e, root.Close())
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(censusTaskStoreContext(task, o.durablePublicRoot))
}
