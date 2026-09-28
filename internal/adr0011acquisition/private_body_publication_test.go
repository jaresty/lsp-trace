package adr0011acquisition

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func privatePublicationRoot(t *testing.T) *publication.Root {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func privatePublicationSlice(body []byte) privateBodySlice {
	return privateBodySlice{body: body, value: body[:1], offset: 0, length: 1, bodyDigest: privateDigest(body), valueDigest: privateDigest(body[:1])}
}

func TestPrivateBodyPublication(t *testing.T) {
	root := privatePublicationRoot(t)
	body := []byte(`{ "jsonrpc" : "2.0", "id" : 7, "result" : [] }`)
	var selectors []string
	for _, kind := range []string{"write", "read"} {
		r, err := publishPrivateBody(root, kind, privatePublicationSlice(body), nil)
		if err != nil || r.stage != "VERIFIED" || r.byteCount != len(body) || r.digest != privateDigest(body) || r.selector != "adr0011-references-"+kind+"-v1-"+strings.TrimPrefix(r.digest, "sha256:")+".bin" {
			t.Fatalf("ASSERT_PRIVATE_EXACT_%s: %+v %v", kind, r, err)
		}
		got, err := publication.ReadVerifiedBoundFile(root, r.selector, privateBodyPublicationLimit)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("ASSERT_PRIVATE_BODY_BYTES: %v", err)
		}
		selectors = append(selectors, r.selector)
	}
	if selectors[0] == selectors[1] {
		t.Fatal("ASSERT_PRIVATE_KIND_SEPARATION")
	}
	if r, err := publishPrivateBody(root, "write", privatePublicationSlice(body), nil); err == nil || r.stage != "ABSENT" || r.selector != "" {
		t.Fatalf("ASSERT_PRIVATE_NO_REPLACE: %+v %v", r, err)
	}
	other := append([]byte(nil), body...)
	other[1] = 'x'
	altered := func(*publication.Root, string, int64) ([]byte, error) { return other, nil }
	fresh := privatePublicationRoot(t)
	if r, err := publishPrivateBody(fresh, "write", privatePublicationSlice(body), altered); err == nil || r.stage != "COMMITTED_UNVERIFIED" || r.selector != selectors[0] || r.digest != privateDigest(body) || r.byteCount != len(body) {
		t.Fatalf("ASSERT_PRIVATE_COMMITTED_UNVERIFIED: %+v %v", r, err)
	}
	for _, s := range []privateBodySlice{{}, privatePublicationSlice(make([]byte, privateBodyPublicationLimit+1))} {
		if r, err := publishPrivateBody(root, "read", s, nil); err == nil || r.stage != "ABSENT" {
			t.Fatalf("ASSERT_PRIVATE_REJECT_INPUT: %+v %v", r, err)
		}
	}
	limit := privatePublicationSlice(bytes.Repeat([]byte{'x'}, privateBodyPublicationLimit))
	if r, err := publishPrivateBody(privatePublicationRoot(t), "read", limit, nil); err != nil || r.stage != "VERIFIED" {
		t.Fatalf("ASSERT_PRIVATE_AT_LIMIT: %+v %v", r, err)
	}
	closed := privatePublicationRoot(t)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if r, err := publishPrivateBody(closed, "write", privatePublicationSlice(body), nil); err == nil || r.stage != "ABSENT" {
		t.Fatalf("ASSERT_PRIVATE_CLOSED_ROOT: %+v %v", r, err)
	}
	if r, err := publishPrivateBody(privatePublicationRoot(t), "read", privatePublicationSlice(body), func(*publication.Root, string, int64) ([]byte, error) { return nil, errors.New("secret payload") }); err == nil || strings.Contains(err.Error(), "secret payload") || r.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_PRIVATE_REDACTED_ERROR: %+v %v", r, err)
	}
}
