//go:build linux

package privatehostproductv3

import (
	"os"
	"strings"
)

func applicationBootEpoch() (string, error) {
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if len(b) > 128 {
		return "", ErrInvalid
	}
	return strings.TrimSpace(string(b)), e
}
