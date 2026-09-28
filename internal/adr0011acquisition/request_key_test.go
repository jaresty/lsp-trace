package adr0011acquisition

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

func TestOriginalRequestKeyFromCapturedBodies(t *testing.T) {
	frame := func(body string) []byte { return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)) }
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	pair := sessionruntime.OwnedMethodPair{SessionID: "owned", Generation: 7, Key: key, Write: sessionruntime.RequestWriteObservation{SessionID: "owned", Generation: 7, Key: key}, Read: sessionruntime.ResponseReadObservation{SessionID: "owned", Generation: 7, Key: key}}
	request := `{"jsonrpc":"2.0","id":31,"method":"textDocument/documentSymbol","params":{}}`
	response := `{"jsonrpc":"2.0","id":31,"result":[]}`
	pair.Source = &sessionruntime.OwnedDocumentBinding{URI: "file:///owned.go", Version: 1, SHA256: "sha256:source"}
	pair.Write.FrameBytes = int64(len(frame(request)))
	pair.Read.FrameBytes = int64(len(frame(response)))
	pair.Write.FrameSHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(frame(request)))
	pair.Read.FrameSHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(frame(response)))
	frames := ownedFrames{request: frame(request), response: frame(response), pair: pair, invocation: "symbol"}
	valid := func(f ownedFrames, p sessionruntime.OwnedMethodPair, session string, gen uint64, invocation, expected, digest string) bool {
		f.invocation = invocation
		return verifyOriginalRequestKey(f, p, session, gen, expected, digest)
	}
	if !valid(frames, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest) {
		t.Fatal("ASSERT_ORIGINAL_KEY_CAPTURED_PASS")
	}
	for name, mutation := range map[string]func() bool{
		"missing-request": func() bool {
			f := frames
			f.request = nil
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"string": func() bool {
			f := frames
			f.request = frame(`{"jsonrpc":"2.0","id":"31","method":"textDocument/documentSymbol","params":{}}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"float": func() bool {
			f := frames
			f.request = frame(`{"id":31.0}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"sign": func() bool {
			f := frames
			f.request = frame(`{"id":+31}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"leading-zero": func() bool {
			f := frames
			f.request = frame(`{"id":031}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"null": func() bool {
			f := frames
			f.request = frame(`{"id":null}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"overflow": func() bool {
			f := frames
			f.request = frame(`{"id":18446744073709551616}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"response": func() bool {
			f := frames
			f.response = frame(`{"id":32}`)
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"runtime-key": func() bool {
			f := frames
			f.pair.Key.ID = 32
			return valid(f, pair, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"generation": func() bool { return valid(frames, pair, "owned", 8, "symbol", "symbol", reviewedSuccessorSchemaDigest) },
		"owner-wire": func() bool {
			p := pair
			p.Write.Key.ID = 32
			return valid(frames, p, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"source-key": func() bool {
			p := pair
			p.Key.ID = 32
			return valid(frames, p, "owned", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
		"invocation": func() bool {
			return valid(frames, pair, "owned", 7, "references", "symbol", reviewedSuccessorSchemaDigest)
		},
		"missing-invocation": func() bool {
			return valid(frames, pair, "owned", 7, "", "", reviewedSuccessorSchemaDigest)
		},
		"swapped-captured-invocation": func() bool {
			f := frames
			f.invocation = "other"
			return verifyOriginalRequestKey(f, pair, "owned", 7, "symbol", reviewedSuccessorSchemaDigest)
		},
		"digest": func() bool { return valid(frames, pair, "owned", 7, "symbol", "symbol", "claimant-selected") },
		"session": func() bool {
			return valid(frames, pair, "different", 7, "symbol", "symbol", reviewedSuccessorSchemaDigest)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if mutation() {
				t.Fatalf("ASSERT_ORIGINAL_KEY_REJECT_%s", name)
			}
		})
	}
}
