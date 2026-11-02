//go:build darwin || linux

package main

import (
	"encoding/json"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func publishApplicationGap(a *product.ApplicationJournal, s *applicationState, entry applicationCatalogEntry, unitID, revision, reason string) error {
	raw, _ := json.Marshal(entry)
	page := product.ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: s.ApplicationSHA256, Previous: a.Head().LastCompletePublicationSHA256, Gaps: []product.ApplicationSourceGap{{UnitID: unitID, InputSHA256: pilot.Digest(raw), RevisionSHA256: pilot.Digest([]byte(revision)), Path: entry.Path, Reason: reason}}}
	id, e := a.PublishInventory(page, *s)
	if e == nil {
		s.LastPage = id
	}
	return e
}
