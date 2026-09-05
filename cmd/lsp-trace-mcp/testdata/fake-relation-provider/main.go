// Command fake-relation-provider is a deterministic test-only provider process.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if marker := os.Getenv("LSP_TRACE_PROVIDER_START_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("started\n"), 0o600)
	}
	mode := os.Getenv("LSP_TRACE_PROVIDER_MODE")
	body, err := readFrame(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	if exitMarker := os.Getenv("LSP_TRACE_PROVIDER_EXIT_MARKER"); exitMarker != "" {
		defer os.WriteFile(exitMarker, []byte("exited\n"), 0o600) //nolint:errcheck
	}
	switch mode {
	case "hang":
		for {
			time.Sleep(time.Hour)
		}
	case "malformed-frame":
		_, _ = io.WriteString(os.Stdout, "Content-Length: nope\r\n\r\n{}")
	case "malformed-payload":
		writeFrame([]byte(`{"provider_id":`))
	case "oversized":
		_, _ = io.WriteString(os.Stdout, "Content-Length: 1048576\r\n\r\n{}")
	case "no-item":
		writeObservationEnvelope(body, nil, "UNKNOWN")
	case "adapter-mismatch", "revision-mismatch", "kind-mismatch", "invalid-coverage":
		writeObservationEnvelope(body, fixtureObservations(mode), map[string]string{"invalid-coverage": "INVALID"}[mode])
	default:
		var request map[string]any
		if json.Unmarshal(body, &request) != nil {
			os.Exit(2)
		}
		writeObservationEnvelope(body, fixtureObservations("success"), "COMPLETE_WITHIN_BOUNDS")
	}
}

func fixtureObservations(mode string) []map[string]any {
	kind := "PASSES_CALLBACK"
	if mode == "kind-mismatch" {
		kind = "RENDERS_FROM"
	}
	revision := strings.Repeat("b", 40)
	if mode == "revision-mismatch" {
		revision = strings.Repeat("d", 40)
	}
	return []map[string]any{{
		"kind":             kind,
		"from":             map[string]any{"node_id": "fixture:caller", "role": "CALLABLE_REFERENCE"},
		"to":               map[string]any{"node_id": "fixture:callback", "role": "CALLBACK_PARAMETER"},
		"original_anchor":  map[string]any{"document_id": "fixture-original", "uri": "file:///fixture/main.go", "revision": revision, "blob": strings.Repeat("c", 40), "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 7}}},
		"supports":         []string{"source_dependency_relation"},
		"does_not_support": []string{"runtime_execution", "callback_invocation", "repaint", "feature_identity", "whole_source_completeness"},
	}}
}

func readFrame(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	header, err := br.ReadString('\n')
	if err != nil || !strings.HasSuffix(header, "\r\n") || !strings.HasPrefix(header, "Content-Length: ") {
		return nil, fmt.Errorf("invalid header")
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "Content-Length: ")))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid length")
	}
	if separator, err := br.ReadString('\n'); err != nil || separator != "\r\n" {
		return nil, fmt.Errorf("invalid separator")
	}
	body := make([]byte, n)
	_, err = io.ReadFull(br, body)
	return body, err
}

func writeObservationEnvelope(requestBody []byte, observations []map[string]any, coverageStatus string) {
	if observations == nil {
		observations = []map[string]any{}
	}
	providerName, providerVersion := "fake-relation-provider", "1.0.0"
	adapterName, adapterVersion := "fake-relation-observations", "1.0.0"
	requestID, originalURI := "", "file:///fixture/main.go"
	revision := map[string]any{"kind": "git", "value": strings.Repeat("b", 40), "blob": strings.Repeat("c", 40), "custody": "PROVIDER_PROVED"}
	var request struct {
		ProviderID string `json:"provider_id"`
		AdapterID  string `json:"adapter_id"`
		Session    struct {
			SessionID  string `json:"session_id"`
			Generation uint64 `json:"generation"`
		} `json:"session"`
		Seed struct {
			URI string `json:"uri"`
		} `json:"seed"`
		Documents struct {
			OriginalURI       string         `json:"original_uri"`
			WorkspaceRevision map[string]any `json:"workspace_revision"`
		} `json:"document_custody"`
	}
	if json.Unmarshal(requestBody, &request) == nil && request.ProviderID != "" {
		providerName, providerVersion = splitIdentity(request.ProviderID)
		adapterName, adapterVersion = splitIdentity(request.AdapterID)
		originalURI = request.Documents.OriginalURI
		requestID = fmt.Sprintf("%s:%d:%s", request.Session.SessionID, request.Session.Generation, request.Seed.URI)
		if len(request.Documents.WorkspaceRevision) != 0 {
			revision = request.Documents.WorkspaceRevision
			if _, ok := revision["blob"]; !ok {
				revision["blob"] = strings.Repeat("c", 40)
			}
			if _, ok := revision["custody"]; !ok {
				revision["custody"] = "PROVIDER_PROVED"
			}
		}
	}
	if coverageStatus == "" {
		coverageStatus = "COMPLETE_WITHIN_BOUNDS"
	}
	if os.Getenv("LSP_TRACE_PROVIDER_MODE") == "revision-mismatch" {
		revision["value"] = strings.Repeat("d", 40)
	}
	if mode := os.Getenv("LSP_TRACE_PROVIDER_MODE"); mode == "adapter-mismatch" {
		adapterVersion = "mismatch"
	}
	envelope := map[string]any{
		"provider":   map[string]string{"name": providerName, "version": providerVersion},
		"protocol":   map[string]string{"name": "lsp-trace.provider-observations", "version": "1"},
		"adapter":    map[string]string{"name": adapterName, "version": adapterVersion},
		"request_id": requestID,
		"authority":  "PROVIDER_REPORTED",
		"coverage": map[string]any{
			"Status": coverageStatus, "Denominator": []string{"fixture-original"},
			"Covered": []string{"fixture-original"}, "CoveredCount": 1,
		},
		"documents": []map[string]any{{
			"document_id": "fixture-original", "original_uri": originalURI,
			"content_sha256": strings.Repeat("a", 64),
			"revision":       revision,
			"coordinates":    "ORIGINAL",
		}},
		"observations": observations,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		os.Exit(2)
	}
	writeFrame(body)
}

func splitIdentity(identity string) (string, string) {
	name, version, ok := strings.Cut(identity, "@")
	if !ok {
		return identity, ""
	}
	return name, version
}

func writeFrame(body []byte) {
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n", len(body))
	_, _ = os.Stdout.Write(body)
	_ = os.Stdout.Sync()
	time.Sleep(time.Millisecond)
}
