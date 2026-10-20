package censuschunking

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/censusacquisition"
)

type coordinatorStore struct {
	objects map[string][]byte
	limit   int64
	puts    int
	failAt  int
}

func newCoordinatorStore(limit int64) *coordinatorStore {
	return &coordinatorStore{objects: map[string][]byte{}, limit: limit}
}
func (s *coordinatorStore) MaxObjectBytes() int64 { return s.limit }
func (s *coordinatorStore) Put(_ context.Context, b []byte) (string, error) {
	s.puts++
	if s.failAt > 0 && s.puts == s.failAt {
		return "", errors.New("publication sentinel")
	}
	if int64(len(b)) > s.limit {
		return "", errors.New("store limit sentinel")
	}
	id := digestBytes(b)
	if old, ok := s.objects[id]; ok && string(old) != string(b) {
		return "", errors.New("collision")
	}
	s.objects[id] = append([]byte(nil), b...)
	return id, nil
}
func (s *coordinatorStore) Get(_ context.Context, id string) ([]byte, error) {
	b, ok := s.objects[id]
	if !ok {
		return nil, errors.New("missing sentinel")
	}
	return append([]byte(nil), b...), nil
}

type validatorFunc func(context.Context, SessionBinding) error

func (f validatorFunc) Validate(ctx context.Context, b SessionBinding) error { return f(ctx, b) }

func coordinatorFixture(t *testing.T, n int) CoordinatorRequest {
	t.Helper()
	in := fixture(t, n)
	built, err := Build(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := make([]Projection, len(built.Chunks))
	s := make([]Sidecar, len(built.Chunks))
	for i := range built.Chunks {
		p[i] = built.Chunks[i].Projection
		s[i] = built.Chunks[i].Sidecar
	}
	return CoordinatorRequest{Input: in, Plan: built.Plan, Index: built.Index, Projections: p, Sidecars: s}
}

func TestCoordinatorThreeChunksResumeAndTerminal(t *testing.T) {
	r := coordinatorFixture(t, ChunkSize*2+1)
	store := newCoordinatorStore(MaxCanonicalBytes)
	validations, runs := 0, 0
	r.Store = store
	r.Validator = validatorFunc(func(_ context.Context, b SessionBinding) error {
		validations++
		if b.SessionID != r.Plan.SessionID || b.Generation != r.Plan.Generation || b.WorkspaceURI != r.Input.WorkspaceURI || !b.Ready {
			return errors.New("bad binding")
		}
		return nil
	})
	r.CoreFactory = func(int) censusacquisition.Core { runs++; return executionCore(t) }
	out, err := Coordinate(context.Background(), r)
	if err != nil {
		t.Fatalf("ASSERT_COORDINATOR_COMPLETE: %v", err)
	}
	if runs != 3 || validations != 3 || len(out.CommitSelectors) != 3 || out.TerminalSelector == "" {
		t.Fatalf("ASSERT_THREE_CHUNKS_10000_10000_REST runs=%d validations=%d out=%+v", runs, validations, out)
	}
	beforeRuns := runs
	r.ResumeHead = out.FinalCheckpoint
	out2, err := Coordinate(context.Background(), r)
	if err != nil || runs != beforeRuns || out2.TerminalSelector != out.TerminalSelector {
		t.Fatalf("ASSERT_RESUME_NO_DUPLICATE_CORE err=%v runs=%d", err, runs)
	}
	raw, err := store.Get(context.Background(), out.TerminalSelector)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := ParseTerminalIndex(raw)
	if err != nil || terminal.Authority != 0 || terminal.Accepted || terminal.SourceGraphComplete != "UNKNOWN" || terminal.CrossChunkCalls != "NOT_COMPUTED" || terminal.AggregateCapture != "NOT_MATERIALIZED" {
		t.Fatalf("ASSERT_TERMINAL_PRIVATE_AUTHORITY: %+v %v", terminal, err)
	}
}

func TestCoordinatorSessionAndCrashBoundaries(t *testing.T) {
	want := errors.New("session sentinel")
	r := coordinatorFixture(t, ChunkSize+1)
	store := newCoordinatorStore(MaxCanonicalBytes)
	runs := 0
	r.Store = store
	r.CoreFactory = func(int) censusacquisition.Core { runs++; return executionCore(t) }
	r.Validator = validatorFunc(func(context.Context, SessionBinding) error { return want })
	if _, err := Coordinate(context.Background(), r); !errors.Is(err, want) || !IsCoordinatorStage(err, CoordinatorStageSession) || runs != 0 {
		t.Fatalf("ASSERT_SESSION_BEFORE_CORE err=%v runs=%d", err, runs)
	}
	for _, failAt := range []int{1, 7, 10} {
		t.Run(string(rune(failAt)), func(t *testing.T) {
			rr := coordinatorFixture(t, ChunkSize+1)
			st := newCoordinatorStore(MaxCanonicalBytes)
			st.failAt = failAt
			rr.Store = st
			rr.Validator = readyValidator()
			rr.CoreFactory = func(int) censusacquisition.Core { return executionCore(t) }
			out, err := Coordinate(context.Background(), rr)
			if err == nil {
				t.Fatalf("ASSERT_CRASH_BOUNDARY_FAIL out=%+v", out)
			}
			if out.TerminalSelector != "" {
				t.Fatal("ASSERT_TERMINAL_WITHHELD_ON_PARTIAL")
			}
		})
	}
}

func TestCoordinatorRejectsCorruptResumeAndStoreBoundary(t *testing.T) {
	r := coordinatorFixture(t, ChunkSize+1)
	st := newCoordinatorStore(MaxCanonicalBytes)
	runs := 0
	r.Store = st
	r.Validator = readyValidator()
	r.CoreFactory = func(int) censusacquisition.Core { runs++; return executionCore(t) }
	out, err := Coordinate(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	st.objects[out.FinalCheckpoint] = []byte(`{}`)
	before := runs
	r.ResumeHead = out.FinalCheckpoint
	if _, err = Coordinate(context.Background(), r); err == nil || runs != before || (!IsCoordinatorStage(err, CoordinatorStageCheckpoint) && !IsCoordinatorStage(err, CoordinatorStageReadback)) {
		t.Fatalf("ASSERT_CORRUPT_PREFIX_BEFORE_CORE err=%v runs=%d", err, runs)
	}
	r = coordinatorFixture(t, ChunkSize+1)
	r.Store = newCoordinatorStore(1)
	r.Validator = readyValidator()
	r.CoreFactory = func(int) censusacquisition.Core { return executionCore(t) }
	if _, err = Coordinate(context.Background(), r); err == nil || !IsCoordinatorStage(err, CoordinatorStagePublication) {
		t.Fatalf("ASSERT_STORE_SIZE_BOUNDARY: %v", err)
	}
}

func readyValidator() SessionValidator {
	return validatorFunc(func(_ context.Context, b SessionBinding) error {
		if !b.Ready {
			return errors.New("not ready")
		}
		return nil
	})
}
func executionCore(t *testing.T) censusacquisition.Core {
	return censusacquisition.Core{Acquirer: chunkAcquirerFunc(func(_ context.Context, r censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		return censusacquisition.AcquiredV5{Session: r.Session, Raw: exactChunkV5(t, r)}, nil
	})}
}
