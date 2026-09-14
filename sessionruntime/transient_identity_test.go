package sessionruntime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sync"
	"testing"

	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

type identityReader struct {
	mu   sync.Mutex
	data [][]byte
	err  error
}

func (r *identityReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.data) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	b := r.data[0]
	r.data = r.data[1:]
	return copy(p, b), nil
}

func identityProfile(t *testing.T, workspace string) runtimeprofile.Profile {
	t.Helper()
	v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	return runtimeprofile.Resolve(v)
}

func identityManager(t *testing.T, max int, reader io.Reader, children ...Child) *Manager {
	t.Helper()
	m, err := New(Config{
		Limits:  Limits{MaxSessions: max, MaxRequests: 1, MaxChildren: max + 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 64, MaxOperations: max + 1},
		Starter: &sequenceStarter{children: children}, transientIdentityRandom: reader,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func tokenBytes(v byte) []byte { return bytes.Repeat([]byte{v}, 16) }

func TestTransientIdentityFormatUniqueExactLookupAndSelectorClosure(t *testing.T) {
	r := &identityReader{data: [][]byte{tokenBytes(1), tokenBytes(2)}}
	m := identityManager(t, 2, r, referenceChild{}, referenceChild{})
	a := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/a"), BootstrapAlias: "alias-a"})
	b := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/b")})
	ta, fa := m.TransientSessionIdentity(a.SessionID, a.Generation)
	tb, fb := m.TransientSessionIdentity(b.SessionID, b.Generation)
	if fa != "" || fb != "" || ta == tb || !regexp.MustCompile(`^ts_[0-9a-f]{32}$`).MatchString(ta) || !regexp.MustCompile(`^ts_[0-9a-f]{32}$`).MatchString(tb) {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_FORMAT_UNIQUE: a=%q/%q b=%q/%q", ta, fa, tb, fb)
	}
	for _, id := range []string{"alias-a", ta, "missing"} {
		if token, failure := m.TransientSessionIdentity(id, 1); token != "" || failure != session.SessionNotFound {
			t.Fatalf("ASSERT_TRANSIENT_IDENTITY_INTERNAL_SELECTOR_ONLY: id=%q token=%q failure=%q", id, token, failure)
		}
	}
	if token, failure := m.TransientSessionIdentity(a.SessionID, 2); token != "" || failure != session.StaleGeneration {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_STALE_CLOSED: token=%q failure=%q", token, failure)
	}
}

func TestTransientIdentityStableAcrossRestartAndUnavailableAfterRemoval(t *testing.T) {
	m := identityManager(t, 1, &identityReader{data: [][]byte{tokenBytes(3)}}, referenceChild{}, referenceChild{})
	started := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/restart")})
	before, _ := m.TransientSessionIdentity(started.SessionID, 1)
	restart := m.Restart(context.Background(), started.SessionID, "restart-caller")
	waitOperation(t, m, restart.IntentID, OperationComplete)
	after, failure := m.TransientSessionIdentity(started.SessionID, 2)
	if failure != "" || before != after {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_RESTART_STABLE: before=%q after=%q failure=%q", before, after, failure)
	}
	if token, stale := m.TransientSessionIdentity(started.SessionID, 1); token != "" || stale != session.StaleGeneration {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_RESTART_STALE: token=%q failure=%q", token, stale)
	}
	stop := m.Stop(context.Background(), started.SessionID, "stop-caller")
	waitOperation(t, m, stop.IntentID, OperationComplete)
	if token, removed := m.TransientSessionIdentity(started.SessionID, 2); token != "" || removed != session.SessionNotFound {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_REMOVAL_CLOSED: token=%q failure=%q", token, removed)
	}
}

func TestTransientIdentityEntropyFailureAndCollisionExhaustionAreAtomic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reader io.Reader
		want   session.Failure
	}{
		{"entropy", &identityReader{err: errors.New("entropy unavailable")}, session.Failure("TRANSIENT_IDENTITY_UNAVAILABLE")},
		{"collision-exhaustion", &identityReader{data: func() [][]byte {
			out := make([][]byte, transientIdentityAttempts+1)
			for i := range out {
				out[i] = tokenBytes(7)
			}
			return out
		}()}, session.ResourceExhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := tc.reader
			m := identityManager(t, 2, reader, referenceChild{}, referenceChild{})
			if tc.name == "collision-exhaustion" {
				first := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/first")})
				stop := m.Stop(context.Background(), first.SessionID, "stop")
				waitOperation(t, m, stop.IntentID, OperationComplete)
			}
			before := m.Census()
			got := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/candidate")})
			if got.Failure != tc.want || m.Census() != before || len(m.Records()) != before.Sessions {
				t.Fatalf("ASSERT_TRANSIENT_IDENTITY_FAILURE_ATOMIC: result=%+v before=%+v after=%+v", got, before, m.Census())
			}
		})
	}
}

