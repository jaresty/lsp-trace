//go:build darwin

package describeworker

import "syscall"

func makeRunnerFIFO(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

func runnerProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
