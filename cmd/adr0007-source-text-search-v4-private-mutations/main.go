package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Mut struct{ Name, Kind, Path, Old, New string }

const manifestPath = "docs/pilot/adr0007/source-text-search-v4/PRE_FREEZE_MANIFEST.json"

func main() {
	muts := []Mut{
		{"omit_file", "remove", "docs/pilot/adr0007/source-text-search-v4/contracts/CONTRACT.md", "", ""},
		{"add_file", "write", "docs/pilot/adr0007/source-text-search-v4/contracts/EXTRA.prefreeze-mutation", "", "extra\n"},
		{"implementation", "replace", "internal/adr0007sourcetextsearchv4private/search.go", "i++ {", "i += 2 {"},
		{"oracle_package", "append", "internal/adr0007sourcetextsearchv4oracle/oracle.go", "", "\n// mutation\n"},
		{"validator_package", "append", "internal/adr0007v4contractvalidator/validator.go", "", "\n// mutation\n"},
		{"expected", "append", "docs/pilot/adr0007/source-text-search-v4/oracle/case-01-raw_malformed_json.expected.json", "", "\n"},
		{"bundle", "append", "docs/pilot/adr0007/source-text-search-v4/oracle/case-01-raw_malformed_json.validator-bundle.json", "", "\n"},
		{"raw_case", "append", "docs/pilot/adr0007/source-text-search-v4/cases/case-01-raw_malformed_json/attempt.json", "", "\n"},
		{"pin", "append", "docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2/admission.go", "", "\n// mutation\n"},
		{"go_mod", "append", "go.mod", "", "\n// mutation\n"},
		{"predecessor", "append", "docs/pilot/adr0007/source-text-search-v4/FREEZE_DESIGN.md", "", "\nmutation\n"},
		{"manifest_role_swap", "replace", manifestPath, "\"role\": \"contract\"", "\"role\": \"role_swapped\""},
		{"symlink", "symlink", "docs/pilot/adr0007/source-text-search-v4/contracts/SYMLINK", "", "CONTRACT.md"},
		{"verifier_source", "append", "cmd/adr0007-source-text-search-v4-private-prefreeze/main.go", "", "\n// mutation\n"},
		{"outer_envelope_dryrun", "replace", "docs/pilot/adr0007/source-text-search-v4/FREEZE_ENVELOPE.dryrun.json", "DRYRUN_ONLY_NOT_FINAL_FREEZE", "MUTATED_DRYRUN"},
		{"metadata_stale_payload_commit", "json_set", manifestPath, "/payload_source_commit", "0000000000000000000000000000000000000000"},
		{"metadata_schema", "json_set", manifestPath, "/schema_version", "mutated-schema"},
		{"metadata_stage", "json_set", manifestPath, "/stage", "freeze"},
		{"metadata_allowed_root_add", "json_append", manifestPath, "/allowed_roots", "tmp/not-allowed/"},
		{"metadata_allowed_root_remove", "json_remove_index", manifestPath, "/allowed_roots", "0"},
		{"metadata_allowed_root_reorder", "json_swap", manifestPath, "/allowed_roots", "0,1"},
		{"metadata_exact_file", "json_append", manifestPath, "/allowed_exact_files", "go.work"},
		{"metadata_exclusion", "json_set", manifestPath, "/excluded_self", "docs/pilot/adr0007/source-text-search-v4/OTHER.json"},
		{"metadata_terminals_embed_true", "json_set", manifestPath, "/terminals_embed_outer_root", "true"},
		{"metadata_unknown_field", "json_unknown", manifestPath, "", "unexpected"},
		{"metadata_duplicate_field", "duplicate_field", manifestPath, "\"stage\": \"pre_freeze\"", "\"stage\": \"pre_freeze\",\n  \"stage\": \"pre_freeze\""},
		{"metadata_trailing_data", "append", manifestPath, "", "\n{}\n"},
	}
	pass := true
	for _, m := range muts {
		dir, err := os.MkdirTemp("", "adr0007-v4-prefreeze-mut-")
		if err != nil {
			panic(err)
		}
		if out, err := exec.Command("cp", "-R", ".", dir+"/repo").CombinedOutput(); err != nil {
			panic(string(out))
		}
		repo := filepath.Join(dir, "repo")
		if err := apply(repo, m); err != nil {
			fmt.Printf("%s mutation_setup_failed %v\n", m.Name, err)
			pass = false
			os.RemoveAll(dir)
			continue
		}
		c := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-prefreeze", "--mode", "verify")
		c.Dir = repo
		out, err := c.CombinedOutput()
		if err == nil {
			fmt.Printf("%s UNDETECTED\n", m.Name)
			pass = false
		} else {
			fmt.Printf("%s detected_nonzero %q\n", m.Name, firstLine(string(out)))
		}
		os.RemoveAll(dir)
	}
	if !pass {
		os.Exit(1)
	}
}

func apply(repo string, m Mut) error {
	p := filepath.Join(repo, m.Path)
	switch m.Kind {
	case "remove":
		return os.Remove(p)
	case "write":
		return os.WriteFile(p, []byte(m.New), 0644)
	case "append":
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(p, append(b, []byte(m.New)...), 0644)
	case "replace", "duplicate_field":
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s := string(b)
		if !strings.Contains(s, m.Old) {
			return fmt.Errorf("old text missing")
		}
		return os.WriteFile(p, []byte(strings.Replace(s, m.Old, m.New, 1)), 0644)
	case "symlink":
		return os.Symlink(m.New, p)
	case "json_set", "json_append", "json_remove_index", "json_swap", "json_unknown":
		return mutateJSON(p, m)
	default:
		return fmt.Errorf("unknown kind %s", m.Kind)
	}
}

func mutateJSON(path string, m Mut) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return err
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return fmt.Errorf("root JSON is not object")
	}
	switch m.Kind {
	case "json_unknown":
		obj[m.New] = true
	case "json_set":
		key := strings.TrimPrefix(m.Old, "/")
		if m.New == "true" {
			obj[key] = true
		} else {
			obj[key] = m.New
		}
	case "json_append":
		key := strings.TrimPrefix(m.Old, "/")
		a, ok := obj[key].([]any)
		if !ok {
			return fmt.Errorf("%s is not array", key)
		}
		obj[key] = append(a, m.New)
	case "json_remove_index":
		key := strings.TrimPrefix(m.Old, "/")
		a, ok := obj[key].([]any)
		if !ok {
			return fmt.Errorf("%s is not array", key)
		}
		obj[key] = a[1:]
	case "json_swap":
		key := strings.TrimPrefix(m.Old, "/")
		a, ok := obj[key].([]any)
		if !ok {
			return fmt.Errorf("%s is not array", key)
		}
		a[0], a[1] = a[1], a[0]
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0644)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
