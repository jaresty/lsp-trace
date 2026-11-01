//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// Private, unadvertised, deliberately distinct from old inventory config/wire.
// RuntimeGrantSHA256 is an explicit current caller grant, NOT authentication.
// Neither setup nor preview creates original claims or starts any provider.
type hostProductRequestV3 struct {
	Schema                                                             string
	Configuration                                                      product.Config
	CensusArguments                                                    []string
	Policy                                                             pilot.ScopeEffectivePolicyPrivateV2
	RevisionAssertion, AuxiliaryRoot, ServerSHA256, RuntimeGrantSHA256 string
	ApplicationAllowance, BootstrapAllowance                           pilot.PageCostPrivateV3
	PayloadBytes, ComputationUnitBytes                                 uint64
	OperationTimeoutMS, ExchangeTimeoutMS                              int
	FileDenominator                                                    int
}

func hostProductRequestDigestV3(r hostProductRequestV3) string {
	r.RuntimeGrantSHA256 = ""
	b, _ := json.Marshal(r)
	return pilot.Digest(b)
}
func hostProductProducerDigestV3(r hostProductRequestV3) string {
	b, _ := json.Marshal(struct {
		Args             []string
		Policy           pilot.ScopeEffectivePolicyPrivateV2
		Revision, Server string
	}{r.CensusArguments, r.Policy, r.RevisionAssertion, r.ServerSHA256})
	return pilot.Digest(b)
}
func readHostProductRequestV3(path string) (hostProductRequestV3, error) {
	var r hostProductRequestV3
	raw, e := readHostProductRequestRawV3(path)
	if e != nil {
		return r, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil {
		return r, product.ErrInvalid
	}
	b, _ := json.Marshal(r)
	if !bytes.Equal(raw, b) || r.Schema != "private-existing-host-request.v3.1" || product.ValidateConfig(r.Configuration) != nil || r.Configuration.ProducerConfigSHA256 != hostProductProducerDigestV3(r) || r.OperationTimeoutMS < 1 || r.OperationTimeoutMS > 3600000 || r.ExchangeTimeoutMS < 1 || r.ExchangeTimeoutMS > r.OperationTimeoutMS || r.FileDenominator < 1 || r.PayloadBytes < 1 || r.PayloadBytes > 1<<20 || r.ComputationUnitBytes < 1 || r.ComputationUnitBytes > 16<<20 || r.ApplicationAllowance.LogicalBytes == 0 || r.ApplicationAllowance.ReadHashBytes == 0 || r.ApplicationAllowance.WriteBytes == 0 || r.ApplicationAllowance.Work == 0 || r.BootstrapAllowance.LogicalBytes == 0 || r.BootstrapAllowance.ReadHashBytes == 0 || r.BootstrapAllowance.Work == 0 {
		return r, product.ErrInvalid
	}
	o, e := parseCensusCLIOptions(r.CensusArguments)
	if e != nil || !o.PrivateEnumerationProjection || o.Profile != "" || o.ConfigPath != "" || o.Server == "" || !filepath.IsAbs(o.Server) || o.PrivateInventoryConfigPath != "" || o.PrivateProviderProbePath != "" || o.PrivateAcquisitionDiagnosticPath != "" || o.PrivateControlFlowDiagnosticPath != "" || o.Catalog || o.EnumerationPreflight || o.Resume != "" || hostProductLexicalRelatedV3(o.Workspace, r.AuxiliaryRoot) || hostProductLexicalRelatedV3(o.Workspace, r.Configuration.Store) || hostProductLexicalRelatedV3(r.AuxiliaryRoot, r.Configuration.Store) {
		return r, product.ErrInvalid
	}
	if r.Policy.Planning.MaxBatchTargets != o.MaxBatchTargets || r.Policy.MaxNodes != o.MaxNodes || r.Policy.TimeoutMS != int(o.Timeout.Milliseconds()) || r.Policy.RequestTimeoutMS != int(o.RequestTimeout.Milliseconds()) || r.Policy.Planning.DownDepth != o.DownDepth || r.Policy.Planning.UpDepth != o.UpDepth {
		return r, product.ErrInvalid
	}
	return r, nil
}
func hostProductMailboxV3(r hostProductRequestV3) product.Mailbox {
	return product.Mailbox{Directory: filepath.Join(r.AuxiliaryRoot, fmt.Sprintf("mailbox-%08d", r.Configuration.AllowanceRevision)), MaxRequestBytes: r.Configuration.MaxRequestBytes, MaxResponseBytes: r.Configuration.MaxResponseBytes, ExchangeTimeout: time.Duration(r.ExchangeTimeoutMS) * time.Millisecond}
}
func setupHostProductV3(r hostProductRequestV3) error {
	// Root must already be private and canonical before provider creation. Only
	// this explicit setup creates owned child directories, never global resources.
	o, e := parseCensusCLIOptions(r.CensusArguments)
	if e != nil {
		return e
	}
	for _, root := range []string{r.AuxiliaryRoot, r.Configuration.Store, o.PublicationRoot} {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return product.ErrInvalid
		}
		if e := os.Mkdir(root, 0700); e != nil && !os.IsExist(e) {
			return e
		}
		actual, e := filepath.EvalSymlinks(root)
		st, se := os.Lstat(root)
		if e != nil || se != nil || actual != root || !st.IsDir() || st.Mode().Perm() != 0700 {
			return product.ErrInvalid
		}
	}
	roots := []string{o.Workspace, r.AuxiliaryRoot, r.Configuration.Store, o.PublicationRoot}
	for i := range roots {
		for j := i + 1; j < len(roots); j++ {
			if censusInventoryRelatedPathsPrivate(roots[i], roots[j]) {
				return product.ErrInvalid
			}
		}
	}
	if e := product.Initialize(r.Configuration); e != nil {
		return e
	}
	return product.PrepareMailbox(hostProductMailboxV3(r).Directory)
}
func hostProductPreviewV3(ctx context.Context, r hostProductRequestV3) (pilot.FrozenScopePrivateV2, error) {
	o, e := parseCensusCLIOptions(r.CensusArguments)
	if e != nil {
		return pilot.FrozenScopePrivateV2{}, e
	}
	files, e := enumerateCensusPreflightContextPrivate(ctx, o)
	if e != nil {
		return pilot.FrozenScopePrivateV2{}, e
	}
	if len(files) != r.FileDenominator || len(files) > r.Policy.MaxNominationFiles {
		return pilot.FrozenScopePrivateV2{}, product.ErrInvalid
	}
	// This bounded source unit starts from the native Census enumerator, not graph
	// membership. Larger total application staging remains an explicit limitation.
	return pilot.FreezeCensusScopePrivateV2(ctx, o.Workspace, r.RevisionAssertion, files, r.Policy)
}
func hostProductServerPinV3(r hostProductRequestV3) error {
	o, e := parseCensusCLIOptions(r.CensusArguments)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(o.Server, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > 64<<20 {
		return product.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil || len(raw) > 64<<20 || pilot.Digest(raw) != r.ServerSHA256 {
		return product.ErrInvalid
	}
	return nil
}
func runPrivateHostProductV3(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func(phase string) int { fmt.Fprintf(stderr, "private host product: %s\n", phase); return 1 }
	if len(args) == 0 {
		return fail("INVALID_INVOCATION")
	}
	if args[0] == "ready" {
		if len(args) != 5 {
			return fail("INVALID_READY")
		}
		if product.MarkReady(args[1], product.Ready{Protocol: product.Protocol, Role: args[2], LaunchID: args[3], PromptSHA256: args[4]}) != nil {
			return fail("READY_REFUSED")
		}
		return 0
	}
	if args[0] == "wait" {
		if len(args) != 5 {
			return fail("INVALID_WAIT")
		}
		seq, e := strconv.ParseUint(args[3], 10, 64)
		if e != nil {
			return fail("INVALID_SEQUENCE")
		}
		ms, e := strconv.Atoi(args[4])
		if e != nil || ms < 1 || ms > 3600000 {
			return fail("INVALID_WAIT_BOUND")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ms)*time.Millisecond)
		defer cancel()
		b, e := product.WaitBundle(ctx, args[1], args[2], seq)
		if e != nil {
			return fail("TERMINAL_OR_WAIT_DEADLINE")
		}
		json.NewEncoder(stdout).Encode(b)
		return 0
	}
	if args[0] == "respond" {
		if len(args) != 4 {
			return fail("INVALID_RESPONSE")
		}
		seq, e := strconv.ParseUint(args[3], 10, 64)
		if e != nil {
			return fail("INVALID_SEQUENCE")
		}
		raw, e := io.ReadAll(io.LimitReader(stdin, 64<<10+1))
		if e != nil || len(raw) > 64<<10 || product.Submit(args[1], args[2], seq, raw) != nil {
			return fail("RESPONSE_REFUSED")
		}
		return 0
	}
	if len(args) != 2 {
		return fail("INVALID_INVOCATION")
	}
	requestPath := args[1]
	r, e := readHostProductRequestV3(requestPath)
	if e != nil {
		return fail("CONFIG_ADMISSION_REFUSED")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, deadline := context.WithTimeout(ctx, time.Duration(r.OperationTimeoutMS)*time.Millisecond)
	defer deadline()
	emitEarly := func(phase, class string) int {
		report, e := product.DeliverEarlyFailure(r.Configuration, phase, class)
		encodeHostProductV3(stdout, report)
		_ = hostProductMailboxV3(r).Terminal(report)
		if e != nil {
			return fail("REPORT_REFUSED")
		}
		return 1
	}
	switch args[0] {
	case "fail":
		if e = setupHostProductV3(r); e != nil {
			return fail("CLOSED_STORE_ADMISSION_REFUSED")
		}
		report, e := product.DeliverEarlyFailure(r.Configuration, "HOST_TOOL_PREFLIGHT", "HOST_TOOL_UNAVAILABLE")
		if e != nil {
			return fail("REPORT_REFUSED")
		}
		encodeHostProductV3(stdout, report)
		return 1
	case "setup":
		if e = setupHostProductV3(r); e != nil {
			return fail("CLOSED_STORE_ADMISSION_REFUSED")
		}
		type roleTask struct{ Role, Prompt, PromptSHA256 string }
		return encodeHostProductV3(stdout, struct {
			Profile, Mailbox, GrantRequired string
			RoleTasks                       []roleTask
		}{product.Profile, hostProductMailboxV3(r).Directory, hostProductRequestDigestV3(r), []roleTask{{"interpreter", product.RolePrompt("interpreter"), product.PromptDigest("interpreter")}, {"reviewer", product.RolePrompt("reviewer"), product.PromptDigest("reviewer")}}})
	case "preview":
		f, e := hostProductPreviewV3(ctx, r)
		if e != nil {
			return fail("SOURCE_PREVIEW_REFUSED")
		}
		return encodeHostProductV3(stdout, struct {
			Profile, ScopeSHA256, GrantRequired string
			FileDenominator                     int
		}{product.Profile, pilot.Digest(f.ManifestBytes()), hostProductRequestDigestV3(r), r.FileDenominator})
	case "acquire":
		if r.RuntimeGrantSHA256 != hostProductRequestDigestV3(r) || setupHostProductV3(r) != nil || hostProductMailboxV3(r).CheckReady() != nil {
			return fail("RUNTIME_ADMISSION_REFUSED")
		}
		if product.CheckOriginalFresh(r.Configuration) != nil {
			return emitEarly("ORIGINAL_CLAIM_PREFLIGHT", "ORIGINAL_CLAIM_SPENT")
		}
		if hostProductServerPinV3(r) != nil {
			return emitEarly("PROVIDER_EXECUTABLE_PIN", "SERVER_PIN_REFUSED")
		}
		frozen, e := hostProductPreviewV3(ctx, r)
		if e != nil || pilot.Digest(frozen.ManifestBytes()) != r.Configuration.ScopeSHA256 {
			return emitEarly("SOURCE_SCOPE_PREFLIGHT", "STALE_SOURCE_GRANT_REFUSED")
		}
		if e = product.ClaimOriginal(r.Configuration); e != nil {
			return fail("ORIGINAL_CLAIM_SPENT")
		}
		d := productionCensusRunnerDependencies()
		d.runPrivateInventory = func(o censusCLIOptions, c censusCoreConfig, out, errout io.Writer) int {
			return acquireHostProductV3(ctx, r, frozen, o, c, out, errout)
		}
		code := runCensusWithDependencies(append(append([]string{}, r.CensusArguments...), "--private-inventory-config", requestPath), stdout, stderr, d)
		if code != 0 {
			product.DeliverEarlyFailure(r.Configuration, "ORIGINAL_ACQUISITION", "FAILED_OR_CLEANUP_UNVERIFIED")
		}
		return code
	case "resume":
		if r.RuntimeGrantSHA256 != hostProductRequestDigestV3(r) || setupHostProductV3(r) != nil || hostProductMailboxV3(r).CheckReady() != nil {
			return emitEarly("RETAINED_RUNTIME_PREFLIGHT", "RETAINED_RUNTIME_ADMISSION_REFUSED")
		}
		t, e := product.ReadTransfer(filepath.Join(r.AuxiliaryRoot, "transfer.json"))
		if e != nil {
			return emitEarly("RETAINED_TRANSFER_READ", "TRANSFER_REFUSED")
		}
		if validateHostProductTransferV3(r, t) != nil {
			return emitEarly("TRANSFER_METADATA_PINS", "TRANSFER_PIN_REFUSED")
		}
		bootstrap := r.BootstrapAllowance
		report, runErr := product.Continue(ctx, r.Configuration, t, &bootstrap, hostProductMailboxV3(r))
		_ = hostProductMailboxV3(r).Terminal(report)
		code := encodeHostProductV3(stdout, report)
		if runErr != nil {
			return 1
		}
		return code
	case "run":
		if r.RuntimeGrantSHA256 != hostProductRequestDigestV3(r) || setupHostProductV3(r) != nil || hostProductMailboxV3(r).CheckReady() != nil {
			return fail("RUNTIME_ADMISSION_REFUSED")
		}
		if product.CheckOriginalFresh(r.Configuration) != nil {
			return emitEarly("ORIGINAL_CLAIM_PREFLIGHT", "ORIGINAL_CLAIM_SPENT")
		}
		exe, e := os.Executable()
		if e != nil {
			return fail("EXECUTABLE_UNAVAILABLE")
		}
		child := exec.CommandContext(ctx, exe, "private-host-v3", "acquire", requestPath)
		// Provider stdout/stderr are not echoed. Source/error/environment bytes never
		// become terminal diagnostics. The original producer dies before admission.
		if e = child.Run(); e != nil {
			return emitEarly("ORIGINAL_PROCESS", "FAILED_OR_CANCELLED")
		}
		t, e := product.ReadTransfer(filepath.Join(r.AuxiliaryRoot, "transfer.json"))
		if e != nil {
			return fail("TRANSFER_REFUSED")
		}
		if validateHostProductTransferV3(r, t) != nil {
			return emitEarly("TRANSFER_METADATA_PINS", "TRANSFER_PIN_REFUSED")
		}
		bootstrap := r.BootstrapAllowance
		owner, close, e := product.OpenTransfer(ctx, t, &bootstrap, r.Configuration.Exchange)
		if e != nil {
			return emitEarly("VALIDATED_LEDGER_AND_SOURCE", "RETAINED_ADMISSION_REFUSED")
		}
		backend := hostProductMailboxV3(r)
		backend.Observation = t.Cleanup
		backend.Observation.OuterOriginalProcessWaited = true
		report, runErr := product.RunRetained(ctx, r.Configuration, owner, backend)
		closeErr := close()
		// Inner managed provider cleanup happened in acquire; this outer process wait
		// is a separate observation and does not substitute for it.
		_ = hostProductMailboxV3(r).Terminal(report)
		code := encodeHostProductV3(stdout, report)
		if errors.Join(runErr, closeErr) != nil {
			return 1
		}
		return code
	default:
		return fail("INVALID_PRIVATE_OPERATION")
	}
}
func encodeHostProductV3(out io.Writer, v any) int {
	if json.NewEncoder(out).Encode(v) != nil {
		return 1
	}
	return 0
}
func acquireHostProductV3(ctx context.Context, r hostProductRequestV3, frozen pilot.FrozenScopePrivateV2, o censusCLIOptions, c censusCoreConfig, out, errout io.Writer) int {
	return acquireHostProductWithCleanupV3(ctx, r, frozen, o, c, out, errout, 3*time.Second)
}
func acquireHostProductWithCleanupV3(ctx context.Context, r hostProductRequestV3, frozen pilot.FrozenScopePrivateV2, o censusCLIOptions, c censusCoreConfig, out, errout io.Writer, cleanupGrace time.Duration) (resultCode int) {
	var err error
	phase := "ORIGINAL_RETENTION_ROOTS"
	defer func() {
		if resultCode != 0 {
			product.DeliverEarlyFailure(r.Configuration, phase, pilot.ScopeFailureClassPrivateV2(err))
		}
	}()
	var pair [2]*publication.ContentRetentionRoot
	for i := range pair {
		path := filepath.Join(r.AuxiliaryRoot, fmt.Sprintf("raw-%d", i))
		if err = os.Mkdir(path, 0700); err != nil {
			return 1
		}
		pair[i], err = publication.OpenContentRetentionRoot(ctx, path)
		if err != nil {
			return 1
		}
		defer pair[i].Close()
	}
	source, err := publication.OpenSourceProtectionRootPrivateV2(ctx, o.Workspace)
	if err != nil {
		return 1
	}
	defer source.Close()
	roots, err := pilot.NewProducerPacketRootsPrivateV2(ctx, pair, source, nil)
	if err != nil {
		return 1
	}
	ledgerPath := filepath.Join(r.AuxiliaryRoot, "ledger")
	if os.Mkdir(ledgerPath, 0700) != nil {
		return 1
	}
	ledger, err := pilot.CreatePageLedgerPrivateV3(ctx, ledgerPath, r.Configuration.Application, r.ApplicationAllowance)
	if err != nil {
		return 1
	}
	defer ledger.Close()
	current := c.runner
	current.parent = ctx
	current.shutdownManagedByCaller = true
	var id censusacquisition.SessionIdentity
	stops, joins := 0, 0
	cleanupOK := false
	current.observeStart = func(s sessionruntime.StartResult) {
		id = censusacquisition.SessionIdentity{SessionID: s.SessionID, Generation: s.Generation}
	}
	current.observeStop = func(s session.LifecycleResult) { stops++; cleanupOK = string(s.Failure) == "" }
	current.joinAfterStop = func() {
		joins++
		done, cancel := context.WithTimeout(context.Background(), cleanupGrace)
		defer cancel()
		if current.manager.Shutdown(done) != nil {
			cleanupOK = false
		}
	}
	var transfer product.Transfer
	phase = "ORIGINAL_SESSION_START"
	code := productionCensusCoreDependencies().runSession(current, func(live context.Context, runtime *initializedAcquisitionRuntime) int {
		if runtime == nil || runtime.manager != current.manager {
			return 1
		}
		err = current.manager.WithReadySessionLease(live, sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: o.Workspace}, func(leased context.Context) error {
			phase = "ORIGINAL_CENSUS_DISCOVERY"
			origin, _, e := pilot.DiscoverScopePrivateV2(leased, frozen, runtime.manager, id)
			if e != nil {
				return e
			}
			captures := make([]pilot.OpaqueSuccessfulV2Capture, 0, origin.BatchCount())
			for i := 0; i < origin.BatchCount(); i++ {
				b, e := origin.Batch(i)
				if e != nil {
					return e
				}
				phase = "ORIGINAL_CAPTURE_BATCH"
				v, e := pilot.AcquireScopeBatchPrivateV2(leased, runtime.manager, origin, b)
				if e != nil {
					return e
				}
				captures = append(captures, v)
			}
			phase = "ORIGINAL_PAGED_OWNER"
			pages, e := pilot.NewProducerPagesPrivateV3(ctx, frozen, origin, captures, roots, pilot.PagePolicyPrivateV3{PayloadBytes: r.PayloadBytes, ComputationUnitBytes: r.ComputationUnitBytes, ConfigSHA256: r.Configuration.ProducerConfigSHA256}, ledger)
			if e != nil {
				return e
			}
			phase = "ORIGINAL_PAGES"
			revision, _, e := pages.Advance(ctx, 1, r.Configuration.Exchange, "")
			if e != nil {
				return e
			}
			phase = "ORIGINAL_CHECKPOINT_EXPORT"
			raw, e := pages.ExportRetainedCheckpointPrivateV3(ctx, revision, pilot.Digest([]byte(r.Configuration.Application+"/existing-host-retained")), r.Configuration.Exchange)
			if e != nil {
				return e
			}
			var spec pilot.ScopeSpecPrivateV2
			if json.Unmarshal(frozen.SpecBytes(), &spec) != nil {
				return product.ErrInvalid
			}
			transfer = product.Transfer{Manifest: raw, Spec: spec, LedgerPath: ledgerPath}
			return nil
		})
		if err != nil {
			return 1
		}
		return 0
	})
	if code != 0 || err != nil || stops != 1 || joins != 1 || !cleanupOK {
		return 1
	}
	transfer.Cleanup = product.Lifecycle{OriginalStopCalls: stops, OriginalJoinCalls: joins, InnerGraceful: cleanupOK, RoleCleanup: "UNOBSERVED_HOST_ROLE_RETURN"}
	if product.WriteTransfer(filepath.Join(r.AuxiliaryRoot, "transfer.json"), transfer) != nil {
		return 1
	}
	return encodeHostProductV3(out, struct{ Profile, Status string }{product.Profile, "ORIGINAL_PAGES_EXPORTED_CLEANUP_VERIFIED"})
}
