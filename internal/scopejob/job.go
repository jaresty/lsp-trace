package scopejob

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lsp-trace/internal/publication"
)

type Head struct {
	Schema       string     `json:"schema"`
	ID           string     `json:"id"`
	Config       Config     `json:"config"`
	ConfigDigest string     `json:"config_digest"`
	Started      time.Time  `json:"started"`
	Expires      *time.Time `json:"expires"`
	// Absent on original v1 headers: retain their historical work debit, never
	// rewrite/requote it. New jobs always use the complete policy work model.
	WorkModel string `json:"work_model,omitempty"`
}
type Spent struct {
	LogicalBytes uint64 `json:"logical_bytes"`
	Work         uint64 `json:"work"`
	Steps        uint64 `json:"steps"`
	Packets      uint64 `json:"packets"`
}
type claim struct {
	Sequence          uint64    `json:"sequence"`
	Previous          string    `json:"previous"`
	Spent             Spent     `json:"spent"`
	Steps             uint64    `json:"steps"`
	DiskEscrow        uint64    `json:"disk_escrow"`
	Frontier          []string  `json:"frontier"`
	Started           time.Time `json:"started"`
	OperationDeadline time.Time `json:"operation_deadline"`
}
type segment struct {
	Claim    string   `json:"claim"`
	Records  []Record `json:"records"`
	Frontier []string `json:"frontier"`
	Done     bool     `json:"done"`
	Reason   string   `json:"reason"`
}
type stop struct {
	Previous string   `json:"previous"`
	State    string   `json:"state"`
	Reason   string   `json:"reason"`
	Counts   Counts   `json:"counts"`
	Frontier []string `json:"frontier"`
}
type Report struct {
	Schema                   string     `json:"schema"`
	JobID                    string     `json:"job_id"`
	Head                     Head       `json:"head"`
	State                    string     `json:"state"`
	Reason                   string     `json:"reason"`
	Counts                   Counts     `json:"counts"`
	Spent                    Spent      `json:"spent"`
	RetainedBytes            uint64     `json:"retained_bytes"`
	PendingDiskEscrow        uint64     `json:"pending_disk_escrow"`
	StopDiskEscrow           uint64     `json:"stop_disk_escrow"`
	PendingClaim             bool       `json:"pending_claim"`
	PendingOperationDeadline *time.Time `json:"pending_operation_deadline"`
	PendingOperationExpired  bool       `json:"pending_operation_expired"`
	Frontier                 []string   `json:"frontier"`
	UnknownRemainder         bool       `json:"unknown_remainder"`
	DenominatorComplete      bool       `json:"denominator_complete"`
	Expired                  bool       `json:"expired"`
	SourceContentRead        bool       `json:"source_content_read"`
	AtomicSnapshot           bool       `json:"atomic_snapshot"`
	InventoryPublished       bool       `json:"inventory_published"`
	AcquisitionImplemented   bool       `json:"acquisition_implemented"`
	Gap                      string     `json:"capability_gap"`
	StatusOperation          []string   `json:"status_operation"`
	NamespaceVerification    string     `json:"namespace_verification"`
}

// Hooks are process-local test seams. They cannot change persisted configuration
// or renew a job. AfterClaim permits a test to interrupt after durable debit.
type Hooks struct {
	Now        func() time.Time
	AfterClaim func(uint64) error
	Started    func(string) error // durable job ID is available before enumeration
}

