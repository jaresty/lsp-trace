package sessionruntime

import (
	"context"
	"testing"
)

func TestSessionLanguageIDRequiresExactGenerationAndNonEmptyStoredIdentity(t *testing.T) {
	configured := identityManager(t, 1, &identityReader{data: [][]byte{tokenBytes(1)}}, referenceChild{})
	started := configured.Start(context.Background(), StartRequest{Profile: identityProfile(t, t.TempDir()), LanguageID: "go"})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if got, ok := configured.SessionLanguageID(started.SessionID, started.Generation); !ok || got != "go" {
		t.Fatalf("ASSERT_SESSION_LANGUAGE_ID_EXACT_GENERATION: got=%q ok=%t", got, ok)
	}
	for _, tc := range []struct {
		name       string
		sessionID  string
		generation uint64
	}{
		{name: "unknown", sessionID: "missing", generation: started.Generation},
		{name: "stale", sessionID: started.SessionID, generation: started.Generation + 1},
	} {
		if got, ok := configured.SessionLanguageID(tc.sessionID, tc.generation); ok || got != "" {
			t.Fatalf("ASSERT_SESSION_LANGUAGE_ID_FAILS_CLOSED_%s: got=%q ok=%t", tc.name, got, ok)
		}
	}

	empty := identityManager(t, 1, &identityReader{data: [][]byte{tokenBytes(2)}}, referenceChild{})
	emptyStarted := empty.Start(context.Background(), StartRequest{Profile: identityProfile(t, t.TempDir())})
	if emptyStarted.Failure != "" {
		t.Fatal(emptyStarted)
	}
	if got, ok := empty.SessionLanguageID(emptyStarted.SessionID, emptyStarted.Generation); ok || got != "" {
		t.Fatalf("ASSERT_SESSION_LANGUAGE_ID_EMPTY_FAILS_CLOSED: got=%q ok=%t", got, ok)
	}
}
