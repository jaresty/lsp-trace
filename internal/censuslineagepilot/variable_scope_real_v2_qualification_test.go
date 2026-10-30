//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
	"lsp-trace/sessionruntime"
)

// Independently reporting checks: a failure never prevents the remaining
// applicable assertions from being evaluated. Setup errors are not witnesses.
func qualificationCheck(t *testing.T, path string, ok bool, detail any) {
	t.Helper()
	if ok {
		t.Logf("ASSERT_QUAL/%s: PASS %v", path, detail)
	} else {
		t.Errorf("ASSERT_QUAL/%s: FAIL %v", path, detail)
	}
}

// The script is fixed before execution, including responses for uncalled stages.
// Nominal availability is not inferred from callbacks having run.
type qualificationScript struct {
	nominal     bool
	backendKind string
}

func (s qualificationScript) factory(t *testing.T) variableAutomaticFactory {
	return variableAutomaticOwnedFactory(t, func() {}, s.backendKind)
}

type qualificationPinContext struct {
	context.Context
	n      atomic.Int64
	mutate func()
}

func (c *qualificationPinContext) Deadline() (time.Time, bool) {
	if c.n.Add(1) == 2 {
		c.mutate()
	}
	return time.Time{}, false
}
func TestVariableCallerQualificationPins(t *testing.T) {
	for _, kind := range []string{"config", "source", "executable"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableAutomaticConfig(t, 1)
			factory, starts, _, _ := variableOwnedFactory(t, "")
			changed := false
			ctx := &qualificationPinContext{Context: context.Background(), mutate: func() {
				var e error
				switch kind {
				case "config":
					cc := c
					cc.Automation.TimeoutMS++
					cc.Automation.ApprovalSHA256 = prep.AutomaticHTTPApprovalPrivateV2(*cc.Automation)
					variableSave(t, path, cc)
				case "source":
					e = os.WriteFile(filepath.Join(c.Scope.Root, c.Scope.Members[0].CanonicalRelativePath), []byte("package owned\nfunc changed() {}\n"), 0600)
				case "executable":
					e = os.WriteFile(c.Server, []byte("changed owned executable; never executed\n"), 0700)
				}
				if e != nil {
					t.Fatal(e)
				}
				changed = true
			}}
			_, e := variableRunWithAutomatic(ctx, "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, qualificationScript{nominal: true}.factory(t))
			_, markerErr := os.Stat(c.AttemptMarkerPath)
			qualificationCheck(t, "D/"+kind+"/changed", changed, "input changed between preflight and claim")
			qualificationCheck(t, "D/"+kind+"/no-claim", e != nil && os.IsNotExist(markerErr), markerErr)
			qualificationCheck(t, "D/"+kind+"/no-start", starts.Load() == 0, starts.Load())
		})
	}
}

// Expected namespaces are recomputed from the sealed preparation source binding;
// expected target labels come from its own acquisition selection, never inventory.
type qualificationTargetKey struct{ InputSHA256, Target string }

