//go:build darwin || linux

package privatehostproductv3

import (
	"fmt"
	"io"
)

// Sibling namespace admission: sharing private file/quartet primitives does NOT
// mean accepting a legacy flat store or ignoring foreign numbered proof files.
func openCohortStore(c CohortConfig) (*store, error) {
	s, e := openStore(Config{Store: c.Store})
	if e != nil {
		return nil, e
	}
	fail := func() (*store, error) { s.close(); return nil, ErrInvalid }
	for _, dir := range []string{"plan", "pages", "claim", "host", "result"} {
		d, e := s.root.Open(dir)
		if e != nil {
			return fail()
		}
		names, e := d.Readdirnames(stateLimit/128 + 1)
		d.Close()
		if (e != nil && e != io.EOF) || len(names) > stateLimit/128 {
			return fail()
		}
		for _, name := range names {
			valid := false
			switch dir {
			case "plan":
				valid = name == "lock" || name == "config-00000001.json"
			case "pages":
				valid = cohortNumberedName(name, "state", "json", 1, 99999999)
			case "claim":
				valid = cohortNumberedName(name, "claim", "json", 1, 16)
			case "host":
				valid = cohortNumberedName(name, "claim", "json", 1, 16) || cohortNumberedName(name, "response", "json", 1, 16) || name == "state-00000000.json" || name == "state-00000001.json"
			case "result":
				valid = cohortNumberedName(name, "report", "json", 1, 99999999) || cohortNumberedName(name, "inventory", "md", 1, 99999999)
			}
			if !valid {
				return fail()
			}
		}
	}
	return s, nil
}
func cohortNumberedName(name, kind, ext string, low, high int) bool {
	var n int
	_, e := fmt.Sscanf(name, kind+"-%08d."+ext, &n)
	return e == nil && n >= low && n <= high && name == fmt.Sprintf(kind+"-%08d."+ext, n)
}
