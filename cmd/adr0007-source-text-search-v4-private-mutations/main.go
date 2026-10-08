package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Mut struct{ Name, Path, Old, New, CaseID, Status string }

func main() {
	muts := []Mut{
		{"overlap_stride_plus_2", "internal/adr0007sourcetextsearchv4private/search.go", "i++ {", "i += 2 {", "case-25-overlap_literal", "RUN"},
		{"utf16_nonbmp_unit_1", "internal/adr0007sourcetextsearchv4private/search.go", "u += 2", "u += 1", "case-33-non_bmp_utf16", "RUN"},
		{"deadline_cancel_order_swap", "internal/adr0007sourcetextsearchv4private/search.go", "if o.DeadlineExpired {\n\t\t\treturn \"DEADLINE_EXCEEDED\", true\n\t\t}\n\t\tif o.Cancelled {", "if o.Cancelled {\n\t\t\treturn \"CANCELLED\", true\n\t\t}\n\t\tif o.DeadlineExpired {", "case-09-control_simultaneous_deadline_cancel", "RUN"},
		{"accounting_coefficient", "internal/adr0007sourcetextsearchv4private/search.go", "{31, a.BOutputBytes}", "{29, a.BOutputBytes}", "case-29-lf_positions", "RUN"},
		{"limit_ge", "internal/adr0007sourcetextsearchv4private/search.go", "acc.MMatches+1 > a.Request.Limits.MaxMatches", "acc.MMatches+1 >= a.Request.Limits.MaxMatches", "case-43-max_matches_equal", "RUN"},
		{"admission_bypass", "internal/adr0007sourcetextsearchv4private/search.go", "if ar.Outcome != admit.Complete {", "if false && ar.Outcome != admit.Complete {", "case-21-file_digest_mutation", "RUN"},
		{"range_member_reorder", "internal/adr0007sourcetextsearchv4private/search.go", "c.MemberMatchIDs = append(c.MemberMatchIDs, m.MatchID)", "c.MemberMatchIDs = append(c.MemberMatchIDs, \"mutated-\"+m.MatchID)", "case-24-multi_path_byte_order", "RUN"},
		{"future_oracle_placeholder", "cmd/adr0007-source-text-search-v4-private-oracle/main.go", "placeholder", "mutated", "", "NOT_RUN_EXPECTED"},
		{"future_verifier_placeholder", "cmd/adr0007-source-text-search-v4-private-verify/main.go", "placeholder", "mutated", "", "NOT_RUN_EXPECTED"},
		{"freeze_envelope_dryrun", "docs/pilot/adr0007/source-text-search-v4/FREEZE_ENVELOPE.dryrun.json", "placeholder", "mutated", "", "NOT_RUN_EXPECTED"},
	}
	pass := true
	for _, m := range muts {
		if m.Status != "RUN" {
			fmt.Printf("%s status=%s\n", m.Name, m.Status)
			continue
		}
		attempt := filepath.Join("docs/pilot/adr0007/source-text-search-v4/cases", m.CaseID, "attempt.json")
		baseline, err := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-evaluate", attempt).Output()
		if err != nil {
			panic(err)
		}
		dir, err := os.MkdirTemp("", "adr0007-v4-mut-")
		if err != nil {
			panic(err)
		}
		if out, err := exec.Command("cp", "-R", ".", dir+"/repo").CombinedOutput(); err != nil {
			panic(string(out))
		}
		p := filepath.Join(dir, "repo", m.Path)
		b, err := os.ReadFile(p)
		if err != nil {
			panic(err)
		}
		s := string(b)
		if !strings.Contains(s, m.Old) {
			fmt.Printf("%s mutation_text_missing\n", m.Name)
			pass = false
			os.RemoveAll(dir)
			continue
		}
		os.WriteFile(p, []byte(strings.Replace(s, m.Old, m.New, 1)), 0644)
		c := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-evaluate", attempt)
		c.Dir = filepath.Join(dir, "repo")
		mutated, err := c.Output()
		if err != nil {
			fmt.Printf("%s detected_process_nonzero\n", m.Name)
			os.RemoveAll(dir)
			continue
		}
		if bytes.Equal(mutated, baseline) {
			fmt.Printf("%s UNDETECTED\n", m.Name)
			pass = false
		} else {
			fmt.Printf("%s detected_exact_difference\n", m.Name)
		}
		os.RemoveAll(dir)
	}
	if !pass {
		os.Exit(1)
	}
}
