//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
)

// The calling agent lends a connected local stream descriptor, not credentials,
// executable configuration, native authority, or a Python service. The Go child
// owns a duplicate for this invocation only. Host adapters own their other end
// and must join this process. No standard descriptor is claimed implicitly.
const censusCallerFDEnv = "LSP_TRACE_CALLER_FD"
const censusCallerProtocol = "lsp-trace.census-caller.v1"

type censusCallerHello struct {
	Protocol              string
	Subagents             bool
	Interpreter, Reviewer string
	Pins                  dw.HostPinsV2
}
type censusCallerMessage struct {
	Protocol, Event, RequestPath, RunID string
	ReviewMode                          cat.ReviewModeV3
	RolePrompts                         [2]string
	Frame                               *product.InventoryFrameV3             `json:",omitempty"`
	Terminal                            *applicationInventoryTerminalRecordV3 `json:",omitempty"`
}
type censusCallerReply struct {
	Protocol, RunID, Role, RequestDigest string
	Sequence                             int
	Payload                              []byte
}

func openCensusCaller(ctx context.Context) (net.Conn, censusCallerHello, error) {
	var hello censusCallerHello
	missing := errors.New("CALLER_AGENT_UNAVAILABLE: invoke census from a calling agent with the census caller IPC adapter; a bare shell cannot author Describe/group/review semantics (no API key is required)")
	value := os.Getenv(censusCallerFDEnv)
	fd, err := strconv.Atoi(value)
	if err != nil || fd < 3 || strconv.Itoa(fd) != value {
		return nil, hello, missing
	}
	// Socket type/address validation happens before transferring ownership. Never
	// accept files, pipes, TCP sockets, listeners, or an unconnected descriptor.
	typ, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TYPE)
	peer, pe := unix.Getpeername(fd)
	_, local := peer.(*unix.SockaddrUnix)
	if err != nil || pe != nil || typ != unix.SOCK_STREAM || !local {
		return nil, hello, missing
	}
	duplicate, err := unix.Dup(fd)
	if err != nil {
		return nil, hello, err
	}
	unix.CloseOnExec(duplicate)
	file := os.NewFile(uintptr(duplicate), "census-caller")
	conn, err := net.FileConn(file)
	_ = file.Close()
	if err != nil {
		return nil, hello, err
	}
	deadline, ok := ctx.Deadline()
	if !ok || conn.SetDeadline(deadline) != nil {
		conn.Close()
		return nil, hello, missing
	}
	stopWake := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopWake()
	if err = readCensusCaller(conn, &hello, 4096); err != nil {
		conn.Close()
		return nil, hello, err
	}
	if hello.Protocol != censusCallerProtocol || hello.Interpreter == "" || hello.Reviewer == "" || len(hello.Interpreter) > 256 || len(hello.Reviewer) > 256 || hello.Subagents && hello.Interpreter == hello.Reviewer || !hello.Subagents && hello.Interpreter != hello.Reviewer || !validApplicationDigest(hello.Pins.ModelSHA256) || !validApplicationDigest(hello.Pins.WorkerSHA256) || !validApplicationDigest(hello.Pins.GrammarSHA256) || hello.Pins.PromptSHA256 != "" {
		conn.Close()
		return nil, hello, product.ErrInvalid
	}
	return conn, hello, nil
}

// Length framing bounds allocation before JSON parsing. Closed response grammar
// and request digest/sequence checks prevent stale or cross-role submissions.
func readCensusCaller(r io.Reader, value any, limit uint32) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > limit {
		return product.ErrInvalid
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(r, raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return product.ErrInvalid
	}
	return nil
}
func writeCensusCaller(w io.Writer, value any, limit uint32) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) == 0 || uint64(len(raw)) > uint64(limit) {
		return product.ErrInvalid
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(raw)))
	for _, b := range [][]byte{header[:], raw} {
		if n, e := w.Write(b); e != nil {
			return e
		} else if n != len(b) {
			return io.ErrShortWrite
		}
	}
	return nil
}

