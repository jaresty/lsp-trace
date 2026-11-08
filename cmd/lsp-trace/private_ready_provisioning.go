//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
)

const privateReadyRequestSchema = "lsp-trace.private-ready-request.v2"
const privateReadyDecisionSchema = "lsp-trace.private-ready-decision.v2"
const privateReadyOutputSchema = "lsp-trace.private-ready-initialized.v2"
const privateReadyRecordBytes = 16 << 10 // existing V2 record limit, not a grant
const privateReadyRequestBytes = 64 << 10
const privateReadyDecisionBytes = 1024

type privateReadyVerification struct {
	Records uint64 `json:"records"`
	Bytes   uint64 `json:"bytes"`
	Work    uint64 `json:"work"`
}
type privateReadyRequest struct {
	Schema                 string                   `json:"schema"`
	AccountingRoot         string                   `json:"accounting_root"`
	CanonicalJob           string                   `json:"canonical_job"`
	CanonicalAuthorization string                   `json:"canonical_authorization"`
	Verification           privateReadyVerification `json:"verification"`
}

// A separately supplied descriptor is a caller assertion, NOT authentication or
// proof of approval. Neither stream can derive or default the decision tuple.
type privateReadyDecision struct {
	Schema        string `json:"schema"`
	Job           string `json:"job"`
	Authorization string `json:"authorization"`
	Reference     string `json:"reference"`
}
type privateReadyOutput struct {
	Schema        string `json:"schema"`
	CanonicalJob  []byte `json:"canonical_job"`
	ConfirmedHead string `json:"confirmed_head"`
	Assurance     string `json:"assurance"`
}

func privateReadyCanonical(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	return append(b, '\n'), nil
}
func privateReadyDecode(r io.Reader, limit int64, v any) error {
	if r == nil {
		return errors.New("missing input stream")
	}
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || len(raw) == 0 || int64(len(raw)) > limit {
		return errors.New("input byte bound or read failure")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("invalid closed input")
	}
	canonical, e := privateReadyCanonical(v)
	if e != nil || !bytes.Equal(raw, canonical) {
		return errors.New("noncanonical input")
	}
	return nil
}
func privateReadyRecord(s string) ([]byte, error) {
	if len(s) == 0 || len(s) > base64.StdEncoding.EncodedLen(privateReadyRecordBytes) {
		return nil, errors.New("record text bound")
	}
	// Adjacent decoded lengths share an encoded quartet. Account for padding
	// before allocation, rather than permitting up to two excess decoded bytes.
	decodedLen := base64.StdEncoding.DecodedLen(len(s))
	if strings.HasSuffix(s, "==") {
		decodedLen -= 2
	} else if strings.HasSuffix(s, "=") {
		decodedLen--
	}
	if len(s)%4 != 0 || decodedLen < 1 || decodedLen > privateReadyRecordBytes {
		return nil, errors.New("record decoded bound")
	}
	raw, e := base64.StdEncoding.Strict().DecodeString(s)
	if e != nil || len(raw) == 0 || len(raw) > privateReadyRecordBytes || base64.StdEncoding.EncodeToString(raw) != s {
		return nil, errors.New("record byte bound or encoding")
	}
	return raw, nil
}

func runPrivateReadyProvisioning(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func() int {
		fmt.Fprintln(stderr, "private READY provisioning refused; partial publication may remain; no automatic retry/reset")
		return 1
	}
	if len(args) != 3 || args[0] != "initialize-ready" || args[1] != "--decision-fd" {
		return fail()
	}
	n, e := strconv.ParseUint(args[2], 10, 31)
	if e != nil || n < 3 || strconv.FormatUint(n, 10) != args[2] {
		return fail()
	}
	// Do not close borrowed standard streams, including a custom direct caller's
	// stream object supplied under the same descriptor number.
	for _, s := range []any{stdin, stdout, stderr} {
		if f, ok := s.(*os.File); ok && f.Fd() == uintptr(n) {
			return fail()
		}
	}
	decisionFile := os.NewFile(uintptr(n), "private-ready-decision")
	if decisionFile == nil {
		return fail()
	}
	var request privateReadyRequest
	var decision privateReadyDecision
	// Always close the owned decision descriptor exactly once, including errors.
	inputErr := privateReadyDecode(stdin, privateReadyRequestBytes, &request)
	if inputErr == nil {
		inputErr = privateReadyDecode(decisionFile, privateReadyDecisionBytes, &decision)
	}
	closeDecisionErr := decisionFile.Close()
	if errors.Join(inputErr, closeDecisionErr) != nil || request.Schema != privateReadyRequestSchema || decision.Schema != privateReadyDecisionSchema {
		return fail()
	}
	jb, e := privateReadyRecord(request.CanonicalJob)
	if e != nil {
		return fail()
	}
	ab, e := privateReadyRecord(request.CanonicalAuthorization)
	if e != nil {
		return fail()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if ctx.Err() != nil {
		return fail()
	}
	root, e := publication.OpenRoot(request.AccountingRoot)
	if e != nil {
		return fail()
	}
	if e = root.ValidatePrivate(); e != nil {
		_ = root.Close()
		return fail()
	}
	result, initErr := inventorysuccessor.InitializePrivateReadyV2(ctx, root, jb, ab, inventorysuccessor.PrivateReadyDecisionAssertionV2{Job: decision.Job, Authorization: decision.Authorization, Reference: decision.Reference}, inventorysuccessor.PrivateManagedInputVerificationV2{Records: request.Verification.Records, Bytes: request.Verification.Bytes, Work: request.Verification.Work})
	closeErr := root.Close() // last owned close, including on partial initialization
	if errors.Join(initErr, closeErr) != nil {
		return fail()
	}
	if privateReadyWriteSuccess(ctx, result, closeErr, stdout) != nil {
		return fail()
	}
	return 0
}

// Fixed final barrier, not an IO framework or an injectable initializer. A
// partial/failed output is never exit-zero; bytes already written cannot be
// recalled, and all accounting publication remains retained on output failure.
func privateReadyWriteSuccess(ctx context.Context, result *inventorysuccessor.PrivateReadyInitializedV2, closeErr error, stdout io.Writer) error {
	if closeErr != nil {
		return closeErr
	}
	if e := result.CheckUsable(ctx); e != nil {
		return e
	}
	raw, e := privateReadyCanonical(privateReadyOutput{privateReadyOutputSchema, result.CanonicalJob(), result.ConfirmedHead(), "CALLER_ASSERTED_NOT_AUTHENTICATED"})
	if e != nil || len(raw) > privateReadyRequestBytes {
		return errors.New("output bound")
	}
	if e = result.CheckUsable(ctx); e != nil {
		return e
	}
	n, e := stdout.Write(raw)
	if e != nil {
		return e
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	// Output cannot be recalled, but cancellation or original expiry during a
	// blocking writer must still prohibit exit-zero. No replacement window.
	return result.CheckUsable(ctx)
}
