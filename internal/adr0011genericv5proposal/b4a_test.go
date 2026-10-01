package adr0011genericv5proposal

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func b4aFixture(t *testing.T) B4aInput {
	t.Helper()
	in := B4aInput{Session: "s", Generation: 1, Transaction: "tx", Workspace: "file:///w", Method: "textDocument/references", URI: "file:///w/a.go", Version: "buffer:v1", Encoding: "utf-16", Source: []byte("alpha"), Custody: "OWNER_BUFFER", CompletedKey: "number:3", CompletedOrdinal: 8, WriteCompleted: true}
	in.RequestParams = []byte(`{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}`)
	in.RequestFrame = b4Frame(`{"jsonrpc":"2.0","id":3,"method":"textDocument/references","params":{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}}`)
	in.RequestID = []byte("3")
	tx := selector("ADR0011-GENERIC-TRANSACTION/1", "s", "1", "tx")
	sourceArt := b4aArtifact("ADR0011-GENERIC-SOURCE-ARTIFACT/1", tx, in.URI, "present", in.Version, "5", hash(in.Source), in.Custody)
	sourceSelector, e := A4Selector("references", "SOURCE", "s", "1", sourceArt, tx)
	if e != nil {
		t.Fatal(e)
	}
	queryArt := []byte("held-query-original")
	in.QueryOriginal = append([]byte(nil), queryArt...)
	querySelector, e := A4Selector("references", "QUERY", "s", "1", queryArt, in.URI, in.Version, hash(in.Source), in.Encoding, "0", "0")
	if e != nil {
		t.Fatal(e)
	}
	appArt := b4aArtifact("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1", tx, querySelector, sourceSelector, in.URI, "absent", "", "ORDINARY", "absent", "", "absent", "", "", in.Workspace, "s", "1", "tx")
	in.ApplicabilityOriginal = append([]byte(nil), appArt...)
	appSelector, e := A4Selector("references", "QUERY_APPLICABILITY", "s", "1", appArt, tx)
	if e != nil {
		t.Fatal(e)
	}
	desc := func(b []byte) map[string]any {
		return map[string]any{"length": len(b), "sha256": "sha256:" + hash(b), "private_ref": "held:bytes"}
	}
	dep := func(role, sel string) map[string]any {
		digest := strings.Repeat("0", 64) // Originals for these external roles are not held by B4a.
		switch role {
		case "SOURCE":
			digest = hash(in.Source)
		case "QUERY":
			digest = hash(queryArt)
		case "QUERY_APPLICABILITY":
			digest = hash(appArt)
		}
		return map[string]any{"role": role, "selector": sel, "digest": "sha256:" + digest}
	}
	makeClaim := func(role string, orig, art []byte, payload map[string]any, pred []any) B4aClaim {
		env := map[string]any{"role": role, "identity": map[string]any{"session": "s", "generation": 1, "transaction": "tx"}, "original": desc(orig), "predecessors": pred, "payload": payload}
		raw, e := json.Marshal(env)
		if e != nil {
			t.Fatal(e)
		}
		if e = A4Validate(role, raw); e != nil {
			t.Fatalf("B2/A4 shape %s: %v", role, e)
		}
		return B4aClaim{Role: role, Envelope: raw, Original: orig, Artifact: art}
	}
	src := makeClaim("SOURCE", in.Source, sourceArt, map[string]any{"uri": in.URI, "version": in.Version, "custody": in.Custody, "source_bytes": desc(in.Source)}, []any{})
	query := makeClaim("QUERY", queryArt, queryArt, map[string]any{"uri": in.URI, "version": in.Version, "method": in.Method, "line": 0, "character": 0, "encoding": in.Encoding, "source": "sha256:" + hash(in.Source)}, []any{dep("POLICY", "policy"), dep("SCHEMA", "schema"), dep("SOURCE", sourceSelector)})
	app := makeClaim("QUERY_APPLICABILITY", appArt, appArt, map[string]any{"workspace": in.Workspace, "session": in.Session, "generation": 1, "transaction": in.Transaction, "query_selector": querySelector, "query_source_selector": sourceSelector, "uri_bytes": desc([]byte(in.URI)), "language_id_present": false, "language_id_bytes": nil, "document_kind": "ORDINARY", "notebook_type_bytes": nil, "notebook_uri_bytes": nil, "notebook_source_selector": nil}, []any{dep("QUERY", querySelector), dep("SOURCE", sourceSelector)})
	write := makeClaim("REQUEST_WRITE", in.RequestFrame, in.RequestFrame, map[string]any{"params": desc(in.RequestParams), "frame": desc(in.RequestFrame), "actual_key": in.CompletedKey, "completed": true, "completed_frame_ordinal": 8}, []any{dep("QUERY", querySelector), dep("QUERY_APPLICABILITY", appSelector), dep("CAPABILITY_EVENTS", "cap"), dep("POLICY", "policy")})
	in.Claims = []B4aClaim{src, query, app, write}
	return in
}
func b4aArtifact(fields ...string) []byte {
	var out []byte
	for _, f := range fields {
		out = append(out, lp([]byte(f))...)
	}
	return out
}

