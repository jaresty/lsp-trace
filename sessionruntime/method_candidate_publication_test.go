package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
)

const maxTestMethodCandidateBytes = 128 << 10

// This v0 codec belongs exclusively to the test-owned probe. It is not an ADR
// method receipt schema and its self-described identity does not attest origin.
type testMethodCandidate struct {
	Version       string                  `json:"version"`
	SessionID     string                  `json:"session_id"`
	Generation    uint64                  `json:"generation"`
	Key           lspwire.RequestKey      `json:"key"`
	Method        string                  `json:"method"`
	RequestWrite  RequestWriteObservation `json:"request_write"`
	ResponseRead  ResponseReadObservation `json:"response_read"`
	RequestFrame  []byte                  `json:"request_frame"`
	ResponseFrame []byte                  `json:"response_frame"`
}

func encodeTestMethodCandidate(c methodCandidateObservation) ([]byte, error) {
	if len(c.RequestFrame) == 0 || len(c.ResponseFrame) == 0 || len(c.RequestFrame) > maxTestMethodCandidateBytes/4 || len(c.ResponseFrame) > maxTestMethodCandidateBytes/4 {
		return nil, errors.New("test candidate frame bound")
	}
	raw, err := json.Marshal(testMethodCandidate{
		Version: "adr0011-owner-path-test/v0", SessionID: c.SessionID, Generation: c.Generation, Key: c.Key,
		Method: c.Method, RequestWrite: c.RequestWrite, ResponseRead: c.ResponseRead,
		RequestFrame: c.RequestFrame, ResponseFrame: c.ResponseFrame,
	})
	if err != nil || len(raw) > maxTestMethodCandidateBytes {
		return nil, errors.New("test candidate encoding bound")
	}
	return raw, nil
}

func verifyTestMethodCandidate(raw []byte) error {
	fail := errors.New("test candidate byte consistency failed")
	if len(raw) == 0 || len(raw) > maxTestMethodCandidateBytes {
		return fail
	}
	var c testMethodCandidate
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&c) != nil || dec.Decode(&struct{}{}) != io.EOF {
		return fail
	}
	canonical, err := json.Marshal(c)
	if err != nil || !bytes.Equal(canonical, raw) || c.Version != "adr0011-owner-path-test/v0" || c.SessionID == "" || c.Generation == 0 || c.Key.Generation != c.Generation || c.Key.ID == 0 ||
		(c.Method != "textDocument/definition" && c.Method != "textDocument/references") ||
		c.RequestWrite.SessionID != c.SessionID || c.ResponseRead.SessionID != c.SessionID ||
		c.RequestWrite.Generation != c.Generation || c.ResponseRead.Generation != c.Generation ||
		c.RequestWrite.Key != c.Key || c.ResponseRead.Key != c.Key || c.RequestWrite.Method != c.Method {
		return fail
	}
	for _, f := range []struct {
		raw    []byte
		length int64
		digest string
	}{
		{c.RequestFrame, c.RequestWrite.FrameBytes, c.RequestWrite.FrameSHA256},
		{c.ResponseFrame, c.ResponseRead.FrameBytes, c.ResponseRead.FrameSHA256},
	} {
		if len(f.raw) == 0 || len(f.raw) > maxTestMethodCandidateBytes/4 || f.length != int64(len(f.raw)) || f.digest != fmt.Sprintf("sha256:%x", sha256.Sum256(f.raw)) {
			return fail
		}
	}
	limits := lspwire.Limits{MaxBodyBytes: maxTestMethodCandidateBytes, MaxHeaderBytes: 64 << 10}
	request, observedRequest, _, err := lspwire.NewReader(bytes.NewReader(c.RequestFrame), limits).ReadWithExactFrame(int64(len(c.RequestFrame)))
	if err != nil || observedRequest.FrameBytes != int64(len(c.RequestFrame)) || request.Kind() != lspwire.KindRequest || request.Method != c.Method {
		return fail
	}
	response, observedResponse, _, err := lspwire.NewReader(bytes.NewReader(c.ResponseFrame), limits).ReadWithExactFrame(int64(len(c.ResponseFrame)))
	if err != nil || observedResponse.FrameBytes != int64(len(c.ResponseFrame)) || response.Kind() != lspwire.KindSuccessResponse {
		return fail
	}
	for _, id := range []json.RawMessage{request.ID, response.ID} {
		n, parseErr := strconv.ParseUint(string(id), 10, 64)
		if parseErr != nil || n != c.Key.ID {
			return fail
		}
	}
	return nil
}

