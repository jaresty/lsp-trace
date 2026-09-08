package seedbinding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestExternalValidatorFixture(t *testing.T) {
	source := []byte("class C { void Target() {} }\n")
	sum := sha256.Sum256(source)
	identity := ValidatorIdentity{Language: "csharp", Authority: "HOST_CONFIG", Name: "fake-csharp", Version: "1"}
	manifest := Manifest{SchemaVersion: VersionV2, ID: "seed", Locator: Locator{URI: "file:///workspace/C.cs", Line: 0, Character: 15, Encoding: "utf-8"}, ExpectedSymbol: "Target", ExpectedDeclaringFile: "C.cs", ExpectedDeclarationRange: Range{StartLine: 0, StartCharacter: 10, EndLine: 0, EndCharacter: 26}, SourceRevision: "rev", SourceSHA256: hex.EncodeToString(sum[:]), Validator: identity}
	for _, tc := range []struct {
		name, mode string
		want       Status
	}{
		{"match", "match", Match}, {"mismatch", "mismatch", Mismatch}, {"unavailable", "unavailable", Unavailable}, {"invalid-extra", "extra", Invalid}, {"invalid-contradictory", "contradict", Invalid}, {"oversized", "oversized", Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := NewExternalValidator(ExternalConfig{Protocol: ExternalProtocolV1, Identity: identity, Executable: os.Args[0], Arguments: []string{"-test.run=^TestExternalValidatorHelper$", "--", tc.mode}, Directory: t.TempDir(), TimeoutMillis: 10000, RequestBytes: 1 << 20, ResponseBytes: 2048})
			if err != nil {
				t.Fatal(err)
			}
			got := v.Validate(ValidationInput{Manifest: manifest, Workspace: "/workspace", Source: source})
			if got.Status != tc.want {
				t.Fatalf("ASSERT_EXTERNAL_VALIDATOR_%s: got=%+v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), got)
			}
		})
	}
}

func TestExternalValidatorHelper(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "--" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	var req externalRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		os.Exit(3)
	}
	if mode == "unavailable" {
		os.Exit(4)
	}
	if mode == "oversized" {
		_, _ = io.WriteString(os.Stdout, strings.Repeat("x", 4096))
		return
	}
	response := externalResponse{Protocol: ExternalProtocolV1, Validator: req.Manifest.Validator, Status: Match, DeclarationName: req.Manifest.ExpectedSymbol, DeclaringFile: req.Manifest.ExpectedDeclaringFile, NameRange: Range{StartLine: 0, StartCharacter: 15, EndLine: 0, EndCharacter: 21}, FullRange: req.Manifest.ExpectedDeclarationRange, ObservedSHA256: req.ObservedSHA256, ObservedRevision: req.SourceRevision}
	if mode == "mismatch" {
		response.Status = Mismatch
	}
	if mode == "contradict" {
		response.ObservedRevision = "other"
	}
	encoded, _ := json.Marshal(response)
	if mode == "extra" {
		encoded = append(encoded[:len(encoded)-1], []byte(`,"extra":true}`)...)
	}
	fmt.Println(string(encoded))
	os.Exit(0)
}
