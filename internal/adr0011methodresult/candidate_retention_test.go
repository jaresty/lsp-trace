package adr0011methodresult

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func retentionFixture(t *testing.T) ([]byte, CandidateExpectation) {
	t.Helper()
	params := []byte(`{"textDocument":{"uri":"file:///fixture.go"},"position":{"line":2,"character":3}}`)
	result := []byte(`null`)
	c := CanonicalCandidate{Version: CanonicalCandidateVersion, ClaimCeiling: "UNADMITTED;NO_PRODUCER_AUTHENTICATION",
		SessionID: "session-1", Generation: 1, KeyID: 2, Method: "textDocument/definition", QueryURI: "file:///fixture.go",
		QueryLine: 2, QueryCharacter: 3, PositionEncoding: "utf-16", ProviderName: "fixture",
		MaxMessages: 2, MaxBytes: 1024, DeadlineUnixNano: 123456789, MaxCandidates: 4, Null: true,
		ParamsBase64: base64.StdEncoding.EncodeToString(params), ParamsSHA256: rawSHA(params),
		ResultBase64: base64.StdEncoding.EncodeToString(result), ResultSHA256: rawSHA(result)}
	pre, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	c.ID = queryResultCandidateDigest(pre)
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if _, err := VerifyCanonicalCandidate(raw); err != nil {
		t.Fatalf("fixture replay: %v", err)
	}
	return raw, CandidateExpectation{ID: c.ID, SessionID: c.SessionID, Method: c.Method, QueryURI: c.QueryURI,
		ParamsSHA256: c.ParamsSHA256, Generation: c.Generation, KeyID: c.KeyID, QueryLine: c.QueryLine, QueryCharacter: c.QueryCharacter}
}

func TestPrivateCandidateReviewLifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	raw, expected := retentionFixture(t)
	if _, err := PublishPrivateCandidate(root, "candidate.json", append([]byte(nil), raw[:len(raw)-1]...)); !errors.Is(err, ErrInvalidCanonicalCandidate) {
		t.Fatalf("ASSERT_REJECT_NONCANONICAL: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "candidate.json")); !os.IsNotExist(err) {
		t.Fatalf("bad candidate published: %v", err)
	}
	receipt, err := PublishPrivateCandidate(root, "candidate.json", raw)
	if err != nil || receipt == nil {
		t.Fatalf("ASSERT_IMMUTABLE_PUBLICATION: receipt=%+v err=%v", receipt, err)
	}
	if _, err := PublishPrivateCandidate(root, "candidate.json", raw); !errors.Is(err, os.ErrExist) {
		t.Fatalf("ASSERT_NO_REPLACE: %v", err)
	}
	if got, err := ReadPrivateCandidate(root, "candidate.json", expected); err != nil || got.ID != expected.ID {
		t.Fatalf("ASSERT_REPLAY_AND_BIND: got=%+v err=%v", got, err)
	}
	for name, mutate := range map[string]func(*CandidateExpectation){
		"id":         func(e *CandidateExpectation) { e.ID = "sha256:wrong" },
		"session":    func(e *CandidateExpectation) { e.SessionID = "other" },
		"generation": func(e *CandidateExpectation) { e.Generation++ },
		"key":        func(e *CandidateExpectation) { e.KeyID++ },
		"method":     func(e *CandidateExpectation) { e.Method = "textDocument/references" },
		"uri":        func(e *CandidateExpectation) { e.QueryURI = "file:///other.go" },
		"line":       func(e *CandidateExpectation) { e.QueryLine++ },
		"character":  func(e *CandidateExpectation) { e.QueryCharacter++ },
		"params":     func(e *CandidateExpectation) { e.ParamsSHA256 = "sha256:wrong" },
	} {
		t.Run(name, func(t *testing.T) {
			want := expected
			mutate(&want)
			if _, err := ReadPrivateCandidate(root, "candidate.json", want); !errors.Is(err, ErrInvalidCanonicalCandidate) {
				t.Fatalf("ASSERT_REJECT_SUBSTITUTION: %v", err)
			}
		})
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)-2] ^= 1
	if err := os.WriteFile(filepath.Join(dir, "candidate.json"), changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateCandidate(root, "candidate.json", expected); !errors.Is(err, ErrInvalidCanonicalCandidate) {
		t.Fatalf("ASSERT_REJECT_MUTATED_PAYLOAD: %v", err)
	}
	if removal, err := root.RemovePrivateFile("candidate.json"); err != nil || !removal.Removed || !removal.DirectorySynced {
		t.Fatalf("ASSERT_EXPLICIT_REVIEW_REMOVAL: %+v %v", removal, err)
	}
	if _, err := ReadPrivateCandidate(root, "candidate.json", expected); err == nil {
		t.Fatal("ASSERT_REMOVED_NOT_REPLAYABLE")
	}
}
