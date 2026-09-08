package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/verification"
)

func retainedCallsV2Material(t *testing.T) CustodyMaterial {
	t.Helper()
	input, err := os.ReadFile(filepath.Join("..", "hydratedevidence", "testdata", "focused-fr20.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := retainedcalls.ExportV2(input)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
	if err != nil {
		t.Fatal(err)
	}
	return CustodyMaterial{Artifact: artifact, Receipt: receipt}
}

func retainedCallsV2VerifyRequest(family, version string) Request {
	input, _ := json.Marshal(map[string]any{
		"input":  "retained-v2.json",
		"schema": map[string]any{"family": family, "version": version},
	})
	return Request{Name: VerifyRetainedCallsV2, Input: input}
}

func TestVerifyRetainedCallsV2SuccessAndWrongFamily(t *testing.T) {
	material := retainedCallsV2Material(t)
	calls := 0
	loader := custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) {
		calls++
		return material, nil
	})
	result, failure := NewVerifyRetainedCallsV2Handler(loader)(context.Background(), retainedCallsV2VerifyRequest(retainedcalls.Family, "v2"))
	if failure != nil || calls != 1 || !bytes.Equal(result.Artifact, material.Artifact) {
		t.Fatalf("ASSERT_RETAINED_CALLS_V2_VERIFY_SUCCESS: calls=%d failure=%v", calls, failure)
	}
	_, failure = NewVerifyRetainedCallsV2Handler(loader)(context.Background(), retainedCallsV2VerifyRequest("graph-provenance", "v2"))
	if failure == nil || failure.Code != "INPUT_FAMILY_MISMATCH" || calls != 1 {
		t.Fatalf("ASSERT_RETAINED_CALLS_V2_VERIFY_WRONG_FAMILY: calls=%d failure=%v", calls, failure)
	}
}

func TestVerifyRetainedCallsV2RejectsCorruptPublicationBeforeAdmission(t *testing.T) {
	material := retainedCallsV2Material(t)
	material.Artifact = append([]byte(nil), material.Artifact...)
	material.Artifact[0] ^= 1
	_, failure := NewVerifyRetainedCallsV2Handler(custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) {
		return material, nil
	}))(context.Background(), retainedCallsV2VerifyRequest(retainedcalls.Family, "v2"))
	if failure == nil || failure.Code != "INPUT_INVALID" || failure.Err == nil || !strings.Contains(failure.Err.Error(), "exact-byte integrity mismatch") {
		t.Fatalf("ASSERT_RETAINED_CALLS_V2_VERIFY_CORRUPT_PUBLICATION: failure=%#v", failure)
	}
}
