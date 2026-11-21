package publication

import (
	"errors"
	"io"
)

// VisitNamesBounded visits only immediate entry names through the pinned root.
// It reads no entry metadata/content, follows no child, and retains at most 128
// names. The limit includes unrelated entries: exceeding it fails closed rather
// than claiming a complete namespace inspection. One extra name detects overflow.
// A fresh descriptor gives concurrent callers independent directory offsets.
func (r *Root) VisitNamesBounded(limit uint64, visit func(string) error) (err error) {
	if limit == 0 || limit > 1<<25 || visit == nil {
		return errors.New("invalid directory inspection limit")
	}
	if err := r.ValidatePrivate(); err != nil {
		return err
	}
	f, err := r.handle.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	var seen uint64
	for {
		names, e := f.Readdirnames(int(min(uint64(128), limit-seen+1)))
		for _, name := range names {
			if seen == limit {
				return errors.New("directory inspection limit exceeded")
			}
			seen++
			if err := visit(name); err != nil {
				return err
			}
		}
		if errors.Is(e, io.EOF) {
			return nil
		}
		if e != nil {
			return e
		}
		if len(names) == 0 {
			return errors.New("directory inspection made no progress")
		}
	}
}
