package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// SourceProtectionRootPrivateV2 is topology-only. It has no os.Root, content
// reader, publisher, or method that grants either capability. Borrowed uses and
// Close must not overlap. Observations are not an atomic snapshot or durable claim.
type SourceProtectionRootPrivateV2 struct {
	self *SourceProtectionRootPrivateV2
	path string
	file *os.File
	info os.FileInfo
}

func OpenSourceProtectionRootPrivateV2(ctx context.Context, path string) (out *SourceProtectionRootPrivateV2, err error) {
	if ctx == nil || len(path) == 0 || len(path) > 8192 || !utf8.ValidString(path) || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) || filepath.Clean(path) != path || len(strings.Split(strings.Trim(path, "/"), "/")) > 256 {
		return nil, ErrObservedRootAdmission
	}
	ops := defaultObservedRootOps()
	var current *os.File
	defer func() {
		err = errors.Join(err, ctx.Err())
		if err != nil {
			if current != nil {
				err = errors.Join(err, ops.closeFile(current))
			}
			out = nil
		}
	}()
	current, err = ops.openDir(nil, "/")
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrObservedRootAdmission
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		child, e := ops.openDir(current, part)
		ce := ops.closeFile(current)
		current = child
		if err = errors.Join(e, ce); err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrObservedRootAdmission
		}
	}
	info, e := ops.statFile(current)
	if e != nil {
		return nil, e
	}
	if info == nil || !info.IsDir() {
		return nil, ErrObservedRootAdmission
	}
	if _, _, e = sourceProtectionIdentityPrivateV2(info); e != nil {
		return nil, e
	}
	r := &SourceProtectionRootPrivateV2{path: path, file: current, info: info}
	r.self = r
	// Reopen the entire named chain with pinned parents and verify it both ways.
	if err = checkRootChainsPrivate(ctx, []topologyRootViewPrivate{{path, info, r.validate}}, 0, ops); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *SourceProtectionRootPrivateV2) validate() error {
	if r == nil || r.self != r || r.file == nil || r.info == nil {
		return ErrObservedRootAdmission
	}
	info, err := r.file.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() || !os.SameFile(info, r.info) {
		return ErrObservedRootAdmission
	}
	return nil
}
func (r *SourceProtectionRootPrivateV2) PathPrivateV2() (string, error) {
	if err := r.validate(); err != nil {
		return "", err
	}
	return r.path, nil
}
func (r *SourceProtectionRootPrivateV2) IdentityPrivateV2() (device, inode uint64, err error) {
	if err = r.validate(); err != nil {
		return
	}
	return sourceProtectionIdentityPrivateV2(r.info)
}
func (r *SourceProtectionRootPrivateV2) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	f := r.file
	r.file = nil
	r.self = nil
	return f.Close()
}
func (*SourceProtectionRootPrivateV2) MarshalJSON() ([]byte, error) {
	return nil, ErrObservedRootAdmission
}

// Optional artifact roots are only the supplied set; no hidden-root monitoring
// or complete protected-root inventory is claimed. The required source is separate.
func CheckProducerPacketRetentionTopologyPrivateV2(ctx context.Context, outputs [2]*ContentRetentionRoot, source *SourceProtectionRootPrivateV2, additional []*ObservedRoot) error {
	if ctx == nil || len(additional) > 15 {
		return ErrObservedRootAdmission
	}
	views := make([]topologyRootViewPrivate, 0, 3+len(additional))
	for _, r := range outputs {
		if err := r.ValidatePrivate(); err != nil {
			return err
		}
		p := r.observed.root.path
		if p == "/tmp" || strings.HasPrefix(p, "/tmp/") || p == "/private/tmp" || strings.HasPrefix(p, "/private/tmp/") {
			return ErrObservedRootAdmission
		}
		views = append(views, topologyRootViewPrivate{p, r.observed.root.info, r.ValidatePrivate})
	}
	if err := source.validate(); err != nil {
		return err
	}
	views = append(views, topologyRootViewPrivate{source.path, source.info, source.validate})
	for _, r := range additional {
		if err := r.ValidatePrivate(); err != nil {
			return err
		}
		views = append(views, topologyRootViewPrivate{r.root.path, r.root.info, r.ValidatePrivate})
	}
	return checkRootChainsPrivate(ctx, views, 2, defaultObservedRootOps())
}
