//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
	"lsp-trace/internal/targetpacket"
)

func preparationOwnedInput(t *testing.T) (prep.Input, func() (int, int)) {
	t.Helper()
	h, m, cfg, f := pilot.NewScopedPartialReadableRootFixturePrivate(t)
	report, err := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var inputs [3]*publication.ContentRetentionRoot
	var outputs [2]*publication.ContentRetentionRoot
	for i, path := range cfg.InputRoots {
		inputs[i], err = publication.OpenContentRetentionRoot(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer inputs[i].Close()
	}
	for i, path := range cfg.OutputRoots[:2] {
		outputs[i], err = publication.OpenContentRetentionRoot(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer outputs[i].Close()
	}
	pins := pilot.RetainedPartialQualificationPinsPrivate{Schema: "lsp-trace.retained-partial-qualification-input.private.v1", CodeHEAD: cfg.CodeHEAD, VerifierSourceSHA256: pilot.RetainedPartialQualificationVerifierPinPrivate(), PolicySHA256: pilot.RetainedPartialQualificationPolicyDigestPrivate(), MaxReadBytes: 128 << 20, MaxWorkBytes: 2 << 30}
	records := append(append([]pilot.ScopedRehearsalRecordPrivate{}, report.Records...), *report.OutcomeReceipt)
	for i, r := range records {
		pins.Objects[i] = pilot.RetainedPartialObjectPinPrivate{Name: r.Name, ID: r.ID, Bytes: uint64(r.Bytes)}
	}
	raw, _ := json.Marshal(pins)
	in, err := pilot.NewExactRetainedPartialQualificationInput(raw, inputs, outputs)
	if err != nil {
		t.Fatal(err)
	}
	q, err := pilot.QualifyRetainedPartialNativePrivate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range inputs {
		if err = r.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range outputs {
		if err = r.Close(); err != nil {
			t.Fatal(err)
		}
	}
	proof, err := pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, pins, pilot.RetainedPartialDerivationBudgetPrivate{MaxInputBytes: 8 << 20, MaxOutputBytes: 8 << 20, MaxWorkBytes: 2 << 30})
	if err != nil {
		t.Fatal(err)
	}
	p, fail := derived.Project(proof)
	if fail != nil {
		t.Fatal(fail)
	}
	a, err := derived.ComputeInstabilityPrivate(context.Background(), p, []uint64{42, 43, 44, 45, 46, 47})
	if err != nil {
		t.Fatal(err)
	}
	// All temporary roots are closed before the new builder receives an input.
	return prep.Input{Projection: p, Instability: a}, func() (int, int) { a, b := f.Traffic(); return int(a), int(b) }
}

func TestOwnedPrivateDescribePreparation(t *testing.T) {
	in, traffic := preparationOwnedInput(t)
	before, bp := traffic()
	limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	for _, count := range []int{1, 4, 25} {
		input := in
		e, carrier, err := in.Projection.PreparationEvidencePrivate()
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range carrier.Evidence.Acquisition.Targets[:count] {
			input.Targets = append(input.Targets, target.Requested.ID)
		}
		start := time.Now()
		r, err := prep.Prepare(context.Background(), []prep.Input{input}, limits)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Coverage) != 1 || r.Coverage[0].OriginalTargets != 25 || r.Coverage[0].OriginalSymbols != 219 || r.Coverage[0].Requested != count || r.Coverage[0].Prepared != 1 || r.Coverage[0].SourceGap != count-1 || r.Coverage[0].ResourceGap != 0 || len(r.Packets) != 1 {
			t.Fatalf("coverage %+v index %+v", r.Coverage, r.Index)
		}
		if r.Coverage[0].DiscoverySHA256 != pilot.Digest(e.Records[3]) {
			t.Fatal("lost original census evidence")
		}
		if r.Packets[0].Target != "root" || len(r.Packets[0].Parts) == 0 || r.Packets[0].Parts[0].Prompt == "" {
			t.Fatal("no positive packet/prompt")
		}
		privatePacketBytes, _ := json.Marshal(r.Packets[0])
		if _, err := targetpacket.Validate(privatePacketBytes); err == nil {
			t.Fatal("private preparation admitted as ordinary Describe packet")
		}
		declarations, runs, pairs, members := 0, 0, 0, 0
		for _, part := range r.Packets[0].Parts {
			for _, rec := range part.Records {
				switch rec.Kind {
				case "RETAINED_SOURCE_AST_DECLARATION":
					declarations++
				case "RUN_COMMUNITY":
					runs++
				case "INSTABILITY_PAIR":
					pairs++
				case "COMMUNITY_MEMBER":
					members++
				}
			}
		}
		if declarations != 1 || runs != 6 || pairs != 15 || members != 6 {
			t.Fatalf("records %d %d %d %d", declarations, runs, pairs, members)
		}
		again, err := prep.Prepare(context.Background(), []prep.Input{input}, limits)
		if err != nil || !bytes.Equal(r.CanonicalBytes(), again.CanonicalBytes()) {
			t.Fatal("nondeterministic", err)
		}
		t.Logf("requested=%d original=25 prepared=1 sourcegap=%d parts=%d bytes=%d elapsed=%s instability=%s", count, count-1, r.PartCount, r.PartBytes, time.Since(start), in.Instability.Outcome)
	}
	// Force bounded multi-part output while preserving all target-owned rows.
	one := in
	one.Targets = []string{"root"}
	multipart := limits
	multipart.MaxPartBytes = 8 << 10
	multi, err := prep.Prepare(context.Background(), []prep.Input{one}, multipart)
	if err != nil || len(multi.Packets) != 1 || multi.PartCount < 2 {
		t.Fatal("multipart positive", err, multi.Index)
	}
	members := 0
	for _, part := range multi.Packets[0].Parts {
		for _, r := range part.Records {
			if r.Kind == "COMMUNITY_MEMBER" {
				members++
			}
		}
	}
	if members != 6 {
		t.Fatal("multipart lost memberships")
	}
	multipart.MaxParts = 1
	limited, err := prep.Prepare(context.Background(), []prep.Input{one}, multipart)
	if err != nil || limited.Coverage[0].ResourceGap != 1 || len(limited.Packets) != 0 {
		t.Fatal("part resource gap", err)
	}
	// Mutating detached evidence cannot mutate sealed producer state.
	_, carrier, _ := in.Projection.PreparationEvidencePrivate()
	carrier.Evidence.Captures[0].Content = nil
	one.Targets = []string{"root", "root"}
	if _, err := prep.Prepare(context.Background(), []prep.Input{one}, limits); err == nil {
		t.Fatal("duplicate selection")
	}
	one.Targets = []string{"not-a-nomination"}
	if _, err := prep.Prepare(context.Background(), []prep.Input{one}, limits); err == nil {
		t.Fatal("unknown selection")
	}
	one.Targets = []string{"root"}
	small := limits
	small.MaxTargets = 1
	r, err := prep.Prepare(context.Background(), []prep.Input{in}, small)
	if err != nil || r.Coverage[0].ResourceGap != 24 {
		t.Fatal("untouched accounting", err, r.Coverage)
	}
	changed := in
	changed.Instability = derived.CloneInstabilityPrivate(in.Instability)
	changed.Instability.InputSHA256 = "forged"
	if _, err := prep.Prepare(context.Background(), []prep.Input{changed}, limits); err == nil {
		t.Fatal("tampered assessment accepted")
	}
	var decoded derived.InstabilityPrivate
	b, _ := json.Marshal(in.Instability)
	_ = json.Unmarshal(b, &decoded)
	changed = in
	changed.Instability = decoded
	if _, err := prep.Prepare(context.Background(), []prep.Input{changed}, limits); err == nil {
		t.Fatal("DTO seal accepted")
	}
	after, ap := traffic()
	if before != after || bp != ap {
		t.Fatal("preparation performed traffic")
	}
}

func TestIndependentPrivatePreparationInputs(t *testing.T) {
	a, _ := preparationOwnedInput(t)
	b, _ := preparationOwnedInput(t)
	a.Targets = []string{"root"}
	b.Targets = []string{"root"}
	limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	r, err := prep.Prepare(context.Background(), []prep.Input{a, b}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Packets) != 2 || r.Packets[0].InputSHA256 == r.Packets[1].InputSHA256 || r.Packets[0].ID == r.Packets[1].ID {
		t.Fatal("independent inputs merged")
	}
	mixed := a
	mixed.Instability = b.Instability
	if _, err := prep.Prepare(context.Background(), []prep.Input{mixed}, limits); err == nil {
		t.Fatal("cross-input assessment accepted")
	}
}
