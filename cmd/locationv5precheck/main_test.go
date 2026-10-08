package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSameFilesDetectsBeforePhysicalSelfImageMismatch(t *testing.T) {
	zero := []rec{{Path: "FREEZE.json", Bytes: 0, SHA256: digest(nil)}}
	physical := []rec{{Path: "FREEZE.json", Bytes: 123, SHA256: digest([]byte("physical"))}}
	if sameFiles(zero, physical) {
		t.Fatalf("expected preserved before manifest physical-self-image mismatch to be detectable")
	}
}

func strictFreezeBytes(t *testing.T, raw string) error {
	t.Helper()
	p := filepath.Join(t.TempDir(), "FREEZE.json")
	if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	var f freezeFile
	return strict(p, &f)
}

func validFreezeJSON() string {
	return `{"schema":"s","status":"IMPLEMENTATION_AGREEMENT_GO","design":{"accepted":false,"authority":0,"completeness":"UNKNOWN","dispatch":false,"execution":"designGO","featureIdentity":"UNRESOLVED","immutablePredecessors":["v1"],"qualificationExecution":false},"custody":{"evaluatorSource":"e","independence":"procedural-not-cryptographic","inputBase":"b","inputCorrectionA":"a","inputCorrectionB":"b","integration":"i","mergeBase":"m","oracleSource":"o","skippedPatchEquivalent":"p","spec":"s"},"counts":{"boundaryBundles":4,"evaluatorResults":26,"files":230,"inputCases":26,"oracleDerivations":26,"oracleResults":26},"files":[],"rootIdentity":"r"}`
}

func strictBytes(t *testing.T, name, raw string, dst any) error {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return strict(p, dst)
}

func TestAuthorizationAndAssignmentsStrictCompleteSchemas(t *testing.T) {
	auth := `{"accepted":false,"attemptID":"attempt-01","authorityCeiling":0,"authorizedFreezeRootIdentity":"r","baseCommit":"c","completeness":"UNKNOWN","executionRoot":"e","externalInference":false,"featureIdentity":"UNRESOLVED","freezeRoot":"f","frozenFilesImmutable":230,"headAuthorityCeiling":0,"noPublicProductionReleasePush":true,"phaseCommitRequiredBeforeAttempts":true,"producerAssignments":26,"reviewerAssignments":26,"schema":"s","status":"AUTHORIZED"}`
	assign := `{"attemptID":"attempt-01","cases":[{"caseID":"01","producer":{"assignmentID":"p","attemptID":"attempt-01","forbidden":["oracle"],"mayRead":["input"],"role":"producer"},"reviewer":{"assignmentID":"r","attemptID":"attempt-01","forbidden":[],"mayRead":["producer"],"role":"reviewer"}}],"schema":"s"}`
	if err := strictBytes(t, "AUTHORIZATION.json", auth, &authFile{}); err != nil {
		t.Fatalf("complete authorization rejected: %v", err)
	}
	if err := strictBytes(t, "ASSIGNMENTS.json", assign, &assignmentFile{}); err != nil {
		t.Fatalf("complete assignments rejected: %v", err)
	}
	for i, tc := range []struct {
		raw string
		dst any
	}{
		{strings.Replace(auth, `"baseCommit":`, `"unknown":1,"baseCommit":`, 1), &authFile{}},
		{strings.Replace(auth, `"schema":"s"`, `"schema":"s","schema":"duplicate"`, 1), &authFile{}},
		{auth + `[]`, &authFile{}},
		{strings.Replace(assign, `"caseID":`, `"unknownCase":1,"caseID":`, 1), &assignmentFile{}},
		{strings.Replace(assign, `"assignmentID":"p"`, `"unknownRole":1,"assignmentID":"p"`, 1), &assignmentFile{}},
		{strings.Replace(assign, `"caseID":"01"`, `"caseID":"01","caseID":"duplicate"`, 1), &assignmentFile{}},
		{assign + `[]`, &assignmentFile{}},
	} {
		if err := strictBytes(t, "mutation.json", tc.raw, tc.dst); err == nil {
			t.Fatalf("mutation %d accepted", i)
		}
	}
}

func TestFreezeStrictSchemaAcceptsExactAndRejectsMutations(t *testing.T) {
	valid := validFreezeJSON()
	if err := strictFreezeBytes(t, valid); err != nil {
		t.Fatalf("valid freeze rejected: %v", err)
	}
	mutations := []string{
		strings.Replace(valid, `"schema":"s"`, `"schema":"s","schema":"duplicate"`, 1),
		strings.Replace(valid, `"status":`, `"extra":1,"status":`, 1),
		strings.Replace(valid, `"accepted":`, `"unknownDesign":1,"accepted":`, 1),
		strings.Replace(valid, `"evaluatorSource":`, `"unknownCustody":1,"evaluatorSource":`, 1),
		strings.Replace(valid, `"boundaryBundles":`, `"unknownCounts":1,"boundaryBundles":`, 1),
		valid + `[]`,
	}
	for i, raw := range mutations {
		if err := strictFreezeBytes(t, raw); err == nil {
			t.Fatalf("mutation %d accepted", i)
		}
	}
}
