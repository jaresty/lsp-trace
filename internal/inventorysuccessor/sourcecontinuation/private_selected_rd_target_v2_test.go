package sourcecontinuation

import (
	"context"
	"testing"

	cc "lsp-trace/internal/censuscontinuation"
	inv "lsp-trace/internal/inventorysuccessor"
	tp "lsp-trace/internal/targetpacket"
)

// DATA ONLY: synthetic status/intent rows do not mint native admissions. These
// helpers test selection policy, not native D resolution or qualification.
func rdTargetRowsV2(t *testing.T) (cc.PrivateAllModeResult, PrivateAllModeSourcesResult, inv.PrivateSelectedConfigurationV2) {
	t.Helper()
	m, _, c := selectedRowsTestV2("RD", false)
	d := m.Nominations[0].Endpoints[0]
	r, q := d, d
	r.Kind, r.Role, r.Node, r.Ordinal = "REFERENCES_SYMBOL", "REFERENCED_SYMBOL", "r", 0
	q.Role, q.Node, q.Ordinal = "QUERY_OCCURRENCE", "query", 2
	m.Nominations[0].Endpoints = []tp.PrivateTypedEndpoint{r, d, q}
	s := rdTargetRebuildV2(t, &m)
	return m, s, c
}

func rdTargetRebuildV2(t *testing.T, m *cc.PrivateAllModeResult) PrivateAllModeSourcesResult {
	t.Helper()
	// Build a valid nomination membership for every synthetic endpoint.
	m.Nominations[0].Members = nil
	seen := map[string]bool{}
	for _, e := range m.Nominations[0].Endpoints {
		if !seen[e.Node] {
			m.Nominations[0].Members = append(m.Nominations[0].Members, e.Node)
			seen[e.Node] = true
		}
	}
	var err error
	m.Intents, err = tp.BuildPrivateSourceIntents(m.Nominations[0], 1000)
	if err != nil {
		t.Fatal(err)
	}
	s := PrivateAllModeSourcesResult{}
	for i := range m.Intents {
		s.Outcomes = append(s.Outcomes, PrivateSourceOutcome{Intent: i, Status: "SOURCE_ADMITTED", AdmissionID: "data-only-admission", RequestID: "data-only-request"})
	}
	return s
}

func TestSelectedV2RDTargetSelection(t *testing.T) {
	m, s, c := rdTargetRowsV2(t)
	row, err := selectSelectedRowCriterionV2(m, s, c, nil, true)
	if err != nil || row.RowPosition != 1 || row.Intent != m.Intents[1] || row.Slot != selectedSlotV2(c) {
		t.Fatal("D_TARGET_UNIQUE_ACTUAL_ROW", row, err)
	}
	t.Log("D_TARGET_UNIQUE_ACTUAL_ROW: PASS; DATA_ONLY")
	if _, err := selectSelectedRowV2(m, s, c, nil); err == nil {
		t.Fatal("LEGACY_NIL_REMAINS_AMBIGUOUS")
	}
	exact := PrivateSelectedEndpointV2{RowPosition: 0, Slot: selectedSlotV2(c), Intent: m.Intents[0]}
	if got, err := selectSelectedRowV2(m, s, c, &exact); err != nil || got != exact {
		t.Fatal("LEGACY_EXACT_REMAINS_EXACT", got, err)
	}
	if _, err := selectSelectedRowCriterionV2(m, s, c, &exact, true); err == nil {
		t.Fatal("BOTH_SELECTION_POLICIES_REFUSED")
	}
	t.Log("LEGACY_AND_EXCLUSIVE_POLICY: PASS")
}

func TestSelectedV2RDTargetRefusals(t *testing.T) {
	for _, name := range []string{"zero", "two", "identical", "kind", "role", "slot", "run", "lineage", "position", "unexpected", "unadmitted", "missing-admission", "missing-request", "mode"} {
		t.Run(name, func(t *testing.T) {
			m, s, c := rdTargetRowsV2(t)
			switch name {
			case "zero":
				m.Nominations[0].Endpoints = m.Nominations[0].Endpoints[:1]
				s = rdTargetRebuildV2(t, &m)
			case "two", "identical":
				d := m.Nominations[0].Endpoints[1]
				if name == "two" {
					d.Node, d.Ordinal = "other-target", 3
				}
				m.Nominations[0].Endpoints = append(m.Nominations[0].Endpoints, d)
				s = rdTargetRebuildV2(t, &m)
			case "kind", "role":
				// Isolate one wrong endpoint: another query row would mask a
				// missing role filter by accidentally creating ambiguity.
				m.Nominations[0].Endpoints = m.Nominations[0].Endpoints[:2]
				if name == "kind" {
					m.Nominations[0].Endpoints[1].Kind = "REFERENCES_SYMBOL"
					m.Nominations[0].Endpoints[1].Role = "REFERENCED_SYMBOL"
				} else {
					m.Nominations[0].Endpoints[1].Role = "QUERY_OCCURRENCE"
				}
				s = rdTargetRebuildV2(t, &m)
			case "slot":
				m.Intents[1].Slot++
			case "run":
				m.Intents[1].RunID += "wrong"
			case "lineage":
				m.Intents[1].Lineage += "wrong"
			case "position":
				s.Outcomes[1].Intent = 0
			case "unexpected":
				m.Intents[1].Endpoint.Node = "not-nominated"
			case "unadmitted":
				s.Outcomes[1].Status = "OBSERVED_NONEMPTY"
			case "missing-admission":
				s.Outcomes[1].AdmissionID = ""
			case "missing-request":
				s.Outcomes[1].RequestID = ""
			case "mode":
				c.Mode = "D"
			}
			if _, err := selectSelectedRowCriterionV2(m, s, c, nil, true); err == nil {
				t.Fatal("D_TARGET_REFUSAL", name)
			}
			t.Log("D_TARGET_REFUSAL: PASS", name)
		})
	}
}

func TestSelectedV2RDTargetLegacyDuplicatePosition(t *testing.T) {
	m, s, c := selectedRowsTestV2("RD", true)
	for _, p := range []int{-1, 0, 1, 2} {
		x := PrivateSelectedEndpointV2{RowPosition: p, Slot: selectedSlotV2(c), Intent: m.Intents[0]}
		got, err := selectSelectedRowV2(m, s, c, &x)
		if want := p == 0 || p == 1; (err == nil) != want || want && got != x {
			t.Fatal("LEGACY_DUPLICATE_POSITION", p, got, err)
		}
	}
}

func TestSelectedV2RDTargetNoAuthority(t *testing.T) {
	m, s, c := rdTargetRowsV2(t)
	row, err := selectSelectedRowCriterionV2(m, s, c, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if (inv.PrivateSourceAdmission{}).Matches(row.Intent) {
		t.Fatal("FORGED_DTO_IS_NOT_NATIVE_AUTHORITY")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, cx := range []context.Context{nil, context.Background(), ctx} {
		got, err := PrepareSelectedDTargetV2(cx, inv.PrivateSelectedSourceOperationV2{})
		if err == nil || got.Available() {
			t.Fatal("ZERO_OP_CANNOT_MINT_RESULT")
		}
	}
	t.Log("FORGED_DTO_IS_NOT_NATIVE_AUTHORITY: PASS; ZERO_AUTHORITY_BRANCH_PROBE")
}
