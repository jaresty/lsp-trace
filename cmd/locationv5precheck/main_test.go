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
