//go:build darwin || linux

package main

// This additive, unadvertised route is PRIVATE_AST, not ADR0012's qualified
// CALLS_ONLY_FROZEN_CENSUS_V1 coordinator. It never restores owners from JSON.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/censuspreflight"
	"lsp-trace/internal/managedprocess"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
	"lsp-trace/internal/session"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

const censusInventoryProfilePrivate = "PRIVATE_CENSUS_AST_V2"

type censusInventoryConfigPrivate struct {
	Profile            string
	RevisionAssertion  string
	MaxOrigins         int
	OperationTimeoutMS int // explicit whole continuation budget; acquisition retains CLI timeout
	Policy             pilot.ScopeEffectivePolicyPrivateV2
	Preparation        prep.Limits
	Post               prep.PostPreparationLimits
	Seeds              []uint64
	// Nil is preparation-only. Approval is exact scope/config data, not an
	// authenticated acceptance decision. Parent must separately approve disclosure.
	Automation *prep.AutomaticHandoffConfigPrivateV2
}

type censusInventoryFilePrivate struct {
	File        censusacquisition.SourceFile
	Disposition string
	Origin      int // -1 means no origin attempted; never a feature assignment.
}
type censusInventoryOriginPrivate struct {
	Ordinal        int
	Files          []int
	Status         string
	ScopeSHA256    string
	Discovery      json.RawMessage // complete original Census ledgers, including noncallables
	Batches        []censusInventoryBatchPrivate
	ProducerCopies [2]pilot.ProducerPacketCopyReceiptPrivateV2
}
type censusInventoryBatchPrivate struct {
	Ordinal                int
	Status, ArtifactSHA256 string
	ArtifactBytes          uint64
}
type censusInventoryPagePrivate struct {
	Selector, SHA256    string
	ByteLength, Entries int
}

type censusInventoryReportPrivate struct {
	Schema, Profile, InputFamily, Status, Failure, AttemptDirectory string
	Authority                                                       int
	Accepted, Qualified                                             bool
	FeatureIdentity, SourceCompleteness, AbsenceInference           string
	ScopeSHA256                                                     string
	Configuration                                                   censusInventoryConfigPrivate
	Sources, Includes, Excludes, PrunedDirectories                  []string
	Files                                                           []censusInventoryFilePrivate
	FileDenominator                                                 int
	EnumerationLimit                                                *censuspreflight.Diagnostic
	FilePages                                                       []censusInventoryPagePrivate
	Origins                                                         []censusInventoryOriginPrivate
	Coverage                                                        []prep.Coverage
	Index                                                           []prep.TargetIndex
	PreparationSHA256                                               string
	HostExportBindingPath, HostExportBindingSHA256                  string
	PreparationBytes                                                int
	Outcome                                                         *prep.PostPublicationOutcomePrivateV2
	Inventory                                                       *prep.AutomaticInventoryPrivateV2
	Session                                                         censusacquisition.SessionIdentity
	SessionCode                                                     int
	StopCalls                                                       int
	StopFailure                                                     string
	StopIntentID                                                    string
	ShutdownCalls                                                   int
	ShutdownJoined, CleanupVerified                                 bool
	ApplicationCoverage                                             string
	Errors                                                          []string
}
type censusInventoryDependenciesPrivate struct {
	enumerate  func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error)
	runSession func(initializedAcquisitionRunnerConfig, func(context.Context, *initializedAcquisitionRuntime) int) int
	automatic  func(prep.AutomaticHandoffConfigPrivateV2, string, prep.PostPreparationPrivateV2) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error)
}

func productionCensusInventoryDependenciesPrivate() censusInventoryDependenciesPrivate {
	return censusInventoryDependenciesPrivate{enumerate: enumerateCensusPreflightContextPrivate, runSession: productionCensusCoreDependencies().runSession, automatic: prep.NewAutomaticHandoffPrivateV2}
}

