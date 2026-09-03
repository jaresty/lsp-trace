package session

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func hx(s string) []byte { b, _ := hex.DecodeString(strings.ReplaceAll(s, " ", "")); return b }

func TestCanonicalValueVectors(t *testing.T) {
	vectors := []struct {
		name string
		v    Value
		hex  string
	}{
		{"null", Null(), "00 0000000000000000"}, {"false", Bool(false), "01 0000000000000000"}, {"true", Bool(true), "02 0000000000000000"},
		{"u0", Uint(0), "03 0000000000000001 00"}, {"u255", Uint(255), "03 0000000000000001 ff"}, {"u256", Uint(256), "03 0000000000000002 0100"},
		{"i0", Int(0), "04 0000000000000001 00"}, {"i127", Int(127), "04 0000000000000001 7f"}, {"i128", Int(128), "04 0000000000000002 0080"}, {"i-1", Int(-1), "04 0000000000000001 ff"}, {"i-129", Int(-129), "04 0000000000000002 ff7f"},
		{"nfc", String("e\u0301"), "06 0000000000000002 c3a9"}, {"posix", Path("/a/./c/../b"), "07 0000000000000004 2f612f62"},
		{"drive", Path(`c:\A\b`), "07 0000000000000006 433a2f412f62"}, {"unc", Path(`\\Srv\Share\x`), "07 000000000000000d 2f2f5372762f53686172652f78"},
		{"list", List(Bool(false), Bool(true)), "08 000000000000001a 0000000000000002 010000000000000000 020000000000000000"},
	}
	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.v.Encode()
			if err != nil || !bytes.Equal(got, hx(tc.hex)) {
				t.Fatalf("got %x,%v want %x", got, err, hx(tc.hex))
			}
		})
	}
}

func TestMapOrderDuplicateAndVersion(t *testing.T) {
	a := Map(Entry{"z", Uint(1)}, Entry{"a", Uint(2)})
	b := Map(Entry{"a", Uint(2)}, Entry{"z", Uint(1)})
	ae, _ := a.Encode()
	be, _ := b.Encode()
	if !bytes.Equal(ae, be) {
		t.Fatal("map order changed encoding")
	}
	if _, err := Map(Entry{"a", Uint(1)}, Entry{"a", Uint(2)}).Encode(); err == nil {
		t.Fatal("duplicate map key accepted")
	}
}

func validSessionKeyValues() [8]Value {
	return [8]Value{String("local"), Path("/work/repo"), String("gopls"), String("gopls"), List(String("serve")), Bytes(make([]byte, 32)), LanguageConfiguration(Entry{"language_id", String("go")}, Entry{"initialization_options", Null()}, Entry{"workspace_configuration", Null()}), ServerAffectingOptions()}
}

func TestSessionKeyRejectsWrongTopLevelTags(t *testing.T) {
	for i := range validSessionKeyValues() {
		values := validSessionKeyValues()
		values[i] = Null()
		if _, err := EncodeSessionKeyV1(values); err == nil {
			t.Fatalf("ASSERT_SESSION_KEY_TOP_LEVEL_TAGS: component %d accepted wrong tag", i)
		}
	}
}

func TestEnvironmentCanonicalIdentity(t *testing.T) {
	processKey := []byte("key")
	decomposed := map[string][]byte{"e\u0301": []byte("accent"), "f": []byte("plain")}
	canonical := map[string][]byte{"é": []byte("accent"), "f": []byte("plain")}

	got, err := EnvironmentValue(processKey, decomposed)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_CANONICAL_EQUIVALENCE: decomposed vector: %v", err)
	}
	want, err := EnvironmentValue(processKey, canonical)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_CANONICAL_ORDER: canonical vector: %v", err)
	}
	gotBytes, err := got.Encode()
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_CANONICAL_EQUIVALENCE: encode decomposed vector: %v", err)
	}
	wantBytes, err := want.Encode()
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_CANONICAL_ORDER: encode canonical vector: %v", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_CANONICAL_ORDER: canonical UTF-8 order changed digest: got %x want %x", gotBytes, wantBytes)
	}
}

func TestEnvironmentRejectsCanonicalDuplicate(t *testing.T) {
	if _, err := EnvironmentValue([]byte("key"), map[string][]byte{"e\u0301": []byte("one"), "é": []byte("two")}); err == nil || !strings.Contains(err.Error(), "duplicate canonical environment name") {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_CANONICAL_DUPLICATE: got %v", err)
	}
}

