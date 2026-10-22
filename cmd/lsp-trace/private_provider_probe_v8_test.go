package main

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"lsp-trace/sessionruntime"
)

func TestPrivateProviderProbeV8VersionAdmission(t *testing.T) {
	if _, err := newPrivateProviderProbeVersion(probePath(t), "v8"); err != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_VERSION err=%v", err)
	}
}

func completePrivateProviderProbeV8Artifact() privateProviderProbeArtifactV8 {
	p := &privateProviderProbe{version: "v8"}
	completed := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1)
	completed.CompletionSequence = 1
	p.observeV8("DOCUMENT_SYMBOL", "", "", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, completed, true, 64, 4096, 1, 10)
	nonPoison := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1)
	nonPoison.CompletionSequence = 2
	p.observeV8("PROVIDER_REQUEST", "JSON_RPC_CODE_BUCKET", "METHOD_NOT_FOUND", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, nonPoison, true, 64, 4096, 1, 20)
	poison := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecyclePoisoned, 0, 0)
	poison.CompletionSequence = 3
	p.observeV8("PROVIDER_REQUEST", "TYPED_SESSION_RUNTIME_FAILURE", "SESSION_POISONED", sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, poison, true, 64, 4096, 0, 30)
	p.observeV8("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", "LIFECYCLE_CONFLICT", sessionruntime.ProviderMessageComposition{}, false, sessionruntime.ProviderTransactionObservation{}, false, 0, 0, 0, 0)
	return p.artifactV8()
}

func marshalPrivateProviderProbeV8TestArtifact(t *testing.T, artifact privateProviderProbeArtifactV8) []byte {
	t.Helper()
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestPrivateProviderProbeV8StrictParserClosure(t *testing.T) {
	tests := []struct {
		name      string
		wantValid bool
		build     func(*testing.T) []byte
	}{
		{name: "canonical complete", wantValid: true, build: func(t *testing.T) []byte {
			return marshalPrivateProviderProbeV8TestArtifact(t, completePrivateProviderProbeV8Artifact())
		}},
		{name: "canonical incomplete", wantValid: true, build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.Status, a.FirstPoison = "INCOMPLETE", nil
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "unknown field", build: func(t *testing.T) []byte {
			var loose map[string]any
			if err := json.Unmarshal(marshalPrivateProviderProbeV8TestArtifact(t, completePrivateProviderProbeV8Artifact()), &loose); err != nil {
				t.Fatal(err)
			}
			loose["extra"] = true
			raw, _ := json.Marshal(loose)
			return append(raw, '\n')
		}},
		{name: "missing newline", build: func(t *testing.T) []byte {
			raw := marshalPrivateProviderProbeV8TestArtifact(t, completePrivateProviderProbeV8Artifact())
			return raw[:len(raw)-1]
		}},
		{name: "double newline", build: func(t *testing.T) []byte {
			return append(marshalPrivateProviderProbeV8TestArtifact(t, completePrivateProviderProbeV8Artifact()), '\n')
		}},
		{name: "noncanonical json", build: func(t *testing.T) []byte {
			return append([]byte{' '}, marshalPrivateProviderProbeV8TestArtifact(t, completePrivateProviderProbeV8Artifact())...)
		}},
		{name: "forbidden privacy field", build: func(t *testing.T) []byte {
			var loose map[string]any
			if err := json.Unmarshal(marshalPrivateProviderProbeV8TestArtifact(t, completePrivateProviderProbeV8Artifact()), &loose); err != nil {
				t.Fatal(err)
			}
			loose["path"] = "/forbidden"
			raw, _ := json.Marshal(loose)
			return append(raw, '\n')
		}},
		{name: "bad schema", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.Schema = "v9"
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "bad status", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.Status = "UNKNOWN"
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "invalid witness vocabulary", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.FirstCompleted.Phase = "SECRET"
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "incoherent nonpoison lifecycle", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.FirstNonPoisonFailure.Transaction.TerminalLifecycle = sessionruntime.ProviderTransactionLifecyclePoisoned
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "incoherent poison selection", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.FirstPoison.Composition.Selection = "FIRST_COMPLETED"
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "incoherent completion order", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.FirstCompleted.Composition.Sequence, a.FirstCompleted.Transaction.CompletionSequence = 4, 4
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "complete missing witness", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.FirstPoison = nil
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
		{name: "complete missing dispatched", build: func(t *testing.T) []byte {
			a := completePrivateProviderProbeV8Artifact()
			a.MissingDispatchedObservationCount = 1
			return marshalPrivateProviderProbeV8TestArtifact(t, a)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parsePrivateProviderProbeV8(test.build(t))
			if (err == nil) != test.wantValid {
				t.Fatalf("ASSERT_PROVIDER_PROBE_V8_STRICT_PARSER want_valid=%v err=%v", test.wantValid, err)
			}
		})
	}
}

func observePrivateProviderProbeV8TestCompletion(p *privateProviderProbe, kind string, sequence uint64) {
	switch kind {
	case "success":
		o := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1)
		o.CompletionSequence = sequence
		p.observeV8("DOCUMENT_SYMBOL", "", "", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, o, true, 64, 4096, 1, int64(sequence))
	case "nonpoison":
		o := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1)
		o.CompletionSequence = sequence
		p.observeV8("PROVIDER_REQUEST", "JSON_RPC_CODE_BUCKET", "METHOD_NOT_FOUND", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, o, true, 64, 4096, 1, int64(sequence))
	case "poison":
		o := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecyclePoisoned, 0, 0)
		o.CompletionSequence = sequence
		p.observeV8("PROVIDER_REQUEST", "TYPED_SESSION_RUNTIME_FAILURE", "SESSION_POISONED", sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, o, true, 64, 4096, 0, int64(sequence))
	}
}

