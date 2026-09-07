package source

import "os"

// Windows has no filesystem FIFO nodes. os.Root disallows reserved device names
// and escape to the named-pipe namespace; the opened handle is still checked for
// regular-file type before reading. Windows runtime qualification is separate.
func openInput(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
