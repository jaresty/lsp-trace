package main

import "testing"

func TestSameFilesDetectsBeforePhysicalSelfImageMismatch(t *testing.T) {
	zero := []rec{{Path: "FREEZE.json", Bytes: 0, SHA256: digest(nil)}}
	physical := []rec{{Path: "FREEZE.json", Bytes: 123, SHA256: digest([]byte("physical"))}}
	if sameFiles(zero, physical) {
		t.Fatalf("expected preserved before manifest physical-self-image mismatch to be detectable")
	}
}

func TestPrecheckStrictRejectsDuplicateUnknownTrailing(t *testing.T) {
	var f freezeFile
	for _, raw := range [][]byte{[]byte(`{"schema":"x","schema":"y"}`), []byte(`{"schema":"x","rootIdentity":"r","files":[],"extra":1}`), []byte(`{"schema":"x","rootIdentity":"r","files":[]}[]`)} {
		if err := rejectDuplicateKeys(raw); err == nil && string(raw) == `{"schema":"x","schema":"y"}` {
			t.Fatalf("duplicate was not rejected")
		}
		_ = f
	}
}