func TestB4aQueryWritePrivateFocusedGreen(t *testing.T) {
	in := b4aFixture(t)
	if err := CheckB4a(in); err != nil {
		t.Fatalf("valid independently held B4a correspondence: %v", err)
	}
	for name, change := range map[string]func(*B4aInput){
		"source byte":                 func(x *B4aInput) { x.Source = []byte("alphA") },
		"query original":              func(x *B4aInput) { x.QueryOriginal = []byte("substituted") },
		"applicability original":      func(x *B4aInput) { x.ApplicabilityOriginal = []byte("substituted") },
		"query version":               func(x *B4aInput) { x.Version = "buffer:v2" },
		"query encoding":              func(x *B4aInput) { x.Encoding = "utf-8" },
		"query line":                  func(x *B4aInput) { x.Line = 1 },
		"query character":             func(x *B4aInput) { x.Character = 1 },
		"query URI":                   func(x *B4aInput) { x.URI = "file:///w/b.go" },
		"write key":                   func(x *B4aInput) { x.CompletedKey = "number:4" },
		"write ordinal":               func(x *B4aInput) { x.CompletedOrdinal = 9 },
		"write params":                func(x *B4aInput) { x.RequestParams = []byte(`{}`) },
		"write ID":                    func(x *B4aInput) { x.RequestID = []byte("4") },
		"write method":                func(x *B4aInput) { x.Method = "textDocument/definition" },
		"write incomplete":            func(x *B4aInput) { x.WriteCompleted = false },
		"cross transaction":           func(x *B4aInput) { x.Transaction = "other" },
		"missing":                     func(x *B4aInput) { x.Claims = x.Claims[:3] },
		"extra":                       func(x *B4aInput) { x.Claims = append(x.Claims, x.Claims[0]) },
		"duplicate":                   func(x *B4aInput) { x.Claims[3] = x.Claims[0] },
		"claim original mutation":     func(x *B4aInput) { x.Claims[1].Original = []byte("mutated") },
		"claim envelope substitution": func(x *B4aInput) { x.Claims[1].Envelope = x.Claims[0].Envelope },
	} {
		t.Run(name, func(t *testing.T) {
			x := b4aFixture(t)
			change(&x)
			if err := CheckB4a(x); err == nil {
				t.Fatal("correspondence substitution accepted")
			}
		})
	}
	t.Log("B4A_QUERY_WRITE_PRIVATE_FOCUSED_GREEN")
}

