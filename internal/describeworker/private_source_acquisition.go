package describeworker

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/targetpacket"
)

// A constant cannot be reassigned to nil (or another result) by a public caller.
type sourceExecutionDisabledError string

func (e sourceExecutionDisabledError) Error() string { return string(e) }

const ErrSourceExecutionDisabled sourceExecutionDisabledError = "source acquisition execution disabled"

// SourceAcquisitionContext is a lower, nonadmitting description. Only the upper
// source continuation can join it to an actual opaque producer admission.
// Deadline and ceilings are the original source contract, never a new grant.
type SourceAcquisitionContext struct {
	HandoffID, CheckpointID, SourceUnitID, SourceDigest, Workspace, PolicyID string
	Deadline                                                                 time.Time
	SourceLimits                                                             targetpacket.PrivateSourceLimits
	MaxWireBytes, MaxParentBytes                                             int
}

// SourceAcquisitionPlan is runner-owned, prepare-only state, not an invocation,
// source admission, current-file attestation, or permission to start a process.
// Pins describe the config validated by NewRunner at construction time only.
type SourceAcquisitionPlan struct{ state *sourceAcquisitionPlan }
type sourceAcquisitionPlan struct {
	owner                     *Runner
	config                    Config
	request                   describerequest.Record
	source                    SourceAcquisitionContext
	attempt, id, promptDigest string
	retained                  uint64
}

func (p SourceAcquisitionPlan) ID() string {
	if p.state == nil {
		return ""
	}
	return p.state.id
}

// PrepareSourceAcquisition performs no filesystem, supervisor or backend work.
// All variable metadata is bounded before validation's marshal/hash operations.
func (r *Runner) PrepareSourceAcquisition(ctx context.Context, request describerequest.Record, attempt string, source SourceAcquisitionContext) (SourceAcquisitionPlan, error) {
	bad := func() (SourceAcquisitionPlan, error) {
		return SourceAcquisitionPlan{}, errors.New("source acquisition candidate refused")
	}
	if r == nil || r.manager == nil || !sourceAcquisitionLive(ctx, source.Deadline) {
		return bad()
	}
	c := r.validated.Config()
	upper, ok := sourceAcquisitionUpper(c, request, attempt, source)
	if !ok {
		return bad()
	}
	if err := describerequest.ValidateRecord(request); err != nil {
		return bad()
	}
	if request.Lineage.ConsumerResolution != describerequest.PrivateSourceOnly || request.Envelope.AdmissionID != source.SourceUnitID || request.Lineage.PacketID != source.SourceUnitID || "sha256:"+request.Envelope.SourceRevision != source.SourceDigest {
		return bad()
	}
	// UTC changes only the identity encoding, not the retained original deadline.
	// Do not ask time's JSON encoder to resolve an ambient local time zone.
	wireSource := source
	wireSource.Deadline = source.Deadline.UTC()
	wire := struct {
		Config  Config
		Request describerequest.Record
		Attempt string
		Source  SourceAcquisitionContext
	}{c, request, attempt, wireSource}
	raw, err := json.Marshal(wire)
	if err != nil || uint64(len(raw)) > upper/4 {
		return bad()
	}
	id := digestBytes(raw)
	prompt := digestBytes([]byte(request.Envelope.Prompt))
	if !sourceAcquisitionLive(ctx, source.Deadline) {
		return bad()
	}
	return SourceAcquisitionPlan{&sourceAcquisitionPlan{r, c, request, source, attempt, id, prompt, upper}}, nil
}

// SourceAcquisitionRetentionUpper is nonadmitting sizing, so the upper source
// bridge can check existing retained state PLUS this allowance before preparing
// any new marshal/hash buffers. It does not issue a plan or admission.
func (r *Runner) SourceAcquisitionRetentionUpper(request describerequest.Record, attempt string, source SourceAcquisitionContext) (uint64, error) {
	if r == nil || r.manager == nil {
		return 0, errors.New("source acquisition unvalidated runner")
	}
	n, ok := sourceAcquisitionUpper(r.validated.Config(), request, attempt, source)
	if !ok {
		return 0, errors.New("source acquisition metadata bound")
	}
	return n, nil
}

// ValidateSourceAcquisition checks exact ownership and the complete immutable
// tuple. Its returned allowance counts variable config paths/identities/pins,
// the retained record/context and finite prepare/validation marshal/hash copies.
// It does not re-read pins or infer execution authorization from Preflight.
func (r *Runner) ValidateSourceAcquisition(ctx context.Context, p SourceAcquisitionPlan, request describerequest.Record, attempt string, source SourceAcquisitionContext) (uint64, error) {
	s := p.state
	if r == nil || r.manager == nil || s == nil || s.owner != r || r.validated.Config() != s.config || s.request != request || s.attempt != attempt || s.source != source || !sourceAcquisitionLive(ctx, source.Deadline) {
		return 0, errors.New("source acquisition exact runner/config/request/source/attempt mismatch")
	}
	return s.retained, nil
}

