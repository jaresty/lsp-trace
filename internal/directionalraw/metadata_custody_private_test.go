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
	if _, _, ok := lease.CompletedFramesPrivate(7, 9, 1024, 1); ok {
		t.Fatal("ASSERT_FRAME_INCOMPLETE_REFUSED")
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
	for _, q := range []struct {
		id, gen       uint64
		bytes, frames int
	}{{8, 9, 1024, 1}, {7, 10, 1024, 1}, {7, 9, len(request) + len(response) - 1, 1}, {7, 9, 1024, 0}} {
		if _, _, ok := lease.CompletedFramesPrivate(q.id, q.gen, q.bytes, q.frames); ok {
			t.Fatal("ASSERT_FRAME_FOREIGN_OR_BOUND_REFUSED")
		}
	}
	requestCopy, responseCopies, ok := lease.CompletedFramesPrivate(7, 9, len(request)+len(response), 1)
	if !ok || !bytes.Equal(requestCopy, request) || len(responseCopies) != 1 || !bytes.Equal(responseCopies[0], response) {
		t.Fatal("ASSERT_FRAME_EXACT_BOUND")
	}
	requestCopy[0] = 'X'
	if request[0] != 'r' {
		t.Fatal("ASSERT_FRAME_NO_MUTABLE_ALIAS")
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
	if _, _, ok := lease.CompletedFramesPrivate(7, 9, 1024, 1); ok {
		t.Fatal("ASSERT_FRAME_CLOSED_REFUSED")
	}
	if !bytes.Equal(responseCopies[0], []byte(`[{"x":1}]`)) {
		t.Fatal("ASSERT_FRAME_COPY_SURVIVES_TERMINATION")
	}
	t.Log("ASSERT_FRAME_EXTRACTION: PASS; exact bound, incomplete/foreign/closed refusal, caller copies independent")
	if _, ok := lease.CompletedMetadataPrivate(); ok {
		t.Fatal("ASSERT_METADATA_COMPLETION_INVALID_AFTER_RELEASE")
	}
}
