package lspwire

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

// Baseline red-fit.log retains the original compiling 4C predicate. This
// successor exercises actual production admission with the same cap and budget.
func TestPrivateDiscoveryCumulativeFit(t *testing.T) {
	const cap = 1 << 20
	remaining := uint64(512<<20) - 16*16*(cap+1)
	body := []byte(`{"jsonrpc":"2.0","method":"notice"}`)
	frame := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	for i := 0; i < 65; i++ {
		_, _, _, _, err := ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(frame), Limits{cap, 8192}, cap, func(q PrivateDiscoveryCharge) error {
			if q.Retained > remaining {
				t.Fatalf("source16-small65 fit: frame=%d requested=%d remaining=%d acceptanceCap=%d", i, q.Retained, remaining, cap)
			}
			remaining -= q.Retained
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func discoveryTestFrame(body []byte) []byte {
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}
func discoveryTestRead(body []byte) (Message, ReadFrameObservation, []byte, uint64, error) {
	return ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(discoveryTestFrame(body)), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
}
func TestPrivateDiscoveryDifferential(t *testing.T) {
	fixtures := []string{
		`{"jsonrpc":"2.0","id":1,"result":null}`,
		`{"jsonrpc":"2.0","method":"hello","unknown":{"arbitrary":[1,{},null]}}`,
		`{"JSONRPC":"2.0","metho\\u0064":"hi"}`,
		`{"jsonrpc":"2.0","id":{},"result":[ 1, "<>&  ", {"x":2} ]}`,
		`{"jsonrpc":"2.0","id":[],"error":{"code":1,"message":"first","data":null},"error":{"code":null,"message":null}}`,
		`{"jsonrpc":"2.0","id":null,"error":{"code":4,"message":"x","data":[]},"error":null,"error":{"message":"new"}}`,
		`{"jsonrpc":"2.0","id":0,"error":{"code":1.0},"error":{"code":2}}`,
		`{"jsonrpc":false,"jsonrpc":"2.0","method":"ok"}`,
		`{"jsonrpc":"2.0","id":2,"error":{"code":9223372036854775807,"message":"\\ud800"}}`,
		`{"jsonrpc":"2.0","id":2,"error":{"code":-9223372036854775808,"message":"\\udc00"}}`,
		`{"jsonrpc":"2.0","id":2,"error":{"code":9223372036854775808}}`,
		`{"jsonrpc":"2.0","id":2,"error":{"code":-9223372036854775809}}`,
		`{"jsonrpc":"2.0","id":2,"error":{"code":"bad"},"error":null,"result":null}`,
		`{"jsonrpc":"2.0","method":"a","method":null}`,
		`{"jsonrpc":"2.0","id":1,"result":null,"result":{}}`,
		`{"jsonrpc":"1.0","method":"x"}`, `null`, `[]`, `1`, `"x"`, `{}`, `{"jsonrpc":"2.0","method":"x"} true`,
		`{"jsonrpc":"2.0","method":"x",}`, `{"jsonrpc":"2.0","method":"x","u":[1,]}`,
		`{"jsonrpc":"2.0","method":"x","u":01}`, `{"jsonrpc":"2.0","method":"x","u":1e}`,
		`{"jsonrpc":"2.0","method":"x","u":"\\uZZZZ"}`,
	}
	fixtures = append(fixtures, "{\"jsonrpc\":\"2.0\",\"method\":\"\xff\",\"params\":{\"x\":\"\xff<\"}}")
	fixtures = append(fixtures, `{"jsonrpc":"2.0","method":"x","params":`+strings.Repeat("[", 9999)+`0`+strings.Repeat("]", 9999)+`}`)
	fixtures = append(fixtures, `{"jsonrpc":"2.0","method":"x","params":`+strings.Repeat("[", 10000)+`0`+strings.Repeat("]", 10000)+`}`)
	fixtures = append(fixtures, `{"jsonrpc":"2.0","method":"x","params":"`+strings.Repeat("a", (1<<20)-100)+`"}`)
	categories := []error{ErrMalformedJSON, ErrWrongVersion, ErrInvalidMessage}
	for i, body := range fixtures {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			wire := discoveryTestFrame([]byte(body))
			old, oe := NewReader(bytes.NewReader(wire), Limits{1 << 20, 8192}).Read()
			got, obs, raw, count, err := discoveryTestRead([]byte(body))
			if (oe == nil) != (err == nil) {
				t.Fatalf("acceptance old=%v new=%v", oe, err)
			}
			for _, cat := range categories {
				if errors.Is(oe, cat) != errors.Is(err, cat) {
					t.Fatalf("category %v old=%v new=%v", cat, oe, err)
				}
			}
			if err != nil {
				if raw != nil || obs.FrameBytes != 0 || count != 0 {
					t.Fatal("failed frame disclosed successful observation")
				}
				return
			}
			if !reflect.DeepEqual(old, got) {
				t.Fatalf("shape old=%#v new=%#v", old, got)
			}
			canonical, e := json.Marshal(old)
			if e != nil {
				t.Fatal(e)
			}
			if count != uint64(len(canonical)) {
				t.Fatalf("canonical count got=%d want=%d", count, len(canonical))
			}
			sum := sha256.Sum256(wire)
			if obs.FrameSHA256 != fmt.Sprintf("sha256:%x", sum) || obs.FrameBytes != int64(len(wire)) || !bytes.Equal(raw, wire) {
				t.Fatal("wire identity")
			}
			for j := range raw {
				raw[j] = '!'
			}
			if !reflect.DeepEqual(old, got) {
				t.Fatal("raw aliases envelope")
			}
		})
	}
	t.Logf("deterministic fixtures=%d including depth10000/10001 and near1MiB", len(fixtures))
}

type discoveryCountReader struct {
	r *bytes.Reader
	n int
}

func (r *discoveryCountReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p)
	r.n += n
	return n, e
}
func TestPrivateDiscoveryFieldLedger(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":{"x":1},"error":{"message":"discarded","message":"\ud800","data":null}}`)
	var q [4]PrivateDiscoveryCharge
	calls := 0
	m, _, _, _, e := ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(discoveryTestFrame(body)), Limits{1 << 20, 8192}, 1<<20, func(c PrivateDiscoveryCharge) error {
		if calls >= len(q) {
			t.Fatal("per-occurrence charge/storage")
		}
		q[calls] = c
		calls++
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	expected := uint64(len(m.ID)+len(m.Error.Data)+2*(len(m.JSONRPC)+len(m.Error.Message))) + uint64(unsafe.Sizeof(RPCError{}))
	if e != nil || calls != 4 || q[2].Retained != discoveryDepth+discoveryPlanBytes || q[3].Retained != expected {
		t.Fatalf("field ledger calls=%d charges=%+v expected=%d err=%v", calls, q, expected, e)
	}
	if discoveryPlanBytes%uint64(unsafe.Alignof(discoveryPlan{})) != 0 || unsafe.Offsetof(discoveryPlan{}.code)%unsafe.Alignof(int(0)) != 0 {
		t.Fatal("plan layout/alignment")
	}
	for _, id := range []string{`true`, `false`, `null`, `{ "nested": [1] }`, `[1,"x"]`} {
		if _, _, _, _, e := discoveryTestRead([]byte(`{"jsonrpc":"2.0","id":` + id + `,"result":null}`)); e != nil {
			t.Fatalf("raw ID %s: %v", id, e)
		}
	}
	// Fixed slots, not occurrence-sized state: many ignored and overwritten keys.
	long := `{"jsonrpc":"2.0","method":"final",` + strings.Repeat(`"method":"discarded",`, 4096) + `"` + strings.Repeat("unknown", 4096) + `":{"deep":[{}]},"method":"final"}`
	got, _, _, _, e := discoveryTestRead([]byte(long))
	if e != nil || got.Method != "final" {
		t.Fatal("unknown/duplicate scan", e)
	}
}

func TestPrivateDiscoveryOrdinaryObserverIsolation(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":null}`)
	wire := discoveryTestFrame(body)
	r := NewReaderObservedAtDecode(bytes.NewReader(wire), Limits{1 << 20, 8192}, func(Event) { panic("observer") }, func() { panic("entry") }, func() { panic("retain") })
	_, o, raw, e := r.ReadWithExactFrame(1 << 20)
	if e != nil || o.FrameBytes != int64(len(wire)) || !bytes.Equal(raw, wire) {
		t.Fatal("ordinary optional observer changed outcome", e)
	}
	sentinel := errors.New("private callback text")
	_, _, _, _, e = ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(wire), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return sentinel })
	if e != ErrPrivateDiscoveryAdmission || strings.Contains(e.Error(), sentinel.Error()) {
		t.Fatal("callback error text escaped private path")
	}
}

