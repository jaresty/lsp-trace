package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func TestPrivateProviderProbeVersionAdmission(t *testing.T) {
	path := probePath(t)
	base := []string{"census", "--workspace", ".", "--server", "gopls", "--publication-root", probePath(t), "--machine", "--private-provider-probe-path", path}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"duplicate", append(append([]string{}, base...), "--private-provider-probe-version", "v1", "--private-provider-probe-version", "v2")},
		{"unknown", append(append([]string{}, base...), "--private-provider-probe-version", "v7")},
		{"empty", append(append([]string{}, base...), "--private-provider-probe-version=")},
		{"case-variant", append(append([]string{}, base...), "--private-provider-probe-version", "V2")},
		{"without-path", []string{"census", "--workspace", ".", "--server", "gopls", "--publication-root", probePath(t), "--machine", "--private-provider-probe-version", "v2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseCensusCLIOptions(tc.args); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, version := range []string{"", "v1", "v2", "v3", "v4", "v5", "v6"} {
		args := append([]string{}, base...)
		if version != "" {
			args = append(args, "--private-provider-probe-version", version)
		}
		got, err := parseCensusCLIOptions(args)
		if err != nil {
			t.Fatalf("version=%q: %v", version, err)
		}
		want := version
		if want == "" {
			want = "v1"
		}
		if got.PrivateProviderProbeVersion != want {
			t.Fatalf("version=%q got=%q", version, got.PrivateProviderProbeVersion)
		}
	}
	if strings.Contains(censusUsage, "private-provider-probe-version") {
		t.Fatal("private version flag disclosed")
	}
}

