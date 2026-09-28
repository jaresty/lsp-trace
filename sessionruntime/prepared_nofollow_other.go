//go:build !darwin

package sessionruntime

import "errors"

func readPreparedNoFollow(_, _ string) ([]byte, error) {
	return nil, errors.New("descriptor-bound no-follow prepared supply unsupported")
}
