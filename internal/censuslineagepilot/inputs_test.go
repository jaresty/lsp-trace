package censuslineagepilot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
)

func inputFiles() []censusacquisition.SourceFile {
	return []censusacquisition.SourceFile{{Path: "b.go", URI: "file:///work/b.go", LanguageID: "go"}, {Path: "a.go", URI: "file:///work/a.go", LanguageID: "go"}}
}
func inputFixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	s, err := NewSourceSelection("/work", inputFiles())
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPartitionPlan(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := EncodeSourceSelection(s)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := EncodePartitionPlan(p, s)
	if err != nil {
		t.Fatal(err)
	}
	return sb, pb
}
func TestInputCanonicalAndBuild(t *testing.T) {
	sb, pb := inputFixture(t)
	const golden = `{"schema_version":"lsp-trace.census-lineage-pilot-source-selection.private.v1","authority":0,"accepted":false,"workspace":"/work","files":[{"path":"a.go","uri":"file:///work/a.go","language_id":"go","disposition":""},{"path":"b.go","uri":"file:///work/b.go","language_id":"go","disposition":""}]}`
	if string(sb) != golden {
		t.Fatalf("selection golden: %s", sb)
	}
	s, err := ParseSourceSelection(sb)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParsePartitionPlan(pb, s)
	if err != nil {
		t.Fatal(err)
	}
	built, err := censussourceplan.Build(inputFiles(), "/work", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.SourcePlan(), built) {
		t.Fatal("not exact Build reconstruction")
	}
	encoded, _ := EncodePartitionPlan(p, s)
	if !bytes.Equal(encoded, pb) {
		t.Fatal("plan roundtrip")
	}
	const planGolden = `{"schema_version":"lsp-trace.census-lineage-pilot-partition-plan.private.v1","authority":0,"accepted":false,"source_selection_sha256":"sha256:481b00ace62144a646c2fade00b87a674fa15ab7ccb4e22d52a5a1f3110b740c","source_selection_byte_length":296,"partition_size":1,"plan":{"identity":"sha256:f45eed7408c4b5546f7a0b8609a0916432eb6c58aed3ccc4b93d160103cfbe88","authority":0,"files":[{"path":"a.go","uri":"file:///work/a.go","language_id":"go","disposition":""},{"path":"b.go","uri":"file:///work/b.go","language_id":"go","disposition":""}],"partitions":[{"ordinal":0,"members":[{"path":"a.go","uri":"file:///work/a.go","language_id":"go","disposition":""}]},{"ordinal":1,"members":[{"path":"b.go","uri":"file:///work/b.go","language_id":"go","disposition":""}]}]}}`
	if string(pb) != planGolden {
		t.Fatalf("plan byte golden: %s", pb)
	}
	const planDigest = "sha256:c87656d68e0b2437aff601c0165c026d203b8aae4a66dcb22113f9f7831b3561"
	if Digest(pb) != planDigest {
		t.Fatalf("plan golden digest: %s; bytes: %s", Digest(pb), pb)
	}
}
func TestInputRejectSelection(t *testing.T) {
	sb, _ := inputFixture(t)
	cases := map[string][]byte{"nil": nil, "empty": {}, "null": []byte("null"), "trailing": append(append([]byte{}, sb...), []byte("{}")...), "space": append([]byte(" "), sb...), "duplicate": bytes.Replace(sb, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1), "unknown": bytes.Replace(sb, []byte(`"authority":0`), []byte(`"extra":0,"authority":0`), 1)}
	for name, change := range map[string]func(*SourceSelection){
		"authority":      func(s *SourceSelection) { s.Authority = 1 },
		"accepted":       func(s *SourceSelection) { s.Accepted = true },
		"schema":         func(s *SourceSelection) { s.SchemaVersion = "bad" },
		"nil-files":      func(s *SourceSelection) { s.Files = nil },
		"empty-files":    func(s *SourceSelection) { s.Files = []SourceIdentity{} },
		"reorder":        func(s *SourceSelection) { s.Files[0], s.Files[1] = s.Files[1], s.Files[0] },
		"duplicate-file": func(s *SourceSelection) { s.Files[1] = s.Files[0] },
		"identity":       func(s *SourceSelection) { s.Files[0].URI = "file:///outside/a.go" },
		"path":           func(s *SourceSelection) { s.Files[0].Path = "../a.go" },
		"workspace":      func(s *SourceSelection) { s.Workspace = "relative" },
		"disposition":    func(s *SourceSelection) { s.Files[0].Disposition = "UNKNOWN" },
	} {
		var s SourceSelection
		if err := json.Unmarshal(sb, &s); err != nil {
			t.Fatal(err)
		}
		change(&s)
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		cases[name] = raw
		if _, err := EncodeSourceSelection(s); err == nil {
			t.Fatalf("encoded invalid selection: %s", name)
		}
	}
	cases["missing-authority"] = bytes.Replace(sb, []byte(`"authority":0,`), nil, 1)
	cases["missing-accepted"] = bytes.Replace(sb, []byte(`"accepted":false,`), nil, 1)
	cases["missing-files"] = []byte(`{"schema_version":"lsp-trace.census-lineage-pilot-source-selection.private.v1","authority":0,"accepted":false,"workspace":"/work"}`)
	// Semantic cases retain DTO field order; canonical JSON alone cannot reject them.
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSourceSelection(raw); err == nil {
				t.Fatal("accepted invalid selection")
			}
		})
	}
	if _, err := NewSourceSelection("/work", nil); err == nil {
		t.Fatal("nil constructor")
	}
	bad := inputFiles()
	bad[0].Path = "../b.go"
	if _, err := NewSourceSelection("/work", bad); err == nil {
		t.Fatal("malformed constructor")
	}
}
func TestInputRejectPlan(t *testing.T) {
	sb, pb := inputFixture(t)
	s, _ := ParseSourceSelection(sb)
	tests := map[string]func(*PartitionPlan){
		"identity": func(p *PartitionPlan) { p.Plan.Identity = Digest([]byte("other")) }, "file": func(p *PartitionPlan) { p.Plan.Files[0].LanguageID = "other" }, "ordinal": func(p *PartitionPlan) { p.Plan.Partitions[0].Ordinal = 1 }, "member": func(p *PartitionPlan) { p.Plan.Partitions[0].Members[0] = p.Plan.Files[1] }, "size": func(p *PartitionPlan) { p.PartitionSize = 2 }, "zero-size": func(p *PartitionPlan) { p.PartitionSize = 0 }, "partition-shape": func(p *PartitionPlan) {
			p.Plan.Partitions[0].Members = append(p.Plan.Partitions[0].Members, p.Plan.Partitions[1].Members...)
			p.Plan.Partitions = p.Plan.Partitions[:1]
		}, "authority": func(p *PartitionPlan) { p.Authority = 1 }, "plan-authority": func(p *PartitionPlan) { p.Plan.Authority = 1 }, "accepted": func(p *PartitionPlan) { p.Accepted = true }, "selection-hash": func(p *PartitionPlan) { p.SourceSelectionSHA256 = Digest([]byte("other")) }, "selection-length": func(p *PartitionPlan) { p.SourceSelectionByteLength++ },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			var q PartitionPlan
			_ = json.Unmarshal(pb, &q)
			change(&q)
			raw, _ := json.Marshal(q)
			if _, err := ParsePartitionPlan(raw, s); err == nil {
				t.Fatal("accepted mismatched plan")
			}
			if _, err := EncodePartitionPlan(q, s); err == nil {
				t.Fatal("encoded invalid plan")
			}
		})
	}
	for _, raw := range [][]byte{nil, {}, []byte("null"), append(pb, ' '), append(append([]byte{}, pb...), pb...), bytes.Replace(pb, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1), bytes.Replace(pb, []byte(`"authority":0`), []byte(`"unknown":0,"authority":0`), 1)} {
		if _, err := ParsePartitionPlan(raw, s); err == nil {
			t.Fatal("invalid JSON plan")
		}
	}
	if _, err := NewPartitionPlan(s, censussourceplan.MaxPartitionSize+1); err == nil {
		t.Fatal("oversize partition")
	}
	other := s
	other.Workspace = "/elsewhere"
	if _, err := ParsePartitionPlan(pb, other); err == nil {
		t.Fatal("selection substitution")
	}
	if _, err := ParsePartitionPlan(pb, SourceSelection{}); err == nil {
		t.Fatal("empty selection")
	}
}

