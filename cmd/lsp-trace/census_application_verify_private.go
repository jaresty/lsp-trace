//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func censusInventoryScopeBytesPrivate(o censusCLIOptions, c censusInventoryConfigPrivate, pages []censusInventoryPagePrivate, manifests []json.RawMessage, groups [][]int, application *censusApplicationScopePrivate) []byte {
	c.Automation = nil
	c.CohortAutomation = nil
	var partitions [][]int
	if c.MaxOrigins > 1 {
		partitions = groups
	}
	raw, _ := json.Marshal(struct {
		Workspace                           string
		Config                              censusInventoryConfigPrivate
		Sources, Includes, Excludes, Pruned []string
		Files                               []censusInventoryPagePrivate
		Manifests                           []json.RawMessage
		Partitions                          [][]int                        `json:",omitempty"`
		Application                         *censusApplicationScopePrivate `json:",omitempty"`
	}{o.Workspace, c, o.Sources, o.Includes, o.Excludes, o.PrunedDirectories, pages, manifests, partitions, application})
	return raw
}

// Exact hash/length plus physically distinct opened artifact identities. No
// graph concatenation, envelope rewriting, or restored retention authority.
func (s *censusApplicationProgressPrivate) pair(attempt, label, digest string, n uint64) ([]byte, error) {
	a := s.config.Application
	if n < 1 || n > uint64(a.MaxArtifactBytes) {
		return nil, errors.New("application constituent byte bound")
	}
	var prior []byte
	var info os.FileInfo
	for i := 0; i < 2; i++ {
		root, err := openCensusApplicationStorePrivate(s.store.ctx, filepath.Join(attempt, fmt.Sprintf("%s-%d", label, i)), int(s.store.remaining))
		if err != nil {
			return nil, err
		}
		names, err := root.names(1)
		if err != nil || len(names) != 1 || names[0] != digest {
			_ = root.close()
			return nil, errors.Join(err, errors.New("application missing/extra pair constituent"))
		}
		raw, err := root.read(digest, a.MaxArtifactBytes)
		opened, e := root.root.Lstat(digest)
		err = errors.Join(err, e, root.close())
		s.store.remaining = root.remaining
		if err != nil || uint64(len(raw)) != n || pilot.Digest(raw) != digest {
			return nil, errors.Join(err, errors.New("application constituent digest/length mismatch"))
		}
		if i > 0 && (!bytes.Equal(prior, raw) || os.SameFile(info, opened)) {
			return nil, errors.New("application pair differs or physically aliased")
		}
		prior, info = raw, opened
	}
	return prior, s.store.check()
}
func (s *censusApplicationProgressPrivate) verify(segment int) (v censusApplicationVerifiedSegmentPrivate, err error) {
	read := func(kind string, value any) ([]byte, error) {
		raw, e := s.store.read(censusApplicationLeafPrivate(segment, kind), 64<<10)
		if e == nil {
			e = decodeCensusApplicationPrivate(raw, value)
		}
		return raw, e
	}
	var host censusApplicationHostObservationPrivate
	hostRaw, err := read("host", &host)
	if err != nil {
		return v, err
	}
	if host.Schema != "lsp-trace.census-application-host-process.private.v1" || host.Trust != censusApplicationHostTrustPrivate || host.FullPlanSHA256 != s.id || host.Segment != segment || host.ProcessID < 1 || host.ExitCode != 0 || !host.Joined || host.Stopped {
		return v, errors.New("application host completion binding/refusal")
	}
	var claim censusApplicationClaimPrivate
	claimRaw, err := read("claim", &claim)
	if err != nil {
		return v, err
	}
	if claim.Schema != "lsp-trace.census-application-claim.private.v1" || claim.FullPlanSHA256 != s.id || claim.Segment != segment || claim.ProcessID != host.ProcessID || claim.AttemptDirectory != host.AttemptDirectory || claim.ScopeSHA256 != host.ScopeSHA256 || pilot.Digest(claimRaw) != host.ClaimSHA256 {
		return v, errors.New("application foreign/spent claim")
	}
	attempt := claim.AttemptDirectory
	if filepath.Clean(attempt) != attempt || filepath.Dir(attempt) != s.options.PublicationRoot || !strings.HasPrefix(filepath.Base(attempt), "private-census-inventory-") {
		return v, errors.New("application foreign attempt path")
	}
	var result censusApplicationResultPrivate
	resultRaw, err := read("result", &result)
	if err != nil {
		return v, err
	}
	if result.Schema != "lsp-trace.census-application-result.private.v1" || result.ClaimSHA256 != host.ClaimSHA256 || result.ReportSHA256 != host.ReportSHA256 || pilot.Digest(resultRaw) != host.ResultSHA256 || result.Failure != "" || result.Status == "RETAINED_FAILURE" {
		return v, errors.New("application failed/result binding")
	}
	reportRaw, err := s.store.external(filepath.Join(attempt, "report.json"), s.config.Application.MaxArtifactBytes)
	if err != nil {
		return v, err
	}
	var r censusInventoryReportPrivate
	if decodeCensusApplicationPrivate(reportRaw, &r) != nil || r.Schema != "lsp-trace.census-application-inventory.private.v1" || r.Profile != censusInventoryProfilePrivate || r.InputFamily != "PRIVATE_AST" || pilot.Digest(reportRaw) != host.ReportSHA256 || r.AttemptDirectory != attempt || r.ScopeSHA256 != claim.ScopeSHA256 || r.Status != result.Status || r.Failure != "" || r.Authority != 0 || r.Accepted || r.Qualified || r.ApplicationBinding == nil || r.ApplicationBinding.FullPlanSHA256 != s.id || r.ApplicationBinding.Segment != segment || r.ApplicationBinding.ProcessID != host.ProcessID || r.ApplicationBinding.ClaimSHA256 != host.ClaimSHA256 {
		return v, errors.New("application report binding")
	}
	terminalSegment := censusTerminalEnabledPrivate(s.config) && r.Status == censusTerminalSegmentStatusPrivate
	if !terminalSegment && (!r.Configuration.hasAutomation() || r.Outcome == nil || r.Inventory == nil) {
		return v, errors.New("application native delivery missing")
	}
	if terminalSegment && (host.NativeSHA256 != "" || host.NativeBytes != 0 || r.Outcome != nil || r.Inventory != nil || r.PreparationSHA256 != "" || r.PreparationBytes != 0 || len(r.Coverage) != 0 || len(r.Index) != 0 || r.HostExportBindingPath != "" || r.HostExportBindingSHA256 != "") {
		return v, errors.New("application terminal must not claim native delivery")
	}
	if r.FeatureIdentity != "UNRESOLVED" || r.SourceCompleteness != "UNKNOWN" || r.AbsenceInference != "PROHIBITED" || !reflect.DeepEqual(r.Sources, s.options.Sources) || !reflect.DeepEqual(r.Includes, s.options.Includes) || !reflect.DeepEqual(r.Excludes, s.options.Excludes) || !reflect.DeepEqual(r.PrunedDirectories, s.options.PrunedDirectories) {
		return v, errors.New("application report ceiling/source controls mismatch")
	}
	config, _ := json.Marshal(r.Configuration)
	if pilot.Digest(config) != claim.ConfigurationSHA256 || r.ApplicationBinding.ConfigurationSHA256 != claim.ConfigurationSHA256 {
		return v, errors.New("application configuration receipt mismatch")
	}
	approved := r.Configuration
	approved.Automation = nil
	approved.CohortAutomation = nil
	expectedConfig, _ := json.Marshal(s.plan.Configuration)
	actualConfig, _ := json.Marshal(approved)
	if !bytes.Equal(expectedConfig, actualConfig) || r.Configuration.validateAutomation() != nil || (r.Configuration.hasAutomation() && r.Configuration.automationScope() != r.ScopeSHA256) {
		return v, errors.New("application approval/configuration mismatch")
	}
	groups := s.plan.groups(segment)
	manifests := make([]json.RawMessage, len(groups))
	for j, origin := range s.plan.Segments[segment] {
		manifests[j] = s.plan.Origins[origin].Manifest
	}
	expectedScope := censusInventoryScopeBytesPrivate(s.options, s.plan.Configuration, s.pages, manifests, groups, &censusApplicationScopePrivate{s.id, segment})
	if pilot.Digest(expectedScope) != r.ScopeSHA256 {
		return v, errors.New("application scope not the planned segment")
	}
	scope, err := s.store.external(filepath.Join(attempt, "scope.json"), s.config.Application.MaxArtifactBytes)
	if err != nil || !bytes.Equal(scope, expectedScope) {
		return v, errors.Join(err, errors.New("application retained scope mismatch"))
	}
	if len(r.Origins) != len(groups) || !r.CleanupVerified || !r.ShutdownJoined {
		return v, errors.New("application original-owner cleanup missing")
	}
	type producerObservation struct {
		packet pilot.ProducerPacketInspectionPrivateV2
		origin censusInventoryOriginPrivate
	}
	producers := map[string]producerObservation{}
	terminalOrigins := 0
	for j, origin := range r.Origins {
		if terminalSegment && !censusTerminalCleanupPrivate(origin) {
			return v, errors.New("application terminal concrete cleanup missing")
		}
		if origin.Ordinal != j || !reflect.DeepEqual(origin.Files, groups[j]) || origin.ScopeSHA256 != pilot.Digest(manifests[j]) || origin.SessionCode != 0 || origin.StopCalls != 1 || origin.ShutdownCalls != 1 || !origin.ShutdownJoined || !origin.CleanupVerified || origin.Cleanup == nil || origin.StopFailure != "" {
			return v, errors.New("application original-origin lineage mismatch")
		}
		var discovery struct {
			Session struct {
				SessionID  string
				Generation uint64
			}
			Accounting         struct{ FileDenominator, SymbolDenominator int }
			Targets            []json.RawMessage
			EnumerationSymbols []struct{ Disposition string }
			Complete           bool
		}
		if json.Unmarshal(origin.Discovery, &discovery) != nil || discovery.Session.SessionID != origin.Session.SessionID || discovery.Session.Generation != origin.Session.Generation || discovery.Accounting.FileDenominator != len(origin.Files) || discovery.Accounting.SymbolDenominator != len(discovery.EnumerationSymbols) {
			return v, errors.New("application original discovery accounting mismatch")
		}
		accounting := censusApplicationOriginAccountingPrivate{PlanOrigin: s.plan.Segments[segment][j], DiscoverySHA256: pilot.Digest(origin.Discovery), FileEntries: discovery.Accounting.FileDenominator, SymbolEntries: discovery.Accounting.SymbolDenominator, CallableTargets: len(discovery.Targets), DiscoveryComplete: discovery.Complete}
		for _, symbol := range discovery.EnumerationSymbols {
			if symbol.Disposition == "non-callable" {
				accounting.NonCallableSymbols++
			}
		}
		if terminalSegment {
			accounting.Reason = origin.Status
			accounting.Disposition = "TERMINAL_ACCOUNTED_GAP"
		} else if censusTerminalEnabledPrivate(s.config) {
			accounting.Disposition = "COMPLETED_NATIVE"
		}
		v.Origins = append(v.Origins, accounting)
		if censusTerminalOriginStatusPrivate(origin.Status) {
			if !terminalSegment {
				return v, errors.New("application terminal in native segment")
			}
			if e := s.verifyTerminalOriginPrivate(attempt, j, origin, manifests[j]); e != nil {
				return v, e
			}
			terminalOrigins++
			continue
		}
		if origin.Terminal != nil {
			return v, errors.New("application unexpected terminal owner record")
		}
		if origin.Status == "SUCCESSFUL_EMPTY" {
			if len(discovery.Targets) != 0 || !discovery.Complete {
				return v, errors.New("application original discovery contradicts empty origin")
			}
			continue
		}
		if len(discovery.Targets) == 0 {
			return v, errors.New("application original discovery contradicts computed origin")
		}
		if (!terminalSegment && origin.Status != "COMPUTED_PRIVATE_PER_BATCH") || (terminalSegment && origin.Status != censusComputedSegmentGapPrivate) {
			return v, errors.New("application origin not completed")
		}
		copy := origin.ProducerCopies[0].Receipt
		other := origin.ProducerCopies[1].Receipt
		if copy == nil || other == nil || copy.Digest != other.Digest || copy.FinalSelector != copy.Digest || other.FinalSelector != copy.Digest || copy.ByteLength != other.ByteLength || copy.VerificationStatus != "VERIFIED" || other.VerificationStatus != "VERIFIED" {
			return v, errors.New("application producer receipt mismatch")
		}
		packet, err := s.pair(attempt, fmt.Sprintf("producer-%d", j), copy.Digest, copy.ByteLength)
		if err != nil {
			return v, err
		}
		// The complete pure inspector validates all detached source, role, batch,
		// seed, Native, runtime and enumeration constituents before ANY advance.
		// It cannot restore an owner or mint custody authority.
		producer, inspectErr := pilot.PureParseProducerPacketPrivateV2(packet)
		if inspectErr != nil || !bytes.Equal(producer.FrozenManifest, manifests[j]) {
			return v, errors.New("application producer source lineage mismatch")
		}
		var enumeration struct {
			Enumeration json.RawMessage `json:"actual_enumeration"`
			ScopeSHA256 string          `json:"scope_sha256"`
		}
		if json.Unmarshal(producer.EnumerationOrigin, &enumeration) != nil || !bytes.Equal(enumeration.Enumeration, origin.Discovery) || enumeration.ScopeSHA256 != origin.ScopeSHA256 {
			return v, errors.New("application producer original discovery mismatch")
		}
		policy, _ := json.Marshal(s.plan.Configuration.Policy)
		if !bytes.Equal(producer.EffectivePolicy, policy) || len(producer.Batches) != len(origin.Batches) {
			return v, errors.New("application producer policy/report batch mismatch")
		}
		for i, batch := range producer.Batches {
			observed := origin.Batches[i]
			if observed.Ordinal != i || observed.Status != "ACQUIRED" || observed.ArtifactSHA256 != pilot.Digest(batch.SourceArtifact) || observed.ArtifactBytes != uint64(len(batch.SourceArtifact)) {
				return v, errors.New("application producer original batch/report mismatch")
			}
		}
		producers[copy.Digest] = producerObservation{producer, origin}
	}
	if terminalSegment {
		if terminalOrigins == 0 {
			return v, errors.New("application segment gap without terminal origin")
		}
		v.Segment, v.AttemptDirectory, v.ScopeSHA256, v.ReportSHA256, v.HostObservationSHA256 = segment, attempt, r.ScopeSHA256, host.ReportSHA256, pilot.Digest(hostRaw)
		v.Disposition = "TERMINAL_ACCOUNTED_GAP"
		if err := s.verifyTerminalPages(attempt, r); err != nil {
			return v, err
		}
		return v, s.store.check()
	}
	expectedCeilings := prep.PrivateCeilings{FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", OrdinaryNative: "NOT_ATTEMPTED", Describe: "NOT_ATTEMPTED", Workflow: "NOT_ATTEMPTED"}
	if r.Outcome.PrivateCeilings != expectedCeilings || r.Outcome.Absence != "PROHIBITED" || r.Outcome.Profile != prep.PostPublicationProfilePrivateV2 || r.Outcome.PolicySHA256 != pilot.Digest([]byte(prep.PostPublicationPolicyPrivateV2)) || r.Outcome.PreparationSHA256 != r.PreparationSHA256 || r.Outcome.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" || r.Outcome.EnvelopeSHA256 != host.NativeSHA256 || r.Outcome.ByteLength != host.NativeBytes {
		return v, errors.New("application native outcome mismatch")
	}
	for _, copy := range r.Outcome.Copies {
		if copy.Receipt == nil || copy.Receipt.Digest != host.NativeSHA256 || copy.Receipt.FinalSelector != host.NativeSHA256 || copy.Receipt.ByteLength != host.NativeBytes || copy.Receipt.VerificationStatus != "VERIFIED" {
			return v, errors.New("application native copy receipt mismatch")
		}
	}
	raw, err := s.pair(attempt, "inventory", host.NativeSHA256, host.NativeBytes)
	if err != nil {
		return v, err
	}
	prepared, inventory, observationErr := prep.ValidateAutomaticPublicationObservationPrivateV2(s.store.ctx, raw, s.plan.Configuration.Preparation, s.plan.Configuration.Post)
	if observationErr != nil {
		return v, errors.Join(errors.New("application native observation refused"), observationErr)
	}
	preparedRaw := prepared.CanonicalBytes()
	if pilot.Digest(preparedRaw) != r.PreparationSHA256 || len(preparedRaw) != r.PreparationBytes || !reflect.DeepEqual(prepared.Coverage, r.Coverage) || !reflect.DeepEqual(prepared.Index, r.Index) {
		return v, errors.New("application native preparation/report mismatch")
	}
	left, _ := json.Marshal(inventory)
	right, _ := json.Marshal(r.Inventory)
	if !bytes.Equal(left, right) {
		return v, errors.New("application native inventory/report mismatch")
	}
	// Compare detached source references with already-read original producer
	// bytes, not with mutually self-consistent native/report hashes alone.
	for _, c := range prepared.Coverage {
		if c.PrivateV2 == nil {
			return v, errors.New("application native source family missing")
		}
		source := c.PrivateV2.Source
		p, ok := producers[source.PacketSHA256]
		if !ok || source.BatchOrdinal < 0 || source.BatchOrdinal >= len(p.packet.Batches) || c.DiscoverySHA256 != pilot.Digest(p.origin.Discovery) || c.PrivateV2.OriginID != pilot.Digest(p.packet.EnumerationOrigin) || source.SessionID != p.origin.Session.SessionID || source.Generation != p.origin.Session.Generation || source.RuntimeMetadataSHA256 != pilot.Digest(p.packet.RuntimeMetadata) {
			return v, errors.New("application native source/original discovery correspondence")
		}
		batch := p.packet.Batches[source.BatchOrdinal]
		if batch.Ordinal != source.BatchOrdinal || source.CanonicalSeedsSHA256 != pilot.Digest(batch.Seeds) || source.CaptureOriginSHA256 != pilot.Digest(batch.Origin) || source.SourceArtifactSHA256 != pilot.Digest(batch.SourceArtifact) {
			return v, errors.New("application native source/original batch correspondence")
		}
		var original struct {
			Policy   json.RawMessage `json:"source_policy"`
			Evidence struct {
				GraphBytes []byte `json:"graph_bytes"`
			} `json:"evidence"`
		}
		if json.Unmarshal(batch.SourceArtifact, &original) != nil || source.SourcePolicySHA256 != pilot.Digest(original.Policy) || source.OriginalNativeV3SHA256 != pilot.Digest(original.Evidence.GraphBytes) {
			return v, errors.New("application native source/original policy correspondence")
		}
	}
	byInput := map[string]string{}
	seen := map[string]bool{}
	for _, cov := range r.Coverage {
		if cov.PrivateV2 == nil || cov.PrivateV2.OriginID == "" {
			return v, errors.New("application missing original origin identity")
		}
		id := cov.PrivateV2.OriginID
		byInput[cov.InputSHA256] = id
		if !seen[id] {
			v.OriginIDs = append(v.OriginIDs, id)
			seen[id] = true
		}
	}
	for i, candidate := range r.Inventory.Candidates {
		origin := byInput[candidate.InputSHA256]
		if origin == "" {
			return v, errors.New("application candidate missing original namespace")
		}
		v.Candidates = append(v.Candidates, censusApplicationCandidateRefPrivate{fmt.Sprintf("%s/segment:%d/origin:%s/candidate:%d", s.id, segment, origin, i), origin, candidate.ID, candidate.Label, i})
	}
	v.Segment, v.AttemptDirectory, v.ScopeSHA256, v.ReportSHA256, v.HostObservationSHA256, v.NativeSHA256, v.NativeBytes = segment, attempt, r.ScopeSHA256, host.ReportSHA256, pilot.Digest(hostRaw), host.NativeSHA256, host.NativeBytes
	v.Coverage = r.Coverage
	if censusTerminalEnabledPrivate(s.config) {
		v.Disposition = "COMPLETED_NATIVE"
	}
	// Complete, hashed terminal denominator pages must still exist, even when the
	// report omitted its large inline convenience array.
	if err := s.verifyTerminalPages(attempt, r); err != nil {
		return v, err
	}
	return v, s.store.check()
}

