// Package strictjson checks raw JSON before a decoder can discard members.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RejectDuplicates rejects duplicate decoded object names at every depth,
// including objects inside arrays and escaped spellings of the same name.
// It validates one complete JSON value and never rewrites the input bytes.
func RejectDuplicates(raw []byte) error {
	if !json.Valid(raw) {
		return fmt.Errorf("invalid JSON value")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name := key.(string)
				if seen[name] {
					return fmt.Errorf("duplicate JSON member %q", name)
				}
				seen[name] = true
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
		}
		_, err = d.Token()
		return err
	}
	return value()
}
