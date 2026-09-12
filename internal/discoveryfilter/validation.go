// Package discoveryfilter defines the shared automatic-discovery pattern subset.
package discoveryfilter

import (
	"fmt"
	"path"
	"strings"
)

func ValidatePattern(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("automatic discovery pattern must not be empty")
	}
	if strings.HasPrefix(pattern, "!") {
		return fmt.Errorf("automatic discovery pattern does not support leading !")
	}
	if strings.HasSuffix(pattern, "\\") {
		return fmt.Errorf("automatic discovery pattern is malformed")
	}
	normalized := strings.ReplaceAll(pattern, "\\", "/")
	if strings.Contains(normalized, "//") {
		return fmt.Errorf("automatic discovery pattern is malformed")
	}
	normalized = strings.TrimPrefix(normalized, "/")
	if strings.HasSuffix(normalized, "/") {
		normalized += "**"
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, "x"); err != nil {
			return fmt.Errorf("automatic discovery pattern is malformed")
		}
	}
	return nil
}

func ValidatePatterns(includes, excludes []string) error {
	for _, patterns := range [][]string{includes, excludes} {
		for _, pattern := range patterns {
			if err := ValidatePattern(pattern); err != nil {
				return err
			}
		}
	}
	return nil
}
