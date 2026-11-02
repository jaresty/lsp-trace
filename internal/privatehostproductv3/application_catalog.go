//go:build darwin || linux

package privatehostproductv3

import (
	"html"
	"os"
	"time"
)

type ApplicationCatalogEntry struct {
	Schema, ApplicationSHA256, Path, Identity, Disposition, Next string
	Bytes                                                        int64
	ModTimeNS                                                    int64
}
type ApplicationCatalogPage struct {
	Schema, ApplicationSHA256 string
	Entries                   []ApplicationCatalogEntry
}

func (a *ApplicationJournal) HasIndexInPage(key string, r OperationReservation) (bool, error) {
	j := a.implementation
	if !digestValue(key) || r.EventID != j.activePage || r.Kind != "directory_page" || j.pageLookups < 1 {
		return false, ErrInvalid
	}
	j.pageLookups--
	j.io.RecordLookups++
	_, e := j.root.Lstat(applicationObjectName("index", key))
	if os.IsNotExist(e) {
		return false, nil
	}
	return e == nil, e
}
func ApplicationCatalogEncoding(entries []ApplicationCatalogEntry, app string) ([]byte, []byte, error) {
	if len(entries) < 1 || len(entries) > 8 {
		return nil, nil, ErrInvalid
	}
	page := ApplicationCatalogPage{"private-application-catalog-page.v1", app, entries}
	raw, e := applicationMarshal(page, ApplicationRecordLimit)
	if e != nil {
		return nil, nil, e
	}
	human := []byte("# Provisional source observations\n\nAuthority 0; accepted false; feature UNRESOLVED; source UNKNOWN; absence PROHIBITED.\n\n" + html.EscapeString(string(raw)) + "\n")
	if len(human) > ApplicationRecordLimit {
		return nil, nil, ErrInvalid
	}
	return raw, human, nil
}
func (a *ApplicationJournal) CommitCatalogPage(keys []string, entries []ApplicationCatalogEntry, state any) (string, error) {
	if len(keys) != len(entries) || len(entries) < 1 || len(entries) > 8 {
		return "", ErrInvalid
	}
	j := a.implementation
	if e := j.attempt("CATALOGUE_PAGE", time.Now()); e != nil {
		return "", e
	}
	raw, human, e := ApplicationCatalogEncoding(entries, a.Head().ApplicationSHA256)
	if e != nil {
		return "", e
	}
	id := hash(raw)
	sr, e := applicationMarshal(state, 16<<10)
	if e != nil || len(sr) > 8192 {
		return "", ErrInvalid
	}
	sid := hash(sr)
	objects := []applicationObject{{applicationObjectName("data", id), raw}, {applicationObjectName("data", sid), sr}}
	for n, entry := range entries {
		if !digestValue(keys[n]) || entry.ApplicationSHA256 != a.Head().ApplicationSHA256 || j.exists("index", keys[n]) {
			return "", ErrInvalid
		}
		er, e := applicationMarshal(entry, 16<<10)
		if e != nil || len(er) > 8192 {
			return "", ErrInvalid
		}
		eid := hash(er)
		objects = append(objects, applicationObject{applicationObjectName("data", eid), er}, applicationObject{applicationObjectName("index", keys[n]), mustJSON(struct{ SHA256 string }{eid})}, applicationObject{applicationObjectName("entry-page", eid), mustJSON(struct{ SHA256 string }{id})})
	}
	for _, kind := range []string{"catalog-a", "catalog-b"} {
		objects = append(objects, applicationObject{applicationObjectName(kind, id), raw})
	}
	for _, kind := range []string{"view-a", "view-b"} {
		objects = append(objects, applicationObject{applicationObjectName(kind, id), human})
	}
	next, ok := applicationAdd(j.head.Reserved, ApplicationBounds{OutputBytes: uint64(len(raw) + len(human))})
	if !ok || !applicationWithin(next, j.config.Grant.Ceiling) {
		return "", ErrPaused
	}
	h := j.head
	h.Reserved = next
	h.CursorSHA256 = sid
	return id, j.commit("CATALOGUE_PAGE_AND_CURSOR", id, h, objects...)
}
func (a *ApplicationJournal) CatalogPageForEntry(entryID string) (string, error) {
	if !digestValue(entryID) {
		return "", ErrInvalid
	}
	j := a.implementation
	if e := j.attempt("CATALOGUE_PAGE_LOOKUP", time.Now()); e != nil {
		return "", e
	}
	var ref struct{ SHA256 string }
	if e := j.object("entry-page", entryID, &ref); e != nil {
		return "", e
	}
	if !digestValue(ref.SHA256) {
		return "", ErrInvalid
	}
	return ref.SHA256, nil
}
