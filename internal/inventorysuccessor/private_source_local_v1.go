package inventorysuccessor

import (
	"fmt"
	"time"

	"lsp-trace/internal/describeworker"
	rg "lsp-trace/internal/referencegrouping"
)

// PrivateSourceLocalWorkerAdapterV1 explicitly opts into the private local adapter.
// It conveys no model authentication, qualification, or authorization.
const PrivateSourceLocalWorkerAdapterV1 = "SOURCE_LOCAL_WORKER_V1"

const localSystemSandboxV1 = "/usr/bin/sandbox-exec"

// Closed reviewed SBPL form: no comments or later overrides. Staged pins and
// immutable OS libraries/devices are the entire permitted file-read set.
const privateLocalSandboxProfileV1 = "(version 1)\n(allow default)\n(deny network*)\n(deny file-write*)\n(deny file-read*)\n(allow file-read* (subpath (param \"ARTIFACT_ROOT\")) (subpath \"/usr/lib\") (subpath \"/System/Library\") (literal \"/dev/null\") (literal \"/dev/urandom\"))\n"

// Raw stdout and framed adapter observations are distinct and non-authenticating.
type PrivateSourceLocalWorkerObservationV1 struct {
	RawStdoutSHA256  string `json:"raw_stdout_sha256"`
	RawStdoutBytes   uint64 `json:"raw_stdout_bytes"`
	AdapterSHA256    string `json:"adapter_sha256"`
	ExecutorContract string `json:"executor_contract"`
}

func localWorkerFrameCapV1(n int) int {
	if n <= 0 {
		return 0
	}
	if n > 8192/6 {
		return 8192
	}
	return n * 6
}
func localStageBytesV1(c PrivateSourceLocalConfigV1) (uint64, error) {
	if c.Managed != nil {
		if !c.Managed.matches(c) {
			return 0, ErrPrivateSourceLocalV1
		}
		return 0, nil
	}
	var n uint64
	for i, v := range c.Lengths {
		if i == 3 && c.Config.SandboxExecutable.Path == localSystemSandboxV1 {
			continue
		}
		if v > ^uint64(0)-n {
			return 0, ErrPrivateSourceLocalV1
		}
		n += v
	}
	if c.Config.Limits.TempBytes <= 0 || c.Lengths[4] > 16384 || (!c.managedAdmission && n > uint64(c.Config.Limits.TempBytes)) {
		return 0, fmt.Errorf("%w: artifact staging exceeds existing TempBytes/profile bound", ErrPrivateSourceLocalV1)
	}
	return n, nil
}

// PrivateSourceLocalConfigV1 is a non-authorizing fixed six-pin declaration.
// Lengths bind Worker, Model, Library, SandboxExecutable, SandboxProfile, Grammar.
// No file is read to derive the pre-effects quote. Limits are capacities, not grants.
type PrivateSourceLocalConfigV1 struct {
	Config           describeworker.Config
	Lengths          [6]uint64
	io               privateLocalIOV1             // invocation-local actual descriptor seam; never caller-set funding
	Managed          *PrivateSourceLocalManagedV1 // opaque; zero/JSON cannot establish custody
	managedAdmission bool
}

// sourceLocalChargeV1 joins the closed implementation-derived local component
// into the original P quote across all fourteen currencies. ReserveRPreparationV1
// uses it BEFORE the existing reserve; the sum itself never grants execution.
func sourceLocalChargeV1(m rg.AllModeMetadataPrivate, now, deadline time.Time, local countersV2) (countersV2, error) {
	base, err := sourceRPreparationChargeV1(m, now, deadline)
	if err != nil {
		return countersV2{}, err
	}
	return addCountersV2(base, local)
}
