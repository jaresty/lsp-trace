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
	if got.EnvironmentNames != other.EnvironmentNames || got.EnvironmentPairs != other.EnvironmentPairs {
		t.Fatal("ASSERT_PROCESS_IDENTITY_ENV_ORDER_INSENSITIVE")
	}
	envValue := spec
	envValue.Env = []string{"Z=changed", "A=other"}
	if got.EnvironmentPairs == ObserveIdentity(envValue, "/private/config", "/private/workspace").EnvironmentPairs {
		t.Fatal("ASSERT_PROCESS_IDENTITY_ENV_VALUE_SENSITIVE")
	}
	duplicate := spec
	duplicate.Env = []string{"A=first", "A=second"}
	if status := ObserveIdentity(duplicate, "/private/config", "/private/workspace").ExecutableStatus; status != IdentityUnavailable {
		t.Fatalf("ASSERT_PROCESS_IDENTITY_DUPLICATE_ENV_REJECTED: %v", status)
	}
	v := reflect.ValueOf(got)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.String {
			t.Fatalf("ASSERT_PROCESS_IDENTITY_NO_RAW_STRINGS: field %s", v.Type().Field(i).Name)
		}
	}
}

func TestObserveIdentityDescriptorPathRace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server")
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(path, []byte("original"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte("replacement"), 0o700); err != nil {
		t.Fatal(err)
	}
	oldStat := identityPathStat
	defer func() { identityPathStat = oldStat }()
	identityPathStat = func(name string) (os.FileInfo, error) {
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal(err)
		}
		return os.Stat(name)
	}
	got := ObserveIdentity(Spec{Path: path}, "config", "workspace")
	if got.ExecutableStatus != IdentityRaced {
		t.Fatalf("ASSERT_PROCESS_IDENTITY_DESCRIPTOR_PATH_RACE: %v", got.ExecutableStatus)
	}
}

func TestObserveIdentityRejectsExecutableSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "server")
	if err := os.WriteFile(target, []byte("target"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got := ObserveIdentity(Spec{Path: link}, "config", "workspace"); got.ExecutableStatus != IdentityUnavailable {
		t.Fatalf("ASSERT_PROCESS_IDENTITY_EXECUTABLE_SYMLINK_REJECTED: %v", got.ExecutableStatus)
	}
}

func TestObserveIdentityOriginalPathSymlinkRetargetRace(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	path := filepath.Join(dir, "server")
	for name, content := range map[string]string{first: "first", second: "second"} {
		if err := os.WriteFile(name, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Link(first, path); err != nil {
		t.Fatal(err)
	}

	oldCanonical := identityCanonicalPath
	defer func() { identityCanonicalPath = oldCanonical }()
	identityCanonicalPath = func(name string) (string, error) {
		canonical, err := filepath.EvalSymlinks(name)
		if err != nil {
			return "", err
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(second, path); err != nil {
			t.Fatal(err)
		}
		return canonical, nil
	}

	got := ObserveIdentity(Spec{Path: path}, "config", "workspace")
	if got.ExecutableStatus != IdentityRaced {
		t.Fatalf("ASSERT_PROCESS_IDENTITY_ORIGINAL_PATH_RETARGET_RACE: %v", got.ExecutableStatus)
	}
}

func TestObserveIdentityDetectsExecutableReplacementAtBarrier(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server")
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(path, []byte("before"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte("after"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := observeIdentity(Spec{Path: path}, "config", "workspace", func() {
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal(err)
		}
	})
	if got.ExecutableStatus != IdentityRaced {
		t.Fatalf("ASSERT_PROCESS_IDENTITY_RACE_BARRIER: %+v", got)
	}
}

func TestIdentityDomainsDoNotAlias(t *testing.T) {
	a := digestDomain("cwd", "same")
	b := digestDomain("workspace", "same")
	if a == b {
		t.Fatal("ASSERT_PROCESS_IDENTITY_DOMAIN_SEPARATION")
	}
}
