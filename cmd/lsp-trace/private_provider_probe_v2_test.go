package main

import (
	"bytes"
	"encoding/json"
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
		{"unknown", append(append([]string{}, base...), "--private-provider-probe-version", "v3")},
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
	for _, version := range []string{"", "v1", "v2"} {
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
		{"negative-messages", -1, 64, 0, 100, "", true},
		{"negative-bytes", 0, 64, -1, 100, "", true},
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

	t.Run("exact-arbitrary-pairing-reproducer", func(t *testing.T) {
		const raw = "{\"schema\":\"lsp-trace.private-provider-probe.v2\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[{\"cause\":\"RESOURCE_EXHAUSTED\",\"count\":1}],\"records\":[{\"sequence\":1,\"phase\":\"DOCUMENT_SYMBOL\",\"class\":\"DECODE_FAILURE\",\"cause\":\"RESOURCE_EXHAUSTED\"}],\"rerun_count\":0,\"work\":1}\n"
		if _, err := parsePrivateProviderProbeV2([]byte(raw)); err == nil {
			t.Fatal("accepted arbitrary class/cause pairing")
		}
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
		a.Work = 3
		a.CauseCounts[0].Count = 3
		parse(t, a, true)
	})
	t.Run("success-remainder", func(t *testing.T) {
		a := clone()
		a.Work = 3
		parse(t, a, true)
	})
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
		return privateProviderProbeArtifactV2{Schema: privateProviderProbeSchemaV2, Status: "COMPLETE", CauseCounts: counts, Records: records, Work: work}
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
