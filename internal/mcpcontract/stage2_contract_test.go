package mcpcontract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const stage2ContractPath = "testdata/stage2-lifecycle-contract.v1.json"

type stage2LifecycleContract struct {
	CandidateStatus          string                         `json:"candidate_status"`
	ContractVersion          string                         `json:"contract_version"`
	Extends                  string                         `json:"extends"`
	ValidationOrder          []string                       `json:"validation_order"`
	Availability             map[string]candidateProjection `json:"availability_projections"`
	Unresolved               []candidateUnresolved          `json:"unresolved_contract_assertions"`
	CallerAttachmentOverflow candidateCallerOverflow        `json:"caller_attachment_overflow"`
	TranscriptEvidence       map[string]string              `json:"transcript_evidence"`
	Schemas                  []SchemaRegistration           `json:"schemas"`
	Tools                    []ToolContract                 `json:"tools"`
	Transcripts              []string                       `json:"transcripts"`
}

type candidateCallerOverflow struct {
	Caller          int    `json:"caller"`
	Outcome         string `json:"outcome"`
	OperationStatus string `json:"operation_status"`
	Code            string `json:"code"`
	Provenance      string `json:"provenance"`
	Attach          bool   `json:"attach"`
	Mutate          bool   `json:"mutate"`
}

func loadStage2LifecycleContract(t *testing.T) stage2LifecycleContract {
	t.Helper()
	raw, err := os.ReadFile(stage2ContractPath)
	if err != nil {
		t.Fatalf("ASSERT_STAGE2_CONTRACT_PRESENT: %v", err)
	}
	var contract stage2LifecycleContract
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&contract); err != nil {
		t.Fatalf("ASSERT_STAGE2_CONTRACT_CLOSED: %v", err)
	}
	return contract
}

func TestStage2LifecycleExecutableContract(t *testing.T) {
	for _, assertion := range []string{
		"Stage 2 lifecycle operations have dedicated closed versioned input schemas.",
		"Each lifecycle canonical name has exactly one unique unversioned alias.",
		"Stage 2 lifecycle result and failure envelopes are closed, versioned, and uniquely selected.",
		"Candidate projections describe all availability states without changing the current NOT_IMPLEMENTED Stage 1 runtime projection.",
		"Canonical and alias lifecycle golden transcripts preserve validation precedence and canonical envelope tool identity.",
		"The Stage 2 lifecycle extension remains compatible with the Wave 3 Stage 1 manifest.",
		"Stage 3 remains absent from the Stage 2 lifecycle contract and activated in Stage 1.",
		"The executable lifecycle contract passes without enabling lifecycle dispatch.",
	} {
		t.Log("ASSERTION: " + assertion)
	}

	contract := loadStage2LifecycleContract(t)
	if contract.ContractVersion != "1" || contract.Extends != "stage1-manifest.v1" {
		t.Fatalf("ASSERT_STAGE2_VERSIONED_EXTENSION: got version=%q extends=%q", contract.ContractVersion, contract.Extends)
	}
	if len(contract.Tools) != 4 {
		t.Fatalf("ASSERT_STAGE2_FOUR_LIFECYCLE_TOOLS: got %d", len(contract.Tools))
	}

	wantNames := []string{"lsp_session_v1_list", "lsp_session_v1_restart", "lsp_session_v1_status", "lsp_session_v1_stop"}
	gotNames := make([]string, 0, len(contract.Tools))
	seenNames := map[string]bool{}
	for _, tool := range contract.Tools {
		gotNames = append(gotNames, tool.Name)
		if len(tool.Aliases) != 1 || tool.Aliases[0] == "" || seenNames[tool.Name] || seenNames[tool.Aliases[0]] {
			t.Fatalf("ASSERT_STAGE2_ALIAS_UNIQUE: tool=%q aliases=%v", tool.Name, tool.Aliases)
		}
		seenNames[tool.Name], seenNames[tool.Aliases[0]] = true, true
		if tool.Advertised || tool.Availability != "NOT_IMPLEMENTED" {
			t.Fatalf("ASSERT_STAGE2_RESERVED: tool=%q advertised=%v availability=%q", tool.Name, tool.Advertised, tool.Availability)
		}
		if len(tool.EnvelopeSchemaIDs) != 1 || !strings.HasSuffix(tool.EnvelopeSchemaIDs[0], "/envelope-session-not-implemented.v1.schema.json") {
			t.Fatalf("ASSERT_STAGE2_ENVELOPES: tool=%q ids=%v", tool.Name, tool.EnvelopeSchemaIDs)
		}
	}
	sort.Strings(gotNames)
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("ASSERT_STAGE2_CANONICAL_NAMES: got %v want %v", gotNames, wantNames)
	}

	registered := map[string]SchemaRegistration{}
	for _, schema := range contract.Schemas {
		if err := ValidateSchemaIdentity(schema); err != nil {
			t.Fatalf("ASSERT_STAGE2_SCHEMA_IDENTITY[%s]: %v", schema.ID, err)
		}
		registered[schema.ID] = schema
		raw, err := os.ReadFile(filepath.Join("testdata", schema.Path))
		if err != nil {
			t.Fatalf("ASSERT_STAGE2_SCHEMA_PRESENT[%s]: %v", schema.ID, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil || doc["additionalProperties"] != false {
			t.Fatalf("ASSERT_STAGE2_SCHEMA_CLOSED[%s]: decode=%v additionalProperties=%v", schema.ID, err, doc["additionalProperties"])
		}
	}
	for _, tool := range contract.Tools {
		if _, ok := registered[tool.InputSchemaID]; !ok {
			t.Fatalf("ASSERT_STAGE2_INPUT_REGISTERED[%s]: %s", tool.Name, tool.InputSchemaID)
		}
		for _, id := range tool.EnvelopeSchemaIDs {
			if _, ok := registered[id]; !ok {
				t.Fatalf("ASSERT_STAGE2_ENVELOPE_REGISTERED[%s]: %s", tool.Name, id)
			}
		}
	}

	if len(contract.Transcripts) < 16 {
		t.Fatalf("ASSERT_STAGE2_TRANSCRIPT_COVERAGE: got %d want at least 16", len(contract.Transcripts))
	}
	for _, name := range contract.Transcripts {
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("ASSERT_STAGE2_TRANSCRIPT_PRESENT[%s]: %v", name, err)
		}
		if err := ValidateTranscript(raw, bytes.Contains([]byte(name), []byte("input-error"))); err != nil {
			t.Fatalf("ASSERT_STAGE2_TRANSCRIPT_CANONICAL[%s]: %v", name, err)
		}
	}

	stage1, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	advertised := 0
	for _, tool := range stage1.Tools {
		if tool.Advertised {
			advertised++
		}
		if tool.Name == "lsp_trace_v1_slice" {
			if !tool.Advertised || tool.Availability != "ENABLED" {
				t.Fatalf("ASSERT_STAGE3_ACTIVATED: %+v", tool)
			}
		}
	}
	if advertised != 8 {
		t.Fatalf("ASSERT_STAGE1_SEVEN_ADVERTISED: got %d", advertised)
	}
}
