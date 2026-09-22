package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sync"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
)

// censusAssembly and censusPublicationCapability are package-main authority.
// Neither can be named or constructed by another package.
type censusAssemblyState struct {
	projection censusacquisition.Projection
	token      [32]byte
	mu         sync.Mutex
	published  bool
}

type censusAssembly struct {
	state *censusAssemblyState
	token [32]byte
}

type censusPublicationCapability struct {
	state *censusAssemblyState
	token [32]byte
}

type censusPublisher interface {
	publishCensus(context.Context, censusPublicationCapability) error
}

type censusPublisherFunc func(context.Context, censusPublicationCapability) error

func (f censusPublisherFunc) publishCensus(ctx context.Context, capability censusPublicationCapability) error {
	return f(ctx, capability)
}

type censusContinuationCustody struct {
	projection  censusacquisition.Projection
	result      censusresult.Result
	publication censusprogramc.VerifiedPublication
	metadata    programccompose.ExactMetadata
	workspace   censuscontinuation.WorkspaceIdentity
}

type censusPublicationOutcome struct {
	Result              *censusCLIResult
	Diagnostic          *censusCLIDiagnostic
	DiscoveryDiagnostic *censusresult.DiscoveryDiagnostic
	RequestReceipt      *censusrequest.Receipt
	continuation        *censusContinuationCustody
}

func (o censusPublicationOutcome) BuildCommittedHandoff() (censuscontinuation.CommittedHandoff, error) {
	if o.Result == nil || o.Diagnostic != nil || o.continuation == nil {
		return censuscontinuation.CommittedHandoff{}, errors.New("committed continuation custody incomplete")
	}
	custody, err := cloneCensusContinuationCustody(*o.continuation)
	if err != nil {
		return censuscontinuation.CommittedHandoff{}, errors.New("committed continuation custody incomplete")
	}
	return censuscontinuation.BuildHandoff(censuscontinuation.BuildInput{
		Result: custody.result, Projection: custody.projection, Publication: custody.publication,
		Metadata: custody.metadata, Workspace: custody.workspace,
	})
}

type censusCaptureSetPublisher interface {
	PublishCaptureSet(captureset.Manifest, [][]byte, captureset.ExactBytesAuthority) captureset.PublicationResult
	Verify(string, captureset.ExactBytesAuthority) (captureset.Manifest, error)
	ResolveConstituent(string, string, captureset.ExactBytesAuthority) ([]byte, error)
}

var openCensusPublicationRoot = publication.OpenRoot
var newCensusCaptureSetPublisher = func(root *publication.Root) censusCaptureSetPublisher {
	return captureset.NewPublisher(root)
}

func publishCensusCaptureSetWithContinuation(ctx context.Context, capability censusPublicationCapability, rootPath string, metadata programccompose.ExactMetadata) censusPublicationOutcome {
	return publishCensusCaptureSetInternal(ctx, capability, rootPath, &metadata)
}

func publishCensusCaptureSet(ctx context.Context, capability censusPublicationCapability, rootPath string) censusPublicationOutcome {
	return publishCensusCaptureSetInternal(ctx, capability, rootPath, nil)
}

