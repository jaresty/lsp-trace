package adr0011genericv5proposal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Asset hashes and lengths are independently pinned literals; never take an
// expected digest or classification from a claimant envelope.
func b4bHeldSuccessorAsset(t *testing.T, dir, name string, length int, digest string) []byte {
	t.Helper()
	p := filepath.Join("testdata", dir, name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("held %s: %v", p, err)
	}
	h := sha256.Sum256(b)
	if len(b) != length || hex.EncodeToString(h[:]) != digest {
		t.Fatalf("held %s pin mismatch: length %d hash %x", p, len(b), h)
	}
	return b
}

func b4bHeldSuccessorFixture(t *testing.T) B4bHeldSuccessorInput {
	t.Helper()
	const stage1 = "adr0011-b4b-held-synthetic-originals"
	const stage1b = "adr0011-b4b-held-query-synthetic-v1"
	const stage2a = "adr0011-b4b-capability-derived-v1"
	const stage2b = "adr0011-b4b-b4a-transaction-derived-v1"
	b4bHeldSuccessorAsset(t, stage1, "manifest.json", 2566, "9d83ad7c3d50358318347cb5429637cf1e73af574b3d2b6b7040272851465e9f")
	b4bHeldSuccessorAsset(t, stage1b, "manifest.json", 3239, "9c8df20103872f1f8f9653496674b4e07339c383369e960d83537f09135a1424")
	b4bHeldSuccessorAsset(t, stage2a, "manifest.json", 1331, "724f4e19203bac69419289d6ec61b1fa211cf4ae617ef33cec3bedd6e734caa1")
	b4bHeldSuccessorAsset(t, stage2b, "manifest.json", 2040, "0be56c759e2faf154c40e6cabce1952afaed41ca68c780d4ffe5e556b53f348b")
	load := func(dir, name string, n int, h string) []byte { return b4bHeldSuccessorAsset(t, dir, name, n, h) }
	names := []string{"00-initialize-request.frame", "01-initialize-response.frame", "02-initialized.frame", "03-register-request.frame", "04-register-response.frame", "05-references-write.frame"}
	lengths := []int{137, 75, 74, 209, 62, 207}
	hashes := []string{"5ee617aa7efa8cb1ad34e6d170be201f02f612681b4b7973fea53d5fefb3493f", "7ac854e3d04180081bede182be1f7aae87451541bcbe6c7dbb758643806e0730", "3323265525f25974ce4cac579cc2fbc1674e11000d02aaa2f88cb5f88bc18f62", "fb902a3fe0804127b305745e079214c23483d11277141d32521869e9f017d2a5", "d9837db167818799d86feb8cba9a998322ae9cc2f97d83bfade94f02e3b47fd9", "acdb0346ecb1889439836d05fc8e6c20dc9d69ebbe0f494a975a21da8f41a12e"}
	directions := []string{"CLIENT_TO_SERVER", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "CLIENT_TO_SERVER"}
	frames := make([]B4bHeldSuccessorFrame, 6)
	for i := range frames {
		frames[i] = B4bHeldSuccessorFrame{uint64(i), directions[i], load(stage1, names[i], lengths[i], hashes[i])}
	}
	selector := load(stage1, "client-selector.json", 19, "844261003650303449410591b51cd4ea076082fe47d6330e9873cb10aec16c57")
	source := load(stage1b, "source.bytes", 5, "8ed3f6ad685b959ead7022518e1af76cd816f8e8ec7ccdda1ed4018e8f2223f8")
	query := load(stage1b, "query.original", 379, "b91bec39ad766bf2e8dfc7cf38cd93683c79ed82ec696b10b9268a036300845c")
	app := load(stage1b, "query-applicability.original", 448, "60a571041dac82ca4952cfc6863af6e9ea380c16b27c2e69848b18b7c14cd8b8")
	capOriginal := load(stage2a, "capability-artifact.bin", 1829, "d0d36273d838e9546ff8ceb9cef05d3549f3b4385892fa8b28ffd0ecdaaa951c")
	capEnvelope := load(stage2a, "capability-envelope.json", 4335, "a946485f4ef447d44b04655846737ff5022596c063082f9e2428f6f24696dd80")
	claims := []struct {
		role, name         string
		n                  int
		hash               string
		original, artifact []byte
	}{
		{"SOURCE", "source-envelope.json", 584, "6ab4df1e27d454e2f88331f73c6e7ccaa835058878222febe03a2c9f68539d59", source, load(stage2b, "source-artifact.bin", 275, "01a2e62768010b43cb6f4d7e108a8496b57e04ebdc41adfa7832f844dcb1a948")},
		{"QUERY", "query-envelope.json", 1157, "e81f44dac4371d5cb28b51e813a60e314e4fb684e8c00ee95ec8a0a8e6662e2e", query, query},
		{"QUERY_APPLICABILITY", "query-applicability-envelope.json", 1455, "1dc9112f6c662ef44cf19eca336d7a6f81817738ed993cb7aa6d42583ea0e599", app, app},
		{"REQUEST_WRITE", "request-write-envelope.json", 1680, "54379d35bef356539590c95b9150db79db1c732753f91139795301d7e4726bbe", frames[5].Bytes, frames[5].Bytes},
	}
	in := B4aInput{Session: "s", Generation: 1, Transaction: "tx", Workspace: "file:///w", Method: "textDocument/references", URI: "file:///w/a.go", Version: "buffer:v1", Encoding: "utf-16", Source: source, QueryOriginal: query, ApplicabilityOriginal: app, Custody: "OWNER_BUFFER", RequestFrame: frames[5].Bytes, RequestID: []byte("3"), CompletedKey: "key-1", CompletedOrdinal: 5, WriteCompleted: true}
	request, err := b4aFrame(frames[5].Bytes)
	if err != nil {
		t.Fatal(err)
	}
	in.RequestParams = append([]byte(nil), request["params"]...)
	if len(in.RequestParams) != 115 {
		t.Fatalf("held params length %d", len(in.RequestParams))
	}
	h := sha256.Sum256(in.RequestParams)
	if hex.EncodeToString(h[:]) != "cc218fae12bc05a595cdee533a29db7ff271a40d5842681946d7123aa22ad152" {
		t.Fatalf("held params digest %x", h)
	}
	for _, c := range claims {
		envelope := load(stage2b, c.name, c.n, c.hash)
		if err := A4Validate(c.role, envelope); err != nil {
			t.Fatalf("A4 %s: %v", c.role, err)
		}
		in.Claims = append(in.Claims, B4aClaim{Role: c.role, Envelope: envelope, Original: c.original, Artifact: c.artifact})
	}
	if err := A4Validate("CAPABILITY_EVENTS", capEnvelope); err != nil {
		t.Fatalf("A4 capability: %v", err)
	}
	if err := CheckB4a(in); err != nil {
		t.Fatalf("held B4a: %v", err)
	}
	return B4bHeldSuccessorInput{Write: in, CapabilityEnvelope: capEnvelope, CapabilityOriginal: capOriginal, Frames: frames, ClientSelector: selector}
}

