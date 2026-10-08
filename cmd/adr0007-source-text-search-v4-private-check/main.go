package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	admit "lsp-trace/docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2"
	v4 "lsp-trace/internal/adr0007sourcetextsearchv4private"
	"lsp-trace/internal/adr0007v4contractvalidator"
)

type Row struct {
	ID              string `json:"id"`
	Requirement     string `json:"requirement"`
	Stimulus        string `json:"stimulus"`
	Assertion       string `json:"assertion"`
	ExpectedOutcome string `json:"expected_outcome"`
	ExpectedCode    string `json:"expected_code"`
}

type bundle struct {
	SchemaBytes              json.RawMessage   `json:"schema_bytes"`
	RawAttemptBytes          string            `json:"raw_attempt_bytes"`
	TerminalBytes            string            `json:"terminal_bytes"`
	AdmittedSourceBytes      map[string]string `json:"admitted_source_bytes"`
	AdmittedBindingBytes     string            `json:"admitted_binding_bytes"`
	ToolingManifestBytes     string            `json:"tooling_manifest_bytes"`
	PredecessorManifestBytes string            `json:"predecessor_manifest_bytes"`
	PayloadFreezeBytes       string            `json:"payload_freeze_binding_bytes"`
}

func main() {
	root := "docs/pilot/adr0007/source-text-search-v4/cases"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	rows := readRows(root)
	dirs := map[string]bool{}
	entries, err := os.ReadDir(root)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			fail("symlink in case root: " + e.Name())
		}
		if e.IsDir() {
			dirs[e.Name()] = true
		}
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.ID] {
			fail("duplicate id " + r.ID)
		}
		seen[r.ID] = true
		ids = append(ids, r.ID)
		if !dirs[r.ID] {
			fail("missing case dir " + r.ID)
		}
		validateRow(root, r)
	}
	for d := range dirs {
		if !seen[d] {
			fail("extra case dir " + d)
		}
	}
	sort.Strings(ids)
	bundleDir := os.Getenv("ADR0007_V4_BUNDLE_DIR")
	if bundleDir != "" {
		if err := os.MkdirAll(bundleDir, 0755); err != nil {
			panic(err)
		}
	}
	complete, failed, contractOK := 0, 0, 0
	for _, id := range ids {
		r := find(rows, id)
		attemptPath := filepath.Join(root, id, "attempt.json")
		out, err := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-evaluate", attemptPath).Output()
		if err != nil {
			fail(fmt.Sprintf("%s process error: %v", id, err))
		}
		var tr v4.TerminalResult
		if err := json.Unmarshal(out, &tr); err != nil {
			fail(id + " invalid terminal json")
		}
		code := ""
		if tr.Failure != nil {
			code = tr.Failure.Code
		}
		if tr.Terminal != r.ExpectedOutcome || code != r.ExpectedCode {
			fail(fmt.Sprintf("%s expected %s/%s got %s/%s", id, r.ExpectedOutcome, r.ExpectedCode, tr.Terminal, code))
		}
		if tr.Replay.ToolingIdentitySHA256 != v4.ToolDigest() || tr.Replay.PredecessorLockSHA256 != v4.PredecessorDigest() {
			fail(id + " replay manifest digest mismatch")
		}
		bun := buildBundle(attemptPath, string(out), tr)
		bb, _ := json.Marshal(bun)
		if bundleDir != "" {
			if err := os.WriteFile(filepath.Join(bundleDir, id+".bundle.json"), bb, 0644); err != nil {
				panic(err)
			}
		}
		if err := adr0007v4contractvalidator.ValidateBundleBytes(bb); err != nil {
			fail(fmt.Sprintf("%s contract validation failed: %v", id, err))
		}
		contractOK++
		if tr.Terminal == "COMPLETE" {
			complete++
		} else {
			failed++
		}
		fmt.Printf("%s stimulus=%s assertion=%s outcome=%s code=%s contract=ok\n", id, r.Stimulus, r.Assertion, tr.Terminal, code)
	}
	fmt.Printf("SUMMARY cases=%d complete=%d failed=%d contract_validations=%d\n", len(rows), complete, failed, contractOK)
}

