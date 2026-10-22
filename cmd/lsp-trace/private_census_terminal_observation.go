package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/sessionruntime"
)

const privateCensusTerminalObservationSchemaVersion = "lsp-trace.private-census-terminal-observation.v1"

const (
	terminalNotApplicable = "NOT_APPLICABLE"
	terminalUnknown       = "UNKNOWN"
)

type privateCensusTerminalObservation struct {
	SchemaVersion     string `json:"schema_version"`
	Authority         int    `json:"authority"`
	Scope             string `json:"scope"`
	Stage             string `json:"stage"`
	Cause             string `json:"cause"`
	LifecycleSubcause string `json:"lifecycle_subcause"`
	ContextState      string `json:"context_state"`
	EncodingCategory  string `json:"encoding_category"`
	FirstOrdinal      uint64 `json:"first_ordinal"`
	Count             uint64 `json:"count"`
	ContiguousSuffix  string `json:"contiguous_suffix"`
}

type terminalCauseVocabulary struct {
	scope, stage, cause string
	rank                int
}

// privateCensusTerminalVocabulary is the authoritative validator and rank table.
var privateCensusTerminalVocabulary = []terminalCauseVocabulary{
	{"FILE", "DOCUMENT_SUPPLY", "SESSION_POISONED", 0},
	{"FILE", "DOCUMENT_SUPPLY", "LIFECYCLE_CONFLICT", 1},
	{"FILE", "DOCUMENT_SUPPLY", "SESSION_UNAVAILABLE", 2},
	{"FILE", "DISCOVERY", "CANCELED", 3},
	{"FILE", "ENUMERATION", "CANCELED", 4},
	{"FILE", "DISCOVERY", "FILE_LIMIT", 5},
	{"FILE", "DISCOVERY", "CHILD_INCOMPLETE", 6},
	{"FILE", "DOCUMENT_SYMBOL_FLATTEN", "NODE_LIMIT", 7},
	{"FILE", "DOCUMENT_SYMBOL_FLATTEN", "DEPTH_LIMIT", 8},
	{"FILE", "DOCUMENT_SYMBOL_FLATTEN", "STRING_BYTE_LIMIT", 9},
	{"SYMBOL", "DISCOVERY", "CANCELED", 10},
	{"SYMBOL", "DISCOVERY", "SYMBOL_LIMIT", 11},
	{"SYMBOL", "DISCOVERY", "TARGET_LIMIT", 12},
	{"SYMBOL", "DISCOVERY", "COORDINATE_OVERFLOW", 13},
	{"SYMBOL", "PREPARE_CALL_HIERARCHY", "RESULT_MISMATCH", 14},
}

var lifecycleSubcauses = map[string]bool{
	"MANAGER_CLOSED": true, "SESSION_NOT_READY": true, "PROTOCOL_OWNED": true,
	"LEASE_CLOSING": true, "LEASE_TOKEN_MISMATCH": true, "LANGUAGE_ID_CONFLICT": true,
	terminalUnknown: true, terminalNotApplicable: true,
}
var contextStates = map[string]bool{"ACTIVE": true, "CANCELED": true, "DEADLINE_EXCEEDED": true}
var encodingCategories = map[string]bool{"BOUND_EXHAUSTION": true, "INVALID_INPUT": true, "OTHER_ENCODING_FAILURE": true, terminalNotApplicable: true}
var suffixStates = map[string]bool{"TRUE": true, "FALSE": true, terminalUnknown: true}

func validatePrivateCensusTerminalVocabularyClosure(produced []sessionruntime.LifecycleSubcause) error {
	for _, value := range produced {
		if !lifecycleSubcauses[string(value)] || value == sessionruntime.LifecycleSubcause(terminalUnknown) || value == sessionruntime.LifecycleSubcause(terminalNotApplicable) {
			return errors.New("producer lifecycle subcause outside terminal vocabulary")
		}
	}
	if len(produced)+2 != len(lifecycleSubcauses) {
		return errors.New("terminal lifecycle vocabulary has no producer closure")
	}
	return nil
}

func terminalCauseRank(c censusacquisition.IncompleteCause) (int, bool) {
	for _, value := range privateCensusTerminalVocabulary {
		if c.Scope == value.scope && c.Stage == value.stage && c.Code == value.cause {
			return value.rank, true
		}
	}
	return 0, false
}

