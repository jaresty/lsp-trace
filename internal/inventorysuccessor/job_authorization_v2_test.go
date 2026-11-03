package inventorysuccessor

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func sealTestV2(t *testing.T, v any) []byte {
	t.Helper()
	b, e := sealV2(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestJobV2Canonical(t *testing.T) {
	j, a, _ := fixtureV2(t)
	r := reservationV2{Schema: reservationSchemaV2, Job: j.Identity, Authorization: a.Identity, Revision: 1, PreviousHead: hashLabelV2("predecessor"), Sequence: 2, Attempt: hashLabelV2("attempt"), Phase: "RESOURCE_WINDOW", Tranche: countersV2{Calls: 1}, Charged: countersV2{Calls: 1}, ClaimedAt: 11, Deadline: 20, Disposition: "PENDING"}
	h := resourceHeadV2{Schema: resourceHeadSchemaV2, Job: j, Sequence: 1, Previous: noneV2, Authorization: a.Identity, AuthorizationSelector: recordSelectorV2(authorizationsNamespaceV2, a.Identity), ReservationSelector: noneV2, Attempt: noneV2, Disposition: "READY", Event: "INITIALIZE", At: 10}
	for _, v := range []any{&j, &a, &r, &h} {
		name, _, limit := metaV2(v)
		t.Run(name, func(t *testing.T) {
			raw := sealTestV2(t, v)
			fresh := func() any { return reflect.New(reflect.TypeOf(v).Elem()).Interface() }
			out := fresh()
			if err := parseV2(raw, out); err != nil || !reflect.DeepEqual(v, out) {
				t.Fatalf("full canonical roundtrip: %v", err)
			}
			_, id, _ := metaV2(v)
			copy := fresh()
			if json.Unmarshal(raw, copy) != nil {
				t.Fatal("decode")
			}
			schema, empty, _ := metaV2(copy)
			*empty = ""
			u, _ := json.Marshal(copy)
			want := hashLabelV2("inventory-successor/private/v2\x00" + schema + "\x00" + string(append(u, '\n')))
			if *id != want {
				t.Fatal("domain/LF seal mismatch")
			}
			cases := map[string][]byte{
				"missing LF": raw[:len(raw)-1], "double LF": append(bytes.Clone(raw), '\n'), "leading space": append([]byte(" "), raw...), "trailing JSON": append(bytes.Clone(raw), []byte("{}")...),
				"duplicate":     bytes.Replace(raw, []byte("{\"schema\":"), []byte("{\"schema\":\"duplicate\",\"schema\":"), 1),
				"unknown":       bytes.Replace(raw, []byte("{\"schema\":"), []byte("{\"unknown\":0,\"schema\":"), 1),
				"case alias":    bytes.Replace(raw, []byte("\"schema\""), []byte("\"Schema\""), 1),
				"escaped alias": bytes.Replace(raw, []byte("\"schema\""), []byte("\"\\u0073chema\""), 1),
				"oversize":      bytes.Repeat([]byte(" "), limit+1), "invalid UTF8": {0xff, '\n'},
				"reordered": bytes.Replace(raw, []byte("{\"schema\":"), []byte("{ \"schema\":"), 1),
			}
			for label, b := range cases {
				t.Run(label, func(t *testing.T) {
					if parseV2(b, fresh()) == nil {
						t.Fatal("invalid canonical accepted")
					}
				})
			}
			if _, err := ParseHeadRevision(raw); err == nil {
				t.Fatal("v1 parser widened")
			}
		})
	}
}

func TestJobV2SemanticResealRefusals(t *testing.T) {
	j, a, _ := fixtureV2(t)
	for name, mutate := range map[string]func(*jobV2){
		"v1 import": func(x *jobV2) { x.Origin = "V1_IMPORT" }, "scope": func(x *jobV2) { x.Scope = "" }, "source": func(x *jobV2) { x.SourceAuthorization = noneV2 }, "empty policy": func(x *jobV2) { x.ProviderPolicies = []string{} }, "duplicate policy": func(x *jobV2) { x.SemanticPolicies = []string{x.SemanticPolicies[0], x.SemanticPolicies[0]} }, "unsorted": func(x *jobV2) {
			x.EvidencePolicies = []string{hashLabelV2("a"), hashLabelV2("b")}
			sort.Sort(sort.Reverse(sort.StringSlice(x.EvidencePolicies)))
		},
	} {
		t.Run(name, func(t *testing.T) {
			x := j
			mutate(&x)
			if _, e := sealV2(&x); e == nil {
				t.Fatal("semantic reseal accepted")
			}
		})
	}
	for name, mutate := range map[string]func(*authorizationV2){
		"zero revision": func(x *authorizationV2) { x.Revision = 0 }, "no windows": func(x *authorizationV2) { x.MaxWindows = 0 }, "expiry": func(x *authorizationV2) { x.Expires = 0 }, "unbounded deadline": func(x *authorizationV2) { x.MaxWindowNS = math.MaxUint64 }, "snapshot import": func(x *authorizationV2) { x.Charged.Calls = 1 }, "authority flag": func(x *authorizationV2) { x.Assurance = "AUTHENTICATED" }, "missing chain": func(x *authorizationV2) { x.Revision = 2 }, "per-window": func(x *authorizationV2) { x.MaxTranche.Calls = 9 },
	} {
		t.Run(name, func(t *testing.T) {
			x := a
			mutate(&x)
			if _, e := sealV2(&x); e == nil {
				t.Fatal("authorization reseal accepted")
			}
		})
	}
	raw := sealTestV2(t, &a)
	for _, n := range []string{"-0", "1.0", "1e0", "18446744073709551616", "null"} {
		b := bytes.Replace(raw, []byte("\"revision\":1"), []byte("\"revision\":"+n), 1)
		var x authorizationV2
		if parseV2(b, &x) == nil {
			t.Fatalf("noncanonical uint64 %s accepted", n)
		}
	}
	var unsupported struct{}
	if _, e := sealV2(&unsupported); e == nil {
		t.Fatal("open record type")
	}
	old := j.Identity
	j.Scope = hashLabelV2("other-scope")
	sealTestV2(t, &j)
	if old == j.Identity {
		t.Fatal("scope missing from stable identity")
	}
	for _, raw := range []string{strings.Repeat("x", recordLimitV2+1), "{}\n"} {
		var x jobV2
		if parseV2([]byte(raw), &x) == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}

func TestJobV2ProducerShapeBeforeSerialization(t *testing.T) {
	for _, kind := range []string{"schema", "policy-count", "policy-label", "authorization"} {
		t.Run(kind, func(t *testing.T) {
			j, a, _ := fixtureV2(t)
			var record any = &j
			switch kind {
			case "schema":
				j.Schema = strings.Repeat("x", recordLimitV2+1)
			case "policy-count":
				j.EvidencePolicies = make([]string, 33)
			case "policy-label":
				j.ProviderPolicies = []string{strings.Repeat("x", recordLimitV2+1)}
			case "authorization":
				a.Decision = strings.Repeat("x", recordLimitV2+1)
				record = &a
			}
			_, id, _ := metaV2(record)
			before := *id
			calls := 0
			_, err := sealWithMarshalV2(record, func(v any) ([]byte, error) {
				calls++
				return json.Marshal(v)
			})
			if err == nil || calls != 0 || *id != before {
				t.Fatalf("ASSERT_JOB_V2_PRODUCER_SHAPE_BEFORE_MARSHAL err=%v marshal_calls=%d identity_unchanged=%t", err, calls, *id == before)
			}
		})
	}
	j, _, _ := fixtureV2(t)
	calls := 0
	raw, err := sealWithMarshalV2(&j, func(v any) ([]byte, error) {
		calls++
		return json.Marshal(v)
	})
	var decoded jobV2
	if err != nil || calls != 2 || parseV2(raw, &decoded) != nil || decoded.Identity != j.Identity {
		t.Fatalf("valid canonical producer changed: err=%v calls=%d", err, calls)
	}
}

func TestJobV2CheckedCurrencies(t *testing.T) {
	for i := 0; i < 14; i++ {
		v := [14]uint64{}
		v[i] = math.MaxUint64
		c := countersFromV2(v)
		v[i] = 1
		one := countersFromV2(v)
		if _, err := addCountersV2(c, one); err == nil {
			t.Fatalf("currency %d overflow accepted", i)
		}
		if !c.within(c) || c.within(one) {
			t.Fatalf("currency %d bound wrong", i)
		}
		got, err := addCountersV2(one, one)
		v[i] = 2
		if err != nil || got != countersFromV2(v) {
			t.Fatalf("currency %d sum", i)
		}
	}
	if _, err := nextV2(math.MaxUint64); err == nil {
		t.Fatal("sequence/revision/window overflow accepted")
	}
}

func fixtureV2(t *testing.T) (jobV2, authorizationV2, decisionV2) {
	t.Helper()
	j := jobV2{Schema: jobSchemaV2, StableID: hashLabelV2("stable-job"), Application: hashLabelV2("app"), Scope: hashLabelV2("scope"), SourceAuthorization: hashLabelV2("source-authorization"), RevisionPolicy: hashLabelV2("revision-policy"), EvidencePolicies: []string{hashLabelV2("evidence")}, ProviderPolicies: []string{hashLabelV2("provider")}, SemanticPolicies: []string{hashLabelV2("semantic")}, Privacy: hashLabelV2("privacy"), Retention: hashLabelV2("retention"), ResourceRules: hashLabelV2("rules"), Origin: "NEW_EMPTY"}
	if _, err := sealV2(&j); err != nil {
		t.Fatal(err)
	}
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: countersV2{Calls: 8}, MaxWindows: 4, Expires: 100, MaxTranche: countersV2{Calls: 4}, MaxWindowNS: 30, Decision: hashLabelV2("decision"), Assurance: assuranceV2}
	if _, err := sealV2(&a); err != nil {
		t.Fatal(err)
	}
	return j, a, decisionV2{Job: j.Identity, Authorization: a.Identity, Reference: a.Decision}
}