func buildBundle(attemptPath, terminal string, tr v4.TerminalResult) bundle {
	raw, err := os.ReadFile(attemptPath)
	if err != nil {
		panic(err)
	}
	schema, err := os.ReadFile("docs/pilot/adr0007/source-text-search-v4/contracts/terminal-envelope-v4.schema.json")
	if err != nil {
		panic(err)
	}
	toolingBytes, err := os.ReadFile("docs/pilot/adr0007/source-text-search-v4/TOOLING_CENSUS.json")
	if err != nil {
		panic(err)
	}
	predBytes, err := os.ReadFile("docs/pilot/adr0007/source-text-search-v4/PAYLOAD_MANIFEST.json")
	if err != nil {
		panic(err)
	}
	tooling := string(toolingBytes)
	pred := string(predBytes)
	freeze := "UNFROZEN-PROSPECTIVE-V4-DESIGN-IDENTITY"
	admitted := map[string]string{}
	binding := ""
	if tr.Admission.Completed {
		a, perr := v4.StrictParseAttempt(raw)
		if perr != "" {
			panic(perr)
		}
		inputs := map[uint64]v4.SourceInput{}
		for _, in := range a.SourceInputs {
			inputs[in.Ordinal] = in
		}
		adm := v4.AdmissionRecord{SchemaVersion: v4.AdmissionRecordSchema, AdmissionSchema: admit.Schema, OrderedSources: []v4.SourceTuple{}}
		selected := []admit.SelectedSource{}
		for _, s := range tr.Sources {
			in := inputs[s.Ordinal]
			data, err := base64.StdEncoding.DecodeString(in.BytesBase64)
			if err != nil {
				panic(err)
			}
			admitted[s.SourceID] = string(data)
			selected = append(selected, admit.SelectedSource{Path: in.Path, Revision: in.Revision, FileDigest: in.FileDigest, ObjectDigest: in.ObjectDigest, Bytes: data})
			adm.OrderedSources = append(adm.OrderedSources, v4.SourceTuple{Ordinal: s.Ordinal, Path: in.Path, Revision: in.Revision, FileDigest: in.FileDigest, ObjectDigest: in.ObjectDigest, SourceByteLength: uint64(len(data))})
		}
		ar := admit.Admit(selected, admit.Limits{MaxSources: len(selected), MaxSourceBytes: int(^uint(0) >> 1), MaxTotalBytes: int(^uint(0) >> 1)})
		adm.AdmissionDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		if ar.Binding != nil {
			adm.AdmissionDigest = ar.Binding.AdmissionDigest
		}
		binding = string(v4.Canon(adm))
	}
	return bundle{SchemaBytes: schema, RawAttemptBytes: string(raw), TerminalBytes: terminal, AdmittedSourceBytes: admitted, AdmittedBindingBytes: binding, ToolingManifestBytes: tooling, PredecessorManifestBytes: pred, PayloadFreezeBytes: freeze}
}

func readRows(root string) []Row {
	b, err := os.ReadFile(filepath.Join(root, "CASE_MATRIX.json"))
	if err != nil {
		panic(err)
	}
	var rows []Row
	if err := json.Unmarshal(b, &rows); err != nil {
		panic(err)
	}
	return rows
}
func find(rows []Row, id string) Row {
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	panic(id)
}
func validateRow(root string, r Row) {
	if r.ID == "" || r.Requirement == "" || r.Stimulus == "" || r.Assertion == "" || r.ExpectedOutcome == "" {
		fail("incomplete row " + r.ID)
	}
	if r.ExpectedOutcome != "COMPLETE" && r.ExpectedOutcome != "FAILED" {
		fail("bad outcome " + r.ID)
	}
	if r.ExpectedOutcome == "COMPLETE" && r.ExpectedCode != "" {
		fail("complete with code " + r.ID)
	}
	if r.ExpectedOutcome == "FAILED" && r.ExpectedCode == "" {
		fail("failed missing code " + r.ID)
	}
	dir := filepath.Join(root, r.ID)
	mustFile(filepath.Join(dir, "attempt.json"))
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			fail(err.Error())
		}
		if d.Type()&os.ModeSymlink != 0 {
			fail("symlink " + p)
		}
		return nil
	})
}
func mustFile(p string) {
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() {
		fail("missing file " + p)
	}
}
func fail(s string) { fmt.Fprintln(os.Stderr, s); os.Exit(1) }
