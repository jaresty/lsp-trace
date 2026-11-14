package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"sync"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

var ErrPrivateDiscoveryOperationV2 = errors.New("private discovery operation refused")

// PrivateDiscoveryOwnerV2 can reopen only an existing canonical, authorized READY
// chain. It has no initialization, renewal, refund, grant or native-target API.
type PrivateDiscoveryOwnerV2 struct {
	mu        sync.Mutex
	k         *kernelV2
	job       jobV2
	head      string
	limits    verificationLimitsV2
	uncertain bool
}
type PrivateDiscoveryBindingV2 struct {
	Revision, PlanIdentity, DenominatorDigest, MemberDigest, Workspace, RuntimeFingerprint, PolicyDigest string
	Ordinal                                                                                              int
	Deadline                                                                                             time.Time
	Limits                                                                                               sessionruntime.PrivateDiscoveryLimits
}
type PrivateDiscoveryOperationV2 struct{ state *privateDiscoveryOperationV2 }
type privateDiscoveryOperationV2 struct {
	mu                       sync.Mutex
	owner                    *PrivateDiscoveryOwnerV2
	binding                  PrivateDiscoveryBindingV2
	attempt, head            string
	used, finished, executed bool
	observation              sessionruntime.PrivateDiscoveryObservation
	executionErr             error
}

func OpenPrivateDiscoveryOwnerV2(ctx context.Context, root *publication.Root, canonicalJob []byte, expected string, l PrivateManagedInputVerificationV2) (*PrivateDiscoveryOwnerV2, error) {
	var j jobV2
	if ctx == nil || root == nil || !hashV2(expected) || parseV2(canonicalJob, &j) != nil || j.Scope != PrivateDiscoveryScopeDigestV2() {
		return nil, ErrPrivateDiscoveryOperationV2
	}
	limits := verificationLimitsV2{l.Records, l.Bytes, l.Work}
	k := &kernelV2{root: root}
	h, e := k.reopen(ctx, j, limits)
	if e != nil || h.Identity != expected || !readyV2(h) || h.Windows != 0 || h.Charged != (countersV2{}) {
		return nil, ErrPrivateDiscoveryOperationV2
	}
	return &PrivateDiscoveryOwnerV2{k: k, job: j, head: expected, limits: limits}, nil
}