func testCandidateRoot(t *testing.T) (*publication.Root, string) {
	t.Helper()
	path := t.TempDir()
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, path
}

func TestManagerTestOwnedPrivateCandidatePublication(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method string
		messages           int
	}{
		{"definition", "success", "textDocument/definition", 3},
		{"references-null", "references-null", "textDocument/references", 1},
		{"references-repeated", "references-repeated", "textDocument/references", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := testCandidateRoot(t)
			var selector string
			var raw []byte
			var published *publication.BoundFileReceipt
			var publishErr error
			calls := 0
			m, started := methodCandidateManager(t, tc.mode, func(c methodCandidateObservation) {
				calls++
				raw, publishErr = encodeTestMethodCandidate(c)
				if publishErr != nil {
					return
				}
				sum := sha256.Sum256(raw)
				selector = fmt.Sprintf("adr0011-test-candidate-%x.bin", sum)
				published, publishErr = publication.PublishBoundFile(root, selector, raw, verifyTestMethodCandidate)
			})
			got := m.RoundTrip(context.Background(), methodCandidateRequest(started, tc.method, tc.messages))
			if got.Failure != "" || got.ServerError != nil || calls != 1 || publishErr != nil || published == nil || published.VerificationStatus != "VERIFIED" || published.FinalSelector != selector || published.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) || published.ByteLength != uint64(len(raw)) {
				t.Fatalf("ASSERT_ADR0011_OWNER_TEST_PRIVATE_PUBLISH: case=%s failure=%s calls=%d err=%v receipt=%+v", tc.name, got.Failure, calls, publishErr, published)
			}
			committed, err := root.ReadSelector(selector, maxTestMethodCandidateBytes)
			if err != nil || !bytes.Equal(committed, raw) || verifyTestMethodCandidate(committed) != nil {
				t.Fatalf("ASSERT_ADR0011_OWNER_TEST_IMMUTABLE_READBACK: err=%v equal=%v", err, bytes.Equal(committed, raw))
			}
			if second, err := publication.PublishBoundFile(root, selector, raw, verifyTestMethodCandidate); !errors.Is(err, os.ErrExist) || second != nil {
				t.Fatalf("ASSERT_ADR0011_OWNER_TEST_NO_REPLACE: receipt=%+v err=%v", second, err)
			}
			gotAgain, err := root.ReadSelector(selector, maxTestMethodCandidateBytes)
			if err != nil || !bytes.Equal(gotAgain, raw) {
				t.Fatal("ASSERT_ADR0011_OWNER_TEST_NO_REPLACE: existing bytes changed")
			}
			t.Log("ASSERT_ADR0011_OWNER_TEST_PRIVATE_PUBLISH: PASS (candidate bytes, not producer attestation)")
		})
	}
}

func TestManagerCandidatePublicationFailureNotMethodFailure(t *testing.T) {
	root, path := testCandidateRoot(t)
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	var published *publication.BoundFileReceipt
	var publishErr error
	calls := 0
	m, started := methodCandidateManager(t, "references-empty", func(c methodCandidateObservation) {
		calls++
		raw, err := encodeTestMethodCandidate(c)
		if err != nil {
			publishErr = err
			return
		}
		published, publishErr = publication.PublishBoundFile(root, "closed-root-test.bin", raw, verifyTestMethodCandidate)
	})
	got := m.RoundTrip(context.Background(), methodCandidateRequest(started, "textDocument/references", 1))
	entries, err := os.ReadDir(path)
	if got.Failure != "" || got.ServerError != nil || calls != 1 || publishErr == nil || published != nil || err != nil || len(entries) != 0 {
		t.Fatalf("ASSERT_ADR0011_OWNER_TEST_PUBLISH_FAILURE_SEPARATE: failure=%s calls=%d publishErr=%v receipt=%+v entries=%d readErr=%v", got.Failure, calls, publishErr, published, len(entries), err)
	}
	t.Log("ASSERT_ADR0011_OWNER_TEST_PUBLISH_FAILURE_SEPARATE: PASS")
}

