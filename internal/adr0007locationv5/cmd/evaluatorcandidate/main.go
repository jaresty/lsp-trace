package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	v5 "lsp-trace/internal/adr0007locationv5"
)

type condition struct {
	Cancel   bool `json:"cancel"`
	Deadline bool `json:"deadlineExpired"`
}
type entry struct {
	CaseID              string `json:"caseId"`
	ResultPath          string `json:"resultPath"`
	ResultSHA256        string `json:"resultSHA256"`
	RequestSHA256       string `json:"requestSHA256"`
	BindingSHA256       string `json:"bindingSHA256"`
	MeasuredWork        uint64 `json:"measuredWork"`
	MeasuredOutputBytes uint64 `json:"measuredOutputBytes"`
}
type manifest struct {
	Schema                 string   `json:"schema"`
	Producer               string   `json:"producer"`
	Status                 string   `json:"status"`
	Cases                  []entry  `json:"cases"`
	MaxMeasuredWork        uint64   `json:"maxMeasuredWork"`
	MaxMeasuredOutputBytes uint64   `json:"maxMeasuredOutputBytes"`
	Variants               []string `json:"variants"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	in := flag.String("inputs", "", "immutable approved26 input root")
	out := flag.String("out", "", "new evaluator candidate output root")
	flag.Parse()
	if *in == "" || *out == "" {
		panic("-inputs and -out required")
	}
	if _, e := os.Stat(*out); !os.IsNotExist(e) {
		panic("output root must not exist")
	}
	dirs, e := os.ReadDir(*in)
	must(e)
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	m := manifest{Schema: "lsp-trace.adr0007.location-evaluator-candidate-manifest.private.v5", Producer: "internal/adr0007locationv5", Status: "PRODUCER_SPECIFIC_UNACCEPTED"}
	lim := v5.PublishedLimits()
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		caseID := d.Name()
		base := filepath.Join(*in, caseID)
		raw, e := os.ReadFile(filepath.Join(base, "REQUEST.raw.json"))
		must(e)
		bindPath := filepath.Join(base, "BINDING.json")
		bind, e := os.ReadFile(bindPath)
		if os.IsNotExist(e) {
			bind = nil
		} else {
			must(e)
		}
		var c condition
		cb, e := os.ReadFile(filepath.Join(base, "CONDITION.json"))
		must(e)
		must(json.Unmarshal(cb, &c))
		r, e := v5.Evaluate(raw, bind, v5.StaticControl{Cancel: c.Cancel, Deadline: c.Deadline}, lim)
		must(e)
		b, e := v5.Canonical(r)
		must(e)
		p := filepath.Join(*out, "cases", caseID, "RESULT.json")
		must(os.MkdirAll(filepath.Dir(p), 0755))
		must(os.WriteFile(p, b, 0644))
		en := entry{caseID, filepath.ToSlash(filepath.Join("cases", caseID, "RESULT.json")), v5.SHA256(b), v5.SHA256(raw), v5.SHA256(bind), r.Counters.Work, r.Counters.OutputBytes}
		if en.MeasuredWork > m.MaxMeasuredWork {
			m.MaxMeasuredWork = en.MeasuredWork
		}
		if en.MeasuredOutputBytes > m.MaxMeasuredOutputBytes {
			m.MaxMeasuredOutputBytes = en.MeasuredOutputBytes
		}
		m.Cases = append(m.Cases, en)
	}
	vars := []struct {
		Name        string
		Work, Bytes uint64
	}{{"work-at-measured", m.MaxMeasuredWork, m.MaxMeasuredOutputBytes}, {"work-plus-one-below", m.MaxMeasuredWork - 1, m.MaxMeasuredOutputBytes}, {"output-at-measured", m.MaxMeasuredWork, m.MaxMeasuredOutputBytes}, {"output-plus-one-below", m.MaxMeasuredWork, m.MaxMeasuredOutputBytes - 1}}
	for _, x := range vars {
		p := filepath.Join(*out, "provisional-boundary-input-variants", x.Name+".json")
		must(os.MkdirAll(filepath.Dir(p), 0755))
		b, _ := json.Marshal(struct {
			Schema         string `json:"schema"`
			Producer       string `json:"producer"`
			Status         string `json:"status"`
			Boundary       string `json:"boundary"`
			MaxWork        uint64 `json:"maxWork"`
			MaxOutputBytes uint64 `json:"maxOutputBytes"`
			ObservedW      uint64 `json:"observedW"`
			ObservedB      uint64 `json:"observedB"`
		}{"lsp-trace.adr0007.location-boundary-input-variant.private.v5", "internal/adr0007locationv5", "PRODUCER_SPECIFIC_UNACCEPTED", x.Name, x.Work, x.Bytes, m.MaxMeasuredWork, m.MaxMeasuredOutputBytes})
		b = append(b, '\n')
		must(os.WriteFile(p, b, 0644))
		m.Variants = append(m.Variants, filepath.ToSlash(p[len(*out)+1:]))
	}
	mb, _ := json.Marshal(m)
	mb = append(mb, '\n')
	must(os.WriteFile(filepath.Join(*out, "MANIFEST.json"), mb, 0644))
	fmt.Printf("EVALUATOR_CANDIDATE_WRITTEN cases=%d maxW=%d maxB=%d\n", len(m.Cases), m.MaxMeasuredWork, m.MaxMeasuredOutputBytes)
}
