//go:build darwin || linux

package privatedescribeprep

// Private local IPC only. The original caller keeps h and its root capabilities
// alive. This adapter neither executes host roles nor reconstructs authority.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
)

const AutomaticHandoffProtocolPrivateV2 = "PRIVATE_V2_AST_LOCAL_HANDOFF.v1"

type AutomaticHandoffConfigPrivateV2 struct {
	Enabled                                                           bool
	Directory, Invocation, InterpreterRole, ReviewerRole, ScopeSHA256 string
	TimeoutMS, OverallTimeoutMS, PollMS                               int
	Limits                                                            AutomaticLimitsPrivateV2
	ApprovalSHA256                                                    string
}

func AutomaticHandoffApprovalPrivateV2(c AutomaticHandoffConfigPrivateV2) string {
	c.ApprovalSHA256 = ""
	return automaticDigest(c)
}
func ValidateAutomaticHandoffPrivateV2(c AutomaticHandoffConfigPrivateV2) error {
	if !c.Enabled || !c.Limits.valid() || c.TimeoutMS < 1 || c.TimeoutMS > 240000 || c.OverallTimeoutMS < c.TimeoutMS || c.OverallTimeoutMS > 240000 || c.PollMS < 1 || c.PollMS > 1000 || c.ApprovalSHA256 != AutomaticHandoffApprovalPrivateV2(c) || len(c.ScopeSHA256) != 71 {
		return errors.New("private handoff exact configuration refused")
	}
	for _, s := range []string{c.Invocation, c.InterpreterRole, c.ReviewerRole} {
		if len(s) < 1 || len(s) > 256 || strings.ContainsAny(s, "\x00\r\n") {
			return errors.New("private handoff role/invocation refused")
		}
	}
	if c.InterpreterRole == c.ReviewerRole {
		return errors.New("private handoff requires distinct isolated role/session tags, not distinct models")
	}
	return nil
}
func handoffMetadata(i os.FileInfo, directory bool) bool {
	s, ok := i.Sys().(*syscall.Stat_t)
	return ok && s.Uid == uint32(os.Getuid()) && ((directory && i.IsDir() && i.Mode().Perm() == 0700) || (!directory && i.Mode().IsRegular() && i.Mode().Perm() == 0600 && s.Nlink == 1))
}

// Validate the empty mailbox without creating a claim. The caller additionally
// compares it against all source, producer, journal and output roots.
func CheckAutomaticHandoffDirectoryPrivateV2(path string) (err error) {
	r, _, err := openHandoff(path, true)
	if err == nil {
		err = r.Close()
	}
	return
}
func openHandoff(path string, empty bool) (*os.Root, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || path == "/tmp" || strings.HasPrefix(path, "/tmp/") || strings.HasPrefix(path, "/private/tmp/") {
		return nil, nil, errors.New("private handoff canonical outside-tmp directory required")
	}
	p := "/"
	for _, s := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		p = filepath.Join(p, s)
		i, e := os.Lstat(p)
		if e != nil || i.Mode()&os.ModeSymlink != 0 {
			return nil, nil, errors.New("private handoff path alias refused")
		}
	}
	i, e := os.Lstat(path)
	if e != nil || !handoffMetadata(i, true) {
		return nil, nil, errors.New("private handoff private current-owner directory required")
	}
	r, e := os.OpenRoot(path)
	if e != nil {
		return nil, nil, e
	}
	j, e := r.Stat(".")
	if e != nil || !os.SameFile(i, j) {
		r.Close()
		return nil, nil, errors.New("private handoff root changed")
	}
	if empty {
		f, e := r.Open(".")
		if e != nil {
			r.Close()
			return nil, nil, e
		}
		entries, e := f.ReadDir(1)
		ce := f.Close()
		if (e != nil && e != io.EOF) || ce != nil || len(entries) != 0 {
			r.Close()
			return nil, nil, errors.New("private handoff requires fresh empty mailbox")
		}
	}
	return r, i, nil
}

