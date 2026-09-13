package main

import (
	"context"
	"crypto/sha256"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
)

type censusPublisherFunc func(context.Context, censusPublicationCapability) error

func (f censusPublisherFunc) publishCensus(ctx context.Context, capability censusPublicationCapability) error {
	return f(ctx, capability)
}

func TestCensusAssemblyCapabilityZeroAndPublicationReplay(t *testing.T) {
	if _, err := (censusAssembly{}).inspect(); err == nil {
		t.Fatal("ASSERT_ZERO_CENSUS_ASSEMBLY_REJECTED")
	}
	if err := publishCensus(context.Background(), censusPublicationCapability{}, censusPublisherFunc(func(context.Context, censusPublicationCapability) error { return nil })); err == nil {
		t.Fatal("ASSERT_ZERO_CENSUS_PUBLICATION_REJECTED")
	}

	projection := censusacquisition.Projection{ManifestBytes: []byte("product-created-test-completion")}
	_, canonical, err := cloneCensusProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	token := sha256.Sum256(append([]byte("census-assembly-capability\x00"), canonical...))
	state := &censusAssemblyState{projection: projection, token: token}
	assembly := censusAssembly{state: state, token: token}
	capability, err := assembly.publicationCapability()
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := assembly.inspect()
	if err != nil {
		t.Fatal(err)
	}
	inspected.ManifestBytes[0] ^= 0xff
	if assembly.state.projection.ManifestBytes[0] == inspected.ManifestBytes[0] {
		t.Fatal("ASSERT_CENSUS_ASSEMBLY_INSPECTION_IMMUTABLE")
	}
	calls := 0
	publisher := censusPublisherFunc(func(context.Context, censusPublicationCapability) error {
		calls++
		return nil
	})
	if err := publishCensus(context.Background(), capability, publisher); err != nil || calls != 1 {
		t.Fatalf("ASSERT_PRODUCT_COMPLETION_PUBLISHES_ONCE: err=%v calls=%d", err, calls)
	}
	if err := publishCensus(context.Background(), capability, publisher); err == nil || calls != 1 {
		t.Fatalf("ASSERT_CENSUS_PUBLICATION_REPLAY_REJECTED: err=%v calls=%d", err, calls)
	}
}

func TestRunInitializedCensusAcquisitionRejectsMissingRuntime(t *testing.T) {
	if _, err := runInitializedCensusAcquisition(context.Background(), nil, nil, acquisitionops.Limits{}); err == nil {
		t.Fatal("ASSERT_INITIALIZED_RUNTIME_REQUIRED")
	}
}
