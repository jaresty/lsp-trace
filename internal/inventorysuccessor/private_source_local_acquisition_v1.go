//go:build darwin || linux

package inventorysuccessor

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var ErrPrivateLocalAcquisitionV1 = errors.New("private local candidate acquisition refused; debit/partial artifacts may remain")

const acquisitionWallV1 = 3 * time.Minute
const acquisitionHopsV1 uint64 = 8          // two assets, at most four GETs each
const acquisitionHeaderV1 uint64 = 32 << 10 // includes rejecting parser/header slack
const acquisitionMetadataV1 uint64 = 512    // 128 preparation + 128 cleanup + bounded dial/deadline/close slack

type PrivateLocalAcquisitionRequestV1 struct {
	// Existing separately owned private directory, disjoint from accounting. No
	// arbitrary extraction, install, discovery, resume or overwrite is supported.
	StorageRoot, Attempt, ExpectedHead string
	Deadline                           time.Time
}
type localAcquisitionPinV1 struct {
	URL, Name, SHA256 string
	Bytes             uint64
}

// PrivateLocalAcquisitionV1 is retained accounting custody, not runtime admission.
// Copies share retirement. No manual completion, renewal or source credit exists.
// The root/context must survive until Retire; the lifecycle stays PENDING meanwhile.
type PrivateLocalAcquisitionV1 struct{ state *localAcquisitionStateV1 }
type localAcquisitionStateV1 struct {
	inspectionIssued, inspectionFailed bool
	activeUses                         uint64
	namespace                          *localStageNamespaceV1
	account                            *os.File
	accountInfo                        *syscall.Stat_t
	files                              [2]*os.File
	namespaceIO                        privateLocalIOV1
	dialObserver                       acquisitionDialObserverV1
	mu                                 sync.Mutex
	owner                              *PrivateManagedInputOwnerV2
	ctx                                context.Context
	deadline                           time.Time
	head, attempt, dirname, storage    string
	root, dir                          *os.File
	ids                                [2]syscall.Stat_t
	present                            [2]bool
	pins                               [2]localAcquisitionPinV1
	budget                             *acquisitionBudgetV1
	retired, made                      bool
	transportUncertain                 bool
}

func localAcquisitionPinsV1() [2]localAcquisitionPinV1 {
	return [2]localAcquisitionPinV1{
		{"https://huggingface.co/bartowski/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/1f629da0c8bed16b9e50cee91c70693650e66c35/Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf", "Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf", "sha256:1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6", 4683074336},
		{"https://github.com/ggml-org/llama.cpp/releases/download/b10809/llama-b10809-bin-macos-arm64.tar.gz", "llama-b10809-bin-macos-arm64.tar.gz", "sha256:7d692df9e1e386e62f1c12b843903218041e6cd74c9415aa39a7ed3176f9eaa2", 11123196},
	}
}

// All fourteen logical currencies are derived, not caller expenditure knobs.
// T=sum(length+1) pays rejecting EOF bytes; B=sum(ceil(length/32768)+4096)
// bounds actual body Read, file Write AND hash Write attempts, including short/zero reads.
// W=2*T+8MiB is an ENFORCED aggregate raw TCP read/write envelope (TLS included),
// NOT a claim that arbitrary servers or TLS transcripts fit it. Raw connections
// have 4*B+32768 attempts, charged BEFORE every actual Read/Write. Dial attempts,
// GET/hops, header parsing, metadata, same-FD hashing and fixed cleanup are prepaid.
// DNS/DialContext and filesystem calls remain OS synchronous operations: these
// logical method units/deadlines do NOT establish physical syscall/RAM/CPU bounds.
// Sixteen trusted-history envelopes cover reserve, lease reopen and terminal
// observe/readback, including their bounded parsing/hashing/rejecting effects;
// setup initialization/Open is separately caller-owned, never authorized here.
// The kernel's ceilings/windows/currency definitions and source formula do not change.
func localAcquisitionQuoteV1(p [2]localAcquisitionPinV1, l verificationLimitsV2) (countersV2, error) {
	if _, e := budgetV2(l); e != nil {
		return countersV2{}, e
	}
	var total, body uint64
	for i, a := range p {
		u, e := url.Parse(a.URL)
		if e != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" || len(a.URL) > 8192 || a.Name == "." || a.Name == "" || len(a.Name) > 128 || filepath.Base(a.Name) != a.Name || strings.ContainsAny(a.Name, "\\/") || !hashV2(a.SHA256) || a.Bytes == 0 || a.Bytes > 5<<30 || (i == 1 && a.Bytes > acquisitionArchiveMaxV1) || (i == 1 && a.Name == p[0].Name) {
			return countersV2{}, ErrPrivateLocalAcquisitionV1
		}
		total += a.Bytes + 1
		body += (a.Bytes+32767)/32768 + 4096
	}
	wire := 2*total + acquisitionHopsV1*(1<<20)
	rawCalls := 4*body + 32768
	namespaceOps := acquisitionNamespaceChecksV1(p) * acquisitionNamespaceOpsV1
	metadata := (acquisitionMetadataV1 + namespaceOps) * 4096
	verification := 16 * (4*l.Bytes + 32*headLimitV2)
	inspectionBytes := 2*(p[1].Bytes+1) + 2*acquisitionExpandedV1 + acquisitionArchiveReportV1
	inspectionCalls := 2*acquisitionInspectionCallsV1 + acquisitionExpandedCallsV1 + 64
	records := 4*l.Records + acquisitionMetadataV1 + namespaceOps + acquisitionHopsV1 + 16
	calls := rawCalls + 3*body + inspectionCalls + namespaceOps + acquisitionMetadataV1 + 4*l.Work + 64 + 2*acquisitionHopsV1
	request := acquisitionHopsV1 * 16384
	response := total + acquisitionHopsV1*acquisitionHeaderV1
	// Operands above have validated hard maxima (<2^40 lengths and <2^27 trusted
	// bytes); each sum/product below is <2^48, including all metadata and work terms.
	return countersV2{
		LogicalBytes:  4*total + wire + metadata + verification + inspectionBytes,
		ReadHashBytes: 2*total + wire + metadata + verification + 2*(p[1].Bytes+1) + acquisitionExpandedV1,
		WriteBytes:    total + 4*(recordLimitV2+2*headLimitV2), Records: records,
		ParserBytes:      wire + acquisitionHopsV1*acquisitionHeaderV1 + metadata + verification + p[1].Bytes + 1 + acquisitionExpandedV1,
		ProviderRequests: acquisitionHopsV1, ProviderMessages: 2 * acquisitionHopsV1,
		Work: 4*total + wire + metadata + verification + calls + inspectionBytes, ActiveNS: uint64(acquisitionWallV1), Calls: calls,
		RequestBytes: request, ResponseBytes: response, WireBytes: wire,
		OutputBytes: total + metadata + 8*(headLimitV2+recordLimitV2) + 128*l.Records + acquisitionArchiveReportV1,
	}, nil
}

