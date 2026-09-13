package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sync"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
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

func runInitializedCensusAcquisition(ctx context.Context, runtime *initializedAcquisitionRuntime, discoverer censusacquisition.Discoverer, limits acquisitionops.Limits) (censusAssembly, error) {
	if runtime == nil {
		return censusAssembly{}, errors.New("initialized census runtime required")
	}
	acquirer := newInitializedCensusBatchAcquirer(runtime, limits)
	projection, err := (censusacquisition.Core{Discoverer: discoverer, Acquirer: acquirer}).Run(ctx, acquirer.session.identity())
	if err != nil {
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
