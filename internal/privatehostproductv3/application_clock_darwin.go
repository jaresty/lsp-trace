//go:build darwin

package privatehostproductv3

import "golang.org/x/sys/unix"

func applicationBootEpoch() (string, error) { return unix.Sysctl("kern.bootsessionuuid") }
