package adr0007locationv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

type AttemptKey struct {
	Case, Role string
	Ordinal    int
}
type Attempt struct {
	Key        AttemptKey
	Raw        []byte
	Digest     string
	ByteLength int
}
type DeliveryConflict struct {
	Key                       AttemptKey
	FirstDigest, SecondDigest string
}
type Review struct {
	SchemaValid, OutcomeMatches, AccountingComplete, WitnessesConcrete, BindingExact bool
	Verdict                                                                          bool
}
type Ledger struct {
	Attempts  []Attempt
	Conflicts []DeliveryConflict
}

func CanonicalJSON(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	return append(b, '\n'), nil
}
func StrictDecode(raw []byte, v any) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing JSON")
	}
	c, err := CanonicalJSON(v)
	if err != nil || !bytes.Equal(c, raw) {
		return errors.New("non-canonical JSON")
	}
	return nil
}
func CommitAttempt(l Ledger, k AttemptKey, raw []byte) (Ledger, error) {
	var x any
	if err := StrictDecode(raw, &x); err != nil {
		return l, err
	}
	sum := sha256.Sum256(raw)
	dg := "sha256:" + hex.EncodeToString(sum[:])
	for _, a := range l.Attempts {
		if a.Key == k {
			if a.Digest != dg {
				l.Conflicts = append(l.Conflicts, DeliveryConflict{k, a.Digest, dg})
			}
			return cloneLedger(l), nil
		}
	}
	l.Attempts = append(l.Attempts, Attempt{k, append([]byte(nil), raw...), dg, len(raw)})
	return cloneLedger(l), nil
}
func RecomputeReview(r Review) Review {
	r.Verdict = r.SchemaValid && r.OutcomeMatches && r.AccountingComplete && r.WitnessesConcrete && r.BindingExact
	return r
}
func cloneLedger(l Ledger) Ledger {
	o := Ledger{Attempts: append([]Attempt(nil), l.Attempts...), Conflicts: append([]DeliveryConflict(nil), l.Conflicts...)}
	for i := range o.Attempts {
		o.Attempts[i].Raw = append([]byte(nil), o.Attempts[i].Raw...)
	}
	return o
}