func TestPrivateProviderProbeV8ProducerFinalizeStrictParseAcceptsIdenticalEarliestOverlap(t *testing.T) {
	for _, earliest := range []string{"nonpoison", "poison"} {
		t.Run(earliest, func(t *testing.T) {
			path := probePath(t)
			p, err := newPrivateProviderProbeVersion(path, "v8")
			if err != nil {
				t.Fatal(err)
			}
			observePrivateProviderProbeV8TestCompletion(p, earliest, 1)
			if earliest == "nonpoison" {
				observePrivateProviderProbeV8TestCompletion(p, "poison", 2)
			} else {
				observePrivateProviderProbeV8TestCompletion(p, "nonpoison", 2)
			}
			observePrivateProviderProbeV8TestCompletion(p, "success", 3)
			if err := p.finalize(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parsePrivateProviderProbeV8(raw); err != nil {
				t.Fatalf("ASSERT_PROVIDER_PROBE_V8_IDENTICAL_EARLIEST_%s_OVERLAP err=%v artifact=%s", earliest, err, raw)
			}
		})
	}
}

func TestPrivateProviderProbeV8ProducerFinalizeStrictParseAcceptsEarliestSuccess(t *testing.T) {
	path := probePath(t)
	p, err := newPrivateProviderProbeVersion(path, "v8")
	if err != nil {
		t.Fatal(err)
	}
	observePrivateProviderProbeV8TestCompletion(p, "success", 1)
	observePrivateProviderProbeV8TestCompletion(p, "nonpoison", 2)
	observePrivateProviderProbeV8TestCompletion(p, "poison", 3)
	if err := p.finalize(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsePrivateProviderProbeV8(raw); err != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_EARLIEST_SUCCESS_ROUND_TRIP err=%v artifact=%s", err, raw)
	}
}

func TestPrivateProviderProbeV8StrictParserRejectsEqualSequenceDistinctWitnesses(t *testing.T) {
	for _, classified := range []string{"nonpoison", "poison"} {
		t.Run(classified, func(t *testing.T) {
			a := completePrivateProviderProbeV8Artifact()
			var source *privateProviderProbeWitnessV7
			if classified == "nonpoison" {
				source = a.FirstNonPoisonFailure
			} else {
				source = a.FirstPoison
			}
			distinct := *source
			distinct.Phase = "DOCUMENT_SYMBOL"
			a.FirstCompleted = &distinct
			raw := marshalPrivateProviderProbeV8TestArtifact(t, a)
			if _, err := parsePrivateProviderProbeV8(raw); err == nil {
				t.Fatalf("ASSERT_PROVIDER_PROBE_V8_EQUAL_SEQUENCE_DISTINCT_%s_WITNESSES accepted artifact=%s", classified, raw)
			}
		})
	}
}

func TestPrivateProviderProbeV8StrictParserRejectsDuplicateClassifiedCompletionSequence(t *testing.T) {
	a := completePrivateProviderProbeV8Artifact()
	a.FirstPoison.Composition.Sequence = a.FirstNonPoisonFailure.Transaction.CompletionSequence
	a.FirstPoison.Transaction.CompletionSequence = a.FirstNonPoisonFailure.Transaction.CompletionSequence
	raw := marshalPrivateProviderProbeV8TestArtifact(t, a)
	if _, err := parsePrivateProviderProbeV8(raw); err == nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_DISTINCT_CLASSIFIED_COMPLETION_SEQUENCE accepted artifact=%s", raw)
	}
}

