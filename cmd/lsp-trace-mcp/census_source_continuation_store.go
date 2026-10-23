package main

import (
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/publication"
)

const censusSourceContinuationMaxBytes = censussourceassembly.MaxBytes

type censusSourceContinuationObjects = censussourceassembly.Objects

type censusSourceContinuationHeads = censussourceassembly.Heads

func newCensusSourceContinuationObjects(root *publication.Root) (*censusSourceContinuationObjects, error) {
	return censussourceassembly.NewObjects(root)
}

func censusSourceContinuationDigest(raw []byte) string {
	return censussourceassembly.ObjectSelector(raw)
}

func newCensusSourceContinuationHeads(root *publication.Root) (*censusSourceContinuationHeads, error) {
	return censussourceassembly.NewHeads(root)
}

func censusSourceContinuationHeadSelector(key string) (string, error) {
	return censussourceassembly.HeadSelector(key)
}
