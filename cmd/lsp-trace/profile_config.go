package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type profileFile struct {
	Profiles map[string]serverProfile `toml:"profiles"`
}

type serverProfile struct {
	Command     *string   `toml:"command"`
	Args        *[]string `toml:"args"`
	Environment *[]string `toml:"env"`
	LanguageIDs *[]string `toml:"language_ids"`
}

type profileFlags struct {
	ConfigPath string
	Name       string
}

type cliServerFields struct {
	Command, Args, Environment, LanguageID bool
}

type resolvedProfile struct {
	Command        string
	Args           []string
	Environment    []string
	LanguageID     string
	SecretEnvNames map[string]struct{}
}

var exactEnvironmentReference = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func loadRequestedProfile(workspace string, flags profileFlags) (resolvedProfile, error) {
	if flags.Name == "" {
		return resolvedProfile{}, nil
	}
	var paths []string
	if flags.ConfigPath != "" {
		paths = []string{flags.ConfigPath}
	} else {
		if userPath, err := userProfileConfigPath(); err == nil && userPath != "" {
			paths = append(paths, userPath)
		}
		paths = append(paths, filepath.Join(workspace, ".lsp-trace.toml"))
	}
	var merged serverProfile
	found := false
	for _, path := range paths {
		file, exists, err := decodeProfileFile(path, flags.ConfigPath != "")
		if err != nil {
			return resolvedProfile{}, err
		}
		if !exists {
			continue
		}
		profile, ok := file.Profiles[flags.Name]
		if !ok {
			continue
		}
		found = true
		mergeServerProfile(&merged, profile)
	}
	if !found {
		return resolvedProfile{}, fmt.Errorf("profile %q not found", flags.Name)
	}
	return resolveProfileEnvironment(flags.Name, merged)
}

func userProfileConfigPath() (string, error) {
	if root := os.Getenv("XDG_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "lsp-trace", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(home, ".config", "lsp-trace", "config.toml"), nil
}

func decodeProfileFile(path string, required bool) (profileFile, bool, error) {
	var file profileFile
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return file, false, nil
		}
		return file, false, fmt.Errorf("read config %q: %w", path, err)
	}
	metadata, err := toml.Decode(string(data), &file)
	if err != nil {
		return file, false, fmt.Errorf("invalid TOML config %q: %w", path, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		sort.Strings(keys)
		return file, false, fmt.Errorf("unknown config field(s) in %q: %s", path, strings.Join(keys, ", "))
	}
	return file, true, nil
}

func mergeServerProfile(dst *serverProfile, src serverProfile) {
	if src.Command != nil {
		dst.Command = src.Command
	}
	if src.Args != nil {
		dst.Args = src.Args
	}
	if src.Environment != nil {
		dst.Environment = src.Environment
	}
	if src.LanguageIDs != nil {
		dst.LanguageIDs = src.LanguageIDs
	}
}

func resolveProfileEnvironment(name string, profile serverProfile) (resolvedProfile, error) {
	resolved := resolvedProfile{SecretEnvNames: map[string]struct{}{}}
	if profile.Command != nil {
		resolved.Command = *profile.Command
	}
	if profile.Args != nil {
		resolved.Args = append([]string(nil), (*profile.Args)...)
	}
	if profile.LanguageIDs != nil && len(*profile.LanguageIDs) > 0 {
		resolved.LanguageID = (*profile.LanguageIDs)[0]
	}
	if profile.Environment == nil {
		return resolved, nil
	}
	seen := map[string]struct{}{}
	for _, declaration := range *profile.Environment {
		key, reference := declaration, declaration
		if left, right, ok := strings.Cut(declaration, "="); ok {
			key, reference = left, right
		}
		if !environmentName.MatchString(key) {
			return resolvedProfile{}, fmt.Errorf("profile %q has invalid environment name %q", name, key)
		}
		match := exactEnvironmentReference.FindStringSubmatch(reference)
		variable := reference
		if len(match) == 2 {
			variable = match[1]
		} else if reference != key {
			return resolvedProfile{}, fmt.Errorf("profile %q environment %q must be a variable name or exact ${VAR} reference", name, declaration)
		}
		if _, duplicate := seen[key]; duplicate {
			return resolvedProfile{}, fmt.Errorf("profile %q has duplicate environment name %q", name, key)
		}
		value, ok := os.LookupEnv(variable)
		if !ok {
			return resolvedProfile{}, fmt.Errorf("profile %q requires missing environment variable %q", name, variable)
		}
		seen[key] = struct{}{}
		resolved.SecretEnvNames[key] = struct{}{}
		resolved.Environment = append(resolved.Environment, key+"="+value)
	}
	return resolved, nil
}

func applyIncomingProfile(c *config, flags profileFlags, explicit cliServerFields) error {
	profile, err := loadRequestedProfile(c.workspace, flags)
	if err != nil {
		return err
	}
	if flags.Name == "" {
		return nil
	}
	if !explicit.Command {
		c.command = profile.Command
	}
	if !explicit.Args {
		c.args = append(c.args[:0], profile.Args...)
	}
	if !explicit.Environment {
		c.env = append(c.env[:0], profile.Environment...)
	}
	if !explicit.LanguageID {
		c.languageID = profile.LanguageID
	}
	return nil
}

func applySliceProfile(c *sliceConfig, flags profileFlags, explicit cliServerFields) error {
	profile, err := loadRequestedProfile(c.workspace, flags)
	if err != nil {
		return err
	}
	if flags.Name == "" {
		return nil
	}
	if !explicit.Command {
		c.command = profile.Command
	}
	if !explicit.Args {
		c.args = append(c.args[:0], profile.Args...)
	}
	if !explicit.Environment {
		c.env = append(c.env[:0], profile.Environment...)
	}
	if !explicit.LanguageID {
		c.languageID = profile.LanguageID
	}
	return nil
}

func effectiveEnvironmentNames(groups ...[]string) []string {
	set := map[string]struct{}{}
	for _, group := range groups {
		for _, declaration := range group {
			name, _, _ := strings.Cut(declaration, "=")
			if name != "" {
				set[name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func recordedServerEnvironment(environment map[string]string) map[string]string {
	if len(environment) == 0 {
		return nil
	}
	result := make(map[string]string, len(environment))
	for name := range environment {
		result[name] = "${" + name + "}"
	}
	return result
}

func explicitServerFields(fs *flag.FlagSet) cliServerFields {
	var fields cliServerFields
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "server":
			fields.Command = true
		case "server-arg":
			fields.Args = true
		case "server-env":
			fields.Environment = true
		case "language-id":
			fields.LanguageID = true
		}
	})
	return fields
}
