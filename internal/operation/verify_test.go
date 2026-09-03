package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/verification"
)

const (
	assertVerifyCustody   = "P_VERIFY_EXPLICIT_CUSTODY_LOADER"
	assertVerifyBytes     = "P_VERIFY_EXACT_ARTIFACT_BYTES"
	assertVerifyIntegrity = "P_VERIFY_ACCEPTED_INTEGRITY_CORE"
	assertVerifyAuthority = "P_VERIFY_ACCEPTED_SEMANTIC_AUTHORITY"
	assertVerifyFailure   = "P_VERIFY_FAILURE_CLASSIFICATION"
	assertVerifyRace      = "P_VERIFY_CONCURRENT_DETERMINISM"
)

type custodyLoaderFunc func(context.Context, json.RawMessage) (CustodyMaterial, *Failure)

func (f custodyLoaderFunc) Load(ctx context.Context, input json.RawMessage) (CustodyMaterial, *Failure) {
	return f(ctx, input)
}

func validCustodyMaterial(t *testing.T) CustodyMaterial {
	t.Helper()
	artifact, err := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV3})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
	if err != nil {
		t.Fatal(err)
	}
	return CustodyMaterial{Artifact: artifact, Receipt: receipt}
}

func verifyRequest(input string) Request {
	return Request{Name: Verify, Input: json.RawMessage(`{"input":` + input + `}`)}
}

func TestVerifyHandlerUsesInjectedCustodyAndPreservesExactBytes(t *testing.T) {
	material := validCustodyMaterial(t)
	selector := json.RawMessage(`{"generation":"generation-7"}`)
	calls := 0
	loader := custodyLoaderFunc(func(_ context.Context, got json.RawMessage) (CustodyMaterial, *Failure) {
		calls++
		if !bytes.Equal(got, selector) {
			t.Fatalf("%s: input=%s want=%s", assertVerifyCustody, got, selector)
		}
		return material, nil
	})

	result, failure := NewVerifyHandler(loader)(context.Background(), verifyRequest(string(selector)))
	if failure != nil || calls != 1 {
		t.Fatalf("%s: calls=%d failure=%v", assertVerifyCustody, calls, failure)
	}
	if !bytes.Equal(result.Artifact, material.Artifact) {
		t.Fatalf("%s: artifact changed\ngot:  %q\nwant: %q", assertVerifyBytes, result.Artifact, material.Artifact)
	}
}

func TestVerifyHandlerPreservesLoaderAndCoreFailures(t *testing.T) {
	loaderErr := errors.New("pinned generation moved")
	loaderFailure := &Failure{Code: "CUSTODY_UNAVAILABLE", Diagnostics: []string{"no-follow custody failed"}, Err: loaderErr}
	result, failure := NewVerifyHandler(custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) {
		return CustodyMaterial{}, loaderFailure
	}))(context.Background(), verifyRequest(`"selector.json"`))
	if len(result.Artifact) != 0 || failure != loaderFailure || !errors.Is(failure, loaderErr) {
		t.Fatalf("%s: result=%#v failure=%#v", assertVerifyFailure, result, failure)
	}

	material := validCustodyMaterial(t)
	material.Artifact = append(append([]byte(nil), material.Artifact...), ' ')
	_, failure = NewVerifyHandler(custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) {
		return material, nil
	}))(context.Background(), verifyRequest(`"selector.json"`))
	if failure == nil || failure.Code != "VERIFICATION_FAILED" || failure.Err == nil || failure.Err.Error() != "exact-byte integrity mismatch" {
		t.Fatalf("%s: failure=%#v", assertVerifyIntegrity, failure)
	}

	material = validCustodyMaterial(t)
	material.Artifact = []byte(`{"schema_version":"lsp-trace.graph.v2"}`)
	material.Receipt, _ = verification.ReceiptBytes(material.Artifact, verification.DirectoryDurabilityChecked)
	_, failure = NewVerifyHandler(custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) {
		return material, nil
	}))(context.Background(), verifyRequest(`"selector.json"`))
	if failure == nil || failure.Code != "VERIFICATION_FAILED" || failure.Err == nil || !strings.Contains(failure.Err.Error(), "verification requires lsp-trace.graph.v3") {
		t.Fatalf("%s: failure=%#v", assertVerifyAuthority, failure)
	}
}

func TestVerifyHandlerClassifiesInvalidInputAndIsConcurrentDeterministic(t *testing.T) {
	loaderCalls := 0
	loader := custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) {
		loaderCalls++
		return validCustodyMaterial(t), nil
	})
	_, failure := NewVerifyHandler(loader)(context.Background(), Request{Name: Verify, Input: json.RawMessage(`{"verification":{}}`)})
	if failure == nil || failure.Code != FailureInvalidInput || loaderCalls != 0 {
		t.Fatalf("%s: calls=%d failure=%#v", assertVerifyFailure, loaderCalls, failure)
	}

	material := validCustodyMaterial(t)
	loader = custodyLoaderFunc(func(context.Context, json.RawMessage) (CustodyMaterial, *Failure) { return material, nil })
	handler := NewVerifyHandler(loader)
	const workers = 24
	artifacts := make([][]byte, workers)
	failures := make([]*Failure, workers)
	var wg sync.WaitGroup
	for i := range artifacts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, failure := handler(context.Background(), verifyRequest(`"selector.json"`))
			artifacts[i], failures[i] = result.Artifact, failure
		}(i)
	}
	wg.Wait()
	for i := range artifacts {
		if failures[i] != nil || !bytes.Equal(artifacts[i], material.Artifact) {
			t.Fatalf("%s: run=%d artifact=%q failure=%v", assertVerifyRace, i, artifacts[i], failures[i])
		}
	}
}
