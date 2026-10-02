// Package adr0011cobserve carries optional, scalar-only private test probes.
// It neither admits evidence nor authenticates or publishes a candidate.
package adr0011cobserve

import "context"

type Stage uint8

const (
	DecodeEntry Stage = iota + 1
	RetainEntry
	B4EvaluationEntry
	PrivateCandidatePublishEntry
)

type Observer func(Stage)
type contextKey struct{}

// With is an internal test-only carrier. Nil leaves the context unchanged.
// The observer cannot return a decision or modify the reader's frame.
func With(ctx context.Context, observer Observer) context.Context {
	if observer == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, observer)
}

func From(ctx context.Context) Observer {
	if ctx == nil {
		return nil
	}
	observer, _ := ctx.Value(contextKey{}).(Observer)
	return observer
}

// Notify isolates panicking test observers from the operation outcome.
func Notify(observer Observer, stage Stage) {
	if observer == nil {
		return
	}
	defer func() { _ = recover() }()
	observer(stage)
}