func TestEnvironmentNameBoundariesAndErrorPropagation(t *testing.T) {
	processKey := []byte("key")
	for _, name := range []string{"", "\U0010ffff"} {
		first, err := EnvironmentValue(processKey, map[string][]byte{name: []byte("value")})
		if err != nil {
			t.Fatalf("ASSERT_SESSION_KEY_ENV_BOUNDARIES: name %q: %v", name, err)
		}
		second, err := EnvironmentValue(processKey, map[string][]byte{name: []byte("value")})
		if err != nil {
			t.Fatalf("ASSERT_SESSION_KEY_ENV_ACCEPTED_VECTOR: name %q: %v", name, err)
		}
		firstBytes, firstErr := first.Encode()
		secondBytes, secondErr := second.Encode()
		if firstErr != nil || secondErr != nil || !bytes.Equal(firstBytes, secondBytes) {
			t.Fatalf("ASSERT_SESSION_KEY_ENV_ACCEPTED_VECTOR: name %q: %x/%v != %x/%v", name, firstBytes, firstErr, secondBytes, secondErr)
		}
	}

	_, err := EnvironmentValue(processKey, map[string][]byte{"\xff": []byte("value")})
	if err == nil || !strings.Contains(err.Error(), "environment name: invalid UTF-8") {
		t.Fatalf("ASSERT_SESSION_KEY_ENV_ERROR_PROPAGATION: got %v", err)
	}
}

func TestSessionKeyRejectsMalformedWindowsPaths(t *testing.T) {
	for _, value := range []string{`C:/a/../../b`, `//Srv/Share/../../x`, `\\?\C:\x`, `\\.\pipe\x`, `C:/file:stream`} {
		if _, err := Path(value).Encode(); err == nil {
			t.Fatalf("ASSERT_SESSION_KEY_WINDOWS_PATH_REJECTION: accepted %q", value)
		}
	}
}

func TestSessionKeyComponentPerturbation(t *testing.T) {
	base := validSessionKeyValues()
	baseline, err := EncodeSessionKeyV1(base)
	if err != nil {
		t.Fatal(err)
	}
	changes := [8]Value{
		String("other"), Path("/work/other"), String("other-profile"), String("other-command"),
		List(String("other-arg")), Bytes(bytes.Repeat([]byte{1}, 32)),
		LanguageConfiguration(Entry{"language_id", String("rust")}, Entry{"initialization_options", Null()}, Entry{"workspace_configuration", Null()}),
		ServerAffectingOptions(Entry{"mode", String("strict")}),
	}
	for i := range changes {
		changed := base
		changed[i] = changes[i]
		encoded, err := EncodeSessionKeyV1(changed)
		if err != nil || bytes.Equal(encoded, baseline) {
			t.Fatalf("ASSERT_SESSION_KEY_COMPONENT_PERTURBATION: component %d: %v", i, err)
		}
	}
}

func TestLanguageConfigurationClosedKeys(t *testing.T) {
	values := validSessionKeyValues()
	values[6] = LanguageConfiguration(Entry{"language_id", String("go")})
	if _, err := EncodeSessionKeyV1(values); err == nil {
		t.Fatal("ASSERT_SESSION_KEY_LANGUAGE_KEYS: incomplete map accepted")
	}
}

func TestSessionKeyEncodingIdentity(t *testing.T) {
	values := validSessionKeyValues()
	e, err := EncodeSessionKeyV1(values)
	if err != nil {
		t.Fatal(err)
	}
	prefix := append([]byte("lsp-trace/session-key\x00"), []byte{0, 0, 0, 0, 0, 0, 0, 1}...)
	if !bytes.HasPrefix(e, prefix) {
		t.Fatalf("prefix %x", e[:len(prefix)])
	}
	decoded, err := DecodeSessionKeyV1(e)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_CANONICAL_ROUND_TRIP: %v", err)
	}
	roundTrip, err := EncodeSessionKeyV1(decoded)
	if err != nil || !bytes.Equal(roundTrip, e) {
		t.Fatalf("ASSERT_SESSION_KEY_CANONICAL_ROUND_TRIP: %x %v", roundTrip, err)
	}
	unknown := append([]byte(nil), e...)
	unknown[len("lsp-trace/session-key\x00")+7] = 2
	if _, err := DecodeSessionKeyV1(unknown); err == nil {
		t.Fatal("ASSERT_SESSION_KEY_UNKNOWN_VERSION: accepted")
	}
	k := DigestSessionKey(e)
	if !strings.HasPrefix(k, "sha256:") || len(k) != 71 {
		t.Fatalf("digest %q", k)
	}
	if !Reusable("p", e, "p", append([]byte(nil), e...)) || Reusable("p", e, "q", e) {
		t.Fatal("process-local canonical equality violated")
	}
}
