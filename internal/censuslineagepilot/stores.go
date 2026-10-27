package censuslineagepilot

import (
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

const headNamespace = "census-lineage-pilot/heads/private/v1"

// NewHeads binds pilot committed heads to their private production namespace.
func NewHeads(root *publication.Root) (censussourcecontinuation.HeadStore, error) {
	return censussourceassembly.NewHeadsWithParser(root, headNamespace, ParseHeadRevision)
}
