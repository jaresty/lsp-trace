package sessionruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const privateB4QualifiedJSONStreamSHA256 = "bb26b2442db6a686fc7f823ea9cc89babe26ac988ec7d8caf4a157aeb60d92f2"

func TestPrivateB4QualifiedJSONEncoderSource(t *testing.T) {
	if runtime.Version() != privateB4QualifiedGoVersion {
		t.Fatalf("ASSERT_C15_JSON_TOOLCHAIN_VERSION got=%q want=%q", runtime.Version(), privateB4QualifiedGoVersion)
	}
	source, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src", "encoding", "json", "stream.go"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(source)
	if got := hex.EncodeToString(digest[:]); got != privateB4QualifiedJSONStreamSHA256 {
		t.Fatalf("ASSERT_C15_JSON_ENCODER_SOURCE got=%s want=%s", got, privateB4QualifiedJSONStreamSHA256)
	}
}

func TestEncodePrivateB4RequestOnePassExactBodyAndReservation(t *testing.T) {
	message := lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Method: "textDocument/definition", Params: json.RawMessage(`{"x":"<&>"}`)}
	want, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	ledger := mustPrivateB4ByteAccountV2(t)
	body, lease, failure, err := encodePrivateB4Request(message, ledger, lspwire.DefaultLimits(), nil)
	if err != nil || failure != "" {
		t.Fatalf("ASSERT_C15_JSON_ONE_PASS failure=%q err=%v", failure, err)
	}
	if string(body) != string(want) || len(body) != cap(body) {
		t.Fatalf("ASSERT_C15_JSON_EXACT_BODY len=%d cap=%d body=%q want=%q", len(body), cap(body), body, want)
	}
	wantBytes := uint64(canonicalRequestFrameBytes(len(want)))
	if got := ledger.snapshot(); got != (privateB4ByteLedgerSnapshot{Live: privateB4ByteLedgerV2TableBytes + wantBytes, Cumulative: privateB4ByteLedgerV2TableBytes + wantBytes}) {
		t.Fatalf("ASSERT_C15_JSON_EXACT_RESERVATION got=%+v want=%d", got, wantBytes)
	}
	lease.release()
	if got := ledger.snapshot().Live; got != privateB4ByteLedgerV2TableBytes {
		t.Fatalf("ASSERT_C15_JSON_RELEASE live=%d", got)
	}
}

func TestEncodePrivateB4RequestPlusOneRefusesBeforeBodyCopy(t *testing.T) {
	message := lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Method: "textDocument/definition", Params: json.RawMessage(`{"x":1}`)}
	want, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	need := uint64(canonicalRequestFrameBytes(len(want)))
	ledger := mustPrivateB4ByteAccountV2(t)
	filler, failure := ledger.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes - need + 1)
	if failure != "" {
		t.Fatal(failure)
	}
	before := ledger.snapshot()
	body, lease, failure, err := encodePrivateB4Request(message, ledger, lspwire.DefaultLimits(), nil)
	if failure != session.ResourceExhausted || err == nil || body != nil || lease != (privateB4ByteLeaseV2{}) {
		t.Fatalf("ASSERT_C15_JSON_PLUS_ONE body=%v lease=%+v failure=%q err=%v", body, lease, failure, err)
	}
	if after := ledger.snapshot(); after != before {
		t.Fatalf("ASSERT_C15_JSON_PLUS_ONE_ATOMIC before=%+v after=%+v", before, after)
	}
	filler.release()
}