func qualificationExpectedKeys(coverage []prep.Coverage) map[qualificationTargetKey]int {
	keys := map[qualificationTargetKey]int{}
	for _, cv := range coverage {
		if cv.PrivateV2 == nil {
			return nil
		}
		raw, err := json.Marshal(struct {
			RequestType string
			Source      derived.SourceBindingPrivateV2
		}{prep.RequestTypePrivateV2, cv.PrivateV2.Source})
		if err != nil {
			return nil
		}
		input := pilot.Digest(raw)
		if input != cv.InputSHA256 {
			return nil
		}
		for _, target := range cv.Selection {
			keys[qualificationTargetKey{input, target}]++
		}
	}
	return keys
}
func qualificationInventoryKeys(r variableCallerReport) map[qualificationTargetKey]int {
	keys := map[qualificationTargetKey]int{}
	if r.Inventory == nil {
		return nil
	}
	for _, row := range r.Inventory.Rows {
		keys[qualificationTargetKey{row.InputSHA256, row.Target}]++
	}
	return keys
}
func TestVariableCallerQualificationConditionalDenominator(t *testing.T) {
	for _, kind := range []string{"before-admission", "acquisition-failure", "semantic-unavailable"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableAutomaticConfig(t, 3)
			runtimeKind := ""
			script := qualificationScript{nominal: false, backendKind: "unavailable"}
			if kind == "before-admission" {
				c.Automation.ApprovalSHA256 = "refused"
				variableSave(t, path, c)
			}
			if kind == "acquisition-failure" {
				runtimeKind = "capture-second-failed"
			}
			factory, _, _, _ := variableOwnedFactory(t, runtimeKind)
			r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, script.factory(t))
			switch kind {
			case "before-admission":
				qualificationCheck(t, "C/unknown-before-admission", e != nil && !r.Acquisition.ExpectedBatchesKnown && r.Inventory == nil, "no invented denominator")
			case "acquisition-failure":
				ok := e != nil && r.Acquisition.ExpectedBatchesKnown && len(r.Acquisition.Batches) == 3 && !r.Acquisition.Complete
				for i, b := range r.Acquisition.Batches {
					ok = ok && b.Ordinal == i
				}
				qualificationCheck(t, "C/known-acquisition-failure", ok, r.Acquisition.Batches)
			case "semantic-unavailable":
				expected := qualificationExpectedKeys(r.Coverage)
				ok := e == nil && r.Inventory != nil && len(expected) == 3 && reflect.DeepEqual(expected, qualificationInventoryKeys(r))
				if r.Inventory != nil {
					for _, row := range r.Inventory.Rows {
						ok = ok && row.InputSHA256 != "" && row.Target != "" && row.Status == "UNAVAILABLE"
					}
				}
				qualificationCheck(t, "C/known-semantic-failure", ok, fmt.Sprint("expected=", expected, " actual=", qualificationInventoryKeys(r)))
			}
		})
	}
}
func qualificationReadPacket(c variableCallerConfig, r variableCallerReport) (pilot.ProducerPacketInspectionPrivateV2, []byte, error) {
	var packet pilot.ProducerPacketInspectionPrivateV2
	var first []byte
	for i, path := range c.ProducerOutputs {
		receipt := r.ProducerCopies[i].Receipt
		if receipt == nil {
			return packet, nil, fmt.Errorf("missing actual copy receipt %d", i)
		}
		root, e := publication.OpenContentRetentionRoot(context.Background(), path)
		if e != nil {
			return packet, nil, e
		}
		raw, e := root.ReadProducerPacketPrivateV2(context.Background(), receipt.Digest, int64(receipt.ByteLength))
		ce := root.Close()
		if e != nil || ce != nil {
			return packet, nil, fmt.Errorf("read actual packet: %v %v", e, ce)
		}
		if pilot.Digest(raw) != receipt.Digest || uint64(len(raw)) != receipt.ByteLength {
			return packet, nil, fmt.Errorf("copy fingerprint")
		}
		if i == 0 {
			first = raw
		} else if !bytes.Equal(first, raw) {
			return packet, nil, fmt.Errorf("copies differ")
		}
	}
	if e := json.Unmarshal(first, &packet); e != nil {
		return packet, nil, e
	}
	return packet, first, nil
}
func TestVariableCallerQualificationArtifacts(t *testing.T) {
	if !*qualificationCoveredChild {
		qualificationRunCovered(t, "^TestVariableCallerQualificationArtifacts$")
		return
	}
	qualificationInstallCleanupObserver()
	for _, kind := range []string{"nominal", "semantic-unavailable"} {
		t.Run(kind, func(t *testing.T) { qualificationArtifacts(t, kind) })
	}
}
func qualificationArtifacts(t *testing.T, kind string) {
	qualificationCleanupMu.Lock()
	qualificationCaptureEntries = nil
	qualificationCleanupMu.Unlock()
	c, path, git := variableAutomaticConfig(t, 3)
	c.Seeds = []uint64{42, 43, 47}
	variableSave(t, path, c)
	factory, starts, _, _ := variableOwnedFactory(t, "")
	script := qualificationScript{nominal: kind == "nominal"}
	if kind == "semantic-unavailable" {
		script.backendKind = "unavailable"
	}
	r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, script.factory(t))
	if script.nominal {
		qualificationCheck(t, "A/exogenous-nominal", e == nil && r.Phase == "PROVISIONAL_COMPLETE" && r.Outcome != nil && starts.Load() == 1, fmt.Sprint("script nominal=", script.nominal, " terminal=", r.Phase, " error=", e))
	} else {
		t.Log("ASSERT_QUAL/A/exogenous-nominal: INAPPLICABLE exogenous semantic-unavailable script")
	}
	// Exogenous fixture availability, not an implementation-controlled phase label.
	// This fixture supplies writable owned retention roots and no retention fault.
	retentionAvailable := true
	observedCaptures := qualificationCaptures()
	t.Logf("actual successful capture returns=%d retention availability=%v", len(observedCaptures), retentionAvailable)
	if len(observedCaptures) != 3 || !retentionAvailable {
		t.Log("ASSERT_QUAL/G/exact-original-artifacts: INAPPLICABLE acquisition or external retention antecedent false")
		if !r.Acquisition.ExpectedBatchesKnown {
			t.Log("ASSERT_QUAL/C/known-final-denominator: INAPPLICABLE unknown denominator")
		}
		t.Log("ASSERT_QUAL/H/all: UNOBSERVED no retained computation input")
		return
	}
	packet, raw, pe := qualificationReadPacket(c, r)
	qualificationCheck(t, "G/actual-copy-read", pe == nil, pe)
	retained := pe == nil && len(packet.Batches) == 3
	for i, b := range packet.Batches {
		retained = retained && i < len(r.Acquisition.Batches) && i < len(observedCaptures) && b.Ordinal == i
		if i < len(r.Acquisition.Batches) && i < len(observedCaptures) {
			retained = retained && pilot.Digest(b.SourceArtifact) == r.Acquisition.Batches[i].ArtifactSHA256 && uint64(len(b.SourceArtifact)) == r.Acquisition.Batches[i].ArtifactBytes && pilot.Digest(b.SourceArtifact) == observedCaptures[i].Digest && uint64(len(b.SourceArtifact)) == observedCaptures[i].Bytes
		}
	}
	qualificationCheck(t, "G/exact-original-artifacts", retained, fmt.Sprint("retained=", len(packet.Batches), " acquired=", len(r.Acquisition.Batches)))
	var env struct{ Preparation prep.Result }
	prepared := r.Outcome != nil && json.Unmarshal(r.Outcome.EnvelopeBytes(), &env) == nil
	if !prepared {
		t.Log("ASSERT_QUAL/C/known-final-denominator: UNOBSERVED preparation not reached; acquisition denominator remains recorded")
		t.Log("ASSERT_QUAL/H/all: UNOBSERVED no completed preparation carrier")
		return
	}
	expected := qualificationExpectedKeys(r.Coverage)
	qualificationCheck(t, "C/known-final-denominator", len(env.Preparation.Coverage) == 3 && len(expected) == 3 && reflect.DeepEqual(expected, qualificationInventoryKeys(r)), fmt.Sprint("expected=", expected, " actual=", qualificationInventoryKeys(r)))
	for i := 0; i < 3; i++ {
		var a derived.InstabilityReportPrivateV2
		found := false
		for _, p := range env.Preparation.Packets {
			for _, part := range p.Parts {
				for _, record := range part.Records {
					if record.Kind == "ASSESSMENT_PRIVATE_V2" {
						var v derived.InstabilityReportPrivateV2
						if json.Unmarshal(record.Value, &v) == nil && v.Source.BatchOrdinal == i {
							a = v
							found = true
						}
					}
				}
			}
		}
		prefix := fmt.Sprintf("H/batch-%d/", i)
		qualificationCheck(t, prefix+"assessment-present", found, found)
		seedOK := found && reflect.DeepEqual(a.Seeds, c.Seeds) && len(a.Runs) == len(c.Seeds)
		for j, run := range a.Runs {
			seedOK = seedOK && j < len(c.Seeds)
			if j < len(c.Seeds) {
				seedOK = seedOK && run.Seed == c.Seeds[j] && run.ID == fmt.Sprintf("run-%d-seed-%d", j, c.Seeds[j]) && (run.Status == "COMPLETE" || run.Status == "EMPTY")
			}
		}
		qualificationCheck(t, prefix+"exact-seeds", seedOK, a.Runs)
		pairs := map[string]int{}
		for _, p := range a.Pairs {
			pairs[p.Left+"|"+p.Right]++
		}
		pairOK := found && len(a.Pairs) == 3 && len(pairs) == 3
		for j := range c.Seeds {
			for k := j + 1; k < len(c.Seeds); k++ {
				pairOK = pairOK && pairs[fmt.Sprintf("run-%d-seed-%d|run-%d-seed-%d", j, c.Seeds[j], k, c.Seeds[k])] == 1
			}
		}
		qualificationCheck(t, prefix+"exact-unordered-pairs", pairOK, pairs)
		binding := found && retained && i < len(packet.Batches)
		if binding {
			b := packet.Batches[i]
			binding = a.Source.SourceArtifactSHA256 == pilot.Digest(b.SourceArtifact) && a.Source.CanonicalSeedsSHA256 == pilot.Digest(b.Seeds) && a.Source.CaptureOriginSHA256 == pilot.Digest(b.Origin) && a.Source.PacketSHA256 == pilot.Digest(raw) && a.Source.BatchOrdinal == i && a.Source.SessionID == c.Session.SessionID && a.Source.Generation == c.Session.Generation
		}
		qualificationCheck(t, prefix+"exact-artifact-binding", binding, a.Source.SourceArtifactSHA256)
	}
}
func TestVariableCallerQualificationSameClaim(t *testing.T) {
	_, path, git := variableAutomaticConfig(t, 1)
	var startCounters []*atomic.Int64
	var children []*variableOwnedChild
	factory := func() (*sessionruntime.Manager, error) {
		f, s, ch, _ := variableOwnedFactory(t, "")
		startCounters = append(startCounters, s)
		children = append(children, ch)
		return f()
	}
	counts := func() (int64, int64) {
		var starts, traffic int64
		for _, s := range startCounters {
			starts += s.Load()
		}
		for _, ch := range children {
			traffic += ch.traffic.Load()
		}
		return starts, traffic
	}
	// A separate counter at the existing factory boundary makes repeated entry
	// visible even when the first owned child has already retired.
	entries := 0
	wrapped := func() (*sessionruntime.Manager, error) { entries++; return factory() }
	script := qualificationScript{nominal: true}
	first, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, wrapped, variableClose, variableOwnedCodeInputs, script.factory(t))
	qualificationCheck(t, "W/first-owned-completion", e == nil && first.Phase == "PROVISIONAL_COMPLETE", e)
	before := entries
	beforeStarts, beforeTraffic := counts()
	second, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, wrapped, variableClose, variableOwnedCodeInputs, script.factory(t))
	qualificationCheck(t, "W/same-config-no-additional-factory", e != nil && entries == before, fmt.Sprint("first=", before, " after=", entries, " phase=", second.Phase, " refusal=", e))
	afterStarts, afterTraffic := counts()
	qualificationCheck(t, "W/same-claim-no-additional-start", afterStarts == beforeStarts, fmt.Sprint(beforeStarts, " -> ", afterStarts))
	qualificationCheck(t, "W/same-claim-no-additional-traffic", afterTraffic == beforeTraffic, fmt.Sprint(beforeTraffic, " -> ", afterTraffic))
}