func TestB4bHeldSuccessorPositive(t *testing.T) {
	in := b4bHeldSuccessorFixture(t)
	if got := CheckB4bHeldSuccessor(in); got != "SUPPORTED" {
		t.Fatalf("claimed null with held file selector after successful ack: %s want SUPPORTED", got)
	}
}

// Mutate a complete L-P field while preserving valid CAP shape and its descriptor.
// Neither the independently held stream nor the pinned B4a write is changed.
func b4bHeldSuccessorClaimField(t *testing.T, in *B4bHeldSuccessorInput, index int, value []byte) {
	t.Helper()
	var fields [][]byte
	for raw := in.CapabilityOriginal; len(raw) > 0; {
		if len(raw) < 8 {
			t.Fatal("truncated held artifact")
		}
		n := binary.BigEndian.Uint64(raw[:8])
		raw = raw[8:]
		if n > uint64(len(raw)) {
			t.Fatal("truncated held field")
		}
		fields = append(fields, append([]byte(nil), raw[:int(n)]...))
		raw = raw[int(n):]
	}
	if index < 0 || index > len(fields) {
		t.Fatalf("field index %d outside %d", index, len(fields))
	}
	if index == len(fields) {
		fields = append(fields, value)
	} else if value == nil {
		fields = append(fields[:index], fields[index+1:]...)
	} else {
		fields[index] = value
	}
	var artifact []byte
	for _, field := range fields {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		artifact = append(artifact, size[:]...)
		artifact = append(artifact, field...)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(in.CapabilityEnvelope, &envelope); err != nil {
		t.Fatal(err)
	}
	var descriptor map[string]any
	if err := json.Unmarshal(envelope["original"], &descriptor); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(artifact)
	descriptor["length"] = len(artifact)
	descriptor["sha256"] = "sha256:" + hex.EncodeToString(digest[:])
	var err error
	envelope["original"], err = json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	in.CapabilityEnvelope, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	in.CapabilityOriginal = artifact
	if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		t.Fatalf("mutated CAP shape: %v", err)
	}
}

