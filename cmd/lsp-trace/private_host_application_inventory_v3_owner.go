//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	inv "lsp-trace/internal/inventorysuccessor"
	product "lsp-trace/internal/privatehostproductv3"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// An instance-local constructor boundary, not an operator/result hook. Production
// always uses the fixed functions below. Tests can refuse construction without
// creating opaque owners, Managers, providers or selected results.
type applicationInventoryOwnerStagesV3 struct {
	reserve   func(context.Context, product.InventoryNativeOwnerV3) error
	construct func(context.Context, product.InventoryNativeOwnerV3, product.InventoryNativeInputV3) (*applicationInventoryOwnedV3, error)
}
type applicationInventoryOwnedV3 struct {
	input    product.InventoryNativeInputV3
	shutdown func(context.Context) error
	close    func() error
}
type applicationInventoryOwnerRouteV3 struct {
	production bool
	preparer   ai.NativePreparer
}

func applicationInventoryOwnerRoute(q product.InventoryRequestV3, a *product.ApplicationJournal, runID string, inputs product.InventoryNativeInputsV3) applicationInventoryOwnerRouteV3 {
	if inputs != nil {
		return applicationInventoryOwnerRouteV3{preparer: inputs.Preparer(q.Configuration.SourceRoot, q.Inventory.SourceRevision, q.Inventory.Targets...)}
	}
	q.Inventory = q.Inventory.Detached()
	return applicationInventoryOwnerRouteV3{production: true, preparer: applicationInventoryOwnerPreparerV3(q, applicationInventoryOwnerStagesV3{
		reserve: func(ctx context.Context, d product.InventoryNativeOwnerV3) error {
			return a.ReserveInventoryOwnerConstructionV3(ctx, q, runID, d)
		},
		construct: func(ctx context.Context, d product.InventoryNativeOwnerV3, v product.InventoryNativeInputV3) (*applicationInventoryOwnedV3, error) {
			if err := admitApplicationInventoryOwnerPathsV3(ctx, d.AccountingRoot, q.Configuration.SourceRoot, q.Configuration.Store); err != nil {
				return nil, err
			}
			return constructApplicationInventoryOwnerV3(ctx, d, v)
		},
	})}
}

func applicationInventoryOwnerPreparerV3(q product.InventoryRequestV3, stages applicationInventoryOwnerStagesV3) ai.NativePreparer {
	// Freeze DATA within the admitted 64KiB request, not opaque authority. Retain
	// the exact absolute deadline. Creating this closure performs no native I/O.
	q.Inventory = q.Inventory.Detached()
	used := map[string]bool{}
	var mu sync.Mutex
	return func(ctx context.Context, l ai.Lead) (ai.NativeResult, error) {
		mu.Lock()
		if used[l.ID] {
			mu.Unlock()
			return ai.NativeResult{}, product.ErrInvalid
		}
		used[l.ID] = true
		mu.Unlock()
		if ctx == nil || ctx.Err() != nil || stages.reserve == nil || stages.construct == nil {
			return ai.NativeResult{}, product.ErrInvalid
		}
		var d *product.InventoryNativeOwnerV3
		for i := range q.Inventory.NativeOwners {
			if q.Inventory.NativeOwners[i].Lead == l {
				d = &q.Inventory.NativeOwners[i]
				break
			}
		}
		if d == nil || product.ValidateInventoryRequestV3(q) != nil || !product.InventoryNativeDeadlineValidV3(ctx, d.Deadline) {
			return ai.NativeResult{}, product.ErrInvalid
		}
		v, e := d.DescriptorInput()
		if e != nil {
			return ai.NativeResult{}, e
		}
		var target product.InventoryTargetV3
		found := false
		for _, t := range q.Inventory.Targets {
			if t.LeadID == l.ID {
				target = t
				found = true
			}
		}
		if !found || product.InventoryNativeTargetV3(v) != target {
			return ai.NativeResult{}, product.ErrInvalid
		}
		// Persist construction debit BEFORE root admission, owner reopen, starter or
		// Manager creation. It grants no native credit. Refusal retains the census.
		if e = stages.reserve(ctx, *d); e != nil {
			return ai.NativeResult{}, e
		}
		nativeCtx, cancel := context.WithDeadline(ctx, d.Deadline)
		defer cancel()
		if !product.InventoryNativeDeadlineValidV3(nativeCtx, d.Deadline) {
			return ai.NativeResult{}, product.ErrInvalid
		}
		owned, e := stages.construct(nativeCtx, *d, v)
		if owned == nil {
			return ai.NativeResult{}, errors.Join(e, product.ErrInvalid)
		}
		var result ai.NativeResult
		if e == nil {
			// This is the existing fixed target/owner admission and M -> P -> S operator,
			// NOT the R-only legacy Run and NOT a callback that can invent native output.
			p := (product.InventoryNativeInputsV3{l.ID: owned.input}).Preparer(q.Configuration.SourceRoot, q.Inventory.SourceRevision, target)
			result, e = p(nativeCtx, l)
		}
		return finishApplicationInventoryOwnerV3(nativeCtx, d.Deadline, result, e, owned)
	}
}

