package discoveryfailure

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type opaque66 struct{}

func (opaque66) Error() string { panic("must not format raw error") }

type typed66 struct{ c Cause }

func (typed66) Error() string                { panic("must not format raw error") }
func (e typed66) PrepareFailureCause() Cause { return e.c }

type wrapped66 struct{ inner error }

func (wrapped66) Error() string   { return "closed wrapper" }
func (e wrapped66) Unwrap() error { return e.inner }

func TestCause66Closed(t *testing.T) {
	for _, c := range []Cause{Unknown, Canceled, Deadline, RPCServer, SessionUnavailable, SessionCrashed, EnclosureBudget, InvalidResponse} {
		raw, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		var got Cause
		if json.Unmarshal(raw, &got) != nil || got != c {
			t.Fatal("CLOSED_CAUSE_ROUNDTRIP")
		}
		if FromError(typed66{c}) != c {
			t.Fatal("TYPED_CAUSE")
		}
	}
	for _, raw := range []string{`"secret://source"`, `"FUTURE"`, `1`, `null`, `""`} {
		var c Cause
		if json.Unmarshal([]byte(raw), &c) == nil {
			t.Fatal("REJECT_UNKNOWN_CAUSE", raw)
		}
	}
	if _, e := json.Marshal(Cause("FUTURE")); e == nil {
		t.Fatal("REJECT_INVALID_MARSHAL")
	}
	if FromError(opaque66{}) != Unknown || FromError(typed66{"FUTURE"}) != Unknown || FromError(nil) != Unknown {
		t.Fatal("UNKNOWN_NOT_EMPTY")
	}
	probe := error(typed66{RPCServer})
	if FromError(wrapped66{probe}) != RPCServer {
		t.Fatal("WRAPPED_CLOSED_CAUSE")
	}
	if n := testing.AllocsPerRun(100, func() {
		if FromError(probe) != RPCServer {
			panic("closed cause")
		}
	}); n != 0 {
		t.Fatal("NO_CLASSIFIER_LOOKUP_ALLOCATION", n)
	}
	for _, tc := range []struct {
		err  error
		want Cause
	}{{context.Canceled, Canceled}, {context.DeadlineExceeded, Deadline}} {
		if FromError(fmt.Errorf("closed wrapper: %w", tc.err)) != tc.want {
			t.Fatal("CONTEXT_IS")
		}
	}
}
