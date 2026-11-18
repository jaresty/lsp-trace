package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// V2 excludes attempt-derived request seals. V1 profiles must be explicitly
// re-exported to a new owned file; they are never silently migrated in place.
const censusCallerHostSettingsSchema = "census-caller-host-settings.v2"

// Settings are policy data, never an execution envelope or a grant. In
// particular they deliberately have no expiration, run, journal or funding head.
type censusCallerHostSettings struct {
	Schema        string
	Bridge        map[string]any
	RequestPolicy map[string]any
}

func censusCallerHostObject(raw []byte) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var m map[string]any
	if d.Decode(&m) != nil || m == nil || d.Decode(new(any)) != io.EOF {
		return nil, errCensusInventoryPrivate
	}
	return m, nil
}

func censusCallerHostPick(m map[string]any, keys ...string) map[string]any {
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		if v, ok := m[key]; ok {
			out[key] = v
		}
	}
	return out
}

func censusCallerHostMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func censusCallerHostStable(c censusInventoryBridgeConfig) (censusCallerHostSettings, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return censusCallerHostSettings{}, err
	}
	bridge, err := censusCallerHostObject(raw)
	if err != nil {
		return censusCallerHostSettings{}, err
	}
	s := censusCallerHostSettings{Schema: censusCallerHostSettingsSchema,
		Bridge: censusCallerHostPick(bridge, "Header", "Profile", "Process", "LanguageID", "Verification", "Limits", "Target", "Mode")}
	if len(c.Request) == 0 || bytes.Equal(c.Request, []byte("null")) {
		return s, nil
	}
	q, err := censusCallerHostObject(c.Request)
	if err != nil {
		return s, err
	}
	configuration := censusCallerHostMap(q, "Configuration")
	inventory := censusCallerHostMap(q, "Inventory")
	if configuration == nil || inventory == nil {
		return s, errCensusInventoryPrivate
	}
	// SelectionRuleSHA256 seals Manifest (including retained cursor provenance),
	// and UnitPolicySHA256 seals Inventory (including original deadlines and new
	// owner identities). They are execution seals, not reusable policy. The
	// product decoder still checks both seals on every executable request.
	policy := censusCallerHostPick(configuration, "Schema", "Profile", "SourceRoot", "RevisionAssertion", "ProducerExecutableSHA256")
	policy["GrantPolicy"] = censusCallerHostPick(censusCallerHostMap(configuration, "Grant"), "Ceiling", "BootstrapAttempts", "BootstrapEach")
	ip := censusCallerHostPick(inventory, "SourceRevision", "MaxNativeAttempts", "NativeRetainedUpper", "BudgetBytes", "HostWindowMS", "ReviewMode")
	ip["Pins"] = censusCallerHostPick(censusCallerHostMap(inventory, "Pins"), "grammar_sha256", "model_sha256", "worker_sha256")
	owners, ok := inventory["NativeOwners"].([]any)
	if !ok || len(owners) != 1 {
		return s, errCensusInventoryPrivate
	}
	owner, ok := owners[0].(map[string]any)
	if !ok {
		return s, errCensusInventoryPrivate
	}
	op := censusCallerHostPick(owner, "Verification", "Profile", "LanguageID", "Provider", "ProcessPath", "ProcessArgs", "Revision", "Privacy", "OccurrencePolicy", "CompositionPolicy", "Kinds", "SourceLimits", "ExpectedSourceAuthorization", "Seed")
	op["Contract"] = censusCallerHostPick(censusCallerHostMap(owner, "Contract"), "CaptureLimits", "DescribeDeadlineMS", "PacketMaxResponseBytes", "PacketResolveLimits", "PacketSourcePolicy", "ProgramCSeed", "ResponseVersion", "RetryPolicy", "SchemaVersions")
	ip["NativeOwnerPolicy"] = op
	s.RequestPolicy = map[string]any{"Schema": q["Schema"], "Configuration": policy, "Inventory": ip}
	return s, nil
}

