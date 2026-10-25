package main

import (
	"context"
	"encoding/json"
	"io"

	"lsp-trace/internal/programctransitionhost"
)

// runPrivateProgramCTransition invokes exactly one private transition. It is
// deliberately absent from public flag parsing and command dispatch.
func runPrivateProgramCTransition(ctx context.Context, request programctransitionhost.Request, stdout io.Writer) error {
	if stdout == nil {
		return context.Canceled
	}
	result, err := programctransitionhost.InvokeOne(ctx, request)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}