func loadCensusInventoryConfigPrivate(path string) (c censusInventoryConfigPrivate, err error) {
	f, err := os.Open(path)
	if err != nil {
		return c, errors.New("private inventory config unavailable")
	}
	raw, e := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	ce := f.Close()
	if e != nil || ce != nil || len(raw) > 64<<10 {
		return c, errors.New("private inventory config byte bound")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil {
		return c, errors.New("private inventory config invalid")
	}
	canonical, _ := json.Marshal(c)
	if !bytes.Equal(raw, canonical) || c.Profile != censusInventoryProfilePrivate || c.MaxOrigins != 1 || len(c.Seeds) < 2 || len(c.Seeds) > 16 || c.OperationTimeoutMS < c.Policy.TimeoutMS || c.OperationTimeoutMS < 1 || c.OperationTimeoutMS > 240000 {
		return c, errors.New("private inventory config noncanonical or profile/bounds invalid")
	}
	if c.Automation != nil && prep.ValidateAutomaticHandoffPrivateV2(*c.Automation) != nil {
		return c, errors.New("private inventory handoff approval invalid")
	}
	return c, nil
}

func censusInventoryPlanPrivate(o censusCLIOptions, c censusInventoryConfigPrivate, files []censusacquisition.SourceFile) ([]censusInventoryFilePrivate, [][]int) {
	files = append([]censusacquisition.SourceFile(nil), files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	rows := make([]censusInventoryFilePrivate, len(files))
	groups := [][]int{}
	for i, f := range files {
		rows[i] = censusInventoryFilePrivate{File: f, Disposition: "UNPROCESSED", Origin: -1}
		if f.Disposition != "" {
			rows[i].Disposition = string(f.Disposition)
			continue
		}
		if !(censusacquisition.Filters{Includes: o.Includes, Excludes: o.Excludes}).Select(f.Path, func(pattern, path string) bool { return matchesAnyDiscoveryPattern([]string{pattern}, path) }) {
			rows[i].Disposition = "EXCLUDED"
			continue
		}
		if f.LanguageID == "" {
			rows[i].Disposition = "UNSUPPORTED_LANGUAGE"
			continue
		}
		if len(groups) == 0 || len(groups[len(groups)-1]) == c.Policy.MaxNominationFiles {
			if len(groups) == c.MaxOrigins {
				rows[i].Disposition = "RESOURCE_GAP_ORIGIN_LIMIT"
				continue
			}
			groups = append(groups, []int{})
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], i)
	}
	return rows, groups
}

func writeCensusInventoryFilePrivate(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) > 16<<20 {
		return errors.New("private inventory report bound")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, e := f.Write(raw)
	return errors.Join(e, f.Sync(), f.Close())
}

func runCensusInventoryPrivate(o censusCLIOptions, cfg censusCoreConfig, stdout, stderr io.Writer) int {
	return runCensusInventoryWithDependenciesPrivate(o, cfg, stdout, stderr, productionCensusInventoryDependenciesPrivate())
}
func runCensusInventoryWithDependenciesPrivate(o censusCLIOptions, cfg censusCoreConfig, stdout, stderr io.Writer, deps censusInventoryDependenciesPrivate) int {
	joined := false
	shutdown := func() error {
		if joined {
			return nil
		}
		joined = true
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		return cfg.runner.manager.Shutdown(cleanup)
	}
	defer shutdown() // closes unused Managers on configuration/refusal paths too
	cfg.runner.shutdownManagedByCaller = true
	c, err := loadCensusInventoryConfigPrivate(o.PrivateInventoryConfigPath)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, true, err)
	}
	// CLI acquisition controls remain binding. Config may not silently change them.
	p := c.Policy
	if p.MaxNominationFiles < 1 || p.MaxNominationFiles > 8 || p.Planning.MaxBatchTargets != o.MaxBatchTargets || p.MaxNodes != o.MaxNodes || p.Planning.DownDepth != o.DownDepth || p.Planning.UpDepth != o.UpDepth || p.TimeoutMS != int(o.Timeout.Milliseconds()) || p.RequestTimeoutMS != int(o.RequestTimeout.Milliseconds()) {
		return writeCensusFailure(stderr, censusStageConfig, true, errors.New("private inventory policy differs from CLI bounds"))
	}
	if censusInventoryRelatedPathsPrivate(o.Workspace, o.PublicationRoot) {
		return writeCensusFailure(stderr, censusStageConfig, true, errors.New("private inventory source/publication roots overlap"))
	}
	// Mailbox topology must be admitted before even the attempt/denominator writes.
	if c.Automation != nil && (prep.CheckAutomaticHandoffDirectoryPrivateV2(c.Automation.Directory) != nil || censusInventoryRelatedPathsPrivate(c.Automation.Directory, o.Workspace) || censusInventoryRelatedPathsPrivate(c.Automation.Directory, o.PublicationRoot)) {
		return writeCensusFailure(stderr, censusStageConfig, true, errors.New("private inventory handoff roots overlap or unavailable"))
	}
	lifetime, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(lifetime, time.Duration(c.OperationTimeoutMS)*time.Millisecond)
	defer cancel()
	// Fresh namespace is the attempt identity. No saved receipt or generation is
	// accepted, and no failed attempt is overwritten, resumed, retried or deleted.
	attempt, err := os.MkdirTemp(o.PublicationRoot, "private-census-inventory-")
	if err != nil {
		return writeCensusFailure(stderr, censusStagePublication, true, err)
	}
	r := censusInventoryReportPrivate{Schema: "lsp-trace.census-application-inventory.private.v1", Profile: censusInventoryProfilePrivate, InputFamily: "PRIVATE_AST", Status: "PREPARING", AttemptDirectory: attempt, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Configuration: c, Sources: o.Sources, Includes: o.Includes, Excludes: o.Excludes, PrunedDirectories: o.PrunedDirectories, ApplicationCoverage: "UNKNOWN"}
	// Approval pins the full automatic nomination denominator and all acquisition
	// and preparation limits, not a caller-selected file list or a target count.
	var closes []func() error
	finish := func(cause error) int {
		for i := len(closes) - 1; i >= 0; i-- {
			if e := closes[i](); e != nil {
				r.Errors = append(r.Errors, "ROOT_CLOSE_FAILED")
				cause = errors.Join(cause, e)
			}
		}
		if r.Files != nil {
			var pageErr error
			r.FilePages, pageErr = retainCensusInventoryPagesPrivate(attempt, "terminal-files", r.Files)
			cause = errors.Join(cause, pageErr)
			// Large denominators live losslessly in named pages, not a clipped
			// inline array. Small reports retain the convenience view.
			if len(r.Files) > 128 {
				r.Files = nil
			}
		}
		if cause != nil {
			r.Status = "RETAINED_FAILURE"
			if r.Failure == "" {
				r.Failure = "PRIVATE_CONTINUATION_FAILED"
			}
		}
		if e := writeCensusInventoryFilePrivate(filepath.Join(attempt, "report.json"), r); e != nil {
			fmt.Fprintln(stderr, "private inventory report retention failed")
			return 1
		}
		// Metadata only. Exact source and packets stay under the private root.
		raw, _ := json.Marshal(struct {
			Profile, InputFamily, Status, AttemptDirectory, ScopeSHA256 string
			Authority                                                   int
			Accepted, Qualified                                         bool
		}{r.Profile, r.InputFamily, r.Status, attempt, r.ScopeSHA256, 0, false, false})
		if _, e := fmt.Fprintf(stdout, "%s\n", raw); e != nil {
			return 1
		}
		if cause != nil {
			return 1
		}
		return 0
	}
	files, enumErr := deps.enumerate(ctx, o)
	if enumErr != nil {
		r.Failure = "ENUMERATION_FAILED"
		var limit *source.LimitError
		if errors.As(enumErr, &limit) {
			r.EnumerationLimit = &censuspreflight.Diagnostic{SchemaVersion: censuspreflight.DiagnosticSchemaVersion, Category: limit.Category, Observed: limit.Observed, Limit: limit.Limit}
		}
		return finish(enumErr)
	}
	rows, groups := censusInventoryPlanPrivate(o, c, files)
	r.Files = rows
	r.FileDenominator = len(rows)
	plannedPages, pageErr := retainCensusInventoryPagesPrivate(attempt, "planned-files", rows)
	if pageErr != nil {
		return finish(pageErr)
	}
	r.ApplicationCoverage = "BOUNDED_ACCOUNTED_NOT_SOURCE_COMPLETE"
	for _, row := range rows {
		if strings.HasPrefix(row.Disposition, "RESOURCE_GAP") {
			r.ApplicationCoverage = "PARTIAL_RESOURCE_GAPS"
		}
	}
	// Freeze every automatically selected group before startup or disclosure.
	// Approval includes observed source hashes/lengths, not just mutable paths.
	frozenScopes := make([]pilot.FrozenScopePrivateV2, len(groups))
	manifests := make([]json.RawMessage, len(groups))
	for ordinal, indices := range groups {
		r.Origins = append(r.Origins, censusInventoryOriginPrivate{Ordinal: ordinal, Files: indices, Status: "FREEZE_ATTEMPTED"})
		subset := make([]censusacquisition.SourceFile, len(indices))
		for j, i := range indices {
			subset[j] = r.Files[i].File
			r.Files[i].Origin = ordinal
			r.Files[i].Disposition = "FREEZE_ATTEMPTED"
		}
		frozen, e := pilot.FreezeCensusScopePrivateV2(ctx, o.Workspace, c.RevisionAssertion, subset, p)
		if e != nil {
			r.Origins[ordinal].Status = "SOURCE_FREEZE_FAILED"
			for _, i := range indices {
				r.Files[i].Disposition = "SOURCE_FREEZE_FAILED"
			}
			return finish(e)
		}
		frozenScopes[ordinal] = frozen
		manifests[ordinal] = frozen.ManifestBytes()
		r.Origins[ordinal].ScopeSHA256 = pilot.Digest(manifests[ordinal])
		r.Origins[ordinal].Status = "FROZEN"
		for _, i := range indices {
			r.Files[i].Disposition = "FROZEN"
		}
	}
	approvalConfig := c
	approvalConfig.Automation = nil
	scopeBytes, _ := json.Marshal(struct {
		Workspace                           string
		Config                              censusInventoryConfigPrivate
		Sources, Includes, Excludes, Pruned []string
		Files                               []censusInventoryPagePrivate
		Manifests                           []json.RawMessage
	}{o.Workspace, approvalConfig, o.Sources, o.Includes, o.Excludes, o.PrunedDirectories, plannedPages, manifests})
	r.ScopeSHA256 = pilot.Digest(scopeBytes)
	if err = writeCensusInventoryFilePrivate(filepath.Join(attempt, "scope.json"), json.RawMessage(scopeBytes)); err != nil {
		return finish(err)
	}
	if c.Automation != nil && c.Automation.ScopeSHA256 != r.ScopeSHA256 {
		r.Failure = "DISCLOSURE_SCOPE_MISMATCH"
		return finish(errors.New(r.Failure))
	}
	if c.Automation != nil {
		// A deterministic metadata-only export binding is separate from the
		// fresh mailbox; it never becomes a v4 worker-launch configuration.
		r.HostExportBindingPath = c.Automation.Directory + ".census-export.json"
		if _, e := os.Lstat(r.HostExportBindingPath); !errors.Is(e, os.ErrNotExist) {
			r.Failure = "HOST_EXPORT_BINDING_NOT_FRESH"
			return finish(errors.New(r.Failure))
		}
		if prep.CheckAutomaticHandoffDirectoryPrivateV2(c.Automation.Directory) != nil || censusInventoryRelatedPathsPrivate(c.Automation.Directory, o.Workspace) || censusInventoryRelatedPathsPrivate(c.Automation.Directory, o.PublicationRoot) {
			r.Failure = "HANDOFF_TOPOLOGY_REFUSED"
			return finish(errors.New(r.Failure))
		}
	}
	source, err := publication.OpenSourceProtectionRootPrivateV2(ctx, o.Workspace)
	if err != nil {
		return finish(err)
	}
	closes = append(closes, source.Close)
	var inputs []prep.InputPrivateV2
	var protected []*publication.ObservedRoot
	openPair := func(label string) (out [2]*publication.ContentRetentionRoot, e error) {
		for i := range out {
			path := filepath.Join(attempt, fmt.Sprintf("%s-%d", label, i))
			if e = os.Mkdir(path, 0700); e != nil {
				return
			}
			out[i], e = publication.OpenContentRetentionRoot(ctx, path)
			if e != nil {
				return
			}
			closes = append(closes, out[i].Close)
		}
		return
	}
	// Existing lifecycle owner starts, readies, stops and joins the one runtime.
	// The equality guard is irreversible for this generation: this checkpoint
	// admits ONE original origin only. Overflow is retained, never restarted
	// under a fresh Manager or presented as a complete application inventory.
	var pipelineErr error
	cfg.runner.observeStop = func(s session.LifecycleResult) {
		r.StopCalls++
		r.StopFailure = string(s.Failure)
		r.StopIntentID = s.IntentID
	}
	r.SessionCode = deps.runSession(cfg.runner, func(live context.Context, runtime *initializedAcquisitionRuntime) int {
		r.Session = censusacquisition.SessionIdentity{SessionID: runtime.SessionID(), Generation: runtime.Generation()}
		for ordinal, indices := range groups {
			if ctx.Err() != nil {
				pipelineErr = ctx.Err()
				return 1
			}
			obs := &r.Origins[ordinal]
			frozen := frozenScopes[ordinal]
			obs.Status = "DISCOVERY_ATTEMPTED"
			for _, i := range indices {
				r.Files[i].Disposition = "DISCOVERY_ATTEMPTED"
			}
			var e error
			var origin pilot.EnumerationOriginPrivateV2
			var captures []pilot.OpaqueSuccessfulV2Capture
			e = runtime.manager.WithReadySessionLease(live, sessionruntime.SessionLeaseRequest{SessionID: r.Session.SessionID, Generation: r.Session.Generation, Workspace: o.Workspace}, func(leased context.Context) error {
				bounded, stop := context.WithDeadline(leased, mustCensusInventoryDeadlinePrivate(ctx))
				defer stop()
				var discovery pilot.ScopeDiscoveryReportPrivateV2
				var de error
				origin, discovery, de = pilot.DiscoverScopePrivateV2(bounded, frozen, runtime.manager, r.Session)
				obs.Discovery = discovery.Bytes()
				if de != nil {
					return de
				}
				obs.Batches = make([]censusInventoryBatchPrivate, origin.BatchCount())
				for i := range obs.Batches {
					obs.Batches[i] = censusInventoryBatchPrivate{Ordinal: i, Status: "UNPROCESSED"}
				}
				for i := range obs.Batches {
					b, e := origin.Batch(i)
					if e != nil {
						return e
					}
					obs.Batches[i].Status = "ACQUISITION_ATTEMPTED"
					v, e := pilot.AcquireScopeBatchPrivateV2(bounded, runtime.manager, origin, b)
					obs.Batches[i].ArtifactSHA256 = v.ArtifactDigest()
					obs.Batches[i].ArtifactBytes = v.ArtifactByteLength()
					if e != nil {
						obs.Batches[i].Status = "ACQUISITION_FAILED"
						return e
					}
					obs.Batches[i].Status = "ACQUIRED"
					captures = append(captures, v)
				}
				return nil
			})
			if e != nil {
				obs.Status = "DISCOVERY_OR_ACQUISITION_FAILED"
				if errors.Is(e, pilot.ErrScopeDiscoveryEmptyPrivateV2) {
					obs.Status = "SUCCESSFUL_EMPTY"
					for _, i := range indices {
						r.Files[i].Disposition = obs.Status
					}
					continue
				}
				for _, i := range indices {
					r.Files[i].Disposition = obs.Status
				}
				pipelineErr = e
				return 1
			}
			pair, e := openPair(fmt.Sprintf("producer-%d", ordinal))
			if e != nil {
				pipelineErr = e
				return 1
			}
			roots, e := pilot.NewProducerPacketRootsPrivateV2(ctx, pair, source, nil)
			if e != nil {
				pipelineErr = e
				return 1
			}
			retained, e := pilot.RetainCompleteScopePrivateV2(ctx, frozen, origin, captures, roots)
			obs.ProducerCopies = retained.CopyReceipts()
			if e != nil {
				obs.Status = "PRODUCER_RETENTION_FAILED"
				pipelineErr = e
				return 1
			}
			qualified, e := pilot.QualifyProducerPacketPrivateV2(ctx, retained.VerifiedPair())
			if e != nil {
				pipelineErr = e
				return 1
			}
			proof, e := pilot.DeriveProducerPacketPrivateV2(ctx, qualified)
			if e != nil {
				pipelineErr = e
				return 1
			}
			view, e := pilot.ReadDerivedProducerPacketPrivateV2(ctx, proof)
			if e != nil {
				pipelineErr = e
				return 1
			}
			for i := range view.Batches {
				projection, f := derived.ProjectPrivateV2(ctx, proof, i)
				if f != nil {
					pipelineErr = f
					return 1
				}
				assessment, _, e := derived.InstabilityComputePrivateV2(ctx, projection, c.Seeds)
				if e != nil {
					pipelineErr = e
					return 1
				}
				inputs = append(inputs, prep.InputPrivateV2{Projection2: projection, Assessment2: assessment})
			}
			// Required protected roots are the exact producer pair, never inferred from
			// equal graph bytes or shared session identifiers.
			for i := 0; i < 2; i++ {
				v, e := publication.OpenObservedRoot(ctx, filepath.Join(attempt, fmt.Sprintf("producer-%d-%d", ordinal, i)))
				if e != nil {
					pipelineErr = e
					return 1
				}
				protected = append(protected, v)
				closes = append(closes, v.Close)
			}
			obs.Status = "COMPUTED_PRIVATE_PER_BATCH"
			for _, i := range indices {
				r.Files[i].Disposition = obs.Status
			}
		}
		return 0
	})
	r.ShutdownCalls++
	joinErr := shutdown()
	r.ShutdownJoined = joinErr == nil
	terminal, found := cfg.runner.manager.Operation(r.StopIntentID)
	cleanup, observed := terminal.CleanupObservationPrivateV2()
	r.CleanupVerified = found && terminal.State == sessionruntime.OperationComplete && terminal.Failure == "" && terminal.SessionID == r.Session.SessionID && terminal.Generation == r.Session.Generation && observed && cleanup.ResourceClose == managedprocess.ResourcesClosed && cleanup.Reap == managedprocess.ReapComplete && (cleanup.Death == managedprocess.DeathExited || cleanup.Death == managedprocess.DeathSignaled) && (cleanup.RetiredTransportReused || (cleanup.ShutdownWrite == sessionruntime.CleanupSucceededPrivateV2 && cleanup.ShutdownRead == sessionruntime.ShutdownMatchedPrivateV2 && (cleanup.ExitWrite == sessionruntime.CleanupSucceededPrivateV2 || cleanup.Death == managedprocess.DeathExited)))
	if r.SessionCode != 0 || pipelineErr != nil || r.StopCalls != 1 || r.StopFailure != "" || !r.ShutdownJoined || !r.CleanupVerified {
		r.Failure = "ACQUISITION_OR_CLEANUP_FAILED"
		return finish(errors.Join(pipelineErr, errors.New(r.Failure)))
	}
	if len(inputs) == 0 {
		r.Status = "NO_PREPARABLE_TARGETS"
		return finish(nil)
	}
	prepared, e := prep.PreparePrivateV2(ctx, inputs, c.Preparation)
	if e != nil {
		return finish(e)
	}
	r.Coverage, r.Index = prepared.Coverage, prepared.Index
	h, e := prep.NewPostPreparationPrivateV2(ctx, inputs, c.Preparation, c.Post)
	if e != nil {
		return finish(e)
	}
	raw, e := h.PreparedBytes()
	if e != nil {
		return finish(e)
	}
	r.PreparationSHA256 = pilot.Digest(raw)
	r.PreparationBytes = len(raw)
	outputs, e := openPair("inventory")
	if e != nil {
		return finish(e)
	}
	var outcome prep.PostPublicationOutcomePrivateV2
	if c.Automation == nil {
		outcome, e = prep.RetainPostPreparationPrivateV2(ctx, h, nil, nil, outputs, protected)
		r.Status = "PENDING_INTERPRETATION_PRIVATE_V2"
	} else {
		binding := struct {
			Schema, Profile, Protocol                             string
			Authority                                             int
			Accepted, Qualified                                   bool
			FeatureIdentity, SourceCompleteness, AbsenceInference string
			ScopeSHA256, PreparationSHA256                        string
			Automation                                            prep.AutomaticHandoffConfigPrivateV2
			Roots                                                 []string
		}{"private-census-inventory-host-export.v1", censusInventoryProfilePrivate, prep.AutomaticHandoffProtocolPrivateV2, 0, false, false, "UNRESOLVED", "UNKNOWN", "PROHIBITED", r.ScopeSHA256, r.PreparationSHA256, *c.Automation, []string{o.Workspace, o.PublicationRoot, o.PrivateInventoryConfigPath, cfg.runner.start.Process.Path}}
		if e := writeCensusInventoryFilePrivate(r.HostExportBindingPath, binding); e != nil {
			return finish(e)
		}
		bindingRaw, _ := json.Marshal(binding)
		r.HostExportBindingSHA256 = pilot.Digest(bindingRaw)
		interpreter, reviewer, bindingErr := deps.automatic(*c.Automation, r.ScopeSHA256, h)
		if bindingErr != nil {
			return finish(bindingErr)
		}
		result, automaticErr := prep.AutomatePrivateV2(ctx, h, c.Automation.Limits, interpreter, reviewer)
		inventory := result.Inventory()
		r.Inventory = &inventory
		if automaticErr == nil {
			outcome, automaticErr = prep.RetainAutomaticPrivateV2(ctx, h, result, outputs, protected)
			r.Outcome = &outcome
		}
		e = errors.Join(automaticErr, prep.FinishAutomaticHandoffPrivateV2(interpreter, r.Outcome, automaticErr))
		r.Status = inventory.Status
	}
	r.Outcome = &outcome
	if e != nil {
		return finish(e)
	}
	for _, root := range outputs {
		b, e := root.ReadPostPreparationPrivateV2(ctx, outcome.EnvelopeSHA256, int64(outcome.ByteLength))
		if e != nil || !bytes.Equal(b, outcome.EnvelopeBytes()) {
			return finish(errors.Join(e, errors.New("private inventory exact pair mismatch")))
		}
	}
	if r.Inventory != nil && r.ApplicationCoverage == "PARTIAL_RESOURCE_GAPS" {
		r.Status = "PROVISIONAL_WITH_APPLICATION_GAPS"
	}
	return finish(nil)
}