type AutomaticHandoffBindingPrivateV2 struct {
	Protocol, Invocation, OwnerNonce                                     string
	Sequence                                                             int
	Stage, RoleID, RequestSHA256, PreparationSHA256, InputSHA256, Target string
	Attempt                                                              int
	PredecessorDigest                                                    string
	ResponseAllowance                                                    int
	Deadline                                                             string
}
type AutomaticHandoffRequestPrivateV2 struct {
	Binding AutomaticHandoffBindingPrivateV2
	Request json.RawMessage
}
type AutomaticHandoffResponsePrivateV2 struct {
	Binding           AutomaticHandoffBindingPrivateV2
	PayloadSHA256     string
	PayloadByteLength int
	Payload           []byte // Base64 retains the exact model bytes, even malformed JSON.
}
type automaticHandoffPreparation struct {
	Protocol, Invocation, OwnerNonce, ScopeSHA256, PreparationSHA256 string
	PreparationByteLength                                            int
	Limits                                                           AutomaticLimitsPrivateV2
	Deadline                                                         string
	Preparation                                                      json.RawMessage
}
type automaticHandoff struct {
	cfg              AutomaticHandoffConfigPrivateV2
	root             *os.Root
	info             os.FileInfo
	owner            *postState
	nonce            string
	deadline         time.Time
	exchangeDeadline time.Time
	sequence         int
	failed, closed   bool
	consumed         map[string]os.FileInfo
}
type automaticHandoffBackend struct {
	shared       *automaticHandoff
	role         string
	lastResponse int
}

func (b *automaticHandoffBackend) Identity() string             { return b.role }
func (b *automaticHandoffBackend) automaticOwner() *postState   { return b.shared.owner }
func (b *automaticHandoffBackend) DeadlinePrivateV2() time.Time { return b.shared.deadline }
func (b *automaticHandoffBackend) wireResponseBytes() int       { return b.lastResponse }
func (b *automaticHandoffBackend) frame(r AutomaticRequestPrivateV2, allowance int) AutomaticHandoffRequestPrivateV2 {
	s := b.shared
	if s.exchangeDeadline.IsZero() {
		s.exchangeDeadline = minTime(s.deadline, time.Now().Add(time.Duration(s.cfg.TimeoutMS)*time.Millisecond))
	}
	raw, _ := json.Marshal(r)
	return AutomaticHandoffRequestPrivateV2{AutomaticHandoffBindingPrivateV2{AutomaticHandoffProtocolPrivateV2, s.cfg.Invocation, s.nonce, s.sequence + 1, r.Stage, b.role, pilot.Digest(raw), r.PreparationSHA256, r.InputSHA256, r.Target, r.Attempt, r.PredecessorDigest, allowance, s.exchangeDeadline.Format(time.RFC3339Nano)}, raw}
}

