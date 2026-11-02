//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"os"
	"path/filepath"
	"time"
)

// A parent-only immutable transition after native Requested is observed and the
// exact pre-authorized source derivation is checked. This DTO is not a source owner.
type applicationUnitDerivation struct {
	Binding UnitBinding
	Source  Config
	Cohort  CohortConfig
}

func (a *ApplicationJournal) BindUnitResultConfiguration(u UnitBinding, source Config, cohort CohortConfig) error {
	j := a.implementation
	if e := j.attempt("BIND_UNIT_RESULT", time.Now()); e != nil {
		return e
	}
	var admitted applicationUnitLedger
	if j.object("unit", u.UnitID, &admitted) != nil || !bytes.Equal(mustJSON(admitted.Binding), mustJSON(u)) || !validApplicationUnit(j.config, u) || u.RevisionSHA256 != hash([]byte(j.config.RevisionAssertion)) || ValidateConfig(source) != nil || ValidateCohortConfig(cohort) != nil {
		return ErrInvalid
	}
	root := filepath.Join(j.config.Store, "units", u.UnitID[7:])
	if source.Store != filepath.Join(root, "original") || cohort.Store != filepath.Join(root, "cohort") || source.ScopeSHA256 != u.ScopeSHA256 || cohort.ScopeSHA256 != u.ScopeSHA256 || cohort.SourceConfigSHA256 != ConfigDigest(source) || cohort.Requested != source.Requested || cohort.RolePromptSHA256 != j.config.RolePromptSHA256 {
		return ErrInvalid
	}
	if !applicationMatchesDerivation(admitted.Contract, source, cohort) {
		return ErrInvalid
	}
	d := applicationUnitDerivation{u, source, cohort}
	return j.commit("BIND_UNIT_RESULT", u.UnitID, j.head, applicationObject{applicationObjectName("derivation", u.UnitID), mustJSON(d)})
}

func applicationUnitDeadline(now time.Time, ns uint64, expires int64) int64 {
	if ns > uint64(240*time.Second) || now.UnixNano() > int64(^uint64(0)>>1)-int64(ns) {
		return 0
	}
	d := now.UnixNano() + int64(ns)
	if d > expires {
		d = expires
	}
	return d
}

// A child reservation covers the whole bounded orchestration interval, including
// concurrent owned HOST polls. This is reserved wall-window time, NOT summed CPU
// time or parent reservation plus measured child spend. Read-only data conveys
// neither a new semantic grant nor an original source capability.
func ReadApplicationUnitDeadline(root, app, id string) (time.Time, error) {
	return readApplicationUnitDeadline(root, app, id, applicationClockNow)
}
func readApplicationUnitDeadline(root, app, id string, clock func() (applicationClockStamp, error)) (time.Time, error) {
	if privateDir(root) != nil || !digestValue(id) || !digestValue(app) {
		return time.Time{}, ErrInvalid
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return time.Time{}, e
	}
	defer r.Close()
	raw, e := (&store{root: r}).read(applicationObjectName("unit", id), ApplicationRecordLimit)
	var unit applicationUnitLedger
	if e != nil || strict(raw, &unit) != nil || unit.Binding.ApplicationSHA256 != app || unit.Binding.UnitID != id || unit.ActiveDeadlineUnixNano <= 0 || unit.Allowance.ActiveNS == 0 || unit.Allowance.ActiveNS > uint64(240*time.Second) {
		return time.Time{}, ErrInvalid
	}
	observed := time.Now()
	stamp, e := clock()
	if e != nil {
		return time.Time{}, e
	}
	remaining, e := applicationIntervalRemaining(unit, stamp)
	if e != nil {
		return time.Time{}, e
	}
	// Add on a live time.Time preserves the process monotonic watchdog.
	return observed.Add(remaining), nil
}