// QuotePrivateLocalCandidateAcquisitionV1 returns a quote, NEVER credit. The
// caller must explicitly initialize an independently funded job with these terms.
func QuotePrivateLocalCandidateAcquisitionV1(l PrivateManagedInputVerificationV2) ([14]uint64, error) {
	c, e := localAcquisitionQuoteV1(localAcquisitionPinsV1(), verificationLimitsV2{l.Records, l.Bytes, l.Work})
	return c.vector(), e
}

type acquisitionBudgetV1 struct {
	archiveHashes                                                                     uint64
	archiveReads, expandedReads                                                       uint64
	namespace                                                                         uint64
	hashWrites                                                                        uint64
	activeMethods                                                                     uint64
	mu                                                                                sync.Mutex
	wire, rawCalls, bodyReads, fileWrites, requests, dials, metadata, cleanup, closes uint64
	connections                                                                       []*acquisitionConnV1
	notify                                                                            chan struct{}
	activeDials                                                                       uint64
	stopping                                                                          bool
}

func newAcquisitionBudgetV1(p [2]localAcquisitionPinV1) *acquisitionBudgetV1 {
	var n, b uint64
	for _, a := range p {
		n += a.Bytes + 1
		b += (a.Bytes+32767)/32768 + 4096
	}
	return &acquisitionBudgetV1{archiveHashes: acquisitionInspectionCallsV1, archiveReads: acquisitionInspectionCallsV1, expandedReads: acquisitionExpandedCallsV1, namespace: acquisitionNamespaceChecksV1(p), wire: 2*n + acquisitionHopsV1*(1<<20), rawCalls: 4*b + 32768, bodyReads: b, fileWrites: b, hashWrites: b, requests: 8, dials: 8, metadata: 128, cleanup: 128, closes: 8, notify: make(chan struct{}, 1)}
}
func (b *acquisitionBudgetV1) take(n *uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if *n == 0 {
		return ErrPrivateLocalAcquisitionV1
	}
	*n--
	return nil
}

type acquisitionConnV1 struct {
	closing  bool // protected by b.mu
	deadline time.Time
	net.Conn
	b        *acquisitionBudgetV1
	once     sync.Once
	done     chan struct{}
	closeErr error // published by closing done
}

func (c *acquisitionConnV1) Close() error {
	c.once.Do(func() {
		c.b.mu.Lock()
		c.closing = true
		c.b.mu.Unlock()
		c.closeErr = c.b.take(&c.b.closes)
		if c.closeErr == nil {
			c.closeErr = c.Conn.Close()
		}
		close(c.done)
		select {
		case c.b.notify <- struct{}{}:
		default:
		}
	})
	return c.closeErr
}

// Join the actual owned raw connections and in-flight DialContext invocations,
// without a new clock. A close error or uncertain join withholds success.
func (s *localAcquisitionStateV1) stopConnections() error {
	s.budget.mu.Lock()
	s.budget.stopping = true
	conns := append([]*acquisitionConnV1(nil), s.budget.connections...)
	s.budget.mu.Unlock()
	var out error
	for _, c := range conns {
		out = errors.Join(out, c.Close())
	}
	return errors.Join(out, s.joinConnections())
}
func (s *localAcquisitionStateV1) joinConnections() error {
	for {
		s.budget.mu.Lock()
		conns := append([]*acquisitionConnV1(nil), s.budget.connections...)
		busy := s.budget.activeDials != 0 || s.budget.activeMethods != 0
		s.budget.mu.Unlock()
		var out error
		for _, c := range conns {
			select {
			case <-c.done:
				out = errors.Join(out, c.closeErr)
			default:
				busy = true
			}
		}
		if !busy {
			return out
		}
		if !s.live() {
			return errors.Join(out, ErrPrivateLocalAcquisitionV1)
		}
		t := time.NewTimer(time.Until(s.deadline))
		select {
		case <-s.budget.notify:
			if !t.Stop() {
				<-t.C
			}
		case <-s.ctx.Done():
			t.Stop()
			return ErrPrivateLocalAcquisitionV1
		case <-t.C:
			return ErrPrivateLocalAcquisitionV1
		}
	}
}

