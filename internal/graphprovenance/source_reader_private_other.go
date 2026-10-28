//go:build !darwin && !linux

package graphprovenance

import "context"

// No os.Root/OpenFile fallback: the private read boundary requires qualified
// descriptor-relative no-follow traversal, not historical source semantics.
type sourceRootPrivateV1 struct{}

func openSourceRootPrivateV1(context.Context, string) (*sourceRootPrivateV1, error) {
	return nil, errSourcePrivateUnsupportedV1
}
func (*sourceRootPrivateV1) Close() error { return nil }
func readSourcePrivateV1(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) {
	return nil, errSourcePrivateUnsupportedV1
}