// ValidateSourceAcquisitionResponseLimits constrains supplied parser buffers to
// the same immutable runner output/work caps; it does not authorize execution.
func (r *Runner) ValidateSourceAcquisitionResponseLimits(p SourceAcquisitionPlan, limits SuppliedResponseLimits) error {
	if r == nil || p.state == nil || p.state.owner != r || r.validated.Config() != p.state.config || !limits.Valid() || limits.MaxAdapterBytes > p.state.config.Limits.StdoutBytes || limits.MaxSemanticBytes > p.state.config.Limits.WorkBytes {
		return errors.New("source acquisition supplied limits exceed runner snapshot")
	}
	return nil
}

// RunSourceAcquisition is intentionally incapable of execution. Even a nil
// receiver and a valid plan return before IO, profile reads or manager access.
// There is no flag, callback, environment switch or authorization object.
func (r *Runner) RunSourceAcquisition(_ context.Context, _ SourceAcquisitionPlan) error {
	return ErrSourceExecutionDisabled
}

func sourceAcquisitionLive(ctx context.Context, deadline time.Time) bool {
	return ctx != nil && ctx.Err() == nil && !deadline.IsZero() && time.Now().Before(deadline)
}

func sourceAcquisitionUpper(c Config, r describerequest.Record, attempt string, s SourceAcquisitionContext) (uint64, bool) {
	if (s.Deadline.Location() != time.Local && s.Deadline.Location() != time.UTC) || c.ResponseVersion != ResponseVersionV2 || validateLimits(c.Limits) != nil || !s.SourceLimits.Valid() || s.MaxWireBytes <= 0 || s.MaxWireBytes > 8<<20 || s.MaxParentBytes <= 0 || len(attempt) == 0 || len(attempt) > 256 || strings.TrimSpace(attempt) != attempt {
		return 0, false
	}
	if c.RuntimeIdentity == "" || c.AdapterIdentity == "" || c.ModelIdentity == "" || s.HandoffID == "" || s.CheckpointID == "" || s.SourceUnitID == "" || s.SourceDigest == "" || s.Workspace == "" || s.PolicyID == "" {
		return 0, false
	}
	cap := uint64(s.MaxWireBytes)
	for _, n := range []int{s.MaxParentBytes, c.Limits.WorkBytes, c.Limits.TempBytes} {
		if uint64(n) < cap {
			cap = uint64(n)
		}
	}
	// Sixfold JSON escaping plus fixed numeric/key/time overhead. Four complete
	// uppers cover retained values, encoding, identity input and defensive copies.
	upper := uint64(8192)
	add := func(v string) bool {
		if uint64(len(v)) > cap/6 || upper > cap || uint64(len(v))*6 > cap-upper {
			return false
		}
		upper += uint64(len(v)) * 6
		return true
	}
	for _, p := range []FilePin{c.Worker, c.Model, c.Library, c.SandboxExecutable, c.SandboxProfile, c.Grammar} {
		if !add(p.Path) || !add(p.SHA256) || len(p.SHA256) != 71 || !digestPattern.MatchString(p.SHA256) || !filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path {
			return 0, false
		}
	}
	for _, v := range []string{c.RuntimeIdentity, c.AdapterIdentity, c.ModelIdentity, c.ResponseVersion, attempt, s.HandoffID, s.CheckpointID, s.SourceUnitID, s.SourceDigest, s.Workspace, s.PolicyID,
		r.RecordID, r.Envelope.Protocol, r.Envelope.MessageType, r.Envelope.MessageID, r.Envelope.CorrelationID, r.Envelope.AdmissionID, r.Envelope.Corpus, r.Envelope.ItemType, r.Envelope.AcquisitionMode, r.Envelope.InputSHA256, r.Envelope.SourceRevision, r.Envelope.TargetMessageID, r.Envelope.Prompt,
		r.Lineage.PacketID, r.Lineage.CensusID, r.Lineage.LineageIdentity, r.Lineage.ConsumerResolution, r.Lineage.AlternativeID, r.Lineage.Completeness, r.Lineage.SourceGraphComplete} {
		if !add(v) {
			return 0, false
		}
	}
	if upper > cap || upper > uint64(s.MaxParentBytes)/4 {
		return 0, false
	}
	return upper * 4, true
}
