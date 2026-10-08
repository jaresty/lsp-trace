package adr0007locationv4

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Oracle contains independently fixed, literal v4 inputs and semantic expected
// results. Generation copies these bytes; it never invokes the production
// evaluator or a shared result-building helper.
//
//go:embed oracle/*/*.json oracle/*/source/*
var oracleFS embed.FS

func OracleCases() ([]string, error) {
	entries, err := fs.ReadDir(oracleFS, "oracle")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("non-directory oracle entry %q", entry.Name())
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) != 24 {
		return nil, fmt.Errorf("oracle case census=%d", len(names))
	}
	return names, nil
}

func OracleRead(caseName, name string) ([]byte, error) {
	if strings.Contains(caseName, "/") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid oracle path")
	}
	return oracleFS.ReadFile("oracle/" + caseName + "/" + name)
}

func OracleSourceNames(caseName string) ([]string, error) {
	entries, err := fs.ReadDir(oracleFS, "oracle/"+caseName+"/source")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("oracle source directory %q", entry.Name())
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}
