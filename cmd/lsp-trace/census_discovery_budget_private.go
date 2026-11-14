package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/sessionruntime"
	"reflect"
	"time"
)

var errCensusInventoryPrivate = errors.New("private connected inventory refused")

const censusInventorySourceRevision = "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73"

func censusInventoryDigest(raw []byte) string {
	h := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(h[:])
}
func censusInventoryHash(s string) bool {
	if len(s) != 71 || s[:7] != "sha256:" {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil && s == "sha256:"+string(bytes.ToLower([]byte(s[7:])))
}

// Quote every typed field before JSON allocation, including direct Go callers.
// A strict upper bound is deliberately conservative; it cannot expand a grant.
func censusInventoryMarshal(v any, limit int) ([]byte, error) {
	left := int64(limit)
	var quote func(reflect.Value, int) bool
	quote = func(x reflect.Value, depth int) bool {
		if depth > 32 || left < 0 {
			return false
		}
		if !x.IsValid() {
			left -= 4
			return left >= 0
		}
		if x.Type() == reflect.TypeOf(time.Time{}) {
			left -= 64
			return left >= 0
		}
		switch x.Kind() {
		case reflect.Interface, reflect.Pointer:
			if x.IsNil() {
				left -= 4
				return left >= 0
			}
			return quote(x.Elem(), depth+1)
		case reflect.String:
			left -= int64(x.Len())*6 + 2
		case reflect.Bool:
			left -= 5
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			left -= 24
		case reflect.Struct:
			left -= 2
			for i := 0; i < x.NumField(); i++ {
				left -= int64(len(x.Type().Field(i).Name))*6 + 4
				if !quote(x.Field(i), depth+1) {
					return false
				}
			}
		case reflect.Slice, reflect.Array:
			if x.Len() > 100000 {
				return false
			}
			left -= 2
			for i := 0; i < x.Len(); i++ {
				left--
				if !quote(x.Index(i), depth+1) {
					return false
				}
			}
		default:
			return false
		}
		return left >= 0
	}
	if limit <= 0 || !quote(reflect.ValueOf(v), 0) {
		return nil, errCensusInventoryPrivate
	}
	raw, e := json.Marshal(v)
	if e != nil || len(raw) > limit {
		return nil, errCensusInventoryPrivate
	}
	return raw, nil
}
func censusInventoryDecode(raw []byte, v any, limit int) error {
	if len(raw) == 0 || len(raw) > limit {
		return errCensusInventoryPrivate
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		return errCensusInventoryPrivate
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, raw) {
		return errCensusInventoryPrivate
	}
	return nil
}
func censusPrivateDiscoveryLimits() sessionruntime.PrivateDiscoveryLimits {
	return sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (1<<20 + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
}
func censusInventoryCheckContext(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return errCensusInventoryPrivate
	}
	return nil
}
