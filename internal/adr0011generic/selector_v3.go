package adr0011generic

import (
	"fmt"
	"strconv"
	"strings"
)

// A3-only selector recomputation; it is not an issuer or a runtime choice.
const v3Domain = "ADR0011-GENERIC-EXACT/3"

func v3Policy(method string) (profile, role string, err error) {
	switch method {
	case "textDocument/references":
		return "GENERIC_LSP_REFERENCES_EXACT_V3", "REFERENCES", nil
	case "textDocument/definition":
		return "GENERIC_LSP_DEFINITION_EXACT_V3", "DEFINITION", nil
	default:
		return "", "", fmt.Errorf("unsupported V3 method")
	}
}

func v3Prefix(method, role, session string, generation uint64, artifact []byte) ([]string, error) {
	profile, policyRole, err := v3Policy(method)
	if err != nil {
		return nil, err
	}
	if session == "" || generation == 0 || len(artifact) == 0 {
		return nil, fmt.Errorf("incomplete V3 selector identity")
	}
	schema, err := loadV3Original("SCHEMA")
	if err != nil {
		return nil, err
	}
	transport, err := loadV3Original("TRANSPORT")
	if err != nil {
		return nil, err
	}
	policy, err := loadV3Original(policyRole)
	if err != nil {
		return nil, err
	}
	return []string{v3Domain, profile, method, role, digest(schema), digest(transport), digest(policy), session, strconv.FormatUint(generation, 10), digest(artifact)}, nil
}

func v3HashParts(parts []string) string {
	var preimage []byte
	for _, part := range parts {
		preimage = append(preimage, component(part)...)
	}
	return "sha256:" + digest(preimage)
}

func calculateV3Selector(in selectorInput) (string, error) {
	if in.Role != "QUERY" && in.Role != "TARGET_EVENTS" {
		return "", fmt.Errorf("unsupported V3 selector role")
	}
	parts, err := v3Prefix(in.Method, in.Role, in.SessionID, in.Generation, in.Artifact)
	if err != nil {
		return "", err
	}
	if in.Role == "QUERY" {
		if in.URI == "" || in.Version == "" || !strings.Contains(in.Version, ":") || !validateSHA(in.SourceDigest) || (in.Encoding != "utf-8" && in.Encoding != "utf-16" && in.Encoding != "utf-32") {
			return "", fmt.Errorf("invalid V3 query identity")
		}
		parts = append(parts, in.URI, in.Version, in.SourceDigest, in.Encoding, strconv.FormatUint(in.Line, 10), strconv.FormatUint(in.Character, 10))
	} else {
		if in.ActualWriteKey == "" || !validateSHA(in.ResultDigest) {
			return "", fmt.Errorf("invalid V3 occurrence identity")
		}
		parts = append(parts, in.ActualWriteKey, in.ResultDigest, strconv.FormatUint(in.Ordinal, 10))
	}
	return v3HashParts(parts), nil
}

func heldV3SourceSelector(method string, id syntheticTransactionIdentity, source HeldSourceOriginal) (string, error) {
	tx, err := heldSourceTX(id)
	if err != nil {
		return "", err
	}
	if source.TransactionIdentity != tx {
		return "", fmt.Errorf("cross-transaction SOURCE")
	}
	if err = sourceByteBounds(source.URIBytes, source.VersionBytes, source.ContentBytes); err != nil {
		return "", err
	}
	if !validSourceCustody(source.CustodyKind) {
		return "", fmt.Errorf("invalid custody")
	}
	var artifact []byte
	for _, field := range [][]byte{[]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1"), []byte(tx), source.URIBytes, []byte("present"), source.VersionBytes, []byte(strconv.Itoa(len(source.ContentBytes))), []byte(digest(source.ContentBytes)), []byte(source.CustodyKind)} {
		artifact = append(artifact, component(string(field))...)
	}
	// The outer V3 selector uses the digest of the SOURCE artifact as its
	// artifact identity. No claimant-provided digest enters the preimage.
	parts, err := v3Prefix(method, "SOURCE", id.session, id.generation, artifact)
	if err != nil {
		return "", err
	}
	return v3HashParts(append(parts, tx)), nil
}
