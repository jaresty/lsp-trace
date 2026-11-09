package inventorysuccessor

import (
	"lsp-trace/internal/describeworker"
	"time"
)

// Closed additional read parameter, not a policy override or OS qualification.
const privateManagedSandboxProfileV1 = "(version 1)\n(allow default)\n(deny network*)\n(deny file-write*)\n(deny file-read*)\n(allow file-read* (subpath (param \"ARTIFACT_ROOT\")) (subpath (param \"ATTEMPT_ROOT\")) (subpath \"/usr/lib\") (subpath \"/System/Library\") (literal \"/dev/null\") (literal \"/dev/urandom\"))\n"

type PrivateSourceLocalDependencyV1 struct {
	Pin   describeworker.FilePin
	Bytes uint64
}

// Declaration, not authorization. Flat explicit native file closure only: no
// discovery, symlinks, search paths, arbitrary directories or loader fallback.
type PrivateSourceLocalManagedRequestV1 struct {
	Config                PrivateSourceLocalConfigV1
	Dependencies          [16]PrivateSourceLocalDependencyV1
	Count                 int
	MaxBytes, MaxUses     uint64
	Attempt, ExpectedHead string
	Deadline              time.Time
}

// Opaque in-process custody. Zero values and public JSON cannot mint a generation.
type PrivateSourceLocalManagedV1 struct{ state *localManagedGenerationV1 }
