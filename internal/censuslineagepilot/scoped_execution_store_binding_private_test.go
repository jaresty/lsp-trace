//go:build linux || darwin

package censuslineagepilot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func TestScopedExecutionStorePrivateBindingRefusals(t *testing.T) {
	f := executionClaimFixture(t)
	o := f.owner(t)
	proof := executionTopologyProofPrivate(t, f)
	for _, kind := range []string{"alias", "roles", "nil", "closed", "selector"} {
		t.Run(kind, func(t *testing.T) {
			roots := f.roots
			owner := f.owner(t)
			switch kind {
			case "alias":
				roots[1] = roots[0]
			case "roles":
				roots[0], roots[1] = roots[1], roots[0]
			case "nil":
				roots[0] = nil
			case "closed":
				other := executionClaimFixture(t)
				roots[0] = other.roots[0]
				if err := roots[0].Close(); err != nil {
					t.Fatal(err)
				}
			case "selector":
				owner.selector = "feature-scope-execution-head-private-v1-" + strings.Repeat("f", 64) + ".json"
				roots = [3]*publication.ContentRetentionRoot{}
			}
			store, err := newScopedExecutionStorePrivate(context.Background(), owner, roots, proof)
			if store != nil || err == nil {
				t.Fatal("substitution admitted")
			}
			if kind == "selector" && !errors.Is(err, ErrInvalid) {
				t.Fatalf("selector not bounded before root IO: %v", err)
			}
		})
	}
	s := realExecutionStorePrivate(t, f, o)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Load(ctx, o.selector); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.CompareAbsent(ctx, o.selector, o.raw); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(context.Background(), o.selector); err == nil {
		t.Fatal("closed store")
	}
}
