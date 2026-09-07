package source

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
)

// ReadRegularInputBounded returns only a complete, bounded regular-file read.
// It uses the existing platform-qualified scoped opener: symlink escape and
// nonregular files fail closed, and a FIFO replacement cannot block Unix open.
// It does not freeze a source tree or cancel slow regular-file filesystem IO.
// The caller owns the bytes; no recorder or content cache retains them.
func ReadRegularInputBounded(root *os.Root, name string, maxBytes int64) ([]byte, error) {
	if root == nil || maxBytes < 1 || maxBytes > 1<<30 {
		return nil, errors.New("invalid bounded input root or byte limit")
	}
	if name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00:") {
		return nil, errors.New("input path must be a canonical root-relative path")
	}
	file, err := openInput(root, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("input must be a regular file")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, errors.New("input exceeds byte limit")
	}
	return content, nil
}
