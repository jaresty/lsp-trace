// Package liveprojection orchestrates request-ephemeral source projection from
// one exact managed language-server session generation.
package liveprojection

import (
	"sort"

	"lsp-trace/internal/sourceprojection"
)

// PlanDocuments selects the logical source documents eligible for live
// acquisition. It returns each public source once, with an eligible target
// first and all remaining URIs in lexical order.
func PlanDocuments(candidates []sourceprojection.Candidate, targetURI string) []string {
	selected := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.PrivacyClassification != "PUBLIC" || candidate.LogicalSourceID == "" {
			continue
		}
		selected[candidate.LogicalSourceID] = struct{}{}
	}

	uris := make([]string, 0, len(selected))
	for uri := range selected {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	if _, ok := selected[targetURI]; !ok {
		return uris
	}
	for i, uri := range uris {
		if uri == targetURI {
			copy(uris[1:i+1], uris[:i])
			uris[0] = targetURI
			break
		}
	}
	return uris
}
