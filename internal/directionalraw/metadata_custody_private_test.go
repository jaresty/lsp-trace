package directionalraw

import (
	"bytes"
	"testing"
)

func TestDirectionalMetadataCustodyPrivateInvalidatesInternalViews(t *testing.T) {
	reservation := NewReservation("metadata-custody")
	allocations, releases := 0, 0
	if !reservation.AttachPrivate(func(_, _ string, n int) ([]byte, error) {
		allocations++
		return make([]byte, n), nil
	}, func(Observation) {}, func() bool { return reservation.Terminate() }, func() { releases++ }) {
		t.Fatal("ASSERT_METADATA_CUSTODY_ATTACH")
	}
	lease := reservation.Begin("textDocument/prepareCallHierarchy", 7, 9)
	request, err := lease.Allocate("request-frame", 3)
	if err != nil {
		t.Fatal(err)
	}
	copy(request, "req")
	if !lease.Frame("request-frame", len(request)) {
		t.Fatal("ASSERT_METADATA_REQUEST_FRAME")
	}
	response, err := lease.Allocate("response-frame", len(`[{"x":1}]`))
	if err != nil {
		t.Fatal(err)
	}
	copy(response, `[{"x":1}]`)
	if !lease.Frame("response-frame", len(response)) || !lease.Result(response) {
		t.Fatal("ASSERT_METADATA_RESPONSE_ALIAS")
	}
	if err := lease.BindElementsPrivate(); err != nil {
		t.Fatal(err)
	}
	lease.Complete("")
	selection, ok := Select(reservation.Context(t.Context()), Request{Method: "textDocument/prepareCallHierarchy", Ordinal: 0})
	if !ok || !selection.View().Equal([]byte(`{"x":1}`)) {
		t.Fatal("ASSERT_METADATA_SELECTION_LIVE")
	}
	metadata, ok := lease.CompletedMetadataPrivate()
	if !ok || metadata.RequestLengthPrivate != 3 || metadata.ResponseLengthPrivate != uint64(len(response)) || metadata.ResultLengthPrivate != uint64(len(response)) {
		t.Fatalf("ASSERT_METADATA_AUTHENTIC: %+v", metadata)
	}
	if !reservation.Release() || allocations != 2 || releases != 1 {
		t.Fatalf("ASSERT_METADATA_RELEASE_ONCE: allocations=%d releases=%d", allocations, releases)
	}
	if selection.View().Len() != 0 || selection.View().Equal([]byte(`{"x":1}`)) {
		t.Fatal("ASSERT_METADATA_VIEW_INVALID_AFTER_RELEASE")
	}
	if got, ok := selection.AppendToPrivate(nil); ok || !bytes.Equal(got, nil) {
		t.Fatal("ASSERT_METADATA_TOKEN_INVALID_AFTER_RELEASE")
	}
	if _, ok := lease.CompletedMetadataPrivate(); ok {
		t.Fatal("ASSERT_METADATA_COMPLETION_INVALID_AFTER_RELEASE")
	}
}
