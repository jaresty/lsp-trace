package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Golden produced independently with Python:
// hashlib.sha256(b'ADR0011_SYNTHETIC_SOURCE_SET_V1\0' +
// (len(p)).to_bytes(8,'big') + p + (len(b)).to_bytes(8,'big') + hashlib.sha256(b).digest()).hexdigest()
// where p=b'internal/adr0011acquisition/a.go', b=b'package a\n'.
func TestSyntheticSourcePin(t *testing.T) {
	root := t.TempDir()
	for dir := range syntheticSourceDirectories {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	name := "internal/adr0011acquisition/a.go"
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		t.Fatal(err)
	}
	body := []byte("package a\n")
	if err := os.WriteFile(filename, body, 0600); err != nil {
		t.Fatal(err)
	}
	entry := SyntheticSourceEntry{Path: name, ByteLength: uint64(len(body)), SHA256: sha256.Sum256(body)}
	// Independent byte encoding (not the verifier's helper or digest routine).
	packet := append([]byte("ADR0011_SYNTHETIC_SOURCE_SET_V1\x00"), make([]byte, 8)...)
	packet[len(packet)-1] = byte(len(name))
	packet = append(packet, name...)
	packet = append(packet, make([]byte, 8)...)
	packet[len(packet)-1] = byte(len(body))
	packet = append(packet, entry.SHA256[:]...)
	golden := sha256.Sum256(packet)
	if err := verifySyntheticSourcePinAt(root, []SyntheticSourceEntry{entry}, golden); err != nil {
		t.Fatalf("golden: %v", err)
	}
	if got := hex.EncodeToString(golden[:]); got != "632b59cd0308c38701b6f51fa2f3fa6e51268a29e8baf68606339491128a0918" {
		t.Fatalf("independent golden mismatch: %s", got)
	}
	assertReject := func(label string, entries []SyntheticSourceEntry, digest [32]byte) {
		t.Helper()
		err := verifySyntheticSourcePinAt(root, entries, digest)
		if err == nil || err != errSyntheticSourcePin || strings.Contains(err.Error(), "package a") {
			t.Fatalf("%s: unsafe result %v", label, err)
		}
	}
	wrong := golden
	wrong[0] ^= 1
	assertReject("aggregate", []SyntheticSourceEntry{entry}, wrong)
	omitted := filepath.Join(root, "internal/adr0011acquisition/omitted.go")
	if err := os.WriteFile(omitted, body, 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("omitted", []SyntheticSourceEntry{entry}, golden)
	if err := os.Remove(omitted); err != nil {
		t.Fatal(err)
	}
	altered := entry
	altered.SHA256[0] ^= 1
	assertReject("hash", []SyntheticSourceEntry{altered}, golden)
	altered = entry
	altered.ByteLength--
	assertReject("length", []SyntheticSourceEntry{altered}, golden)
	altered = entry
	altered.Path = "internal/adr0011acquisition/../a.go"
	assertReject("traversal", []SyntheticSourceEntry{altered}, golden)
	altered = entry
	altered.Path = "internal/adr0011acquisition/missing.go"
	assertReject("absent", []SyntheticSourceEntry{altered}, golden)
	assertReject("duplicate", []SyntheticSourceEntry{entry, entry}, golden)
	second := entry
	second.Path = "internal/adr0011acquisition/0.go"
	assertReject("unsorted", []SyntheticSourceEntry{entry, second}, golden)
	if err := os.WriteFile(filename, []byte("package b\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("changed file", []SyntheticSourceEntry{entry}, golden)
	replacement := filepath.Join(root, "replacement.tmp")
	if err := os.WriteFile(replacement, []byte("package c\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filename); err != nil {
		t.Fatal(err)
	}
	assertReject("swapped inode", []SyntheticSourceEntry{entry}, golden)
	if err := os.WriteFile(filename, body[:len(body)-1], 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("truncated", []SyntheticSourceEntry{entry}, golden)
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.go"), filename); err != nil {
		t.Fatal(err)
	}
	assertReject("symlink", []SyntheticSourceEntry{entry}, golden)
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, bytes.Repeat([]byte("x"), syntheticSourceFileLimit+1), 0600); err != nil {
		t.Fatal(err)
	}
	assertReject("overlimit", []SyntheticSourceEntry{entry}, golden)
	tooLarge := entry
	tooLarge.ByteLength = syntheticSourceFileLimit + 1
	assertReject("manifest overlimit", []SyntheticSourceEntry{tooLarge}, golden)
}
