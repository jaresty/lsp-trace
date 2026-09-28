package adr0011acquisition

import (
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func TestPrivateResponseReadPrecommitAndUnverified(t *testing.T) {
	root, x := responseReadFixture(t, "null")
	blocked := func(r *publication.Root, s string, n int64) ([]byte, error) {
		if s == x.OwnerRead.selector {
			return nil, errors.New("missing")
		}
		return publication.ReadVerifiedBoundFile(r, s, n)
	}
	if state, e := publishResponseRead(root, x, blocked); e == nil || state.stage != "ABSENT" {
		t.Fatalf("ASSERT_RESPONSE_READ_PRECOMMIT: %+v %v", state, e)
	}
	changed := func(r *publication.Root, s string, n int64) ([]byte, error) {
		v, e := publication.ReadVerifiedBoundFile(r, s, n)
		if e == nil && strings.Contains(s, "-response-read-") {
			v = append([]byte(nil), v...)
			v[0] = '!'
		}
		return v, e
	}
	state, e := publishResponseRead(root, x, changed)
	if e == nil || state.stage != "COMMITTED_UNVERIFIED" || state.selector == "" {
		t.Fatalf("ASSERT_RESPONSE_READ_UNVERIFIED: %+v %v", state, e)
	}
	if replayResponseRead(root, state, x, nil) {
		t.Fatal("ASSERT_RESPONSE_READ_UNVERIFIED_REPLAY")
	}
}
