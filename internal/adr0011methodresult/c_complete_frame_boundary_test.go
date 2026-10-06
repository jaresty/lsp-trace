package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/adr0011cobserve"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// ORIGINAL FIXTURES: header spelling, decimal width, CRLF separator and body
// are held together and hashed independently of the manager's re-encoding.
const (
	cFrameCap   = 2097152
	cAtCapSHA   = "e8afbf2e8df082a638a85720717e8aa9d5d7557faf44269c432a0e901a7ae2f6"
	cOverCapSHA = "8eb1cfec2b71d2371eabdcd72a5cb3b3e0afee6e2b4ddaf294ffb8dcea3e3241"
)

func cHeldOriginal(t *testing.T, name, digest string, size int) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "adr0011-c-frames-v1", name+"."+digest))
	if err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: read original: %v", err)
	}
	h := sha256.Sum256(data)
	if len(data) != size || hex.EncodeToString(h[:]) != digest {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: original byte pin %s", name)
	}
	head, body, found := bytes.Cut(data, []byte("\r\n\r\n"))
	if !found || !bytes.HasPrefix(head, []byte("content-length:\t")) || !bytes.HasSuffix(head, []byte("\r\nX-C-Original: fixture")) || !bytes.Equal(head, []byte(fmt.Sprintf("content-length:\t%d\r\nX-C-Original: fixture", len(body)))) || len(head)+4+len(body) != size {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: original header/separator/body %s", name)
	}
	message, observed, exact, retained, err := lspwire.NewReader(bytes.NewReader(data), lspwire.DefaultLimits()).ReadWithFrameIfWithin(int64(size))
	if err != nil || !retained || message.Method != "$/progress" || observed.FrameBytes != int64(size) || !bytes.Equal(exact, data) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: valid notification preflight %s: %v", name, err)
	}
	return data
}

type cFrameDelivery struct {
	frameBytes    int
	frameError    error
	responseBytes int
	responseError error
}

func cBoundaryChild(inFrame, original, response []byte) (*composedChild, <-chan struct{}, <-chan cFrameDelivery) {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
	attempt := make(chan struct{}, 1)
	sent := make(chan cFrameDelivery, 1)
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		if err := composedServeReadiness(reader, output); err != nil {
			child.observed <- err
			return
		}
		msg, _, frame, retained, err := reader.ReadWithFrameIfWithin(4096)
		if err != nil || !retained || !bytes.Equal(frame, inFrame) || msg.Method != "textDocument/definition" || !bytes.Equal(msg.ID, []byte("1")) {
			child.observed <- fmt.Errorf("exact WRITE: %v", err)
			return
		}
		child.observed <- nil
		attempt <- struct{}{}
		delivery := cFrameDelivery{}
		delivery.frameBytes, delivery.frameError = output.Write(original)
		if delivery.frameError == nil {
			delivery.responseBytes, delivery.responseError = output.Write(response)
		}
		sent <- delivery
	}()
	return child, attempt, sent
}

