//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
	"lsp-trace/internal/publication"
)

// This private rehearsal accepts only exact precreated roots. It creates no
// directories, changes no modes, deletes nothing, and offers no execution admission.
type scopeRehearsalOptions struct {
	source, revision string
	roots            [3]string
}

func scopeRehearsalEnvironment(get func(string) string) (scopeRehearsalOptions, bool, error) {
	var o scopeRehearsalOptions
	opt := get("LSP_TRACE_SCOPE_REHEARSAL")
	if opt == "" {
		return o, false, nil
	}
	if opt != "retain-inputs" {
		return o, false, ErrInvalid
	}
	o.source, o.revision = get("LSP_TRACE_SCOPE_SOURCE"), get("LSP_TRACE_SCOPE_REVISION")
	for i, name := range []string{"LSP_TRACE_SCOPE_PRIMARY", "LSP_TRACE_SCOPE_BACKUP", "LSP_TRACE_SCOPE_JOURNAL"} {
		o.roots[i] = get(name)
		if o.roots[i] == "" {
			return o, true, ErrInvalid
		}
	}
	if o.source == "" || !featureScopeRevision.MatchString(o.revision) {
		return o, true, ErrInvalid
	}
	return o, true, nil
}

type scopeRehearsalBlob struct {
	ID         string `json:"id"`
	ByteLength uint64 `json:"byte_length"`
}

func rehearsalBlob(b []byte) scopeRehearsalBlob { return scopeRehearsalBlob{Digest(b), uint64(len(b))} }

type scopeRehearsalStep struct {
	Outcome      string                                `json:"outcome"`
	Error        string                                `json:"error,omitempty"`
	CASCommitted bool                                  `json:"cas_committed"`
	CAS          *publication.CompareAndReplaceReceipt `json:"cas"`
	Head         []byte                                `json:"head_bytes_base64,omitempty"`
	Receipt      []byte                                `json:"receipt_bytes_base64,omitempty"`
	ReceiptID    string                                `json:"receipt_id,omitempty"`
	Linked       [][]byte                              `json:"linked_receipt_bytes_base64,omitempty"`
}

func rehearsalStep(r scopeBundleResult, err error) scopeRehearsalStep {
	s := scopeRehearsalStep{Outcome: r.outcome, CASCommitted: r.casCommitted, CAS: r.cas, Head: r.HeadBytes(), Receipt: r.ReceiptBytes(), ReceiptID: r.receiptSelector}
	if err != nil {
		s.Error = err.Error()
	}
	for _, raw := range r.LinkedReceiptBytes() {
		s.Linked = append(s.Linked, raw)
	}
	return s
}

type scopeRehearsalUsage struct {
	Files          uint64 `json:"files"`
	LogicalBytes   uint64 `json:"logical_file_bytes"`
	AllocatedBytes uint64 `json:"allocated_file_bytes"`
	// Blocks are measured for regular files only, not directory/volume overhead.
}
type scopeRehearsalReport struct {
	Schema                        string                                    `json:"schema"`
	Authority                     uint8                                     `json:"authority"`
	Accepted                      bool                                      `json:"accepted"`
	Completeness                  string                                    `json:"completeness"`
	FeatureIdentity               string                                    `json:"feature_identity"`
	RevisionQualification         string                                    `json:"revision_qualification"`
	Scope                         scopeRehearsalBlob                        `json:"scope"`
	Selection                     scopeRehearsalBlob                        `json:"selection"`
	Plan                          scopeRehearsalBlob                        `json:"plan"`
	Nomination                    int                                       `json:"nomination_count"`
	Context                       int                                       `json:"context_count"`
	References                    int                                       `json:"reference_count"`
	UniqueObjects                 int                                       `json:"unique_object_count"`
	ReferencePayloadBytes         uint64                                    `json:"reference_payload_bytes"`
	UniquePayloadBytes            uint64                                    `json:"unique_payload_bytes"`
	MetadataBytesPerCopy          uint64                                    `json:"metadata_bytes_per_copy"`
	ReservedLogicalBytes          uint64                                    `json:"reserved_logical_bytes_not_actual_usage"`
	ReservedWorkBytes             uint64                                    `json:"reserved_work_bytes"`
	OwnerChargedLogicalBytes      uint64                                    `json:"owner_charged_logical_bytes"`
	OwnerChargedWorkBytes         uint64                                    `json:"owner_charged_work_bytes"`
	FreshOwnerChargedLogicalBytes uint64                                    `json:"fresh_owner_charged_logical_bytes"`
	FreshOwnerChargedWorkBytes    uint64                                    `json:"fresh_owner_charged_work_bytes"`
	Primary                       scopeRehearsalUsage                       `json:"primary_actual_files"`
	Backup                        scopeRehearsalUsage                       `json:"backup_actual_files"`
	Journal                       scopeRehearsalUsage                       `json:"journal_actual_files"`
	PrimaryDataBytes              uint64                                    `json:"primary_actual_data_bytes"`
	BackupDataBytes               uint64                                    `json:"backup_actual_data_bytes"`
	FinalHead                     scopeRehearsalBlob                        `json:"final_head"`
	FinalReceipt                  string                                    `json:"final_receipt_id"`
	FinalReceiptSelector          string                                    `json:"final_receipt_selector"`
	FinalReceiptByteLength        uint64                                    `json:"final_receipt_byte_length"`
	FinalHeadSelector             string                                    `json:"final_head_selector"`
	Roots                         [3]publication.RetentionDirectoryIdentity `json:"roots"`
	Steps                         []scopeRehearsalStep                      `json:"steps"`
	Verification                  string                                    `json:"verification"`
	ProcessRestartTested          bool                                      `json:"process_restart_tested"`
	Failure                       string                                    `json:"failure,omitempty"`
}