func publishCensusCaptureSetInternal(ctx context.Context, capability censusPublicationCapability, rootPath string, metadata *programccompose.ExactMetadata) censusPublicationOutcome {
	var outcome censusPublicationOutcome
	err := publishCensus(ctx, capability, censusPublisherFunc(func(context.Context, censusPublicationCapability) error {
		projection, _, cloneErr := cloneCensusProjection(capability.state.projection)
		if cloneErr != nil {
			return cloneErr
		}
		root, openErr := openCensusPublicationRoot(rootPath)
		if openErr != nil {
			return openErr
		}
		defer root.Close()
		publisher := newCensusCaptureSetPublisher(root)
		exact := make([][]byte, len(projection.Constituents))
		for i := range projection.Constituents {
			exact[i] = append([]byte(nil), projection.Constituents[i].Raw...)
		}
		authority := captureset.NativeV5Authority()
		published := publisher.PublishCaptureSet(projection.Manifest, exact, authority)
		if published.Err != nil || published.Receipt == nil {
			return errors.New("capture-set publication failed")
		}
		verificationStatus := published.Receipt.VerificationStatus
		verified, verifyErr := publisher.Verify(published.Receipt.Selector, authority)
		resolved := make([]censusprogramc.ResolvedConstituent, 0, len(verified.Constituents))
		if verifyErr == nil {
			for _, constituent := range verified.Constituents {
				raw, resolveErr := publisher.ResolveConstituent(published.Receipt.Selector, constituent.ImmutableSelector, authority)
				if resolveErr != nil {
					verifyErr = resolveErr
					break
				}
				resolved = append(resolved, censusprogramc.ResolvedConstituent{ImmutableSelector: constituent.ImmutableSelector, Bytes: append([]byte(nil), raw...)})
			}
		}
		if verifyErr != nil {
			verificationStatus = "COMMITTED_VERIFICATION_FAILED"
		}
		receipt := publication.BoundFileReceipt{
			FinalSelector: published.Receipt.Selector, Digest: published.Receipt.ArtifactSHA256,
			ByteLength: published.Receipt.ByteLength, Mechanism: published.Receipt.Mechanism,
			NamespaceAtomic: published.Receipt.NamespaceAtomic, CrashDurability: published.Receipt.CrashDurability,
			DirectorySyncStatus: published.Receipt.DirectorySyncStatus, CloseStatus: published.Receipt.CloseStatus,
			VerificationStatus: verificationStatus,
		}
		result, resultErr := buildCensusCLIResult(projection, receipt)
		if resultErr != nil {
			// Publication has committed; projection failure is degradation, never retryable failure.
			diagnostic, _ := buildCensusCLIDiagnostic(censusStageCommitted, nil)
			outcome.Diagnostic = &diagnostic
			return nil
		}
		outcome.Result = &result
		if verificationStatus != "VERIFIED" || receipt.DirectorySyncStatus == publication.DirectorySyncFailed || receipt.CloseStatus == publication.CloseFailed {
			diagnostic, _ := buildCensusCLIDiagnostic(censusStageCommitted, nil)
			outcome.Diagnostic = &diagnostic
			return nil
		}
		if metadata != nil {
			historical, historicalErr := censusresult.Build(projection, censusresult.PublicationEvidence{
				Selector: published.Receipt.Selector, Digest: published.Receipt.ArtifactSHA256, ByteLength: published.Receipt.ByteLength,
				VerificationStatus: verificationStatus, DirectorySyncStatus: published.Receipt.DirectorySyncStatus, CloseStatus: published.Receipt.CloseStatus,
			})
			if historicalErr == nil {
				workspaceURI := (&url.URL{Scheme: "file", Path: projection.Workspace}).String()
				workspaceSum := sha256.Sum256([]byte("lsp-trace:census-workspace:v1\x00" + workspaceURI))
				m := *metadata
				m.WorkspaceIdentity = workspaceURI
				custody := censusContinuationCustody{
					projection: projection, result: historical,
					publication: censusprogramc.VerifiedPublication{Receipt: *published.Receipt, Manifest: verified, Resolved: resolved},
					metadata:    m,
					workspace:   censuscontinuation.WorkspaceIdentity{URI: workspaceURI, Digest: "sha256:" + hex.EncodeToString(workspaceSum[:])},
				}
				if cloned, cloneErr := cloneCensusContinuationCustody(custody); cloneErr == nil {
					outcome.continuation = &cloned
				}
			}
		}
		return nil
	}))
	if err != nil {
		diagnostic, _ := buildCensusCLIDiagnostic(censusStagePublication, nil)
		outcome.Result = nil
		outcome.Diagnostic = &diagnostic
	}
	return outcome
}