func TestManagerUnconfiguredHasNoCandidatePublication(t *testing.T) {
	root, path := testCandidateRoot(t)
	_ = root // A private root alone cannot configure the manager's unexported hook.
	m, started := methodCandidateManager(t, "references-empty", nil)
	got := m.RoundTrip(context.Background(), methodCandidateRequest(started, "textDocument/references", 1))
	entries, err := os.ReadDir(path)
	if got.Failure != "" || err != nil || len(entries) != 0 {
		t.Fatalf("ASSERT_ADR0011_OWNER_TEST_DEFAULT_OFF: failure=%s entries=%d err=%v", got.Failure, len(entries), err)
	}
	t.Log("ASSERT_ADR0011_OWNER_TEST_DEFAULT_OFF: PASS")
}

// A holder of the same process's test root can publish a self-consistent
// candidate without the manager publishing it. This documents the test codec's
// byte-integrity ceiling, not a production receipt or provider attestation.
func TestTestCandidateByteVerificationDoesNotAuthenticatePublisher(t *testing.T) {
	var captured []methodCandidateObservation
	m, started := methodCandidateManager(t, "references-empty", func(c methodCandidateObservation) {
		captured = append(captured, c)
	})
	got := m.RoundTrip(context.Background(), methodCandidateRequest(started, "textDocument/references", 1))
	if got.Failure != "" || len(captured) != 1 {
		t.Fatalf("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_SETUP: failure=%s callbacks=%d", got.Failure, len(captured))
	}
	original := captured[0]
	originalFrame, ok := got.CompletedMethodRequestFrame()
	const from, to = "file:///fixture/main.go", "file:///fixture/evil.go"
	if !ok || bytes.Count(originalFrame, []byte(from)) != 1 || len(from) != len(to) {
		t.Fatal("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_SETUP: expected exact one-URI frame")
	}

	forged := original
	forged.RequestFrame = bytes.Replace(originalFrame, []byte(from), []byte(to), 1)
	inconsistent, err := encodeTestMethodCandidate(forged)
	if err != nil || verifyTestMethodCandidate(inconsistent) == nil {
		t.Fatalf("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_DIGEST_CONTROL: err=%v", err)
	}
	forged.RequestWrite.FrameSHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(forged.RequestFrame))
	forgedRaw, err := encodeTestMethodCandidate(forged)
	if err != nil || verifyTestMethodCandidate(forgedRaw) != nil {
		t.Fatalf("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_SELF_CONSISTENT: encode=%v", err)
	}

	root, _ := testCandidateRoot(t)
	const selector = "same-process-forged-query-test.bin"
	published, err := publication.PublishBoundFile(root, selector, forgedRaw, verifyTestMethodCandidate)
	readback, readErr := root.ReadSelector(selector, maxTestMethodCandidateBytes)
	var decoded testMethodCandidate
	decodeErr := json.Unmarshal(readback, &decoded)
	if err != nil || published == nil || published.VerificationStatus != "VERIFIED" || readErr != nil || !bytes.Equal(readback, forgedRaw) || verifyTestMethodCandidate(readback) != nil || decodeErr != nil || !bytes.Contains(decoded.RequestFrame, []byte(to)) {
		t.Fatalf("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_READBACK: publish=%v receipt=%+v read=%v decode=%v", err, published, readErr, decodeErr)
	}
	if !bytes.Equal(original.RequestFrame, originalFrame) || bytes.Equal(originalFrame, forged.RequestFrame) {
		t.Fatal("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_MANAGER_UNCHANGED: manager frame replaced")
	}
	retained, retainedOK := got.CompletedMethodRequestFrame()
	if !retainedOK || !bytes.Equal(retained, originalFrame) {
		t.Fatal("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_MANAGER_UNCHANGED: manager observation changed")
	}
	t.Log("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_DIGEST_CONTROL: PASS (stale digest rejected)")
	t.Log("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_READBACK: PASS (forged query accepted without owner authentication)")
	t.Log("ASSERT_ADR0011_TEST_PUBLISH_FORGERY_MANAGER_UNCHANGED: PASS")
}