// This test is a frozen contract candidate. Run it only after independent
// pre-execution review of its exact source and both held original frames.
func TestCManagerCompleteOriginalFrameBoundary(t *testing.T) {
	fixtureRoot, assets := cTrackedABAssets(t)
	// Independent A/B manager and B4 controls must pass in THIS invocation,
	// not just a separately selected go test command.
	if !t.Run("independent-A-B-controls", TestCObservedB4SuccessorEquivalenceAndOrder) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: independent A/B controls")
	}
	originals := []struct {
		name, digest string
		size         int
		over         bool
	}{
		{"at-cap", "", cFrameCap, false},
		{"plus-one", "", cFrameCap + 1, true},
	}
	for _, item := range originals {
		if !t.Run(item.name, func(t *testing.T) {
			frame := cFrameV2Original(t, item.name)
			in, occurrence, sources, wants, _, req, owner, profile := composedFixture(t, assets, fixtureRoot, "A")
			response := bridgeAssetBytes(t, fixtureRoot, "A/response.frame", assets)
			req.MaxBytes = 4 << 20 // transaction budget, NOT the per-frame admission limit
			req.MaxMessages = 2
			req.Deadline = time.Now().Add(12 * time.Second)
			child, attempt, sent := cBoundaryChild(in.Write.RequestFrame, frame, response)
			manager, started := cFixtureManager(t, profile, child, req)
			probe := &cProbe{}
			result, lease := manager.RoundTripPrivateB4(adr0011cobserve.With(context.Background(), probe.observe), req, owner)
			cExactWrite(t, child)
			select {
			case <-attempt:
			default:
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: frame delivery not attempted")
			}
			if result.RequestMessages != 1 || result.RequestBytes <= 0 || result.Failure == session.RequestTimeout || result.Failure == session.SessionPoisoned {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: request accounting/timeout/transport: %s messages=%d bytes=%d", result.Failure, result.RequestMessages, result.RequestBytes)
			}
			root, dir := cPrivateRoot(t)
			raw, _ := retentionFixture(t)
			var decision PrivateB4Decision
			var publicationError error
			continued := cContinue(result, lease, func() {
				decision = cObservedB4(probe.observe, manager, lease, cSelection(started, result, owner), in, occurrence, sources)
				if decision.Status == DefinitionBridgeCandidateItems {
					_, publicationError = publishPrivateCandidateObserved(func() { adr0011cobserve.Notify(probe.observe, adr0011cobserve.PrivateCandidatePublishEntry) }, root, "synthetic.json", raw)
				}
			})
			counts, _, overflow := probe.snapshot()
			if overflow {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: observation overflow")
			}
			if !item.over {
				var delivery cFrameDelivery
				select {
				case delivery = <-sent:
				case <-time.After(4 * time.Second):
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: full at-cap frame delivery timeout")
				}
				if delivery.frameBytes != len(frame) || delivery.frameError != nil || delivery.responseBytes != len(response) || delivery.responseError != nil || result.Failure != "" || result.ServerError != nil || lease == (sessionruntime.B4DefinitionLease{}) {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: at-cap transport/lease: delivery=%+v failure=%s lease=%v", delivery, result.Failure, lease != (sessionruntime.B4DefinitionLease{}))
				}
				notificationCount, notificationReleased := privateRetainedNotificationCount(result)
				if !continued || decision.Status != DefinitionBridgeCandidateItems || len(decision.Candidates) != len(wants) || publicationError != nil || result.Messages != 2 || len(result.Notifications) != 0 || notificationCount != 1 || !notificationReleased || counts[adr0011cobserve.DecodeEntry] != 2 || counts[adr0011cobserve.RetainEntry] != 2 || counts[adr0011cobserve.B4EvaluationEntry] != 1 || counts[adr0011cobserve.PrivateCandidatePublishEntry] != 1 {
					t.Errorf("SEMANTIC_RED_FRAME_AT_CAP: continued=%v status=%s counts=%v publish=%v", continued, decision.Status, counts, publicationError)
				}
				return
			}
			// Header rejection is allowed to stop a pipe writer before the complete
			// frame is consumed; require at least the complete original header and
			// separator to have crossed, or the entire frame on the unguarded path.
			if result.Failure != "" {
				_ = child.output.Close()
			} // unblock an early-rejected producer
			var delivery cFrameDelivery
			select {
			case delivery = <-sent:
			case <-time.After(4 * time.Second):
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: over-cap producer join timeout")
			}
			header, _, _ := bytes.Cut(frame, []byte("\r\n\r\n"))
			if delivery.frameBytes < len(header)+4 || (delivery.frameError != nil && delivery.frameBytes == len(frame)) || (delivery.frameError == nil && delivery.frameBytes != len(frame)) ||
				(delivery.frameBytes == len(frame) && (delivery.responseBytes != len(response) || delivery.responseError != nil)) ||
				result.ServerError != nil || (result.Failure != "" && result.Failure != session.ResourceExhausted) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: over-cap transport not C rejection: delivery=%+v failure=%s", delivery, result.Failure)
			}
			if result.Failure != session.ResourceExhausted || lease != (sessionruntime.B4DefinitionLease{}) || continued || counts[adr0011cobserve.DecodeEntry] != 0 || counts[adr0011cobserve.RetainEntry] != 0 || counts[adr0011cobserve.B4EvaluationEntry] != 0 || counts[adr0011cobserve.PrivateCandidatePublishEntry] != 0 || publicationError != nil {
				t.Errorf("SEMANTIC_RED_FRAME_OVER_CAP: failure=%s lease=%v continued=%v counts=%v publish=%v", result.Failure, lease != (sessionruntime.B4DefinitionLease{}), continued, counts, publicationError)
			}
			if _, err := os.Stat(filepath.Join(dir, "synthetic.json")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("SEMANTIC_RED_FRAME_OVER_CAP_PUBLICATION: %v", err)
			}
		}) && !item.over {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: at-cap prerequisite for +1")
		}
	}
}
