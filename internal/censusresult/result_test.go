package censusresult

import (
	"testing"

	"lsp-trace/internal/publication"
)

func TestNormalizePublicationStatuses(t *testing.T) {
	directory := map[string]string{
		publication.DirectorySyncComplete:               DirectorySyncComplete,
		publication.DirectorySyncFailed:                 DirectorySyncFailed,
		publication.DirectorySyncNotAttemptedPostCommit: DirectorySyncNotAttemptedPostCommit,
	}
	for input, want := range directory {
		got, err := normalizeDirectorySync(input)
		if err != nil || got != want {
			t.Fatalf("directory status %q: got %q, %v; want %q", input, got, err, want)
		}
	}
	closeStatuses := map[string]string{
		publication.CloseComplete: CloseComplete,
		publication.CloseFailed:   CloseFailed,
	}
	for input, want := range closeStatuses {
		got, err := normalizeClose(input)
		if err != nil || got != want {
			t.Fatalf("close status %q: got %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestNormalizePublicationStatusesRejectsPrivateVocabulary(t *testing.T) {
	for _, status := range []string{"", publication.CloseNotAttempted, "COMMITTED_UNKNOWN"} {
		if _, err := normalizeDirectorySync(status); err == nil {
			t.Fatalf("directory status %q accepted", status)
		}
		if _, err := normalizeClose(status); err == nil {
			t.Fatalf("close status %q accepted", status)
		}
	}
}
