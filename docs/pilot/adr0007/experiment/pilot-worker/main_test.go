package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectGrammarAndSystemMode(t *testing.T) {
	got, external, err := selectGrammar("")
	if err != nil || external || got != legacyGrammar {
		t.Fatalf("legacy mode: external=%v err=%v grammar=%q", external, err, got)
	}
	root := t.TempDir()
	path := filepath.Join(root, "v2.gbnf")
	want := []byte(`root ::= "V2_ONLY"`)
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	got, external, err = selectGrammar(path)
	if err != nil || !external || got != string(want) {
		t.Fatalf("external mode: external=%v err=%v grammar=%q", external, err, got)
	}
	legacy := formatPrompt([]byte("packet"), false)
	if !strings.Contains(legacy, legacySystemPrompt) || !strings.Contains(legacy, "verdict must be SUPPORTED") {
		t.Fatalf("legacy wrapper changed: %q", legacy)
	}
	v2 := formatPrompt([]byte("packet"), true)
	if !strings.Contains(v2, v2SystemPrompt) || strings.Contains(v2, "SUPPORTED") || strings.Contains(v2, "nearest_outward_consumer") {
		t.Fatalf("v2 wrapper is legacy-shaped: %q", v2)
	}
}

func TestSelectGrammarRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "grammar.gbnf")
	if err := os.WriteFile(regular, []byte(`root ::= "ok"`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.gbnf")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(root, "large.gbnf")
	if err := os.WriteFile(oversized, make([]byte, maxGrammarBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative.gbnf", link, root, oversized, filepath.Join(root, "missing.gbnf")} {
		if _, _, err := selectGrammar(path); err == nil {
			t.Fatalf("unsafe grammar path accepted: %q", path)
		}
	}
}
