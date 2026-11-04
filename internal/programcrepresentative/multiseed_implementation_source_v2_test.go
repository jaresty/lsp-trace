package programcrepresentative

import (
	"fmt"
	"strings"
	"testing"
)

type boundedImplementationFixtureV2 struct {
	raw   []byte
	reads int
}

func (s *boundedImplementationFixtureV2) ReadImplementationFileV2(_ string, limit uint64) ([]byte, error) {
	s.reads++
	if uint64(len(s.raw)) > limit {
		return nil, fmt.Errorf("bounded test-only source refusal")
	}
	return append([]byte{}, s.raw...), nil
}
func TestMultiSeedV2ReviewedImplementationSource(t *testing.T) {
	t.Log("procedure: exact test-local reviewed declaration -> bounded source bytes; wrong manifest/root/blob/source/length refuse; production pin declaration remains absent")
	declaration := multiSeedSubjectExpectedFixtureV2(t).Stage6Implementation
	good := []byte("controlled test-only implementation source\n")
	reader := &boundedImplementationFixtureV2{raw: good}
	if err := verifyStage6ImplementationSourceAgainstDeclarationV2(declaration, declaration, reader, 1000); err != nil || reader.reads != 1 {
		t.Fatalf("EXACT_REVIEWED_IMPLEMENTATION_SOURCE: %v", err)
	}
	for _, name := range []string{"foreign-manifest", "bad-root", "wrong-blob", "wrong-source", "source-bound"} {
		t.Run(name, func(t *testing.T) {
			expected, decl := cloneStage6ImplementationDTO(declaration), cloneStage6ImplementationDTO(declaration)
			source := &boundedImplementationFixtureV2{raw: append([]byte{}, good...)}
			limit := uint64(1000)
			switch name {
			case "foreign-manifest":
				expected.RepositoryCommit = strings.Repeat("e", 40)
			case "bad-root":
				decl.Identity = rawDigest([]byte("foreign-root"))
				expected = decl
			case "wrong-blob":
				decl.Files[0].GitBlobSHA1 = strings.Repeat("f", 40)
				decl.Identity = ""
				n, e := CanonicalSizeStage6ImplementationDTO(decl)
				raw, e := marshalSizedV2(decl, n, e)
				if e != nil {
					t.Fatal(e)
				}
				decl.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-implementation:v2", raw)
				expected = decl
			case "wrong-source":
				source.raw[0] = 'x'
			case "source-bound":
				limit = 1
			}
			if err := verifyStage6ImplementationSourceAgainstDeclarationV2(expected, decl, source, limit); err == nil {
				t.Fatal("WRONG_REVIEWED_IMPLEMENTATION_REFUSAL")
			}
			if (name == "foreign-manifest" || name == "bad-root" || name == "source-bound") && source.reads != 0 {
				t.Fatal("IMPLEMENTATION_ADMISSION_BEFORE_SOURCE_READ")
			}
		})
	}
}
