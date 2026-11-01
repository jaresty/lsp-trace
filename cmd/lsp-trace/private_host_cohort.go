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

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// Explicit sibling. The SOURCE request remains exact legacy bytes and has its
// own caller-supplied source/runtime grant. No semantic budget is inferred from it.
type hostCohortRequest struct {
	Schema, SourceRequest, SourceRequestSHA256, RuntimeGrantSHA256 string
	Configuration                                                  product.CohortConfig
	BackendTimeoutMS                                               int
}

func hostCohortDigest(r hostCohortRequest) string {
	r.RuntimeGrantSHA256 = ""
	b, _ := json.Marshal(r)
	return pilot.Digest(b)
}
func readHostCohortRequest(path string) (hostCohortRequest, hostProductRequestV3, error) {
	var r hostCohortRequest
	var source hostProductRequestV3
	raw, e := readHostProductRequestRawV3(path)
	if e != nil {
		return r, source, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil {
		return r, source, product.ErrInvalid
	}
	canonical, _ := json.Marshal(r)
	if !bytes.Equal(raw, canonical) || r.Schema != "private-existing-host-cohort-request.v1" || product.ValidateCohortConfig(r.Configuration) != nil || !filepath.IsAbs(r.Configuration.Store) || filepath.Clean(r.Configuration.Store) != r.Configuration.Store || r.BackendTimeoutMS < 1 || r.BackendTimeoutMS > 180000 {
		return r, source, product.ErrInvalid
	}
	source, e = readHostProductRequestV3(r.SourceRequest)
	if e != nil {
		return r, source, e
	}
	sourceBytes, _ := json.Marshal(source)
	if r.SourceRequestSHA256 != pilot.Digest(sourceBytes) {
		return r, source, product.ErrInvalid
	}
	o, e := parseCensusCLIOptions(source.CensusArguments)
	if e != nil || r.Configuration.Requested != source.Configuration.Requested || source.OperationTimeoutMS > 240000 || source.ExchangeTimeoutMS > 60000 || r.Configuration.SourceConfigSHA256 != product.ConfigDigest(source.Configuration) || r.Configuration.ScopeSHA256 != source.Configuration.ScopeSHA256 || hostProductLexicalRelatedV3(r.Configuration.Store, source.Configuration.Store) || hostProductLexicalRelatedV3(r.Configuration.Store, source.AuxiliaryRoot) || hostProductLexicalRelatedV3(r.Configuration.Store, o.Workspace) || hostProductLexicalRelatedV3(r.Configuration.Store, o.PublicationRoot) {
		return r, source, product.ErrInvalid
	}
	return r, source, nil
}
func hostCohortMailbox(r hostCohortRequest, source hostProductRequestV3) product.CohortMailbox {
	return product.CohortMailbox{Directory: filepath.Join(source.AuxiliaryRoot, "cohort-mailbox"), ExchangeTimeout: time.Duration(source.ExchangeTimeoutMS) * time.Millisecond}
}
func runPrivateHostCohort(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func(s string) int { fmt.Fprintln(stderr, "private cohort:", s); return 1 }
	if len(args) == 0 {
		return fail("INVALID_INVOCATION")
	}
	switch args[0] {
	case "prompts":
		if len(args) != 1 {
			return fail("INVALID_PROMPT_QUERY")
		}
		type roleTask struct{ Role, Prompt, PromptSHA256 string }
		roles := []roleTask{}
		for _, role := range []string{"interpreter", "reviewer"} {
			prompt := product.CohortRolePrompt(role)
			roles = append(roles, roleTask{role, prompt, pilot.Digest([]byte(prompt))})
		}
		return encodeHostProductV3(stdout, struct {
			Profile, Protocol string
			RoleTasks         []roleTask
		}{product.CohortProfile, product.CohortProtocol, roles})
	case "ready":
		if len(args) != 5 {
			return fail("INVALID_READY")
		}
		if product.MarkCohortReady(args[1], product.Ready{Protocol: product.CohortProtocol, Role: args[2], LaunchID: args[3], PromptSHA256: args[4]}) != nil {
			return fail("READY_REFUSED")
		}
		return 0
	case "wait":
		if len(args) != 5 {
			return fail("INVALID_WAIT")
		}
		seq, e := strconv.ParseUint(args[3], 10, 64)
		ms, me := strconv.Atoi(args[4])
		if e != nil || me != nil || ms < 1 || ms > 270000 {
			return fail("INVALID_WAIT")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ms)*time.Millisecond)
		defer cancel()
		result, e := product.WaitCohort(ctx, args[1], args[2], seq)
		// Keep the existing JSON bytes, including its newline, but check the
		// physical write count as well as its error. The event is bounded by
		// the mailbox metadata ceiling; no role payload is buffered here.
		var publication bytes.Buffer
		publicationCode := encodeHostProductV3(&publication, result)
		if publicationCode == 0 {
			n, writeErr := stdout.Write(publication.Bytes())
			if writeErr != nil || n != publication.Len() {
				publicationCode = 1
			}
		}
		if publicationCode != 0 {
			return publicationCode
		}
		if e != nil {
			return 1
		}
		return 0
	case "respond":
		if len(args) != 4 {
			return fail("INVALID_RESPONSE")
		}
		seq, e := strconv.ParseUint(args[3], 10, 64)
		raw, re := io.ReadAll(io.LimitReader(stdin, 64<<10+1))
		if e != nil || re != nil || product.SubmitCohort(args[1], args[2], seq, raw) != nil {
			return fail("RESPONSE_REFUSED")
		}
		return 0
	}
	if len(args) != 2 {
		return fail("INVALID_INVOCATION")
	}
	r, source, e := readHostCohortRequest(args[1])
	if e != nil {
		return fail("CONFIG_ADMISSION_REFUSED")
	}
	mailbox := hostCohortMailbox(r, source)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, time.Duration(source.OperationTimeoutMS)*time.Millisecond)
	defer stop()
	if args[0] == "finalize" {
		raw, e := io.ReadAll(io.LimitReader(stdin, 16385))
		returns, decodeErr := product.DecodeCohortRoleReturns(raw)
		if e != nil || decodeErr != nil {
			return fail("INVALID_HOST_RETURNS")
		}
		view, e := product.FinalizeCohortHost(r.Configuration, mailbox.Directory, returns)
		if e != nil {
			return fail("HOST_RETURN_REFUSED")
		}
		return encodeHostProductV3(stdout, view)
	}
	setup := func() error {
		if e := setupHostProductV3(source); e != nil {
			return e
		}
		if e := product.PrepareMailbox(mailbox.Directory); e != nil {
			return e
		}
		return product.InitializeCohort(r.Configuration)
	}
	switch args[0] {
	case "setup":
		if e = setup(); e != nil {
			return fail("SETUP_REFUSED")
		}
		type roleTask struct{ Role, Prompt, PromptSHA256 string }
		roles := []roleTask{}
		for _, role := range []string{"interpreter", "reviewer"} {
			p := product.CohortRolePrompt(role)
			roles = append(roles, roleTask{role, p, pilot.Digest([]byte(p))})
		}
		return encodeHostProductV3(stdout, struct {
			Profile, Mailbox, GrantRequired string
			RoleTasks                       []roleTask
		}{product.CohortProfile, mailbox.Directory, hostCohortDigest(r), roles})
	case "preview":
		f, e := hostProductPreviewV3(ctx, source)
		if e != nil {
			return fail("PREVIEW_REFUSED")
		}
		return encodeHostProductV3(stdout, struct{ ScopeSHA256, GrantRequired string }{pilot.Digest(f.ManifestBytes()), hostCohortDigest(r)})
	case "run", "acquire", "fail":
	default:
		return fail("INVALID_OPERATION")
	}
	// Every runtime exit attempts a terminal marker. Publication errors are surfaced,
	// never ignored or presented as a role-return observation.
	result := product.CohortReport{}
	recoveryAttempted := false
	code := func() (code int) {
		if e = setup(); e != nil {
			return fail("SETUP_REFUSED")
		}
		if args[0] == "fail" {
			recoveryAttempted = true
			result, e = product.DeliverCohortFailure(r.Configuration, source.Configuration.Requested, "HOST_TOOL_UNAVAILABLE")
			if e != nil {
				return fail("REPORT_REFUSED")
			}
			return 1
		}
		if r.RuntimeGrantSHA256 != hostCohortDigest(r) || source.RuntimeGrantSHA256 != hostProductRequestDigestV3(source) || mailbox.CheckReady() != nil {
			return fail("RUNTIME_GRANT_OR_ROLE_REFUSED")
		}
		if args[0] == "acquire" {
			if product.CheckOriginalFresh(source.Configuration) != nil || hostProductServerPinV3(source) != nil {
				return fail("ORIGINAL_PREFLIGHT_REFUSED")
			}
			frozen, e := hostProductPreviewV3(ctx, source)
			if e != nil || pilot.Digest(frozen.ManifestBytes()) != source.Configuration.ScopeSHA256 {
				return fail("STALE_SOURCE_REFUSED")
			}
			if product.ClaimOriginal(source.Configuration) != nil {
				return fail("ORIGINAL_CLAIM_SPENT")
			}
			deps := productionCensusRunnerDependencies()
			deps.runPrivateInventory = func(o censusCLIOptions, c censusCoreConfig, out, errout io.Writer) int {
				return acquireHostProductWithCleanupV3(ctx, source, frozen, o, c, out, errout, 2*time.Second)
			}
			return runCensusWithDependencies(append(append([]string{}, source.CensusArguments...), "--private-inventory-config", r.SourceRequest), stdout, stderr, deps)
		}
		exe, e := os.Executable()
		if e != nil {
			return fail("EXECUTABLE_UNAVAILABLE")
		}
		child := exec.CommandContext(ctx, exe, "private-host-cohort", "acquire", args[1])
		if e = child.Run(); e != nil {
			return fail("ORIGINAL_PROCESS_FAILED")
		}
		transfer, e := product.ReadTransfer(filepath.Join(source.AuxiliaryRoot, "transfer.json"))
		if e != nil || validateHostProductTransferV3(source, transfer) != nil {
			return fail("TRANSFER_REFUSED")
		}
		bootstrap := source.BootstrapAllowance
		owner, close, e := product.OpenTransfer(ctx, transfer, &bootstrap, source.Configuration.Exchange)
		if e != nil {
			return fail("RETAINED_ADMISSION_REFUSED")
		}
		mailbox.Observation = transfer.Cleanup
		mailbox.Observation.OuterOriginalProcessWaited = true
		backendCtx, backendStop := context.WithTimeout(ctx, time.Duration(r.BackendTimeoutMS)*time.Millisecond)
		defer backendStop()
		result, e = product.RunCohortRetained(backendCtx, source.Configuration, r.Configuration, owner, mailbox)
		closeErr := close()
		if errors.Join(e, closeErr) != nil {
			return 1
		}
		return 0
	}()
	// Acquire child must NOT announce whole-workflow terminal before retained phase.
	if args[0] == "acquire" {
		return code
	}
	if result.Schema == "" && !recoveryAttempted {
		recoveryAttempted = true
		result, e = product.DeliverCohortFailure(r.Configuration, source.Configuration.Requested, "EARLY_FAILURE")
		if e != nil {
			code = 1
			fail("REPORT_REFUSED")
		}
	}
	// A field-graph bound is not admission of an empty report. Refusal before
	// configuration/store admission emits no unqualified zero-spend DTO.
	if result.Schema == "" {
		return fail("REPORT_REFUSED")
	}
	if e = mailbox.TerminalCohort(result); e != nil {
		code = 1
		fail("TERMINAL_PUBLICATION_FAILED")
	}
	if !product.AdmitCohortReport(result) {
		return fail("REPORT_FIELD_GRAPH_REFUSED")
	}
	if encodeHostProductV3(stdout, result) != 0 {
		code = 1
	}
	return code
}
