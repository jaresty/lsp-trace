package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/programc"
	"sort"
)

type VerifiedDenominatorV2 struct {
	dto           DenominatorV2
	binding       ArtifactBindingDTO
	canonical     []byte
	qualification *checkedQualificationCalculationStateV2
}
type VerifiedLineageV2 struct {
	dto           LineageV2
	binding       ArtifactBindingDTO
	canonical     []byte
	denominator   ArtifactBindingDTO
	qualification *checkedQualificationCalculationStateV2
	artifacts     map[ArtifactBindingDTO][]byte
}

func (d VerifiedDenominatorV2) BindingV2() (ArtifactBindingDTO, error) {
	if d.qualification == nil {
		return ArtifactBindingDTO{}, fmt.Errorf("unverified v2 denominator")
	}
	return d.binding, nil
}
func (d VerifiedDenominatorV2) CanonicalBytesV2() ([]byte, error) {
	if d.qualification == nil {
		return nil, fmt.Errorf("unverified v2 denominator")
	}
	if _, err := CanonicalSizeDenominatorV2(d.dto); err != nil {
		return nil, err
	}
	return append([]byte{}, d.canonical...), nil
}
func (d VerifiedDenominatorV2) AccountingV2() (DenominatorAccountingV2, error) {
	if d.qualification == nil {
		return DenominatorAccountingV2{}, fmt.Errorf("unverified v2 denominator")
	}
	return d.dto.Accounting, nil
}
func (l VerifiedLineageV2) BindingV2() (ArtifactBindingDTO, error) {
	if l.qualification == nil {
		return ArtifactBindingDTO{}, fmt.Errorf("unverified v2 lineage")
	}
	return l.binding, nil
}
func (l VerifiedLineageV2) CanonicalBytesV2() ([]byte, error) {
	if l.qualification == nil {
		return nil, fmt.Errorf("unverified v2 lineage")
	}
	if _, err := CanonicalSizeLineageV2(l.dto); err != nil {
		return nil, err
	}
	return append([]byte{}, l.canonical...), nil
}
func (l VerifiedLineageV2) ArtifactsV2() (map[ArtifactBindingDTO][]byte, error) {
	if l.qualification == nil {
		return nil, fmt.Errorf("unverified v2 lineage")
	}
	out := map[ArtifactBindingDTO][]byte{}
	for b, raw := range l.artifacts {
		if err := VerifyRawArtifactBindingV2(b, raw); err != nil {
			return nil, err
		}
		out[b] = append([]byte{}, raw...)
	}
	return out, nil
}

