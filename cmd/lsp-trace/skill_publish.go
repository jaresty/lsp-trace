package main

import (
	"errors"
	"fmt"
	"os"
)

var publishSkillDirectory = renameDirectoryNoReplace

func publishStagedSkill(parent, stagingHandle *os.File, staging, final string) error {
	if err := publishSkillDirectory(parent, stagingHandle, staging, final); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("skill destination already exists")
		}
		return fmt.Errorf("publish skill directory: %w", err)
	}
	return nil
}
