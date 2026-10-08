package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	oracle "lsp-trace/internal/adr0007locationoraclev5"
)

type manifestFile struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type manifest struct {
	Schema       string         `json:"schema"`
	CaseCount    int            `json:"caseCount"`
	Cases        []string       `json:"cases"`
	Files        []manifestFile `json:"files"`
	BoundaryBase struct {
		Work        uint64 `json:"work"`
		OutputBytes int    `json:"outputBytes"`
	} `json:"boundaryBase"`
}
type derivation struct {
	Schema    string   `json:"schema"`
	CaseID    string   `json:"caseId"`
	Inputs    []string `json:"inputs"`
	Algorithm string   `json:"algorithm"`
	Outcome   string   `json:"outcome"`
	Detail    string   `json:"detail"`
}

func write(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
func canonical(v any) []byte { b, _ := json.Marshal(v); return append(b, '\n') }
func evaluate(dir string, l oracle.Limits) (oracle.Result, error) {
	raw, e := os.ReadFile(filepath.Join(dir, "REQUEST.raw.json"))
	if e != nil {
		return oracle.Result{}, e
	}
	var c oracle.Condition
	b, e := os.ReadFile(filepath.Join(dir, "CONDITION.json"))
	if e != nil {
		return oracle.Result{}, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return oracle.Result{}, e
	}
	binding, e := os.ReadFile(filepath.Join(dir, "BINDING.json"))
	present := e == nil
	return oracle.Evaluate(raw, binding, present, c, l), nil
}
func main() {
	spec := flag.String("spec", "", "prospective-v5 root")
	out := flag.String("out", "", "oracle-candidate root")
	flag.Parse()
	if *spec == "" || *out == "" {
		panic("-spec and -out required")
	}
	entries, e := os.ReadDir(filepath.Join(*spec, "inputs"))
	if e != nil {
		panic(e)
	}
	m := manifest{Schema: "lsp-trace.adr0007.location-oracle-candidate-manifest.private.v5"}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		r, e := evaluate(filepath.Join(*spec, "inputs", id), oracle.PublishedLimits())
		if e != nil {
			panic(e)
		}
		rp := filepath.Join(*out, "cases", id, "RESULT.json")
		if e = write(rp, oracle.Canonical(r)); e != nil {
			panic(e)
		}
		d := derivation{"lsp-trace.adr0007.location-oracle-derivation.private.v5", id, []string{"REQUEST.raw.json", "BINDING.json|BINDING.ABSENT", "CONDITION.json"}, "internal/adr0007locationoraclev5", r.Outcome, r.Detail}
		if e = write(filepath.Join(*out, "cases", id, "DERIVATION.json"), canonical(d)); e != nil {
			panic(e)
		}
		m.Cases = append(m.Cases, id)
	}
	if len(m.Cases) != 26 {
		panic(fmt.Sprintf("case count %d", len(m.Cases)))
	}
	m.CaseCount = len(m.Cases)
	baseDir := filepath.Join(*spec, "inputs", "01-exact-intersects")
	base, e := evaluate(baseDir, oracle.PublishedLimits())
	if e != nil || base.Outcome != "COMPLETE" {
		panic("base case not complete")
	}
	m.BoundaryBase.Work = base.Counters.Work
	m.BoundaryBase.OutputBytes = base.Counters.OutputBytes
	variants := []struct {
		Name string
		L    oracle.Limits
	}{{"W", oracle.PublishedLimits()}, {"W-1", oracle.PublishedLimits()}, {"B", oracle.PublishedLimits()}, {"B-1", oracle.PublishedLimits()}}
	variants[0].L.MaxWork = base.Counters.Work
	variants[1].L.MaxWork = base.Counters.Work - 1
	variants[2].L.MaxOutputBytes = base.Counters.OutputBytes
	variants[3].L.MaxOutputBytes = base.Counters.OutputBytes - 1
	raw, _ := os.ReadFile(filepath.Join(baseDir, "REQUEST.raw.json"))
	binding, _ := os.ReadFile(filepath.Join(baseDir, "BINDING.json"))
	cond, _ := os.ReadFile(filepath.Join(baseDir, "CONDITION.json"))
	for _, v := range variants {
		bd := filepath.Join(*out, "boundaries", v.Name)
		_ = write(filepath.Join(bd, "REQUEST.raw.json"), raw)
		_ = write(filepath.Join(bd, "BINDING.json"), binding)
		_ = write(filepath.Join(bd, "CONDITION.json"), cond)
		input := map[string]any{"schema": "lsp-trace.adr0007.location-boundary-input.private.v5", "variant": v.Name, "baseWork": base.Counters.Work, "baseOutputBytes": base.Counters.OutputBytes, "maxWork": v.L.MaxWork, "maxOutputBytes": v.L.MaxOutputBytes}
		_ = write(filepath.Join(bd, "BOUNDARY.json"), canonical(input))
		r, e := evaluate(baseDir, v.L)
		if e != nil {
			panic(e)
		}
		_ = write(filepath.Join(bd, "RESULT.json"), oracle.Canonical(r))
	}
	var paths []string
	_ = filepath.Walk(*out, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(p) != "MANIFEST.json" {
			paths = append(paths, p)
		}
		return err
	})
	sort.Strings(paths)
	for _, p := range paths {
		b, _ := os.ReadFile(p)
		h := sha256.Sum256(b)
		rel, _ := filepath.Rel(*out, p)
		m.Files = append(m.Files, manifestFile{filepath.ToSlash(rel), len(b), "sha256:" + hex.EncodeToString(h[:])})
	}
	_ = write(filepath.Join(*out, "MANIFEST.json"), canonical(m))
	fmt.Printf("ORACLE_OUTPUTS_WRITTEN cases=%d work=%d outputBytes=%d\n", len(m.Cases), base.Counters.Work, base.Counters.OutputBytes)
}