func TestPrivateProviderProbeV1OmittedAndExplicitGoldenBytes(t *testing.T) {
	const want = "{\"schema\":\"lsp-trace.private-provider-probe.v1\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[{\"cause\":\"RESOURCE_EXHAUSTED\",\"count\":1}],\"records\":[{\"sequence\":1,\"phase\":\"DOCUMENT_SYMBOL\",\"class\":\"TYPED_SESSION_RUNTIME_FAILURE\",\"cause\":\"RESOURCE_EXHAUSTED\"}],\"rerun_count\":0,\"work\":1}\n"
	var raws [][]byte
	for _, version := range []string{"", "v1"} {
		path := probePath(t)
		var p *privateProviderProbe
		var err error
		if version == "" {
			p, err = newPrivateProviderProbe(path)
		} else {
			p, err = newPrivateProviderProbeVersion(path, version)
		}
		if err != nil {
			t.Fatal(err)
		}
		p.observe("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), false)
		if err := p.finalize(); err != nil {
			t.Fatal(err)
		}
		raw, err := osReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raws = append(raws, raw)
		if string(raw) != want {
			t.Fatalf("version=%q\n got=%q\nwant=%q", version, raw, want)
		}
	}
	if !bytes.Equal(raws[0], raws[1]) {
		t.Fatal("omitted and explicit v1 differ")
	}
}

func TestPrivateProviderProbeV2BoundaryDerivation(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		messages, maxMessages int
		bytes, maxBytes       int64
		want                  string
		wantErr               bool
	}{
		{"negative-messages", -1, 64, 0, 100, "INVALID_OBSERVATION", false},
		{"negative-bytes", 0, 64, -1, 100, "INVALID_OBSERVATION", false},
		{"unknown-messages", 1, 0, 1, 100, "UNKNOWN", false},
		{"unknown-bytes", 1, 64, 1, 0, "UNKNOWN", false},
		{"bytes-precede-messages", 64, 64, 101, 100, "BYTE_LIMIT_EXCEEDED", false},
		{"message-ceiling", 64, 64, 100, 100, "MESSAGE_CEILING_OBSERVED", false},
		{"neither", 63, 64, 100, 100, "NEITHER_BOUNDARY_OBSERVED", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := privateProviderProbeBoundary(tc.messages, tc.bytes, tc.maxMessages, tc.maxBytes)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}

func TestPrivateProviderProbeV2NthSuccessAndPostNthExhaustionAreObservational(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v2")
	if err != nil {
		t.Fatal(err)
	}
	r := &privateAcquisitionRuntime{probe: p}
	r.observeRoundTrip("textDocument/documentSymbol", 64, 100, sessionruntime.RoundTripResult{Messages: 64, Bytes: 100, Result: json.RawMessage(`[]`)}, nil)
	if got := p.artifactV2(); got.Work != 1 || len(got.Records) != 0 || len(got.CauseCounts) != 0 {
		t.Fatalf("success synthesized exhaustion: %+v", got)
	}
	r.observeRoundTrip("textDocument/documentSymbol", 64, 100, sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, Messages: 65, Bytes: 101}, nil)
	got := p.artifactV2()
	if got.Work != 2 || len(got.Records) != 1 || got.Records[0].Sequence != 2 || got.Records[0].Exhaustion == nil || got.Records[0].Exhaustion.Boundary != "BYTE_LIMIT_EXCEEDED" {
		t.Fatalf("post-Nth observation=%+v", got)
	}
}

func TestPrivateProviderProbeV2StatusWorkBoundary(t *testing.T) {
	parse := func(t *testing.T, artifact privateProviderProbeArtifactV2, wantValid bool) {
		t.Helper()
		raw, err := json.Marshal(artifact)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parsePrivateProviderProbeV2(append(raw, '\n'))
		if (err == nil) != wantValid {
			t.Fatalf("ASSERT_PRIVATE_PROVIDER_PROBE_V2_STATUS_WORK_BOUNDARY wantValid=%v err=%v artifact=%s", wantValid, err, raw)
		}
	}
	base := func(status string, work uint64) privateProviderProbeArtifactV2 {
		return privateProviderProbeArtifactV2{Schema: privateProviderProbeSchemaV2, Status: status, CauseCounts: []privateProviderProbeCount{}, Work: work}
	}

	t.Run("complete-zero-work", func(t *testing.T) { parse(t, base("COMPLETE", 0), true) })
	t.Run("incomplete-zero-work", func(t *testing.T) { parse(t, base("INCOMPLETE", 0), false) })
	t.Run("complete-positive-work", func(t *testing.T) { parse(t, base("COMPLETE", 1), true) })
	t.Run("incomplete-positive-work-produced", func(t *testing.T) {
		p := &privateProviderProbe{counts: make(map[string]uint64), work: math.MaxUint64}
		p.observeSuccess(false)
		artifact := p.artifactV2()
		if artifact.Status != "INCOMPLETE" || artifact.Work == 0 {
			t.Fatalf("producer did not construct positive-work incomplete artifact: %+v", artifact)
		}
		parse(t, artifact, true)
	})
	t.Run("incomplete-counter-saturation-produced", func(t *testing.T) {
		p := &privateProviderProbe{counts: map[string]uint64{"DECODE_FAILURE": math.MaxUint64}, work: math.MaxUint64 - 1}
		p.observe("DOCUMENT_SYMBOL", "DECODE_FAILURE", "DECODE_FAILURE", false)
		artifact := p.artifactV2()
		if artifact.Status != "INCOMPLETE" || artifact.Work == 0 {
			t.Fatalf("producer did not construct saturated positive-work incomplete artifact: %+v", artifact)
		}
		parse(t, artifact, true)
	})

	invalidExhaustion := func(status string, work uint64) privateProviderProbeArtifactV2 {
		return privateProviderProbeArtifactV2{
			Schema:      privateProviderProbeSchemaV2,
			Status:      status,
			CauseCounts: []privateProviderProbeCount{{Cause: string(session.ResourceExhausted), Count: 1}},
			Records: []privateProviderProbeRecordV2{{
				Sequence: 1, Phase: "DOCUMENT_SYMBOL", Class: "TYPED_SESSION_RUNTIME_FAILURE", Cause: string(session.ResourceExhausted),
				Exhaustion: &privateProviderProbeExhaustionV2{ObservedMessages: -1, ConfiguredMaxMessages: 64, ConfiguredMaxBytes: 100, Boundary: "INVALID_OBSERVATION"},
			}},
			Work: work,
		}
	}
	for _, tc := range []struct {
		name      string
		status    string
		work      uint64
		wantValid bool
	}{
		{"incomplete-invalid-low-work", "INCOMPLETE", 1, true},
		{"incomplete-invalid-max-work", "INCOMPLETE", math.MaxUint64, true},
		{"complete-invalid-low-work", "COMPLETE", 1, false},
		{"complete-invalid-max-work-reviewer-reproducer", "COMPLETE", math.MaxUint64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parse(t, invalidExhaustion(tc.status, tc.work), tc.wantValid)
		})
	}
	t.Run("complete-max-work-without-invalid-observation", func(t *testing.T) { parse(t, base("COMPLETE", math.MaxUint64), true) })
	t.Run("incomplete-max-work-without-invalid-observation", func(t *testing.T) { parse(t, base("INCOMPLETE", math.MaxUint64), true) })
}