func BuildDenominatorArtifactV2(q CheckedQualificationCalculationV2) (VerifiedDenominatorV2, error) {
	zero := VerifiedDenominatorV2{}
	if q.state == nil {
		return zero, fmt.Errorf("v2 denominator requires verified qualification")
	}
	state := q.state
	s := state.subjectDTO
	if len(state.constituents) != len(s.ReceiptInventory) || len(state.constituents) == 0 {
		return zero, fmt.Errorf("v2 complete constituent inventory mismatch")
	}
	accounting := DenominatorAccountingV2{P: uint64(len(state.constituents))}
	runCounts := map[string]uint64{}
	for i, c := range state.constituents {
		receipt := s.ReceiptInventory[i]
		if c.ConstituentOrdinal != i || c.Receipt.SchemaVersion != receipt.ReceiptSchemaVersion || c.Receipt.Identity != "" || c.Receipt.SHA256 != receipt.ReceiptSHA256 || c.Receipt.ByteLength != receipt.ReceiptByteLength || len(c.Runs) != 6 || c.PublicationSelector == "" || c.ConstituentSelector == "" || !validDigest(c.NativeV5SHA256) || c.NativeV5ByteLength == 0 || !validDigest(c.InstabilitySHA256) || c.InstabilityByteLength == 0 {
			return zero, fmt.Errorf("v2 constituent binding mismatch")
		}
		switch c.Classification {
		case "COMPLETE":
			if c.ProgramCOutcome != "COMPLETE" || c.Disposition != "" || c.InstabilitySchemaVersion != programc.InstabilityVersionV2 {
				return zero, fmt.Errorf("v2 COMPLETE classification mismatch")
			}
			accounting.C++
		case "EMPTY":
			if c.ProgramCOutcome != "EMPTY" || c.Disposition != "" {
				return zero, fmt.Errorf("v2 EMPTY classification mismatch")
			}
			accounting.E++
		case "COMMITTED_INCOMPLETE_MATCHING_RESOURCE":
			if c.ProgramCOutcome != "COMPLETE" || c.Disposition != "COMMITTED_INCOMPLETE_MATCHING_RESOURCE" || c.InstabilitySchemaVersion != programc.InstabilityVersionV3 {
				return zero, fmt.Errorf("v2 excluded classification mismatch")
			}
			accounting.R++
		default:
			return zero, fmt.Errorf("v2 unresolved constituent")
		}
		for j, run := range c.Runs {
			seed, replay := uint64(j/3+1), uint8(j%3+1)
			if run.Seed != seed || run.ReplayOrdinal != replay || run.RunID != fmt.Sprintf("seed-%020d-replay-%d", seed, replay) {
				return zero, fmt.Errorf("v2 complete run identity mismatch")
			}
			if c.Classification == "EMPTY" {
				if run.Outcome != "EMPTY" || run.CommunityCount != 0 || run.CommunityArtifact != (ArtifactBindingDTO{}) {
					return zero, fmt.Errorf("v2 EMPTY run must have exact zero community binding")
				}
			} else {
				b := run.CommunityArtifact
				if run.Outcome != "COMPLETE" || run.CommunityCount == 0 || run.CommunityCount > 10000 || b.SchemaVersion != "lsp-trace.community.v1" || b.Identity != "" || VerifyRawArtifactBindingV2(b, state.artifacts[b]) != nil {
					return zero, fmt.Errorf("v2 COMPLETE run community artifact mismatch")
				}
			}
			if c.Classification == "COMPLETE" {
				runCounts[fmt.Sprintf("%d/%s", i, run.RunID)] = run.CommunityCount
			}
		}
	}
	if err := admitDenominatorBeforeOutputCopiesV2(state, accounting); err != nil {
		return zero, err
	}
	type item struct {
		dto MechanicsIdentityDTOv2
		raw []byte
		key []byte
	}
	ordered := make([]item, 0, len(state.mechanics))
	for id, raw := range state.mechanics {
		handle, ok := state.mechanicsHandles[id]
		if !ok {
			return zero, fmt.Errorf("v2 mechanics handle missing")
		}
		m := handle.dto
		key, _ := json.Marshal(m.CommunityMembers)
		ordered = append(ordered, item{m, raw, key})
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		ca, cb := state.custody[a.dto.CustodyIdentity], state.custody[b.dto.CustodyIdentity]
		if ca.ConstituentOrdinal != cb.ConstituentOrdinal {
			return ca.ConstituentOrdinal < cb.ConstituentOrdinal
		}
		if a.dto.RunID != b.dto.RunID {
			return a.dto.RunID < b.dto.RunID
		}
		if cmp := bytes.Compare(a.key, b.key); cmp != 0 {
			return cmp < 0
		}
		return a.dto.CommunityOrdinal < b.dto.CommunityOrdinal
	})
	entries := make([]DenominatorEntryV2, 0, len(ordered))
	seen := map[string]bool{}
	actualRuns := map[string]uint64{}
	for _, item := range ordered {
		m := item.dto
		custody, ok := state.custody[m.CustodyIdentity]
		if !ok || custody.ConstituentOrdinal < 0 || custody.ConstituentOrdinal >= len(state.constituents) {
			return zero, fmt.Errorf("v2 entry custody missing")
		}
		c := state.constituents[custody.ConstituentOrdinal]
		if m.Seed < 1 || m.Seed > 2 || m.ReplayOrdinal < 1 || m.ReplayOrdinal > 3 {
			return zero, fmt.Errorf("v2 entry run conversion refused")
		}
		key := fmt.Sprintf("%d/%s/%d", custody.ConstituentOrdinal, m.RunID, m.CommunityOrdinal)
		if seen[key] || c.Classification != "COMPLETE" || m.CommunityOrdinal < 0 || uint64(m.CommunityOrdinal) >= c.Runs[int(m.Seed-1)*3+int(m.ReplayOrdinal-1)].CommunityCount {
			return zero, fmt.Errorf("v2 duplicate/foreign entry context")
		}
		seen[key] = true
		run := c.Runs[int(m.Seed-1)*3+int(m.ReplayOrdinal-1)]
		if m.CommunityArtifactSHA256 != run.CommunityArtifact.SHA256 || custody.PublicationSelector != c.PublicationSelector || custody.ConstituentSelector != c.ConstituentSelector || custody.NativeV5SHA256 != c.NativeV5SHA256 || custody.NativeV5ByteLength != c.NativeV5ByteLength || custody.ReceiptSelector != c.Receipt.SHA256 || m.SelectionContext.ConstituentSelector != c.ConstituentSelector {
			return zero, fmt.Errorf("v2 entry source/community splice")
		}
		entry := DenominatorEntryV2{SchemaVersion: DenominatorEntrySchemaVersionV2, ConstituentOrdinal: c.ConstituentOrdinal, Receipt: c.Receipt, Custody: custody, RunID: m.RunID, Seed: m.Seed, ReplayOrdinal: m.ReplayOrdinal, CommunityOrdinal: m.CommunityOrdinal, CommunityArtifact: run.CommunityArtifact, Mechanics: multiSeedBindingV2(MechanicsSchemaVersionV2, m.Identity, item.raw), QualificationEnvelope: state.envelope}
		entry, _, _, err := EncodeDenominatorEntryV2(entry)
		if err != nil {
			return zero, err
		}
		entries = append(entries, entry)
		runKey := fmt.Sprintf("%d/%s", c.ConstituentOrdinal, m.RunID)
		actualRuns[runKey]++
		accounting.G, err = checkedSumV2(accounting.G, uint64(len(m.SelectionContext.SeedGroups)))
		if err != nil {
			return zero, err
		}
	}
	if len(actualRuns) != len(runCounts) {
		return zero, fmt.Errorf("v2 omitted run entries")
	}
	for run, count := range runCounts {
		if actualRuns[run] != count {
			return zero, fmt.Errorf("v2 incomplete eligible community enumeration")
		}
	}
	var err error
	accounting.Evaluated, err = checkedSumV2(accounting.C, accounting.E)
	if err != nil {
		return zero, err
	}
	accounting.Excluded = accounting.R
	accounting.TerminalRuns, err = checkedProductV2(accounting.P, 6)
	if err != nil {
		return zero, err
	}
	accounting.EligibleRuns, err = checkedProductV2(accounting.C, 6)
	if err != nil {
		return zero, err
	}
	accounting.EmptyRuns, err = checkedProductV2(accounting.E, 6)
	if err != nil {
		return zero, err
	}
	accounting.ResourceExcludedRuns, err = checkedProductV2(accounting.R, 6)
	if err != nil {
		return zero, err
	}
	accounting.D = uint64(len(entries))
	dto := DenominatorV2{SchemaVersion: DenominatorSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Subject: state.subject, QualificationEnvelope: state.envelope, CorpusIdentity: s.CorpusIdentity, WorkflowIdentity: s.WorkflowIdentity, TerminalProgramCHead: s.TerminalProgramCHead, Constituents: state.constituents, Entries: entries, Accounting: accounting, ClaimCeiling: fixedQualificationClaimCeiling()}
	dto, b, raw, err := EncodeDenominatorV2(dto)
	if err != nil {
		return zero, err
	}
	copy, err := ParseDenominatorV2(raw)
	if err != nil {
		return zero, err
	}
	return VerifiedDenominatorV2{dto: copy, binding: b, canonical: raw, qualification: state}, nil
}

