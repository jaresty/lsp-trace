//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type completeDescribeDestinationConfigV1 struct {
	Path     string
	Identity publication.RetentionDirectoryIdentity
}

// Trusted bootstrap declaration, never evaluator packet authority. ProtectedRoots
// is the independently supplied COMPLETE private-root set; Workspace is separate.
// No Manager, process, model, HOST, worker, mailbox, Prepare or execution knobs.
type privateSourceCompleteDescribeConfig struct {
	Schema, AccountingRoot, ExpectedHead, Attempt, Workspace, SuppliedFile string
	CanonicalJob                                                           []byte
	Verification                                                           inv.PrivateManagedInputVerificationV2
	Open                                                                   inv.PreparedDescribeOpenRequestV1
	Destinations                                                           [2]completeDescribeDestinationConfigV1
	ProtectedRoots                                                         []string
}

func decodePrivateSourceCompleteDescribeConfig(raw []byte) (privateSourceCompleteDescribeConfig, error) {
	var c privateSourceCompleteDescribeConfig
	fail := func() (privateSourceCompleteDescribeConfig, error) {
		return privateSourceCompleteDescribeConfig{}, inv.ErrCompleteDescribeV1
	}
	if len(raw) == 0 || len(raw) > 64<<10 {
		return fail()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil {
		return fail()
	}
	canonical, e := json.Marshal(c)
	if e != nil || !bytes.Equal(raw, canonical) || c.Schema != "private-source-complete-describe.v1" || len(c.CanonicalJob) == 0 || len(c.CanonicalJob) > 32<<10 || c.Open.Anchors.RootPath != c.AccountingRoot || c.Open.Deadline.IsZero() || len(c.ProtectedRoots) < 1 || len(c.ProtectedRoots) > 15 {
		return fail()
	}
	paths := []string{c.AccountingRoot, c.Workspace, c.SuppliedFile, c.Destinations[0].Path, c.Destinations[1].Path}
	paths = append(paths, c.ProtectedRoots...)
	for _, p := range paths {
		if len(p) == 0 || len(p) > 8192 || !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return fail()
		}
	}
	return c, nil
}
func openPrivateCompleteFileV1(path string, limit int64) (*os.File, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() < 1 || info.Size() > limit {
		return nil, errors.Join(inv.ErrCompleteDescribeV1, e, f.Close())
	}
	return f, nil
}

type privateCompleteDescribeOwnedV1 struct {
	root      *publication.Root
	owner     *inv.PrivateManagedInputOwnerV2
	outputs   [2]*publication.ContentRetentionRoot
	source    *publication.SourceProtectionRootPrivateV2
	protected []*publication.ObservedRoot
	data      []*os.File
	// Invocation-local reports run AFTER the actual owned close, never instead.
	// No public callback, authority result or mutable global hook is exposed.
	report func(kind string, index int) error
}

