package adr0011generic

// Private, default-off contract originals. This package does not acquire an LSP
// result, issue an occurrence, select a session, or confer production authority.

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
)

//go:embed originals/*
var originals embed.FS // V1 historical; V2 remains selected by originalPins. V3 is private and separately pinned.

const (
	schemaFile     = "adr0011-generic-envelope-v2.schema.json"
	transportFile  = "generic-lsp-exact-transport-v2.json"
	referencesFile = "generic-lsp-references-exact-v2.json"
	definitionFile = "generic-lsp-definition-exact-v2.json"
)

type originalPin struct {
	name   string
	length int
	hash   string
}

var originalPins = map[string]originalPin{
	"SCHEMA":     {schemaFile, 13243, "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e"},
	"TRANSPORT":  {transportFile, 1206, "0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f"},
	"REFERENCES": {referencesFile, 724, "cd92d4167abc951432804991e576a52a43236f9061dfe21c412410a6a851ecc5"},
	"DEFINITION": {definitionFile, 633, "f8fa1a0cb9f1379d69e347ef9573f6356d28f448f0cfb2529d5cd5747751a9af"},
}

var v3OriginalPins = map[string]originalPin{
	"SCHEMA":        originalPins["SCHEMA"],
	"TRANSPORT":     originalPins["TRANSPORT"],
	"APPLICABILITY": {"generic-lsp-selector-applicability-v1.json", 2247, "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d"},
	"REFERENCES":    {"generic-lsp-references-exact-v3.json", 1134, "8c76a71da5e888e278280bbb1802979b907ba7a4ad6b0aec910dfe06e24e225e"},
	"DEFINITION":    {"generic-lsp-definition-exact-v3.json", 1043, "52dd415cb5a331462042103014991fcd4ff43a777d45ab5a9abd7d83a45a8051"},
}

// loadV3Original is a private, separately pinned selection. It never changes
// V2's selected originals or enables a runtime route.
func loadV3Original(role string) ([]byte, error) {
	pin, ok := v3OriginalPins[role]
	if !ok {
		return nil, fmt.Errorf("unknown V3 original role %q", role)
	}
	b, err := originals.ReadFile("originals/" + pin.name)
	if err != nil {
		return nil, err
	}
	if len(b) != pin.length || digest(b) != pin.hash {
		return nil, errors.New("V3 embedded original identity mismatch")
	}
	return bytes.Clone(b), nil
}
func verifyV3Original(role string, b []byte) error {
	selected, err := loadV3Original(role)
	if err != nil {
		return err
	}
	if !bytes.Equal(selected, b) {
		return errors.New("V3 original byte substitution")
	}
	return nil
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// loadOriginal reads an embedded original; a caller cannot supply or override
// the expected identity. Returns a clone to prevent mutation of an expectation.
func loadOriginal(role string) ([]byte, error) {
	pin, ok := originalPins[role]
	if !ok {
		return nil, fmt.Errorf("unknown original role %q", role)
	}
	b, err := originals.ReadFile("originals/" + pin.name)
	if err != nil {
		return nil, err
	}
	if len(b) != pin.length || digest(b) != pin.hash {
		return nil, errors.New("embedded original identity mismatch")
	}
	return bytes.Clone(b), nil
}
func verifyOriginal(role string, b []byte) error {
	selected, err := loadOriginal(role)
	if err != nil {
		return err
	}
	if !bytes.Equal(selected, b) {
		return errors.New("original byte substitution")
	}
	return nil
}