// Size is independent of allowance digits: conservatively reserve the largest
// allowance's encoded frame. This can charge a few reserved digit bytes more
// than the emitted frame when the aggregate allowance narrows; never less.
func (b *automaticHandoffBackend) wireSize(r AutomaticRequestPrivateV2, allowance int) (int, error) {
	raw, e := json.Marshal(b.frame(r, allowance))
	return len(raw), e
}
func (s *automaticHandoff) check() error {
	if s.closed || s.failed {
		return errors.New("handoff closed or failed")
	}
	i, e := os.Lstat(s.cfg.Directory)
	if e != nil || !os.SameFile(i, s.info) || !handoffMetadata(i, true) {
		return errors.New("handoff root substituted")
	}
	for n, before := range s.consumed {
		after, e := s.root.Lstat(n)
		if e != nil || !os.SameFile(before, after) || !handoffMetadata(after, false) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
			return errors.New("handoff consumed file substituted")
		}
	}
	return nil
}
func (s *automaticHandoff) publish(name string, raw []byte) (err error) {
	if filepath.Base(name) != name || len(raw) == 0 {
		return errors.New("handoff selector/empty file")
	}
	tmp := ".publish-" + name
	f, e := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer s.root.Remove(tmp)
	n, e := f.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	e = errors.Join(e, f.Sync(), f.Close())
	if e != nil {
		return e
	}
	// Link is create-only, never rename-overwrite; remove the temporary hard link
	// before exposing completion to the next local observation.
	if e = s.root.Link(tmp, name); e != nil {
		return e
	}
	if e = s.root.Remove(tmp); e != nil {
		return e
	}
	d, e := s.root.Open(".")
	if e != nil {
		return e
	}
	return errors.Join(d.Sync(), d.Close())
}
func (s *automaticHandoff) read(name string, limit int) (raw []byte, err error) {
	before, e := s.root.Lstat(name)
	if e != nil {
		return nil, e
	}
	if !handoffMetadata(before, false) {
		// A create-only link publication is not complete until its matching
		// temporary link is removed. Never read a two-link/partial file.
		if tmp, e := s.root.Lstat(".publish-" + name); e == nil && os.SameFile(before, tmp) {
			return nil, os.ErrNotExist
		}
		// The temporary unlink may have completed between the two stats.
		current, e := s.root.Lstat(name)
		if e != nil || !os.SameFile(before, current) || !handoffMetadata(current, false) {
			return nil, errors.New("handoff unsafe response file")
		}
		before = current
	}
	f, e := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	i, e := f.Stat()
	if e != nil || !os.SameFile(before, i) || !handoffMetadata(i, false) {
		return nil, errors.New("handoff file substituted")
	}
	raw, e = io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil {
		return raw, e
	}
	after, e := s.root.Lstat(name)
	if e != nil || !os.SameFile(i, after) || !handoffMetadata(after, false) || after.Size() != i.Size() || !after.ModTime().Equal(i.ModTime()) || len(raw) == 0 {
		return raw, errors.New("handoff partial/substituted response")
	}
	if len(raw) <= limit && int64(len(raw)) != i.Size() {
		return raw, errors.New("handoff response length changed")
	}
	s.consumed[name] = after
	return raw, nil
}
func NewAutomaticHandoffPrivateV2(c AutomaticHandoffConfigPrivateV2, scope string, h PostPreparationPrivateV2) (AutomaticBackendPrivateV2, AutomaticBackendPrivateV2, error) {
	if e := ValidateAutomaticHandoffPrivateV2(c); e != nil {
		return nil, nil, e
	}
	if c.ScopeSHA256 != scope || h.state == nil {
		return nil, nil, errors.New("handoff scope/owner mismatch")
	}
	r, i, e := openHandoff(c.Directory, true)
	if e != nil {
		return nil, nil, e
	}
	nonce := make([]byte, 32)
	if _, e = rand.Read(nonce); e != nil {
		r.Close()
		return nil, nil, e
	}
	s := &automaticHandoff{cfg: c, root: r, info: i, owner: h.state, nonce: hex.EncodeToString(nonce), deadline: time.Now().Add(time.Duration(c.OverallTimeoutMS) * time.Millisecond), consumed: map[string]os.FileInfo{}}
	raw, e := json.Marshal(automaticHandoffPreparation{AutomaticHandoffProtocolPrivateV2, c.Invocation, s.nonce, scope, h.state.sha, len(h.state.prepared), c.Limits, s.deadline.Format(time.RFC3339Nano), bytes.Clone(h.state.prepared)})
	if e == nil {
		e = s.publish("preparation.json", raw)
	}
	if e != nil {
		r.Close()
		return nil, nil, e
	}
	return &automaticHandoffBackend{shared: s, role: c.InterpreterRole}, &automaticHandoffBackend{shared: s, role: c.ReviewerRole}, nil
}
func (b *automaticHandoffBackend) Exchange(ctx context.Context, r AutomaticRequestPrivateV2, cap int) (payload []byte, err error) {
	s := b.shared
	b.lastResponse = 0
	defer func() {
		s.exchangeDeadline = time.Time{}
		if err != nil {
			s.failed = true
			err = automaticHandoffFailure{automaticClass(err)}
		}
	}()
	if ctx == nil || cap < 1 || cap > s.cfg.Limits.MaxResponseBytes || s.sequence >= s.cfg.Limits.MaxCalls || s.owner == nil || r.PreparationSHA256 != s.owner.sha {
		return nil, AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}
	}
	if e := s.check(); e != nil {
		return nil, e
	}
	if (r.Stage == "INTERPRET" || r.Stage == "NAVIGATE") && b.role != s.cfg.InterpreterRole || (r.Stage == "REVIEW_TECHNICAL" || r.Stage == "REVIEW_NAVIGATION") && b.role != s.cfg.ReviewerRole {
		return nil, errors.New("handoff role mismatch")
	}
	switch r.Stage {
	case "INTERPRET", "NAVIGATE", "REVIEW_TECHNICAL", "REVIEW_NAVIGATION":
	default:
		return nil, errors.New("handoff stage refused")
	}
	frame := b.frame(r, cap)
	ctx, cancel := context.WithDeadline(ctx, s.exchangeDeadline)
	defer cancel()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(frame)
	if len(raw) > s.cfg.Limits.MaxRequestBytes {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	name := fmt.Sprintf("request-%06d.json", frame.Binding.Sequence)
	responseName := fmt.Sprintf("response-%06d.json", frame.Binding.Sequence)
	// No pre-existing foreign/early response can be consumed.
	if _, e := s.root.Lstat(responseName); !errors.Is(e, os.ErrNotExist) {
		return nil, errors.New("handoff early response")
	}
	if e := s.publish(name, raw); e != nil {
		return nil, e
	}
	s.sequence++
	ticker := time.NewTicker(time.Duration(s.cfg.PollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if e := s.check(); e != nil {
			return nil, e
		}
		raw, e := s.read(responseName, cap)
		b.lastResponse = len(raw)
		if errors.Is(e, os.ErrNotExist) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
				continue
			}
		}
		if len(raw) > cap {
			return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
		}
		if e != nil {
			return nil, e
		}
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		var response AutomaticHandoffResponsePrivateV2
		if strictAutomatic(raw, cap, &response) != nil || response.Binding != frame.Binding || response.PayloadByteLength != len(response.Payload) || response.PayloadSHA256 != pilot.Digest(response.Payload) {
			return nil, errors.New("handoff frame binding/digest refused")
		}
		return bytes.Clone(response.Payload), nil
	}
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