func TestPrivateProviderProbeV8ProducerRejectsIncoherentObservation(t *testing.T) {
	p := &privateProviderProbe{version: "v8"}
	incoherent := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1)
	incoherent.CompletionSequence = 1
	incoherent.SelectedResponseObserved = false
	incoherent.SelectedResponseReadCompleted = true
	p.observeV8("DOCUMENT_SYMBOL", "", "", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, incoherent, true, 64, 4096, 1, 10)
	observePrivateProviderProbeV8TestCompletion(p, "nonpoison", 2)
	observePrivateProviderProbeV8TestCompletion(p, "poison", 3)
	a := p.artifactV8()
	retainedIncoherent := false
	for _, witness := range []*privateProviderProbeWitnessV7{a.FirstCompleted, a.FirstNonPoisonFailure, a.FirstPoison} {
		retainedIncoherent = retainedIncoherent || witness != nil && witness.Transaction.CompletionSequence == 1
	}
	if a.Status != "INCOMPLETE" || retainedIncoherent {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_INCOHERENT_OBSERVATION_FAILS_CLOSED status=%q retained_incoherent=%t artifact=%+v", a.Status, retainedIncoherent, a)
	}
}

func TestPrivateProviderProbeV8EarliestByManagerCompletionReverseDelivery(t *testing.T) {
	p := &privateProviderProbe{version: "v8"}
	for _, delivered := range []struct {
		kind     string
		sequence uint64
	}{{"poison", 30}, {"nonpoison", 20}, {"poison", 25}, {"nonpoison", 15}, {"success", 10}} {
		observePrivateProviderProbeV8TestCompletion(p, delivered.kind, delivered.sequence)
	}
	a := p.artifactV8()
	if a.FirstCompleted == nil || a.FirstCompleted.Transaction.CompletionSequence != 10 || a.FirstNonPoisonFailure == nil || a.FirstNonPoisonFailure.Transaction.CompletionSequence != 15 || a.FirstPoison == nil || a.FirstPoison.Transaction.CompletionSequence != 25 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_REVERSE_COMPLETION_ORDER artifact=%+v", a)
	}
}

func TestPrivateProviderProbeV8EarliestByManagerCompletionConcurrent(t *testing.T) {
	p := &privateProviderProbe{version: "v8"}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		for offset, kind := range []string{"success", "nonpoison", "poison"} {
			wg.Add(1)
			sequence := uint64(i*3 + offset + 1)
			go func() {
				defer wg.Done()
				observePrivateProviderProbeV8TestCompletion(p, kind, sequence)
			}()
		}
	}
	wg.Wait()
	a := p.artifactV8()
	if a.FirstCompleted == nil || a.FirstCompleted.Transaction.CompletionSequence != 1 || a.FirstNonPoisonFailure == nil || a.FirstNonPoisonFailure.Transaction.CompletionSequence != 2 || a.FirstPoison == nil || a.FirstPoison.Transaction.CompletionSequence != 3 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_CONCURRENT_COMPLETION_ORDER artifact=%+v", a)
	}
}