func (h Hooks) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}
func prefix(id string) string {
	if strings.HasPrefix(id, "v2-") {
		return "scope-" + id + "/packet-"
	}
	return "scope-v1-" + id + "-"
}
func validID(id string) bool {
	id = strings.TrimPrefix(id, "v2-")
	raw, e := hex.DecodeString(id)
	return e == nil && len(raw) == 16 && hex.EncodeToString(raw) == id
}
func filename(id, kind string, seq uint64) string {
	return fmt.Sprintf("%s%08d-%s.json", prefix(id), seq, kind)
}
func put(root *publication.Root, name string, v any, limit uint64) ([]byte, error) {
	raw, e := seal(v)
	if e != nil || uint64(len(raw)) > limit {
		return nil, ErrInvalid
	}
	r, e := publication.PublishBoundFile(root, name, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrIntegrity
		}
		return nil
	})
	if e != nil || r == nil || !r.NamespaceAtomic || r.VerificationStatus != "VERIFIED" || r.DirectorySyncStatus != publication.DirectorySyncComplete || r.CloseStatus != publication.CloseComplete {
		return nil, errors.Join(ErrIntegrity, e)
	}
	return raw, nil
}
func get(root *publication.Root, name string, v any, limit uint64) ([]byte, error) {
	return getContext(context.Background(), root, name, v, limit)
}
func getContext(ctx context.Context, root *publication.Root, name string, v any, limit uint64) ([]byte, error) {
	raw, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, root, name, int64(limit))
	if e != nil {
		return nil, e
	}
	if unseal(raw, v) != nil {
		return nil, ErrIntegrity
	}
	return raw, nil
}
func baseSpent() Spent { return Spent{LogicalBytes: 4 * (HeadBytes + StopBytes), Packets: 2} }
func initialSpent(h Head) Spent {
	s := baseSpent()
	if h.Schema == Schema {
		s.Work = NamespaceWork
	}
	return s
}
func quote(h Head, s Spent, n uint64) (Spent, bool) {
	b := h.Config.Budget
	// Logical units, not instructions: eight fixed operations cover entry read,
	// metadata, directory opens/stat/close and disposition; Depth prefix checks;
	// Depth frame plus Roots frontier copies; Prunes comparisons; the historical
	// filter coefficient reserves validation plus matching per step. Preparation
	// is now operation-local, but persisted policy-v2 debits are not discounted.
	// Pattern/path byte processing remains separately bounded by config ceilings.
	work, ok := EnumerationWork(h.Config, n)
	if !within(h, s) || !ok {
		return s, false
	}
	if n > b.Steps-s.Steps || work > b.Work-s.Work || BatchBytes > b.LogicalBytes-s.LogicalBytes || 2 > b.Packets-s.Packets {
		return s, false
	}
	s.Steps += n
	s.Work += work
	s.LogicalBytes += BatchBytes
	s.Packets += 2
	return s, true
}
func within(h Head, s Spent) bool {
	b := h.Config.Budget
	return s.Steps <= b.Steps && s.Work <= b.Work && s.LogicalBytes <= b.LogicalBytes && s.Packets <= b.Packets
}

