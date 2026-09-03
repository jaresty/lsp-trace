package stage2scenario

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed testdata/schemas/*.json
var candidateSchemas embed.FS

const (
	ScenarioSchemaID = ScenarioVersion
	LedgerSchemaID   = LedgerVersion
)

var (
	schemaOnce sync.Once
	schemaSet  map[string]*jsonschema.Schema
	schemaErr  error
)

func candidateSchema(id string) (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		schemaSet = map[string]*jsonschema.Schema{}
		compiler := jsonschema.NewCompiler()
		compiler.DefaultDraft(jsonschema.Draft2020)
		entries, err := fs.ReadDir(candidateSchemas, "testdata/schemas")
		if err != nil {
			schemaErr = err
			return
		}
		for _, entry := range entries {
			raw, err := candidateSchemas.ReadFile("testdata/schemas/" + entry.Name())
			if err != nil {
				schemaErr = err
				return
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				schemaErr = err
				return
			}
			var header struct {
				ID string `json:"$id"`
			}
			if err := json.Unmarshal(raw, &header); err != nil || header.ID == "" {
				schemaErr = fmt.Errorf("candidate schema %s has no $id", entry.Name())
				return
			}
			if err := compiler.AddResource(header.ID, doc); err != nil {
				schemaErr = err
				return
			}
		}
		for _, id := range []string{ScenarioSchemaID, LedgerSchemaID, "stage2-catalog-v1", "stage2-replacements-v1"} {
			schemaSet[id], err = compiler.Compile(id)
			if err != nil {
				schemaErr = err
				return
			}
		}
	})
	if schemaErr != nil {
		return nil, schemaErr
	}
	s := schemaSet[id]
	if s == nil {
		return nil, fmt.Errorf("unknown candidate schema %q", id)
	}
	return s, nil
}

func validateJSON(id string, raw []byte) error {
	schema, err := candidateSchema(id)
	if err != nil {
		return err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("structural %s: %w", id, err)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("structural %s: %w", id, err)
	}
	return nil
}

func ValidateScenarioStructure(raw []byte) error { return validateJSON(ScenarioSchemaID, raw) }

func ValidateLedgerJSONL(raw []byte) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return fmt.Errorf("ledger must be nonempty and LF-terminated")
	}
	for n, line := range bytes.Split(raw[:len(raw)-1], []byte("\n")) {
		if len(line) == 0 {
			return fmt.Errorf("ledger line %d is empty", n+1)
		}
		if err := validateJSON(LedgerSchemaID, line); err != nil {
			return fmt.Errorf("ledger line %d: %w", n+1, err)
		}
	}
	return nil
}

type corpusCatalog struct {
	Version         string `json:"version"`
	AcceptanceState string `json:"acceptance_state"`
	NonCorpusFiles  []struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	} `json:"non_corpus_files"`
	Rows []struct {
		ID, Scenario, Golden, Evidence string
		Coverage                       []string `json:"coverage"`
	} `json:"rows"`
	Coverage map[string]struct{ Status, Guard, Gate string } `json:"coverage"`
}

func ValidateCorpus(root string) error {
	catalogRaw, err := os.ReadFile(filepath.Join(root, "catalog.v1.json"))
	if err != nil {
		return err
	}
	if err := validateJSON("stage2-catalog-v1", catalogRaw); err != nil {
		return err
	}
	replacementsRaw, err := os.ReadFile(filepath.Join(root, "replacements.v1.json"))
	if err != nil {
		return err
	}
	if err := validateJSON("stage2-replacements-v1", replacementsRaw); err != nil {
		return err
	}
	var catalog corpusCatalog
	if err := json.Unmarshal(catalogRaw, &catalog); err != nil {
		return err
	}
	seenScenario, seenGolden, seenRow := map[string]bool{}, map[string]bool{}, map[string]bool{}
	allowedRoot := map[string]bool{"catalog.v1.json": true, "replacements.v1.json": true}
	for _, item := range catalog.NonCorpusFiles {
		if allowedRoot[item.Path] || item.Reason == "" {
			return fmt.Errorf("invalid non-corpus declaration %q", item.Path)
		}
		if _, err := os.Stat(filepath.Join(root, item.Path)); err != nil {
			return fmt.Errorf("non-corpus file %q: %w", item.Path, err)
		}
		allowedRoot[item.Path] = true
	}
	for _, row := range catalog.Rows {
		if seenRow[row.ID] {
			return fmt.Errorf("duplicate catalog row %q", row.ID)
		}
		seenRow[row.ID] = true
		if row.Evidence != "reference" {
			return fmt.Errorf("row %q has noncanonical evidence %q", row.ID, row.Evidence)
		}
		seenScenario[row.Scenario], seenGolden[row.Golden] = true, true
		allowedRoot[row.Scenario] = true
		scenarioRaw, err := os.ReadFile(filepath.Join(root, row.Scenario))
		if err != nil {
			return err
		}
		if err := ValidateScenarioStructure(scenarioRaw); err != nil {
			return fmt.Errorf("row %q: %w", row.ID, err)
		}
		scenario, err := Parse(scenarioRaw)
		if err != nil {
			return fmt.Errorf("row %q semantic: %w", row.ID, err)
		}
		ledger, err := ReplayIntegrated(scenario)
		if err != nil {
			return fmt.Errorf("row %q replay: %w", row.ID, err)
		}
		golden, err := os.ReadFile(filepath.Join(root, row.Golden))
		if err != nil {
			return err
		}
		if err := ValidateLedgerJSONL(golden); err != nil {
			return fmt.Errorf("row %q: %w", row.ID, err)
		}
		if !bytes.Equal(ledger.Intent, golden) {
			return fmt.Errorf("row %q golden byte mismatch", row.ID)
		}
		for _, coverage := range row.Coverage {
			if _, ok := catalog.Coverage[coverage]; !ok {
				return fmt.Errorf("row %q has unknown coverage %q", row.ID, coverage)
			}
		}
	}
	hasAcceptanceBlocker := false
	for name, item := range catalog.Coverage {
		if item.Status != "covered" && item.Status != "guarded" && item.Status != "partial" && item.Status != "runtime_gate" {
			return fmt.Errorf("coverage %q has noncanonical status %q", name, item.Status)
		}
		if item.Status == "runtime_gate" && item.Gate != "RUNTIME_GATE_PRODUCTION_EQUIVALENT_FAKE_SERVER_PATH" {
			return fmt.Errorf("coverage %q has noncanonical gate %q", name, item.Gate)
		}
		if item.Status == "partial" || item.Status == "runtime_gate" {
			hasAcceptanceBlocker = true
		}
	}
	if hasAcceptanceBlocker && catalog.AcceptanceState != "open" {
		return fmt.Errorf("acceptance_state must remain open while partial or runtime_gate coverage exists")
	}
	checkFiles := func(pattern string, seen map[string]bool, trim func(string) string) error {
		paths, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return err
		}
		sort.Strings(paths)
		for _, path := range paths {
			name := trim(path)
			if !seen[name] {
				return fmt.Errorf("silent corpus file %q", name)
			}
		}
		return nil
	}
	rootJSON, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(rootJSON)
	for _, path := range rootJSON {
		name := filepath.Base(path)
		if !allowedRoot[name] {
			return fmt.Errorf("silent corpus file %q", name)
		}
	}
	return checkFiles(filepath.Join("goldens", "*.ledger.jsonl"), seenGolden, func(path string) string {
		return filepath.ToSlash(filepath.Join("goldens", filepath.Base(path)))
	})
}

// IntegratedLedgersCompatibility documents that the legacy ledger-named fields are
// compatibility aliases of one combined ledger, not independent evidence streams.
const IntegratedLedgersCompatibility = "Ownership, Intent, TerminalHistory, and ResourceCensus are compatibility aliases of one combined ledger"
