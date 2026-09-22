package censuscontinuation

import (
	"encoding/json"
	"errors"

	"lsp-trace/internal/describeworker"
)

const workerRecordsV2Schema = "lsp-trace.describe-records.v2"

type workerRecordsV2Wire struct {
	SchemaVersion string            `json:"schema_version"`
	Responses     []json.RawMessage `json:"responses"`
}

type workerHistoryV2Wire struct {
	SchemaVersion string            `json:"schema_version"`
	Invocations   []json.RawMessage `json:"invocations"`
	Responses     []json.RawMessage `json:"responses"`
}

// encodeWorkerRecordsV2 retains each validated V2 record as opaque canonical bytes.
func encodeWorkerRecordsV2(records []describeworker.ResponseRecordV2) ([]byte, error) {
	wire := workerRecordsV2Wire{SchemaVersion: workerRecordsV2Schema, Responses: []json.RawMessage{}}
	seen := map[string]bool{}
	for _, record := range records {
		raw, err := record.Bytes()
		if err != nil || seen[record.ID()] {
			return nil, errors.New("invalid response v2 history")
		}
		seen[record.ID()] = true
		wire.Responses = append(wire.Responses, append(json.RawMessage(nil), raw...))
	}
	return json.Marshal(wire)
}

func decodeWorkerRecordsV2(raw []byte) ([]describeworker.ResponseRecordV2, error) {
	var wire workerRecordsV2Wire
	if err := strictCanonical(raw, &wire); err != nil || wire.SchemaVersion != workerRecordsV2Schema {
		return nil, errors.New("invalid response v2 history")
	}
	out := make([]describeworker.ResponseRecordV2, 0, len(wire.Responses))
	seen := map[string]bool{}
	for _, item := range wire.Responses {
		record, err := describeworker.ParseResponseRecordV2(item)
		if err != nil || seen[record.ID()] {
			return nil, errors.New("invalid response v2 history")
		}
		seen[record.ID()] = true
		out = append(out, record)
	}
	return out, nil
}

func encodeWorkerHistoryV2(invocations []describeworker.InvocationRecordV2, responses []describeworker.ResponseRecordV2) ([]byte, error) {
	wire := workerHistoryV2Wire{SchemaVersion: workerRecordsV2Schema, Invocations: []json.RawMessage{}, Responses: []json.RawMessage{}}
	for _, invocation := range invocations {
		raw, err := invocation.Bytes()
		if err != nil {
			return nil, errors.New("invalid invocation v2 history")
		}
		wire.Invocations = append(wire.Invocations, raw)
	}
	for _, response := range responses {
		raw, err := response.Bytes()
		if err != nil {
			return nil, errors.New("invalid response v2 history")
		}
		wire.Responses = append(wire.Responses, raw)
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if _, _, err = decodeWorkerHistoryV2(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func decodeWorkerHistoryV2(raw []byte) ([]describeworker.InvocationRecordV2, []describeworker.ResponseRecordV2, error) {
	var wire workerHistoryV2Wire
	if err := strictCanonical(raw, &wire); err != nil || wire.SchemaVersion != workerRecordsV2Schema {
		return nil, nil, errors.New("invalid worker v2 history")
	}
	invocations := make([]describeworker.InvocationRecordV2, 0, len(wire.Invocations))
	responses := make([]describeworker.ResponseRecordV2, 0, len(wire.Responses))
	responseByID := map[string]describeworker.ResponseRecordV2{}
	for _, item := range wire.Responses {
		record, err := describeworker.ParseResponseRecordV2(item)
		if err != nil || responseByID[record.ID()].ID() != "" {
			return nil, nil, errors.New("invalid worker v2 history")
		}
		responseByID[record.ID()] = record
		responses = append(responses, record)
	}
	seenInvocations, usedResponses := map[string]bool{}, map[string]bool{}
	for _, item := range wire.Invocations {
		record, err := describeworker.ParseInvocationRecordV2(item)
		if err != nil || seenInvocations[record.ID()] {
			return nil, nil, errors.New("invalid worker v2 history")
		}
		seenInvocations[record.ID()] = true
		if response := record.Response(); response.ID() != "" {
			stored, ok := responseByID[response.ID()]
			if !ok {
				return nil, nil, errors.New("invalid worker v2 history")
			}
			left, _ := response.Bytes()
			right, _ := stored.Bytes()
			if string(left) != string(right) || usedResponses[response.ID()] {
				return nil, nil, errors.New("invalid worker v2 history")
			}
			usedResponses[response.ID()] = true
		}
		invocations = append(invocations, record)
	}
	if len(usedResponses) != len(responseByID) {
		return nil, nil, errors.New("invalid worker v2 history")
	}
	return invocations, responses, nil
}
