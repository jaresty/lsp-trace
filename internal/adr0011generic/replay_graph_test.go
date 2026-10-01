package adr0011generic

import (
	"strings"
	"testing"
)

// These test-only selectors name independently held synthetic choices. The B2
// verifier must not issue selectors or infer expectations from claimant edges.
func b2Fixture(method string, contextual bool, targets int) (syntheticGraphSelection, []syntheticGraphRole) {
	id := syntheticTransactionIdentity{session: "s", generation: 7, transaction: "tx-1"}
	held := syntheticGraphSelection{identity: id, method: method}
	profile := "GENERIC_LSP_REFERENCES_EXACT_V2"
	if method == "textDocument/definition" {
		profile = "GENERIC_LSP_DEFINITION_EXACT_V2"
	}
	names := []string{"POLICY", "SCHEMA", "SOURCE:query", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS", "TERMINAL"}
	if contextual {
		names = append(names, "PROCESS")
	}
	for i := 0; i < targets; i++ {
		names = append(names, "SOURCE:target:"+string(rune('a'+i)))
	}
	refs := make(map[string]syntheticPredecessor, len(names))
	for _, name := range names {
		role := name
		if strings.HasPrefix(name, "SOURCE:") {
			role = "SOURCE"
		}
		ref := syntheticPredecessor{role: role, selector: "sha256:" + digest([]byte("selected "+name)), digest: "sha256:" + digest([]byte("original "+name))}
		if role == "SCHEMA" || role == "POLICY" {
			originalRole := "SCHEMA"
			if role == "POLICY" {
				originalRole = "REFERENCES"
				if method == "textDocument/definition" {
					originalRole = "DEFINITION"
				}
			}
			original, err := loadOriginal(originalRole)
			if err != nil {
				panic(err)
			}
			ref.digest = "sha256:" + digest(original)
		}
		held.roles = append(held.roles, ref)
		refs[name] = ref
	}
	held.querySourceSelector = refs["SOURCE:query"].selector
	link := func(names ...string) []syntheticPredecessor {
		var out []syntheticPredecessor
		for _, name := range names {
			out = append(out, refs[name])
		}
		return out
	}
	var records []syntheticGraphRole
	for _, name := range names {
		ref := refs[name]
		record := syntheticGraphRole{role: ref.role, selector: ref.selector, digest: ref.digest, method: method, profile: profile, identity: id}
		switch name {
		case "QUERY":
			record.predecessors = link("POLICY", "SCHEMA", "SOURCE:query")
		case "CAPABILITY_EVENTS":
			if contextual {
				record.predecessors = link("PROCESS")
			}
		case "REQUEST_WRITE":
			record.predecessors = link("QUERY", "CAPABILITY_EVENTS", "POLICY")
			if contextual {
				record.predecessors = append(record.predecessors, refs["PROCESS"])
			}
		case "INBOUND_FRAMES":
			record.predecessors = link("REQUEST_WRITE")
		case "RESULT_READ":
			record.predecessors = link("INBOUND_FRAMES", "REQUEST_WRITE")
		case "TARGET_EVENTS":
			record.predecessors = link("RESULT_READ")
			for i := 0; i < targets; i++ {
				record.predecessors = append(record.predecessors, refs["SOURCE:target:"+string(rune('a'+i))])
			}
		case "TERMINAL":
			record.predecessors = link("POLICY", "SCHEMA", "SOURCE:query", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS")
			for i := 0; i < targets; i++ {
				record.predecessors = append(record.predecessors, refs["SOURCE:target:"+string(rune('a'+i))])
			}
		}
		records = append(records, record)
	}
	return held, records
}

func b2Copy(records []syntheticGraphRole) []syntheticGraphRole {
	copied := append([]syntheticGraphRole(nil), records...)
	for i := range copied {
		copied[i].predecessors = append([]syntheticPredecessor(nil), records[i].predecessors...)
	}
	return copied
}

func b2Index(records []syntheticGraphRole, role string) int {
	for i, record := range records {
		if record.role == role {
			return i
		}
	}
	panic("missing fixture role " + role)
}

func TestB2AcceptsSelectedSyntheticGraphs(t *testing.T) {
	for _, method := range []string{"textDocument/references", "textDocument/definition"} {
		for _, contextual := range []bool{false, true} {
			for _, targets := range []int{0, 1} {
				held, records := b2Fixture(method, contextual, targets)
				if err := validateSyntheticGraph(records, held); err != nil {
					t.Fatalf("B2 valid graph %s PROCESS=%v targets=%d rejected: %v", method, contextual, targets, err)
				}
			}
		}
	}
}

func TestB2RejectsPredecessorMutations(t *testing.T) {
	held, base := b2Fixture("textDocument/references", false, 1)
	for _, tc := range []struct {
		name string
		want string
		edit func([]syntheticGraphRole)
	}{
		{"missing predecessor", "predecessor", func(r []syntheticGraphRole) { q := b2Index(r, "QUERY"); r[q].predecessors = r[q].predecessors[1:] }},
		{"wrong role", "role", func(r []syntheticGraphRole) { q := b2Index(r, "QUERY"); r[q].predecessors[0].role = "SCHEMA" }},
		{"duplicate predecessor", "duplicate", func(r []syntheticGraphRole) {
			terminal := b2Index(r, "TERMINAL")
			r[terminal].predecessors = append(r[terminal].predecessors, r[terminal].predecessors[0])
		}},
		{"substituted predecessor", "predecessor", func(r []syntheticGraphRole) {
			q := b2Index(r, "QUERY")
			target := r[len(r)-1] // a different, fully selected SOURCE
			r[q].predecessors[2] = syntheticPredecessor{role: target.role, selector: target.selector, digest: target.digest}
		}},
		{"substituted digest", "digest", func(r []syntheticGraphRole) {
			q := b2Index(r, "QUERY")
			r[q].predecessors[0].digest = "sha256:" + digest([]byte("substituted"))
		}},
		{"cycle", "cycle", func(r []syntheticGraphRole) {
			q := b2Index(r, "QUERY")
			r[q].predecessors[2] = syntheticPredecessor{role: r[q].role, selector: r[q].selector, digest: r[q].digest}
		}},
		{"cross-transaction substitution", "transaction", func(r []syntheticGraphRole) { q := b2Index(r, "QUERY"); r[q].identity.transaction = "tx-other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records := b2Copy(base)
			tc.edit(records)
			if err := validateSyntheticGraph(records, held); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("B2 %s accepted or misclassified: %v (want %q)", tc.name, err, tc.want)
			}
		})
	}
}

func TestB2RejectsMethodAndContextSubstitution(t *testing.T) {
	held, base := b2Fixture("textDocument/references", true, 0)
	for _, tc := range []struct {
		name string
		want string
		edit func([]syntheticGraphRole)
	}{
		{"definition method substitution", "method", func(r []syntheticGraphRole) { r[b2Index(r, "QUERY")].method = "textDocument/definition" }},
		{"definition profile substitution", "profile", func(r []syntheticGraphRole) { r[b2Index(r, "POLICY")].profile = "GENERIC_LSP_DEFINITION_EXACT_V2" }},
		{"selector substitution", "selector", func(r []syntheticGraphRole) {
			r[b2Index(r, "QUERY")].selector = "sha256:" + digest([]byte("other query"))
		}},
		{"contextual PROCESS mismatch", "PROCESS", func(r []syntheticGraphRole) {
			r[b2Index(r, "PROCESS")].digest = "sha256:" + digest([]byte("other context"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records := b2Copy(base)
			tc.edit(records)
			if err := validateSyntheticGraph(records, held); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("B2 %s accepted or misclassified: %v (want %q)", tc.name, err, tc.want)
			}
		})
	}
}

func TestB2OptionalProcessEdgesDoNotGate(t *testing.T) {
	held, base := b2Fixture("textDocument/references", true, 0)
	process := base[b2Index(base, "PROCESS")]
	for _, role := range []string{"CAPABILITY_EVENTS", "REQUEST_WRITE"} {
		records := b2Copy(base)
		index := b2Index(records, role)
		without := records[index].predecessors[:0]
		for _, predecessor := range records[index].predecessors {
			if predecessor.role != "PROCESS" {
				without = append(without, predecessor)
			}
		}
		records[index].predecessors = without
		if err := validateSyntheticGraph(records, held); err != nil {
			t.Fatalf("B2 optional %s PROCESS edge incorrectly required: %v", role, err)
		}
	}
	records := b2Copy(base)
	terminal := b2Index(records, "TERMINAL")
	records[terminal].predecessors = append(records[terminal].predecessors, syntheticPredecessor{role: process.role, selector: process.selector, digest: process.digest})
	if err := validateSyntheticGraph(records, held); err != nil {
		t.Fatalf("B2 optional TERMINAL PROCESS edge rejected: %v", err)
	}
}

func TestB2V2Former32EdgeBoundary(t *testing.T) {
	t.Run("32 mandatory TERMINAL accepted", func(t *testing.T) {
		held, records := b2Fixture("textDocument/references", false, 23) // eight fixed + 24 SOURCE
		if got := len(records[b2Index(records, "TERMINAL")].predecessors); got != 32 {
			t.Fatalf("B2 at-limit fixture has %d predecessors, want 32", got)
		}
		if err := validateSyntheticGraph(records, held); err != nil {
			t.Fatalf("B2 32-predecessor TERMINAL rejected: %v", err)
		}
	})
	t.Run("33 mandatory TERMINAL accepted", func(t *testing.T) {
		held, records := b2Fixture("textDocument/references", false, 24) // eight fixed + 25 SOURCE
		if got := len(records[b2Index(records, "TERMINAL")].predecessors); got != 33 {
			t.Fatalf("B2 over-limit fixture has %d predecessors, want 33", got)
		}
		inventory := make([]syntheticRole, 0, len(records))
		for _, record := range records {
			inventory = append(inventory, syntheticRole{role: record.role})
		}
		if err := validateRoleInventory(inventory, syntheticRoleSelection{sourceCount: 25}); err != nil {
			t.Fatalf("B1's independent 25-SOURCE inventory unexpectedly rejected: %v", err)
		}
		if err := validateSyntheticGraph(records, held); err != nil {
			t.Fatalf("B2 V2 33-predecessor TERMINAL rejected: %v", err)
		}
	})
	t.Run("32 contextual TERMINAL accepted", func(t *testing.T) {
		held, records := b2Fixture("textDocument/definition", true, 22) // eight fixed + 23 SOURCE + PROCESS
		process := records[b2Index(records, "PROCESS")]
		terminal := b2Index(records, "TERMINAL")
		records[terminal].predecessors = append(records[terminal].predecessors, syntheticPredecessor{role: process.role, selector: process.selector, digest: process.digest})
		if got := len(records[terminal].predecessors); got != 32 {
			t.Fatalf("B2 contextual at-limit fixture has %d predecessors, want 32", got)
		}
		if err := validateSyntheticGraph(records, held); err != nil {
			t.Fatalf("B2 32-predecessor contextual TERMINAL rejected: %v", err)
		}
	})
	t.Run("33 contextual TERMINAL accepted", func(t *testing.T) {
		held, records := b2Fixture("textDocument/definition", true, 23) // eight fixed + 24 SOURCE + PROCESS
		process := records[b2Index(records, "PROCESS")]
		terminal := b2Index(records, "TERMINAL")
		records[terminal].predecessors = append(records[terminal].predecessors, syntheticPredecessor{role: process.role, selector: process.selector, digest: process.digest})
		if got := len(records[terminal].predecessors); got != 33 {
			t.Fatalf("B2 contextual over-limit fixture has %d predecessors, want 33", got)
		}
		if err := validateSyntheticGraph(records, held); err != nil {
			t.Fatalf("B2 V2 33-predecessor contextual TERMINAL rejected: %v", err)
		}
	})
}

func TestB2RejectsCoherentHeldOriginalSubstitution(t *testing.T) {
	for _, role := range []string{"POLICY", "SCHEMA"} {
		t.Run(role, func(t *testing.T) {
			held, records := b2Fixture("textDocument/references", false, 0)
			index := b2Index(records, role)
			originalRole := "DEFINITION"
			if role == "SCHEMA" {
				originalRole = "SCHEMA"
			}
			original, err := loadOriginal(originalRole)
			if err != nil {
				t.Fatal(err)
			}
			wrong := "sha256:" + digest(append(original, '\n'))
			if role == "POLICY" {
				wrong = "sha256:" + digest(original) // valid definition original, wrong method
			}
			selector := records[index].selector
			records[index].digest = wrong
			for i := range held.roles {
				if held.roles[i].selector == selector {
					held.roles[i].digest = wrong
				}
			}
			for i := range records {
				for j := range records[i].predecessors {
					if records[i].predecessors[j].selector == selector {
						records[i].predecessors[j].digest = wrong
					}
				}
			}
			if err := validateSyntheticGraph(records, held); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(role)) {
				t.Fatalf("B2 coherent %s original substitution accepted or misclassified: %v", role, err)
			}
		})
	}
}

func TestB2V2RoleSpecificBoundaries(t *testing.T) {
	for _, method := range []string{"textDocument/references", "textDocument/definition"} {
		t.Run(method, func(t *testing.T) {
			held, base := b2Fixture(method, true, 255) // one query SOURCE + 255 distinct target SOURCE
			terminal := b2Index(base, "TERMINAL")
			target := b2Index(base, "TARGET_EVENTS")
			if len(base[terminal].predecessors) != 264 || len(base[target].predecessors) != 256 {
				t.Fatal("B2 V2 at-limit fixture malformed")
			}
			if err := validateSyntheticGraph(base, held); err != nil {
				t.Errorf("B2 V2 264 TERMINAL and 256 TARGET_EVENTS rejected: %v", err)
			}
			process := base[b2Index(base, "PROCESS")]
			over := b2Copy(base)
			over[terminal].predecessors = append(over[terminal].predecessors, syntheticPredecessor{role: process.role, selector: process.selector, digest: process.digest})
			if err := validateSyntheticGraph(over, held); err == nil || !strings.Contains(err.Error(), "predecessor limit") {
				t.Errorf("B2 V2 265 TERMINAL not rejected by role limit: %v", err)
			}
			over = b2Copy(base)
			over[target].predecessors = append(over[target].predecessors, over[target].predecessors[1])
			if err := validateSyntheticGraph(over, held); err == nil || !strings.Contains(err.Error(), "predecessor limit") {
				t.Errorf("B2 V2 257 TARGET_EVENTS not rejected by role limit: %v", err)
			}
			invalidHeld, invalidRecords := b2Fixture(method, false, 256) // 257 SOURCE documents
			if err := validateSyntheticGraph(invalidRecords, invalidHeld); err == nil || !strings.Contains(err.Error(), "cardinality") {
				t.Errorf("B2 V2 257 held SOURCE not rejected: %v", err)
			}
		})
	}
}

func TestB2V2PerRoleCardinalityAndCaps(t *testing.T) {
	held, base := b2Fixture("textDocument/references", true, 1)
	// These are independently held selected edges, not arbitrary unselected input.
	for _, tc := range []struct {
		role  string
		extra string
	}{
		{"POLICY", "SOURCE"}, {"SCHEMA", "SOURCE"}, {"SOURCE", "POLICY"}, {"PROCESS", "SOURCE"},
		{"QUERY", "SOURCE"}, {"CAPABILITY_EVENTS", "PROCESS"}, {"REQUEST_WRITE", "PROCESS"},
		{"INBOUND_FRAMES", "REQUEST_WRITE"}, {"RESULT_READ", "REQUEST_WRITE"},
	} {
		t.Run(tc.role+" cap+1", func(t *testing.T) {
			records := b2Copy(base)
			index := b2Index(records, tc.role)
			ref := records[b2Index(records, tc.extra)]
			records[index].predecessors = append(records[index].predecessors, syntheticPredecessor{role: ref.role, selector: ref.selector, digest: ref.digest})
			if err := validateSyntheticGraph(records, held); err == nil || !strings.Contains(err.Error(), "predecessor limit") {
				t.Errorf("B2 V2 %s cap+1 not rejected by role limit: %v", tc.role, err)
			}
		})
	}
	// Eight fixed role edges without any SOURCE cannot pass independently held equality.
	records := b2Copy(base)
	terminal := b2Index(records, "TERMINAL")
	without := []syntheticPredecessor{}
	for _, edge := range records[terminal].predecessors {
		if edge.role != "SOURCE" {
			without = append(without, edge)
		}
	}
	records[terminal].predecessors = without
	if err := validateSyntheticGraph(records, held); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("B2 V2 TERMINAL eight fixed zero SOURCE accepted or misclassified: %v", err)
	}
}

func TestB2RejectsReadback(t *testing.T) {
	held, base := b2Fixture("textDocument/definition", false, 0)
	base = append(base, syntheticGraphRole{role: "READBACK", identity: held.identity, method: held.method})
	if err := validateSyntheticGraph(base, held); err == nil {
		t.Fatal("B2 READBACK role accepted")
	}
}
