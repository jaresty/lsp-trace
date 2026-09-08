package hydratedevidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// CheckFocusRequest is input-only preflight, safe before opening any artifact.
func CheckFocusRequest(f FocusRequest) error {
	if err := checkPolicy(f.CorePolicy); err != nil {
		return err
	}
	if f.PositionEncoding != "" && f.PositionEncoding != "utf-8" && f.PositionEncoding != "utf-16" && f.PositionEncoding != "utf-32" {
		return errors.New("unsupported position encoding")
	}
	if len(f.NodeIDs)+len(f.RelationIDs)+len(f.SidecarRecordIDs) > f.CorePolicy.MaxOrigins {
		return errors.New("focused origin input budget")
	}
	for _, ids := range [][]string{f.NodeIDs, f.RelationIDs, f.SidecarRecordIDs} {
		for _, id := range ids {
			if len(id) > 1024 {
				return errors.New("focused ID byte budget")
			}
		}
	}
	raw, err := json.Marshal(f)
	if err != nil || len(raw) > 4<<20 {
		return errors.New("focused selection byte budget")
	}
	return nil
}

// FocusedText retains the validated full catalog, but only renders selected sources.
// Manifest-only outcomes remain visible independently of Bundle.Complete.
func FocusedText(input Input, f FocusRequest, result FocusResult) (string, error) {
	if err := ValidateFocused(input, f, result); err != nil {
		return "", err
	}
	selected := map[string]bool{}
	var out strings.Builder
	fmt.Fprintf(&out, "Focus %s (manifest dispositions are not Bundle.Complete)\n", result.Manifest.Digest)
	for _, o := range result.Manifest.Origins {
		fmt.Fprintf(&out, "Focus origin %d kind=%q requested=%q disposition=%q call_sites=%d\n", o.Ordinal, o.Kind, o.RequestedID, o.Status, o.CallSiteCount)
		for _, s := range o.Sites {
			fmt.Fprintf(&out, "  Site role=%q pointer=%q disposition=%q origins=%q\n", s.Role, s.Pointer, s.Status, s.OriginIDs)
		}
		if out.Len() > result.Request.Policy.MaxOutputBytes {
			return "", errors.New("text output byte budget")
		}
	}
	for _, o := range result.Bundle.Origins {
		selected[o.Selection.SourceID] = true
	}
	body, err := text(result.Request, result.Bundle, selected)
	if err != nil {
		return "", err
	}
	if out.Len()+len(body) > result.Request.Policy.MaxOutputBytes {
		return "", errors.New("text output byte budget")
	}
	out.WriteString(body)
	return out.String(), nil
}
