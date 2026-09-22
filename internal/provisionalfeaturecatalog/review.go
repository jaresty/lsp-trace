package provisionalfeaturecatalog

import (
	"fmt"
	"sort"
	"strings"
)

const reviewBanner = "PROVISIONAL — NOT ACCEPTED · Authority 0 · Completeness UNKNOWN"

// RenderReview renders only catalog-owned mechanical IDs and copied semantic fields.
func RenderReview(c Catalog) (string, error) {
	if err := Validate(c); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintln(&b, reviewBanner)
	fmt.Fprintf(&b, "Catalog %s · Outcome %s\n", c.wire.CatalogID, c.wire.Outcome)
	communities := map[string][]Entry{}
	for _, e := range c.wire.Entries {
		communities[e.Lineage.CommunityIdentity] = append(communities[e.Lineage.CommunityIdentity], e)
	}
	ids := make([]string, 0, len(communities))
	for id := range communities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "\n## Mechanical community %s\n", id)
		fmt.Fprintln(&b, "Community is a mechanical review grouping, not an accepted feature.")
		for _, e := range communities[id] {
			fmt.Fprintf(&b, "- Entry %s\n", e.EntryID)
			fmt.Fprintf(&b, "  Packet %s · Alternative %d/%s · Request %s · Invocation %s · Response %s\n", e.Lineage.PacketID, e.Lineage.AlternativeOrdinal, e.Lineage.AlternativeID, e.Lineage.RequestRecordID, e.Lineage.InvocationID, e.Lineage.ResponseID)
			fmt.Fprintf(&b, "  Terminal: %s\n", e.TerminalOutcome)
			fmt.Fprintf(&b, "  Model target_role (model text, not catalog identity): %s\n", safeText(e.Semantic.TargetRole))
			fmt.Fprintf(&b, "  Model nearest_outward_consumer: %s\n", safeText(e.Semantic.NearestOutwardConsumer))
			fmt.Fprintf(&b, "  Model consumer_need: %s\n", safeText(e.Semantic.ConsumerNeed))
			fmt.Fprintf(&b, "  Model provided_behavior: %s\n", safeText(e.Semantic.ProvidedBehavior))
			fmt.Fprintf(&b, "  Model boundary_contribution: %s\n", safeText(e.Semantic.BoundaryContribution))
			fmt.Fprintln(&b, "  Limitations:")
			for _, v := range e.Semantic.Limitations {
				fmt.Fprintf(&b, "    - %s\n", safeText(v.Text))
			}
			fmt.Fprintln(&b, "  Citations:")
			for _, v := range e.Semantic.Citations {
				fmt.Fprintf(&b, "    - %s\n", safeText(v.Text))
			}
		}
	}
	a := c.wire.Accounting
	fmt.Fprintf(&b, "\n## Terminal accounting\nTotal %d · COMPLETE %d · ABSTAINED %d · INVALID_INPUT %d · MODEL_UNAVAILABLE %d · CONTEXT_LIMIT %d · OUTPUT_INVALID %d · TIMEOUT %d · CANCELLED %d · RESOURCE_LIMIT %d · BACKEND_FAILURE %d · POLICY_MISMATCH %d · DUPLICATE_INPUT %d\n", a.Total, a.Complete, a.Abstained, a.InvalidInput, a.ModelUnavailable, a.ContextLimit, a.OutputInvalid, a.Timeout, a.Cancelled, a.ResourceLimit, a.BackendFailure, a.PolicyMismatch, a.DuplicateInput)
	fmt.Fprintln(&b, "\n## Corrections")
	if len(c.wire.Corrections) == 0 {
		fmt.Fprintln(&b, "- None")
	}
	for _, k := range c.wire.Corrections {
		fmt.Fprintf(&b, "- %s · actor %s (%s) · %s · rebuild %s\n", k.CorrectionID, safeText(k.ActorID), k.ActorAuthority, safeText(k.Reason), k.RebuildDisposition)
	}
	return b.String(), nil
}

func safeText(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
