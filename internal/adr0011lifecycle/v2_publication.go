package adr0011lifecycle

// V2 codec publication is a private single-record seam only. No head adoption,
// complete successor enumeration, fence authority or cleanup follows from it.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"lsp-trace/internal/publication"
)

type v2Binding struct {
	l                            *syntheticRoot
	codec                        *v2Codec
	rootIdentity, anchorIdentity map[string]any
	syncForTest                  func(*publication.Root) (bool, error) // nil uses pinned directory sync
}

func v2FileURI(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
func bindV2Synthetic(l *syntheticRoot, c *v2Codec, installationID string) (*v2Binding, error) {
	if l == nil || c == nil || c.roles == nil {
		return nil, errors.New("unavailable V2 binding")
	}
	if len(installationID) != 71 || installationID[:7] != "sha256:" {
		return nil, errors.New("independently selected installation ID required")
	}
	for _, letter := range installationID[7:] {
		if letter < '0' || letter > '9' {
			if letter < 'a' || letter > 'f' {
				return nil, errors.New("invalid installation digest")
			}
		}
	}
	if err := l.validate(); err != nil {
		return nil, err
	}
	rootDev, rootIno, err := directoryNumbers(l.rootInfo)
	if err != nil {
		return nil, err
	}
	anchorDev, anchorIno, err := directoryNumbers(l.anchorInfo)
	if err != nil {
		return nil, err
	}
	if rootDev > 9007199254740991 || rootIno > 9007199254740991 || anchorDev > 9007199254740991 || anchorIno > 9007199254740991 {
		return nil, errors.New("device/inode outside exact JSON integer range")
	}
	identity := func(path string, dev, ino uint64) map[string]any {
		return map[string]any{"installation_id": installationID, "root_uri": v2FileURI(path), "device": int64(dev), "inode": int64(ino)}
	}
	return &v2Binding{l: l, codec: c, rootIdentity: identity(l.root.Path(), rootDev, rootIno), anchorIdentity: identity(l.anchor.Path(), anchorDev, anchorIno)}, nil
}

// selectedIdentities checks references' exact selected directory and spelling.
// It does not establish that all future or external referenced records exist.
func (b *v2Binding) selectedIdentities(record map[string]any, role string) error {
	if !reflect.DeepEqual(record["root_identity"], b.rootIdentity) {
		return errors.New("record root identity differs from selected descriptor")
	}
	if v2Roles[role].host && !reflect.DeepEqual(record["anchor_identity"], b.anchorIdentity) {
		return errors.New("record anchor identity differs from selected descriptor")
	}
	if role == "trustedHostRoot" {
		selectedURI := v2FileURI(b.l.anchor.Path())
		if b.anchorIdentity["root_uri"] != selectedURI || record["anchor_uri"] != selectedURI {
			return errors.New("trusted host anchor URI differs from selected anchor descriptor path")
		}
	}
	var visit func(any, int) error
	visit = func(value any, depth int) error {
		if depth > 64 {
			return errors.New("V2 reference depth exceeded")
		}
		switch x := value.(type) {
		case map[string]any:
			if uri, has := x["role_uri"]; has {
				roleName := ""
				for candidate := range v2Roles {
					if uri == v2RoleURI(candidate) {
						roleName = candidate
						break
					}
				}
				if roleName == "" {
					return errors.New("V2 reference role not in supported subset")
				}
				selected := v2Roles[roleName]
				dir := b.l.root
				if selected.host {
					dir = b.l.anchor
				}
				selector, ok := x["selector"].(string)
				if !ok || !strings.HasPrefix(selector, "adr0011-"+selected.slug+"-v2-") || filepath.Base(selector) != selector {
					return errors.New("V2 referenced selector role mismatch")
				}
				if x["path"] != v2FileURI(filepath.Join(dir.Path(), selector)) || !reflect.DeepEqual(x["root_identity"], b.rootIdentity) {
					return errors.New("V2 reference path/root identity mismatch")
				}
				if selected.host {
					if !reflect.DeepEqual(x["anchor_identity"], b.anchorIdentity) {
						return errors.New("V2 host reference anchor identity mismatch")
					}
				} else if _, exists := x["anchor_identity"]; exists {
					return errors.New("V2 private reference has host anchor identity")
				}
				digest, ok := x["digest"].(string)
				if !ok || len(digest) != 71 || selector != "adr0011-"+selected.slug+"-v2-"+strings.TrimPrefix(digest, "sha256:")+".json" {
					return errors.New("V2 reference digest/selector mismatch")
				}
			}
			for _, child := range x {
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range x {
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(record, 0)
}
func (b *v2Binding) ref(role, selector string, raw []byte) (map[string]any, error) {
	selected, ok := v2Roles[role]
	if !ok {
		return nil, errors.New("V2 role not selected")
	}
	if len(raw) < 1 || len(raw) > 1048576 {
		return nil, errors.New("V2 ref byte limit invalid")
	}
	if selector != "adr0011-"+selected.slug+"-v2-"+strings.TrimPrefix(v2Digest(raw), "sha256:")+".json" {
		return nil, errors.New("V2 reference selector does not bind complete bytes")
	}
	dir := b.l.root
	schemaRole := "privateRef"
	if selected.host {
		dir = b.l.anchor
		schemaRole = "hostSlotRef"
	}
	ref := map[string]any{"role_uri": v2RoleURI(role), "selector": selector, "path": v2FileURI(filepath.Join(dir.Path(), selector)), "root_identity": b.rootIdentity, "digest": v2Digest(raw), "byte_length": int64(len(raw)), "no_follow": true, "single_link": true}
	if selected.host {
		ref["anchor_identity"] = b.anchorIdentity
	}
	if err := b.codec.refs[schemaRole].Validate(ref); err != nil {
		return nil, err
	}
	return ref, nil
}
func (b *v2Binding) readHeldLocked(role string, expected map[string]any) ([]byte, error) {
	selected, ok := v2Roles[role]
	if !ok {
		return nil, errors.New("V2 role not selected")
	}
	if err := b.l.validate(); err != nil {
		return nil, err
	}
	selector, ok := expected["selector"].(string)
	if !ok || len(selector) > 255 || len(selector) < 1 || filepath.Base(selector) != selector {
		return nil, errors.New("unsafe held selector")
	}
	dir := b.l.root
	if selected.host {
		dir = b.l.anchor
	}
	path := filepath.Join(dir.Path(), selector)
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm() != 0600 || !singleLink(before) {
		return nil, errors.New("V2 selector not owner-only single-link regular file")
	}
	f, err := openLockNoFollow(path)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) || after.Size() < 1 || after.Size() > 1048576 {
		_ = f.Close()
		return nil, errors.New("V2 selector identity or byte limit invalid")
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, 1048577))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil || len(raw) > 1048576 {
		return nil, errors.Join(readErr, closeErr, errors.New("V2 read/close unverified"))
	}
	record, err := b.codec.decode(role, selector, raw)
	if err != nil {
		return nil, err
	}
	if err := b.selectedIdentities(record, role); err != nil {
		return nil, err
	}
	actual, err := b.ref(role, selector, raw)
	if err != nil {
		return nil, err
	}
	if err := v2MatchHeldRef(expected, actual); err != nil {
		return nil, err
	}
	if err := b.l.validate(); err != nil {
		return nil, err
	}
	return raw, nil
}

// readHeld takes the checkpoint-A root lock; a self-consistent file never
// substitutes for the caller's independently held expected reference.
func (b *v2Binding) readHeld(role string, expected map[string]any) (raw []byte, err error) {
	if b == nil || b.l == nil {
		return nil, errors.New("nil V2 binding")
	}
	err = b.l.withLock(func() error { var e error; raw, e = b.readHeldLocked(role, expected); return e })
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// publishRecord does not infer V2 authority from schema validity. The caller
// supplies fields and owns all semantic antecedents not checked by this seam.
func (b *v2Binding) publishRecord(role string, fields map[string]any) (ref map[string]any, err error) {
	if b == nil || b.l == nil || b.codec == nil {
		return nil, errors.New("nil V2 binding")
	}
	selected, ok := v2Roles[role]
	if !ok {
		return nil, errors.New("V2 role not selected")
	}
	selector, raw, err := b.codec.encode(role, fields)
	if err != nil {
		return nil, err
	}
	normalized, err := b.codec.decode(role, selector, raw)
	if err != nil {
		return nil, err
	}
	if err := b.selectedIdentities(normalized, role); err != nil {
		return nil, err
	}
	expected, err := b.ref(role, selector, raw)
	if err != nil {
		return nil, err
	}
	dir := b.l.root
	if selected.host {
		dir = b.l.anchor
	}
	err = b.l.withLock(func() error {
		if err := b.l.validate(); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(dir.Path(), selector), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		n, writeErr := f.Write(raw)
		syncErr := error(nil)
		if writeErr == nil && n == len(raw) {
			syncErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil || n != len(raw) || syncErr != nil || closeErr != nil {
			return errors.Join(writeErr, syncErr, closeErr, errors.New("V2 publication write/close/sync unverified"))
		}
		syncDirectory := (*publication.Root).SyncDirectory
		if b.syncForTest != nil {
			syncDirectory = b.syncForTest
		}
		synced, err := syncDirectory(dir)
		if err != nil || !synced {
			return errors.Join(err, errors.New("V2 parent sync unverified"))
		}
		got, err := b.readHeldLocked(role, expected)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, got) {
			return fmt.Errorf("V2 exact readback differs")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return expected, nil
}
