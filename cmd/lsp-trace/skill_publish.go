package main

import (
	"errors"
	"fmt"
	"os"
)

var (
	publishSkillDirectory     = renameDirectoryNoReplace
	afterSkillContainerPinned = func(*os.File, string) error { return nil }
)

func publishStagedSkill(parent, containerHandle, payloadHandle *os.File, container, payload, final string) error {
	if err := publishSkillDirectory(parent, containerHandle, payloadHandle, container, payload, final); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("skill destination already exists")
		}
		return fmt.Errorf("publish skill directory: %w", err)
	}
	return nil
}