func TestB4aValidShapeClaimantSubstitution(t *testing.T) {
	for _, tc := range []struct {
		name, role, field string
		value             any
	}{
		{"SOURCE version", "SOURCE", "version", "buffer:v2"},
		{"QUERY URI", "QUERY", "uri", "file:///w/b.go"},
		{"QUERY version", "QUERY", "version", "buffer:v2"},
		{"QUERY encoding", "QUERY", "encoding", "utf-8"},
		{"QUERY line", "QUERY", "line", 1},
		{"QUERY character", "QUERY", "character", 1},
		{"QUERY source digest", "QUERY", "source", "sha256:" + strings.Repeat("0", 64)},
		{"QUERY_APPLICABILITY workspace", "QUERY_APPLICABILITY", "workspace", "file:///other"},
		{"QUERY_APPLICABILITY query selector", "QUERY_APPLICABILITY", "query_selector", "substituted"},
		{"REQUEST_WRITE ordinal", "REQUEST_WRITE", "completed_frame_ordinal", 9},
		{"REQUEST_WRITE key", "REQUEST_WRITE", "actual_key", "number:4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4aFixture(t)
			for i := range in.Claims {
				if in.Claims[i].Role != tc.role {
					continue
				}
				var env map[string]any
				if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
					t.Fatal(err)
				}
				env["payload"].(map[string]any)[tc.field] = tc.value
				in.Claims[i].Envelope = a4JSON(t, env)
				if err := A4Validate(tc.role, in.Claims[i].Envelope); err != nil {
					t.Fatalf("claimant no longer A4 valid: %v", err)
				}
			}
			if err := CheckB4a(in); err == nil {
				t.Fatal("A4-valid substituted claimant accepted")
			}
		})
	}
}

func TestB4aCompletedKeyMatchesIndependentHeldWrite(t *testing.T) {
	in := b4aFixture(t)
	in.CompletedKey = "key-1"
	for i := range in.Claims {
		if in.Claims[i].Role != "REQUEST_WRITE" {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
			t.Fatal(err)
		}
		env["payload"].(map[string]any)["actual_key"] = in.CompletedKey
		in.Claims[i].Envelope = a4JSON(t, env)
		if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
			t.Fatalf("claimant no longer A4 valid: %v", err)
		}
	}
	if err := CheckB4a(in); err != nil {
		t.Fatalf("A4-valid key-1 matched held completed WRITE key with independently matched numeric ID 3 rejected: %v", err)
	}
	// Keep the independently held completed WRITE key fixed; alter only the claimant.
	for i := range in.Claims {
		if in.Claims[i].Role != "REQUEST_WRITE" {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
			t.Fatal(err)
		}
		env["payload"].(map[string]any)["actual_key"] = "key-2"
		in.Claims[i].Envelope = a4JSON(t, env)
		if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
			t.Fatalf("claimant no longer A4 valid: %v", err)
		}
	}
	if err := CheckB4a(in); err == nil || !strings.Contains(err.Error(), "completed WRITE payload correspondence") {
		t.Fatalf("claimant key-2 diverged from independently held completed key-1: %v", err)
	}
}

func TestB4aTypedWireIDIndependentOfCompletedKey(t *testing.T) {
	for _, tc := range []struct {
		name, wireID, heldID string
	}{
		{"string", `"3"`, `"3"`},
		{"equivalent decimal", `3.0`, `3`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4aFixture(t)
			in.RequestID = []byte(tc.heldID)
			in.RequestFrame = b4Frame(`{"jsonrpc":"2.0","id":` + tc.wireID + `,"method":"textDocument/references","params":` + string(in.RequestParams) + `}`)
			for i := range in.Claims {
				if in.Claims[i].Role != "REQUEST_WRITE" {
					continue
				}
				in.Claims[i].Original = in.RequestFrame
				in.Claims[i].Artifact = in.RequestFrame
				var env map[string]any
				if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
					t.Fatal(err)
				}
				env["original"] = map[string]any{"length": len(in.RequestFrame), "sha256": "sha256:" + hash(in.RequestFrame), "private_ref": "held:bytes"}
				env["payload"].(map[string]any)["frame"] = env["original"]
				in.Claims[i].Envelope = a4JSON(t, env)
				if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
					t.Fatalf("A4 shape: %v", err)
				}
			}
			if err := CheckB4a(in); err != nil {
				t.Fatalf("typed wire ID with independently held key: %v", err)
			}
			in.RequestID = []byte(`4`)
			if err := CheckB4a(in); err == nil || !strings.Contains(err.Error(), "held request ID") {
				t.Fatalf("divergent held typed ID accepted: %v", err)
			}
		})
	}
}

