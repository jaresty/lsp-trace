package transientstructural

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type countingRuntime struct {
	manager *sessionruntime.Manager
	mu      sync.Mutex
	account Accounting
}

func (r *countingRuntime) Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return r.manager.Metadata(id, generation)
}

func (r *countingRuntime) Records() []sessionruntime.Record { return r.manager.Records() }

func (r *countingRuntime) RoundTrip(ctx context.Context, request sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.mu.Lock()
	r.account.Requests.Attempted++
	frontier := request.Method == "callHierarchy/incomingCalls" || request.Method == "callHierarchy/outgoingCalls"
	preparation := request.Method == "textDocument/prepareCallHierarchy"
	if frontier {
		r.account.Frontier.Observed++
	}
	if preparation {
		r.account.Preparation.Attempted++
	}
	r.mu.Unlock()
	result := r.manager.RoundTrip(ctx, request)
	r.mu.Lock()
	defer r.mu.Unlock()
	if result.Failure == "" && result.ServerError == nil {
		r.account.Requests.Succeeded++
		if frontier {
			r.account.Frontier.Expanded++
		}
		if preparation {
			var items []json.RawMessage
			raw := bytes.TrimSpace(result.Result)
			switch {
			case bytes.Equal(raw, []byte("null")) || bytes.Equal(raw, []byte("[]")):
				r.account.Preparation.Empty++
			case json.Unmarshal(raw, &items) != nil:
				r.account.Preparation.Failed++
			case len(items) == 0:
				r.account.Preparation.Empty++
			default:
				r.account.Preparation.Returned++
			}
		}
	} else if result.Failure == session.RequestCancelled {
		r.account.Requests.Cancelled++
		if preparation {
			r.account.Preparation.Failed++
		}
		if frontier {
			r.account.Frontier.Unexpanded++
			addOmission(&r.account, OmissionCancellation, 1)
		}
	} else {
		r.account.Requests.Failed++
		if preparation {
			r.account.Preparation.Failed++
		}
		if frontier {
			r.account.Frontier.Unexpanded++
			addOmission(&r.account, omissionForFailure(result.Failure), 1)
		}
	}
	return result
}

func (r *countingRuntime) snapshot() Accounting {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.account
	out.Omissions = append([]OmissionCount(nil), out.Omissions...)
	return out
}

func omissionForFailure(failure session.Failure) OmissionReason {
	switch failure {
	case session.RequestCancelled:
		return OmissionCancellation
	case session.RequestTimeout, session.InitializationTimeout:
		return OmissionTimeout
	case session.StaleGeneration:
		return OmissionCancellation
	case session.ResourceExhausted:
		return OmissionRequestBound
	default:
		return OmissionInvalidResponse
	}
}

func terminalForContext(ctx context.Context) TerminalState {
	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		return StateTimeout
	}
	return StateCancelled
}

func terminalForSessionFailure(failure session.Failure) TerminalState {
	switch failure {
	case session.StaleGeneration:
		return StateGenerationChanged
	case session.RequestTimeout, session.InitializationTimeout:
		return StateTimeout
	case session.RequestCancelled, session.LifecycleConflict:
		return StateCancelled
	case session.ResourceExhausted:
		return StateResourceLimit
	case session.SessionNotFound:
		return StateInvalidServerResponse
	default:
		return StateInvalidServerResponse
	}
}

func reconciles(accounting Accounting) bool {
	return accounting.Requests.Attempted == accounting.Requests.Succeeded+accounting.Requests.Failed+accounting.Requests.Cancelled &&
		accounting.Preparation.Attempted == accounting.Preparation.Returned+accounting.Preparation.Empty+accounting.Preparation.Failed &&
		accounting.Nodes.Observed == accounting.Nodes.Admitted+accounting.Nodes.Rejected+accounting.Nodes.Omitted &&
		accounting.Occurrences.Observed == accounting.Occurrences.Admitted+accounting.Occurrences.Rejected+accounting.Occurrences.Omitted &&
		accounting.Frontier.Observed == accounting.Frontier.Expanded+accounting.Frontier.Unexpanded
}

func hasNonDuplicateOmission(accounting Accounting) bool {
	for _, omission := range accounting.Omissions {
		if omission.Reason != OmissionDuplicate && omission.Count > 0 {
			return true
		}
	}
	return false
}

func accountUnadmitted(accounting Accounting, nodes []graph.Node, edges []graph.Edge, reason OmissionReason) Accounting {
	accounting.Nodes.Observed = len(nodes)
	for _, node := range nodes {
		if node.ID == "" || graph.ValidateItem(node.Item) != nil {
			accounting.Nodes.Rejected++
		} else {
			accounting.Nodes.Omitted++
		}
	}
	for _, edge := range edges {
		count := len(edge.CallSites)
		if count == 0 {
			count = 1
		}
		accounting.Occurrences.Observed += count
		if edge.CallerNodeID == "" || edge.CalleeNodeID == "" {
			accounting.Occurrences.Rejected += count
		} else {
			accounting.Occurrences.Omitted += count
		}
	}
	omitted := accounting.Nodes.Omitted + accounting.Occurrences.Omitted
	addOmission(&accounting, reason, omitted)
	return accounting
}
