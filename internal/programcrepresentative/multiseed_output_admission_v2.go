package programcrepresentative

import "fmt"

const fixedDigestWidthPlaceholderV2 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func admitDenominatorBeforeOutputCopiesV2(state *checkedQualificationCalculationStateV2, a DenominatorAccountingV2) error {
	if state == nil {
		return fmt.Errorf("missing v2 denominator context")
	}
	s := state.subjectDTO
	a.D = uint64(len(state.mechanics))
	a.G = 0
	var err error
	for id := range state.mechanics {
		m, ok := state.mechanicsHandles[id]
		if !ok {
			return fmt.Errorf("missing v2 mechanics handle")
		}
		a.G, err = checkedSumV2(a.G, uint64(len(m.dto.SelectionContext.SeedGroups)))
		if err != nil {
			return err
		}
	}
	a.Evaluated, err = checkedSumV2(a.C, a.E)
	if err != nil {
		return err
	}
	a.Excluded = a.R
	a.TerminalRuns, err = checkedProductV2(a.P, 6)
	if err != nil {
		return err
	}
	a.EligibleRuns, err = checkedProductV2(a.C, 6)
	if err != nil {
		return err
	}
	a.EmptyRuns, err = checkedProductV2(a.E, 6)
	if err != nil {
		return err
	}
	a.ResourceExcludedRuns, err = checkedProductV2(a.R, 6)
	if err != nil {
		return err
	}
	base := DenominatorV2{SchemaVersion: DenominatorSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Subject: state.subject, QualificationEnvelope: state.envelope, CorpusIdentity: s.CorpusIdentity, WorkflowIdentity: s.WorkflowIdentity, TerminalProgramCHead: s.TerminalProgramCHead, Constituents: state.constituents, Entries: []DenominatorEntryV2{}, Accounting: a, ClaimCeiling: fixedQualificationClaimCeiling(), Identity: fixedDigestWidthPlaceholderV2}
	n, err := CanonicalSizeDenominatorV2(base)
	if err != nil {
		return err
	}
	z := canonicalSizerV2{n: n}
	index := 0
	for id, raw := range state.mechanics {
		m := state.mechanicsHandles[id].dto
		c, ok := state.custody[m.CustodyIdentity]
		if !ok || c.ConstituentOrdinal < 0 || c.ConstituentOrdinal >= len(state.constituents) || m.Seed < 1 || m.Seed > 2 || m.ReplayOrdinal < 1 || m.ReplayOrdinal > 3 {
			return fmt.Errorf("v2 denominator admission context mismatch")
		}
		constituent := state.constituents[c.ConstituentOrdinal]
		if len(constituent.Runs) != 6 {
			return fmt.Errorf("v2 denominator admission incomplete run inventory")
		}
		run := constituent.Runs[int(m.Seed-1)*3+int(m.ReplayOrdinal-1)]
		entry := DenominatorEntryV2{SchemaVersion: DenominatorEntrySchemaVersionV2, ConstituentOrdinal: c.ConstituentOrdinal, Receipt: constituent.Receipt, Custody: c, RunID: m.RunID, Seed: m.Seed, ReplayOrdinal: m.ReplayOrdinal, CommunityOrdinal: m.CommunityOrdinal, CommunityArtifact: run.CommunityArtifact, Mechanics: multiSeedBindingV2(MechanicsSchemaVersionV2, m.Identity, raw), QualificationEnvelope: state.envelope, Identity: fixedDigestWidthPlaceholderV2}
		size, err := CanonicalSizeDenominatorEntryV2(entry)
		if err != nil {
			return err
		}
		if index > 0 {
			z.add(1)
		}
		z.add(size)
		if z.err != nil {
			return z.err
		}
		index++
	}
	return z.err
}

func admitLineageAppendV2(existingBytes uint64, existingCount int, record LineageRecordV2, a LineageAccountingV2, d, env ArtifactBindingDTO) (uint64, error) {
	n, err := CanonicalSizeLineageRecordV2(record)
	if err != nil {
		return 0, err
	}
	total, err := checkedSumV2(existingBytes, n)
	if err != nil {
		return 0, err
	}
	if existingCount > 0 {
		total, err = checkedSumV2(total, 1)
		if err != nil {
			return 0, err
		}
	}
	base := LineageV2{SchemaVersion: LineageSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Denominator: d, QualificationEnvelope: env, Records: []LineageRecordV2{}, Accounting: a, ClaimCeiling: fixedQualificationClaimCeiling(), Identity: fixedDigestWidthPlaceholderV2}
	size, err := CanonicalSizeLineageV2(base)
	if err != nil {
		return 0, err
	}
	z := canonicalSizerV2{n: size}
	z.add(total)
	if z.err != nil {
		return 0, z.err
	}
	return total, nil
}
func admitSelectedRecordCopyV2(record RepresentativeRecord) error {
	base := RecordV2{Outcome: record.Outcome, SelectedNode: record.SelectedNode, Distance: record.Distance, SCCMembers: record.SCCMembers, IncomingPredecessors: []PredecessorV2{}}
	size, err := CanonicalSizeRecordV2(base)
	if err != nil {
		return err
	}
	z := canonicalSizerV2{n: size}
	for i, p := range record.IncomingPredecessors {
		one := PredecessorV2{OccurrenceID: p.OccurrenceID, RelationID: p.RelationID, CallerID: p.CallerID, TargetID: p.TargetID, CallSite: RangeDTO{Start: PositionDTO{Line: p.CallSite.Start.Line, Character: p.CallSite.Start.Character}, End: PositionDTO{Line: p.CallSite.End.Line, Character: p.CallSite.End.Character}}}
		n, err := CanonicalSizePredecessorV2(one)
		if err != nil {
			return err
		}
		if i > 0 {
			z.add(1)
		}
		z.add(n)
		if z.err != nil {
			return z.err
		}
	}
	return z.err
}
