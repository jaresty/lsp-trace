//go:build darwin

package main

import (
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
)

// Ordinary quota jobs never reinterpret or upgrade historical durable stores.
// Explicit publication-root and legacy env override are parents for this new
// namespace; every continuation returns the exact resolved child store.
func preflightCensusQuotaStore(workspace, explicit string) (string, string, error) {
	base := explicit
	if base == "" {
		base = os.Getenv(censusDurableStoreEnv)
	}
	if base != "" {
		root, _, e := openSelectedCensusStore(workspace, base)
		if e != nil {
			return "", "", e
		}
		if e = root.Close(); e != nil {
			return "", "", e
		}
	} else {
		var e error
		base, e = os.UserConfigDir()
		if e != nil {
			return "", "", e
		}
		base, e = censusDefaultConfigDirectory(base, workspace)
		if e != nil {
			return "", "", e
		}
		for _, name := range []string{"lsp-trace", "census"} {
			base, e = censusOwnedDirectory(base, name, false)
			if e != nil {
				return "", "", e
			}
		}
	}
	dir, e := censusOwnedDirectory(base, publication.StorageDirectoryName, false)
	if e != nil {
		return "", "", e
	}
	if censusPathWithin(workspace, dir) {
		return "", "", publication.ErrStorageIntegrity
	}
	canonical, e := filepath.EvalSymlinks(dir)
	if e != nil {
		return "", "", e
	}
	return canonical, base, nil
}
