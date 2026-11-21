package main

import (
	"errors"
	"strings"
)

func validateCensusScopeJobSyntax(o censusCLIOptions, args []string) error {
	if !o.ScopeJobCandidate {
		if censusNamedFlagCount(args, "scope-job-budget") != 0 {
			return errors.New("scope-job-budget requires scope-job-candidate")
		}
		return nil
	}
	if !o.FolderInvocation || o.DurableDescribe {
		return errors.New("scope-job candidate requires a normal folder, not a legacy route")
	}
	allowed := map[string]bool{"scope-job-candidate": true, "scope-job-budget": true, "source": true, "include": true, "exclude": true, "prune-dir": true, "publication-root": true, "profile": true, "machine": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		if !allowed[name] {
			return errors.New("scope-job candidate rejects provider/acquisition/legacy controls")
		}
		if name != "source" && name != "include" && name != "exclude" && name != "prune-dir" && censusNamedFlagCount(args, name) != 1 {
			return errors.New("duplicate scope-job control")
		}
		if name != "scope-job-candidate" && name != "machine" && !strings.Contains(arg, "=") {
			i++
		}
	}
	return nil
}
