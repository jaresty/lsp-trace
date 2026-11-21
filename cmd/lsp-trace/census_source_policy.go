package main

import (
	"errors"
	"fmt"
	"lsp-trace/internal/scopejob"
	"path/filepath"
)

var errCensusEligibilityUnknown = errors.New("DECLARED_ELIGIBILITY_UNKNOWN: selected profile must declare nonempty language_ids")

// freezeCensusSelection is configuration-only: no provider lifecycle or source
// scan. Project associations are deliberately not merged from user config.
func freezeCensusSelection(workspace, name string) (string, *scopejob.SelectionPolicy, error) {
	project, _, err := decodeProfileFileLimit(filepath.Join(workspace, ".lsp-trace.toml"), false, 32768)
	if err != nil {
		return "", nil, err
	}
	ids := []string{"go"}
	profileName := name
	if profileName == "" {
		profileName = "gopls"
	} else {
		declaration, e := loadProfileDeclarationLimit(workspace, profileFlags{Name: name}, 32768)
		if e != nil && !(name == "gopls" && errors.Is(e, errProfileNotFound)) {
			return "", nil, e
		}
		if e == nil {
			if declaration.LanguageIDs == nil || len(*declaration.LanguageIDs) == 0 {
				return "", nil, fmt.Errorf("%w: profile %q", errCensusEligibilityUnknown, name)
			}
			ids = *declaration.LanguageIDs
		}
	}
	policy, err := scopejob.FreezeSelection(ids, project.FileAssociations)
	return profileName, policy, err
}