func TestInputReconstructionBeyondValidate(t *testing.T) {
	sb, pb := inputFixture(t)
	s, _ := ParseSourceSelection(sb)
	p, _ := ParsePartitionPlan(pb, s)
	p.Plan.Partitions[0].Members = append(p.Plan.Partitions[0].Members, p.Plan.Partitions[1].Members...)
	p.Plan.Partitions = p.Plan.Partitions[:1]
	if err := censussourceplan.Validate(p.SourcePlan(), 1); err != nil {
		t.Fatalf("witness must pass weaker Validate: %v", err)
	}
	raw, _ := json.Marshal(p)
	if _, err := ParsePartitionPlan(raw, s); err == nil {
		t.Fatal("accepted non-reconstructed partition shape")
	}
	for _, size := range []int{1, 2, 3, censussourceplan.MaxPartitionSize} {
		p, err := NewPartitionPlan(s, size)
		if err != nil {
			t.Fatal(err)
		}
		want, err := censussourceplan.Build(inputFiles(), "/work", size)
		if err != nil || !reflect.DeepEqual(p.SourcePlan(), want) {
			t.Fatal("Build equivalence", size, err)
		}
		raw, err := EncodePartitionPlan(p, s)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParsePartitionPlan(raw, s)
		if err != nil || !reflect.DeepEqual(got.SourcePlan(), want) {
			t.Fatal("parsed Build equivalence", size, err)
		}
		copy := p.SourcePlan()
		copy.Files[0].Path = "mutated"
		copy.Partitions[0].Members[0].Path = "mutated"
		if p.Plan.Files[0].Path != "a.go" || p.Plan.Partitions[0].Members[0].Path != "a.go" {
			t.Fatal("SourcePlan alias")
		}
	}
}
func TestInputCanonicalSubstitutionAndConstructorCopies(t *testing.T) {
	sb, pb := inputFixture(t)
	original, err := ObserveInputs(&inputSource{selection: sb, plan: pb}, DefaultInputLimits(), nil)
	if err != nil {
		t.Fatal(err)
	}
	files := inputFiles()
	s, err := NewSourceSelection("/work", files)
	if err != nil {
		t.Fatal(err)
	}
	files[0].Path = "mutated"
	if s.Files[1].Path != "b.go" {
		t.Fatal("constructor retained files alias")
	}
	p, err := NewPartitionPlan(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.Files[0].LanguageID = "changed"
	if p.Plan.Files[0].LanguageID != "go" || p.Plan.Partitions[0].Members[0].LanguageID != "go" {
		t.Fatal("plan retained selection alias")
	}
	changedSB, err := EncodeSourceSelection(s)
	if err != nil {
		t.Fatal(err)
	}
	changedP, err := NewPartitionPlan(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	changedPB, err := EncodePartitionPlan(changedP, s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ObserveInputs(&inputSource{selection: changedSB, plan: changedPB}, DefaultInputLimits(), nil); err != nil {
		t.Fatal("substitution witness must be independently valid", err)
	}
	if _, err := ObserveInputs(&inputSource{selection: changedSB, plan: changedPB}, DefaultInputLimits(), original.Artifacts()); err == nil {
		t.Fatal("accepted canonical-valid byte substitution")
	}
	if _, err := ObserveInputs(&inputSource{selection: changedSB, plan: pb}, DefaultInputLimits(), nil); err == nil {
		t.Fatal("observed selection not linked to observed plan")
	}
	// Files, partition membership, and partitions must also not alias each other.
	p.Plan.Files[0].Path = "changed"
	if p.Plan.Partitions[0].Members[0].Path != "a.go" {
		t.Fatal("DTO nested alias")
	}
}

func TestInputHardBoundsAndZero(t *testing.T) {
	if _, err := ParseSourceSelection(make([]byte, MaxSelectionInputBytes+1)); err == nil {
		t.Fatal("selection cap")
	}
	if _, err := ParsePartitionPlan(make([]byte, MaxPlanInputBytes+1), SourceSelection{}); err == nil {
		t.Fatal("plan cap")
	}
	if _, err := NewSourceSelection("/work", make([]censusacquisition.SourceFile, MaxInputFiles+1)); err == nil {
		t.Fatal("file cap")
	}
	var zero ObservedInputs
	if zero.SelectionBytes() != nil || zero.PlanBytes() != nil || zero.Artifacts() != nil || zero.Linkage() != (InputLinkage{}) || len(zero.Selection().Files) != 0 || len(zero.PartitionPlan().Plan.Files) != 0 {
		t.Fatal("zero contains validated data")
	}
	sb, pb := inputFixture(t)
	s, _ := ParseSourceSelection(sb)
	s.Files = append(s.Files, make([]SourceIdentity, MaxInputFiles)...)
	raw, _ := json.Marshal(s)
	if _, err := ParseSourceSelection(raw); err == nil {
		t.Fatal("parsed file cap")
	}
	limits := DefaultInputLimits()
	limits.SelectionBytes = int64(len(sb))
	limits.PlanBytes = int64(len(pb))
	limits.Files = 2
	if _, err := ObserveInputs(&inputSource{selection: sb, plan: pb}, limits, nil); err != nil {
		t.Fatalf("exact limits: %v", err)
	}
	source := &inputSource{selection: sb, plan: append(pb, ' ')}
	if _, err := ObserveInputs(source, limits, nil); err == nil || source.read != len(sb)+len(pb)+1 {
		t.Fatal("plan bounded read", err, source.read)
	}
}

type readerInputSource struct{ r io.ReadCloser }

func (s readerInputSource) OpenInput(string) (io.ReadCloser, error) { return s.r, nil }

type failingInputReader struct{ closed bool }

func (r *failingInputReader) Read(p []byte) (int, error) {
	p[0] = 'x'
	return 1, errors.New("partial read")
}
func (r *failingInputReader) Close() error { r.closed = true; return nil }
func TestInputReaderFailure(t *testing.T) {
	r := &failingInputReader{}
	if _, err := ObserveInputs(readerInputSource{r}, DefaultInputLimits(), nil); err == nil || !r.closed {
		t.Fatal("partial error not rejected and closed")
	}
	if _, err := ObserveInputs(readerInputSource{}, DefaultInputLimits(), nil); err == nil {
		t.Fatal("nil reader")
	}
	sb, pb := inputFixture(t)
	valid, err := ObserveInputs(&inputSource{selection: sb, plan: pb}, DefaultInputLimits(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a := valid.Artifacts()
	a[1] = a[0]
	src := &inputSource{selection: sb, plan: pb}
	if _, err := ObserveInputs(src, DefaultInputLimits(), a); err == nil || len(src.opens) != 0 {
		t.Fatal("duplicate expectations must fail before reading")
	}
}

type inputSource struct {
	selection, plan []byte
	read            int
	opens           []string
	fail            bool
	closeFail       bool
}
type inputReader struct {
	io.Reader
	owner *inputSource
}

func (r *inputReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.owner.read += n
	return n, e
}
func (r *inputReader) Close() error {
	if r.owner.closeFail {
		return errors.New("close")
	}
	return nil
}
func (s *inputSource) OpenInput(kind string) (io.ReadCloser, error) {
	s.opens = append(s.opens, kind)
	if s.fail {
		return nil, errors.New("open")
	}
	switch kind {
	case SourceSelectionKind:
		return &inputReader{bytes.NewReader(s.selection), s}, nil
	case PartitionPlanKind:
		return &inputReader{bytes.NewReader(s.plan), s}, nil
	}
	return nil, errors.New("kind")
}
func TestInputObservationCustody(t *testing.T) {
	sb, pb := inputFixture(t)
	source := &inputSource{selection: sb, plan: pb}
	in, err := ObserveInputs(source, DefaultInputLimits(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a := in.Artifacts()
	if len(a) != 2 || a[0].Kind != PartitionPlanKind || a[1].Kind != SourceSelectionKind {
		t.Fatal(a)
	}
	for _, r := range a {
		raw := sb
		if r.Kind == PartitionPlanKind {
			raw = pb
		}
		if r.SHA256 != Digest(raw) || r.Selector != r.SHA256 || r.ByteLength != uint64(len(raw)) {
			t.Fatal(r)
		}
	}
	links := in.Linkage()
	if links.SourceSelectionSHA256 != Digest(sb) || links.SourceSelectionByteLength != uint64(len(sb)) || links.PartitionPlanSHA256 != Digest(pb) || links.PartitionPlanByteLength != uint64(len(pb)) {
		t.Fatal(links)
	}
	if _, err := ObserveInputs(source, DefaultInputLimits(), a); err != nil {
		t.Fatal(err)
	}
	// Every exposed collection and all acquisition buffers must be isolated.
	beforeS := in.SelectionBytes()
	beforeP := in.PlanBytes()
	sb[0] = '!'
	pb[0] = '!'
	b := in.SelectionBytes()
	b[0] = '!'
	b = in.PlanBytes()
	b[0] = '!'
	s := in.Selection()
	s.Files[0].Path = "changed"
	p := in.PartitionPlan()
	p.Plan.Files[0].Path = "changed"
	p.Plan.Partitions[0].Members[0].Path = "changed"
	built := p.SourcePlan()
	built.Files[0].Path = "changed"
	built.Partitions[0].Members[0].Path = "changed"
	a[0].SHA256 = "changed"
	if !bytes.Equal(in.SelectionBytes(), beforeS) || !bytes.Equal(in.PlanBytes(), beforeP) || in.Selection().Files[0].Path != "a.go" || in.PartitionPlan().Plan.Partitions[0].Members[0].Path != "a.go" || in.Artifacts()[0].SHA256 == "changed" {
		t.Fatal("alias leak")
	}
	if _, err := ObserveInputs(source, DefaultInputLimits(), in.Artifacts()); err == nil {
		t.Fatal("byte substitution")
	}
}
func TestInputObservationBoundsAndKinds(t *testing.T) {
	sb, pb := inputFixture(t)
	for name, limits := range map[string]InputLimits{"zero": {}, "negative": {SelectionBytes: -1, PlanBytes: 1, Files: 1}, "selection": {SelectionBytes: int64(len(sb) - 1), PlanBytes: MaxPlanInputBytes, Files: MaxInputFiles}, "plan": {SelectionBytes: MaxSelectionInputBytes, PlanBytes: int64(len(pb) - 1), Files: MaxInputFiles}, "files": {SelectionBytes: MaxSelectionInputBytes, PlanBytes: MaxPlanInputBytes, Files: 1}, "excess": {SelectionBytes: MaxSelectionInputBytes + 1, PlanBytes: MaxPlanInputBytes, Files: MaxInputFiles}} {
		t.Run(name, func(t *testing.T) {
			if _, err := ObserveInputs(&inputSource{selection: sb, plan: pb}, limits, nil); err == nil {
				t.Fatal("bounds bypass")
			}
		})
	}
	source := &inputSource{selection: []byte(strings.Repeat("x", 1000)), plan: pb}
	limits := DefaultInputLimits()
	limits.SelectionBytes = 10
	if _, err := ObserveInputs(source, limits, nil); err == nil || source.read != 11 || len(source.opens) != 1 {
		t.Fatalf("not bounded: %d %v", source.read, err)
	}
	for _, source := range []*inputSource{{fail: true}, {selection: sb, plan: pb, closeFail: true}, {selection: nil, plan: pb}} {
		if _, err := ObserveInputs(source, DefaultInputLimits(), nil); err == nil {
			t.Fatal("source failure")
		}
	}
	if _, err := ObserveInputs(nil, DefaultInputLimits(), nil); err == nil {
		t.Fatal("nil source")
	}
	valid, _ := ObserveInputs(&inputSource{selection: sb, plan: pb}, DefaultInputLimits(), nil)
	for _, expected := range [][]Artifact{{}, {valid.Artifacts()[0]}, append(valid.Artifacts(), valid.Artifacts()[0]), {{Kind: "PROBE", SHA256: Digest(sb), Selector: Digest(sb), ByteLength: uint64(len(sb))}}} {
		if _, err := ObserveInputs(&inputSource{selection: sb, plan: pb}, DefaultInputLimits(), expected); err == nil {
			t.Fatal("unsupported inventory")
		}
	}
	for _, field := range []string{"digest", "length", "selector", "kind"} {
		a := valid.Artifacts()
		switch field {
		case "digest":
			a[0].SHA256 = Digest([]byte("other"))
		case "length":
			a[0].ByteLength++
		case "selector":
			a[0].Selector = Digest([]byte("other"))
		case "kind":
			a[0].Kind = "PROBE"
		}
		if _, err := ObserveInputs(&inputSource{selection: sb, plan: pb}, DefaultInputLimits(), a); err == nil {
			t.Fatal("trusted expected metadata")
		}
	}
}