// Reject namespace aliases with <=129 metadata checks per declared root, after
// prepayment. No symlink is followed to inspect its target. This additional
// screening is not an atomic namespace proof: the unchanged publication.Root
// opener/ValidatePrivate and kernel enforce descriptor identity and custody.
func admitApplicationInventoryOwnerPathsV3(ctx context.Context, paths ...string) error {
	if len(paths) != 3 {
		return product.ErrInvalid
	}
	var ancestors [3][]os.FileInfo
	for i, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || len(path) > 4096 || strings.Count(path, "/") > 128 {
			return product.ErrInvalid
		}
		current := string(filepath.Separator)
		for _, component := range strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator)) {
			if ctx == nil || ctx.Err() != nil {
				return product.ErrInvalid
			}
			current = filepath.Join(current, component)
			info, err := os.Lstat(current)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return errors.Join(product.ErrInvalid, err)
			}
			ancestors[i] = append(ancestors[i], info)
		}
	}
	// Compare pinned observations, not path spelling: reject equal roots and
	// root/descendant aliases on case-insensitive filesystems too. At most
	// 3*2*128 constant-size comparisons; these are metadata, not source handles.
	for i, chain := range ancestors {
		leaf := chain[len(chain)-1]
		for j, other := range ancestors {
			if i == j {
				continue
			}
			for _, node := range other {
				if os.SameFile(leaf, node) {
					return product.ErrInvalid
				}
			}
		}
	}
	return nil
}

// Reuse the production SOURCE constructor chain with no alternate backend or
// initialization. The existing kernel alone verifies the supplied canonical job,
// exact current head and original authorization. All failure paths own cleanup.
func constructApplicationInventoryOwnerV3(ctx context.Context, d product.InventoryNativeOwnerV3, v product.InventoryNativeInputV3) (*applicationInventoryOwnedV3, error) {
	if !product.InventoryNativeDeadlineValidV3(ctx, d.Deadline) {
		return nil, product.ErrInvalid
	}
	root, e := publication.OpenRoot(d.AccountingRoot)
	if e != nil {
		return nil, e
	}
	owned := &applicationInventoryOwnedV3{input: v, close: root.Close}
	if !product.InventoryNativeDeadlineValidV3(ctx, d.Deadline) {
		return owned, product.ErrInvalid
	}
	if e = root.ValidatePrivate(); e != nil {
		return owned, e
	}
	owner, e := inv.OpenPrivateManagedInputOwnerV2(ctx, root, d.CanonicalJob, d.ExpectedHead, d.Verification)
	if e != nil {
		return owned, e
	}
	if !product.InventoryNativeDeadlineValidV3(ctx, d.Deadline) {
		return owned, product.ErrInvalid
	}
	deps := productionCensusRunnerDependencies()
	starter, e := deps.newStarter()
	if e != nil {
		return owned, e
	}
	if !product.InventoryNativeDeadlineValidV3(ctx, d.Deadline) {
		return owned, product.ErrInvalid
	}
	manager, e := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 8, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: starter})
	if e != nil {
		return owned, e
	}
	owned.shutdown = manager.Shutdown
	owned.input.Owner = owner
	owned.input.Capture.Root = root
	owned.input.Capture.Manager = manager
	return owned, nil
}

// Always join shutdown and final root close BEFORE exposing ANY result. No
// context.Background, extra timeout, renewed window, reset or native retry.
func finishApplicationInventoryOwnerV3(ctx context.Context, deadline time.Time, result ai.NativeResult, prior error, owned *applicationInventoryOwnedV3) (ai.NativeResult, error) {
	if owned == nil {
		return ai.NativeResult{}, errors.Join(prior, product.ErrInvalid)
	}
	if owned.shutdown != nil {
		prior = errors.Join(prior, owned.shutdown(ctx))
	}
	if owned.close != nil {
		prior = errors.Join(prior, owned.close())
	} else {
		prior = errors.Join(prior, product.ErrInvalid)
	}
	if !product.InventoryNativeDeadlineValidV3(ctx, deadline) {
		prior = errors.Join(prior, product.ErrInvalid)
	}
	if prior != nil {
		return ai.NativeResult{}, prior
	}
	return result, nil
}