func TestPrivateProviderProbeV2IncompleteProducerSchedules(t *testing.T) {
	parse := func(t *testing.T, artifact privateProviderProbeArtifactV2, wantValid bool) {
		t.Helper()
		raw, err := json.Marshal(artifact)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parsePrivateProviderProbeV2(append(raw, '\n'))
		if (err == nil) != wantValid {
			t.Fatalf("ASSERT_PRIVATE_PROVIDER_PROBE_V2_INCOMPLETE_SCHEDULE wantValid=%v err=%v artifact=%s", wantValid, err, raw)
		}
	}
	artifact := func(p *privateProviderProbe) privateProviderProbeArtifactV2 {
		p.version = "v2"
		if p.counts == nil {
			p.counts = make(map[string]uint64)
		}
		return p.artifactV2()
	}

	t.Run("exact-low-work-no-cause-reviewer-reproducer", func(t *testing.T) {
		parse(t, privateProviderProbeArtifactV2{Schema: privateProviderProbeSchemaV2, Status: "INCOMPLETE", CauseCounts: []privateProviderProbeCount{}, Work: 1}, false)
	})
	t.Run("work-saturation", func(t *testing.T) {
		p := &privateProviderProbe{work: math.MaxUint64}
		p.observeSuccess(false)
		parse(t, artifact(p), true)
	})
	t.Run("sequence-saturation", func(t *testing.T) {
		p := &privateProviderProbe{work: math.MaxUint64 - 1, sequence: math.MaxUint64}
		p.observeSuccess(false)
		parse(t, artifact(p), true)
	})
	t.Run("file-saturation", func(t *testing.T) {
		p := &privateProviderProbe{work: math.MaxUint64 - 1, sequence: math.MaxUint64 - 1, files: math.MaxUint64}
		p.observeSuccess(true)
		parse(t, artifact(p), true)
	})
	t.Run("cause-counter-at-saturation", func(t *testing.T) {
		p := &privateProviderProbe{work: math.MaxUint64, sequence: math.MaxUint64, counts: map[string]uint64{"DECODE_FAILURE": math.MaxUint64}}
		p.observe("DOCUMENT_SYMBOL", "DECODE_FAILURE", "DECODE_FAILURE", false)
		parse(t, artifact(p), true)
	})
	t.Run("exact-low-work-supply-only-resource-exhausted-reviewer-reproducer", func(t *testing.T) {
		parse(t, privateProviderProbeArtifactV2{
			Schema: privateProviderProbeSchemaV2, Status: "INCOMPLETE", FileDenominator: 1,
			CauseCounts: []privateProviderProbeCount{{Cause: string(session.ResourceExhausted), Count: 1}}, Work: 1,
		}, false)
	})
	t.Run("invalid-exhaustion-observation", func(t *testing.T) {
		p := &privateProviderProbe{counts: make(map[string]uint64), version: "v2"}
		p.observeExhaustion("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), 64, 100, sessionruntime.RoundTripResult{Messages: -1})
		got := p.artifactV2()
		if got.Status != "INCOMPLETE" || got.Work != 1 || len(got.CauseCounts) != 1 || got.CauseCounts[0].Cause != string(session.ResourceExhausted) || len(got.Records) != 1 || got.Records[0].Exhaustion == nil || got.Records[0].Exhaustion.Boundary != "INVALID_OBSERVATION" {
			t.Fatalf("unexpected exhaustion schedule: %+v", got)
		}
		parse(t, got, true)
	})
}

func TestPrivateProviderProbeV2RetentionIsIndependentFromV1(t *testing.T) {
	newProbe := func(t *testing.T) *privateProviderProbe {
		t.Helper()
		p, err := newPrivateProviderProbeVersion(probePath(t), "v2")
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	invalid := sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, Messages: -1}
	valid := sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, Messages: 64, Bytes: 100}
	assertRoundTrip := func(t *testing.T, artifact privateProviderProbeArtifactV2) {
		t.Helper()
		raw, err := json.Marshal(artifact)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parsePrivateProviderProbeV2(append(raw, '\n')); err != nil {
			t.Fatalf("producer emitted parser-invalid v2 artifact: %v: %s", err, raw)
		}
	}

	t.Run("invalid-first-exhaustion-then-supply", func(t *testing.T) {
		p := newProbe(t)
		p.observeExhaustion("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), 64, 100, invalid)
		p.observe("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", string(sessionruntime.DocumentSourceUnavailable), true)
		got := p.artifactV2()
		if got.Status != "INCOMPLETE" || got.Work != 2 || got.FileDenominator != 1 || len(got.Records) != 2 || got.Records[0].Phase != "DOCUMENT_SYMBOL" || got.Records[0].Exhaustion == nil || got.Records[0].Exhaustion.Boundary != "INVALID_OBSERVATION" || got.Records[1].Phase != "DOCUMENT_SUPPLY" || got.Records[0].Sequence != 1 || got.Records[1].Sequence != 2 {
			t.Fatalf("ASSERT_V2_INVALID_FIRST_EXHAUSTION_THEN_SUPPLY: %+v", got)
		}
		assertRoundTrip(t, got)
	})

	t.Run("valid-first-exhaustion-then-invalid-later", func(t *testing.T) {
		p := newProbe(t)
		p.observeExhaustion("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), 64, 100, valid)
		p.observeExhaustion("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), 64, 100, invalid)
		p.observe("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", string(sessionruntime.DocumentSourceUnavailable), true)
		got := p.artifactV2()
		if got.Status != "COMPLETE" || got.Work != 3 || got.FileDenominator != 1 || len(got.Records) != 2 || got.Records[0].Sequence != 1 || got.Records[0].Exhaustion == nil || got.Records[0].Exhaustion.Boundary != "MESSAGE_CEILING_OBSERVED" || got.Records[1].Phase != "DOCUMENT_SUPPLY" || got.Records[1].Sequence != 3 {
			t.Fatalf("ASSERT_V2_VALID_FIRST_EXHAUSTION_THEN_INVALID_LATER: %+v", got)
		}
		assertRoundTrip(t, got)
	})

	t.Run("max-two-v2-chronology-ignores-legacy-only-record", func(t *testing.T) {
		p := newProbe(t)
		p.records = append(p.records, privateProviderProbeRecord{Sequence: 99, Phase: "DOCUMENT_SYMBOL", Class: "DECODE_FAILURE", Cause: "DECODE_FAILURE"})
		p.observe("DOCUMENT_SYMBOL", "DECODE_FAILURE", "DECODE_FAILURE", false)
		p.observe("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", string(sessionruntime.DocumentSourceUnavailable), true)
		p.observe("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", string(sessionruntime.DocumentURIUnavailable), true)
		got := p.artifactV2()
		if got.Work != 3 || got.FileDenominator != 2 || len(got.Records) != 2 || got.Records[0].Phase != "DOCUMENT_SYMBOL" || got.Records[0].Sequence != 1 || got.Records[1].Phase != "DOCUMENT_SUPPLY" || got.Records[1].Sequence != 2 {
			t.Fatalf("ASSERT_V2_INDEPENDENT_MAX_TWO_CHRONOLOGY: %+v", got)
		}
		assertRoundTrip(t, got)
	})
}

func TestPrivateProviderProbeV2StrictCanonicalParser(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v2")
	if err != nil {
		t.Fatal(err)
	}
	r := &privateAcquisitionRuntime{probe: p}
	r.observeRoundTrip("textDocument/documentSymbol", 64, 100, sessionruntime.RoundTripResult{Failure: session.ResourceExhausted, Messages: 64, Bytes: 100}, nil)
	good, err := json.Marshal(p.artifactV2())
	if err != nil {
		t.Fatal(err)
	}
	good = append(good, '\n')
	parsed, err := parsePrivateProviderProbeV2(good)
	if err != nil || !reflect.DeepEqual(parsed, p.artifactV2()) {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
	var loose map[string]any
	if err := json.Unmarshal(good, &loose); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(map[string]any){
		"unknown-field": func(v map[string]any) { v["extra"] = true },
		"wrong-boundary": func(v map[string]any) {
			v2Record(v)["exhaustion"].(map[string]any)["boundary"] = "NEITHER_BOUNDARY_OBSERVED"
		},
		"negative-observed":     func(v map[string]any) { v2Record(v)["exhaustion"].(map[string]any)["observed_messages"] = -1.0 },
		"missing-exhaustion":    func(v map[string]any) { delete(v2Record(v), "exhaustion") },
		"unexpected-exhaustion": func(v map[string]any) { v2Record(v)["cause"] = "REQUEST_TIMEOUT" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copyBytes, _ := json.Marshal(loose)
			var candidate map[string]any
			_ = json.Unmarshal(copyBytes, &candidate)
			mutate(candidate)
			raw, _ := json.Marshal(candidate)
			if _, err := parsePrivateProviderProbeV2(raw); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	if _, err := parsePrivateProviderProbeV2(append(bytes.TrimSuffix(good, []byte{'\n'}), []byte(" {}")...)); err == nil {
		t.Fatal("extra value accepted")
	}
	if _, err := parsePrivateProviderProbeV2(bytes.TrimSuffix(good, []byte{'\n'})); err == nil {
		t.Fatal("missing trailing newline accepted")
	}
}

func TestPrivateProviderProbeV2SemanticProducibility(t *testing.T) {
	valid := privateProviderProbeArtifactV2{
		Schema: privateProviderProbeSchemaV2, Status: "COMPLETE",
		CauseCounts: []privateProviderProbeCount{{Cause: "RESOURCE_EXHAUSTED", Count: 1}},
		Records: []privateProviderProbeRecordV2{{
			Sequence: 1, Phase: "DOCUMENT_SYMBOL", Class: "TYPED_SESSION_RUNTIME_FAILURE", Cause: "RESOURCE_EXHAUSTED",
			Exhaustion: &privateProviderProbeExhaustionV2{ObservedMessages: 1, ObservedBytes: 1, ConfiguredMaxMessages: 1, ConfiguredMaxBytes: 1, Boundary: "MESSAGE_CEILING_OBSERVED"},
		}},
		Work: 1,
	}
	parse := func(t *testing.T, artifact privateProviderProbeArtifactV2, wantValid bool) {
		t.Helper()
		raw, err := json.Marshal(artifact)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parsePrivateProviderProbeV2(append(raw, '\n'))
		if (err == nil) != wantValid {
			t.Fatalf("wantValid=%v err=%v artifact=%s", wantValid, err, raw)
		}
	}
	clone := func() privateProviderProbeArtifactV2 {
		copy := valid
		copy.CauseCounts = append([]privateProviderProbeCount(nil), valid.CauseCounts...)
		copy.Records = append([]privateProviderProbeRecordV2(nil), valid.Records...)
		return copy
	}
	parseRaw := func(t *testing.T, raw string, wantValid bool) {
		t.Helper()
		_, err := parsePrivateProviderProbeV2([]byte(raw + "\n"))
		if (err == nil) != wantValid {
			t.Fatalf("wantValid=%v err=%v artifact=%s", wantValid, err, raw)
		}
	}

	for _, test := range []struct {
		name      string
		counts    string
		records   string
		wantValid bool
	}{
		{name: "valid-cause-counts-empty-records-null", counts: "[]", records: "null", wantValid: true},
		{name: "invalid-cause-counts-null-records-null", counts: "null", records: "null"},
		{name: "invalid-cause-counts-empty-records-empty", counts: "[]", records: "[]"},
		{name: "invalid-cause-counts-null-records-empty", counts: "null", records: "[]"},
	} {
		t.Run("zero-work-"+test.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"schema":"lsp-trace.private-provider-probe.v2","status":"COMPLETE","file_denominator":0,"cause_counts":%s,"records":%s,"rerun_count":0,"work":0}`, test.counts, test.records)
			parseRaw(t, raw, test.wantValid)
		})
		t.Run("positive-work-all-success-"+test.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"schema":"lsp-trace.private-provider-probe.v2","status":"COMPLETE","file_denominator":0,"cause_counts":%s,"records":%s,"rerun_count":0,"work":1}`, test.counts, test.records)
			parseRaw(t, raw, test.wantValid)
		})
	}

	t.Run("exact-arbitrary-pairing-reproducer", func(t *testing.T) {
		const raw = "{\"schema\":\"lsp-trace.private-provider-probe.v2\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[{\"cause\":\"RESOURCE_EXHAUSTED\",\"count\":1}],\"records\":[{\"sequence\":1,\"phase\":\"DOCUMENT_SYMBOL\",\"class\":\"DECODE_FAILURE\",\"cause\":\"RESOURCE_EXHAUSTED\"}],\"rerun_count\":0,\"work\":1}\n"
		if _, err := parsePrivateProviderProbeV2([]byte(raw)); err == nil {
			t.Fatal("accepted arbitrary class/cause pairing")
		}
	})
	t.Run("exact-symbol-work-accounting-reproducer", func(t *testing.T) {
		a := clone()
		a.FileDenominator = 1
		parse(t, a, false)
	})
	t.Run("exact-document-supply-denominator-reproducer", func(t *testing.T) {
		const raw = "{\"schema\":\"lsp-trace.private-provider-probe.v2\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[{\"cause\":\"DECODE_FAILURE\",\"count\":1},{\"cause\":\"DOCUMENT_SOURCE_UNAVAILABLE\",\"count\":1}],\"records\":[{\"sequence\":1,\"phase\":\"DOCUMENT_SYMBOL\",\"class\":\"DECODE_FAILURE\",\"cause\":\"DECODE_FAILURE\"},{\"sequence\":2,\"phase\":\"DOCUMENT_SUPPLY\",\"class\":\"DOCUMENT_FAILURE\",\"cause\":\"DOCUMENT_SOURCE_UNAVAILABLE\"}],\"rerun_count\":0,\"work\":2}\n"
		if _, err := parsePrivateProviderProbeV2([]byte(raw)); err == nil {
			t.Fatal("accepted retained document supply beyond file denominator")
		}
	})
	t.Run("valid-one-document-supply-with-denominator-one", func(t *testing.T) {
		a := clone()
		a.FileDenominator = 1
		a.Work = 2
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}, {Cause: "DOCUMENT_SOURCE_UNAVAILABLE", Count: 1}}
		a.Records = []privateProviderProbeRecordV2{
			{Sequence: 1, Phase: "DOCUMENT_SYMBOL", Class: "DECODE_FAILURE", Cause: "DECODE_FAILURE"},
			{Sequence: 2, Phase: "DOCUMENT_SUPPLY", Class: "DOCUMENT_FAILURE", Cause: "DOCUMENT_SOURCE_UNAVAILABLE"},
		}
		parse(t, a, true)
	})
	t.Run("valid-zero-file-without-document-supply", func(t *testing.T) {
		parse(t, clone(), true)
	})
	t.Run("valid-minimum-document-supply-work", func(t *testing.T) {
		a := clone()
		a.FileDenominator = 1
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DOCUMENT_SOURCE_UNAVAILABLE", Count: 1}}
		a.Records = nil
		parse(t, a, true)
	})
	t.Run("valid-minimum-document-symbol-work", func(t *testing.T) {
		a := clone()
		a.FileDenominator = 1
		a.Work = 2
		parse(t, a, true)
	})
	t.Run("symbol-work-accounting-overflow-boundaries", func(t *testing.T) {
		a := clone()
		a.FileDenominator = ^uint64(0) - 1
		a.Work = ^uint64(0)
		parse(t, a, true)
		a.FileDenominator = ^uint64(0)
		parse(t, a, false)
		a.CauseCounts = []privateProviderProbeCount{}
		a.Records = nil
		parse(t, a, true)
	})
	t.Run("invalid-pair", func(t *testing.T) {
		a := clone()
		a.Records[0].Class = "JSON_RPC_CODE_BUCKET"
		a.Records[0].Exhaustion = nil
		parse(t, a, false)
	})
	t.Run("unknown-aggregate-cause", func(t *testing.T) {
		a := clone()
		a.CauseCounts[0].Cause = "UNKNOWN_CAUSE"
		parse(t, a, false)
	})
	t.Run("count-exceeds-work", func(t *testing.T) {
		a := clone()
		a.CauseCounts[0].Count = 2
		parse(t, a, false)
	})
	t.Run("sum-exceeds-work", func(t *testing.T) {
		a := clone()
		a.Work = 2
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}, {Cause: "RESOURCE_EXHAUSTED", Count: 2}}
		parse(t, a, false)
	})
	t.Run("sum-overflow", func(t *testing.T) {
		a := clone()
		a.Work = ^uint64(0)
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}, {Cause: "RESOURCE_EXHAUSTED", Count: ^uint64(0)}}
		parse(t, a, false)
	})
	t.Run("missing-retained-cause", func(t *testing.T) {
		a := clone()
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}}
		parse(t, a, false)
	})
	t.Run("retained-multiplicity-exceeds-count", func(t *testing.T) {
		a := clone()
		a.Work = 2
		a.Records = append(a.Records, privateProviderProbeRecordV2{Sequence: 2, Phase: "DOCUMENT_SUPPLY", Class: "TYPED_SESSION_RUNTIME_FAILURE", Cause: "RESOURCE_EXHAUSTED", Exhaustion: a.Records[0].Exhaustion})
		parse(t, a, false)
	})
	t.Run("aggregates-beyond-retained-records", func(t *testing.T) {
		a := clone()
		a.FileDenominator = 3
		a.Work = 4
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}, {Cause: "DOCUMENT_SOURCE_UNAVAILABLE", Count: 3}}
		a.Records = []privateProviderProbeRecordV2{
			{Sequence: 1, Phase: "DOCUMENT_SYMBOL", Class: "DECODE_FAILURE", Cause: "DECODE_FAILURE"},
			{Sequence: 2, Phase: "DOCUMENT_SUPPLY", Class: "DOCUMENT_FAILURE", Cause: "DOCUMENT_SOURCE_UNAVAILABLE"},
		}
		parse(t, a, true)
	})
	t.Run("document-supply-denominator-overflow-boundary", func(t *testing.T) {
		a := clone()
		a.FileDenominator = ^uint64(0) - 1
		a.Work = ^uint64(0)
		a.CauseCounts = []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}, {Cause: "DOCUMENT_SOURCE_UNAVAILABLE", Count: 1}}
		a.Records = []privateProviderProbeRecordV2{
			{Sequence: 1, Phase: "DOCUMENT_SYMBOL", Class: "DECODE_FAILURE", Cause: "DECODE_FAILURE"},
			{Sequence: 2, Phase: "DOCUMENT_SUPPLY", Class: "DOCUMENT_FAILURE", Cause: "DOCUMENT_SOURCE_UNAVAILABLE"},
		}
		parse(t, a, true)
	})
	t.Run("success-remainder", func(t *testing.T) {
		a := clone()
		a.Work = 3
		parse(t, a, true)
	})
}

