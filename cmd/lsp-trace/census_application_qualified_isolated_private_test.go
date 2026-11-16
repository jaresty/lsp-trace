//go:build census_qualified_cli_isolated

package main

// Explicit-file-only refusal harness. Do not enable this tag in package-wide
// tests: real unrelated runtime definitions intentionally are not compiled.
// The actual, unmodified census_runner.go is compiled with these inert types.
import (
	"context"
	"errors"
	"io"
	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censuscontrolflow"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censuspreflight"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/sessionruntime"
	"time"
)

var qualifiedProfileCallsPrivate int
var qualifiedRefusalPrivate = errors.New("isolated runtime refusal")

type profileFlags struct{ Name, ConfigPath string }
type qualifiedProfilePrivate struct {
	Command           string
	Args, Environment []string
	LanguageID        string
}

func loadRequestedProfile(string, profileFlags) (qualifiedProfilePrivate, error) {
	qualifiedProfileCallsPrivate++
	return qualifiedProfilePrivate{}, qualifiedRefusalPrivate
}

type initializedAcquisitionRunnerConfig struct {
	manager                      *sessionruntime.Manager
	providerProbe                *privateProviderProbe
	privateEnumerationProjection bool
	start                        sessionruntime.StartRequest
	timeout, requestTimeout      time.Duration
	stderr                       io.Writer
}
type censusCoreConfig struct {
	privateFreshRunner func() (initializedAcquisitionRunnerConfig, error)
	runner             initializedAcquisitionRunnerConfig
	limits             acquisitionops.Limits
	callHierarchy      bool
	diagnostic         *censusdiagnostic.Recorder
	controlFlow        *censusControlFlowObserver
}
type censusCoreDependencies struct{}
type censusPublicationOutcome struct {
	EnumerationDiagnostic      *censuspreflight.Diagnostic
	DiscoveryDiagnostic        *censusresult.DiscoveryDiagnostic
	RequestReceiptV2           *censusrequest.ReceiptV2
	RequestReceipt             *censusrequest.Receipt
	PrivateDiscoveryCauses     *struct{}
	PrivateTerminalObservation *struct{}
	PrivateChunkTerminal       *struct{}
	EnumerationReceipt         *censusenumerationprojection.Receipt
	Result                     *censusCLIResult
	Diagnostic                 *censusCLIDiagnostic
	controlFlow                *censuscontrolflow.Record
}

func productionCensusCoreDependencies() censusCoreDependencies { panic("runtime forbidden") }
func runCensusCore(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
	panic("runtime forbidden")
}
func runCensusNativeCatalog(censusCLIOptions, censusCoreConfig, io.Writer, io.Writer) int {
	panic("native forbidden")
}
func runCensusInventoryPrivate(censusCLIOptions, censusCoreConfig, io.Writer, io.Writer) int {
	panic("inventory execution forbidden")
}
func runCensusInventoryBridgePrivate(censusCLIOptions, io.Writer, io.Writer, censusRunnerDependencies) int {
	panic("bridge forbidden")
}
func runCensusCatalogResume(censusCLIOptions, io.Writer, io.Writer, censusRunnerDependencies) int {
	panic("resume forbidden")
}
func openCensusCatalogHost(string, string) (*continuationhost.Bundle, error) {
	panic("catalog forbidden")
}
func publishCensusCatalogDescriptor(context.Context, *continuationhost.Bundle, continuationhost.DescriptorInput) (string, error) {
	panic("catalog forbidden")
}
func resolveCensusCatalogDescriptor(context.Context, *continuationhost.Bundle, string) (continuationhost.DescriptorInput, error) {
	panic("catalog forbidden")
}
func censusCatalogContract(string) (censuscontinuation.ContinuationContract, error) {
	panic("catalog forbidden")
}
func newLazyCensusCatalogWorker(string, string) censuscontinuation.WorkerV2 {
	panic("catalog forbidden")
}

type privateProviderProbe struct{ byteTimeNotifications bool }

func newPrivateProviderProbeVersion(string, string) (*privateProviderProbe, error) {
	panic("provider forbidden")
}
func (*privateProviderProbe) finalize() error { panic("provider forbidden") }

type censusControlFlowObserver struct{}

func newCensusControlFlowObserver() *censusControlFlowObserver { panic("control flow forbidden") }
func (*censusControlFlowObserver) record() (censuscontrolflow.Record, bool) {
	panic("control flow forbidden")
}

type censusWorkspaceEnumerator struct {
	workspace         string
	roots             []string
	filters           censusacquisition.Filters
	maxFiles          int
	prunedDirectories []string
}

func (censusWorkspaceEnumerator) Enumerate(context.Context) ([]censusacquisition.SourceFile, error) {
	panic("enumeration forbidden")
}

const censusMaxDiscoveredEntries = 10000
const censusMaxEnumerationWork = 100000
const censusMaxSourcePathBytes = 16384
const censusMaxSourceDepth = 64

func matchesAnyDiscoveryPattern([]string, string) bool                 { panic("enumeration forbidden") }
func marshalPrivateCensusDiscoveryCauses(struct{}) ([]byte, error)     { panic("diagnostic forbidden") }
func marshalPrivateCensusTerminalObservation(struct{}) ([]byte, error) { panic("diagnostic forbidden") }
func marshalPrivateCensusChunkTerminal(struct{}) ([]byte, error)       { panic("execution forbidden") }

type censusInventoryConfigReaderPrivate struct {
	ctx context.Context
	r   io.Reader
}

func (r censusInventoryConfigReaderPrivate) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