func censusFolderInventoryRequest(o censusCLIOptions, deadline time.Time, hello censusCallerHello, targets []product.InventoryTargetV3, leads []ai.Lead, grant product.ApplicationGrant, producer string) (product.InventoryRequestV3, error) {
	expected, err := product.InventoryLocalGrantV3(deadline, uint64(len(leads)+2), applicationCoordinatorQuote(1, 16))
	if err != nil || grant != expected || !validApplicationDigest(producer) {
		return product.InventoryRequestV3{}, product.ErrInvalid
	}
	// Review mode is fixed before either policy seal, approval, setup or readiness.
	mode := cat.ReviewModeSameThreadV3
	if hello.Subagents {
		mode = cat.ReviewModeSubagentsV3
	}
	revision := censusInventorySourceRevision
	if len(targets) > 0 {
		revision = targets[0].Revision
	}
	manifest := ai.Manifest{Scope: o.Workspace, Provenance: "bounded normal-folder paid acquisition; original D/R and both CALLS retained", Leads: leads, UntouchedGaps: []string{"bounded one-partition acquisition; all unselected source and whole-application completeness remain unknown"}}
	// The configured V3 host ceiling is 30s per exchange; its transport has an
	// independent 120s cumulative reservation. Neither can renew the original
	// native owner deadline (currently stricter than the application 180s cap).
	window := min(o.RequestTimeout, 30*time.Second, 120*time.Second/time.Duration(len(leads)+2), time.Until(deadline))
	if window < time.Millisecond {
		return product.InventoryRequestV3{}, product.ErrInvalid
	}
	// Existing caller-memory/output caps remain upper bounds, not native credit.
	// Intersect with the admitted logical envelope; actual retained output is
	// checked by Run and cannot silently enlarge this pre-effects quote.
	callerBytes := min(uint64(64<<20), grant.Ceiling.LogicalBytes)
	retainedUpper := min(uint64(8<<20), callerBytes)
	q := product.InventoryRequestV3{Schema: "private-host-application-inventory-request.v3", Inventory: product.InventoryOptionsV3{SourceRevision: revision, Manifest: manifest, Pins: hello.Pins, MaxNativeAttempts: len(leads), NativeRetainedUpper: retainedUpper, BudgetBytes: callerBytes, HostWindowMS: int(window / time.Millisecond), Targets: targets, ReviewMode: mode}}
	// Actual current one-shot parent admission supplies this derived grant.
	// Native/provider and preparation grants remain separate; no shared credit.
	q.Configuration = product.ApplicationConfig{Schema: "private-host-application-config.v3", Profile: product.InventoryProfileV3, Store: filepath.Join(o.PublicationRoot, "application"), SourceRoot: o.Workspace, RevisionAssertion: revision, ProducerExecutableSHA256: producer, StorageIdentitySHA256: digestCatalog(o.PublicationRoot), RolePromptSHA256: [2]string{censusInventoryDigest([]byte(product.InventoryRolePromptV3("interpreter"))), censusInventoryDigest([]byte(product.InventoryRolePromptV3("reviewer")))}, Grant: grant}
	policy, err := json.Marshal(q.Inventory)
	if err != nil {
		return q, err
	}
	census, err := json.Marshal(manifest)
	if err != nil {
		return q, err
	}
	q.Configuration.UnitPolicySHA256 = censusInventoryDigest(policy)
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.SelectionRuleSHA256 = censusInventoryDigest(census)
	q.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = product.InventoryRequestDigestV3(q)
	return q, product.ValidateInventoryRequestV3(q)
}

// The controller is the sole M/P/S preparer. This function returns only after its
// goroutine and caller service loop have joined, so opaque roots cannot escape.
func runCensusFolderController(ctx context.Context, o censusCLIOptions, inputs product.InventoryNativeInputsV3, targets []product.InventoryTargetV3, leads []ai.Lead, out io.Writer, grant product.ApplicationGrant, producer string) error {
	if o.DurableInventory {
		return startCensusDurableInventory(ctx, o, inputs, targets, leads, out)
	}
	if o.DurableDescribe {
		return startCensusDurableDescribe(ctx, o, inputs, targets, leads, out)
	}
	conn, hello, err := openCensusCaller(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Cancellation must wake a blocked IPC read, not wait for a full host turn.
	stopWake := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopWake()
	deadline, ok := ctx.Deadline()
	if !ok {
		return product.ErrInvalid
	}
	q, err := censusFolderInventoryRequest(o, deadline, hello, targets, leads, grant, producer)
	if err != nil {
		return fmt.Errorf("folder inventory request: %w", err)
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return err
	}
	path := filepath.Join(o.PublicationRoot, "inventory-request.json")
	if err = applicationWriteOnce(path, raw); err != nil {
		return err
	}
	var setup, diagnostic bytes.Buffer
	if runApplicationInventoryControllerV3([]string{"setup", path}, raw, nil, &setup, &diagnostic, inputs, nil) != 0 {
		return fmt.Errorf("folder controller setup: %s", diagnostic.String())
	}
	var ticket struct {
		Schema, RunID string
		RoleTasks     []struct{ Role, Prompt, PromptSHA256 string }
	}
	if json.Unmarshal(setup.Bytes(), &ticket) != nil || !validApplicationDigest(ticket.RunID) {
		return product.ErrInvalid
	}
	for i, role := range []string{"interpreter", "reviewer"} {
		actor := hello.Interpreter
		if i == 1 {
			actor = hello.Reviewer
		}
		if applicationInventoryHostV3([]string{"register", path, ticket.RunID, role, actor, q.Configuration.RolePromptSHA256[i]}, q, nil, io.Discard, &diagnostic) != 0 {
			return fmt.Errorf("folder controller register: %s", diagnostic.String())
		}
	}
	activation := censusCallerMessage{Protocol: censusCallerProtocol, Event: "ACTIVATION_READY", RequestPath: path, RunID: ticket.RunID, ReviewMode: q.Inventory.ReviewMode, RolePrompts: [2]string{product.InventoryRolePromptV3("interpreter"), product.InventoryRolePromptV3("reviewer")}}
	if err = writeCensusCaller(conn, activation, 1<<20); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		code       int
		raw        []byte
		diagnostic string
	}
	done := make(chan result, 1)
	go func() {
		var terminal, diag bytes.Buffer
		code := runApplicationInventoryControllerContextV3(runCtx, []string{"run", path, ticket.RunID}, raw, nil, &terminal, &diag, inputs, nil)
		done <- result{code, terminal.Bytes(), diag.String()}
	}()
	serviceErr := serveCensusFolderCaller(runCtx, conn, q, path, ticket.RunID)
	if serviceErr != nil {
		cancel()
	}
	finished := <-done // unconditional ownership join, using the original deadline
	if serviceErr != nil {
		return fmt.Errorf("caller exchange: %w", serviceErr)
	}
	if finished.code != 0 {
		return fmt.Errorf("folder controller: %s", finished.diagnostic)
	}
	var terminal applicationInventoryTerminalRecordV3
	if !applicationInventoryValidTerminalV3(finished.raw) || json.Unmarshal(finished.raw, &terminal) != nil || terminal.Status != "PROVISIONAL" {
		return product.ErrInvalid
	}
	var publication product.InventoryPublicationV3
	if err = product.ReadApplicationRecord(q.Configuration.Store, terminal.InventoryHead, &publication); err != nil {
		return err
	}
	if err = product.VerifyInventoryPublicationV3(publication, q); err != nil {
		return fmt.Errorf("folder inventory publication readback: %w", err)
	}
	// Caller completion is informational, never a substitute for publication proof.
	if err = writeCensusCaller(conn, censusCallerMessage{Protocol: censusCallerProtocol, Event: "TERMINAL", RunID: ticket.RunID, ReviewMode: q.Inventory.ReviewMode, Terminal: &terminal}, 1<<20); err != nil {
		return err
	}
	_, err = out.Write(finished.raw)
	return err
}

