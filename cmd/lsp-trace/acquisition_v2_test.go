package main

import (
	"reflect"
	"testing"
)

func TestAcquisitionVersionPreservesArgumentValues(t *testing.T) {
	for _, args := range [][]string{{"--server-arg", "--acquisition-version=v2"}, {"--output", "--acquisition-version"}, {"--server-arg", "--acquisition-version", "--at", "x:1:1"}} {
		version, rest, err := acquisitionVersion(args)
		if err != nil || version != "" || !reflect.DeepEqual(args, rest) {
			t.Fatalf("ASSERT_VERSION_FLAG_NOT_ARGUMENT_VALUE: %v -> %q %v %v", args, version, rest, err)
		}
	}
}
