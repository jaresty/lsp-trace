package provider

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var stableProviderIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*$`)

var supportedRelations = map[string]struct{}{
	"CALLS": {}, "BINDS_ARGUMENT": {}, "PASSES_CALLBACK": {}, "INVOKES_TASK": {},
	"TRIGGERS_RELOAD": {}, "UPDATES_STATE": {}, "RENDERS_FROM": {},
}

type ProtocolIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Capabilities struct {
	Relations  []string `json:"relations"`
	Languages  []string `json:"languages"`
	Frameworks []string `json:"frameworks"`
}
type Declaration struct {
	Identity, Version   string
	Protocol            ProtocolIdentity
	Executable          string
	Arguments           []string
	Directory           string
	Environment         []string
	ExecutableAvailable bool
	ConformanceVerified bool
	Capabilities        Capabilities
	Limits              Limits
}

var MaxProviderLimits = Limits{
	RequestBytes: 16 << 20, ResponseBytes: 16 << 20, ProtocolMessages: 64,
	StderrBytes: 1 << 20, WallTime: 10 * time.Minute, TerminationGrace: 30 * time.Second,
}

type SelectorAdmission struct{ identities []string }

func (s SelectorAdmission) Validate(selector string) error {
	if selector == "auto" || selector == "none" {
		return nil
	}
	i := sort.SearchStrings(s.identities, selector)
	if i < len(s.identities) && s.identities[i] == selector {
		return nil
	}
	return fmt.Errorf("provider selector %q is not auto, none, or a registered stable identity", selector)
}
func (s SelectorAdmission) Allowed() []string {
	return append([]string{"auto", "none"}, s.identities...)
}

type Provisioned struct {
	Registry     *Registry
	Declarations []Declaration
	Admission    SelectorAdmission
}

func Provision(declarations []Declaration) (Provisioned, error) {
	canonical := make([]Declaration, len(declarations))
	for i, declaration := range declarations {
		if err := validateDeclaration(declaration); err != nil {
			return Provisioned{}, fmt.Errorf("provider declaration %d: %w", i, err)
		}
		canonical[i] = cloneDeclaration(declaration)
		canonicalize(&canonical[i])
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].Identity < canonical[j].Identity })
	registry := NewRegistry()
	identities := make([]string, 0, len(canonical))
	for _, declaration := range canonical {
		if len(identities) > 0 && identities[len(identities)-1] == declaration.Identity {
			return Provisioned{}, fmt.Errorf("duplicate provider identity %q", declaration.Identity)
		}
		registration := Registration{ID: declaration.Identity, Path: declaration.Executable, Args: append([]string(nil), declaration.Arguments...), Dir: declaration.Directory, Env: append([]string(nil), declaration.Environment...)}
		if err := registry.Register(registration); err != nil {
			return Provisioned{}, err
		}
		identities = append(identities, declaration.Identity)
	}
	return Provisioned{Registry: registry, Declarations: canonical, Admission: SelectorAdmission{identities: identities}}, nil
}

func validateDeclaration(d Declaration) error {
	if !stableProviderIdentity.MatchString(d.Identity) {
		return errors.New("identity must be a stable name@version identity")
	}
	if strings.TrimSpace(d.Version) == "" || strings.TrimSpace(d.Protocol.Name) == "" || strings.TrimSpace(d.Protocol.Version) == "" {
		return errors.New("provider version and protocol identity/version are required")
	}
	if !filepath.IsAbs(d.Executable) {
		return errors.New("executable path must be absolute")
	}
	if d.Directory != "" && !filepath.IsAbs(d.Directory) {
		return errors.New("working directory must be absolute when supplied")
	}
	if err := validateCapabilities(d.Capabilities); err != nil {
		return err
	}
	if !positiveBounded(d.Limits.RequestBytes, MaxProviderLimits.RequestBytes) || !positiveBounded(d.Limits.ResponseBytes, MaxProviderLimits.ResponseBytes) || !positiveBounded(d.Limits.ProtocolMessages, MaxProviderLimits.ProtocolMessages) || !positiveBounded(d.Limits.StderrBytes, MaxProviderLimits.StderrBytes) || d.Limits.WallTime <= 0 || d.Limits.WallTime > MaxProviderLimits.WallTime || d.Limits.TerminationGrace <= 0 || d.Limits.TerminationGrace > MaxProviderLimits.TerminationGrace {
		return errors.New("all provider limits must be positive and within host maxima")
	}
	return nil
}
func positiveBounded(value, maximum int) bool { return value > 0 && value <= maximum }
func validateCapabilities(c Capabilities) error {
	if len(c.Relations) == 0 || len(c.Languages) == 0 || len(c.Frameworks) == 0 {
		return errors.New("relations, languages, and frameworks must be non-empty")
	}
	for _, relation := range c.Relations {
		if _, ok := supportedRelations[relation]; !ok {
			return fmt.Errorf("unsupported relation %q", relation)
		}
	}
	for name, values := range map[string][]string{"relations": c.Relations, "languages": c.Languages, "frameworks": c.Frameworks} {
		seen := map[string]struct{}{}
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s contain an empty value", name)
			}
			if _, duplicate := seen[value]; duplicate {
				return fmt.Errorf("%s contain duplicate %q", name, value)
			}
			seen[value] = struct{}{}
		}
	}
	return nil
}
func canonicalize(d *Declaration) {
	sort.Strings(d.Capabilities.Relations)
	sort.Strings(d.Capabilities.Languages)
	sort.Strings(d.Capabilities.Frameworks)
}
func cloneDeclaration(d Declaration) Declaration {
	d.Arguments = append([]string(nil), d.Arguments...)
	d.Environment = append([]string(nil), d.Environment...)
	d.Capabilities.Relations = append([]string(nil), d.Capabilities.Relations...)
	d.Capabilities.Languages = append([]string(nil), d.Capabilities.Languages...)
	d.Capabilities.Frameworks = append([]string(nil), d.Capabilities.Frameworks...)
	return d
}
