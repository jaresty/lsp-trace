package inventorysuccessor

import (
	"context"

	"lsp-trace/internal/censussourcecontinuation"
)

// VerifyMultiSeedWorkflowIdentityV2 verifies the exact loaded predecessor or
// migrated head; a migrated head additionally resolves its immutable v1 prior.
// headKey remains an external storage coordinate, never a snapshot claim.
func VerifyMultiSeedWorkflowIdentityV2(ctx context.Context, workflowKey, headKey string, expected WorkflowIdentity, heads censussourcecontinuation.HeadStore, objects *MultiSeedObjectsV2) (WorkflowIdentity, error) {
	if ctx == nil || workflowKey == "" || headKey == "" || !validIdentity(expected) || heads == nil || objects == nil {
		return WorkflowIdentity{}, ErrInvalid
	}
	raw, err := heads.Load(ctx, headKey)
	if err != nil {
		return WorkflowIdentity{}, err
	}
	revision, err := ParseMultiSeedSuccessorHeadRevisionV2(raw)
	if err != nil {
		return WorkflowIdentity{}, err
	}
	if revision.Generation == 1 {
		var v head
		if strict(raw, &v) != nil || v.WorkflowKey != workflowKey || v.Identity != expected {
			return WorkflowIdentity{}, ErrInvalid
		}
		return v.Identity, nil
	}
	if revision.Generation != 2 {
		return WorkflowIdentity{}, ErrInvalid
	}
	v, err := parseMultiSeedSuccessorV2(raw)
	if err != nil || v.WorkflowKey != workflowKey || WorkflowIdentity(v.Identity) != expected {
		return WorkflowIdentity{}, ErrInvalid
	}
	if _, err = objects.ResolveSuccessorPredecessorV2(ctx, v.PriorSuccessor, workflowKey, headKey, expected, v.PredecessorHead); err != nil {
		return WorkflowIdentity{}, err
	}
	return WorkflowIdentity(v.Identity), nil
}
