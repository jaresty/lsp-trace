// Private managed grouping entries delegate the entire fixed operation to its
// inventorysuccessor owner. This package cannot publish operation completion.
package adr0011methodresult

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

var ErrPrivateManagedGrouping = inventorysuccessor.ErrPrivateManagedGrouping

type PrivateManagedGroupingInput = inventorysuccessor.PrivateManagedGroupingInput
type PrivateManagedGroupingRequest = inventorysuccessor.PrivateManagedGroupingRequest
type PrivateManagedGroupingOccurrence = inventorysuccessor.PrivateManagedGroupingOccurrence

func PrivateManagedGroupingOperationBinding(m *sessionruntime.Manager, q PrivateManagedGroupingRequest, mode string, h sessionruntime.PrivateGroupingOwnedHandoff) (inventorysuccessor.PrivateManagedInputBindingV2, error) {
	return inventorysuccessor.PrivateManagedGroupingOperationBinding(m, q, mode, h)
}
func privateManagedFacadeError(err error) error {
	if errors.Is(err, inventorysuccessor.ErrPrivateManagedGroupingPublication) {
		return errors.Join(ErrPrivateManagedGrouping, ErrChain)
	}
	return err
}
func AcquirePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	out, err := inventorysuccessor.AcquirePrivateManagedGroupingInput(ctx, m, q)
	return out, privateManagedFacadeError(err)
}
func IssuePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, h sessionruntime.PrivateGroupingOwnedHandoff, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	out, err := inventorysuccessor.IssuePrivateManagedGroupingInput(ctx, m, h, q)
	return out, privateManagedFacadeError(err)
}

type privateGroupingNative struct {
	Method, SessionID     string
	Generation, RequestID uint64
	Params, Result        []byte
	Write                 sessionruntime.RequestWriteObservation
	Read                  sessionruntime.ResponseReadObservation
	Source                *sessionruntime.OwnedDocumentBinding
}

func privateGroupingNativePair(p sessionruntime.OwnedMethodPair) privateGroupingNative {
	return privateGroupingNative{p.Method, p.SessionID, p.Generation, p.Key.ID, []byte(p.Params), []byte(p.Result), p.Write, p.Read, p.Source}
}

type privateGroupingSupply struct {
	Classification, SessionID string
	Generation                uint64
	URI                       string
	DocumentVersion           int
	Method                    string
	Content, Params           []byte
}

func privateGroupingDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func privateGroupingParseLimits() referencegrouping.ParseLimits {
	return referencegrouping.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}
}
