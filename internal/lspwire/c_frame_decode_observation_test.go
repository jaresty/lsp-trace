package lspwire

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

// This test characterizes notification order only. It is not a C frame-limit RED.
func TestCFrameDecodeEntryObservationIsOptionalAndPreDecode(t *testing.T) {
	valid := []byte(frame(`{"jsonrpc":"2.0","id":1,"result":null}`))
	invalidJSON := []byte(frame(`{`))
	for _, tc := range []struct {
		name    string
		wire    []byte
		wantErr error
		want    []string
	}{
		{"valid", valid, nil, []string{"entry", "decode"}},
		{"invalid-json", invalidJSON, ErrMalformedJSON, []string{"entry", "decode"}},
		{"invalid-header", []byte("Content-Length: nope\r\n\r\n{}"), ErrInvalidContentLength, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sequence []string
			_, gotErr := NewReaderObservedAtDecode(bytes.NewReader(tc.wire), DefaultLimits(), func(e Event) {
				if e.Stage == EventDecode {
					sequence = append(sequence, "decode")
				}
			}, func() { sequence = append(sequence, "entry") }, nil).Read()
			if tc.wantErr == nil && gotErr != nil || tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("OBSERVATION_SETUP: err=%v want=%v", gotErr, tc.wantErr)
			}
			if !reflect.DeepEqual(sequence, tc.want) {
				t.Fatalf("OBSERVATION_ENTRY_ORDER: got=%v want=%v", sequence, tc.want)
			}
			_, nilErr := NewReaderObservedAtDecode(bytes.NewReader(tc.wire), DefaultLimits(), nil, nil, nil).Read()
			if (nilErr == nil) != (gotErr == nil) || tc.wantErr != nil && !errors.Is(nilErr, tc.wantErr) {
				t.Fatalf("NIL_OBSERVER_OUTCOME: observed=%v nil=%v", gotErr, nilErr)
			}
		})
	}
}
