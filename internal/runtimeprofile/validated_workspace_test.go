package runtimeprofile

import "testing"

func TestValidatedWorkspaceComparisonValue(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"/w", "/w"}, {"/w/../x", "/x"}, {`/w\a`, "/w/a"}} {
		v, e := Validate(Selector{TrustDomain: "test", Workspace: tc.input, Profile: "go", EnvironmentReference: "local"})
		if e != nil {
			t.Fatal(e)
		}
		if got := v.Workspace().String(); got != tc.want {
			t.Fatalf("canonical comparison=%q want=%q", got, tc.want)
		}
	}
}
