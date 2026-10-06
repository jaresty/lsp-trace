package lspwire

import (
	"bytes"
	"testing"
)

type c06CountingReader struct{ reads int }

func (r *c06CountingReader) Read([]byte) (int, error) { r.reads++; return 0, nil }

func TestC06SuccessorHistoryEntryBoundaryAndCopy(t *testing.T) {
	base := testSuccessorOptions()
	at := &ImmutableHistoryMetadata{Identity: "generation", Cut: "cut", Entries: MaxImmutableHistoryEntries, CutOrdinal: MaxImmutableHistoryEntries, Acquired: 8 << 20, Outstanding: 8 << 20}
	base.History = at
	r, err := NewSuccessorIngressReader(bytes.NewReader(nil), base)
	if err != nil {
		t.Fatalf("ASSERT_C06_SUCCESSOR_EQUALITY: %v", err)
	}
	at.Identity, at.Cut, at.Entries, at.CutOrdinal = "mutated", "mutated", 1, 1
	if r.options.History.Identity != "generation" || r.options.History.Cut != "cut" || r.options.History.Entries != MaxImmutableHistoryEntries || r.options.History.CutOrdinal != MaxImmutableHistoryEntries {
		t.Fatalf("ASSERT_C06_SUCCESSOR_COPY: %+v", *r.options.History)
	}

	for _, history := range []*ImmutableHistoryMetadata{
		{Identity: "generation", Cut: "cut", Entries: MaxImmutableHistoryEntries + 1, CutOrdinal: MaxImmutableHistoryEntries + 1},
		{Identity: "generation", Cut: "cut", Entries: 2, CutOrdinal: 1},
		{Identity: "generation", Cut: "cut", Entries: 0, CutOrdinal: 0},
	} {
		transport := &c06CountingReader{}
		base.History = history
		if _, err := NewSuccessorIngressReader(transport, base); err == nil || transport.reads != 0 {
			t.Fatalf("ASSERT_C06_SUCCESSOR_REFUSAL_PRE_READ: history=%+v err=%v reads=%d", *history, err, transport.reads)
		}
	}
}
