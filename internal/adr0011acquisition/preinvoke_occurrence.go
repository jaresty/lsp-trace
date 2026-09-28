package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"lsp-trace/internal/publication"
)

// preinvokeFacts is a test-only declaration of facts available before either
// method request. Source is a private immutable receipt, never a wire identity.
type preinvokeFacts struct {
	SessionID    string                `json:"session_id"`
	Generation   uint64                `json:"generation"`
	Workspace    string                `json:"workspace"`
	URI          string                `json:"uri"`
	Line         uint32                `json:"line"`
	Character    uint32                `json:"character"`
	Version      int                   `json:"version"`
	SourceDigest string                `json:"source_digest"`
	SourceLength int                   `json:"source_length"`
	Source       []byte                `json:"-"` // checked against separately retained exact bytes
	GitRoot      string                `json:"git_root"`
	GitCommit    string                `json:"git_commit"`
	Executable   selectedGitExecutable `json:"-"`
}

type preinvokeRecord struct {
	Role             string         `json:"role"`
	Facts            preinvokeFacts `json:"facts"`
	ExecutablePath   string         `json:"executable_path"`
	ExecutableURI    string         `json:"executable_uri"`
	ExecutableDigest string         `json:"executable_digest"`
	SourceSelector   string         `json:"source_selector"`
	OccurrenceID     string         `json:"occurrence_id"`
}

func preinvokeCanonical(f preinvokeFacts) ([]byte, string, error) {
	if f.SessionID == "" || f.Generation == 0 || f.Workspace == "" || f.URI == "" || f.GitRoot != f.Workspace || f.GitCommit == "" || f.Version < 0 || f.SourceLength != len(f.Source) || f.SourceLength == 0 || f.SourceDigest != privateDigest(f.Source) || f.Executable.path == "" || f.Executable.uri == "" || f.Executable.digest == "" {
		return nil, "", ErrAcquisition
	}
	// The ID is derived solely from the canonical pre-invocation facts, excluding
	// its own field, selectors, requests, transactions and later observations.
	record := preinvokeRecord{Role: "QUERY_OCCURRENCE_TEST_V1", Facts: f, ExecutablePath: f.Executable.path, ExecutableURI: f.Executable.uri, ExecutableDigest: f.Executable.digest,
		SourceSelector: "query-occurrence-test-source-v1-" + strings.TrimPrefix(f.SourceDigest, "sha256:") + ".bin"}
	basis, err := json.Marshal(record)
	if err != nil {
		return nil, "", ErrAcquisition
	}
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(append([]byte("QUERY_OCCURRENCE_TEST_V1\x00"), basis...)))
	record.OccurrenceID = id
	body, err := json.Marshal(record)
	if err != nil {
		return nil, "", ErrAcquisition
	}
	return body, id, nil
}

// publishPreinvokeOccurrence is called only by tests after READY and independently
// selected clean-Git and prepared-source facts. Publication is no-replace.
func publishPreinvokeOccurrence(root *publication.Root, f preinvokeFacts) (string, string, string, error) {
	body, id, err := preinvokeCanonical(f)
	if err != nil || root == nil {
		return "", "", "", ErrAcquisition
	}
	sourceSelector := "query-occurrence-test-source-v1-" + strings.TrimPrefix(f.SourceDigest, "sha256:") + ".bin"
	sourceReceipt, sourceErr := publication.PublishBoundFile(root, sourceSelector, f.Source, func(got []byte) error {
		if !bytes.Equal(got, f.Source) {
			return ErrAcquisition
		}
		return nil
	})
	if sourceErr != nil || !verifiedTargetPublication(sourceReceipt, sourceSelector, f.SourceDigest, len(f.Source)) {
		return "", "", "", ErrAcquisition
	}
	retainedSource, sourceErr := publication.ReadVerifiedBoundFile(root, sourceSelector, int64(len(f.Source)))
	if sourceErr != nil || !bytes.Equal(retainedSource, f.Source) {
		return "", "", "", ErrAcquisition
	}
	digest := privateDigest(body)
	selector := "query-occurrence-test-v1-" + digest[7:] + ".json"
	receipt, err := publication.PublishBoundFile(root, selector, body, func(got []byte) error {
		if !bytes.Equal(got, body) {
			return ErrAcquisition
		}
		return nil
	})
	if err != nil || !verifiedTargetPublication(receipt, selector, digest, len(body)) {
		return "", "", "", ErrAcquisition
	}
	got, err := publication.ReadVerifiedBoundFile(root, selector, int64(len(body)))
	if err != nil || !bytes.Equal(got, body) {
		return "", "", "", ErrAcquisition
	}
	return selector, digest, id, nil
}

func verifyPreinvokeOccurrence(root *publication.Root, selector, digest string, f preinvokeFacts, claimed string) bool {
	if root == nil || selector == "" || digest == "" || claimed == "" {
		return false
	}
	expected, id, err := preinvokeCanonical(f)
	if err != nil || claimed != id || digest != privateDigest(expected) || selector != "query-occurrence-test-v1-"+digest[7:]+".json" {
		return false
	}
	sourceSelector := "query-occurrence-test-source-v1-" + strings.TrimPrefix(f.SourceDigest, "sha256:") + ".bin"
	retained, sourceErr := publication.ReadVerifiedBoundFile(root, sourceSelector, int64(len(f.Source)))
	if sourceErr != nil || !bytes.Equal(retained, f.Source) || privateDigest(retained) != f.SourceDigest {
		return false
	}
	got, err := publication.ReadVerifiedBoundFile(root, selector, int64(len(expected)))
	return err == nil && bytes.Equal(got, expected)
}