func ReadApplicationOperation(root, id string, out *OperationReservation) error {
	if privateDir(root) != nil || !digestValue(id) {
		return ErrInvalid
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer r.Close()
	raw, e := (&store{root: r}).read(applicationObjectName("claim", id), ApplicationRecordLimit)
	if e != nil || strict(raw, out) != nil || out.EventID != id {
		return ErrInvalid
	}
	return nil
}

// Read-only consistency with the parent's actual full-key prepaid record, not
// agreement between two caller-controlled copies. No ledger/capture is created.
func VerifyApplicationUnitPrepayment(root string, u UnitBinding, b ApplicationBounds, source ...Config) error {
	if privateDir(root) != nil || !digestValue(u.UnitID) {
		return ErrInvalid
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer r.Close()
	s := &store{root: r}
	raw, e := s.read(applicationObjectName("unit", u.UnitID), ApplicationRecordLimit)
	var admitted applicationUnitLedger
	if e != nil || strict(raw, &admitted) != nil || !bytes.Equal(mustJSON(admitted.Binding), mustJSON(u)) || admitted.Allowance != b {
		return ErrInvalid
	}
	if len(source) > 1 {
		return ErrInvalid
	}
	if len(source) == 1 {
		candidate := source[0]
		if candidate.Requested != 0 || admitted.Contract == nil {
			return ErrInvalid
		}
		candidate.ApprovalSHA256 = ""
		if !bytes.Equal(mustJSON(candidate), mustJSON(admitted.Contract.Source)) {
			return ErrInvalid
		}
	}
	deadline, e := ReadApplicationUnitDeadline(root, u.ApplicationSHA256, u.UnitID)
	if e != nil || !time.Now().Before(deadline) {
		return ErrInvalid
	}
	return nil
}

func verifyApplicationRefBinding(ref UnitResultRef, supplied *UnitBinding) error {
	for _, d := range []string{ref.ApplicationSHA256, ref.UnitID, ref.InputSHA256, ref.RevisionSHA256, ref.SourceConfigSHA256, ref.CohortConfigSHA256} {
		if !digestValue(d) {
			return ErrInvalid
		}
	}
	if !applicationCanonicalPath(ref.Store) {
		return ErrInvalid
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(ref.Store)))
	if ref.Store != filepath.Join(root, "units", ref.UnitID[7:], "cohort") || privateDir(root) != nil {
		return ErrInvalid
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer r.Close()
	s := &store{root: r}
	raw, e := s.read("header.json", ApplicationRecordLimit)
	var config ApplicationConfig
	if e != nil || strict(raw, &config) != nil || config.Store != root || config.ApprovalSHA256 != ApplicationConfigDigest(config) || ref.ApplicationSHA256 != config.ApprovalSHA256 {
		return ErrInvalid
	}
	raw, e = s.read(applicationObjectName("unit", ref.UnitID), ApplicationRecordLimit)
	var unit applicationUnitLedger
	if e != nil || strict(raw, &unit) != nil || !validApplicationUnit(config, unit.Binding) {
		return ErrInvalid
	}
	u := unit.Binding
	if u.UnitID != ref.UnitID || u.InputSHA256 != ref.InputSHA256 || u.RevisionSHA256 != ref.RevisionSHA256 || u.RevisionSHA256 != hash([]byte(config.RevisionAssertion)) || supplied != nil && !bytes.Equal(mustJSON(u), mustJSON(*supplied)) {
		return ErrInvalid
	}
	raw, e = s.read(applicationObjectName("derivation", ref.UnitID), ApplicationRecordLimit)
	var derived applicationUnitDerivation
	if e != nil || strict(raw, &derived) != nil || !bytes.Equal(mustJSON(derived.Binding), mustJSON(u)) || ValidateConfig(derived.Source) != nil || ValidateCohortConfig(derived.Cohort) != nil || !applicationMatchesDerivation(unit.Contract, derived.Source, derived.Cohort) {
		return ErrInvalid
	}
	if derived.Source.Store != filepath.Join(root, "units", u.UnitID[7:], "original") || derived.Source.ScopeSHA256 != u.ScopeSHA256 || derived.Source.Requested != ref.Requested || ConfigDigest(derived.Source) != ref.SourceConfigSHA256 || derived.Cohort.Store != ref.Store || derived.Cohort.ScopeSHA256 != u.ScopeSHA256 || derived.Cohort.Requested != ref.Requested || derived.Cohort.SourceConfigSHA256 != ref.SourceConfigSHA256 || !bytes.Equal(mustJSON(derived.Cohort), mustJSON(ref.Configuration)) || CohortConfigDigest(derived.Cohort) != ref.CohortConfigSHA256 {
		return ErrInvalid
	}
	return nil
}