type automaticHandoffFailure struct{ class string }

func (e automaticHandoffFailure) Error() string           { return "PRIVATE_HANDOFF_TERMINAL" }
func (e automaticHandoffFailure) TerminalHandoffFailure() {}
func (e automaticHandoffFailure) Unwrap() error           { return AutomaticBackendErrorPrivateV2{e.class} }

// Terminal is called only by the original caller after exact paired readback (or
// accountable failure). It closes the mailbox even if receipt publication fails.
func FinishAutomaticHandoffPrivateV2(backend AutomaticBackendPrivateV2, outcome *PostPublicationOutcomePrivateV2, runErr error) error {
	b, ok := backend.(*automaticHandoffBackend)
	if !ok {
		return nil
	}
	s := b.shared
	if s.closed {
		return errors.New("handoff already closed")
	}
	checkErr := s.check()
	if !time.Now().Before(s.deadline) {
		checkErr = errors.Join(checkErr, context.DeadlineExceeded)
	}
	if runErr == nil && checkErr != nil {
		runErr = checkErr
	}
	status := "FAILED_CLOSED"
	failure := automaticClass(runErr)
	sha := ""
	length := 0
	if runErr == nil && !s.failed && outcome != nil && outcome.Status == "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" {
		var envelope postPublicationEnvelopePrivateV2
		content := outcome.EnvelopeBytes()
		if len(content) == 0 || int64(len(content)) != int64(outcome.ByteLength) || pilot.Digest(content) != outcome.EnvelopeSHA256 || json.Unmarshal(content, &envelope) != nil || envelope.PreparationSHA256 != s.owner.sha || envelope.Inventory == nil {
			checkErr = errors.Join(checkErr, errors.New("handoff terminal outcome binding refused"))
		} else {
			status = "VERIFIED_PAIR_PRIVATE_V2_AST"
			failure = ""
			sha = outcome.EnvelopeSHA256
			length = int(outcome.ByteLength)
		}
	}
	raw, _ := json.Marshal(struct {
		Protocol, Invocation, OwnerNonce, PreparationSHA256, Status, Failure, EnvelopeSHA256 string
		ByteLength, Exchanges                                                                int
	}{AutomaticHandoffProtocolPrivateV2, s.cfg.Invocation, s.nonce, s.owner.sha, status, failure, sha, length, s.sequence})
	e := s.publish("terminal.json", raw)
	s.closed = true
	s.owner = nil
	return errors.Join(checkErr, e, s.root.Close())
}
