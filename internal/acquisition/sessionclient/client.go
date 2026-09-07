// Package sessionclient binds the neutral acquisition coordinator to an existing
// exact managed runtime. It imports neither incomingops nor sliceops.
package sessionclient

import (
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/acquisition"
	"lsp-trace/sessionruntime"
)

type Runtime interface {
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
}
type documentRuntime interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

// New uses the coordinator's declared exact generation and request context on
// each call. Runtime, not the coordinator, validates that generation's authority.
func New(runtime Runtime) acquisition.Client {
	client := acquisition.NewWireClient(func(ctx context.Context, r acquisition.WireRequest) (json.RawMessage, error) {
		deadline, _ := ctx.Deadline()
		result := runtime.RoundTrip(ctx, sessionruntime.RoundTripRequest{SessionID: r.Context.SessionID, Generation: r.Context.Generation, Method: r.Method, Params: r.Params, Deadline: deadline, MaxMessages: r.MaxMessages, MaxBytes: int64(r.MaxBytes)})
		if result.Failure != "" {
			return nil, fmt.Errorf("%s", result.Failure)
		}
		if result.ServerError != nil {
			return nil, fmt.Errorf("json-rpc error %d: %s", result.ServerError.Code, result.ServerError.Message)
		}
		return result.Result, nil
	})
	supplier, ok := runtime.(documentRuntime)
	if !ok {
		return client
	}
	return acquisition.WithDocumentSupply(client, func(ctx context.Context, a acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
		document := supplier.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: a.SessionID, Generation: a.Generation, URI: l.URI, LanguageID: l.LanguageID, CaptureSupply: true})
		if document.Failure != "" {
			return acquisition.Supply{}, fmt.Errorf("%s", document.Failure)
		}
		var observation json.RawMessage
		if document.Supply != nil {
			var err error
			observation, err = json.Marshal(document.Supply)
			if err != nil {
				return acquisition.Supply{}, err
			}
		}
		return acquisition.Supply{URI: document.URI, LanguageID: document.LanguageID, Observation: observation}, nil
	})
}
