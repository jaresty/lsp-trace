package adr0011genericv2proposal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// An independent, test-only recomputation of the unaccepted SOURCE preimage.
// It is deliberately not imported by adr0011generic or any runtime entrypoint.
type sourceCandidate struct {
	Identity   []json.RawMessage `json:"identity"`
	URIHex     string            `json:"uri_hex"`
	VersionHex *string           `json:"version_hex"`
	ContentHex *string           `json:"content_hex"`
	Custody    string            `json:"custody"`
}

type sourceVectorFile struct {
	Status         string                     `json:"status"`
	Cases          map[string]sourceCandidate `json:"cases"`
	RejectedInputs map[string]sourceCandidate `json:"rejected_inputs"`
	Selectors      map[string]string          `json:"selectors"`
	Pins           map[string]struct {
		Length int    `json:"length"`
		SHA256 string `json:"sha256"`
	} `json:"pins"`
}

func candidateHash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func candidateLP(b []byte) []byte {
	out := make([]byte, 8+len(b))
	binary.BigEndian.PutUint64(out, uint64(len(b)))
	copy(out[8:], b)
	return out
}
func candidatePreimage(fields ...string) []byte {
	var b []byte
	for _, f := range fields {
		b = append(b, candidateLP([]byte(f))...)
	}
	return b
}
func candidateIdentity(entry sourceCandidate) (string, uint64, string, error) {
	if len(entry.Identity) != 3 {
		return "", 0, "", fmt.Errorf("transaction identity fields")
	}
	var session, transaction string
	var generation uint64
	if err := json.Unmarshal(entry.Identity[0], &session); err != nil {
		return "", 0, "", err
	}
	if err := json.Unmarshal(entry.Identity[1], &generation); err != nil {
		return "", 0, "", err
	}
	if err := json.Unmarshal(entry.Identity[2], &transaction); err != nil {
		return "", 0, "", err
	}
	if session == "" || generation == 0 || transaction == "" {
		return "", 0, "", fmt.Errorf("empty transaction identity")
	}
	return session, generation, transaction, nil
}
func candidateSelector(method string, entry sourceCandidate, schema, transport, policy []byte) (string, error) {
	if entry.VersionHex == nil || entry.ContentHex == nil {
		return "", fmt.Errorf("absent original")
	}
	uri, err := hex.DecodeString(entry.URIHex)
	if err != nil {
		return "", err
	}
	version, err := hex.DecodeString(*entry.VersionHex)
	if err != nil {
		return "", err
	}
	content, err := hex.DecodeString(*entry.ContentHex)
	if err != nil {
		return "", err
	}
	if len(uri) == 0 || content == nil || version == nil {
		return "", fmt.Errorf("absent original")
	}
	if !utf8.Valid(uri) || !utf8.Valid(version) {
		return "", fmt.Errorf("invalid UTF-8 URI/version")
	}
	switch entry.Custody {
	case "OWNER_BUFFER", "MANAGED_VIRTUAL", "CLEAN_REGISTERED_WORKTREE", "IMMUTABLE_SOURCE_SNAPSHOT":
	default:
		return "", fmt.Errorf("unknown custody")
	}
	session, generation, transaction, err := candidateIdentity(entry)
	if err != nil {
		return "", err
	}
	tx := "sha256:" + candidateHash(candidatePreimage("ADR0011-GENERIC-TRANSACTION/1", session, strconv.FormatUint(generation, 10), transaction))
	artifact := bytes.Join([][]byte{candidateLP([]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1")), candidateLP([]byte(tx)), candidateLP(uri), candidateLP([]byte("present")), candidateLP(version), candidateLP([]byte(strconv.Itoa(len(content)))), candidateLP([]byte(candidateHash(content))), candidateLP([]byte(entry.Custody))}, nil)
	profile := "GENERIC_LSP_" + strings.ToUpper(method) + "_EXACT_V2"
	outer := candidatePreimage("ADR0011-GENERIC-EXACT/2", profile, "textDocument/"+method, "SOURCE", candidateHash(schema), candidateHash(transport), candidateHash(policy), session, strconv.FormatUint(generation, 10), candidateHash(artifact), tx)
	return "sha256:" + candidateHash(outer), nil
}

func TestProposedSourceSelectorVectorsIndependentRecomputation(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "qualification", "originals")
	raw, err := os.ReadFile(filepath.Join(root, "generic-lsp-source-selector-v2-proposed-vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture sourceVectorFile
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Status != "UNACCEPTED_OFFLINE_CANDIDATE" {
		t.Fatal("unexpected candidate status")
	}
	type pinnedOriginal struct {
		name, sha string
		length    int
	}
	pins := map[string]pinnedOriginal{
		"schema":     {"adr0011-generic-envelope-v2.schema.json", "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e", 13243},
		"transport":  {"generic-lsp-exact-transport-v2.json", "0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f", 1206},
		"references": {"generic-lsp-references-exact-v2.json", "cd92d4167abc951432804991e576a52a43236f9061dfe21c412410a6a851ecc5", 724},
		"definition": {"generic-lsp-definition-exact-v2.json", "f8fa1a0cb9f1379d69e347ef9573f6356d28f448f0cfb2529d5cd5747751a9af", 633},
	}
	originals := make(map[string][]byte)
	for key, expected := range pins {
		body, err := os.ReadFile(filepath.Join(root, expected.name))
		if err != nil {
			t.Fatal(err)
		}
		pin, ok := fixture.Pins[key]
		if !ok || len(body) != expected.length || candidateHash(body) != expected.sha || pin.Length != expected.length || pin.SHA256 != expected.sha {
			t.Fatalf("%s pinned original mismatch", key)
		}
		originals[key] = body
	}
	if len(fixture.Cases) != 9 || len(fixture.Selectors) != 22 || len(fixture.RejectedInputs) != 7 {
		t.Fatalf("unexpected vector coverage: %d cases, %d selectors", len(fixture.Cases), len(fixture.Selectors))
	}
	for _, method := range []string{"references", "definition"} {
		for key, entry := range fixture.Cases {
			got, err := candidateSelector(method, entry, originals["schema"], originals["transport"], originals[method])
			if err != nil {
				t.Fatalf("%s/%s: %v", method, key, err)
			}
			if want := fixture.Selectors[method+"_"+key]; got != want {
				t.Errorf("%s/%s candidate mismatch: got %s want %s", method, key, got, want)
			}
		}
		query := fixture.Cases["query"]
		alteredSchema, alteredPolicy := append(bytes.Clone(originals["schema"]), '\n'), append(bytes.Clone(originals[method]), '\n')
		for _, variant := range []struct {
			name           string
			schema, policy []byte
		}{
			{"schema_lf", alteredSchema, originals[method]}, {"policy_lf", originals["schema"], alteredPolicy},
		} {
			got, err := candidateSelector(method, query, variant.schema, originals["transport"], variant.policy)
			if err != nil {
				t.Fatal(err)
			}
			if want := fixture.Selectors[method+"_"+variant.name]; got != want {
				t.Errorf("%s/%s mismatch", method, variant.name)
			}
			if got == fixture.Selectors[method+"_query"] {
				t.Errorf("%s/%s substitution was invisible", method, variant.name)
			}
		}
		base := fixture.Selectors[method+"_query"]
		for _, key := range []string{"uri_byte", "version_byte", "content_byte", "custody", "transaction", "session", "generation", "target_present_empty_version_and_content"} {
			if fixture.Selectors[method+"_"+key] == base {
				t.Errorf("%s/%s did not change selector", method, key)
			}
		}
		for name, rejected := range fixture.RejectedInputs {
			if _, err := candidateSelector(method, rejected, originals["schema"], originals["transport"], originals[method]); err == nil {
				t.Errorf("%s/%s rejected input received selector", method, name)
			}
		}
		missing := query
		missing.VersionHex = nil
		if _, err := candidateSelector(method, missing, originals["schema"], originals["transport"], originals[method]); err == nil {
			t.Errorf("%s absent version accepted", method)
		}
		missing = query
		missing.ContentHex = nil
		if _, err := candidateSelector(method, missing, originals["schema"], originals["transport"], originals[method]); err == nil {
			t.Errorf("%s absent content accepted", method)
		}
		missing = query
		missing.Custody = "MADE_UP"
		if _, err := candidateSelector(method, missing, originals["schema"], originals["transport"], originals[method]); err == nil {
			t.Errorf("%s unknown custody accepted", method)
		}
	}
	if fixture.Selectors["references_query"] == fixture.Selectors["definition_query"] {
		t.Fatal("method policy substitution invisible")
	}
}
