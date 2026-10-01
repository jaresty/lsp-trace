package adr0011generic

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type selectorInput struct {
	Method, Role, SessionID              string
	Generation                           uint64
	Artifact                             []byte
	URI, Version, SourceDigest, Encoding string
	Line, Character                      uint64
	ActualWriteKey, ResultDigest         string
	Ordinal                              uint64
}

const domain = "ADR0011-GENERIC-EXACT/2"

func component(s string) []byte {
	b := []byte(s)
	r := make([]byte, 8+len(b))
	binary.BigEndian.PutUint64(r[:8], uint64(len(b)))
	copy(r[8:], b)
	return r
}
func validateSHA(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}

// calculateSelector uses only embedded independently selected originals for
// schema and policy identities. Input never supplies expected profile digests.
func calculateSelector(in selectorInput) (string, error) {
	var profile, policy string
	switch in.Method {
	case "textDocument/references":
		profile = "GENERIC_LSP_REFERENCES_EXACT_V2"
		policy = "REFERENCES"
	case "textDocument/definition":
		profile = "GENERIC_LSP_DEFINITION_EXACT_V2"
		policy = "DEFINITION"
	default:
		return "", errors.New("unsupported generic method")
	}
	if in.SessionID == "" || in.Generation == 0 || len(in.Artifact) == 0 {
		return "", errors.New("incomplete selector identity")
	}
	if in.Role != "QUERY" && in.Role != "TARGET_EVENTS" {
		return "", errors.New("selector role not in checkpoint A fixture")
	}
	schema, err := loadOriginal("SCHEMA")
	if err != nil {
		return "", err
	}
	transport, err := loadOriginal("TRANSPORT")
	if err != nil {
		return "", err
	}
	methodPolicy, err := loadOriginal(policy)
	if err != nil {
		return "", err
	}
	parts := []string{domain, profile, in.Method, in.Role, digest(schema), digest(transport), digest(methodPolicy), in.SessionID, strconv.FormatUint(in.Generation, 10), digest(in.Artifact)}
	if in.Role == "QUERY" {
		if in.URI == "" || in.Version == "" || !strings.Contains(in.Version, ":") || !validateSHA(in.SourceDigest) || (in.Encoding != "utf-8" && in.Encoding != "utf-16" && in.Encoding != "utf-32") {
			return "", errors.New("invalid query identity")
		}
		parts = append(parts, in.URI, in.Version, in.SourceDigest, in.Encoding, strconv.FormatUint(in.Line, 10), strconv.FormatUint(in.Character, 10))
	} else {
		if in.ActualWriteKey == "" || !validateSHA(in.ResultDigest) {
			return "", errors.New("missing observed occurrence identity")
		}
		parts = append(parts, in.ActualWriteKey, in.ResultDigest, strconv.FormatUint(in.Ordinal, 10))
	}
	h := sha256.New()
	for _, p := range parts {
		if _, err := h.Write(component(p)); err != nil {
			return "", fmt.Errorf("selector digest: %w", err)
		}
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