func TestPrivateProviderProbeV8CompletionSequenceExhaustionFailsClosed(t *testing.T) {
	p := &privateProviderProbe{version: "v8"}
	p.observeV8("PROVIDER_REQUEST", "", "", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, sessionruntime.ProviderTransactionObservation{}, false, 64, 4096, 1, 10)
	a := p.artifactV8()
	if a.Status != "INCOMPLETE" || a.MissingDispatchedObservationCount != 1 || a.PreProviderRefusalCount != 0 || a.FirstCompleted != nil || a.FirstNonPoisonFailure != nil || a.FirstPoison != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_SEQUENCE_EXHAUSTION artifact=%+v", a)
	}
	p.observeV8("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", "LIFECYCLE_CONFLICT", sessionruntime.ProviderMessageComposition{}, false, sessionruntime.ProviderTransactionObservation{}, false, 0, 0, 0, 0)
	a = p.artifactV8()
	if a.MissingDispatchedObservationCount != 1 || a.PreProviderRefusalCount != 1 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_SEQUENCE_EXHAUSTION_DISJOINT artifact=%+v", a)
	}
	raw := marshalPrivateProviderProbeV8TestArtifact(t, a)
	parsed, err := parsePrivateProviderProbeV8(raw)
	if err != nil || parsed.Status != "INCOMPLETE" || parsed.MissingDispatchedObservationCount != 1 || parsed.PreProviderRefusalCount != 1 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_SEQUENCE_EXHAUSTION_PARSE err=%v artifact=%+v", err, parsed)
	}
}

func TestPrivateProviderProbeV8ArtifactContractRED(t *testing.T) {
	path := probePath(t)
	p, err := newPrivateProviderProbeVersion(path, "v8")
	if err != nil {
		t.Fatal(err)
	}
	completed := providerProbeV7Observation("READY", 0, 1)
	completed.CompletionSequence = 1
	p.observeV8("DOCUMENT_SYMBOL", "", "", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, completed, true, 64, 4096, 1, 10)
	nonPoison := providerProbeV7Observation("READY", 0, 1)
	nonPoison.CompletionSequence = 2
	p.observeV8("PROVIDER_REQUEST", "JSON_RPC_CODE_BUCKET", "METHOD_NOT_FOUND", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, nonPoison, true, 64, 4096, 1, 20)
	poison := providerProbeV7Observation("POISONED", 0, 0)
	poison.CompletionSequence = 3
	p.observeV8("PROVIDER_REQUEST", "TYPED_SESSION_RUNTIME_FAILURE", "SESSION_POISONED", sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, poison, true, 64, 4096, 0, 30)
	p.observeV8("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", "LIFECYCLE_CONFLICT", sessionruntime.ProviderMessageComposition{}, false, sessionruntime.ProviderTransactionObservation{}, false, 0, 0, 0, 0)
	if err := p.finalize(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact map[string]json.RawMessage
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	var schema, status string
	_ = json.Unmarshal(artifact["schema"], &schema)
	_ = json.Unmarshal(artifact["status"], &status)
	if schema != "lsp-trace.private-provider-probe.v8" || artifact["first_completed"] == nil || artifact["first_non_poison_failure"] == nil || artifact["first_poison"] == nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_THREE_WITNESSES schema=%q artifact=%s", schema, raw)
	}
	var refused, missing uint64
	_ = json.Unmarshal(artifact["pre_provider_refusal_count"], &refused)
	_ = json.Unmarshal(artifact["missing_dispatched_observation_count"], &missing)
	if refused != 1 || missing != 0 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_DISPATCH_COUNTS refused=%d missing=%d artifact=%s", refused, missing, raw)
	}
	if status != "COMPLETE" {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V8_COMPOSED_COMPLETE status=%q artifact=%s", status, raw)
	}
}
