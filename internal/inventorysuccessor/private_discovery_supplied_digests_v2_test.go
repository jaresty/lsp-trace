package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func suppliedDocumentsV2(op PrivateDiscoveryOperationV2) sessionruntime.PrivateDiscoverySuppliedDocuments {
	// Missing integration is an evaluated data-release failure, not a build error.
	if value, ok := any(op).(interface {
		SuppliedDocuments() sessionruntime.PrivateDiscoverySuppliedDocuments
	}); ok {
		return value.SuppliedDocuments()
	}
	return sessionruntime.PrivateDiscoverySuppliedDocuments{}
}
func TestPrivateDiscoveryV2SuppliedDigestCompletion(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "uncertain"}[uncertain], func(t *testing.T) {
			workspace, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			profile, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "synthetic", Workspace: workspace, Profile: "go", EnvironmentReference: "offline"})
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(workspace, "paid.go")
			body := []byte("package paid\r\n// retained only\n")
			if e = os.WriteFile(path, body, 0600); e != nil {
				t.Fatal(e)
			}
			c := sessionruntime.PrivateDiscoveryConstruction{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (sessionruntime.MaxDocumentSupplyBytes + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}, Members: []sessionruntime.PrivateDiscoveryMember{{Path: "paid.go", URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"}}, NewStarter: func() (sessionruntime.Starter, error) {
				return discoveryKernelStarterV2{newDiscoveryKernelChildV2()}, nil
			}}
			owner, x, binding := discoverySyntheticConfigurationV2(t, -1, &c)
			op, e := owner.ReservePartition(context.Background(), binding, hashLabelV2("paid-digest-attempt"))
			if e != nil {
				t.Fatal("SETUP_RESERVATION", e)
			}
			if suppliedDocumentsV2(op).Available {
				t.Fatal("ASSERT_PAID_OPERATION_RELEASE FAIL before execution")
			}
			if e = op.Execute(context.Background(), c, func(ctx context.Context, s *sessionruntime.PrivateDiscoverySession) error {
				if e := s.Supply(ctx, c.Members[0].URI, "go"); e != nil {
					return e
				}
				return os.Remove(path)
			}); e != nil {
				t.Fatal("SETUP_EXECUTION", e)
			}
			if suppliedDocumentsV2(op).Available {
				t.Fatal("ASSERT_PAID_OPERATION_RELEASE FAIL before completion")
			}
			if uncertain {
				owner.k.cas = func(ctx context.Context, root *publication.Root, selector string, prior publication.BoundFilePredecessor, raw []byte, verify func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
					receipt, e := publication.CompareAndReplaceBoundFile(ctx, root, selector, prior, raw, verify)
					if e == nil {
						return receipt, errors.New("controlled completion uncertainty")
					}
					return receipt, e
				}
			}
			e = op.Complete(context.Background())
			docs := suppliedDocumentsV2(op)
			if uncertain {
				if e == nil || docs.Available {
					t.Fatal("ASSERT_PAID_OPERATION_RELEASE FAIL uncertain completion")
				}
			} else {
				if e != nil || !docs.Available || docs.Count != 1 || docs.SHA256[0] != sha256.Sum256(body) {
					t.Fatal("ASSERT_PAID_OPERATION_RELEASE FAIL completed paid digest unavailable", e, docs.Available, docs.Count)
				}
				h, e := x.k.reopen(context.Background(), x.j, x.l)
				if e != nil || h.Disposition != "OBSERVED" {
					t.Fatal("ASSERT_PAID_OPERATION_RELEASE FAIL completion readback", e)
				}
			}
			t.Log("ASSERT_PAID_OPERATION_RELEASE: PASS")
		})
	}
}
