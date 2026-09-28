package main

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/adr0011acquisition"
)

func TestADR0011HostConstructionDisabled(t *testing.T) {
	host := newHostSelectorRuntime(nil, nil)
	if host.adr0011Owner == nil {
		t.Fatal("ASSERT_ADR0011_HOST_DEFAULT_OFF: missing private owner")
	}
	receipt, err := host.adr0011Owner.Acquire(context.Background(), adr0011acquisition.Query{})
	if !errors.Is(err, adr0011acquisition.ErrDisabled) || receipt != nil {
		t.Fatalf("ASSERT_ADR0011_HOST_DEFAULT_OFF: receipt=%+v err=%v", receipt, err)
	}
}