func TestPrivateDiscoveryCorpus(t *testing.T) {
	const seed int64 = 60059
	const cases = 2048
	rng := rand.New(rand.NewSource(seed))
	keys := []string{`"jsonrpc"`, `"JSONRPC"`, `"method"`, `"metho\u0064"`, `"id"`, `"params"`, `"result"`, `"reſult"`, `"error"`, `"unknown"`}
	values := []string{`null`, `"2.0"`, `"hello"`, `"\ud800"`, `"\udc00"`, `"\ud83d\ude00"`, `"<>&\u2028"`, `0`, `1.0`, `-0`, `1e+9`, `[]`, `{}`, `[null, {"x": 1}]`, `{"code":1,"message":"one","data":null}`, `{"code":null,"message":null}`, `{"code":"bad"}`, `{"meſſage":"fold","code":-2}`, "\"\xff\""}
	categories := []error{ErrMalformedJSON, ErrWrongVersion, ErrInvalidMessage}
	for n := 0; n < cases; n++ {
		var b strings.Builder
		b.WriteString(`{"jsonrpc":"2.0","id":1,"result":null`)
		for j := 0; j < 1+rng.Intn(12); j++ {
			b.WriteByte(',')
			b.WriteString(keys[rng.Intn(len(keys))])
			b.WriteByte(':')
			b.WriteString(values[rng.Intn(len(values))])
		}
		b.WriteByte('}')
		body := []byte(b.String())
		if n%4 == 0 {
			body = body[:rng.Intn(len(body))]
		}
		old, oe := NewReader(bytes.NewReader(discoveryTestFrame(body)), Limits{1 << 20, 8192}).Read()
		got, _, _, count, err := discoveryTestRead(body)
		if (oe == nil) != (err == nil) {
			t.Fatalf("seed=%d case=%d old=%v new=%v body=%q", seed, n, oe, err, body)
		}
		for _, c := range categories {
			if errors.Is(oe, c) != errors.Is(err, c) {
				t.Fatalf("seed=%d case=%d category=%v old=%v new=%v", seed, n, c, oe, err)
			}
		}
		if err == nil {
			if !reflect.DeepEqual(old, got) {
				t.Fatalf("seed=%d case=%d shapes old=%#v new=%#v", seed, n, old, got)
			}
			canonical, _ := json.Marshal(old)
			if count != uint64(len(canonical)) {
				t.Fatalf("seed=%d case=%d count=%d want=%d", seed, n, count, len(canonical))
			}
		}
	}
	t.Logf("seed=%d corpus=%d P=%d span=%d RPCError=%d", seed, cases, discoveryPlanBytes, unsafe.Sizeof(discoverySpan{}), unsafe.Sizeof(RPCError{}))
}

