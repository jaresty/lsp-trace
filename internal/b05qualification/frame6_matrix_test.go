package b05qualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type frame6Seed struct {
	ID, Relation, Polarity, Path, SHA256, ExpectedOutcome string
}
type frame6Attempt struct {
	ID, SeedID, Operation, Outcome, Transport, ProviderPathKind, ProviderIdentity, Language, Framework, WorkspaceCommit, WorkspaceCustody string
	SchemaValid, DeterministicReplay, CanonicalEnvelope                                                                                   bool
	RelationKind, From, To                                                                                                                string
	Anchors, ContributorIDs, DoesNotSupport                                                                                               []string
	DomainCode, CoverageBoundary                                                                                                          string
	OverclaimsAbsence                                                                                                                     bool
}
type frame6Stage struct{ Relation, Stage, Outcome string }
type frame6Matrix struct {
	SchemaVersion            string                                 `json:"schema_version"`
	ProviderPackageSHA256    string                                 `json:"provider_package_sha256"`
	ProviderExecutableSHA256 string                                 `json:"provider_executable_sha256"`
	ProgramBAdmitted         bool                                   `json:"PROGRAM_B_ADMITTED"`
	AdmissionRule            string                                 `json:"admission_rule"`
	Seeds                    []frame6Seed                           `json:"seeds"`
	Attempts                 []frame6Attempt                        `json:"attempts"`
	Stages                   []frame6Stage                          `json:"stages"`
	Capabilities             struct{ Advertised, Blocked []string } `json:"capabilities"`
	ReleaseCheck             string                                 `json:"release_check"`
}

func loadFrame6(t *testing.T) frame6Matrix {
	t.Helper()
	raw, err := os.ReadFile("../../qualification/retained/b05/qualification-matrix.v2.json")
	if err != nil {
		t.Fatalf("ASSERT_B05_FRAME6_MATRIX_PRESENT: %v", err)
	}
	var m frame6Matrix
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("ASSERT_B05_FRAME6_MATRIX_SCHEMA: %v", err)
	}
	return m
}
func TestFrame6ExactQualificationMatrix(t *testing.T) {
	m := loadFrame6(t)
	if m.SchemaVersion != "lsp-trace.b05-qualification-matrix.v2" {
		t.Fatal("ASSERT_B05_FRAME6_MATRIX_SCHEMA")
	}
	if len(m.Seeds) != 12 {
		t.Fatalf("ASSERT_B05_FRAME6_EXACT_TWELVE_SEEDS: %d", len(m.Seeds))
	}
	pairs := map[string]map[string]bool{}
	for _, s := range m.Seeds {
		if s.ID == "" || s.Path == "" || s.ExpectedOutcome == "" {
			t.Fatalf("ASSERT_B05_FRAME6_SEED_EXACT_EXPECTATION: %+v", s)
		}
		raw, err := os.ReadFile("../../" + s.Path)
		if err != nil {
			t.Fatalf("ASSERT_B05_FRAME6_SEED_IMMUTABLE: %s: %v", s.ID, err)
		}
		d := sha256.Sum256(raw)
		if hex.EncodeToString(d[:]) != s.SHA256 {
			t.Fatalf("ASSERT_B05_FRAME6_SEED_IMMUTABLE: %s", s.ID)
		}
		if pairs[s.Relation] == nil {
			pairs[s.Relation] = map[string]bool{}
		}
		pairs[s.Relation][s.Polarity] = true
	}
	for _, r := range []string{"BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"} {
		if !pairs[r]["positive"] || !pairs[r]["confusable_negative"] || len(pairs[r]) != 2 {
			t.Fatalf("ASSERT_B05_FRAME6_RELATION_POLARITY_PAIR: %s", r)
		}
	}
	if len(m.Attempts) != 24 {
		t.Fatalf("ASSERT_B05_FRAME6_EXACT_TWENTY_FOUR_ATTEMPTS: %d", len(m.Attempts))
	}
	seen := map[string]bool{}
	for _, a := range m.Attempts {
		key := a.SeedID + "/" + a.Operation
		if seen[key] || (a.Operation != "incoming" && a.Operation != "slice") {
			t.Fatalf("ASSERT_B05_FRAME6_ATTEMPT_CARTESIAN: %s", key)
		}
		seen[key] = true
		if a.Transport != "real-lsp-trace-mcp-stdio" || a.ProviderPathKind != "independently-packed-installed" || a.ProviderIdentity != "ember-glint@1" || a.Language != "glimmer-js" || a.Framework != "ember" || a.WorkspaceCommit == "" || a.WorkspaceCustody != "PROVIDER_PROVED" || !a.CanonicalEnvelope || !a.DeterministicReplay || !a.SchemaValid {
			t.Fatalf("ASSERT_B05_FRAME6_PRODUCTION_BOUNDARY: %s", a.ID)
		}
		if a.Outcome == "PASS" {
			if a.RelationKind == "" || a.From == "" || a.To == "" || len(a.Anchors) == 0 || len(a.ContributorIDs) == 0 || len(a.DoesNotSupport) == 0 {
				t.Fatalf("ASSERT_B05_FRAME6_PASS_EXACT_EVIDENCE: %s", a.ID)
			}
		} else if a.Outcome == "NO_RELATION" {
			if a.CoverageBoundary == "" || a.OverclaimsAbsence {
				t.Fatalf("ASSERT_B05_FRAME6_NEGATIVE_BOUNDED: %s", a.ID)
			}
		} else if a.Outcome == "BLOCKED" {
			if a.DomainCode == "" || a.OverclaimsAbsence {
				t.Fatalf("ASSERT_B05_FRAME6_BLOCKED_EXPLICIT_BOUNDED: %s", a.ID)
			}
		} else {
			t.Fatalf("ASSERT_B05_FRAME6_EXPECTED_OUTCOME: %s", a.ID)
		}
	}
	if len(m.Stages) != 24 {
		t.Fatalf("ASSERT_B05_FRAME6_FOUR_STAGES_PER_RELATION: %d", len(m.Stages))
	}
	if m.ProviderPackageSHA256 == "" || m.ProviderExecutableSHA256 == "" {
		t.Fatal("ASSERT_B05_FRAME6_PROVIDER_PACKAGE_DIGESTS")
	}
	if m.ProgramBAdmitted || m.AdmissionRule != "all_requested_relations_supported" {
		t.Fatal("ASSERT_B05_FRAME6_PROGRAM_B_NOT_ADMITTED")
	}
	for _, r := range []string{"INVOKES_TASK", "TRIGGERS_RELOAD"} {
		found := false
		for _, b := range m.Capabilities.Blocked {
			found = found || b == r
		}
		if !found {
			t.Fatalf("ASSERT_B05_FRAME6_BLOCKED_CAPABILITY: %s", r)
		}
		for _, a := range m.Capabilities.Advertised {
			if a == r {
				t.Fatalf("ASSERT_B05_FRAME6_BLOCKED_NOT_ADVERTISED: %s", r)
			}
		}
	}
	if m.ReleaseCheck != "scripts/test-b05-qualification.sh" {
		t.Fatal("ASSERT_B05_FRAME6_RELEASE_CONSUMPTION")
	}
}
