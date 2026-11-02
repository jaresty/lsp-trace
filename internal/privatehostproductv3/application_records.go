//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Hash-addressed immutable application objects. These APIs never enumerate a
// store or materialize a previous-page prefix. Reads select one bounded object.
func (a *ApplicationJournal) PutRecord(v any) (string, error) {
	j := a.implementation
	if e := j.attempt("PUT_RECORD", time.Now()); e != nil {
		return "", e
	}
	raw, e := applicationMarshal(v, ApplicationRecordLimit)
	if e != nil {
		return "", e
	}
	id := hash(raw)
	if j.exists("data", id) {
		old, e := j.read(applicationObjectName("data", id), ApplicationRecordLimit)
		if e != nil || !bytes.Equal(old, raw) {
			return "", ErrInvalid
		}
		return id, nil
	}
	e = j.commit("IMMUTABLE_RECORD", id, j.head, applicationObject{applicationObjectName("data", id), raw})
	return id, e
}
func (a *ApplicationJournal) GetRecord(id string, v any) error {
	if !digestValue(id) {
		return ErrInvalid
	}
	j := a.implementation
	if e := j.attempt("GET_RECORD", time.Now()); e != nil {
		return e
	}
	raw, e := j.read(applicationObjectName("data", id), ApplicationRecordLimit)
	if e != nil || hash(raw) != id {
		return ErrInvalid
	}
	return strict(raw, v)
}
func (a *ApplicationJournal) CommitCursor(id, publication string) error {
	if !digestValue(id) || publication != "" && !digestValue(publication) {
		return ErrInvalid
	}
	j := a.implementation
	if e := j.attempt("CURSOR", time.Now()); e != nil {
		return e
	}
	h := j.head
	h.CursorSHA256 = id
	if publication != "" {
		if _, e := a.VerifyInventory(publication); e != nil {
			return e
		}
		h = j.head
		h.CursorSHA256 = id
		h.LastCompletePublicationSHA256 = publication
	}
	return j.commit("CURSOR", id, h)
}
func (a *ApplicationJournal) WriteActivation(n int, v any) error {
	if n < 1 || n > 16 {
		return ErrInvalid
	}
	j := a.implementation
	if e := j.attempt("ACTIVATION", time.Now()); e != nil {
		return e
	}
	return j.write(fmt.Sprintf("activation-%02d.json", n), mustJSON(v))
}

// Read-only access is for a parent-prepaid original process or HOST role. It is
// local bounded observation, not a constructor for original/retained owners.
func ReadApplicationRecord(root, id string, v any) error {
	if privateDir(root) != nil || !digestValue(id) {
		return ErrInvalid
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer r.Close()
	raw, e := (&store{root: r}).read(applicationObjectName("data", id), ApplicationRecordLimit)
	if e != nil || hash(raw) != id {
		return ErrInvalid
	}
	return strict(raw, v)
}

type ApplicationCohortBackend struct {
	Journal          *ApplicationJournal
	UnitID           string
	Mailbox          CohortMailbox
	MaxResponseBytes uint64
}

func (b ApplicationCohortBackend) cohortRecoveryMailbox() (CohortMailbox, bool) {
	return b.Mailbox, true
}
func (b ApplicationCohortBackend) CohortLifecycle() Lifecycle { return b.Mailbox.CohortLifecycle() }
func (b ApplicationCohortBackend) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	h, e := parseCohortFrame(raw)
	if e != nil {
		return nil, e
	}
	if b.MaxResponseBytes < 1 || b.MaxResponseBytes > 64<<10 {
		return nil, ErrInvalid
	}
	bounds := ApplicationBounds{Calls: 1, RequestBytes: uint64(len(raw)), ResponseBytes: b.MaxResponseBytes, WireBytes: uint64(len(raw)) + b.MaxResponseBytes}
	r, e := b.Journal.ReserveExchange(b.UnitID, uint64(h.Sequence), h.RequestDigest, bounds, time.Now())
	if e != nil {
		return nil, e
	}
	// ctx retains the concrete pin supplied by RunCohort. Do not reopen/replace it.
	started := time.Now()
	reply, exchangeErr := b.Mailbox.ExchangeCohort(ctx, raw)
	if len(reply) > 0 {
		e = b.Journal.RecordReturnedBytes(r.EventID, ObservedWork{EventID: r.EventID, RequestBytes: uint64(len(raw)), ResponseBytes: uint64(len(reply)), ResponseSHA256: hash(reply), ElapsedNS: uint64(time.Since(started)), Accounting: "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES"})
		if e != nil {
			return reply, e
		}
	}
	return reply, exchangeErr
}

