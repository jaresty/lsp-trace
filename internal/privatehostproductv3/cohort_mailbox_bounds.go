//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
)

// One MiB, split at at least 19997 bytes per non-final UTF-8 part: at most
// 53 parts, plus one request and one reply, for each of sixteen exchanges.
// Two READYs, terminal, host returns, and four concurrently staged files.
const cohortMailboxEntryCap = 16*(53+2) + 4 + 4

type cohortDirectoryReader interface{ Readdirnames(int) ([]string, error) }

func boundedCohortMailboxNames(ctx context.Context, d cohortDirectoryReader) ([]string, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	names, e := d.Readdirnames(cohortMailboxEntryCap + 1)
	if e != nil && e != io.EOF {
		return nil, e
	}
	if len(names) > cohortMailboxEntryCap {
		return nil, ErrInvalid
	}
	requests := []string{}
	for _, name := range names {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		valid := name == "ready-interpreter.json" || name == "ready-reviewer.json" || name == "terminal.json" || name == "host-returns.json"
		if cohortNumberedName(name, "request", "json", 1, 16) {
			valid = true
			requests = append(requests, name)
		}
		if cohortNumberedName(name, "reply", "json", 1, 16) {
			valid = true
		}
		var seq, part int
		if _, e := fmt.Sscanf(name, "part-%08d-%04d.json", &seq, &part); e == nil && seq >= 1 && seq <= 16 && part >= 0 && part < 53 && name == fmt.Sprintf("part-%08d-%04d.json", seq, part) {
			valid = true
		}
		if len(name) == len(".pending-")+32 && name[:len(".pending-")] == ".pending-" {
			_, e := hex.DecodeString(name[len(".pending-"):])
			valid = e == nil
		}
		if !valid {
			return nil, ErrInvalid
		}
	}
	// Only sixteen protocol metadata names can reach sorting, never foreign data.
	sort.Strings(requests)
	return requests, nil
}