func TestB4aREDHeldPredecessorOriginalDigest(t *testing.T) {
	in := b4aFixture(t)
	for i := range in.Claims {
		if in.Claims[i].Role != "QUERY_APPLICABILITY" {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
			t.Fatal(err)
		}
		for _, edge := range env["predecessors"].([]any) {
			dep := edge.(map[string]any)
			if dep["role"] == "QUERY" {
				dep["digest"] = "sha256:" + strings.Repeat("0", 64)
			}
		}
		in.Claims[i].Envelope = a4JSON(t, env)
		if err := A4Validate("QUERY_APPLICABILITY", in.Claims[i].Envelope); err != nil {
			t.Fatalf("claimant no longer A4 valid: %v", err)
		}
	}
	if err := CheckB4a(in); err == nil {
		t.Fatal("B4a accepted substituted QUERY predecessor digest with unchanged selector and held QUERY original")
	}
}

func TestB4aZeroCompletedOrdinalCorrespondence(t *testing.T) {
	in := b4aFixture(t)
	in.CompletedOrdinal = 0
	for i := range in.Claims {
		if in.Claims[i].Role != "REQUEST_WRITE" {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
			t.Fatal(err)
		}
		env["payload"].(map[string]any)["completed_frame_ordinal"] = 0
		in.Claims[i].Envelope = a4JSON(t, env)
		if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
			t.Fatalf("zero ordinal claimant must remain A4 valid: %v", err)
		}
	}
	if err := CheckB4a(in); err != nil {
		t.Fatalf("matched zero completed ordinal rejected: %v", err)
	}
	in.CompletedOrdinal = 1
	if err := CheckB4a(in); err == nil || !strings.Contains(err.Error(), "completed WRITE ordinal") {
		t.Fatalf("divergent held ordinal versus zero claimant accepted: %v", err)
	}
}

func TestB4aOptionalContentTypeHeldWriteFrame(t *testing.T) {
	for _, order := range []string{"length-first", "type-first"} {
		t.Run(order, func(t *testing.T) {
			in := b4aFixture(t)
			body := in.RequestFrame[strings.Index(string(in.RequestFrame), "\r\n\r\n")+4:]
			length := fmt.Sprintf("Content-Length: %d", len(body))
			contentType := "Content-Type: application/vscode-jsonrpc; charset=utf-8"
			headers := length + "\r\n" + contentType
			if order == "type-first" {
				headers = contentType + "\r\n" + length
			}
			in.RequestFrame = append([]byte(headers+"\r\n\r\n"), body...)
			in.CompletedKey = "arbitrary-held-key"
			in.CompletedOrdinal = 0
			for i := range in.Claims {
				if in.Claims[i].Role != "REQUEST_WRITE" {
					continue
				}
				in.Claims[i].Original = append([]byte(nil), in.RequestFrame...)
				in.Claims[i].Artifact = append([]byte(nil), in.RequestFrame...)
				var env map[string]any
				if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
					t.Fatal(err)
				}
				d := map[string]any{"length": len(in.RequestFrame), "sha256": "sha256:" + hash(in.RequestFrame), "private_ref": "held:bytes"}
				env["original"] = d
				payload := env["payload"].(map[string]any)
				payload["frame"] = d
				payload["actual_key"] = in.CompletedKey
				payload["completed_frame_ordinal"] = 0
				in.Claims[i].Envelope = a4JSON(t, env)
				if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
					t.Fatalf("A4-valid optional-header claimant: %v", err)
				}
			}
			if err := CheckB4a(in); err != nil {
				t.Fatalf("optional-header held WRITE correspondence: %v", err)
			}
			duplicateBody := []byte(`{"jsonrpc":"2.0","jsonrpc":"2.0"}`)
			for _, bad := range []struct {
				name  string
				frame []byte
			}{
				{"wrong length", append([]byte(strings.Replace(headers, length, fmt.Sprintf("Content-Length: %d", len(body)+1), 1)+"\r\n\r\n"), body...)},
				{"incomplete", in.RequestFrame[:len(in.RequestFrame)-1]},
				{"malformed", append([]byte(strings.Replace(headers, "Content-Length:", "Bad-Length:", 1)+"\r\n\r\n"), body...)},
				{"duplicate decoded key", append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(duplicateBody))), duplicateBody...)},
			} {
				t.Run(bad.name, func(t *testing.T) {
					if _, err := b4aFrame(bad.frame); err == nil {
						t.Fatal("invalid held frame accepted")
					}
				})
			}
		})
	}
}