// Start owns the sole live cursor. There is deliberately no Resume operation.
// The first header reserves the stop artifact. Every claim reserves its segment
// before metadata effects. No failed claim is refunded or retried.
func Start(ctx context.Context, root *publication.Root, c Config, hooks Hooks) (Report, error) {
	c, e := Canonical(c)
	if e != nil || c.Selection != nil || ctx == nil || root == nil || c.Budget.Work < NamespaceWork {
		return Report{}, ErrInvalid
	}
	if e = root.ValidatePrivate(); e != nil {
		return Report{}, e
	}
	// Reject invalid workspace aliases before publishing a job. No source tree
	// enumeration occurs here; the first paid claim owns opening/traversal.
	real, e := filepath.EvalSymlinks(c.Workspace)
	if e != nil || real != c.Workspace {
		return Report{}, ErrInvalid
	}
	info, e := os.Lstat(c.Workspace)
	if e != nil || !info.IsDir() {
		return Report{}, ErrInvalid
	}
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return Report{}, e
	}
	id := "v2-" + hex.EncodeToString(nonce[:])
	// The fixed header/stop reservation includes namespace admission and receipt
	// output. It is admitted before mkdir; no traversal debit is renewed.
	child, e := root.OpenPrivateChild("scope-"+id, true)
	if e != nil {
		return Report{}, e
	}
	if e = child.Close(); e != nil {
		return Report{}, errors.Join(e, root.RemoveEmptyPrivateChild("scope-"+id))
	}
	now := hooks.now()
	configRaw, _ := json.Marshal(c)
	h := Head{Schema: Schema, ID: id, Config: c, ConfigDigest: digest(configRaw), Started: now, Expires: c.JobDeadline, WorkModel: "policy-v2"}
	raw, e := put(root, prefix(id)+"head.json", h, HeadBytes-NamespaceBytes)
	if e != nil {
		// A late publication failure may have installed the header. Never erase it.
		cleanup := root.RemoveEmptyPrivateChild("scope-" + id)
		return Report{JobID: id}, errors.Join(e, cleanup)
	}
	retained := uint64(len(raw))
	previous := digest(raw)
	spent := initialSpent(h)
	frontier := append([]string(nil), c.Roots...)
	counts := Counts{}
	var cur *cursor
	defer func() {
		if cur != nil {
			_ = cur.close()
		}
	}()
	finish := func(state, reason string) (Report, error) {
		if cur != nil {
			if e := cur.close(); e != nil {
				state = "PARTIAL"
				reason = "CLOSE_FAILED"
			}
		}
		_, e := put(root, prefix(id)+"stop.json", stop{previous, state, reason, counts, frontier}, StopBytes)
		if e != nil {
			return Report{JobID: id}, e
		}
		report, err := Inspect(root, id, hooks.now())
		if err != nil {
			report.JobID = id
		}
		return report, err
	}
	if hooks.Started != nil {
		if e := hooks.Started(id); e != nil {
			return finish("PARTIAL", "OUTPUT_UNAVAILABLE")
		}
	}
	for seq := uint64(0); ; seq++ {
		if expired(h, hooks.now()) {
			return finish("PARTIAL", "EXPIRED")
		}
		if ctx.Err() != nil {
			return finish("PARTIAL", "INTERRUPTED")
		}
		remaining := c.Budget.Steps - spent.Steps
		n := min(BatchSteps, remaining)
		if n == 0 {
			return finish("PARTIAL", "STEP_LIMIT")
		}
		next, ok := quote(h, spent, n)
		if !ok {
			return finish("PARTIAL", "BUDGET_EXHAUSTED")
		}
		if ClaimBytes+SegmentBytes+StopBytes > c.Budget.RetainedBytes-retained {
			return finish("PARTIAL", "RETAINED_STORAGE_LIMIT")
		}
		operationStart := hooks.now()
		q := claim{seq, previous, next, n, SegmentBytes, append([]string(nil), frontier...), operationStart, operationStart.Add(c.Budget.OperationTimeout)}
		claimRaw, e := put(root, filename(id, "claim", seq), q, ClaimBytes)
		if e != nil {
			return Report{JobID: id}, e
		}
		spent = next
		retained += uint64(len(claimRaw))
		// A crash or injected failure here leaves an explicit pending claim. Readback
		// exposes full reserved spend and unknown outcomes; never fabricates zero work.
		if hooks.AfterClaim != nil {
			if e = hooks.AfterClaim(seq); e != nil {
				return Report{JobID: id}, e
			}
		}
		operationDeadline := q.OperationDeadline
		leaseReason := func() string {
			if ctx.Err() != nil {
				return "INTERRUPTED"
			}
			now := hooks.now()
			if expired(h, now) {
				return "EXPIRED"
			}
			if !now.Before(operationDeadline) {
				return "OPERATION_TIMEOUT"
			}
			return ""
		}
		result := segment{Claim: digest(claimRaw), Frontier: append([]string(nil), frontier...), Reason: leaseReason()}
		if result.Reason == "" && cur == nil {
			cur, e = newCursor(c)
			if e != nil {
				result.Reason = "SOURCE_UNAVAILABLE"
			}
		}
		for i := uint64(0); result.Reason == "" && i < n; i++ {
			result.Reason = leaseReason()
			if result.Reason != "" {
				break
			}
			r, e := cur.step()
			if r != nil {
				result.Records = append(result.Records, *r)
				counts.add(*r)
			}
			if e != nil {
				switch {
				case errors.Is(e, errEntries):
					result.Reason = "ENTRY_LIMIT"
				case errors.Is(e, errDirectories):
					result.Reason = "DIRECTORY_LIMIT"
				default:
					result.Reason = "TRAVERSAL_UNAVAILABLE"
				}
				break
			}
			result.Frontier = cur.frontier()
			result.Reason = leaseReason()
			if result.Reason != "" {
				break
			}
			result.Done = cur.done()
			if result.Done {
				break
			}
		}
		// On failure, the last successful frontier conservatively includes the
		// enclosing directory/root of any consumed but unrecorded metadata.
		segmentRaw, e := put(root, filename(id, "segment", seq), result, SegmentBytes)
		if e != nil {
			return Report{JobID: id}, e
		}
		retained += uint64(len(segmentRaw))
		previous = digest(segmentRaw)
		frontier = result.Frontier
		if result.Reason != "" {
			return finish("PARTIAL", result.Reason)
		}
		if result.Done {
			return finish("SCOPE_READY", "ACQUISITION_NOT_IMPLEMENTED")
		}
	}
}