// Each page has at most 128 complete file entries. No source bodies occur in
// this ledger. Identity is over exact bytes retained before their use in scope.
func retainCensusInventoryPagesPrivate(directory, prefix string, rows []censusInventoryFilePrivate) ([]censusInventoryPagePrivate, error) {
	pages := []censusInventoryPagePrivate{}
	for start := 0; start < len(rows); start += 128 {
		end := start + 128
		if end > len(rows) {
			end = len(rows)
		}
		raw, err := json.Marshal(rows[start:end])
		if err != nil {
			return pages, err
		}
		name := fmt.Sprintf("%s-%06d.json", prefix, len(pages))
		if err = writeCensusInventoryFilePrivate(filepath.Join(directory, name), json.RawMessage(raw)); err != nil {
			return pages, err
		}
		// Reopen this exact immutable page; a write return alone is not readback.
		got, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || !bytes.Equal(got, raw) {
			return pages, errors.Join(err, errors.New("private denominator page readback mismatch"))
		}
		pages = append(pages, censusInventoryPagePrivate{Selector: name, SHA256: pilot.Digest(got), ByteLength: len(got), Entries: end - start})
	}
	return pages, nil
}

// Read-only, no-follow identity/ancestry observation before publication. Fail
// closed on unavailable roots; this is not a same-user namespace mutation lease.
func censusInventoryRelatedPathsPrivate(a, b string) (related bool) {
	ctx := context.Background()
	x, err := publication.OpenSourceProtectionRootPrivateV2(ctx, a)
	if err != nil {
		return true
	}
	defer func() { err := x.Close(); related = related || err != nil }()
	y, err := publication.OpenSourceProtectionRootPrivateV2(ctx, b)
	if err != nil {
		return true
	}
	defer func() { err := y.Close(); related = related || err != nil }()
	return publication.CheckSourceProtectionSeparationPrivateV2(ctx, x, y) != nil
}

func mustCensusInventoryDeadlinePrivate(ctx context.Context) time.Time {
	d, _ := ctx.Deadline()
	return d
}
