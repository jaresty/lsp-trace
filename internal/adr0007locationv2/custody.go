package adr0007locationv2

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

type AttemptKey struct {
	CaseID  string `json:"caseId"`
	Role    string `json:"role"`
	Ordinal int    `json:"ordinal"`
}
type RawRecord struct {
	Schema     string     `json:"schema"`
	Key        AttemptKey `json:"key"`
	Raw        []byte     `json:"raw"`
	Digest     string     `json:"digest"`
	ByteLength int        `json:"byteLength"`
}
type Attempt struct {
	Schema        string     `json:"schema"`
	Key           AttemptKey `json:"key"`
	RawDigest     string     `json:"rawDigest"`
	RawByteLength int        `json:"rawByteLength"`
}
type Conflict struct {
	Schema       string     `json:"schema"`
	Key          AttemptKey `json:"key"`
	FirstDigest  string     `json:"firstDigest"`
	SecondDigest string     `json:"secondDigest"`
}
type Review struct {
	Schema             string     `json:"schema"`
	Key                AttemptKey `json:"key"`
	SchemaValid        bool       `json:"schemaValid"`
	OutcomeMatches     bool       `json:"outcomeMatches"`
	AccountingComplete bool       `json:"accountingComplete"`
	WitnessesConcrete  bool       `json:"witnessesConcrete"`
	BindingExact       bool       `json:"bindingExact"`
	Verdict            bool       `json:"verdict"`
}
type Account struct {
	Schema           string `json:"schema"`
	ProducerAttempts int    `json:"producerAttempts"`
	ReviewerAttempts int    `json:"reviewerAttempts"`
	Conflicts        int    `json:"conflicts"`
	Reviews          int    `json:"reviews"`
}
type Ledger struct {
	Schema    string      `json:"schema"`
	Raw       []RawRecord `json:"raw"`
	Attempts  []Attempt   `json:"attempts"`
	Conflicts []Conflict  `json:"conflicts"`
	Reviews   []Review    `json:"reviews"`
	Account   Account     `json:"account"`
}

const LedgerSchema = "lsp-trace.adr0007.location.ledger.v2"

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
	if err := rejectDuplicateKeys(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing JSON")
	}
	c, e := CanonicalJSON(v)
	if e != nil || !bytes.Equal(c, raw) {
		return errors.New("non-canonical JSON")
	}
	return nil
}
func rejectDuplicateKeys(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		switch x := t.(type) {
		case json.Delim:
			if x == '{' {
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok {
						return errors.New("object key")
					}
					if seen[s] {
						return fmt.Errorf("duplicate key %q", s)
					}
					seen[s] = true
					if e = walk(); e != nil {
						return e
					}
				}
				_, e = d.Token()
				return e
			}
			if x == '[' {
				for d.More() {
					if e := walk(); e != nil {
						return e
					}
				}
				_, e = d.Token()
				return e
			}
		}
		return nil
	}
	if e := walk(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func Commit(l Ledger, k AttemptKey, raw []byte) (Ledger, error) {
	if l.Raw == nil {
		l.Raw = []RawRecord{}
	}
	if l.Attempts == nil {
		l.Attempts = []Attempt{}
	}
	if l.Conflicts == nil {
		l.Conflicts = []Conflict{}
	}
	if l.Reviews == nil {
		l.Reviews = []Review{}
	}
	if k.CaseID == "" || (k.Role != "producer" && k.Role != "reviewer") || k.Ordinal < 1 {
		return l, errors.New("invalid key")
	}
	if !utf8.Valid(raw) || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return l, errors.New("raw must be UTF-8 canonical line JSON")
	}
	if err := rejectDuplicateKeys(raw); err != nil {
		return l, err
	}
	var doc json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return l, err
	}
	sum := sha256.Sum256(raw)
	dg := "sha256:" + hex.EncodeToString(sum[:])
	for _, a := range l.Attempts {
		if a.Key == k {
			if a.RawDigest != dg {
				l.Conflicts = append(l.Conflicts, Conflict{"lsp-trace.adr0007.location.conflict.v2", k, a.RawDigest, dg})
			}
			recount(&l)
			return cloneLedger(l), nil
		}
	}
	l.Schema = LedgerSchema
	l.Raw = append(l.Raw, RawRecord{"lsp-trace.adr0007.location.raw.v2", k, append([]byte(nil), raw...), dg, len(raw)})
	l.Attempts = append(l.Attempts, Attempt{"lsp-trace.adr0007.location.attempt.v2", k, dg, len(raw)})
	recount(&l)
	return cloneLedger(l), nil
}
func AddReview(l Ledger, r Review) (Ledger, error) {
	if r.Key.Role != "reviewer" || r.Key.Ordinal < 1 {
		return l, errors.New("invalid reviewer key")
	}
	r.Schema = "lsp-trace.adr0007.location.review.v2"
	r.Verdict = r.SchemaValid && r.OutcomeMatches && r.AccountingComplete && r.WitnessesConcrete && r.BindingExact
	l.Reviews = append(l.Reviews, r)
	recount(&l)
	return cloneLedger(l), nil
}
func recount(l *Ledger) {
	l.Account = Account{Schema: "lsp-trace.adr0007.location.account.v2", Conflicts: len(l.Conflicts), Reviews: len(l.Reviews)}
	for _, a := range l.Attempts {
		if a.Key.Role == "producer" {
			l.Account.ProducerAttempts++
		} else {
			l.Account.ReviewerAttempts++
		}
	}
}
func cloneLedger(l Ledger) Ledger {
	o := l
	o.Raw = make([]RawRecord, len(l.Raw))
	copy(o.Raw, l.Raw)
	for i := range o.Raw {
		o.Raw[i].Raw = append([]byte(nil), o.Raw[i].Raw...)
	}
	o.Attempts = make([]Attempt, len(l.Attempts))
	copy(o.Attempts, l.Attempts)
	o.Conflicts = make([]Conflict, len(l.Conflicts))
	copy(o.Conflicts, l.Conflicts)
	o.Reviews = make([]Review, len(l.Reviews))
	copy(o.Reviews, l.Reviews)
	return o
}
