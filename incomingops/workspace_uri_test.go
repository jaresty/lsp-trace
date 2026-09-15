package incomingops

import "testing"

func TestWorkspaceRootURIDiagnostic(t *testing.T) {
	const assertion = "ASSERT_TRAVERSAL_WORKSPACE_ROOT_URI_REJECTED"
	if err := ValidateDocumentURI("/workspace/project", "file:///workspace/project"); err == nil || err.Error() != "uri must identify an exact document; workspace-root URI is invalid" {
		t.Fatalf("%s: err=%v", assertion, err)
	}
	if err := ValidateDocumentURI("/workspace/project", "file:///workspace/project/main.go"); err != nil {
		t.Fatalf("%s_VALID_DOCUMENT: err=%v", assertion, err)
	}
	if err := ValidateDocumentURI("/workspace/project", "untitled:buffer"); err != nil {
		t.Fatalf("%s_NON_FILE_UNCHANGED: err=%v", assertion, err)
	}
}
