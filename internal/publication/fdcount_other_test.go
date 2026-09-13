//go:build !unix

package publication

func openFDCount() int { return -1 }