func TestPrivateDiscoveryHeadersAndPartial(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"notice"}`)
	for _, h := range []string{"\r\n", "Other: yes\r\n\r\n", "Content-Length: -1\r\n\r\n", "Content-Length: 99999999999999999999999\r\n\r\n", "Content-Length: 1\r\ncontent-length: 1\r\n\r\n", "bad\r\n\r\n", "Content-Length: 1048577\r\n\r\n"} {
		r := &discoveryCountReader{r: bytes.NewReader([]byte(h + string(body)))}
		_, o, raw, _, e := ReadPrivateDiscoveryFrame(context.Background(), r, Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
		if e == nil || o.FrameBytes != 0 || raw != nil || r.n > len(h) {
			t.Fatalf("invalid header consumed body: %q n=%d err=%v", h, r.n, e)
		}
	}
	for _, value := range []string{fmt.Sprint(len(body)), "+" + fmt.Sprint(len(body)), "\u00a0" + fmt.Sprint(len(body)) + "\u00a0"} {
		wire := []byte("cOnTeNt-LeNgTh : " + value + "\r\n\r\n" + string(body))
		_, _, _, _, e := ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(wire), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
		if e != nil {
			t.Fatal(e)
		}
	}
	wire := discoveryTestFrame(body)
	for _, cut := range []int{0, 1, len(wire) - 1} {
		_, o, raw, _, e := ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(wire[:cut]), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
		if e == nil || raw != nil || o.FrameBytes != 0 {
			t.Fatal("partial observation")
		}
		if !errors.Is(e, io.EOF) && !errors.Is(e, io.ErrUnexpectedEOF) {
			t.Fatalf("partial error %v", e)
		}
	}
	joined := bytes.NewReader(append(append([]byte(nil), wire...), wire...))
	for i := 0; i < 2; i++ {
		_, _, raw, _, e := ReadPrivateDiscoveryFrame(context.Background(), joined, Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
		if e != nil || !bytes.Equal(raw, wire) || joined.Len() != (1-i)*len(wire) {
			t.Fatal("read ahead")
		}
	}
	if discoveryCountAdd(^uint64(0)-1, 2) != ^uint64(0) {
		t.Fatal("checked canonical accumulation")
	}
}

func TestPrivateDiscoveryAdmissionBoundaries(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":"value"}`)
	wire := discoveryTestFrame(body)
	header := len(wire) - len(body)
	for stage := 1; stage <= 4; stage++ {
		t.Run(fmt.Sprint(stage), func(t *testing.T) {
			r := &discoveryCountReader{r: bytes.NewReader(wire)}
			calls := 0
			var spent uint64
			_, o, raw, _, err := ReadPrivateDiscoveryFrame(context.Background(), r, Limits{1 << 20, 8192}, 1<<20, func(q PrivateDiscoveryCharge) error {
				calls++
				if calls == stage {
					return ErrPrivateDiscoveryAdmission
				}
				spent += q.Retained
				return nil
			})
			want := len(wire)
			if stage == 1 {
				want = 0
			} else if stage == 2 {
				want = header
			}
			if !errors.Is(err, ErrPrivateDiscoveryAdmission) || r.n != want || raw != nil || o.FrameBytes != 0 || calls != stage {
				t.Fatalf("stage=%d reads=%d want=%d calls=%d err=%v", stage, r.n, want, calls, err)
			}
			if stage > 1 && spent == 0 {
				t.Fatal("earlier charges disappeared")
			}
		})
	}
	for stage := 1; stage <= 4; stage++ {
		for _, mode := range []string{"panic", "cancel"} {
			ctx, cancel := context.WithCancel(context.Background())
			calls := 0
			r := &discoveryCountReader{r: bytes.NewReader(wire)}
			_, o, raw, _, e := ReadPrivateDiscoveryFrame(ctx, r, Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error {
				calls++
				if calls == stage {
					if mode == "panic" {
						panic("private source text")
					}
					cancel()
				}
				return nil
			})
			cancel()
			want := len(wire)
			if stage == 1 {
				want = 0
			} else if stage == 2 {
				want = header
			}
			if e == nil || r.n != want || raw != nil || o.FrameBytes != 0 || calls != stage {
				t.Fatalf("%s stage%d reads=%d want=%d err=%v", mode, stage, r.n, want, e)
			}
		}
	}
	for _, admit := range []PrivateDiscoveryAdmission{nil, func(PrivateDiscoveryCharge) error { panic("private source text") }} {
		r := &discoveryCountReader{r: bytes.NewReader(wire)}
		_, _, _, _, err := ReadPrivateDiscoveryFrame(context.Background(), r, Limits{1 << 20, 8192}, 1<<20, admit)
		if !errors.Is(err, ErrPrivateDiscoveryAdmission) || r.n != 0 {
			t.Fatal("nil/panic did not fail closed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &discoveryCountReader{r: bytes.NewReader(wire)}
	_, _, _, _, err := ReadPrivateDiscoveryFrame(ctx, r, Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
	if err == nil || r.n != 0 {
		t.Fatal("cancel reads")
	}
}