// PrivateDiscoveryResourceRulesV2 identifies this target-free PARTITION policy.
// An externally authorized job must already name it. It does not initialize,
// amend, or authorize any job and cannot borrow an MPS/HOST policy window.
func PrivateDiscoveryScopeDigestV2() string {
	return hashLabelV2("private-discovery-partition.v2/PARTITION")
}
func PrivateDiscoveryResourceRulesV2(b PrivateDiscoveryBindingV2) string {
	if !b.Limits.Valid() || len(b.Revision) > 64 || len(b.PlanIdentity) > 71 || len(b.DenominatorDigest) > 71 || len(b.MemberDigest) > 71 || len(b.Workspace) > 4096 || len(b.RuntimeFingerprint) > 71 {
		return ""
	}
	b.PolicyDigest = ""
	raw, e := json.Marshal(b)
	if e != nil {
		return ""
	}
	return hashLabelV2("private-discovery-partition.v2\x00" + string(raw))
}
func PrivateDiscoveryPolicyDigestV2(raw []byte) string {
	var j jobV2
	if parseV2(raw, &j) != nil {
		return ""
	}
	return hashLabelV2(string(raw))
}
func PrivateDiscoveryConstructionIdentityV2(c sessionruntime.PrivateDiscoveryConstruction) (members, runtime string) {
	if len(c.Members) == 0 || len(c.Members) > 16 || c.Start.SeedBinding != nil || !reflect.ValueOf(c.Start.ProviderIdentity).IsZero() || len(c.Start.RelationProviders) > 16 || len(c.Start.LanguageID) > 64 || len(c.Start.BootstrapAlias) > 4096 || len(c.Start.Profile.Workspace().String()) > 4096 || len(c.Start.Process.Path) > 4096 || len(c.Start.Process.Dir) > 4096 || len(c.Start.Process.Args) > 128 || len(c.Start.Process.Env) > 256 {
		return "", ""
	}
	for _, f := range c.Members {
		if len(f.Path) > 4096 || len(f.URI) > 4096 || len(f.LanguageID) > 64 {
			return "", ""
		}
	}
	for _, v := range c.Start.Process.Args {
		if len(v) > 4096 {
			return "", ""
		}
	}
	for _, v := range c.Start.Process.Env {
		if len(v) > 8192 {
			return "", ""
		}
	}
	for _, v := range c.Start.RelationProviders {
		if len(v) > 256 {
			return "", ""
		}
	}
	// Bound total escaped JSON before either identity serialization allocates.
	quoted := uint64(4096)
	take := func(s string) bool { quoted += 6*uint64(len(s)) + 32; return quoted <= 4<<20 }
	for _, f := range c.Members {
		if !take(f.Path) || !take(f.URI) || !take(f.LanguageID) {
			return "", ""
		}
	}
	for _, v := range []string{c.Start.Profile.Workspace().String(), c.Start.LanguageID, c.Start.BootstrapAlias, c.Start.Process.Path, c.Start.Process.Dir} {
		if !take(v) {
			return "", ""
		}
	}
	for _, list := range [][]string{c.Start.Process.Args, c.Start.Process.Env, c.Start.RelationProviders} {
		for _, v := range list {
			if !take(v) {
				return "", ""
			}
		}
	}
	m, e := json.Marshal(c.Members)
	if e != nil {
		return "", ""
	}
	r, e := json.Marshal(struct {
		Workspace, ProfileKey, Language, Alias string
		Process                                any
		Providers                              []string
		StartDeadline                          time.Time
	}{c.Start.Profile.Workspace().String(), c.Start.Profile.SessionKey().String(), c.Start.LanguageID, c.Start.BootstrapAlias, c.Start.Process, c.Start.RelationProviders, c.Start.Deadline})
	if e != nil || len(r) > 4<<20 {
		return "", ""
	}
	return hashLabelV2(string(m)), hashLabelV2(string(r))
}
func privateDiscoveryChargeV2(b PrivateDiscoveryBindingV2, now time.Time) (countersV2, error) {
	if b.Revision != "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73" || !hashV2(b.PlanIdentity) || !hashV2(b.DenominatorDigest) || !hashV2(b.MemberDigest) || !hashV2(b.RuntimeFingerprint) || !hashV2(b.PolicyDigest) || len(b.Workspace) == 0 || len(b.Workspace) > 4096 || b.Ordinal < 0 || !b.Limits.Valid() || !now.Before(b.Deadline) || b.Deadline.Sub(now) > 15*time.Second {
		return countersV2{}, ErrPrivateDiscoveryOperationV2
	}
	l := b.Limits
	// Closed logical currency quote: verification twice, full enumeration/plan and
	// receipt/index/publication/readback allowances, source and protocol passes.
	// Valid() supplies finite upper bounds before every multiplication/addition.
	const verification = 2 * (64 << 20)
	const metadata = 32 << 20
	data := l.SourceBytes + l.RequestBytes + l.ResponseBytes + l.RetainedBytes + metadata
	return countersV2{LogicalBytes: 32 * data, ReadHashBytes: verification + 32*data, WriteBytes: 8*metadata + 2*(recordLimitV2+2*headLimitV2), Records: 8192 + 65536, ParserBytes: verification + 32*data, ProviderRequests: l.Requests, ProviderMessages: l.Messages, Work: 32768 + 32*data + l.Work, ActiveNS: uint64(b.Deadline.Sub(now)), Calls: 32768 + 65536 + l.Requests + l.Messages, RequestBytes: l.RequestBytes, ResponseBytes: l.ResponseBytes, WireBytes: l.WireBytes, OutputBytes: l.RetainedBytes + metadata}, nil
}
func (o *PrivateDiscoveryOwnerV2) ReservePartition(ctx context.Context, b PrivateDiscoveryBindingV2, attempt string) (PrivateDiscoveryOperationV2, error) {
	if o == nil || ctx == nil || !hashV2(attempt) {
		return PrivateDiscoveryOperationV2{}, ErrPrivateDiscoveryOperationV2
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain {
		return PrivateDiscoveryOperationV2{}, ErrPrivateDiscoveryOperationV2
	}
	raw, e := sealV2(&o.job)
	if e != nil || o.job.Scope != PrivateDiscoveryScopeDigestV2() || o.job.ResourceRules != PrivateDiscoveryResourceRulesV2(b) || o.job.SourceAuthorization != b.DenominatorDigest || b.PolicyDigest != hashLabelV2(string(raw)) {
		return PrivateDiscoveryOperationV2{}, ErrPrivateDiscoveryOperationV2
	}
	now := time.Now()
	charge, e := privateDiscoveryChargeV2(b, now)
	if e != nil {
		return PrivateDiscoveryOperationV2{}, e
	}
	h, e := o.k.reserve(ctx, o.job, o.head, attempt, charge, b.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil || h.Disposition != "PENDING" || h.Attempt != attempt {
		o.uncertain = true
		return PrivateDiscoveryOperationV2{}, ErrPrivateDiscoveryOperationV2
	}
	o.head = h.Identity
	return PrivateDiscoveryOperationV2{&privateDiscoveryOperationV2{owner: o, binding: b, attempt: attempt, head: h.Identity}}, nil
}

// Execute exposes only the closed discovery session, not Manager or native
// proof-producing methods. A consumer can retain/fabricate metadata, but cannot
// fabricate genuine native target authority. Any error suppresses completion.
func (op PrivateDiscoveryOperationV2) Execute(ctx context.Context, c sessionruntime.PrivateDiscoveryConstruction, consume func(context.Context, *sessionruntime.PrivateDiscoverySession) error) error {
	if op.state == nil || ctx == nil || consume == nil {
		return ErrPrivateDiscoveryOperationV2
	}
	s := op.state
	s.mu.Lock()
	if s.used || s.finished {
		s.mu.Unlock()
		return ErrPrivateDiscoveryOperationV2
	}
	s.used = true
	s.mu.Unlock()
	// Freeze bounded caller-owned slices before matching the reserved identity.
	if len(c.Members) > 16 || len(c.Start.Process.Args) > 128 || len(c.Start.Process.Env) > 256 || len(c.Start.RelationProviders) > 16 {
		s.mu.Lock()
		s.executed = true
		s.executionErr = ErrPrivateDiscoveryOperationV2
		s.mu.Unlock()
		return ErrPrivateDiscoveryOperationV2
	}
	c.Members = slices.Clone(c.Members)
	c.Start.Process.Args = slices.Clone(c.Start.Process.Args)
	c.Start.Process.Env = slices.Clone(c.Start.Process.Env)
	c.Start.RelationProviders = slices.Clone(c.Start.RelationProviders)
	members, runtime := PrivateDiscoveryConstructionIdentityV2(c)
	b := s.binding
	if members != b.MemberDigest || runtime != b.RuntimeFingerprint || c.Start.Profile.Workspace().String() != b.Workspace || !c.Deadline.Equal(b.Deadline) || c.Limits != b.Limits {
		s.mu.Lock()
		s.executed = true
		s.executionErr = ErrPrivateDiscoveryOperationV2
		s.mu.Unlock()
		return ErrPrivateDiscoveryOperationV2
	}
	observation, e := sessionruntime.RunPrivateDiscovery(ctx, c, consume)
	s.mu.Lock()
	s.executed = true
	s.observation = observation
	s.executionErr = e
	s.mu.Unlock()
	if e != nil || observation.Failed || !observation.CleanupJoined {
		return ErrPrivateDiscoveryOperationV2
	}
	return nil
}

// Complete performs the original kernel observation after the caller's bounded
// receipt/index publication and readback. It cannot fabricate cleanup: only the
// concrete runner installs the private observation. Copies share this barrier.
func (op PrivateDiscoveryOperationV2) Complete(ctx context.Context) error {
	if op.state == nil || ctx == nil {
		return ErrPrivateDiscoveryOperationV2
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.used && !s.executed {
		return ErrPrivateDiscoveryOperationV2
	}
	s.finished = true
	disposition := "OBSERVED"
	if !s.used || s.executionErr != nil || s.observation.Failed {
		disposition = "FAILED"
	}
	if s.used && !s.observation.CleanupJoined {
		disposition = "INDETERMINATE"
	}
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return ErrPrivateDiscoveryOperationV2
	}
	h, e := o.k.observe(ctx, o.job, s.head, s.attempt, disposition, time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return ErrPrivateDiscoveryOperationV2
	}
	o.head = h.Identity
	if disposition != "OBSERVED" {
		return ErrPrivateDiscoveryOperationV2
	}
	return nil
}