// Pin enumeration handles against separately admitted publication capabilities.
// No ambient-path observation authorizes an effect. Subsequent owner acquisitions
// must match these identities; roots remain held until measurement/close completes.
type scopeRehearsalRoots struct {
	files [3]*os.File
	ids   [3]publication.RetentionDirectoryIdentity
}

func (p *scopeRehearsalRoots) close() (err error) {
	for i := 2; i >= 0; i-- {
		if p.files[i] != nil {
			err = errors.Join(err, p.files[i].Close())
			p.files[i] = nil
		}
	}
	return err
}
func pinScopeRehearsalRoots(ctx context.Context, paths [3]string) (p *scopeRehearsalRoots, err error) {
	p = &scopeRehearsalRoots{}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.close())
		}
	}()
	for i, path := range paths {
		r, e := publication.OpenContentRetentionRoot(ctx, path)
		if e != nil {
			return p, e
		}
		id, e := r.DirectoryIdentity()
		if e != nil {
			return p, errors.Join(e, r.Close())
		}
		fd, e := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return p, errors.Join(e, r.Close())
		}
		p.files[i] = os.NewFile(uintptr(fd), path)
		info, e := p.files[i].Stat()
		if e == nil {
			st := info.Sys().(*syscall.Stat_t)
			if !info.IsDir() || info.Mode().Perm() != 0700 || st.Uid != uint32(os.Getuid()) || id != (publication.RetentionDirectoryIdentity{Device: uint64(st.Dev), Inode: uint64(st.Ino)}) {
				e = ErrInvalid
			}
		}
		e = errors.Join(e, r.ValidatePrivate(), r.Close())
		if e != nil {
			return p, e
		}
		for j := 0; j < i; j++ {
			if p.ids[j] == id {
				return p, ErrInvalid
			}
		}
		p.ids[i] = id
	}
	return p, nil
}
func rehearsalNames(f *os.File) ([]string, error) {
	// Open a new description relative to the pinned descriptor; avoid shared offsets.
	fd, err := unix.Openat(int(f.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	d := os.NewFile(uintptr(fd), "rehearsal-enumeration")
	names, err := d.Readdirnames(65) // fixed finite directory bound, including lock/staging
	if err == io.EOF {
		err = nil
	}
	if len(names) > 64 {
		err = errors.Join(err, ErrInvalid)
	}
	return names, errors.Join(err, d.Close())
}
func rehearsalUsage(f *os.File) (u scopeRehearsalUsage, err error) {
	names, err := rehearsalNames(f)
	if err != nil {
		return u, err
	}
	for _, name := range names {
		var st unix.Stat_t
		if err = unix.Fstatat(int(f.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return u, err
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Size < 0 || st.Blocks < 0 {
			return u, ErrInvalid
		}
		u.Files++
		u.LogicalBytes, err = retentionAdd(u.LogicalBytes, uint64(st.Size))
		if err != nil {
			return u, err
		}
		allocated, e := retentionMultiply(uint64(st.Blocks), 512)
		if e != nil {
			return u, e
		}
		u.AllocatedBytes, err = retentionAdd(u.AllocatedBytes, allocated)
		if err != nil {
			return u, err
		}
	}
	return u, nil
}

// At entry only empty roots or this exact head are admissible. Changed source
// pins cannot silently select a new absent key in an already used journal.
func (p *scopeRehearsalRoots) admit(ctx context.Context, o *scopeBundleOwner) (existing bool, err error) {
	var names [3][]string
	for i, f := range p.files {
		names[i], err = rehearsalNames(f)
		if err != nil {
			return false, err
		}
	}
	if len(names[0])+len(names[1])+len(names[2]) == 0 {
		return false, nil
	}
	b := scopeBundleBinding{Scope: o.scopeDigest, Metadata: Digest(o.plan.state.metadata), Policy: scopeBundlePolicy, Roots: p.ids}
	key := scopeBundleKey(b)
	selector := "feature-scope-bundle-head-private-v1-" + key[7:] + ".json"
	found := false
	for _, n := range names[2] {
		if n == selector {
			found = true
		}
		if len(n) >= len("feature-scope-bundle-head-private-v1-") && n[:len("feature-scope-bundle-head-private-v1-")] == "feature-scope-bundle-head-private-v1-" && n != selector {
			return false, ErrInvalid
		}
	}
	if !found {
		return false, ErrInvalid
	}
	r, err := publication.OpenContentRetentionRoot(ctx, o.paths[2])
	if err != nil {
		return false, err
	}
	id, e := r.DirectoryIdentity()
	raw, re := r.ReadScopeBundleHead(ctx, key)
	err = errors.Join(e, re, r.Close())
	if err != nil {
		return false, err
	}
	h, e := parseScopeBundleHead(raw)
	if e != nil || id != p.ids[2] || h.Binding != b {
		return false, ErrInvalid
	}
	// A prior READY prefix is not authorization to execute a resumed run.
	if h.State == "READY" {
		return false, ErrInvalid
	}
	return true, nil
}

func scopeRehearsalLoop(ctx context.Context, o *scopeBundleOwner, report *scopeRehearsalReport) (scopeBundleResult, error) {
	var pending *scopeBundlePending
	var out scopeBundleResult
	for i := 0; i < 2*(len(o.plan.state.objects)+1); i++ {
		var err error
		out, err = o.advance(ctx, pending)
		report.Steps = append(report.Steps, rehearsalStep(out, err))
		if err != nil {
			return out, err
		}
		switch out.outcome {
		case "RETAINED_INPUT_BUNDLE":
			return out, nil
		case "CLAIMED":
			if out.pending == nil {
				return out, ErrInvalid
			}
			pending = out.pending
		case "READY":
			pending = nil
		default:
			return out, fmt.Errorf("rehearsal stopped: %s", out.outcome)
		}
	}
	return out, ErrInvalid
}

func runScopeRehearsal(ctx context.Context, opts scopeRehearsalOptions) (report scopeRehearsalReport, err error) {
	report.Schema = "lsp-trace.feature-scope-rehearsal.private.v1"
	report.Completeness = "UNKNOWN"
	report.FeatureIdentity = "UNRESOLVED"
	report.RevisionQualification = "CALLER_ASSERTED_NOT_AUTHENTICATED"
	report.Verification = "NOT_VERIFIED"
	defer func() {
		if err != nil {
			report.Failure = err.Error()
			report.Verification = "REFUSED_OR_INDETERMINATE"
		}
	}()
	scope, err := freezeFeatureScope(ctx, opts.source, opts.revision, nil)
	if err != nil {
		return report, err
	}
	report.Scope = rehearsalBlob(scope.ManifestBytes())
	report.Selection = rehearsalBlob(scope.SelectionBytes())
	report.Plan = rehearsalBlob(scope.PlanBytes())
	o, err := newScopeBundleOwner(scope, opts.roots[0], opts.roots[1], opts.roots[2])
	if err != nil {
		return report, err
	}
	for _, f := range scope.Files() {
		switch f.Role {
		case "NOMINATION":
			report.Nomination++
		case "CONTEXT_ONLY":
			report.Context++
		}
	}
	report.References = len(o.plan.state.references)
	report.UniqueObjects = len(o.plan.state.objects)
	for _, r := range o.plan.state.references {
		report.ReferencePayloadBytes += r.ByteLength
	}
	for _, obj := range o.plan.state.objects {
		report.UniquePayloadBytes += uint64(len(obj.raw))
	}
	report.MetadataBytesPerCopy = uint64(len(o.plan.state.metadata))
	report.ReservedLogicalBytes = o.budget.bytes
	report.ReservedWorkBytes = o.budget.work
	defer func() {
		report.OwnerChargedLogicalBytes = o.charged.bytes
		report.OwnerChargedWorkBytes = o.charged.work
	}()
	pins, err := pinScopeRehearsalRoots(ctx, opts.roots)
	if err != nil {
		return report, err
	}
	report.Roots = pins.ids
	defer func() {
		targets := []*scopeRehearsalUsage{&report.Primary, &report.Backup, &report.Journal}
		for i, f := range pins.files {
			u, e := rehearsalUsage(f)
			*targets[i] = u
			err = errors.Join(err, e)
		}
		if report.Verification == "FRESH_OWNER_TERMINAL_VERIFIED_NOT_PROCESS_RESTART" {
			per := report.UniquePayloadBytes + report.MetadataBytesPerCopy
			if report.Primary.LogicalBytes != per || report.Backup.LogicalBytes != per {
				err = errors.Join(err, ErrInvalid)
			} else {
				report.PrimaryDataBytes = report.Primary.LogicalBytes - report.MetadataBytesPerCopy
				report.BackupDataBytes = report.Backup.LogicalBytes - report.MetadataBytesPerCopy
			}
		}
		err = errors.Join(err, pins.close())
	}()
	existing, err := pins.admit(ctx, o)
	if err != nil {
		return report, err
	}
	pinOwner := func(owner *scopeBundleOwner) {
		b := scopeBundleBinding{Scope: owner.scopeDigest, Metadata: Digest(owner.plan.state.metadata), Policy: scopeBundlePolicy, Roots: pins.ids}
		owner.binding = &b
	}
	pinOwner(o)
	if existing {
		// Existing state is observation-only, including abandoned claims and failures.
		o.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
			return nil, ErrInvalid
		}
	}
	out, err := scopeRehearsalLoop(ctx, o, &report)
	if err != nil {
		return report, err
	}
	fresh, err := newScopeBundleOwner(scope, opts.roots[0], opts.roots[1], opts.roots[2])
	if err != nil {
		return report, err
	}
	pinOwner(fresh)
	fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
		return nil, ErrInvalid
	}
	verified, e := fresh.advance(ctx, nil)
	report.FreshOwnerChargedLogicalBytes = fresh.charged.bytes
	report.FreshOwnerChargedWorkBytes = fresh.charged.work
	report.Steps = append(report.Steps, rehearsalStep(verified, e))
	if e != nil {
		return report, e
	}
	if verified.outcome != "RETAINED_INPUT_BUNDLE" || verified.casCommitted || string(verified.head) != string(out.head) {
		return report, ErrInvalid
	}
	h, e := parseScopeBundleHead(verified.head)
	if e != nil {
		return report, e
	}
	report.FinalHead = rehearsalBlob(verified.head)
	report.FinalReceipt = h.Receipt
	report.FinalReceiptSelector = scopeBundleReceiptSelector(h.Receipt)
	report.FinalHeadSelector = "feature-scope-bundle-head-private-v1-" + h.Key[7:] + ".json"
	for _, raw := range verified.linked {
		if Digest(raw) == h.Receipt {
			report.FinalReceiptByteLength = uint64(len(raw))
		}
	}
	if report.FinalReceiptByteLength == 0 {
		return report, ErrInvalid
	}
	// Require exact finalized namespaces, not a plausible budget or file count.
	for i, f := range pins.files {
		expected := map[string]bool{}
		if i < 2 {
			for _, obj := range o.plan.state.objects {
				expected[obj.digest] = true
			}
			expected[Digest(o.plan.state.metadata)] = true
		} else {
			expected[".lsp-trace-candidate-publication.lock"] = true
			expected["feature-scope-bundle-head-private-v1-"+h.Key[7:]+".json"] = true
			for _, raw := range verified.linked {
				expected[scopeBundleReceiptSelector(Digest(raw))] = true
			}
		}
		names, e := rehearsalNames(f)
		if e != nil {
			return report, e
		}
		if len(names) != len(expected) {
			return report, ErrInvalid
		}
		for _, n := range names {
			if !expected[n] {
				return report, ErrInvalid
			}
		}
	}
	report.Verification = "FRESH_OWNER_TERMINAL_VERIFIED_NOT_PROCESS_RESTART"
	return report, nil
}