func decodeCensusCallerHostSettings(raw []byte) (censusCallerHostSettings, error) {
	var saved censusCallerHostSettings
	if len(raw) == 0 || len(raw) > 4<<20 {
		return saved, errCensusInventoryPrivate
	}
	body := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	d.DisallowUnknownFields()
	if d.Decode(&saved) != nil || d.Decode(new(any)) != io.EOF {
		return saved, errCensusInventoryPrivate
	}
	canonical, err := json.Marshal(saved)
	if err != nil || !bytes.Equal(canonical, body) || saved.Schema != censusCallerHostSettingsSchema || saved.Bridge == nil || saved.RequestPolicy == nil {
		return saved, errCensusInventoryPrivate
	}
	return saved, nil
}

func censusCallerHostSame(a, b any) bool {
	x, ex := json.Marshal(a)
	y, ey := json.Marshal(b)
	return ex == nil && ey == nil && bytes.Equal(x, y)
}

func censusCallerHostSettingsMatch(saved censusCallerHostSettings, c censusInventoryBridgeConfig) bool {
	stable, err := censusCallerHostStable(c)
	return err == nil && censusCallerHostSame(saved.Bridge, stable.Bridge) &&
		(c.Action == "discover" || censusCallerHostSame(saved.RequestPolicy, stable.RequestPolicy))
}

func saveCensusCallerHostSettings(path string, s censusCallerHostSettings) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || s.RequestPolicy == nil {
		return errCensusInventoryPrivate
	}
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return errCensusInventoryPrivate
	}
	st, err := os.Lstat(parent)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		return errCensusInventoryPrivate
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw)+1 > 4<<20 {
		return errCensusInventoryPrivate
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(raw, '\n'))
	syncErr := f.Sync()
	return errors.Join(writeErr, syncErr, f.Close())
}

func runCensusCallerHost(options censusCLIOptions, out, errout io.Writer, deps censusRunnerDependencies) int {
	fail := func() int {
		return writeCensusFailure(errout, censusStageConfig, options.Machine, errCensusInventoryPrivate)
	}
	raw, err := applicationReadLocatorBounded(options.CatalogConfigPath, 4<<20)
	if err != nil {
		return fail()
	}
	var tag struct{ Schema string }
	if json.Unmarshal(raw, &tag) != nil {
		return fail()
	}
	if tag.Schema != censusCallerHostSettingsSchema {
		if options.CallerHostAttemptConfigPath != "" {
			return fail()
		}
		c, err := decodeCensusInventoryBridge(raw)
		if err != nil {
			return fail()
		}
		if options.SaveCallerHostSettingsPath != "" {
			s, err := censusCallerHostStable(c)
			if err != nil || saveCensusCallerHostSettings(options.SaveCallerHostSettingsPath, s) != nil {
				return fail()
			}
			if json.NewEncoder(out).Encode(struct {
				Schema, Path                 string
				Expires, ExecutionAuthorized bool
			}{
				"census-caller-host-settings-saved.v2", options.SaveCallerHostSettingsPath, false, false}) != nil {
				return 1
			}
			return 0
		}
		return runCensusInventoryBridgeConfig(c, out, errout, deps, options.Machine)
	}
	if options.SaveCallerHostSettingsPath != "" || options.CallerHostAttemptConfigPath == "" {
		return fail()
	}
	saved, err := decodeCensusCallerHostSettings(raw)
	if err != nil {
		return fail()
	}
	attempt, err := applicationReadLocatorBounded(options.CallerHostAttemptConfigPath, 4<<20)
	if err != nil {
		return fail()
	}
	c, err := decodeCensusInventoryBridge(attempt)
	if err != nil {
		return fail()
	}
	// Discovery has no application request yet. Every subsequent action must
	// agree with the saved application policy as well as the complete header.
	if !censusCallerHostSettingsMatch(saved, c) {
		return fail()
	}
	if c.Action == "discover" || c.Action == "setup" || c.Action == "run" {
		remaining := time.Until(c.Deadline)
		if remaining <= 0 || remaining > 15*time.Second {
			return fail()
		}
	}
	// Pass the same already-checked value, not a path that could be reread with
	// different policy bytes between comparison and execution. Native owner,
	// funding, cursor and journal validation remain the existing bridge's job.
	return runCensusInventoryBridgeConfig(c, out, errout, deps, options.Machine)
}
