package adr0011methodtransport

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// Ordered fields match the pre-documentSymbol TransactionObservation definition.
// A mirror built from the historical field names tests the actual JSON bytes,
// including field order and zero-valued fields, without hard-coding time/digests.
func TestHistoricalDRObservationJSONBytes(t *testing.T) {
	names := []string{
		"DeclaredSessionID", "DeclaredGeneration", "DeclaredMethod", "DeclaredQueryURI",
		"DeclaredLine", "DeclaredCharacter", "DeclaredIncludeDeclaration", "IncludeDeclarationPresent",
		"DeclaredDeadline", "DeclaredMaxMessages", "DeclaredMaxBytes", "ParamsSHA256", "ParamsBytes",
		"MetadataObserved", "ReportedMethodAdvertised", "ReportedPositionEncoding",
		"ReportedProviderName", "ReportedProviderVersion", "RoundTripCalled",
		"LocalWriteCorrespondence", "ReportedKey", "ReportedRequestMessages", "ReportedRequestBytes",
		"ReportedResponseMessages", "ReportedResponseBytes", "RawResultDisposition", "RawResultBytes", "RawResultSHA256",
	}
	typ := reflect.TypeOf(TransactionObservation{})
	fields := make([]reflect.StructField, 0, len(names))
	for _, name := range names {
		field, ok := typ.FieldByName(name)
		if !ok {
			t.Fatalf("ASSERT_DR_OBSERVATION_JSON_BYTES: historical field %s missing", name)
		}
		fields = append(fields, reflect.StructField{Name: name, Type: field.Type})
	}
	historical := reflect.StructOf(fields)
	for _, method := range []string{MethodDefinition, MethodReferences} {
		t.Run(method, func(t *testing.T) {
			req := request(method)
			req.Deadline = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			obs, err := newTransactionObservation(req)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(obs)
			if err != nil {
				t.Fatal(err)
			}
			mirror := reflect.New(historical).Elem()
			value := reflect.ValueOf(obs)
			for i, name := range names {
				mirror.Field(i).Set(value.FieldByName(name))
			}
			want, err := json.Marshal(mirror.Interface())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) || obs.DeclaredLine != 2 || obs.DeclaredCharacter != 3 || obs.DeclaredQueryURI != "file:///w/a.go" || obs.IncludeDeclarationPresent != (method == MethodReferences) {
				t.Fatalf("ASSERT_DR_OBSERVATION_JSON_BYTES: method=%s got=%s want=%s line=%d character=%d", method, got, want, obs.DeclaredLine, obs.DeclaredCharacter)
			}
			t.Log("ASSERT_DR_OBSERVATION_JSON_BYTES: PASS exact historical ordered field serialization")
		})
	}
}