func TestB4bHeldSuccessorNonFramePreimage(t *testing.T) {
	in := b4bHeldSuccessorFixture(t)
	b4bHeldSuccessorClaimField(t, &in, 1, []byte("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	if got := CheckB4bHeldSuccessor(in); got != "UNKNOWN" {
		t.Fatalf("changed tx selector: %s want UNKNOWN", got)
	}
}

func TestB4bHeldSuccessorExactArtifactFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index int
		value []byte
	}{
		{"target ordinal", 2, []byte("6")},
		{"exchange count", 3, []byte("1")},
		{"swapped request direction", 4, []byte("SERVER_TO_CLIENT")},
		{"typed numeric ID changed to string", 6, []byte("string:1")},
		{"response status", 13, []byte("ERROR")},
		{"initialized direction", 24, []byte("SERVER_TO_CLIENT")},
		{"omitted observed count", 27, nil},
		{"extra observed field", 43, []byte("CLIENT_TO_SERVER")},
		{"truncated observed frame", 42, []byte("Content-Length: 1\\r\\n\\r\\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bHeldSuccessorFixture(t)
			b4bHeldSuccessorClaimField(t, &in, tc.index, tc.value)
			if got := CheckB4bHeldSuccessor(in); got != "UNKNOWN" {
				t.Fatalf("%s: %s want UNKNOWN", tc.name, got)
			}
		})
	}
}

func TestB4bHeldSuccessorValidatesAllSchemesBeforeMatch(t *testing.T) {
	in := b4bHeldSuccessorFixture(t)
	for _, tc := range []struct{ name, selector, want string }{
		{"match then empty", `[{"scheme":"file"},{"scheme":""}]`, "MALFORMED"},
		{"match then invalid lexical", `[{"scheme":"file"},{"scheme":"1bad"}]`, "MALFORMED"},
		{"match then null", `[{"scheme":"file"},{"scheme":null}]`, "MALFORMED"},
		{"match then omitted", `[{"scheme":"file"},{}]`, "MALFORMED"},
		{"match then non-string", `[{"scheme":"file"},{"scheme":4}]`, "MALFORMED"},
		{"unknown extension", `[{"scheme":"file","future":true}]`, "SUPPORTED"},
		{"case insensitive scheme", `[{"scheme":"FILE"}]`, "SUPPORTED"},
		{"empty selector", `[]`, "MALFORMED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := in
			x.ClientSelector = []byte(tc.selector)
			if got := CheckB4bHeldSuccessor(x); got != tc.want {
				t.Errorf("%s want %s", got, tc.want)
			}
		})
	}
}

func TestB4bHeldSuccessorBoundaries(t *testing.T) {
	in := b4bHeldSuccessorFixture(t)
	for _, tc := range []struct {
		name, want string
		change     func(*B4bHeldSuccessorInput)
	}{
		{"missing selector", "UNKNOWN", func(x *B4bHeldSuccessorInput) { x.ClientSelector = nil }},
		{"scheme mismatch", "UNSUPPORTED", func(x *B4bHeldSuccessorInput) { x.ClientSelector = []byte(`[{"scheme":"untitled"}]`) }},
		{"malformed selector", "MALFORMED", func(x *B4bHeldSuccessorInput) { x.ClientSelector = []byte(`{"scheme":3}`) }},
		{"pending ack", "UNKNOWN", func(x *B4bHeldSuccessorInput) { x.Frames = x.Frames[:4] }},
		{"claimant frame mismatch", "UNKNOWN", func(x *B4bHeldSuccessorInput) {
			x.CapabilityOriginal = append([]byte(nil), x.CapabilityOriginal...)
			x.CapabilityOriginal[0] ^= 1
		}},
		{"held registration mismatch", "UNKNOWN", func(x *B4bHeldSuccessorInput) {
			x.Frames = append([]B4bHeldSuccessorFrame(nil), x.Frames...)
			x.Frames[3].Bytes = append([]byte(nil), x.Frames[3].Bytes...)
			x.Frames[3].Bytes = bytes.Replace(x.Frames[3].Bytes, []byte("reg-1"), []byte("reg-2"), 1)
		}},
		{"malformed selector before pending ack", "MALFORMED", func(x *B4bHeldSuccessorInput) {
			x.ClientSelector = []byte(`{`)
			x.Frames = x.Frames[:4]
		}},
		{"held numeric request ID changed", "UNKNOWN", func(x *B4bHeldSuccessorInput) {
			x.Write.RequestID = []byte(`"3"`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := in
			tc.change(&x)
			if got := CheckB4bHeldSuccessor(x); got != tc.want {
				t.Errorf("%s want %s", got, tc.want)
			}
		})
	}
}
