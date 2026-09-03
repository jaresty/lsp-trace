// Package runtimeprofile resolves validated selectors into immutable,
// authority-free runtime identity. It does not resolve executable configuration.
package runtimeprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"lsp-trace/internal/sessionkey"
)

var referencePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._:-]*$`)

type Option struct {
	Name  string
	Value string
}

// Selector is untrusted until admitted by Validate.
type Selector struct {
	TrustDomain          string
	Workspace            string
	Profile              string
	EnvironmentReference string
	Options              []Option
}

// ValidatedSelector can only be constructed by Validate. Its state is not
// exported and owns its option storage.
type ValidatedSelector struct {
	trustDomain          string
	workspace            string
	profile              string
	environmentReference string
	optionsDigest        string
}

type Workspace struct{ value string }
type EnvironmentReference struct{ value string }

// Profile is immutable and comparable. It contains identity only: no command,
// environment value, credential, capability, or permission can cross this API.
type Profile struct {
	trustDomain string
	workspace   Workspace
	environment EnvironmentReference
	profileName string
	key         sessionkey.Key
}

func Validate(selector Selector) (ValidatedSelector, error) {
	trustDomain, err := canonicalRequired("trust domain", selector.TrustDomain)
	if err != nil {
		return ValidatedSelector{}, err
	}
	profile, err := canonicalRequired("profile", selector.Profile)
	if err != nil {
		return ValidatedSelector{}, err
	}
	environment, err := canonicalRequired("environment reference", selector.EnvironmentReference)
	if err != nil {
		return ValidatedSelector{}, err
	}
	if !referencePattern.MatchString(environment) {
		return ValidatedSelector{}, fmt.Errorf("environment reference must be an opaque name")
	}
	workspace, err := canonicalWorkspace(selector.Workspace)
	if err != nil {
		return ValidatedSelector{}, err
	}
	optionsDigest, err := digestOptions(selector.Options)
	if err != nil {
		return ValidatedSelector{}, err
	}
	return ValidatedSelector{
		trustDomain: trustDomain, workspace: workspace, profile: profile,
		environmentReference: environment, optionsDigest: optionsDigest,
	}, nil
}

func Resolve(selector ValidatedSelector) Profile {
	workspace := Workspace{value: selector.workspace}
	environment := EnvironmentReference{value: selector.environmentReference}
	key := sessionkey.Derive(sessionkey.Material{
		TrustDomain: selector.trustDomain, Workspace: selector.workspace,
		Profile: selector.profile, EnvironmentReference: selector.environmentReference,
		OptionsDigest: selector.optionsDigest,
	})
	return Profile{
		trustDomain: selector.trustDomain, workspace: workspace,
		environment: environment, profileName: selector.profile, key: key,
	}
}

func (profile Profile) Workspace() Workspace              { return profile.workspace }
func (profile Profile) Environment() EnvironmentReference { return profile.environment }
func (profile Profile) ProfileName() string               { return profile.profileName }
func (profile Profile) SessionKey() sessionkey.Key        { return profile.key }
func (workspace Workspace) String() string                { return workspace.value }
func (reference EnvironmentReference) String() string     { return reference.value }

func canonicalRequired(name, value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s is not valid UTF-8", name)
	}
	value = norm.NFC.String(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func canonicalWorkspace(value string) (string, error) {
	value, err := canonicalRequired("workspace", strings.ReplaceAll(value, `\`, "/"))
	if err != nil {
		return "", err
	}
	prefix := ""
	rest := value
	switch {
	case strings.HasPrefix(value, "//"):
		parts := strings.Split(strings.TrimPrefix(value, "//"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("workspace is not an absolute canonical path")
		}
		prefix, rest = "//"+parts[0]+"/"+parts[1], "/"+strings.Join(parts[2:], "/")
	case len(value) >= 3 && value[1] == ':' && value[2] == '/':
		if !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) {
			return "", fmt.Errorf("workspace is not an absolute canonical path")
		}
		prefix, rest = strings.ToUpper(value[:1])+":", value[2:]
	case strings.HasPrefix(value, "/"):
	default:
		return "", fmt.Errorf("workspace is not an absolute canonical path")
	}
	cleaned := path.Clean(rest)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("workspace crosses its root")
	}
	if prefix != "" {
		return prefix + cleaned, nil
	}
	return cleaned, nil
}

func digestOptions(options []Option) (string, error) {
	h := sha256.New()
	previous := ""
	for index, option := range options {
		name, err := canonicalRequired("option name", option.Name)
		if err != nil {
			return "", err
		}
		if index > 0 && previous >= name {
			return "", fmt.Errorf("options must be uniquely sorted by name")
		}
		previous = name
		for _, field := range []string{name, option.Value} {
			_, _ = fmt.Fprintf(h, "%d:", len(field))
			_, _ = h.Write([]byte(field))
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
