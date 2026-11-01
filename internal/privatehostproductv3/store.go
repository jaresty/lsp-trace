//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
)

const stateLimit = 4 << 20 // Explicit remaining application-manifest/state bound.
var numbered = regexp.MustCompile(`^(config|state|claim|response|report|inventory)-[0-9]{8}\.(json|md)$`)

type store struct {
	root                *os.Root
	lock                *os.File
	stateSeq, resultSeq int
	rejectedPrior       bool
	visibleObserver     func() // private test observer; cannot supply bytes or owners
}

func privateDir(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	real, e := filepath.EvalSymlinks(path)
	if e != nil || real != path {
		return ErrInvalid
	}
	info, e := os.Lstat(path)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return ErrInvalid
	}
	return nil
}

// Opening actually scans the production store BEFORE admission. Foreign scope,
// overlays and proof files are not tolerated in this closed normative store.
func openStore(c Config) (*store, error) {
	if e := os.Mkdir(c.Store, 0700); e != nil && !os.IsExist(e) {
		return nil, e
	}
	if e := privateDir(c.Store); e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(c.Store)
	if e != nil {
		return nil, e
	}
	s := &store{root: root}
	fail := func(e error) (*store, error) { s.close(); return nil, e }
	entries, e := os.ReadDir(c.Store)
	if e != nil {
		return fail(e)
	}
	allowed := map[string]bool{"plan": true, "pages": true, "claim": true, "result": true, "host": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return fail(ErrInvalid)
		}
	}
	for _, dir := range []string{"plan", "pages", "claim", "result", "host"} {
		if e = root.Mkdir(dir, 0700); e != nil && !os.IsExist(e) {
			return fail(e)
		}
		if e = privateDir(filepath.Join(c.Store, dir)); e != nil {
			return fail(e)
		}
		children, e := os.ReadDir(filepath.Join(c.Store, dir))
		if e != nil {
			return fail(e)
		}
		for _, child := range children {
			name := child.Name()
			// lock is fixed canonical plan metadata, not a caller-selected path.
			valid := numbered.MatchString(name) || (dir == "plan" && name == "lock") || (dir == "result" && (name == "report.json" || name == "inventory.md"))
			if !valid || !child.Type().IsRegular() {
				return fail(ErrInvalid)
			}
			f, e := root.OpenFile(dir+"/"+name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if e != nil {
				return fail(e)
			}
			st, e := f.Stat()
			f.Close()
			if e != nil || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 || st.Size() > stateLimit {
				return fail(ErrInvalid)
			}
		}
	}
	s.lock, e = root.OpenFile("plan/lock", os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return fail(e)
	}
	if e = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fail(ErrInvalid)
	}
	return s, nil
}
func (s *store) close() {
	if s.lock != nil {
		syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		s.lock.Close()
	}
	if s.root != nil {
		s.root.Close()
	}
}
func (s *store) write(name string, b []byte) error {
	if len(b) > stateLimit {
		return ErrPaused
	}
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return e
	}
	stage := filepath.Join(filepath.Dir(name), ".pending-"+hex.EncodeToString(nonce[:]))
	f, e := s.root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	defer s.root.Remove(stage)
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return e
	}
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	// Link is atomic and refuses existing destination; unlike Rename it cannot
	// replace an old claim/report. Readers take a shared nonblocking lock and
	// cannot admit the brief two-link or unsynced-publication interval.
	if e = s.root.Link(stage, name); e != nil {
		return e
	}
	if s.visibleObserver != nil {
		s.visibleObserver()
	}
	if e = s.root.Remove(stage); e != nil {
		return e
	}
	d, e := s.root.Open(filepath.Dir(name))
	if e != nil {
		return e
	}
	return errors.Join(d.Sync(), d.Close())
}
func (s *store) read(name string, limit int) ([]byte, error) {
	f, e := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); e != nil {
		if errors.Is(e, syscall.EWOULDBLOCK) {
			return nil, os.ErrNotExist
		}
		return nil, ErrInvalid
	}
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() > int64(limit) || st.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(b) > limit {
		return nil, ErrInvalid
	}
	return b, e
}
func strict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrInvalid
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, b) {
		return ErrInvalid
	}
	return nil
}
func (s *store) load(c Config) (progress, error) {
	p := progress{Config: c, Candidates: []Candidate{}, Observations: []preparedObservation{}}
	dir, e := s.root.Open("pages")
	if e != nil {
		return p, e
	}
	names, e := dir.Readdirnames(-1)
	dir.Close()
	if e != nil {
		return p, e
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasPrefix(name, "state-") {
			return p, ErrInvalid
		}
		s.stateSeq++
	}
	if s.stateSeq > 0 {
		b, e := s.read(fmt.Sprintf("pages/state-%08d.json", s.stateSeq), stateLimit)
		if e != nil || strict(b, &p) != nil {
			return p, ErrInvalid
		}
		if ConfigDigest(c) != ConfigDigest(p.Config) {
			old := p.Config
			baseOld, baseNew := old, c
			baseOld.ApprovalSHA256 = ""
			baseNew.ApprovalSHA256 = ""
			baseOld.PreviousConfigSHA256 = ""
			baseNew.PreviousConfigSHA256 = ""
			baseOld.AllowanceRevision = 0
			baseNew.AllowanceRevision = 0
			baseOld.StartBatch = 0
			baseNew.StartBatch = 0
			baseOld.MaxCalls = 0
			baseNew.MaxCalls = 0
			baseOld.MaxTotalBytes = 0
			baseNew.MaxTotalBytes = 0
			if p.LastStatus != "PAUSED_BUDGET" || c.AllowanceRevision != old.AllowanceRevision+1 || c.PreviousConfigSHA256 != ConfigDigest(old) || c.MaxCalls < old.MaxCalls || c.MaxTotalBytes < old.MaxTotalBytes || !bytes.Equal(mustJSON(baseOld), mustJSON(baseNew)) {
				return p, ErrInvalid
			}
		}
		// A pending durable exchange is spent; do not automatically infer a response
		// or re-disclose the same item after process/host cancellation.
		if p.Calls > 0 {
			b, e := s.read(fmt.Sprintf("host/response-%08d.json", p.Calls), 64<<10)
			if e != nil || len(b) == 0 {
				p.Inflight = true
			}
		}
	}
	configName := fmt.Sprintf("plan/config-%08d.json", c.AllowanceRevision)
	existing, e := s.read(configName, stateLimit)
	if os.IsNotExist(e) {
		if e = s.write(configName, mustJSON(c)); e != nil {
			return p, e
		}
	} else if e != nil || !bytes.Equal(existing, mustJSON(c)) {
		return p, ErrInvalid
	}
	p.Config = c
	return p, nil
}
func (s *store) save(p progress) error {
	if !admitProgress(p) {
		return &budgetBoundary{Resource: "application_progress_field_graph", Limit: stateLimit}
	}
	s.stateSeq++
	return s.write(fmt.Sprintf("pages/state-%08d.json", s.stateSeq), mustJSON(p))
}
func (s *store) claim(t Task) error {
	return s.write(fmt.Sprintf("claim/claim-%08d.json", t.Sequence), mustJSON(t))
}
func (s *store) response(t Task, r Response) error {
	return s.write(fmt.Sprintf("host/response-%08d.json", t.Sequence), mustJSON(r))
}
func human(r Report) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Provisional inventory — %s\n\nBounded own-source observations, not accepted features. Product purpose remains unresolved.\n\nCoverage: %d requested; %d candidates; %d gaps; %d pending.\n\n", r.Status, r.Requested, r.Completed, r.Gaps, r.Pending)
	clean := func(s string) string {
		return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ", "[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)", "`", "\\`", "*", "\\*", "_", "\\_").Replace(html.EscapeString(s))
	}
	if a := r.Admission; a != nil {
		fmt.Fprintf(&b, "Current admission: %s. Current ledger validation: %s. Current source validation: %s. Coverage/counts: %s / %s. Ledger values: %s. Refusal resource counts: %s. Prior rows are interpretations from an older revision, not current-source AST ownership.\n\n", clean(a.CurrentValidation), clean(a.LedgerValidation), clean(a.SourceValidation), clean(a.Coverage), clean(a.Counts), clean(a.LedgerObservation), clean(a.ResourceCounts))
		if a.UnadmittedPublications {
			b.WriteString("One or more numbered publications were not admitted: incomplete, unreadable, or inconsistent views. Their progress is UNKNOWN; they remain immutable and supply no carried candidates, counts, ledger values, or view paths.\n\n")
		}
		if a.PriorPublicationValidation != "" {
			fmt.Fprintf(&b, "Prior publication validation: %s. Rejected later publications, if any, remain immutable UNKNOWN observations; local consistency is not producer authentication.\n\n", clean(a.PriorPublicationValidation))
		}
		if a.PriorMachinePaths[0] != "" || a.PriorPublicationValidation == "" {
			fmt.Fprintf(&b, "Prior immutable machine views: %s; %s. Prior immutable human views: %s; %s.\n\n", clean(a.PriorMachinePaths[0]), clean(a.PriorMachinePaths[1]), clean(a.PriorHumanPaths[0]), clean(a.PriorHumanPaths[1]))
		}
	}
	if r.Failure != nil {
		fmt.Fprintf(&b, "Failure phase/class: %s / %s. Resource: %s; exact original error: %s.\n\n", clean(r.Failure.Phase), clean(r.Failure.Class), clean(r.Failure.Resource), clean(r.Failure.ExactOriginalError))
	}
	b.WriteString("ID | Working label | Status | Evidence | Alternatives | Gaps | Revision\n--- | --- | --- | --- | --- | --- | ---\n")
	for _, c := range r.Candidates {
		fmt.Fprintf(&b, "%s | %s: %s | %s | %s | %s | %s | %d\n", c.ID, clean(c.WorkingLabel), clean(c.Explanation), c.Status, fmt.Sprintf("%d typed refs; see machine row", len(c.EvidenceRefs)), clean(strings.Join(c.Alternatives, "; ")), clean(strings.Join(c.Gaps, "; ")), c.Revision)
	}
	if len(r.Candidates) == 0 {
		b.WriteString("\nNo provisional leads delivered; this does not establish absence of features.\n")
	}
	fmt.Fprintf(&b, "\nUnassigned candidates: %d. No navigation grouping asserts canonical identity.\n\nSpend: %d physical semantic calls, %d request bytes, %d response bytes. Policy reservations are not measured memory/CPU or model billing.\nExcluded accounting: %s.\n\nCheckpoint: `%s`\n", len(r.Unassigned), r.Spend.Calls, r.Spend.RequestBytes, r.Spend.ResponseBytes, strings.Join(r.Spend.Excluded, "; "), r.CheckpointSHA256)
	return []byte(b.String())
}
func (s *store) deliver(r *Report) (err error) {
	defer func() {
		if err != nil {
			r.MachinePaths = [2]string{}
			r.HumanPaths = [2]string{}
		}
	}()
	// This family has no feature-acceptance operation, even for resumed DTOs.
	r.Schema = "private-host-product-report.v3.1"
	r.Profile = Profile
	r.Authority = 0
	r.Accepted = false
	r.FeatureIdentity = "UNRESOLVED"
	r.SourceCompleteness = "UNKNOWN"
	r.AbsenceInference = "PROHIBITED"
	for i := range r.Candidates {
		r.Candidates[i].Status = "PROVISIONAL"
	}
	// Each current terminal response explicitly names its immutable view paths;
	// report.json/inventory.md remain the initial historical revision only.
	numbers, e := s.resultNumbers()
	if e != nil {
		return e
	}
	first := 1
	if len(numbers) > 0 {
		first = numbers[0] + 2
	}
	if first > 99999997 {
		return ErrInvalid
	}
	r.MachinePaths, r.HumanPaths = reportPaths(first)
	r.ReportRevision = (first + 1) / 2
	if !admitReport(*r) {
		return &budgetBoundary{Resource: "application_report_field_graph", Limit: stateLimit}
	}
	// Two no-replace physical copies of each view; readback verifies exact bytes,
	// distinct inode, regular/private/single-link identity. Later revisions never
	// erase earlier useful candidates or reports.
	for _, v := range []struct {
		kind, ext string
		raw       []byte
	}{{"report", "json", mustJSON(r)}, {"inventory", "md", human(*r)}} {
		names := r.MachinePaths
		if v.kind == "inventory" {
			names = r.HumanPaths
		}
		for _, name := range names {
			if e := s.write(name, v.raw); e != nil {
				// A partial quartet is retained, but never advertised as paired.
				r.MachinePaths = [2]string{}
				r.HumanPaths = [2]string{}
				return e
			}
		}
		a, e := s.root.Stat(names[0])
		if e != nil {
			return e
		}
		b, e := s.root.Stat(names[1])
		if e != nil || os.SameFile(a, b) {
			return ErrInvalid
		}
		for _, name := range names {
			b, e := s.read(name, stateLimit)
			if e != nil || !bytes.Equal(b, v.raw) {
				return ErrInvalid
			}
		}
		stable := "result/" + v.kind + "." + v.ext
		if _, e := s.root.Lstat(stable); os.IsNotExist(e) {
			if e = s.write(stable, v.raw); e != nil {
				return e
			}
		}
	}
	return nil
}
