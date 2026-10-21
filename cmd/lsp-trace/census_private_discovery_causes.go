package main

import (
	"encoding/json"
	"errors"
	"sort"

	"lsp-trace/internal/censusacquisition"
)

const privateCensusDiscoveryCausesSchemaVersion = "lsp-trace.private-census-discovery-causes.v1"

type privateCensusDiscoveryCause struct {
	Scope        string `json:"scope"`
	Stage        string `json:"stage"`
	Code         string `json:"code"`
	Count        uint64 `json:"count"`
	FirstOrdinal uint64 `json:"first_ordinal"`
}

type privateCensusDiscoveryCauses struct {
	SchemaVersion string                        `json:"schema_version"`
	Authority     int                           `json:"authority"`
	Causes        []privateCensusDiscoveryCause `json:"causes"`
}

type privateCensusDiscoveryCauseKey struct {
	scope string
	stage string
	code  string
}

func validPrivateCensusDiscoveryCause(scope, stage, code string) bool {
	switch scope {
	case "FILE":
		return stage == "DISCOVERY" && (code == "CANCELED" || code == "FILE_LIMIT" || code == "CHILD_INCOMPLETE") ||
			stage == "ENUMERATION" && code == "CANCELED" ||
			stage == "DOCUMENT_SUPPLY" && code == "SESSION_UNAVAILABLE" ||
			stage == "DOCUMENT_SYMBOL_FLATTEN" && (code == "NODE_LIMIT" || code == "DEPTH_LIMIT" || code == "STRING_BYTE_LIMIT")
	case "SYMBOL":
		return stage == "DISCOVERY" && (code == "CANCELED" || code == "SYMBOL_LIMIT" || code == "COORDINATE_OVERFLOW") ||
			stage == "PREPARE_CALL_HIERARCHY" && code == "RESULT_MISMATCH"
	default:
		return false
	}
}

func newPrivateCensusDiscoveryCausesForDiscovery(discovery censusacquisition.Discovery) (privateCensusDiscoveryCauses, error) {
	for _, cause := range discovery.IncompleteCauses {
		switch {
		case cause.Scope == "FILE" && cause.FileOrdinal != nil && cause.SymbolOrdinal == nil:
			if *cause.FileOrdinal < 0 || *cause.FileOrdinal >= discovery.Accounting.FileDenominator {
				return privateCensusDiscoveryCauses{}, errors.New("private discovery cause ordinal out of range")
			}
		case cause.Scope == "SYMBOL" && cause.SymbolOrdinal != nil && cause.FileOrdinal == nil:
			if *cause.SymbolOrdinal < 0 || *cause.SymbolOrdinal >= discovery.Accounting.SymbolDenominator {
				return privateCensusDiscoveryCauses{}, errors.New("private discovery cause ordinal out of range")
			}
		default:
			return privateCensusDiscoveryCauses{}, errors.New("invalid private discovery cause")
		}
	}
	return newPrivateCensusDiscoveryCauses(discovery.IncompleteCauses)
}

func newPrivateCensusDiscoveryCauses(in []censusacquisition.IncompleteCause) (privateCensusDiscoveryCauses, error) {
	out := privateCensusDiscoveryCauses{SchemaVersion: privateCensusDiscoveryCausesSchemaVersion, Authority: 0}
	byKey := make(map[privateCensusDiscoveryCauseKey]int)
	for _, cause := range in {
		var ordinal int
		switch {
		case cause.Scope == "FILE" && cause.FileOrdinal != nil && cause.SymbolOrdinal == nil:
			ordinal = *cause.FileOrdinal
		case cause.Scope == "SYMBOL" && cause.SymbolOrdinal != nil && cause.FileOrdinal == nil:
			ordinal = *cause.SymbolOrdinal
		default:
			return privateCensusDiscoveryCauses{}, errors.New("invalid private discovery cause")
		}
		if ordinal < 0 || !validPrivateCensusDiscoveryCause(cause.Scope, cause.Stage, cause.Code) {
			return privateCensusDiscoveryCauses{}, errors.New("invalid private discovery cause")
		}
		key := privateCensusDiscoveryCauseKey{scope: cause.Scope, stage: cause.Stage, code: cause.Code}
		if index, ok := byKey[key]; ok {
			out.Causes[index].Count++
			if uint64(ordinal) < out.Causes[index].FirstOrdinal {
				out.Causes[index].FirstOrdinal = uint64(ordinal)
			}
			continue
		}
		byKey[key] = len(out.Causes)
		out.Causes = append(out.Causes, privateCensusDiscoveryCause{Scope: cause.Scope, Stage: cause.Stage, Code: cause.Code, Count: 1, FirstOrdinal: uint64(ordinal)})
	}
	sort.Slice(out.Causes, func(i, j int) bool {
		a, b := out.Causes[i], out.Causes[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Stage != b.Stage {
			return a.Stage < b.Stage
		}
		return a.Code < b.Code
	})
	if len(out.Causes) == 0 {
		return privateCensusDiscoveryCauses{}, errors.New("private discovery causes required")
	}
	return out, nil
}

func marshalPrivateCensusDiscoveryCauses(value privateCensusDiscoveryCauses) ([]byte, error) {
	if value.SchemaVersion != privateCensusDiscoveryCausesSchemaVersion || value.Authority != 0 || len(value.Causes) == 0 {
		return nil, errors.New("invalid private discovery causes")
	}
	for i, cause := range value.Causes {
		if cause.Count == 0 || !validPrivateCensusDiscoveryCause(cause.Scope, cause.Stage, cause.Code) {
			return nil, errors.New("invalid private discovery causes")
		}
		if i > 0 {
			previous := value.Causes[i-1]
			if previous.Scope > cause.Scope || previous.Scope == cause.Scope && (previous.Stage > cause.Stage || previous.Stage == cause.Stage && previous.Code >= cause.Code) {
				return nil, errors.New("noncanonical private discovery causes")
			}
		}
	}
	return json.Marshal(value)
}