// Inspect is bounded administrative artifact verification, not execution or a
// new job allowance. It performs no source IO and never writes/renews a job.
// Streaming one segment at a time also makes post-expiry inspection possible.
func Inspect(root *publication.Root, id string, now time.Time) (Report, error) {
	return InspectContext(context.Background(), root, id, now)
}

// InspectContext bounds one administrative readback, not the job lifetime. The
// caller can shorten but cannot extend the default operation timeout. A shorter
// frozen timeout also applies, measured from entry (not renewed after the head).
// Cancellation is cooperative, not OS syscall preemption. No partial report is
// authoritative on error; saved claims and budgets are never changed.
func InspectContext(ctx context.Context, root *publication.Root, id string, now time.Time) (Report, error) {
	return inspectContext(ctx, root, id, now, time.Now())
}

// started is operation-local; a private seam lets tests advance elapsed time
// without sleeps, changing the job's reporting clock, or renewing its deadline.
func inspectContext(ctx context.Context, root *publication.Root, id string, now, started time.Time) (report Report, err error) {
	if ctx == nil || root == nil {
		return Report{}, ErrInvalid
	}
	ctx, cancel := inspectionContext(ctx, started, DefaultBudget().OperationTimeout)
	defer cancel()
	var frozenCancel context.CancelFunc
	defer func() {
		if frozenCancel != nil {
			defer frozenCancel()
		}
		if e := ctx.Err(); e != nil {
			err = errors.Join(err, e)
		}
		if err != nil {
			report = Report{}
		}
	}()
	if e := ctx.Err(); e != nil {
		return Report{}, e
	}
	if !validID(id) {
		return Report{}, ErrInvalid
	}
	if strings.HasPrefix(id, "v2-") {
		child, err := root.OpenPrivateChild("scope-"+id, false)
		if err != nil {
			return Report{}, err
		}
		if err = child.Close(); err != nil {
			return Report{}, err
		}
	}
	var h Head
	raw, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, root, prefix(id)+"head.json", int64(HeadBytes))
	if e != nil {
		return Report{}, e
	}
	var envelope sealed
	if strict(raw, &envelope) != nil || digest(envelope.Payload) != envelope.Digest {
		return Report{}, ErrIntegrity
	}
	var version struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(envelope.Payload, &version) != nil {
		return Report{}, ErrIntegrity
	}
	if version.Schema != Schema && version.Schema != LegacySchema {
		return Report{}, ErrUnsupportedVersion
	}
	if unseal(raw, &h) != nil {
		return Report{}, ErrIntegrity
	}
	// Schema selects a closed, known tuple, never an inferred migration.
	isolated := strings.HasPrefix(id, "v2-")
	if isolated && (uint64(len(raw)) > HeadBytes-NamespaceBytes || h.Config.Budget.Work < NamespaceWork) {
		return Report{}, ErrIntegrity
	}
	if !(isolated && h.Schema == Schema && h.WorkModel == "policy-v2" || !isolated && h.Schema == LegacySchema && (h.WorkModel == "" || h.WorkModel == "policy-v2")) {
		return Report{}, ErrUnsupportedVersion
	}
	canonical, e := Canonical(h.Config)
	configRaw, _ := json.Marshal(canonical)
	original, _ := json.Marshal(h.Config)
	if e != nil || h.Config.Selection != nil || !bytes.Equal(configRaw, original) || h.ID != id || digest(configRaw) != h.ConfigDigest || h.Started.IsZero() || !sameDeadline(h.Expires, h.Config.JobDeadline) {
		return Report{}, ErrIntegrity
	}
	ctx, frozenCancel = inspectionContext(ctx, started, h.Config.Budget.OperationTimeout)
	if e := ctx.Err(); e != nil {
		return Report{}, e
	}
	out := Report{Schema: Schema, JobID: id, Head: h, State: "UNSEALED", Reason: "ENUMERATION_ATTEMPT_NOT_RESUMABLE", Spent: initialSpent(h), RetainedBytes: uint64(len(raw)), StopDiskEscrow: StopBytes, Frontier: append([]string(nil), h.Config.Roots...), UnknownRemainder: true, Expired: expired(h, now), Gap: "Multipart acquisition is not implemented; no executable acquisition continuation is returned.", StatusOperation: []string{"census", "job-status", id}}
	previous := digest(raw)
	lastDone := false
	lastReason := ""
	var claims, segments uint64
	progress := EnumerationReplay{}
	for seq := uint64(0); ; seq++ {
		var q claim
		qr, e := getContext(ctx, root, filename(id, "claim", seq), &q, ClaimBytes)
		if errors.Is(e, os.ErrNotExist) {
			break
		}
		if e != nil {
			return Report{}, e
		}
		if !within(h, out.Spent) || q.Steps == 0 || q.Steps > BatchSteps {
			return Report{}, ErrIntegrity
		}
		expected, ok := quote(h, out.Spent, q.Steps)
		if h.WorkModel == "" {
			// Original candidate v1 used this smaller coefficient. Verification
			// preserves its debit; this is never used to authorize new execution.
			coefficient := uint64(8 + h.Config.Budget.Depth + len(h.Config.Includes) + len(h.Config.Excludes))
			expected = out.Spent
			expected.Steps += q.Steps
			expected.Work += q.Steps * coefficient // Steps<=128, config ceilings validated
			expected.LogicalBytes += BatchBytes
			expected.Packets += 2
			ok = within(h, expected)
		}
		if !ok || lastDone || lastReason != "" || q.Sequence != seq || q.Previous != previous || q.Spent != expected || q.DiskEscrow != SegmentBytes || !q.OperationDeadline.Equal(q.Started.Add(h.Config.Budget.OperationTimeout)) || q.Started.Before(h.Started) || !equalStrings(q.Frontier, out.Frontier) {
			return Report{}, ErrIntegrity
		}
		claims++
		out.Spent = q.Spent
		out.RetainedBytes += uint64(len(qr))
		out.PendingClaim = true
		out.PendingDiskEscrow = SegmentBytes
		out.PendingOperationDeadline = &q.OperationDeadline
		out.PendingOperationExpired = !now.Before(q.OperationDeadline)
		var s segment
		sr, e := getContext(ctx, root, filename(id, "segment", seq), &s, SegmentBytes)
		if errors.Is(e, os.ErrNotExist) {
			break
		}
		if e != nil {
			return Report{}, e
		}
		if s.Claim != digest(qr) {
			return Report{}, ErrIntegrity
		}
		if e := progress.Apply(ctx, h.Config, EnumerationBatch{Records: s.Records, Frontier: s.Frontier, Done: s.Done, Reason: s.Reason}, q.Steps); e != nil {
			return Report{}, e
		}
		out.Counts = progress.Counts()
		segments++
		out.RetainedBytes += uint64(len(sr))
		out.PendingClaim = false
		out.PendingDiskEscrow = 0
		out.PendingOperationDeadline = nil
		out.PendingOperationExpired = false
		out.Frontier = s.Frontier
		previous = digest(sr)
		lastDone = s.Done
		lastReason = s.Reason
		if out.RetainedBytes+StopBytes > h.Config.Budget.RetainedBytes {
			return Report{}, ErrIntegrity
		}
	}
	var final stop
	fr, e := getContext(ctx, root, prefix(id)+"stop.json", &final, StopBytes)
	if e == nil {
		if out.PendingClaim || final.Previous != previous || final.Counts != out.Counts || !equalStrings(final.Frontier, out.Frontier) || !validStop(final, lastDone, lastReason) {
			return Report{}, ErrIntegrity
		}
		out.State = final.State
		out.Reason = final.Reason
		out.UnknownRemainder = final.State != "SCOPE_READY" || out.Counts.Boundaries > 0
		out.DenominatorComplete = !out.UnknownRemainder
		out.RetainedBytes += uint64(len(fr))
		out.StopDiskEscrow = 0
	} else if !errors.Is(e, os.ErrNotExist) {
		return Report{}, e
	}
	if out.RetainedBytes+out.PendingDiskEscrow+out.StopDiskEscrow > h.Config.Budget.RetainedBytes {
		return Report{}, ErrIntegrity
	}
	// No successful prefix report until all same-job names are accounted for.
	// New-layout existing names, not possible ordinals: a sparse far-index orphan
	// costs one name visit. The packet ceiling applies only to the owned child.
	// Legacy flat storage uses explicitly qualified bounded ordinal probes.
	if e := inspectNames(ctx, root, id, h.Config.Budget.Packets, claims, segments, out.StopDiskEscrow == 0); e != nil {
		return Report{}, e
	}
	if isolated {
		out.NamespaceVerification = "COMPLETE_PER_JOB"
	} else {
		out.NamespaceVerification = "LEGACY_LEGAL_ORDINALS_ONLY"
		out.DenominatorComplete = false
		out.UnknownRemainder = true
	}
	return out, nil
}
func inspectionContext(parent context.Context, started time.Time, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, started.Add(min(timeout, DefaultBudget().OperationTimeout)))
}

