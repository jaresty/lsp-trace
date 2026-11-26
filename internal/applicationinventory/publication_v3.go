package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"strings"

	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
)

type ArtifactV3 struct {
	Name  string
	Bytes []byte
}
type PublishedArtifactV3 struct {
	Name, SHA256 string
	Bytes        int
}
type PublicationManifestV3 struct {
	Version                                          string
	Authority                                        int
	Accepted                                         bool
	Status, Feature, Consumer, Completeness, Absence string
	Artifacts                                        []PublishedArtifactV3
	CallerBytes                                      uint64
	SemanticReservedNS                               int64
}

// publicationFaultV3 is test-only package-local failure injection. Production
// PublishV3 passes nil; no global hook or authority constructor exists.
type publicationFaultV3 func(string) error

func PublishV3(ctx context.Context, destination string, artifacts []ArtifactV3, budget *sc.CallerBudgetV3) (PublicationManifestV3, error) {
	return publishV3(ctx, destination, artifacts, budget, nil)
}
func publishV3(ctx context.Context, destination string, artifacts []ArtifactV3, budget *sc.CallerBudgetV3, fault publicationFaultV3) (result PublicationManifestV3, err error) {
	result = PublicationManifestV3{Version: "lsp-trace.application-inventory-publication.v3", Status: "PROVISIONAL", Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Artifacts: []PublishedArtifactV3{}}
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(destination) || filepath.Clean(destination) != destination || len(destination) > 4096 || len(artifacts) == 0 || len(artifacts) > 8 {
		return result, ErrInventoryV3
	}
	base, parent := filepath.Base(destination), filepath.Dir(destination)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return result, ErrInventoryV3
	}
	var total uint64 = 16384
	seen := map[string]bool{"manifest.json": true}
	for _, a := range artifacts {
		if len(a.Name) == 0 || len(a.Name) > 128 || strings.ContainsAny(a.Name, "/\\") || a.Name == "." || a.Name == ".." || publication.StorageReservedSelector(a.Name) || seen[a.Name] || len(a.Bytes) == 0 || len(a.Bytes) > 8<<20 {
			return result, ErrInventoryV3
		}
		seen[a.Name] = true
		total += uint64(len(a.Bytes))
	}
	if total > 64<<20 || budget.Reserve(ctx, total*4+16384) != nil {
		return result, ErrInventoryV3
	}
	result.CallerBytes, _ = budget.Usage()
	_, semantic := budget.Usage()
	result.SemanticReservedNS = int64(semantic)
	// Parent is explicit, existing, canonical and free of symlink components. All
	// path checks/open/write/readback work has a pre-effects reservation above.
	resolved, e := filepath.EvalSymlinks(parent)
	if e != nil || resolved != parent {
		return result, ErrInventoryV3
	}
	// Reserve all final artifact/manifest lengths before mkdir or any write.
	qr, e := publication.OpenRoot(parent)
	if e != nil {
		return result, e
	}
	quota, e := publication.ReserveStorage(ctx, qr, total)
	e = errors.Join(e, qr.Close())
	if e != nil {
		if quota != nil {
			e = errors.Join(e, quota.Complete(false))
		}
		return PublicationManifestV3{}, e
	}
	defer func() {
		if qe := quota.Complete(err == nil); qe != nil {
			err = errors.Join(err, qe)
		}
		if err != nil {
			result = PublicationManifestV3{}
		}
	}()
	// Snapshot mutable byte collections before external effects.
	owned := make([]ArtifactV3, len(artifacts))
	for i, a := range artifacts {
		owned[i] = ArtifactV3{a.Name, append([]byte(nil), a.Bytes...)}
	}
	p, e := os.OpenRoot(parent)
	if e != nil {
		return result, e
	}
	created := false
	var root *os.Root
	defer func() {
		if root != nil {
			err = errors.Join(err, root.Close())
		}
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
		if err != nil && created {
			err = errors.Join(err, p.RemoveAll(base))
		}
		err = errors.Join(err, p.Close())
		if err != nil {
			result = PublicationManifestV3{}
		}
	}()
	if e = p.Mkdir(base, 0700); e != nil {
		return result, e
	}
	created = true
	root, e = p.OpenRoot(base)
	if e != nil {
		return result, e
	}
	check := func(phase string) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if fault != nil {
			return fault(phase)
		}
		return nil
	}
	write := func(name string, data []byte) error {
		if e := check("write:" + name); e != nil {
			return e
		}
		f, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		n, e := f.Write(data)
		if n != len(data) {
			e = errors.Join(e, io.ErrShortWrite)
		}
		e = errors.Join(e, f.Sync(), f.Close())
		if e != nil {
			return e
		}
		if e = check("readback:" + name); e != nil {
			return e
		}
		f, e = root.Open(name)
		if e != nil {
			return e
		}
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() != int64(len(data)) {
			return errors.Join(e, f.Close(), ErrInventoryV3)
		}
		raw, e := io.ReadAll(io.LimitReader(f, int64(len(data))+1))
		e = errors.Join(e, f.Close())
		if e != nil || !bytes.Equal(raw, data) {
			return errors.Join(e, ErrInventoryV3)
		}
		if e := quota.ObserveAnonymous(nil); e != nil {
			return e
		}
		return check("verified:" + name)
	}
	for _, a := range owned {
		if e = write(a.Name, a.Bytes); e != nil {
			return result, e
		}
		result.Artifacts = append(result.Artifacts, PublishedArtifactV3{a.Name, cat.NativeDigestV3(a.Bytes), len(a.Bytes)})
	}
	// Manifest is the sole success marker; it is created only after every owned
	// artifact has exact readback. Cleanup/cancellation errors erase success.
	raw, e := json.Marshal(result)
	if e != nil {
		return result, e
	}
	if e = write("manifest.json", raw); e != nil {
		return result, e
	}
	if e = check("cleanup"); e != nil {
		return result, e
	}
	return result, nil
}