func newPrivateCensusTerminalObservation(discovery censusacquisition.Discovery, receipt censusenumerationprojection.Receipt, contextErr error) (privateCensusTerminalObservation, error) {
	if len(discovery.IncompleteCauses) == 0 {
		return privateCensusTerminalObservation{}, errors.New("terminal cause required")
	}
	causes := append([]censusacquisition.IncompleteCause(nil), discovery.IncompleteCauses...)
	if err := validateTerminalCauseOrdinals(causes, discovery); err != nil {
		return privateCensusTerminalObservation{}, err
	}
	sort.SliceStable(causes, func(i, j int) bool {
		ri, oki := terminalCauseRank(causes[i])
		rj, okj := terminalCauseRank(causes[j])
		if oki != okj {
			return oki
		}
		if ri != rj {
			return ri < rj
		}
		return causeOrdinal(causes[i]) < causeOrdinal(causes[j])
	})
	primary := causes[0]
	if _, ok := terminalCauseRank(primary); !ok {
		return privateCensusTerminalObservation{}, errors.New("unknown terminal cause")
	}
	ordinals := make([]int, 0)
	for _, cause := range causes {
		if cause.Scope == primary.Scope && cause.Stage == primary.Stage && cause.Code == primary.Code && cause.LifecycleSubcause == primary.LifecycleSubcause {
			ordinals = append(ordinals, causeOrdinal(cause))
		}
	}
	sort.Ints(ordinals)
	denominator := discovery.Accounting.FileDenominator
	if primary.Scope == "SYMBOL" {
		denominator = discovery.Accounting.SymbolDenominator
	}
	suffix := terminalUnknown
	if denominator > 0 {
		suffix = "TRUE"
		for i, ordinal := range ordinals {
			if ordinal != denominator-len(ordinals)+i {
				suffix = "FALSE"
				break
			}
		}
	}
	contextState := "ACTIVE"
	if errors.Is(contextErr, context.DeadlineExceeded) {
		contextState = "DEADLINE_EXCEEDED"
	} else if errors.Is(contextErr, context.Canceled) {
		contextState = "CANCELED"
	}
	encoding := terminalNotApplicable
	if receipt.EncodingFailureCategory != "" {
		encoding = string(receipt.EncodingFailureCategory)
	}
	subcause := terminalNotApplicable
	if primary.Code == "LIFECYCLE_CONFLICT" {
		subcause = string(primary.LifecycleSubcause)
		if subcause == "" {
			subcause = terminalUnknown
		}
	}
	out := privateCensusTerminalObservation{privateCensusTerminalObservationSchemaVersion, 0, primary.Scope, primary.Stage, primary.Code, subcause, contextState, encoding, uint64(ordinals[0]), uint64(len(ordinals)), suffix}
	if err := validatePrivateCensusTerminalObservation(out); err != nil {
		return privateCensusTerminalObservation{}, err
	}
	return out, nil
}

func causeOrdinal(c censusacquisition.IncompleteCause) int {
	if c.Scope == "FILE" {
		return *c.FileOrdinal
	}
	return *c.SymbolOrdinal
}

func validateTerminalCauseOrdinals(causes []censusacquisition.IncompleteCause, discovery censusacquisition.Discovery) error {
	seenFile := make(map[int]struct{}, len(causes))
	seenSymbol := make(map[int]struct{}, len(causes))
	for _, cause := range causes {
		if _, ok := terminalCauseRank(cause); !ok {
			return errors.New("unknown terminal cause")
		}
		var ordinal, denominator int
		var seen map[int]struct{}
		switch {
		case cause.Scope == "FILE" && cause.FileOrdinal != nil && cause.SymbolOrdinal == nil:
			ordinal, denominator, seen = *cause.FileOrdinal, discovery.Accounting.FileDenominator, seenFile
		case cause.Scope == "SYMBOL" && cause.SymbolOrdinal != nil && cause.FileOrdinal == nil:
			ordinal, denominator, seen = *cause.SymbolOrdinal, discovery.Accounting.SymbolDenominator, seenSymbol
		default:
			return errors.New("invalid terminal cause ordinal kind")
		}
		if denominator <= 0 || ordinal < 0 || ordinal >= denominator {
			return errors.New("invalid terminal cause ordinal bounds")
		}
		if _, duplicate := seen[ordinal]; duplicate {
			return errors.New("duplicate terminal cause ordinal")
		}
		seen[ordinal] = struct{}{}
	}
	return nil
}

func validatePrivateCensusTerminalObservation(v privateCensusTerminalObservation) error {
	if v.SchemaVersion != privateCensusTerminalObservationSchemaVersion || v.Authority != 0 || v.Count == 0 || !lifecycleSubcauses[v.LifecycleSubcause] || !contextStates[v.ContextState] || !encodingCategories[v.EncodingCategory] || !suffixStates[v.ContiguousSuffix] {
		return errors.New("invalid private terminal observation")
	}
	if _, ok := terminalCauseRank(censusacquisition.IncompleteCause{Scope: v.Scope, Stage: v.Stage, Code: v.Cause}); !ok {
		return errors.New("invalid private terminal cause")
	}
	if (v.Cause == "LIFECYCLE_CONFLICT") != (v.LifecycleSubcause != terminalNotApplicable) {
		return errors.New("invalid lifecycle subcause combination")
	}
	if v.Cause != "LIFECYCLE_CONFLICT" && v.LifecycleSubcause != terminalNotApplicable {
		return errors.New("invalid lifecycle subcause combination")
	}
	return nil
}

func marshalPrivateCensusTerminalObservation(v privateCensusTerminalObservation) ([]byte, error) {
	if err := validatePrivateCensusTerminalObservation(v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