func (h *privateCompleteDescribeOwnedV1) close() error {
	var e error
	report := func(kind string, index int) {
		if h.report != nil {
			e = errors.Join(e, h.report(kind, index))
		}
	}
	for i, f := range h.data {
		if f != nil {
			e = errors.Join(e, f.Close())
			report("data", i)
		}
	}
	h.data = nil
	for i, r := range h.outputs {
		if r != nil {
			e = errors.Join(e, r.Close())
			report("output", i)
		}
	}
	h.outputs = [2]*publication.ContentRetentionRoot{}
	for i, r := range h.protected {
		if r != nil {
			e = errors.Join(e, r.Close())
			report("protected", i)
		}
	}
	h.protected = nil
	if h.source != nil {
		e = errors.Join(e, h.source.Close())
		h.source = nil
		report("source", 0)
	}
	if h.root != nil {
		e = errors.Join(e, h.root.Close())
		h.root = nil
		report("root", 0)
	}
	return e
}
func privateSourceCompleteDescribeOwned(ctx context.Context, c privateSourceCompleteDescribeConfig, h *privateCompleteDescribeOwnedV1, supplied *os.File) (inv.CompletedDescribeV1, error) {
	control, cancel := context.WithDeadline(ctx, c.Open.Deadline)
	defer cancel()
	q := inv.CompleteDescribeRequestV1{Root: h.root, Open: c.Open, ExpectedHead: c.ExpectedHead, Attempt: c.Attempt, Outputs: h.outputs, DestinationIdentities: [2]publication.RetentionDirectoryIdentity{c.Destinations[0].Identity, c.Destinations[1].Identity}, Source: h.source, Protected: h.protected}
	op, e := h.owner.ReserveCompleteDescribeV1(control, q)
	var result inv.CompletedDescribeV1
	if e == nil {
		// Actual supplied-file IO is after both paid Open and NEW Complete reserve.
		var raw []byte
		raw, e = io.ReadAll(io.LimitReader(supplied, (1<<20)+1))
		if e == nil {
			result, e = inv.CompleteDescribeV1(control, op, raw)
		}
	}
	e = errors.Join(e, h.close(), control.Err(), ctx.Err())
	cancel()
	if control.Err() != context.Canceled || ctx.Err() != nil || !time.Now().Before(c.Open.Deadline) {
		e = errors.Join(e, inv.ErrCompleteDescribeV1)
	}
	if e != nil {
		return inv.CompletedDescribeV1{}, e
	}
	return result, nil
}
func finishPrivateSourceCompleteDescribe(ctx context.Context, deadline time.Time, result inv.CompletedDescribeV1, e error, stdout, stderr io.Writer) int {
	fail := func(class string) int { fmt.Fprintln(stderr, "private Complete:", class); return 1 }
	if e != nil || result.ObjectID == "" || ctx.Err() != nil || !time.Now().Before(deadline) {
		return fail("REFUSED_OR_CONSUMED_PENDING_UNCERTAIN")
	}
	// Bounded detached receipt encoding/output is covered by Complete's quote.
	raw, e := json.Marshal(result)
	if e != nil || len(raw)+1 > 1<<20 {
		return fail("OUTPUT_REFUSED")
	}
	raw = append(raw, '\n')
	n, e := stdout.Write(raw)
	if e != nil || n != len(raw) || ctx.Err() != nil || !time.Now().Before(deadline) {
		return fail("OUTPUT_FAILED_POSSIBLY_PARTIAL")
	}
	return 0
}
func executePrivateSourceCompleteDescribe(ctx context.Context, c privateSourceCompleteDescribeConfig, configFile *os.File, stdout, stderr io.Writer) (code int) {
	h := &privateCompleteDescribeOwnedV1{data: []*os.File{configFile}}
	defer func() {
		if e := h.close(); e != nil {
			code = 1
		}
	}()
	fail := func(class string) int { fmt.Fprintln(stderr, "private Complete:", class); return 1 }
	if ctx == nil || ctx.Err() != nil || !time.Now().Before(c.Open.Deadline) || time.Until(c.Open.Deadline) > 15*time.Second {
		return fail("FRESH_ADMISSION_REFUSED")
	}
	control, cancel := context.WithDeadline(ctx, c.Open.Deadline)
	defer cancel()
	var e error
	h.root, e = publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		return fail("ROOT_REFUSED")
	}
	h.owner, e = inv.OpenPrivateManagedInputOwnerV2(control, h.root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if e != nil {
		return fail("CANONICAL_OWNER_REFUSED")
	}
	h.source, e = publication.OpenSourceProtectionRootPrivateV2(control, c.Workspace)
	if e != nil {
		return fail("SOURCE_PROTECTION_REFUSED")
	}
	for _, path := range c.ProtectedRoots {
		r, e := publication.OpenObservedRoot(control, path)
		if e != nil {
			return fail("PROTECTED_ROOT_REFUSED")
		}
		h.protected = append(h.protected, r)
	}
	for i, d := range c.Destinations {
		h.outputs[i], e = publication.OpenContentRetentionRoot(control, d.Path)
		if e != nil {
			return fail("DESTINATION_REFUSED")
		}
	}
	supplied, e := openPrivateCompleteFileV1(c.SuppliedFile, 1<<20)
	if e != nil {
		return fail("SUPPLIED_FILE_REFUSED")
	}
	h.data = append(h.data, supplied)
	result, e := privateSourceCompleteDescribeOwned(control, c, h, supplied)
	return finishPrivateSourceCompleteDescribe(control, c.Open.Deadline, result, e, stdout, stderr)
}
func runPrivateSourceCompleteDescribe(args []string, stdout, stderr io.Writer) int {
	fail := func() int { fmt.Fprintln(stderr, "private Complete: CONFIG_REFUSED"); return 1 }
	if len(args) != 2 || args[0] != "source-complete-describe" {
		return fail()
	}
	f, e := openPrivateCompleteFileV1(args[1], 64<<10)
	if e != nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	c, decodeErr := decodePrivateSourceCompleteDescribeConfig(raw)
	if e != nil || decodeErr != nil {
		e = errors.Join(e, decodeErr, f.Close())
		return fail()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return executePrivateSourceCompleteDescribe(ctx, c, f, stdout, stderr)
}