func inspectNames(ctx context.Context, root *publication.Root, id string, limit, claims, segments uint64, stopped bool) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if strings.HasPrefix(id, "v2-") {
		child, e := root.OpenPrivateChild("scope-"+id, false)
		if e != nil {
			return e
		}
		e = inspectOwnedNames(ctx, child, id, limit, claims, segments, stopped)
		return errors.Join(e, child.Close())
	}
	return inspectLegacyNames(ctx, root, id, limit, claims, segments)
}
func inspectOwnedNames(ctx context.Context, root *publication.Root, id string, limit, claims, segments uint64, stopped bool) error {
	var found uint64
	err := root.VisitNamesBounded(limit, func(name string) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		name = "scope-" + id + "/" + name
		found++
		if name == prefix(id)+"head.json" {
			return nil
		}
		if name == prefix(id)+"stop.json" && stopped {
			return nil
		}
		tail := strings.TrimPrefix(name, prefix(id))
		ordinal, kind, ok := strings.Cut(tail, "-")
		seq, e := strconv.ParseUint(ordinal, 10, 64)
		if !ok || e != nil {
			return ErrIntegrity
		}
		if kind == "claim.json" && seq < claims && name == filename(id, "claim", seq) {
			return nil
		}
		if kind == "segment.json" && seq < segments && name == filename(id, "segment", seq) {
			return nil
		}
		return ErrIntegrity
	})
	expected := uint64(1) + claims + segments
	if stopped {
		expected++
	}
	if err != nil || found != expected {
		return errors.Join(ErrIntegrity, err)
	}
	return nil
}
func expired(h Head, now time.Time) bool { return h.Expires != nil && !now.Before(*h.Expires) }
func sameDeadline(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func validFrontier(c Config, paths []string) bool {
	if len(paths) > 128+c.Budget.Depth {
		return false
	}
	seen := make(map[string]bool, len(paths)) // at most roots + depth
	for _, p := range paths {
		if !scopePath(c, p) || seen[p] {
			return false
		}
		seen[p] = true
	}
	return true
}
func scopePath(c Config, p string) bool {
	if !local(p, c.Budget.PathBytes) || pathDepth(p) > c.Budget.Depth {
		return false
	}
	for _, root := range c.Roots {
		if beneath(root, p) {
			return true
		}
	}
	return false
}
func pruned(c Config, p string) bool {
	for _, root := range c.Prunes {
		if beneath(root, p) {
			return true
		}
	}
	return false
}
func validRecord(c Config, r Record) bool {
	var selection regularSelector
	return validRecordSelected(c, r, &selection)
}
func validRecordSelected(c Config, r Record, selection *regularSelector) bool {
	if !scopePath(c, r.Path) || r.Kind != "REGULAR_FILE" && r.LanguageID != "" {
		return false
	}
	if !r.MetadataObserved {
		return r.Kind == "OTHER" && r.Role == "UNAVAILABLE" && r.UnknownDescendants && r.Size == 0 && r.Mode == 0 && r.ModifiedNS == 0
	}
	if r.Size < 0 {
		return false
	}
	mode := os.FileMode(r.Mode)
	switch r.Kind {
	case "REGULAR_FILE":
		role, language, e := selection.selectRegular(c, r.Path)
		return mode.IsRegular() && !r.UnknownDescendants && e == nil && r.Role == role && r.LanguageID == language
	case "DIRECTORY":
		if !mode.IsDir() {
			return false
		}
		if pruned(c, r.Path) {
			return r.Role == "EXCLUDED" && r.UnknownDescendants
		}
		return r.Role == "DIRECTORY" && !r.UnknownDescendants || r.Role == "UNAVAILABLE" && r.UnknownDescendants
	case "OTHER":
		return !mode.IsRegular() && !mode.IsDir() && r.Role == "UNSUPPORTED_KIND" && !r.UnknownDescendants
	}
	return false
}
func validSegmentReason(reason string) bool {
	switch reason {
	case "", "INTERRUPTED", "EXPIRED", "OPERATION_TIMEOUT", "SOURCE_UNAVAILABLE", "ENTRY_LIMIT", "DIRECTORY_LIMIT", "TRAVERSAL_UNAVAILABLE", "FILE_ASSOCIATION_CONFLICT":
		return true
	}
	return false
}
func validStop(s stop, done bool, reason string) bool {
	if s.State == "SCOPE_READY" {
		return done && reason == "" && len(s.Frontier) == 0 && s.Reason == "ACQUISITION_NOT_IMPLEMENTED"
	}
	if s.State != "PARTIAL" || s.Reason == "" {
		return false
	}
	// close failure can replace any preceding outcome, including completed traversal.
	if s.Reason == "CLOSE_FAILED" {
		return true
	}
	if done {
		return false
	}
	if reason != "" {
		return s.Reason == reason
	}
	switch s.Reason {
	case "OUTPUT_UNAVAILABLE", "EXPIRED", "INTERRUPTED", "STEP_LIMIT", "BUDGET_EXHAUSTED", "RETAINED_STORAGE_LIMIT":
		return len(s.Frontier) != 0
	}
	return false
}
