//go:build darwin || linux

package privatedescribeprep

// Source Describe is a separate, one-shot byte transport, NOT a legacy backend
// or producer admission. The upper source owner must establish eligibility.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	sourceclock "lsp-trace/internal/privatesourceclock"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/describerequest"
)

const SourceHostProtocol = "PRIVATE_SOURCE_DESCRIBE_LOCAL_HANDOFF.v1"
const SourceHostFamily = "SOURCE_DESCRIBE"

type SourceHostConfig struct {
	Protocol string
	Mailbox  AutomaticHandoffConfigPrivateV2
}

// Labels are caller attribution, never authenticated role independence.
type SourceHostContext struct {
	HandoffID, CheckpointID, SourceUnitID, SourceDigest, RequestRecordID, AttemptID, Workspace string
	Deadline                                                                                   time.Time
}

type SourceHostBinding struct {
	Protocol, Invocation, OwnerNonce                string
	Sequence                                        int
	Stage, RoleID, RequestSHA256, PreparationSHA256 string
	Source                                          SourceHostContext
	ResponseAllowance                               int
	Deadline                                        string
}
type SourceHostRequest struct {
	Binding SourceHostBinding
	Request describerequest.Record
}
type SourceHostResponse struct {
	Binding           SourceHostBinding
	PayloadSHA256     string
	PayloadByteLength int
	Payload           []byte
}
type sourceHostPreparation struct {
	Protocol, Invocation, OwnerNonce, ScopeSHA256, PreparationSHA256 string
	PreparationByteLength                                            int
	Limits                                                           AutomaticLimitsPrivateV2
	Deadline                                                         string
	SourceConfiguration                                              SourceHostConfig
	Preparation                                                      SourceHostContext
}

type SourceHostMailbox struct {
	mu           sync.Mutex
	io           *automaticHandoff // only unchanged check/publish/read; owner stays nil
	source       SourceHostContext
	record       describerequest.Record
	preparation  string
	used         bool
	closeErr     error // first close result; shared by all pointer/upper handle copies
	roots        sourceHostRoots
	localPayload []byte // non-admitting one-shot payload; exact binding stays mailbox-owned
	localArmed   bool
}

// SourceHostDelivery is minted only after a successful actual file Exchange.
// A public frame, configuration pin or hash cannot reconstruct this receipt.
type SourceHostDelivery struct{ state *sourceHostDelivery }
type sourceHostDelivery struct {
	Binding                         SourceHostBinding
	FrameSHA256, PayloadSHA256      string
	PayloadByteLength               int
	Observation, Authentication     string
	Authority                       int
	Accepted                        bool
	Identity, Completeness, Absence string
}

// SourceHostDeliveryUpper is the same closed scalar formula used on the actual
// delivered receipt. Fixed 4096 covers all keys, four SHA256/nonce strings,
// scalar numbers, time strings and fixed provenance labels. No payload bytes
// occur in a receipt; response frame base64 is charged by SourceHostRetentionUpper.
func SourceHostDeliveryUpper(c SourceHostConfig, s SourceHostContext) (uint64, error) {
	return sourceHostDeliveryUpper(SourceHostBinding{Protocol: c.Protocol, Invocation: c.Mailbox.Invocation, RoleID: c.Mailbox.InterpreterRole, Source: s})
}
func sourceHostDeliveryUpper(b SourceHostBinding) (uint64, error) {
	n := uint64(4096)
	for _, v := range []string{b.Protocol, b.Invocation, b.RoleID, b.Source.HandoffID, b.Source.CheckpointID, b.Source.SourceUnitID, b.Source.SourceDigest, b.Source.RequestRecordID, b.Source.AttemptID, b.Source.Workspace} {
		if uint64(len(v)) > (^uint64(0)-n)/6 {
			return 0, errors.New("source host delivery size overflow")
		}
		n += 6 * uint64(len(v))
	}
	return n, nil
}
func (d SourceHostDelivery) SerializedUpper() (uint64, error) {
	if d.state == nil {
		return 0, errors.New("source host delivery absent")
	}
	return sourceHostDeliveryUpper(d.state.Binding)
}

func (d SourceHostDelivery) Delivered() bool { return d.state != nil }
func (d SourceHostDelivery) Bytes() ([]byte, error) {
	if d.state == nil {
		return nil, errors.New("source host delivery absent")
	}
	return json.Marshal(d.state)
}