func TestB4aCaseInsensitiveHeldWriteFrame(t *testing.T) {
	for _, headers := range []string{
		"content-length: %d",
		"X-Trace: optional\r\ncontent-length: %d",
		"content-length: %d\r\nX-Trace: optional",
	} {
		t.Run(headers, func(t *testing.T) {
			in := b4aFixture(t)
			body := in.RequestFrame[strings.Index(string(in.RequestFrame), "\r\n\r\n")+4:]
			in.RequestFrame = append([]byte(fmt.Sprintf(headers, len(body))+"\r\n\r\n"), body...)
			in.CompletedKey, in.CompletedOrdinal = "independently-held-key", 0
			for i := range in.Claims {
				if in.Claims[i].Role != "REQUEST_WRITE" {
					continue
				}
				in.Claims[i].Original = append([]byte(nil), in.RequestFrame...)
				in.Claims[i].Artifact = append([]byte(nil), in.RequestFrame...)
				var env map[string]any
				if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
					t.Fatal(err)
				}
				d := map[string]any{"length": len(in.RequestFrame), "sha256": "sha256:" + hash(in.RequestFrame), "private_ref": "held:bytes"}
				env["original"] = d
				payload := env["payload"].(map[string]any)
				payload["frame"], payload["actual_key"], payload["completed_frame_ordinal"] = d, in.CompletedKey, 0
				in.Claims[i].Envelope = a4JSON(t, env)
				if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
					t.Fatalf("A4-valid claimant: %v", err)
				}
			}
			if err := CheckB4a(in); err != nil {
				t.Fatalf("valid independently held frame rejected: %v", err)
			}
			for _, bad := range []string{
				fmt.Sprintf("Content-Length: %d\r\ncontent-length: %d", len(body), len(body)),
				fmt.Sprintf("content-length: %d\r\nMalformed", len(body)),
			} {
				if _, err := b4aFrame(append([]byte(bad+"\r\n\r\n"), body...)); err == nil {
					t.Fatalf("invalid header accepted: %q", bad)
				}
			}
		})
	}
}

// The claimant and held frame agree exactly; only the JSON-RPC request shape changes.
func TestB4aHeldWriteMustBeRequest(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		valid       bool
	}{
		{"result null", `,"result":null`, false},
		{"error object", `,"error":{"code":-32603,"message":"failure"}`, false},
		{"extension field", `,"extension":{"vendor":true}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4aFixture(t)
			body := `{"jsonrpc":"2.0","id":3,"method":"textDocument/references","params":` + string(in.RequestParams) + tc.extra + `}`
			in.RequestFrame = b4Frame(body)
			for i := range in.Claims {
				if in.Claims[i].Role != "REQUEST_WRITE" {
					continue
				}
				in.Claims[i].Original = append([]byte(nil), in.RequestFrame...)
				in.Claims[i].Artifact = append([]byte(nil), in.RequestFrame...)
				var env map[string]any
				if err := json.Unmarshal(in.Claims[i].Envelope, &env); err != nil {
					t.Fatal(err)
				}
				d := map[string]any{"length": len(in.RequestFrame), "sha256": "sha256:" + hash(in.RequestFrame), "private_ref": "held:bytes"}
				env["original"] = d
				env["payload"].(map[string]any)["frame"] = d
				in.Claims[i].Envelope = a4JSON(t, env)
				if err := A4Validate("REQUEST_WRITE", in.Claims[i].Envelope); err != nil {
					t.Fatalf("A4-valid WRITE required: %v", err)
				}
			}
			err := CheckB4a(in)
			if tc.valid && err != nil {
				t.Fatalf("valid extension request rejected: %v", err)
			}
			if !tc.valid && (err == nil || !strings.Contains(err.Error(), "held request shape")) {
				t.Fatalf("invalid held WRITE request shape accepted or rejected for wrong reason: %v", err)
			}
		})
	}
}

func TestB4aREDIndependentlyHeldWriteOrdinal(t *testing.T) {
	in := b4aFixture(t)
	// The claimant and held WRITE are separately shaped and valid; only the ordinal differs.
	in.CompletedOrdinal = 9
	if err := CheckB4a(in); err == nil || !strings.Contains(err.Error(), "completed WRITE ordinal") {
		t.Fatalf("B4a independently held completed WRITE ordinal mismatch accepted: %v", err)
	}
}
