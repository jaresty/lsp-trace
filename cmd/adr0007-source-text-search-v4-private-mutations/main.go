package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Mut struct{ Name, Kind, Path, Old, New string }

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
		{"manifest_role_swap", "replace", "docs/pilot/adr0007/source-text-search-v4/PRE_FREEZE_MANIFEST.json", "\"role\": \"contract\"", "\"role\": \"role_swapped\""},
		{"symlink", "symlink", "docs/pilot/adr0007/source-text-search-v4/contracts/SYMLINK", "", "CONTRACT.md"},
		{"verifier_source", "append", "cmd/adr0007-source-text-search-v4-private-prefreeze/main.go", "", "\n// mutation\n"},
		{"outer_envelope_dryrun", "replace", "docs/pilot/adr0007/source-text-search-v4/FREEZE_ENVELOPE.dryrun.json", "DRYRUN_ONLY_NOT_FINAL_FREEZE", "MUTATED_DRYRUN"},
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
	case "replace":
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
	default:
		return fmt.Errorf("unknown kind %s", m.Kind)
	}
}
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