func ValidateSourceHostConfig(c SourceHostConfig) error {
	if c.Protocol != SourceHostProtocol {
		return errors.New("source host protocol refused")
	}
	return ValidateAutomaticHandoffPrivateV2(c.Mailbox)
}

// Scalar, allocation-free upper, BEFORE Record validation, hashes or mailbox IO.
// Fixed 32768 covers two fixed descriptors/bindings, nonce, receipt and scratch;
// JSON escaping is at most 6 bytes per variable input byte. Frame/parse/hash,
// response payload, retained request and accessor copies are reserved, including
// the complete configured whole-frame allowances. This is logical payload
// accounting, not a physical heap/FIT or unlimited accessor-copy claim.
func SourceHostRetentionUpper(c SourceHostConfig, r describerequest.Record, s SourceHostContext) (uint64, error) {
	if c.Protocol != SourceHostProtocol || !c.Mailbox.Enabled || !c.Mailbox.Limits.valid() || c.Mailbox.TimeoutMS < 1 || c.Mailbox.TimeoutMS > 240000 || c.Mailbox.OverallTimeoutMS < c.Mailbox.TimeoutMS || c.Mailbox.OverallTimeoutMS > 240000 || c.Mailbox.PollMS < 1 || c.Mailbox.PollMS > 1000 {
		return 0, errors.New("source host scalar configuration")
	}
	l := c.Mailbox.Limits
	// Scalar frame uppers precede all IO. Bound only actual variable strings;
	// whole configured frame capacities are separately charged in retention.
	meta := uint64(0)
	for _, v := range []string{c.Mailbox.Directory, c.Mailbox.Invocation, c.Mailbox.InterpreterRole, c.Mailbox.ReviewerRole, c.Mailbox.ScopeSHA256, c.Mailbox.ApprovalSHA256, s.HandoffID, s.CheckpointID, s.SourceUnitID, s.SourceDigest, s.RequestRecordID, s.AttemptID, s.Workspace} {
		if uint64(len(v)) > (^uint64(0)-meta)/6 {
			return 0, errors.New("source host frame overflow")
		}
		meta += 6 * uint64(len(v))
	}
	record := uint64(2048)
	for _, v := range []string{r.RecordID, r.Envelope.Protocol, r.Envelope.MessageType, r.Envelope.MessageID, r.Envelope.CorrelationID, r.Envelope.AdmissionID, r.Envelope.Corpus, r.Envelope.ItemType, r.Envelope.AcquisitionMode, r.Envelope.InputSHA256, r.Envelope.SourceRevision, r.Envelope.TargetMessageID, r.Envelope.Prompt, r.Lineage.PacketID, r.Lineage.CensusID, r.Lineage.LineageIdentity, r.Lineage.ConsumerResolution, r.Lineage.AlternativeID, r.Lineage.Completeness, r.Lineage.SourceGraphComplete} {
		if uint64(len(v)) > (^uint64(0)-record)/6 {
			return 0, errors.New("source host record overflow")
		}
		record += 6 * uint64(len(v))
	}
	if meta > 1<<24 || record > 1<<24 {
		return 0, errors.New("source host frame scalar ceiling")
	}
	prep := uint64(8192) + meta
	request := uint64(4096) + meta + record
	if prep > uint64(l.MaxRequestBytes) || prep > uint64(l.MaxOutputBytes) || request > uint64(l.MaxRequestBytes) || prep+request+uint64(l.MaxResponseBytes) > uint64(l.MaxTotalBytes) {
		return 0, errors.New("source host early frame/output/aggregate ceiling")
	}
	stringsToCount := []string{c.Protocol, c.Mailbox.Directory, c.Mailbox.Invocation, c.Mailbox.InterpreterRole, c.Mailbox.ReviewerRole, c.Mailbox.ScopeSHA256, c.Mailbox.ApprovalSHA256, s.HandoffID, s.CheckpointID, s.SourceUnitID, s.SourceDigest, s.RequestRecordID, s.AttemptID, s.Workspace,
		r.RecordID, r.Envelope.Protocol, r.Envelope.MessageType, r.Envelope.MessageID, r.Envelope.CorrelationID, r.Envelope.AdmissionID, r.Envelope.Corpus, r.Envelope.ItemType, r.Envelope.AcquisitionMode, r.Envelope.InputSHA256, r.Envelope.SourceRevision, r.Envelope.TargetMessageID, r.Envelope.Prompt, r.Lineage.PacketID, r.Lineage.CensusID, r.Lineage.LineageIdentity, r.Lineage.ConsumerResolution, r.Lineage.AlternativeID, r.Lineage.Completeness, r.Lineage.SourceGraphComplete}
	if len(c.Mailbox.Directory) > sourceHostPathMax || len(s.Workspace) > sourceHostPathMax {
		return 0, errors.New("source host root path ceiling")
	}
	// Charge retained and transient bounded metadata snapshots, not file contents.
	n := uint64(32768+sourceHostRootMetadata) + uint64(l.MaxRequestBytes)*8 + uint64(l.MaxResponseBytes)*10
	for _, v := range stringsToCount {
		if uint64(len(v)) > (^uint64(0)-n)/24 {
			return 0, errors.New("source host retention overflow")
		}
		n += uint64(len(v)) * 24
	}
	return n, nil
}

