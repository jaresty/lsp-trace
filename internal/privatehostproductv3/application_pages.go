//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ApplicationCheckpoint struct {
	Schema, ApplicationSHA256, PageSHA256, Previous string
	MachinePaths, HumanPaths                        [2]string
	Digests                                         [4]string
}

func (a *ApplicationJournal) PublishInventory(page ApplicationInventoryPage, cursor any) (string, error) {
	if page.Schema != "private-application-inventory-page.v1" || page.ApplicationSHA256 != a.Head().ApplicationSHA256 || len(page.Rows) > 8 || len(page.Gaps) > 8 {
		return "", ErrInvalid
	}
	j := a.implementation
	if e := j.charge("PUBLICATION_ENVELOPE", ApplicationBounds{ReadHashBytes: uint64(len(page.Rows)) * 64 << 20, ParserBytes: uint64(len(page.Rows)) * 64 << 20, WriteBytes: 8 * ApplicationRecordLimit, Records: 128, Work: 1024}, time.Now()); e != nil {
		return "", e
	}
	for _, gap := range page.Gaps {
		if !digestValue(gap.UnitID) || !digestValue(gap.InputSHA256) || !digestValue(gap.RevisionSHA256) || len(gap.Path) > 4096 || gap.Reason == "" || len(gap.Reason) > 128 {
			return "", ErrInvalid
		}
	}
	for _, ref := range page.Rows {
		if !digestValue(ref.UnitID) || ref.Store != filepath.Join(j.config.Store, "units", ref.UnitID[7:], "cohort") || ref.ApplicationSHA256 != page.ApplicationSHA256 || VerifyUnitResultRef(ref) != nil {
			return "", ErrInvalid
		}
	}
	page.Authority = 0
	page.Accepted = false
	page.FeatureIdentity = "UNRESOLVED"
	page.SourceCompleteness = "UNKNOWN"
	page.AbsenceInference = "PROHIBITED"
	page.NavigationStatus = "UNASSIGNED_NAVIGATION_BACKLOG_NOT_COMPUTED"
	raw, encodingErr := applicationMarshal(page, ApplicationRecordLimit)
	if encodingErr != nil {
		return "", encodingErr
	}
	if len(raw) > ApplicationRecordLimit {
		return "", ErrInvalid
	}
	id := hash(raw)
	human := applicationInventoryHuman(raw)
	if len(page.Rows) == 0 {
		if e := j.charge("SOURCE_GAP_OUTPUT", ApplicationBounds{OutputBytes: uint64(len(raw) + len(human))}, time.Now()); e != nil {
			return "", e
		}
	}
	if len(raw)+len(human) > 32<<10 {
		return "", ErrInvalid
	}
	if e := j.attempt("PUBLICATION", time.Now()); e != nil {
		return "", e
	}
	cp := ApplicationCheckpoint{Schema: "private-application-checkpoint.v1", ApplicationSHA256: page.ApplicationSHA256, PageSHA256: id, Previous: page.Previous}
	objects := []applicationObject{{applicationObjectName("data", id), raw}}
	for n := 0; n < 2; n++ {
		cp.MachinePaths[n] = applicationObjectName(fmt.Sprintf("inventory-%d", n), id)
		cp.HumanPaths[n] = applicationObjectName(fmt.Sprintf("human-%d", n), id)
		cp.Digests[n] = hash(raw)
		cp.Digests[n+2] = hash(human)
		objects = append(objects, applicationObject{cp.MachinePaths[n], raw}, applicationObject{cp.HumanPaths[n], human})
	}
	// Pages are immutable before the create-only selector transaction. Interrupted
	// higher pages remain unselected; no alias is rewritten or candidate prefix read.
	for _, o := range objects {
		if e := j.putObject(o); e != nil {
			return "", e
		}
	}
	for _, o := range objects {
		b, e := j.read(o.Name, ApplicationRecordLimit)
		if e != nil || !bytes.Equal(b, o.Raw) {
			return "", ErrInvalid
		}
	}
	if e := j.verifyInventoryCheckpoint(cp); e != nil {
		return "", e
	}
	cpRaw := mustJSON(cp)
	cpID := hash(cpRaw)
	cr, encodingErr := applicationMarshal(cursor, 16<<10)
	if encodingErr != nil || len(cr) > 8192 {
		return "", ErrInvalid
	}
	crID := hash(cr)
	h := j.head
	h.LastCompletePublicationSHA256 = cpID
	h.CursorSHA256 = crID
	e := j.commit("COMPLETE_INVENTORY_SELECTOR", cpID, h, applicationObject{applicationObjectName("data", cpID), cpRaw}, applicationObject{applicationObjectName("data", crID), cr})
	return cpID, e
}
func (a *ApplicationJournal) VerifyInventory(id string) (ApplicationCheckpoint, error) {
	var cp ApplicationCheckpoint
	if e := a.GetRecord(id, &cp); e != nil {
		return cp, e
	}
	return cp, a.implementation.verifyInventoryCheckpoint(cp)
}
func applicationInventoryHuman(raw []byte) []byte {
	return []byte(fmt.Sprintf("# Provisional application page\n\nAuthority 0; accepted false; feature UNRESOLVED; source UNKNOWN; absence PROHIBITED.\nNavigation UNASSIGNED; backlog not computed.\n\n%s\n", html.EscapeString(string(raw))))
}
func (j *applicationJournal) verifyInventoryCheckpoint(cp ApplicationCheckpoint) error {
	if cp.Schema != "private-application-checkpoint.v1" || cp.ApplicationSHA256 != j.head.ApplicationSHA256 || !digestValue(cp.PageSHA256) || cp.Previous != "" && !digestValue(cp.Previous) {
		return ErrInvalid
	}
	for n := 0; n < 2; n++ {
		if cp.MachinePaths[n] != applicationObjectName(fmt.Sprintf("inventory-%d", n), cp.PageSHA256) || cp.HumanPaths[n] != applicationObjectName(fmt.Sprintf("human-%d", n), cp.PageSHA256) {
			return ErrInvalid
		}
	}
	var page ApplicationInventoryPage
	var machine []byte
	var infos []os.FileInfo
	for n, p := range []string{cp.MachinePaths[0], cp.MachinePaths[1], cp.HumanPaths[0], cp.HumanPaths[1]} {
		if privateDir(filepath.Join(j.config.Store, filepath.Dir(p))) != nil {
			return ErrInvalid
		}
		b, e := j.read(p, ApplicationRecordLimit)
		if e != nil || hash(b) != cp.Digests[n] {
			return ErrInvalid
		}
		st, e := j.root.Stat(p)
		if e != nil {
			return e
		}
		for _, prior := range infos {
			if os.SameFile(prior, st) {
				return ErrInvalid
			}
		}
		infos = append(infos, st)
		if n == 0 {
			if strict(b, &page) != nil || hash(b) != cp.PageSHA256 {
				return ErrInvalid
			}
			machine = b
		}
		if n == 1 && !bytes.Equal(b, machine) || n >= 2 && !bytes.Equal(b, applicationInventoryHuman(machine)) {
			return ErrInvalid
		}
	}
	if page.Schema != "private-application-inventory-page.v1" || page.ApplicationSHA256 != cp.ApplicationSHA256 || page.Previous != cp.Previous || len(page.Rows) > 8 || len(page.Gaps) > 8 || page.Authority != 0 || page.Accepted || page.FeatureIdentity != "UNRESOLVED" || page.SourceCompleteness != "UNKNOWN" || page.AbsenceInference != "PROHIBITED" || page.NavigationStatus != "UNASSIGNED_NAVIGATION_BACKLOG_NOT_COMPUTED" {
		return ErrInvalid
	}
	for _, gap := range page.Gaps {
		if !digestValue(gap.UnitID) || !digestValue(gap.InputSHA256) || gap.RevisionSHA256 != hash([]byte(j.config.RevisionAssertion)) || filepath.IsAbs(gap.Path) || filepath.Clean(gap.Path) != gap.Path || gap.Path == ".." || strings.HasPrefix(gap.Path, ".."+string(filepath.Separator)) || len(gap.Path) > 4096 || gap.Reason == "" || len(gap.Reason) > 128 {
			return ErrInvalid
		}
	}
	for _, r := range page.Rows {
		if !digestValue(r.UnitID) || r.Store != filepath.Join(j.config.Store, "units", r.UnitID[7:], "cohort") || r.ApplicationSHA256 != page.ApplicationSHA256 || VerifyUnitResultRef(r) != nil {
			return ErrInvalid
		}
	}
	return nil
}
