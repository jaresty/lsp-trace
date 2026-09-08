package managedprocess

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestObserveIdentityDomainsOrderingAndBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), int(executableIdentityByteLimit+10)), 0o700); err != nil {
		t.Fatal(err)
	}
	spec := Spec{Path: path, Args: []string{"a", "b"}, Dir: "/private/cwd", Env: []string{"Z=secret", "A=other"}}
	got := ObserveIdentity(spec, "/private/config", "/private/workspace")
	if got.ExecutableStatus != IdentityObserved || got.ExecutableBytesRead != executableIdentityByteLimit {
		t.Fatalf("ASSERT_PROCESS_IDENTITY_BOUNDED_EXECUTABLE: %+v", got)
	}
	reordered := spec
	reordered.Args = []string{"b", "a"}
	if got.OrderedArgs == ObserveIdentity(reordered, "/private/config", "/private/workspace").OrderedArgs {
		t.Fatal("ASSERT_PROCESS_IDENTITY_ORDERED_ARGS")
	}
	envOrder := spec
	envOrder.Env = []string{"A=other", "Z=secret"}
	other := ObserveIdentity(envOrder, "/private/config", "/private/workspace")
	if got.EnvironmentNames != other.EnvironmentNames || got.EnvironmentPairs == other.EnvironmentPairs {
		t.Fatal("ASSERT_PROCESS_IDENTITY_ENV_PROJECTIONS")
	}
	v := reflect.ValueOf(got)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.String {
			t.Fatalf("ASSERT_PROCESS_IDENTITY_NO_RAW_STRINGS: field %s", v.Type().Field(i).Name)
		}
	}
}

func TestIdentityDomainsDoNotAlias(t *testing.T) {
	a := digestDomain("cwd", "same")
	b := digestDomain("workspace", "same")
	if a == b {
		t.Fatal("ASSERT_PROCESS_IDENTITY_DOMAIN_SEPARATION")
	}
}