func TestPrivateProviderProbeV2ProducerScheduleCausePartition(t *testing.T) {
	type scheduleCase struct {
		cause     string
		schedule  string
		wantValid bool
	}
	const (
		roundTripOnly = "round-trip-only"
		supplyOnly    = "supply-only"
		shared        = "shared"
	)
	cases := []scheduleCase{
		{string(session.SessionNotFound), roundTripOnly, false},
		{string(session.SessionStopping), roundTripOnly, false},
		{string(session.SessionCrashed), roundTripOnly, false},
		{"PARSE_ERROR", roundTripOnly, false},
		{"INVALID_REQUEST", roundTripOnly, false},
		{"METHOD_NOT_FOUND", roundTripOnly, false},
		{"INVALID_PARAMS", roundTripOnly, false},
		{"INTERNAL_ERROR", roundTripOnly, false},
		{"SERVER_ERROR", roundTripOnly, false},
		{"OTHER", roundTripOnly, false},
		{"DECODE_FAILURE", roundTripOnly, false},
		{string(sessionruntime.DocumentSupplyUnavailable), supplyOnly, false},
		{string(sessionruntime.DocumentURIUnavailable), supplyOnly, false},
		{string(sessionruntime.DocumentOutsideWorkspace), supplyOnly, false},
		{string(sessionruntime.DocumentSourceUnavailable), supplyOnly, false},
		{string(sessionruntime.LanguageIDUnavailable), supplyOnly, false},
		{"ADAPTER_IDENTITY_MISMATCH", supplyOnly, false},
		{string(session.RequestCancelled), shared, true},
		{string(session.RequestTimeout), shared, true},
		{string(session.StaleGeneration), shared, true},
		{string(session.LifecycleConflict), shared, true},
		{string(session.ResourceExhausted), shared, true},
		{string(session.SessionPoisoned), shared, true},
		{"OTHER_TYPED_FAILURE", shared, true},
	}
	for _, tc := range cases {
		t.Run(strings.ToLower(tc.schedule+"-"+tc.cause), func(t *testing.T) {
			artifact := privateProviderProbeArtifactV2{
				Schema: privateProviderProbeSchemaV2, Status: "COMPLETE", FileDenominator: 1,
				CauseCounts: []privateProviderProbeCount{{Cause: tc.cause, Count: 2}}, Work: 2,
			}
			raw, err := json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			_, err = parsePrivateProviderProbeV2(append(raw, '\n'))
			if (err == nil) != tc.wantValid {
				t.Fatalf("ASSERT_PRIVATE_PROVIDER_PROBE_V2_CAUSE_PARTITION schedule=%s cause=%s wantValid=%v err=%v", tc.schedule, tc.cause, tc.wantValid, err)
			}
		})
	}

	for _, tc := range []struct {
		name        string
		work, files uint64
		causeCounts []privateProviderProbeCount
		wantValid   bool
	}{
		{"exact-round-trip-aggregate-reviewer-reproducer", 2, 1, []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 2}}, false},
		{"exact-supply-aggregate-reviewer-reproducer", 2, 1, []privateProviderProbeCount{{Cause: string(sessionruntime.DocumentSourceUnavailable), Count: 2}}, false},
		{"shared-not-charged-to-either-denominator", 2, 1, []privateProviderProbeCount{{Cause: string(session.RequestTimeout), Count: 2}}, true},
		{"success-remainder", 3, 1, []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}, {Cause: string(sessionruntime.DocumentSourceUnavailable), Count: 1}}, true},
		{"round-trip-overflow-safe-equality", math.MaxUint64, math.MaxUint64 - 1, []privateProviderProbeCount{{Cause: "DECODE_FAILURE", Count: 1}}, true},
		{"supply-overflow-safe-equality", math.MaxUint64, math.MaxUint64, []privateProviderProbeCount{{Cause: string(sessionruntime.DocumentSourceUnavailable), Count: math.MaxUint64}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifact := privateProviderProbeArtifactV2{Schema: privateProviderProbeSchemaV2, Status: "COMPLETE", FileDenominator: tc.files, CauseCounts: tc.causeCounts, Work: tc.work}
			raw, err := json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			_, err = parsePrivateProviderProbeV2(append(raw, '\n'))
			if (err == nil) != tc.wantValid {
				t.Fatalf("ASSERT_PRIVATE_PROVIDER_PROBE_V2_AGGREGATE_SCHEDULE wantValid=%v err=%v artifact=%s", tc.wantValid, err, raw)
			}
		})
	}
}