func serveCensusFolderCaller(ctx context.Context, conn net.Conn, q product.InventoryRequestV3, path, runID string) error {
	role := "interpreter"
	cursors := map[string]uint64{}
	for n := 0; n < 16; n++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var output, diagnostic bytes.Buffer
		// Each wait identity is spent once. Existing finite wait-slice accounting and
		// grant expiry bound the blocking operation; no polling loop resets its cursor.
		if applicationInventoryHostV3([]string{"wait", path, runID, role, strconv.FormatUint(cursors[role], 10), "15000"}, q, nil, &output, &diagnostic) != 0 {
			return fmt.Errorf("wait: %s", diagnostic.String())
		}
		var event applicationHostEvent
		if json.Unmarshal(output.Bytes(), &event) != nil {
			return product.ErrInvalid
		}
		if event.Event == "TERMINAL" {
			return nil
		}
		if event.Event != "NEEDS_HOST" || event.Bundle == nil {
			return fmt.Errorf("caller wait: %s", event.Event)
		}
		frame, err := product.ReadInventoryBundleV3(*event.Bundle)
		if err != nil {
			return err
		}
		if frame.Role != role || frame.RunID != runID || frame.Sequence != int(event.GlobalSequence) || event.GlobalSequence <= cursors[role] {
			return product.ErrInvalid
		}
		frameDeadline := time.Unix(0, frame.DeadlineUnixNano)
		if d, ok := ctx.Deadline(); !ok || frameDeadline.After(d) {
			return product.ErrInvalid
		}
		if err = conn.SetDeadline(frameDeadline); err != nil {
			return err
		}
		if err = writeCensusCaller(conn, censusCallerMessage{Protocol: censusCallerProtocol, Event: "NEEDS_HOST", RunID: runID, ReviewMode: q.Inventory.ReviewMode, Frame: &frame}, (1<<20)+4096); err != nil {
			return err
		}
		var reply censusCallerReply
		if err = readCensusCaller(conn, &reply, 96<<10); err != nil {
			return err
		}
		if reply.Protocol != censusCallerProtocol || reply.RunID != runID || reply.Role != role || reply.Sequence != frame.Sequence || reply.RequestDigest != frame.RequestDigest || len(reply.Payload) > 64<<10 {
			return product.ErrInvalid
		}
		if applicationInventoryHostV3([]string{"respond", path, runID, role, strconv.Itoa(frame.Sequence)}, q, bytes.NewReader(reply.Payload), io.Discard, &diagnostic) != 0 {
			return fmt.Errorf("respond: %s", diagnostic.String())
		}
		cursors[role] = event.GlobalSequence
		if d, ok := ctx.Deadline(); ok {
			if err = conn.SetDeadline(d); err != nil {
				return err
			}
		}
		if frame.Operation == "GroupDecision" {
			role = "reviewer"
		}
		if frame.Operation == "Reviewer" {
			return nil
		}
	}
	return errors.New("caller operation bound reached")
}
