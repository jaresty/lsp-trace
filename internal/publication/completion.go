package publication

import "errors"

const CodeCompletionInputInvalid = "COMPLETION_INPUT_INVALID"

// AdmittedArtifact is the transport-neutral output of an already-admitted
// pipeline. CanonicalBytes are the exact bytes owned by the upstream artifact
// algorithm; completion never re-encodes them.
type AdmittedArtifact struct {
	CanonicalBytes     []byte
	ArtifactSchemaID   string
	Generation         string
	ArtifactReferences []string
}

// Completion is exact-byte publication custody metadata.
type Completion struct {
	Digest             string
	ByteLength         uint64
	Selector           string
	Generation         string
	ArtifactReferences []string
}

type CompletionResult struct {
	Completion *Completion
	Failure    *Failure
}

func (r CompletionResult) Err() error {
	if r.Failure != nil {
		return r.Failure
	}
	return nil
}

// Complete publishes the exact admitted canonical bytes and projects the
// publisher's reread-verified receipt into transport-neutral completion data.
func Complete(publisher *Publisher, root *Root, selector string, admitted AdmittedArtifact) CompletionResult {
	if publisher == nil || root == nil || selector == "" || admitted.CanonicalBytes == nil || admitted.Generation == "" {
		return CompletionResult{Failure: &Failure{Stage: "completion", Code: CodeCompletionInputInvalid, Cleanup: true, cause: errors.New("invalid completion input")}}
	}
	published := publisher.Publish(Request{Root: root, Selector: selector, Bytes: admitted.CanonicalBytes, ArtifactSchemaID: admitted.ArtifactSchemaID})
	if published.Failure != nil {
		return CompletionResult{Failure: published.Failure}
	}
	return CompletionResult{Completion: &Completion{
		Digest:             published.Receipt.Digest,
		ByteLength:         published.Receipt.ByteLength,
		Selector:           selector,
		Generation:         admitted.Generation,
		ArtifactReferences: append([]string(nil), admitted.ArtifactReferences...),
	}}
}