func TestPrivateProviderProbeV2PhaseClassClosure(t *testing.T) {
	type tuple struct {
		class string
		cause string
	}
	roundTrip := []tuple{
		{class: "TYPED_SESSION_RUNTIME_FAILURE", cause: "REQUEST_TIMEOUT"},
		{class: "JSON_RPC_CODE_BUCKET", cause: "INTERNAL_ERROR"},
		{class: "DECODE_FAILURE", cause: "DECODE_FAILURE"},
	}
	preparation := []tuple{
		{class: "DOCUMENT_FAILURE", cause: "DOCUMENT_SOURCE_UNAVAILABLE"},
		{class: "ADAPTER_IDENTITY_MISMATCH", cause: "ADAPTER_IDENTITY_MISMATCH"},
	}
	artifact := func(phase string, value tuple) privateProviderProbeArtifactV2 {
		records := []privateProviderProbeRecordV2{{Sequence: 1, Phase: phase, Class: value.class, Cause: value.cause}}
		causes := []string{value.cause}
		work := uint64(1)
		if phase == "DOCUMENT_SUPPLY" {
			records = append([]privateProviderProbeRecordV2{{Sequence: 1, Phase: "DOCUMENT_SYMBOL", Class: "DECODE_FAILURE", Cause: "DECODE_FAILURE"}}, records...)
			records[1].Sequence = 2
			causes = append(causes, "DECODE_FAILURE")
			work = 2
		}
		sort.Strings(causes)
		counts := make([]privateProviderProbeCount, 0, len(causes))
		for _, cause := range causes {
			if len(counts) > 0 && counts[len(counts)-1].Cause == cause {
				counts[len(counts)-1].Count++
			} else {
				counts = append(counts, privateProviderProbeCount{Cause: cause, Count: 1})
			}
		}
		fileDenominator := uint64(0)
		if phase == "DOCUMENT_SUPPLY" {
			fileDenominator = 1
		}
		return privateProviderProbeArtifactV2{Schema: privateProviderProbeSchemaV2, Status: "COMPLETE", FileDenominator: fileDenominator, CauseCounts: counts, Records: records, Work: work}
	}
	parse := func(t *testing.T, value privateProviderProbeArtifactV2, wantValid bool) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parsePrivateProviderProbeV2(append(raw, '\n'))
		if (err == nil) != wantValid {
			t.Fatalf("wantValid=%v err=%v artifact=%s", wantValid, err, raw)
		}
	}

	for _, value := range preparation {
		t.Run("document-symbol-rejects-"+strings.ToLower(value.class), func(t *testing.T) {
			parse(t, artifact("DOCUMENT_SYMBOL", value), false)
		})
	}
	for _, value := range roundTrip {
		t.Run("document-supply-rejects-"+strings.ToLower(value.class), func(t *testing.T) {
			parse(t, artifact("DOCUMENT_SUPPLY", value), false)
		})
	}
	for _, value := range roundTrip {
		t.Run("document-symbol-accepts-"+strings.ToLower(value.class), func(t *testing.T) {
			parse(t, artifact("DOCUMENT_SYMBOL", value), true)
		})
	}
	for _, value := range preparation {
		t.Run("document-supply-accepts-"+strings.ToLower(value.class), func(t *testing.T) {
			parse(t, artifact("DOCUMENT_SUPPLY", value), true)
		})
	}
}

func v2Record(v map[string]any) map[string]any {
	return v["records"].([]any)[0].(map[string]any)
}

// Kept behind a variable so the golden-byte test's filesystem read remains explicit.
var osReadFile = func(path string) ([]byte, error) { return os.ReadFile(path) }