func (c *acquisitionConnV1) transfer(p []byte, write bool) (int, error) {
	c.b.mu.Lock()
	if c.closing || c.b.rawCalls == 0 || c.b.wire == 0 {
		c.b.mu.Unlock()
		return 0, ErrPrivateLocalAcquisitionV1
	}
	c.b.rawCalls--
	n := uint64(len(p))
	if n > 32768 {
		n = 32768
	}
	if n > c.b.wire {
		n = c.b.wire
	}
	c.b.wire -= n
	c.b.activeMethods++
	c.b.mu.Unlock()
	defer c.endMethod()
	// Predebit requested capacity, not returned bytes. No credit is restored after
	// short/error results; concurrent TLS read/write share the aggregate envelope.
	if write {
		return c.Conn.Write(p[:int(n)])
	}
	return c.Conn.Read(p[:int(n)])
}
func (c *acquisitionConnV1) endMethod() {
	c.b.mu.Lock()
	c.b.activeMethods--
	c.b.mu.Unlock()
	select {
	case c.b.notify <- struct{}{}:
	default:
	}
}
func (c *acquisitionConnV1) setDeadline(t time.Time, kind int) error {
	c.b.mu.Lock()
	if c.closing || c.b.metadata == 0 {
		c.b.mu.Unlock()
		return ErrPrivateLocalAcquisitionV1
	}
	c.b.metadata--
	c.b.activeMethods++
	c.b.mu.Unlock()
	defer c.endMethod()
	if !c.deadline.IsZero() && (t.IsZero() || t.After(c.deadline)) {
		t = c.deadline
	}
	switch kind {
	case 0:
		return c.Conn.SetDeadline(t)
	case 1:
		return c.Conn.SetReadDeadline(t)
	default:
		return c.Conn.SetWriteDeadline(t)
	}
}
func (c *acquisitionConnV1) SetDeadline(t time.Time) error      { return c.setDeadline(t, 0) }
func (c *acquisitionConnV1) SetReadDeadline(t time.Time) error  { return c.setDeadline(t, 1) }
func (c *acquisitionConnV1) SetWriteDeadline(t time.Time) error { return c.setDeadline(t, 2) }
func (c *acquisitionConnV1) Read(p []byte) (int, error)         { return c.transfer(p, false) }
func (c *acquisitionConnV1) Write(p []byte) (int, error)        { return c.transfer(p, true) }
func (s *localAcquisitionStateV1) live() bool {
	return s.ctx.Err() == nil && time.Now().Before(s.deadline)
}
func (s *localAcquisitionStateV1) step(clean bool) error {
	if clean {
		return s.budget.take(&s.budget.cleanup)
	}
	if !s.live() {
		return ErrPrivateLocalAcquisitionV1
	}
	return s.budget.take(&s.budget.metadata)
}

// A ticket prepays a whole bounded accepted-primitive validation. Each path
// has <=128 components. Two protected walks, two <=129-parent identity walks,
// Fstat/ACL/open/close and fixed child checks fit <=4096 primitive attempts.
const acquisitionNamespaceOpsV1 uint64 = 4096
const acquisitionInspectionCallsV1 uint64 = 8192