func runInitializedCensusAcquisition(ctx context.Context, runtime *initializedAcquisitionRuntime, discoverer censusacquisition.Discoverer, limits acquisitionops.Limits, downDepth, upDepth, maxBatchTargets int) (censusAssembly, error) {
	if runtime == nil {
		return censusAssembly{}, errors.New("initialized census runtime required")
	}
	acquirer := newInitializedCensusBatchAcquirer(runtime, limits)
	planning := &censusacquisition.PlanningConfig{DownDepth: downDepth, UpDepth: upDepth, MaxBatchTargets: maxBatchTargets}
	projection, err := (censusacquisition.Core{Discoverer: discoverer, Acquirer: acquirer, Planning: planning}).Run(ctx, acquirer.session.identity())
	if err != nil {
		var batchFailure interface{ BatchOrdinal() int }
		if errors.As(err, &batchFailure) && batchFailure.BatchOrdinal() > 0 {
			return censusAssembly{}, &censusBatchAcquisitionFailure{ordinal: batchFailure.BatchOrdinal(), Err: err}
		}
		return censusAssembly{}, err
	}
	projection, canonical, err := cloneCensusProjection(projection)
	if err != nil {
		return censusAssembly{}, err
	}
	token := sha256.Sum256(append([]byte("census-assembly-capability\x00"), canonical...))
	state := &censusAssemblyState{projection: projection, token: token}
	return censusAssembly{state: state, token: token}, nil
}

func (a censusAssembly) inspect() (censusacquisition.Projection, error) {
	if !a.valid() {
		return censusacquisition.Projection{}, errors.New("invalid census assembly capability")
	}
	projection, _, err := cloneCensusProjection(a.state.projection)
	return projection, err
}

func (a censusAssembly) publicationCapability() (censusPublicationCapability, error) {
	if !a.valid() {
		return censusPublicationCapability{}, errors.New("invalid census assembly capability")
	}
	return censusPublicationCapability{state: a.state, token: a.token}, nil
}

func (a censusAssembly) valid() bool {
	return a.state != nil && a.token != [32]byte{} && a.token == a.state.token
}

func publishCensus(ctx context.Context, capability censusPublicationCapability, publisher censusPublisher) error {
	if publisher == nil || capability.state == nil || capability.token == [32]byte{} || capability.token != capability.state.token {
		return errors.New("invalid census publication capability")
	}
	capability.state.mu.Lock()
	defer capability.state.mu.Unlock()
	if capability.state.published {
		return errors.New("census publication capability replay")
	}
	_, canonical, err := cloneCensusProjection(capability.state.projection)
	if err != nil || sha256.Sum256(append([]byte("census-assembly-capability\x00"), canonical...)) != capability.token {
		return errors.New("census assembly mutation")
	}
	capability.state.published = true
	return publisher.publishCensus(ctx, capability)
}

func cloneCensusContinuationCustody(in censusContinuationCustody) (censusContinuationCustody, error) {
	projection, _, err := cloneCensusProjection(in.projection)
	if err != nil {
		return censusContinuationCustody{}, err
	}
	out := in
	out.projection = projection
	out.publication.Resolved = append([]censusprogramc.ResolvedConstituent(nil), in.publication.Resolved...)
	for i := range out.publication.Resolved {
		out.publication.Resolved[i].Bytes = append([]byte(nil), in.publication.Resolved[i].Bytes...)
	}
	manifestBytes, err := json.Marshal(in.publication.Manifest)
	if err != nil {
		return censusContinuationCustody{}, err
	}
	if err := json.Unmarshal(manifestBytes, &out.publication.Manifest); err != nil {
		return censusContinuationCustody{}, err
	}
	return out, nil
}

func cloneCensusProjection(projection censusacquisition.Projection) (censusacquisition.Projection, []byte, error) {
	canonical, err := json.Marshal(projection)
	if err != nil {
		return censusacquisition.Projection{}, nil, err
	}
	var clone censusacquisition.Projection
	if err := json.Unmarshal(canonical, &clone); err != nil {
		return censusacquisition.Projection{}, nil, err
	}
	return clone, canonical, nil
}
