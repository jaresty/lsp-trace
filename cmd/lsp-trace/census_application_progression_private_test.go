//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusApplicationPrivateAdmission(t *testing.T) {
	base, _, path, _ := inventoryTwoFixturePrivate(t)
	directory := filepath.Join(base, "progression")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := json.Marshal(struct {
		Profile, Directory                                        string
		MaxPlanBytes, MaxArtifactBytes, MaxReadBytes, MaxSegments int
	}{"PRIVATE_CENSUS_APPLICATION_V1", directory, 1 << 20, 1 << 20, 64 << 20, 128})
	raw = append(append(bytes.TrimSuffix(raw, []byte("}")), []byte(`,"Application":`)...), app...)
	raw = append(raw, '}')
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = loadCensusInventoryConfigPrivate(path)
	assertInventoryPrivate(t, "ASSERT_APPLICATION_PRIVATE_PROFILE_ADMITTED", err == nil)
}
