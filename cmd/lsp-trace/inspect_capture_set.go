package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/capturesetinspection"
	"lsp-trace/internal/publication"
)

func runInspectCaptureSet(selector, rootPath string, jsonOutput bool, stdout, stderr io.Writer) int {
	if err := captureset.ValidatePublicationSelector(selector); err != nil {
		fmt.Fprintf(stderr, "inspect capture-set: %v\n", err)
		return 1
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		fmt.Fprintf(stderr, "inspect capture-set: %v\n", err)
		return 1
	}
	defer root.Close()
	manifest, err := captureset.NewPublisher(root).Verify(selector, captureset.NativeV5Authority())
	if err != nil {
		fmt.Fprintf(stderr, "inspect capture-set: %v\n", err)
		return 1
	}
	result := capturesetinspection.Project(manifest)
	if err := capturesetinspection.Validate(result); err != nil {
		fmt.Fprintf(stderr, "inspect capture-set: %v\n", err)
		return 1
	}
	if jsonOutput {
		encoded, err := json.Marshal(result)
		if err == nil {
			_, err = stdout.Write(append(encoded, '\n'))
		}
		if err != nil {
			fmt.Fprintf(stderr, "inspect capture-set: %v\n", err)
			return 1
		}
		return 0
	}
	if _, err := fmt.Fprintf(stdout,
		"capture-set identity: %s\ndisclosure: %s\ncounts: targets=%d batches=%d constituents=%d\nfiles: denominator=%d dispositions=%s\nsymbols: denominator=%d dispositions=%s\nauthority: %d\nsource_graph_complete: %s\nnative_single_capture_custody: %t\ncross_capture_calls: %t\nleiden_admissible: %t\n",
		result.CaptureSetIdentity, result.Disclosure, result.TargetCount, result.BatchCount, result.ConstituentCount,
		result.Files.Denominator, formatDispositionCounts(result.Files.Dispositions), result.Symbols.Denominator, formatDispositionCounts(result.Symbols.Dispositions),
		result.Authority, result.SourceGraphComplete, result.NativeSingleCaptureCustody, result.CrossCaptureCalls, result.LeidenAdmissible); err != nil {
		fmt.Fprintf(stderr, "inspect capture-set: %v\n", err)
		return 1
	}
	return 0
}

func formatDispositionCounts(counts []capturesetinspection.DispositionCount) string {
	if len(counts) == 0 {
		return "[]"
	}
	parts := make([]string, len(counts))
	for i, count := range counts {
		parts[i] = fmt.Sprintf("%s=%d", count.Disposition, count.Count)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