func acquisitionNamespaceChecksV1(p [2]localAcquisitionPinV1) uint64 {
	var b uint64
	for _, a := range p {
		b += (a.Bytes+32767)/32768 + 4096
	}
	return b + acquisitionInspectionCallsV1 + 64
}
func (s *localAcquisitionStateV1) namespaceTicket(clean bool) error {
	if !clean && !s.live() {
		return ErrPrivateLocalAcquisitionV1
	}
	return s.budget.take(&s.budget.namespace)
}
func acquisitionDirectoryAncestryV1(f *os.File, scratch *[4096]byte, owner privateLocalIOV1) (out []*syscall.Stat_t, err error) {
	fd, e := unix.Openat(int(f.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	current := os.NewFile(uintptr(fd), "acquisition-ancestry")
	defer func() {
		if current != nil {
			err = errors.Join(err, current.Close())
		}
	}()
	for depth := 0; depth < 129; depth++ {
		st, e := statLocalNamespaceDirectoryV1(current, owner)
		if e != nil || !protectedLocalDirectoryInfoV1(st, false) {
			return nil, errors.Join(e, ErrPrivateLocalAcquisitionV1)
		}
		if e = localDirectoryACLAdmissionV1(current, scratch); e != nil {
			return nil, e
		}
		if len(out) > 0 && out[len(out)-1].Dev == st.Dev && out[len(out)-1].Ino == st.Ino {
			return out, nil
		}
		out = append(out, st)
		fd, e = unix.Openat(int(current.Fd()), "..", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if e != nil {
			return nil, e
		}
		next := os.NewFile(uintptr(fd), "acquisition-ancestor")
		ce := current.Close()
		current = next
		if ce != nil {
			return nil, ce
		}
	}
	return nil, ErrPrivateLocalAcquisitionV1
}
func acquisitionAncestryDisjointV1(a, b []*syscall.Stat_t) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for _, st := range a {
		if st.Dev == b[0].Dev && st.Ino == b[0].Ino {
			return false
		}
	}
	for _, st := range b {
		if st.Dev == a[0].Dev && st.Ino == a[0].Ino {
			return false
		}
	}
	return true
}
func (s *localAcquisitionStateV1) validateNamespace(clean bool) error {
	if e := s.namespaceTicket(clean); e != nil {
		return e
	}
	if s.namespace == nil || s.root == nil || s.account == nil {
		return ErrPrivateLocalAcquisitionV1
	}
	// Exact accepted implementation: protected ancestry, held descriptor Fstat/ACL,
	// and identity-bound child reopen; no unchecked pathname is used for writes.
	check := privateSourceRPreparationStateV1{localStage: s.namespace, local: &PrivateSourceLocalConfigV1{io: s.namespaceIO}}
	if e := check.validateLocalStageV1(); e != nil {
		return e
	}
	f, info, e := openProtectedLocalDirectoryV1(s.owner.k.root.Path(), &s.namespace.aclScratch, s.namespaceIO)
	if e != nil {
		return e
	}
	ce := f.Close()
	if ce != nil || !sameLocalDirectoryV1(s.accountInfo, info, true) {
		return errors.Join(ce, ErrPrivateLocalAcquisitionV1)
	}
	held, e := statLocalNamespaceDirectoryV1(s.account, s.namespaceIO)
	if e != nil || !sameLocalDirectoryV1(s.accountInfo, held, true) {
		return errors.Join(e, ErrPrivateLocalAcquisitionV1)
	}
	return localDirectoryACLAdmissionV1(s.account, &s.namespace.aclScratch)
}
func (s *localAcquisitionStateV1) cleanup() error {
	var out error
	// A failed preparation without created children has only acquired FDs to close.
	if s.made {
		if s.dir == nil {
			out = ErrPrivateLocalAcquisitionV1
		} else {
			out = s.validateNamespace(true)
		}
		if out == nil {
			for i, p := range s.pins {
				if s.present[i] {
					var st unix.Stat_t
					e := s.step(true)
					if e == nil {
						e = unix.Fstatat(int(s.dir.Fd()), p.Name, &st, unix.AT_SYMLINK_NOFOLLOW)
					}
					if e == nil && uint64(st.Dev) == uint64(s.ids[i].Dev) && uint64(st.Ino) == uint64(s.ids[i].Ino) && st.Mode&unix.S_IFMT == unix.S_IFREG && st.Uid == uint32(os.Geteuid()) {
						if e = s.step(true); e == nil {
							e = unix.Unlinkat(int(s.dir.Fd()), p.Name, 0)
						}
						if e == nil {
							s.present[i] = false
						}
					} else {
						e = errors.Join(e, ErrPrivateLocalAcquisitionV1)
					}
					out = errors.Join(out, e)
				}
			}
		}
		if out == nil {
			if e := s.step(true); e != nil {
				out = e
			} else {
				out = unix.Unlinkat(int(s.root.Fd()), s.dirname, unix.AT_REMOVEDIR)
			}
		}
	}
	// Fixed known handles only. No recursive namespace deletion, retries or renewal.
	for i, f := range s.files {
		if f != nil {
			s.files[i] = nil
			e := s.step(true)
			if e == nil {
				e = f.Close()
			}
			out = errors.Join(out, e)
		}
	}
	for _, p := range []**os.File{&s.dir, &s.root, &s.account} {
		if *p != nil {
			f := *p
			*p = nil
			e := s.step(true)
			if e == nil {
				e = f.Close()
			}
			out = errors.Join(out, e)
		}
	}
	if s.namespace != nil {
		s.namespace.parent = nil
		s.namespace.dir = nil
	}
	return out
}
func (s *localAcquisitionStateV1) prepare(storage string, total uint64) error {
	if e := s.namespaceTicket(false); e != nil {
		return e
	}
	ns := &localStageNamespaceV1{parentPath: storage, name: s.dirname}
	s.namespace = ns
	var e error
	ns.parent, ns.parentInfo, e = openProtectedLocalDirectoryV1(storage, &ns.aclScratch, s.namespaceIO)
	if e != nil {
		return e
	}
	s.root = ns.parent
	if !protectedLocalDirectoryInfoV1(ns.parentInfo, true) {
		return ErrPrivateLocalAcquisitionV1
	}
	s.account, s.accountInfo, e = openProtectedLocalDirectoryV1(s.owner.k.root.Path(), &ns.aclScratch, s.namespaceIO)
	if e != nil {
		return e
	}
	if !protectedLocalDirectoryInfoV1(s.accountInfo, true) {
		return ErrPrivateLocalAcquisitionV1
	}
	a, e := acquisitionDirectoryAncestryV1(s.root, &ns.aclScratch, s.namespaceIO)
	if e != nil {
		return e
	}
	b, e := acquisitionDirectoryAncestryV1(s.account, &ns.aclScratch, s.namespaceIO)
	if e != nil {
		return e
	}
	if !acquisitionAncestryDisjointV1(a, b) {
		return ErrPrivateLocalAcquisitionV1
	}
	var fs unix.Statfs_t
	if e = s.step(false); e == nil {
		e = unix.Fstatfs(int(s.root.Fd()), &fs)
	}
	if e != nil || fs.Bsize <= 0 || uint64(fs.Bavail) > ^uint64(0)/uint64(fs.Bsize) || uint64(fs.Bavail)*uint64(fs.Bsize) < total+(1<<20) {
		return errors.Join(e, ErrPrivateLocalAcquisitionV1)
	}
	if e = s.step(false); e == nil {
		e = unix.Mkdirat(int(s.root.Fd()), s.dirname, 0700)
	}
	if e != nil {
		return e
	}
	s.made = true
	ns.created = true
	ns.dir, ns.dirInfo, e = openOwnedStageDirectoryV1(s.root, s.dirname, &ns.aclScratch, s.namespaceIO)
	s.dir = ns.dir
	if e != nil {
		return e
	}
	return s.validateNamespace(false)
}
func acquisitionRedirectAllowedV1(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || len(u.String()) > 8192 {
		return false
	}
	switch u.Hostname() {
	case "huggingface.co", "cas-bridge.xethub.hf.co", "cdn-lfs.huggingface.co", "cdn-lfs-us-1.hf.co", "us.aws.cdn.hf.co", "github.com", "release-assets.githubusercontent.com":
		return true
	}
	return false
}
func (s *localAcquisitionStateV1) download(c *http.Client, i int) (err error) {
	pin := s.pins[i]
	current := pin.URL
	var body io.ReadCloser
	for hop := 0; hop < 4; hop++ {
		if !s.live() {
			return ErrPrivateLocalAcquisitionV1
		}
		if e := s.budget.take(&s.budget.requests); e != nil {
			return e
		}
		// Exactly one observed response (including rejection) per prepaid GET.
		// Do not silently process an unbounded stream of interim messages.
		requestCtx := httptrace.WithClientTrace(s.ctx, &httptrace.ClientTrace{Got1xxResponse: func(int, textproto.MIMEHeader) error { return ErrPrivateLocalAcquisitionV1 }})
		req, e := http.NewRequestWithContext(requestCtx, http.MethodGet, current, nil)
		if e != nil {
			return e
		}
		req.Header.Set("Accept-Encoding", "identity")
		resp, e := c.Do(req)
		if e != nil {
			return e
		}
		if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
			u, ue := resp.Location()
			ce := resp.Body.Close()
			if ue != nil || ce != nil || hop == 3 || !acquisitionRedirectAllowedV1(u) {
				return errors.Join(ErrPrivateLocalAcquisitionV1, ue, ce)
			}
			current = u.String()
			continue
		}
		if resp.StatusCode != 200 || resp.ContentLength != int64(pin.Bytes) || resp.Header.Get("Content-Encoding") != "" {
			return errors.Join(ErrPrivateLocalAcquisitionV1, resp.Body.Close())
		}
		body = resp.Body
		break
	}
	if body == nil {
		return ErrPrivateLocalAcquisitionV1
	}
	defer func() { err = errors.Join(err, body.Close()) }()
	if e := s.validateNamespace(false); e != nil {
		return e
	}
	if e := s.step(false); e != nil {
		return e
	}
	fd, e := unix.Openat(int(s.dir.Fd()), pin.Name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	f := os.NewFile(uintptr(fd), "acquisition-file")
	s.files[i] = f
	if e = s.step(false); e != nil {
		return e
	}
	st, e := statLocalNamespaceDirectoryV1(f, s.namespaceIO)
	if e != nil {
		return e
	}
	s.ids[i] = *st
	s.present[i] = true
	h := sha256.New()
	buf := make([]byte, 32768)
	var count uint64
	for {
		if !s.live() {
			return ErrPrivateLocalAcquisitionV1
		}
		if e = s.budget.take(&s.budget.bodyReads); e != nil {
			return e
		}
		want := pin.Bytes - count + 1
		if want > uint64(len(buf)) {
			want = uint64(len(buf))
		}
		n, re := body.Read(buf[:int(want)])
		if n < 0 || uint64(n) > want || uint64(n) > pin.Bytes-count {
			return ErrPrivateLocalAcquisitionV1
		}
		if n > 0 {
			if e = s.validateNamespace(false); e != nil {
				return e
			}
			if e = s.budget.take(&s.budget.fileWrites); e != nil {
				return e
			}
			written, we := f.Write(buf[:n])
			if we != nil || written != n {
				return errors.Join(ErrPrivateLocalAcquisitionV1, we)
			}
			if e = s.budget.take(&s.budget.hashWrites); e != nil {
				return e
			}
			_, _ = h.Write(buf[:n])
			count += uint64(n)
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			return re
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	if count != pin.Bytes || fmt.Sprintf("sha256:%x", h.Sum(nil)) != pin.SHA256 || !s.live() {
		return ErrPrivateLocalAcquisitionV1
	}
	if e = s.step(false); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = s.validateNamespace(false); e != nil {
		return e
	}
	if e = s.step(false); e != nil {
		return e
	}
	st, e = statLocalNamespaceDirectoryV1(f, s.namespaceIO)
	if e != nil || st.Size != int64(pin.Bytes) || st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Nlink != 1 || st.Uid != uint32(os.Geteuid()) {
		return errors.Join(e, ErrPrivateLocalAcquisitionV1)
	}
	s.ids[i] = *st
	return nil
}

// Observation only: implemented in tests; never replaces the actual net.Dialer.
type acquisitionDialObserverV1 interface {
	beforeDialAdmission()
	afterDialAdmission()
}

func (s *localAcquisitionStateV1) dialContext(c context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Deadline: s.deadline}
	if s.dialObserver != nil {
		s.dialObserver.beforeDialAdmission()
	}
	// One admission critical section: stop cannot observe an empty set while a
	// prepaid underlying invocation is between admission and registration.
	s.budget.mu.Lock()
	if s.budget.stopping || !s.live() || c.Err() != nil || s.budget.dials == 0 {
		s.budget.mu.Unlock()
		return nil, ErrPrivateLocalAcquisitionV1
	}
	s.budget.dials--
	s.budget.activeDials++
	s.budget.mu.Unlock()
	if s.dialObserver != nil {
		s.dialObserver.afterDialAdmission()
	}
	defer func() {
		s.budget.mu.Lock()
		s.budget.activeDials--
		s.budget.mu.Unlock()
		select {
		case s.budget.notify <- struct{}{}:
		default:
		}
	}()
	conn, e := dialer.DialContext(c, network, addr)
	if e != nil {
		return nil, e
	}
	owned := &acquisitionConnV1{Conn: conn, b: s.budget, done: make(chan struct{}), deadline: s.deadline}
	s.budget.mu.Lock()
	s.budget.connections = append(s.budget.connections, owned)
	stopping := s.budget.stopping
	s.budget.mu.Unlock()
	if stopping {
		return nil, errors.Join(ErrPrivateLocalAcquisitionV1, owned.Close())
	}
	if e = s.step(false); e == nil {
		e = conn.SetDeadline(s.deadline)
	}
	if e != nil {
		return nil, errors.Join(e, owned.Close())
	}
	return owned, nil
}

func (o *PrivateManagedInputOwnerV2) AcquireLocalCandidateV1(ctx context.Context, q PrivateLocalAcquisitionRequestV1) (PrivateLocalAcquisitionV1, error) {
	return o.acquireLocalCandidateV1(ctx, q, localAcquisitionPinsV1())
}

// Only the fixed public entry is production-reachable. Invocation-local tiny
// loopback pins exercise exactly this continuation in package tests; no mutable
// global, exported client/runner, grant callback or manual completion is added.
func (o *PrivateManagedInputOwnerV2) acquireLocalCandidateV1(ctx context.Context, q PrivateLocalAcquisitionRequestV1, pins [2]localAcquisitionPinV1) (PrivateLocalAcquisitionV1, error) {
	fail := func(e error) (PrivateLocalAcquisitionV1, error) {
		return PrivateLocalAcquisitionV1{}, errors.Join(ErrPrivateLocalAcquisitionV1, e)
	}
	now := time.Now()
	if o == nil || ctx == nil || ctx.Err() != nil || !hashV2(q.Attempt) || !hashV2(q.ExpectedHead) || !filepath.IsAbs(q.StorageRoot) || filepath.Clean(q.StorageRoot) != q.StorageRoot || len(q.StorageRoot) > 4096 || !now.Before(q.Deadline) || q.Deadline.Sub(now) > acquisitionWallV1 {
		return fail(nil)
	}
	if d, ok := ctx.Deadline(); !ok || d.Before(q.Deadline) {
		return fail(nil)
	}
	quote, e := localAcquisitionQuoteV1(pins, o.limits)
	if e != nil {
		return fail(e)
	}
	o.mu.Lock()
	if o.k == nil || o.uncertain || o.head != q.ExpectedHead {
		o.mu.Unlock()
		return fail(nil)
	}
	accounting := filepath.Clean(o.k.root.Path())
	if accounting == q.StorageRoot || strings.HasPrefix(q.StorageRoot, accounting+"/") || strings.HasPrefix(accounting, q.StorageRoot+"/") {
		o.mu.Unlock()
		return fail(nil)
	}
	h, e := o.k.reserve(ctx, o.job, q.ExpectedHead, q.Attempt, quote, q.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil {
		if errors.Is(e, errUncertainV2) {
			o.uncertain = true
		}
		o.mu.Unlock()
		return fail(e)
	}
	o.head = h.Identity
	o.mu.Unlock()
	s := &localAcquisitionStateV1{owner: o, ctx: ctx, deadline: q.Deadline, head: h.Identity, attempt: q.Attempt, dirname: "candidate-" + q.Attempt[7:], storage: q.StorageRoot, pins: pins, budget: newAcquisitionBudgetV1(pins)}
	abort := func(cause error) (PrivateLocalAcquisitionV1, error) {
		ce := s.cleanup()
		o.mu.Lock()
		defer o.mu.Unlock()
		if ce == nil && !s.transportUncertain && s.live() && o.head == s.head && !o.uncertain {
			h, oe := o.k.observe(ctx, o.job, s.head, s.attempt, "FAILED", time.Now().UnixNano(), o.limits)
			if oe == nil {
				o.head = h.Identity
			} else {
				o.uncertain = true
				ce = oe
			}
		} else {
			o.uncertain = true
		}
		return fail(errors.Join(cause, ce))
	}
	var total uint64
	for _, p := range pins {
		total += p.Bytes
	}
	if e = s.prepare(q.StorageRoot, total); e != nil {
		return abort(e)
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DisableCompression: true, MaxResponseHeaderBytes: 16384, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, DialContext: s.dialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for i := range pins {
		if e = s.download(client, i); e != nil {
			transport.CloseIdleConnections()
			je := s.stopConnections()
			s.transportUncertain = je != nil
			return abort(errors.Join(e, je))
		}
	}
	transport.CloseIdleConnections()
	if e = s.stopConnections(); e != nil {
		s.transportUncertain = true
		return abort(e)
	}
	if !s.live() {
		return abort(ErrPrivateLocalAcquisitionV1)
	}
	return PrivateLocalAcquisitionV1{s}, nil
}

// The fixed .tar.gz pin is inspected only; this is NOT native ABI admission.
// Policy bounds are conservative supported-layout limits, NOT discovered release
// sizes: <=16 flat regular USTAR members, <=64MiB/member, <=128MiB body total.
const acquisitionArchiveMaxV1 uint64 = 16 << 20
const acquisitionExpandedV1 uint64 = (128 << 20) + (32 << 10)
const acquisitionExpandedCallsV1 uint64 = 36864
const acquisitionArchiveReportV1 uint64 = 8192

type PrivateNativeArchiveInspectionV1 struct{ state *acquisitionInspectionStateV1 }
type acquisitionInspectionStateV1 struct {
	mu                      sync.Mutex
	acquisition             *localAcquisitionStateV1
	file                    *os.File
	started, closed, failed bool
	observer                acquisitionArchiveObserverV1
}

// Observation only; the sole implementation is in _test.go. No caller executor.
type acquisitionArchiveObserverV1 interface{ beforeArchiveRead() }
type PrivateNativeArchiveMemberV1 struct {
	Name  string
	Bytes uint64
}
type PrivateNativeArchiveReportV1 struct {
	ArchiveSHA256 string
	Members       []PrivateNativeArchiveMemberV1
	ExpandedBytes uint64
}

func (a PrivateLocalAcquisitionV1) BeginNativeArchiveInspectionV1(ctx context.Context) (PrivateNativeArchiveInspectionV1, error) {
	var zero PrivateNativeArchiveInspectionV1
	if a.state == nil {
		return zero, ErrPrivateLocalAcquisitionV1
	}
	s := a.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retired || s.inspectionIssued || ctx != s.ctx || !s.live() {
		return zero, ErrPrivateLocalAcquisitionV1
	}
	s.inspectionIssued = true
	s.inspectionFailed = true // refusal burns the fixed inspection, not a new grant
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return zero, ErrPrivateLocalAcquisitionV1
	}
	h, e := o.k.reopen(s.ctx, o.job, o.limits)
	if e != nil || h.Identity != s.head || h.Disposition != "PENDING" || h.Attempt != s.attempt {
		o.uncertain = true
		return zero, errors.Join(e, ErrPrivateLocalAcquisitionV1)
	}
	if e = s.validateNamespace(false); e != nil {
		return zero, e
	}
	if s.files[1] == nil {
		return zero, ErrPrivateLocalAcquisitionV1
	}
	st, e := statLocalNamespaceDirectoryV1(s.files[1], s.namespaceIO)
	if e != nil || !s.archiveIdentity(st) {
		return zero, errors.Join(e, ErrPrivateLocalAcquisitionV1)
	}
	if e = s.step(false); e != nil {
		return zero, e
	}
	if _, e = s.files[1].Seek(0, io.SeekStart); e != nil {
		return zero, e
	}
	s.activeUses++
	return PrivateNativeArchiveInspectionV1{&acquisitionInspectionStateV1{acquisition: s, file: s.files[1]}}, nil
}
func (s *localAcquisitionStateV1) archiveIdentity(st *syscall.Stat_t) bool {
	old := s.ids[1]
	return st != nil && st.Dev == old.Dev && st.Ino == old.Ino && st.Mode == old.Mode && st.Uid == old.Uid && st.Gid == old.Gid && st.Nlink == 1 && st.Size == int64(s.pins[1].Bytes) && st.Mode&syscall.S_IFMT == syscall.S_IFREG
}
func (l *acquisitionInspectionStateV1) live() bool {
	s := l.acquisition
	s.mu.Lock()
	defer s.mu.Unlock()
	if l.closed || s.retired || s.activeUses != 1 || !s.live() {
		return false
	}
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	return !s.owner.uncertain && s.owner.head == s.head
}

type acquisitionArchiveReaderV1 struct {
	digest           hash.Hash
	lease            *acquisitionInspectionStateV1
	r                io.Reader
	remaining, count uint64
	compressed       bool
}

func (r *acquisitionArchiveReaderV1) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.compressed && r.lease.observer != nil {
		r.lease.observer.beforeArchiveRead()
	}
	l := r.lease
	s := l.acquisition
	if !l.live() || r.remaining == 0 {
		return 0, ErrPrivateLocalAcquisitionV1
	}
	if r.compressed {
		if e := s.budget.take(&s.budget.archiveReads); e != nil {
			return 0, e
		}
		if e := s.validateNamespace(false); e != nil {
			return 0, e
		}
		st, e := statLocalNamespaceDirectoryV1(l.file, s.namespaceIO)
		if e != nil || !s.archiveIdentity(st) {
			return 0, errors.Join(e, ErrPrivateLocalAcquisitionV1)
		}
	} else {
		if e := s.budget.take(&s.budget.expandedReads); e != nil {
			return 0, e
		}
	}
	if uint64(len(p)) > r.remaining {
		p = p[:int(r.remaining)]
	}
	n, e := r.r.Read(p)
	if n < 0 || n > len(p) {
		return 0, ErrPrivateLocalAcquisitionV1
	}
	r.remaining -= uint64(n)
	r.count += uint64(n)
	if r.compressed && n > 0 {
		if he := s.budget.take(&s.budget.archiveHashes); he != nil {
			return 0, he
		}
		_, _ = r.digest.Write(p[:n])
	}
	// This is bounded stream consumption, not a refund: the WHOLE byte envelope
	// was charged in the original kernel reserve and is never returned.
	if n == 0 && e == nil {
		return 0, io.ErrNoProgress
	}
	return n, e
}
func acquisitionTarOctalV1(b []byte) (uint64, error) {
	text := strings.Trim(string(b), " \x00")
	if text == "" {
		return 0, nil
	}
	return strconv.ParseUint(text, 8, 64)
}
func acquisitionAllZeroV1(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
func (l PrivateNativeArchiveInspectionV1) Inspect(ctx context.Context) (report PrivateNativeArchiveReportV1, err error) {
	if l.state == nil {
		return report, ErrPrivateLocalAcquisitionV1
	}
	use := l.state
	use.mu.Lock()
	defer use.mu.Unlock()
	if use.closed || use.started {
		return report, ErrPrivateLocalAcquisitionV1
	}
	use.started = true
	use.failed = true
	s := use.acquisition
	if ctx != s.ctx || !use.live() {
		return report, ErrPrivateLocalAcquisitionV1
	}
	defer func() {
		if err == nil && !use.live() {
			err = ErrPrivateLocalAcquisitionV1
		}
		use.failed = err != nil
		if err != nil {
			report = PrivateNativeArchiveReportV1{}
		}
	}()
	compressed := &acquisitionArchiveReaderV1{digest: sha256.New(), lease: use, r: use.file, remaining: s.pins[1].Bytes + 1, compressed: true}
	buffered := bufio.NewReaderSize(compressed, 4096)
	gz, e := gzip.NewReader(buffered)
	if e != nil {
		return report, e
	}
	gz.Multistream(false)
	defer func() { err = errors.Join(err, gz.Close()) }()
	expanded := &acquisitionArchiveReaderV1{lease: use, r: gz, remaining: acquisitionExpandedV1}
	var header [512]byte
	var total uint64
	for {
		if _, e = io.ReadFull(expanded, header[:]); e != nil {
			return report, e
		}
		if acquisitionAllZeroV1(header[:]) {
			if _, e = io.ReadFull(expanded, header[:]); e != nil || !acquisitionAllZeroV1(header[:]) {
				return report, errors.Join(e, ErrPrivateLocalAcquisitionV1)
			}
			var extra [1]byte
			n, re := expanded.Read(extra[:])
			if n != 0 || re != io.EOF {
				return report, errors.Join(re, ErrPrivateLocalAcquisitionV1)
			}
			// A ByteReader prevents hidden gzip read-ahead from accepting another stream.
			if _, re = buffered.ReadByte(); re != io.EOF {
				return report, errors.Join(re, ErrPrivateLocalAcquisitionV1)
			}
			observed := fmt.Sprintf("sha256:%x", compressed.digest.Sum(nil))
			if observed != s.pins[1].SHA256 {
				return report, ErrPrivateLocalAcquisitionV1
			}
			report.ArchiveSHA256 = observed
			report.ExpandedBytes = expanded.count
			return report, nil
		}
		if len(report.Members) >= 16 || (header[156] != 0 && header[156] != '0') || !acquisitionAllZeroV1(header[157:257]) || !acquisitionAllZeroV1(header[345:500]) || (string(header[257:263]) != "ustar\x00" && string(header[257:263]) != "ustar ") {
			return report, ErrPrivateLocalAcquisitionV1
		}
		checksum, e := acquisitionTarOctalV1(header[148:156])
		if e != nil {
			return report, e
		}
		var sum uint64
		for i, b := range header {
			if i >= 148 && i < 156 {
				sum += 32
			} else {
				sum += uint64(b)
			}
		}
		if checksum != sum {
			return report, ErrPrivateLocalAcquisitionV1
		}
		name := strings.TrimRight(string(header[:100]), "\x00")
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\\") {
			return report, ErrPrivateLocalAcquisitionV1
		}
		for _, b := range []byte(name) {
			if b < 33 || b > 126 {
				return report, ErrPrivateLocalAcquisitionV1
			}
		}
		for _, m := range report.Members {
			if m.Name == name {
				return report, ErrPrivateLocalAcquisitionV1
			}
		}
		size, e := acquisitionTarOctalV1(header[124:136])
		if e != nil || size > 64<<20 || total > (128<<20)-size {
			return report, errors.Join(e, ErrPrivateLocalAcquisitionV1)
		}
		total += size
		report.Members = append(report.Members, PrivateNativeArchiveMemberV1{name, size})
		// Traverse actual member bytes/padding with fixed storage; no extraction,
		// arbitrary filesystem writes, target names or native loading is possible.
		left := size + (512-size%512)%512
		var block [32768]byte
		for left > 0 {
			want := left
			if want > uint64(len(block)) {
				want = uint64(len(block))
			}
			if _, e = io.ReadFull(expanded, block[:int(want)]); e != nil {
				return report, e
			}
			left -= want
		}
	}
}
func (l PrivateNativeArchiveInspectionV1) Close() error {
	if l.state == nil {
		return ErrPrivateLocalAcquisitionV1
	}
	use := l.state
	use.mu.Lock()
	defer use.mu.Unlock()
	if use.closed {
		return ErrPrivateLocalAcquisitionV1
	}
	use.closed = true
	s := use.acquisition
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeUses != 1 || s.retired {
		return ErrPrivateLocalAcquisitionV1
	}
	s.activeUses--
	s.inspectionFailed = use.failed || !use.started
	return nil // actual reader method joined under use.mu; FD remains acquisition-owned
}

// Paths exposes only the verified fixed filenames while original opaque custody
// is live. This is NOT a read grant: archive inspection uses only the prepaid
// opaque retained-FD lease; future managed/native reads need separate owned terms.
func (a PrivateLocalAcquisitionV1) Paths() ([2]string, error) {
	var paths [2]string
	if a.state == nil {
		return paths, ErrPrivateLocalAcquisitionV1
	}
	s := a.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retired || !s.live() {
		return paths, ErrPrivateLocalAcquisitionV1
	}
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	if s.owner.uncertain || s.owner.head != s.head {
		return paths, ErrPrivateLocalAcquisitionV1
	}
	for i, p := range s.pins {
		paths[i] = filepath.Join(s.storage, s.dirname, p.Name)
	}
	return paths, nil
}

// Retire uses only the original retained context and deadline, consumes copies
// before validation, removes only owned fixed names, and Observes only after all
// cleanup succeeds. A failed/expired cleanup keeps PENDING/debit truth and poisons
// this owner; no fresh context, retry, refund or automatic recovery is offered.
func (a PrivateLocalAcquisitionV1) Retire(ctx context.Context) error {
	if a.state == nil {
		return ErrPrivateLocalAcquisitionV1
	}
	s := a.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retired || s.activeUses != 0 {
		return ErrPrivateLocalAcquisitionV1
	}
	s.retired = true
	valid := ctx != nil && ctx == s.ctx && s.live()
	ce := s.cleanup()
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if !valid || ce != nil || !s.live() || o.uncertain || o.head != s.head {
		o.uncertain = true
		return errors.Join(ErrPrivateLocalAcquisitionV1, ce)
	}
	disposition := "OBSERVED"
	if s.inspectionIssued && s.inspectionFailed {
		disposition = "FAILED"
	}
	h, e := o.k.observe(s.ctx, o.job, s.head, s.attempt, disposition, time.Now().UnixNano(), o.limits)
	if e != nil || !s.live() {
		o.uncertain = true
		return errors.Join(ErrPrivateLocalAcquisitionV1, e)
	}
	o.head = h.Identity
	return nil
}