// The expected denominator is derived from the original full plan and only the
// selected, verified origins. Future eligible work is not silently completed.
func (s *censusApplicationProgressPrivate) verifyTerminalPages(attempt string, r censusInventoryReportPrivate) error {
	fail := func() error { return errors.New("application terminal denominator mismatch") }
	expected := append([]censusInventoryFilePrivate(nil), s.rows...)
	for _, origin := range r.Origins {
		if origin.Status != "COMPUTED_PRIVATE_PER_BATCH" && origin.Status != "SUCCESSFUL_EMPTY" && !(censusTerminalEnabledPrivate(s.config) && r.Status == censusTerminalSegmentStatusPrivate && (censusTerminalOriginStatusPrivate(origin.Status) || origin.Status == censusComputedSegmentGapPrivate)) {
			return fail()
		}
		for _, index := range origin.Files {
			if index < 0 || index >= len(expected) {
				return fail()
			}
			expected[index].Disposition, expected[index].Origin = origin.Status, origin.Ordinal
		}
	}
	if r.FileDenominator != len(expected) || len(r.FilePages) != (len(expected)+127)/128 || (len(expected) > 0 && len(r.FilePages) == 0) {
		return fail()
	}
	// Validate every selector before opening even the held attempt descriptor.
	for i, page := range r.FilePages {
		if page.Selector != fmt.Sprintf("terminal-files-%06d.json", i) {
			return fail()
		}
	}
	root, err := openCensusApplicationStorePrivate(s.store.ctx, attempt, int(s.store.remaining))
	if err != nil {
		return errors.Join(err, fail())
	}
	defer func() { _ = root.close() }()
	for i, page := range r.FilePages {
		end := (i + 1) * 128
		if end > len(expected) {
			end = len(expected)
		}
		want, e := json.Marshal(expected[i*128 : end])
		if e != nil {
			return errors.Join(e, fail())
		}
		if page.Entries != end-i*128 || page.ByteLength != len(want) || page.SHA256 != pilot.Digest(want) {
			return fail()
		}
		got, e := root.read(page.Selector, s.config.Application.MaxArtifactBytes)
		s.store.remaining = root.remaining
		if e != nil || !bytes.Equal(got, want) {
			return errors.Join(e, fail())
		}
	}
	if r.Files != nil && !reflect.DeepEqual(r.Files, expected) {
		return fail()
	}
	return s.store.check()
}
