package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
	"lsp-trace/internal/verification"
)

func TestCustodyPublicationDirectoryFailure(t *testing.T) {
	for _, operational := range []bool{false, true} {
		for _, failOn := range []int{0, 1, 2} {
			t.Run(map[bool]string{false: "legacy", true: "operational"}[operational]+string(rune('0'+failOn)), func(t *testing.T) {
				dir := t.TempDir()
				root, err := publication.OpenRoot(dir)
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				input := ProductionInput{Root: dir, Source: "legacy"}
				calls := 0
				syncDir := func() (bool, error) {
					calls++
					// The file was already installed and synced; that does not establish
					// directory durability. Both publication call sites must ask separately.
					if _, err := os.Stat(filepath.Join(dir, "artifact.json")); err != nil {
						t.Fatal(err)
					}
					if calls == failOn {
						return false, errors.New("injected directory sync failure")
					}
					return root.SyncDirectory()
				}
				handlers := productionHandlers(input, root, syncDir)
				evidence := &custodyevidence.Evidence{}
				if operational {
					src := t.TempDir()
					if err := os.WriteFile(filepath.Join(src, "input"), []byte("actual"), 0600); err != nil {
						t.Fatal(err)
					}
					input.Source = ""
					input.Operational = &OperationalInput{SourceRoot: src, Inputs: []OperationalFile{{Path: "input", Class: source.InputSource}}}
					handlers = operationalHandlers(input, root, nil, evidence, syncDir)
				}
				op, err := operation.NewCustodyOperation(handlers)
				if err != nil {
					t.Fatal(err)
				}
				_, failure := op.ExecuteCustody(context.Background(), operation.CustodyRequest{OperationID: "durability", Input: []byte(`{}`)})
				if (failure != nil) != (failOn != 0) {
					t.Fatalf("directory failure did not control outcome: %v", failure)
				}
				wantCalls := 2
				if failOn == 1 {
					wantCalls = 1
				}
				if calls != wantCalls {
					t.Fatalf("ASSERT_DIRECTORY_NOT_FILE_SYNC: calls=%d want=%d", calls, wantCalls)
				}
				artifact, err := os.ReadFile(filepath.Join(dir, "artifact.json"))
				if err != nil {
					t.Fatal(err)
				}
				receipt, err := os.ReadFile(filepath.Join(dir, "receipt.json"))
				if failOn == 1 {
					if !os.IsNotExist(err) {
						t.Fatal("receipt issued without directory sync")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if err := verification.VerifyReceipt(artifact, receipt); err != nil {
						t.Fatal(err)
					}
				}
				if operational && len(evidence.InputEvidence.Inputs) != 1 {
					t.Fatal("publication failure lost input evidence")
				}
			})
		}
	}
}