func sourceHostLive(ctx context.Context, deadline time.Time) bool {
	return ctx != nil && ctx.Err() == nil && !deadline.IsZero() && sourceclock.Now(ctx, "host-acceptance").Before(deadline)
}
func sourceHostSHA(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	for _, c := range s[7:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
func sourceHostIdentity(s SourceHostContext, r describerequest.Record) bool {
	for _, v := range []string{s.HandoffID, s.CheckpointID, s.SourceUnitID, s.SourceDigest, s.RequestRecordID, s.AttemptID, s.Workspace} {
		if len(v) == 0 || len(v) > 4096 || strings.TrimSpace(v) != v || strings.ContainsAny(v, "\x00\r\n") {
			return false
		}
	}
	return sourceHostSHA(s.HandoffID) && sourceHostSHA(s.CheckpointID) && sourceHostSHA(s.SourceDigest) && s.RequestRecordID == r.RecordID && s.SourceUnitID == r.Envelope.AdmissionID && s.SourceUnitID == r.Lineage.PacketID && s.SourceUnitID == r.Lineage.CensusID && s.SourceDigest == "sha256:"+r.Envelope.SourceRevision && r.Lineage.ConsumerResolution == describerequest.PrivateSourceOnly && r.Lineage.AlternativeID == "SOURCE_ONLY" && r.Lineage.AlternativeOrdinal == 0
}

// NewSourceHostMailbox is NON-ADMITTING: public data can construct only a byte
// mailbox. Upper opaque source ownership, budgets and exact regeneration must
// precede this call. It does not acquire sources, invoke a host or execute roles.
func NewSourceHostMailbox(ctx context.Context, c SourceHostConfig, r describerequest.Record, source SourceHostContext, boundedLocal ...bool) (*SourceHostMailbox, error) {
	// B explicitly selects stricter read mechanics; this is not admission or
	// credit. Legacy callers retain the exact original default mechanics.
	if len(boundedLocal) > 1 {
		return nil, errors.New("source host read selector refused")
	}
	if _, err := SourceHostRetentionUpper(c, r, source); err != nil {
		return nil, err
	}
	if err := ValidateSourceHostConfig(c); err != nil {
		return nil, err
	}
	if source.HandoffID != c.Mailbox.ScopeSHA256 || !sourceHostIdentity(source, r) || !sourceHostLive(ctx, source.Deadline) {
		return nil, errors.New("source host source/context refused")
	}
	if err := describerequest.ValidateRecord(r); err != nil {
		return nil, err
	}
	// Wire equality must not retain process-local monotonic/location state.
	// UTC preserves the original instant; it never renews the source window.
	source.Deadline = source.Deadline.UTC()
	deadline := source.Deadline
	if d, ok := ctx.Deadline(); ok {
		deadline = minTime(deadline, d)
	}
	// Never grant a new overall window: only shorten the original source window.
	deadline = minTime(deadline, time.Now().Add(time.Duration(c.Mailbox.OverallTimeoutMS)*time.Millisecond))
	raw, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	prepSHA := pilot.Digest(raw)
	root, info, err := openHandoff(c.Mailbox.Directory, true)
	if err != nil {
		return nil, err
	}
	io := &automaticHandoff{cfg: c.Mailbox, root: root, info: info, deadline: deadline, consumed: map[string]os.FileInfo{}}
	if len(boundedLocal) == 1 && boundedLocal[0] {
		io.sourceReadContext = ctx
	}
	fail := func(e error) (*SourceHostMailbox, error) { root.Close(); return nil, e }
	roots, err := observeSourceHostRoots(source.Workspace, c.Mailbox.Directory)
	if err != nil {
		return fail(err)
	}
	m := &SourceHostMailbox{io: io, source: source, record: r, preparation: prepSHA, roots: roots}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return fail(err)
	}
	io.nonce = hex.EncodeToString(nonce)
	prep := sourceHostPreparation{SourceHostProtocol, c.Mailbox.Invocation, io.nonce, c.Mailbox.ScopeSHA256, prepSHA, len(raw), c.Mailbox.Limits, deadline.Format(time.RFC3339Nano), c, source}
	encoded, err := json.Marshal(prep)
	if err != nil {
		return fail(err)
	}
	if len(encoded) > c.Mailbox.Limits.MaxRequestBytes || len(encoded) > c.Mailbox.Limits.MaxOutputBytes || len(encoded) > c.Mailbox.Limits.MaxTotalBytes {
		return fail(errors.New("source host preparation ceiling"))
	}
	if err = m.check(); err != nil {
		return fail(err)
	}
	if !sourceHostLive(ctx, deadline) {
		return fail(context.DeadlineExceeded)
	}
	if err = io.publish("preparation.json", encoded); err != nil {
		return fail(err)
	}
	// Pin our own publication using the exact existing inode/metadata read path.
	if err = m.check(); err != nil {
		return fail(err)
	}
	got, err := io.read("preparation.json", len(encoded))
	if err != nil || !bytes.Equal(got, encoded) {
		return fail(errors.New("source host preparation changed"))
	}
	if !sourceHostPreparedLive(ctx, m.io.deadline) {
		return fail(context.DeadlineExceeded)
	}
	return m, nil
}

// Exchange is sticky one-shot on every success or failure, including framing and
// cancellation. The cap applies to the WHOLE response frame, not just payload.
func (m *SourceHostMailbox) Exchange(ctx context.Context) (payload []byte, receipt SourceHostDelivery, err error) {
	if m == nil {
		return nil, receipt, errors.New("source host mailbox absent")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.used || m.io == nil {
		return nil, receipt, errors.New("source host already consumed")
	}
	m.used = true
	io := m.io
	originalCtx := ctx
	defer func() {
		if err != nil {
			io.failed = true
		}
	}()
	if !sourceHostLive(ctx, io.deadline) {
		return nil, receipt, context.DeadlineExceeded
	}
	if err = m.check(); err != nil {
		return
	}
	deadline := minTime(io.deadline, time.Now().Add(time.Duration(io.cfg.TimeoutMS)*time.Millisecond))
	if d, ok := ctx.Deadline(); ok {
		deadline = minTime(deadline, d)
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	requestRaw, err := json.Marshal(m.record)
	if err != nil {
		return
	}
	binding := SourceHostBinding{SourceHostProtocol, io.cfg.Invocation, io.nonce, 1, SourceHostFamily, io.cfg.InterpreterRole, pilot.Digest(requestRaw), m.preparation, m.source, io.cfg.Limits.MaxResponseBytes, deadline.Format(time.RFC3339Nano)}
	raw, err := json.Marshal(SourceHostRequest{binding, m.record})
	if err != nil {
		return
	}
	prepInfo := io.consumed["preparation.json"]
	if len(raw) > io.cfg.Limits.MaxRequestBytes || prepInfo == nil || int64(len(raw))+prepInfo.Size()+int64(binding.ResponseAllowance) > int64(io.cfg.Limits.MaxTotalBytes) {
		return nil, receipt, errors.New("source host whole frame/aggregate ceiling")
	}
	if _, e := io.root.Lstat("response-000001.json"); !errors.Is(e, os.ErrNotExist) {
		return nil, receipt, errors.New("source host early response")
	}
	if err = m.check(); err != nil {
		return
	}
	if !sourceHostRequestLive(originalCtx, deadline) {
		return nil, receipt, context.DeadlineExceeded
	}
	if err = io.publish("request-000001.json", raw); err != nil {
		return
	}
	if err = m.check(); err != nil {
		return
	}
	got, err := io.read("request-000001.json", len(raw))
	if err != nil || !bytes.Equal(got, raw) {
		return nil, receipt, errors.New("source host request changed")
	}
	if m.localArmed {
		// Same public frame and same actual file transport as an external role.
		// No receipt is minted here; the unchanged read/binding checks below do it.
		// The existing scalar delivery upper covers binding keys, fixed hashes,
		// times and 6x every variable string. Add base64 BEFORE hashing/encoding
		// the response so even a rejected oversized frame is already bounded.
		frameUpper, upperErr := sourceHostDeliveryUpper(binding)
		base64Upper := uint64((len(m.localPayload)+2)/3) * 4
		if upperErr != nil || base64Upper > uint64(binding.ResponseAllowance) || frameUpper > uint64(binding.ResponseAllowance)-base64Upper {
			return nil, receipt, errors.New("source host local whole-frame upper refused")
		}
		response := SourceHostResponse{Binding: binding, PayloadSHA256: pilot.Digest(m.localPayload), PayloadByteLength: len(m.localPayload), Payload: m.localPayload}
		encoded, e := json.Marshal(response)
		if e != nil || len(encoded) > binding.ResponseAllowance || !sourceHostLive(ctx, deadline) {
			return nil, receipt, errors.New("source host local response ceiling/deadline")
		}
		if e = io.publish("response-000001.json", encoded); e != nil {
			return nil, receipt, e
		}
		m.localPayload = nil
	}
	ticker := time.NewTicker(time.Duration(io.cfg.PollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		if !sourceHostLive(ctx, deadline) {
			return nil, receipt, context.DeadlineExceeded
		}
		if err = m.check(); err != nil {
			return
		}
		raw, err = io.read("response-000001.json", binding.ResponseAllowance)
		if errors.Is(err, os.ErrNotExist) {
			if m.localArmed {
				return nil, receipt, errors.New("source host local response missing")
			}
			select {
			case <-ctx.Done():
				return nil, receipt, ctx.Err()
			case <-ticker.C:
				continue
			}
		}
		if err != nil {
			return
		}
		if len(raw) > binding.ResponseAllowance {
			return nil, receipt, errors.New("source host response frame ceiling")
		}
		if !sourceHostResponseLive(originalCtx, deadline) {
			return nil, receipt, context.DeadlineExceeded
		}
		var response SourceHostResponse
		if strictAutomatic(raw, binding.ResponseAllowance, &response) != nil || response.Binding != binding || response.PayloadByteLength != len(response.Payload) || response.PayloadSHA256 != pilot.Digest(response.Payload) {
			return nil, receipt, errors.New("source host exact response binding refused")
		}
		if err = m.check(); err != nil {
			return
		}
		payload = bytes.Clone(response.Payload)
		receiptRaw := &sourceHostDelivery{binding, pilot.Digest(raw), response.PayloadSHA256, len(payload), "LOCAL_HOST_ROLE_FILE_TRANSPORT_DELIVERED", "HOST_OBSERVED_NOT_AUTHENTICATED", 0, false, "UNRESOLVED", "UNKNOWN", "PROHIBITED"}
		if err = m.check(); err != nil {
			return nil, SourceHostDelivery{}, err
		}
		if !sourceHostDeliveryLive(originalCtx, deadline) {
			return nil, SourceHostDelivery{}, context.DeadlineExceeded
		}
		return payload, SourceHostDelivery{receiptRaw}, nil
	}
}

// Metadata-only, bounded logical observations; not an atomic global OS proof.
// Fixed arrays bound allocation before traversal. At most 128 ancestors per root,
// 4096 path bytes, and 1024 logical metadata bytes per entry are charged twice
// (retained and recheck snapshots) before any mailbox buffer IO.
const sourceHostPathMax = 4096
const sourceHostAncestorMax = 128
const sourceHostRootMetadata = 4*sourceHostAncestorMax*1024 + 8*sourceHostPathMax

type sourceHostChain struct {
	n    int
	info [sourceHostAncestorMax]os.FileInfo
}
type sourceHostRoots struct{ workspace, mailbox sourceHostChain }

func sourceHostAncestors(path string) (sourceHostChain, error) {
	var chain sourceHostChain
	if len(path) == 0 || len(path) > sourceHostPathMax || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return chain, errors.New("source host root path refused")
	}
	// Check path depth before walking or allocating metadata.
	if strings.Count(path, string(os.PathSeparator))+1 > sourceHostAncestorMax {
		return chain, errors.New("source host ancestor ceiling")
	}
	for {
		if chain.n == len(chain.info) {
			return sourceHostChain{}, errors.New("source host ancestor ceiling")
		}
		info, err := os.Stat(path)
		if err != nil {
			return sourceHostChain{}, err
		}
		if !info.IsDir() {
			return sourceHostChain{}, errors.New("source host ancestor not directory")
		}
		chain.info[chain.n] = info
		chain.n++
		parent := filepath.Dir(path)
		if parent == path {
			return chain, nil
		}
		path = parent
	}
}

func observeSourceHostRoots(workspace, mailbox string) (sourceHostRoots, error) {
	var roots sourceHostRoots
	if len(workspace) > sourceHostPathMax || len(mailbox) > sourceHostPathMax {
		return roots, errors.New("source host root path ceiling")
	}
	path := workspace
	if strings.HasPrefix(workspace, "file:") {
		u, err := url.Parse(workspace)
		if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
			return roots, errors.New("source host workspace URI")
		}
		path = u.Path
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return roots, errors.New("source host workspace path")
	}
	if strings.Count(path, string(os.PathSeparator))+1 > sourceHostAncestorMax {
		return roots, errors.New("source host ancestor ceiling")
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return roots, err
	}
	roots.workspace, err = sourceHostAncestors(path)
	if err != nil {
		return sourceHostRoots{}, err
	}
	roots.mailbox, err = sourceHostAncestors(mailbox)
	if err != nil {
		return sourceHostRoots{}, err
	}
	a, ok := roots.workspace.info[0].Sys().(*syscall.Stat_t)
	b, ok2 := roots.mailbox.info[0].Sys().(*syscall.Stat_t)
	if !ok || !ok2 || a.Dev != b.Dev {
		return sourceHostRoots{}, errors.New("source host workspace/mailbox filesystem mismatch")
	}
	for _, pair := range [][2]sourceHostChain{{roots.workspace, roots.mailbox}, {roots.mailbox, roots.workspace}} {
		for i := 0; i < pair[1].n; i++ {
			if os.SameFile(pair[0].info[0], pair[1].info[i]) {
				return sourceHostRoots{}, errors.New("source host mailbox overlaps workspace identity")
			}
		}
	}
	return roots, nil
}

func (m *SourceHostMailbox) check() error {
	if err := m.io.check(); err != nil {
		return err
	}
	current, err := observeSourceHostRoots(m.source.Workspace, m.io.cfg.Directory)
	if err != nil {
		return err
	}
	for _, pair := range [][2]sourceHostChain{{m.roots.workspace, current.workspace}, {m.roots.mailbox, current.mailbox}} {
		if pair[0].n != pair[1].n {
			return errors.New("source host ancestor chain changed")
		}
		for i := 0; i < pair[0].n; i++ {
			if !os.SameFile(pair[0].info[i], pair[1].info[i]) {
				return errors.New("source host ancestor identity changed")
			}
		}
	}
	return nil
}

// Named logical liveness boundaries. Production clocks are non-configurable;
// deterministic clock advancement exists only in explicitly tagged test builds.
func sourceHostRequestLive(ctx context.Context, deadline time.Time) bool {
	return sourceHostLive(ctx, deadline)
}
func sourceHostResponseLive(ctx context.Context, deadline time.Time) bool {
	return sourceHostLive(ctx, deadline)
}
func sourceHostDeliveryLive(ctx context.Context, deadline time.Time) bool {
	return sourceHostLive(ctx, deadline)
}

// The constructor's completed readback must still fit its already owned window.
func sourceHostPreparedLive(ctx context.Context, deadline time.Time) bool {
	return sourceHostLive(ctx, deadline)
}

// Deadline is the actual narrowed original/overall transport deadline, not a
// renewable window or an eligibility credential.
func (m *SourceHostMailbox) Deadline() time.Time {
	if m == nil {
		return time.Time{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.io == nil {
		return time.Time{}
	}
	return m.io.deadline
}

// ArmLocalResponse retains only already bounded adapter bytes. This grants no
// source eligibility or delivery; Exchange still publishes, reads, and validates
// its exact nonce/role/request/preparation/source/attempt/deadline file binding.
func (m *SourceHostMailbox) ArmLocalResponse(ctx context.Context, payload []byte) error {
	if m == nil {
		return errors.New("source host mailbox absent")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.io == nil || m.io.sourceReadContext == nil || m.used || m.localArmed || !sourceHostLive(ctx, m.io.deadline) || len(payload) == 0 || len(payload) > 8192 || len(payload) > m.io.cfg.Limits.MaxResponseBytes {
		return errors.New("source host local arm refused")
	}
	m.localArmed = true
	m.localPayload = bytes.Clone(payload)
	return nil
}

func (m *SourceHostMailbox) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.used = true
	if m.io == nil || m.io.closed {
		return m.closeErr
	}
	m.localPayload = nil
	m.io.closed = true // close once, including on failure; never retry the descriptor
	m.closeErr = m.io.root.Close()
	return m.closeErr
}