type UnitResultRef struct {
	ApplicationSHA256, UnitID, InputSHA256, RevisionSHA256, SourceConfigSHA256, CohortConfigSHA256 string
	Store                                                                                          string
	MachinePaths, HumanPaths                                                                       [2]string
	Digests                                                                                        [4]string
	Requested, Reviewed                                                                            int
	Configuration                                                                                  CohortConfig
}
type ApplicationSourceGap struct{ UnitID, InputSHA256, RevisionSHA256, Path, Reason string }
type ApplicationInventoryPage struct {
	Schema, ApplicationSHA256, Previous                   string
	Rows                                                  []UnitResultRef
	Gaps                                                  []ApplicationSourceGap
	NavigationStatus                                      string
	Authority                                             uint8
	Accepted                                              bool
	FeatureIdentity, SourceCompleteness, AbsenceInference string
}

func BuildUnitResultRef(app string, u UnitBinding, c CohortConfig, r CohortReport) (UnitResultRef, error) {
	ref := UnitResultRef{ApplicationSHA256: app, UnitID: u.UnitID, InputSHA256: u.InputSHA256, RevisionSHA256: u.RevisionSHA256, SourceConfigSHA256: c.SourceConfigSHA256, CohortConfigSHA256: CohortConfigDigest(c), Store: c.Store, Configuration: c, MachinePaths: r.MachinePaths, HumanPaths: r.HumanPaths, Requested: r.Requested, Reviewed: r.Reviewed}
	if verifyApplicationRefBinding(ref, &u) != nil || !applicationCanonicalQuartet(r) || app != u.ApplicationSHA256 || r.AccountingValidation != "EXACT" || r.ConfigSHA256 != ref.CohortConfigSHA256 || r.Inflight {
		return ref, ErrInvalid
	}
	s, e := openCohortStore(c)
	if e != nil {
		return ref, e
	}
	defer s.close()
	if verifyCohortQuartet(s, r) != nil || verifyCohortHistory(s, c, r) != nil {
		return ref, ErrInvalid
	}
	for n, p := range []string{r.MachinePaths[0], r.MachinePaths[1], r.HumanPaths[0], r.HumanPaths[1]} {
		raw, e := s.read(p, stateLimit)
		if e != nil {
			return ref, e
		}
		ref.Digests[n] = hash(raw)
	}
	return ref, nil
}
func applicationCanonicalQuartet(r CohortReport) bool {
	var first int
	if _, e := fmt.Sscanf(r.MachinePaths[0], "result/report-%08d.json", &first); e != nil || first < 1 || first > 99999997 || first%2 != 1 {
		return false
	}
	machine, human := reportPaths(first)
	return machine == r.MachinePaths && human == r.HumanPaths
}

func VerifyUnitResultRef(ref UnitResultRef) error {
	if verifyApplicationRefBinding(ref, nil) != nil || privateDir(ref.Store) != nil || privateDir(filepath.Join(ref.Store, "result")) != nil || ValidateCohortConfig(ref.Configuration) != nil || CohortConfigDigest(ref.Configuration) != ref.CohortConfigSHA256 || ref.SourceConfigSHA256 != ref.Configuration.SourceConfigSHA256 || ref.Configuration.Store != ref.Store {
		return ErrInvalid
	}
	root, e := os.OpenRoot(ref.Store)
	if e != nil {
		return e
	}
	defer root.Close()
	s := &store{root: root}
	var report CohortReport
	for n, p := range []string{ref.MachinePaths[0], ref.MachinePaths[1], ref.HumanPaths[0], ref.HumanPaths[1]} {
		raw, e := s.read(p, stateLimit)
		if e != nil || hash(raw) != ref.Digests[n] {
			return ErrInvalid
		}
		if n == 0 && strict(raw, &report) != nil {
			return ErrInvalid
		}
	}
	if !applicationCanonicalQuartet(report) || report.MachinePaths != ref.MachinePaths || report.HumanPaths != ref.HumanPaths || report.Inflight || report.ConfigSHA256 != ref.CohortConfigSHA256 || report.Requested != ref.Requested || report.Reviewed != ref.Reviewed || report.AccountingValidation != "EXACT" {
		return ErrInvalid
	}
	if verifyCohortHistory(s, ref.Configuration, report) != nil {
		return ErrInvalid
	}
	return verifyCohortQuartet(s, report)
}
