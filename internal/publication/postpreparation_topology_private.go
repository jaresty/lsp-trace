package publication

import (
	"context"
	"errors"
	"os"
	"strings"
)

// CheckPostPreparationRetentionTopologyV1Private borrows NONCONCURRENT roots.
// The caller supplies the complete protected artifact-root set; this checks the
// provided capabilities, not their completeness or identities from a DTO.
// Unlike the historical 3+3 topology this contract is exactly 2+(1..16): no
// fabricated output or fixed reference plan can stand in for the variable inputs.
// Canonical chains are pinned and rechecked descriptor-relatively. This is a
// boundary observation, not a lease against same-user namespace mutation.
func CheckPostPreparationRetentionTopologyV1Private(ctx context.Context, outputs [2]*ContentRetentionRoot, protected []*ObservedRoot) error {
	return checkPostPreparationRetentionTopologyV1Private(ctx, outputs, protected, defaultObservedRootOps())
}

func checkPostPreparationRetentionTopologyV1Private(ctx context.Context, outputs [2]*ContentRetentionRoot, protected []*ObservedRoot, ops observedRootOps) (err error) {
	if ctx == nil || len(protected) < 1 || len(protected) > 16 {
		return ErrObservedRootAdmission
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	roots := make([]*ObservedRoot, 2+len(protected))
	for i, r := range outputs {
		if err = r.ValidatePrivate(); err != nil {
			return err
		}
		path := r.observed.root.path
		if path == "/tmp" || strings.HasPrefix(path, "/tmp/") || path == "/private/tmp" || strings.HasPrefix(path, "/private/tmp/") {
			return ErrObservedRootAdmission
		}
		roots[i] = r.observed
	}
	copy(roots[2:], protected)
	views := make([]topologyRootViewPrivate, len(roots))
	for i, r := range roots {
		// Preserve the old per-root validation order inside the chain checker,
		// including closes/errors from chains acquired before an invalid input.
		views[i].validate = r.ValidatePrivate
		if r != nil && r.root != nil {
			views[i].path, views[i].info = r.root.path, r.root.info
		}
	}
	return checkRootChainsPrivate(ctx, views, 2, ops)
}

// Only package-owned capability wrappers construct these neutral views.
// No arbitrary caller predicate or serialized root can enter the checker.
type topologyRootViewPrivate struct {
	path     string
	info     os.FileInfo
	validate func() error
}

func checkRootChainsPrivate(ctx context.Context, roots []topologyRootViewPrivate, outputCount int, ops observedRootOps) (err error) {
	if ctx == nil || outputCount < 0 || outputCount > len(roots) {
		return ErrObservedRootAdmission
	}
	chains := make([][]*os.File, len(roots))
	infos := make([][]os.FileInfo, len(roots))
	defer func() {
		for i := len(chains) - 1; i >= 0; i-- {
			for j := len(chains[i]) - 1; j >= 0; j-- {
				err = errors.Join(err, ops.closeFile(chains[i][j]))
			}
		}
		err = errors.Join(err, ctx.Err())
	}()
	for i, r := range roots {
		if err = r.validate(); err != nil {
			return err
		}
		parts := strings.Split(strings.TrimPrefix(r.path, "/"), "/")
		if len(parts) > 256 {
			return ErrObservedRootAdmission
		}
		names := append([]string{"/"}, parts...)
		var parent *os.File
		for _, name := range names {
			if name == "" {
				continue
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			f, e := ops.openDir(parent, name)
			if f != nil {
				chains[i] = append(chains[i], f)
			}
			if e != nil {
				return e
			}
			if f == nil {
				return ErrObservedRootAdmission
			}
			info, e := ops.statFile(f)
			if e != nil {
				return e
			}
			if info == nil || !info.IsDir() {
				return ErrObservedRootAdmission
			}
			infos[i] = append(infos[i], info)
			parent = f
		}
		if !os.SameFile(infos[i][len(infos[i])-1], r.info) {
			return ErrObservedRootAdmission
		}
	}
	for i := 0; i < outputCount; i++ {
		for j := i + 1; j < len(roots); j++ {
			a, b := infos[i], infos[j]
			for _, ancestor := range b {
				if os.SameFile(a[len(a)-1], ancestor) {
					return ErrObservedRootAdmission
				}
			}
			for _, ancestor := range a {
				if os.SameFile(b[len(b)-1], ancestor) {
					return ErrObservedRootAdmission
				}
			}
		}
	}
	for i, r := range roots {
		names := []string{"/"}
		for _, part := range strings.Split(strings.TrimPrefix(r.path, "/"), "/") {
			if part != "" {
				names = append(names, part)
			}
		}
		for j, name := range names {
			if err = ctx.Err(); err != nil {
				return err
			}
			var parent *os.File
			if j > 0 {
				parent = chains[i][j-1]
			}
			f, e := ops.openDir(parent, name)
			if f != nil {
				info, se := ops.statFile(f)
				if se == nil && (info == nil || !info.IsDir() || !os.SameFile(info, infos[i][j])) {
					se = ErrObservedRootAdmission
				}
				e = errors.Join(e, se, ops.closeFile(f))
			} else if e == nil {
				e = ErrObservedRootAdmission
			}
			if e != nil {
				return e
			}
		}
		if err = r.validate(); err != nil {
			return err
		}
	}
	return nil
}
