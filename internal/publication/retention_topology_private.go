package publication

import (
	"context"
	"errors"
	"os"
	"strings"
)

// CheckRetentionRootTopologyPrivate checks existing capabilities, not assertions
// about paths or identities. It writes nothing and returns no authority. Outputs
// must be mutually disjoint and disjoint from EVERY input (equality and ancestry
// in either direction). Input-input topology is the retained reader's concern.
//
// Every canonical path is traversed no-follow from / with all component handles
// held, matched to the admitted root, and its parent/name bindings rechecked.
// This does not extend the borrowed capabilities' NONCONCURRENT use/Close
// contract. The owner must keep all roots alive until all calls finish. It is a
// safe-boundary observation, not a namespace lease or an arbitrary same-user
// deletion/rename guarantee. No descriptors or general Root escape this API.
func CheckRetentionRootTopologyPrivate(ctx context.Context, outputs [3]*ContentRetentionRoot, inputs [3]*ObservedRoot) error {
	return checkRetentionRootTopologyPrivate(ctx, outputs, inputs, defaultObservedRootOps())
}

func checkRetentionRootTopologyPrivate(ctx context.Context, outputs [3]*ContentRetentionRoot, inputs [3]*ObservedRoot, ops observedRootOps) (err error) {
	if ctx == nil {
		return ErrObservedRootAdmission
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	var roots [6]*ObservedRoot
	for i, r := range outputs {
		if r == nil {
			return ErrObservedRootAdmission
		}
		if err = r.ValidatePrivate(); err != nil {
			return err
		}
		// Output policy is separate from the ancestry comparison. Do not relax
		// existing admission; in particular /tmp is also excluded on Linux.
		path := r.observed.root.path
		if path == "/tmp" || strings.HasPrefix(path, "/tmp/") || path == "/private/tmp" || strings.HasPrefix(path, "/private/tmp/") {
			return ErrObservedRootAdmission
		}
		roots[i] = r.observed
	}
	copy(roots[3:], inputs[:])
	var chains [6][]*os.File
	var infos [6][]os.FileInfo
	// Close every acquired component even when an acquisition returned a handle
	// alongside an error. A close failure invalidates an otherwise good check.
	defer func() {
		for i := len(chains) - 1; i >= 0; i-- {
			for j := len(chains[i]) - 1; j >= 0; j-- {
				err = errors.Join(err, ops.closeFile(chains[i][j]))
			}
		}
		err = errors.Join(err, ctx.Err())
	}()
	for i, r := range roots {
		if err = r.ValidatePrivate(); err != nil {
			return err
		}
		parts := strings.Split(strings.TrimPrefix(r.root.path, "/"), "/")
		// Finite descriptor reservation; longer paths fail closed rather than using
		// an unbounded number of held descriptors.
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
		if !os.SameFile(infos[i][len(infos[i])-1], r.root.info) {
			return ErrObservedRootAdmission
		}
	}
	// Compare actual component identities, not lexical prefixes. This catches
	// identity aliases even when their path strings differ.
	for i := 0; i < 3; i++ {
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
	// Recheck every parent/name edge while the observed chain is still pinned.
	// A root moved during traversal must not turn a stale chain into proof.
	for i, r := range roots {
		parts := strings.Split(strings.TrimPrefix(r.root.path, "/"), "/")
		names := []string{"/"}
		for _, part := range parts {
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
		if err = r.ValidatePrivate(); err != nil {
			return err
		}
	}
	return nil
}
