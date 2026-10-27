package censuslineagepilot

import "os"

// Per-call fault seams, deliberately unexported and never package-global hooks.
// hook is observation orchestration for deterministic fixture races only.
type scopeReadOps struct {
	hook  func(stage, path string) error
	close func(*os.File) error
}