func TestTransientIdentityCollisionRetriesAndRetainsTombstone(t *testing.T) {
	r := &identityReader{data: [][]byte{tokenBytes(4), tokenBytes(4), tokenBytes(5)}}
	m := identityManager(t, 1, r, referenceChild{}, referenceChild{})
	first := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/one")})
	old, _ := m.TransientSessionIdentity(first.SessionID, 1)
	stop := m.Stop(context.Background(), first.SessionID, "stop")
	waitOperation(t, m, stop.IntentID, OperationComplete)
	second := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/two")})
	fresh, failure := m.TransientSessionIdentity(second.SessionID, 1)
	if failure != "" || fresh == old {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_TOMBSTONE_COLLISION_RETRY: old=%q fresh=%q failure=%q", old, fresh, failure)
	}
}

func TestTransientIdentityConcurrentStartsNoDuplicates(t *testing.T) {
	const n = 32
	data := make([][]byte, n)
	children := make([]Child, n)
	for i := range data {
		data[i] = append(make([]byte, 15), byte(i+1))
		children[i] = referenceChild{}
	}
	m := identityManager(t, n, &identityReader{data: data}, children...)
	var wg sync.WaitGroup
	tokens := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/concurrent/"+string(rune('a'+i)))})
			token, failure := m.TransientSessionIdentity(s.SessionID, s.Generation)
			if failure != "" {
				t.Errorf("start %d: %+v lookup=%q", i, s, failure)
				return
			}
			tokens <- token
		}(i)
	}
	wg.Wait()
	close(tokens)
	seen := map[string]bool{}
	for token := range tokens {
		if seen[token] {
			t.Fatalf("ASSERT_TRANSIENT_IDENTITY_CONCURRENT_UNIQUE: duplicate=%q", token)
		}
		seen[token] = true
	}
	if len(seen) != n {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_CONCURRENT_COUNT: got=%d want=%d", len(seen), n)
	}
}

func TestTransientIdentityAbsentFromUnrelatedRuntimeSurfaces(t *testing.T) {
	m := identityManager(t, 1, &identityReader{data: [][]byte{tokenBytes(9)}}, referenceChild{})
	started := m.Start(context.Background(), StartRequest{Profile: identityProfile(t, "/non-leak"), BootstrapAlias: "alias", LanguageID: "go"})
	token, failure := m.TransientSessionIdentity(started.SessionID, started.Generation)
	if failure != "" {
		t.Fatal(failure)
	}
	for name, value := range map[string]any{"start": started, "records": m.Records(), "observations": m.Observations(), "census": m.Census()} {
		encoded, err := json.Marshal(value)
		if err != nil || bytes.Contains(encoded, []byte(token)) {
			t.Fatalf("ASSERT_TRANSIENT_IDENTITY_NOT_PROJECTED_%s: json=%s err=%v", name, encoded, err)
		}
	}
}

func TestTransientIdentityProductionConstructorPath(t *testing.T) {
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 1}, Starter: &sequenceStarter{children: []Child{referenceChild{}}}})
	if err != nil || m.transientIdentityRandom != rand.Reader {
		t.Fatalf("ASSERT_TRANSIENT_IDENTITY_SHARED_CONSTRUCTOR_DEFAULT: manager=%v err=%v", m, err)
	}
}