func VerifyDenominatorArtifactV2(raw []byte, q CheckedQualificationCalculationV2) (VerifiedDenominatorV2, error) {
	expected, err := BuildDenominatorArtifactV2(q)
	if err != nil {
		return VerifiedDenominatorV2{}, err
	}
	if !bytes.Equal(raw, expected.canonical) {
		return VerifiedDenominatorV2{}, fmt.Errorf("v2 denominator does not equal complete reconstructed inventory")
	}
	return expected, nil
}

func BuildLineageArtifactV2(d VerifiedDenominatorV2, q CheckedQualificationCalculationV2) (VerifiedLineageV2, error) {
	zero := VerifiedLineageV2{}
	if q.state == nil || d.qualification != q.state {
		return zero, fmt.Errorf("v2 lineage foreign qualification")
	}
	records := []LineageRecordV2{}
	recordBytes := uint64(0)
	artifacts := map[ArtifactBindingDTO][]byte{}
	a := LineageAccountingV2{D: d.dto.Accounting.D, G: d.dto.Accounting.G}
	for _, entry := range d.dto.Entries {
		m, ok := q.state.mechanicsHandles[entry.Mechanics.Identity]
		if !ok {
			return zero, fmt.Errorf("v2 lineage missing reconstructed mechanics")
		}
		result, err := FinalizeMultiSeedV2(m, q)
		if err != nil {
			return zero, err
		}
		raw, err := result.CanonicalBytes()
		if err != nil {
			return zero, err
		}
		dto, err := result.DTO()
		if err != nil {
			return zero, err
		}
		rb := multiSeedBindingV2(ResultSchemaVersionV2, result.Identity(), raw)
		artifacts[rb] = raw
		n, e := CanonicalSizeCompositionV2(dto.Composition)
		body, e := marshalSizedV2(dto.Composition, n, e)
		if e != nil {
			return zero, e
		}
		artifacts[dto.Grouping.Composition] = body
		n, e = CanonicalSizeGroupingV2(dto.Grouping)
		body, e = marshalSizedV2(dto.Grouping, n, e)
		if e != nil {
			return zero, e
		}
		artifacts[dto.Qualification.Grouping] = body
		record := LineageRecordV2{SchemaVersion: LineageRecordSchemaVersionV2, DenominatorEntryIdentity: entry.Identity, Mechanics: entry.Mechanics, Result: rb, QualificationEnvelope: q.state.envelope}
		record, _, _, err = EncodeLineageRecordV2(record)
		if err != nil {
			return zero, err
		}
		a.TerminalOuterResults++
		a.SelectedGroups, err = checkedSumV2(a.SelectedGroups, dto.GroupAccounting.Selected)
		if err != nil {
			return zero, err
		}
		a.NoCallsGroups, err = checkedSumV2(a.NoCallsGroups, dto.GroupAccounting.NoCalls)
		if err != nil {
			return zero, err
		}
		a.CallsIneligibleGroups, err = checkedSumV2(a.CallsIneligibleGroups, dto.GroupAccounting.CallsIneligible)
		if err != nil {
			return zero, err
		}
		recordBytes, err = admitLineageAppendV2(recordBytes, len(records), record, a, d.binding, q.state.envelope)
		if err != nil {
			return zero, err
		}
		records = append(records, record)
	}
	sum, err := checkedSumV2(a.SelectedGroups, a.NoCallsGroups, a.CallsIneligibleGroups)
	if err != nil || sum != a.G || a.TerminalOuterResults != a.D {
		return zero, fmt.Errorf("v2 lineage D/G conservation")
	}
	dto := LineageV2{SchemaVersion: LineageSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Denominator: d.binding, QualificationEnvelope: q.state.envelope, Records: records, Accounting: a, ClaimCeiling: fixedQualificationClaimCeiling()}
	dto, b, raw, err := EncodeLineageV2(dto)
	if err != nil {
		return zero, err
	}
	copy, err := ParseLineageV2(raw)
	if err != nil {
		return zero, err
	}
	return VerifiedLineageV2{dto: copy, binding: b, canonical: raw, denominator: d.binding, qualification: q.state, artifacts: artifacts}, nil
}
