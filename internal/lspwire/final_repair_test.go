package lspwire

import "testing"

func TestPendingAcceptHistoricalPublicDisposition(t *testing.T) {
	t.Run("empty history unknown", func(t *testing.T) {
		p := NewPending(2)
		if got := p.Accept(ResponseKey{Generation: 99, ID: 1}); got != ResponseUnknown {
			t.Fatalf("ASSERT_LSPWIRE_PENDING_EMPTY_HISTORY_UNKNOWN: %v", got)
		}
	})

	t.Run("known generation unknown ID", func(t *testing.T) {
		p := NewPending(2)
		p.Begin(7)
		if got := p.Accept(ResponseKey{Generation: 7, ID: 999}); got != ResponseUnknown {
			t.Fatalf("ASSERT_LSPWIRE_PENDING_KNOWN_GENERATION_UNKNOWN_ID: %v", got)
		}
	})

	t.Run("never seen generation wrong generation", func(t *testing.T) {
		p := NewPending(2)
		p.Begin(7)
		if got := p.Accept(ResponseKey{Generation: 8, ID: 999}); got != ResponseWrongGeneration {
			t.Fatalf("ASSERT_LSPWIRE_PENDING_NEVER_SEEN_WRONG_GENERATION: %v", got)
		}
	})
}

func TestPendingObserverDetailDoesNotChangePublicDisposition(t *testing.T) {
	var events []Event
	p := NewPendingObserved(2, func(event Event) {
		if event.Stage == EventPendingResponse {
			events = append(events, event)
		}
	})
	p.Begin(7)

	cases := []struct {
		key        ResponseKey
		wantPublic ResponseDisposition
		wantDetail pendingResponseDetail
	}{
		{ResponseKey{Generation: 7, ID: 998}, ResponseUnknown, pendingResponseUnknownKnownGeneration},
		{ResponseKey{Generation: 8, ID: 999}, ResponseWrongGeneration, pendingResponseUnknownUnretainedGeneration},
	}
	for _, tc := range cases {
		if got := p.Accept(tc.key); got != tc.wantPublic {
			t.Fatalf("ASSERT_LSPWIRE_PENDING_OBSERVER_PUBLIC_DISPOSITION: got=%v want=%v", got, tc.wantPublic)
		}
	}
	if len(events) != len(cases) {
		t.Fatalf("ASSERT_LSPWIRE_PENDING_OBSERVER_EVENT_COUNT: %d", len(events))
	}
	for i, tc := range cases {
		if events[i].Disposition != tc.wantPublic || events[i].pendingDetail != tc.wantDetail {
			t.Fatalf("ASSERT_LSPWIRE_PENDING_PRIVATE_OBSERVER_DETAIL[%d]: %+v", i, events[i])
		}
	}
}
