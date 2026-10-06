package lspwire

import (
	"bytes"
	"errors"
	"testing"
)

func TestSuccessorNonGovernedServerRequestCompatibility(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":null,"method":"workspace/x"}`)
	r, err := NewSuccessorIngressReader(bytes.NewReader(p2Frame(body)), testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	message, err := r.ReadFrame()
	if err != nil || message.Kind() != KindRequest || r.LastReadObservation().RequestIDEnd != 0 {
		t.Fatalf("ASSERT_C15_SERVER_REQUEST_NON_GOVERNED_COMPATIBILITY kind=%d err=%v observation=%+v", message.Kind(), err, r.LastReadObservation())
	}
}

func TestPrivateC15ServerRequestIDSpanAndMalformedForms(t *testing.T) {
	valid := []struct {
		body string
		id   string
	}{
		{`{"jsonrpc":"2.0","id":"a\\nb","method":"workspace/x"}`, `"a\\nb"`},
		{` { "jsonrpc" : "2.0", "id" : -17, "method" : "workspace/x" } `, `-17`},
		{`{"jsonrpc":"2.0","id":0,"method":"workspace/x"}`, `0`},
	}
	for _, tc := range valid {
		o := testSuccessorOptions()
		o.GovernServerRequests = true
		r, err := NewSuccessorIngressReader(bytes.NewReader(p2Frame([]byte(tc.body))), o)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.ReadFrame(); err != nil {
			t.Fatalf("ASSERT_C15_SERVER_REQUEST_VALID_ID body=%s err=%v", tc.body, err)
		}
		observation := r.LastReadObservation()
		body := []byte(tc.body)
		if got := string(body[observation.RequestIDStart:observation.RequestIDEnd]); got != tc.id {
			t.Fatalf("ASSERT_C15_SERVER_REQUEST_EXACT_ID got=%q want=%q", got, tc.id)
		}
	}

	invalid := []string{
		`{"jsonrpc":"2.0","id":null,"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":1.5,"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":1e2,"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":true,"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":{},"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":[],"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":1,"id":2,"method":"workspace/x"}`,
		`{"jsonrpc":"2.0","id":1,"method":"workspace/x","method":"workspace/y"}`,
		`{"jsonrpc":"2.0","id":1,"method":"workspace/x","params":{},"params":{}}`,
	}
	for _, body := range invalid {
		o := testSuccessorOptions()
		o.GovernServerRequests = true
		r, err := NewSuccessorIngressReader(bytes.NewReader(p2Frame([]byte(body))), o)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.ReadFrame(); !errors.Is(err, ErrMalformedJSON) && !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("ASSERT_C15_SERVER_REQUEST_INVALID body=%s err=%v", body, err)
		}
		if r.LastReadObservation() != (SuccessorReadObservation{}) {
			t.Fatalf("ASSERT_C15_SERVER_REQUEST_INVALID_NO_OBSERVATION body=%s got=%+v", body, r.LastReadObservation())
		}
	}
}
