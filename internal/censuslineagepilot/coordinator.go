package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"

	"lsp-trace/internal/censussourcecontinuation"
)

var (
	ErrConflict               = errors.New("census lineage pilot conflict")
	ErrAlreadyCommitted       = errors.New("census lineage pilot already committed")
	ErrIndeterminateExecution = errors.New("census lineage pilot indeterminate execution")
)

type ImmutableStore interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
}

type HeadStore interface {
	Load(context.Context, string) ([]byte, error)
	CompareAndSwap(context.Context, string, censussourcecontinuation.HeadRevision, []byte) censussourcecontinuation.CASResult
}

type Request struct {
	WorkflowKey   string
	HeadKey       string
	Expected      censussourcecontinuation.HeadRevision
	Manifest      Manifest
	ManifestBytes []byte
	Objects       ImmutableStore
	Heads         HeadStore
}

type Result struct {
	Head             censussourcecontinuation.HeadRevision
	ManifestSelector string
	Authority        uint8
	Accepted         bool
	State            string
}

func Coordinate(ctx context.Context, r Request) (Result, error) {
	if ctx == nil || r.Objects == nil || r.Heads == nil || r.WorkflowKey == "" || r.HeadKey == "" {
		return Result{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	manifest, err := validateRequestManifest(r.Manifest, r.ManifestBytes)
	if err != nil {
		return Result{}, err
	}
	currentRaw, err := r.Heads.Load(ctx, r.HeadKey)
	if err != nil {
		return Result{}, err
	}
	var current censussourcecontinuation.HeadRevision
	var currentHead head
	if len(currentRaw) != 0 {
		var parseErr error
		currentHead, parseErr = parseHead(currentRaw)
		if parseErr != nil || currentHead.WorkflowKey != r.WorkflowKey || currentHead.LineageID != manifest.LineageID {
			return Result{}, ErrInvalid
		}
		current = censussourcecontinuation.HeadRevision{Generation: currentHead.Generation, Selector: currentHead.Selector}
	}
	if current != r.Expected {
		return Result{}, ErrConflict
	}
	manifestSelector := Digest(r.ManifestBytes)
	if len(currentRaw) != 0 {
		if currentHead.ManifestSelector != manifestSelector {
			return Result{}, ErrConflict
		}
		got, getErr := r.Objects.Get(ctx, currentHead.ManifestSelector)
		if getErr != nil {
			return Result{}, getErr
		}
		if !bytes.Equal(got, r.ManifestBytes) || Digest(got) != currentHead.ManifestSelector {
			return Result{}, ErrInvalid
		}
		return Result{}, ErrAlreadyCommitted
	}
	selector, err := r.Objects.Put(ctx, bytes.Clone(r.ManifestBytes))
	if err != nil {
		return Result{}, err
	}
	got, err := r.Objects.Get(ctx, selector)
	if err != nil {
		return Result{}, err
	}
	if selector != manifestSelector || Digest(got) != selector || !bytes.Equal(got, r.ManifestBytes) {
		return Result{}, ErrInvalid
	}
	generation := r.Expected.Generation + 1
	if generation == 0 {
		return Result{}, ErrInvalid
	}
	_, headRaw, err := newHead(r.WorkflowKey, manifest.LineageID, selector, generation)
	if err != nil {
		return Result{}, err
	}
	cas := r.Heads.CompareAndSwap(ctx, r.HeadKey, r.Expected, headRaw)
	switch cas.Outcome {
	case censussourcecontinuation.CASCommitted:
		if cas.Err != nil {
			return Result{}, ErrInvalid
		}
		revision, parseErr := ParseHeadRevision(headRaw)
		if parseErr != nil {
			return Result{}, parseErr
		}
		return Result{Head: revision, ManifestSelector: selector, State: "COMMITTED"}, nil
	case censussourcecontinuation.CASNotCommitted:
		if cas.Err == nil {
			return Result{}, ErrConflict
		}
		if cancellation(cas.Err) {
			return Result{}, ErrIndeterminateExecution
		}
		return Result{}, cas.Err
	case censussourcecontinuation.CASCommitUncertain:
		return Result{}, ErrIndeterminateExecution
	default:
		if cancellation(cas.Err) {
			return Result{}, ErrIndeterminateExecution
		}
		if cas.Err != nil {
			return Result{}, cas.Err
		}
		return Result{}, ErrInvalid
	}
}

func validateRequestManifest(object Manifest, raw []byte) (Manifest, error) {
	parsed, err := ParseManifest(raw)
	if err != nil {
		return Manifest{}, err
	}
	objectRaw, err := EncodeManifest(object)
	if err != nil || !bytes.Equal(objectRaw, raw) {
		return Manifest{}, ErrInvalid
	}
	return parsed, nil
}

func cancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func VerifyReopen(ctx context.Context, workflowKey, headKey, lineageID string, expected []byte, objects ImmutableStore, heads HeadStore) (Result, error) {
	if ctx == nil || workflowKey == "" || headKey == "" || lineageID == "" || objects == nil || heads == nil {
		return Result{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	manifest, err := ParseManifest(expected)
	if err != nil || manifest.LineageID != lineageID {
		return Result{}, ErrInvalid
	}
	headRaw, err := heads.Load(ctx, headKey)
	if err != nil {
		return Result{}, err
	}
	h, err := parseHead(headRaw)
	if err != nil || h.WorkflowKey != workflowKey || h.LineageID != lineageID || h.ManifestSelector != Digest(expected) {
		return Result{}, ErrInvalid
	}
	selector := h.ManifestSelector
	got, err := objects.Get(ctx, selector)
	if err != nil {
		return Result{}, err
	}
	if Digest(got) != selector || !bytes.Equal(got, expected) {
		return Result{}, ErrInvalid
	}
	return Result{Head: censussourcecontinuation.HeadRevision{Generation: h.Generation, Selector: h.Selector}, ManifestSelector: selector, State: "COMMITTED"}, nil
}
