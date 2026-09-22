package censusresult

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/strictjson"
)

const DiscoveryDiagnosticSchemaVersion = "lsp-trace.census-discovery-diagnostic.v2"

type DiscoveryDimension string
type ObservationState string

const (
	DimensionNodeCeiling           DiscoveryDimension = "NODE_CEILING"
	DimensionDepthBoundary         DiscoveryDimension = "DEPTH_BOUNDARY"
	DimensionTargetSourceOmissions DiscoveryDimension = "TARGET_SOURCE_OMISSIONS"
	DimensionProviderCompleteness  DiscoveryDimension = "PROVIDER_COMPLETENESS"
	DimensionTimeoutRequestBudget  DiscoveryDimension = "TIMEOUT_REQUEST_BUDGET"
	DimensionMessageByteBudget     DiscoveryDimension = "MESSAGE_BYTE_BUDGET"
	ObservationKnown               ObservationState   = "KNOWN"
	ObservationUnknown             ObservationState   = "UNKNOWN"
)

type DiscoveryObservation struct {
	Dimension DiscoveryDimension `json:"dimension"`
	State     ObservationState   `json:"state"`
	Observed  *uint64            `json:"observed,omitempty"`
	Limit     *uint64            `json:"limit,omitempty"`
	Omitted   *uint64            `json:"omitted,omitempty"`
}

type DiscoveryDiagnostic struct {
	SchemaVersion      string                 `json:"schema_version"`
	Status             string                 `json:"status"`
	Stage              string                 `json:"stage"`
	Code               string                 `json:"code"`
	CallerAction       string                 `json:"caller_action"`
	Retry              bool                   `json:"retry"`
	Invariant          string                 `json:"invariant"`
	CorrectionFragment string                 `json:"correction_fragment"`
	Observations       []DiscoveryObservation `json:"observations"`
}

func NewDiscoveryDiagnosticFromAcquisition(observations []censusacquisition.DiscoveryObservation, request censusrequest.SemanticFields) (DiscoveryDiagnostic, error) {
	mapped := make([]DiscoveryObservation, 0, len(observations))
	for _, observation := range observations {
		state := ObservationUnknown
		if observation.Known {
			state = ObservationKnown
		}
		mapped = append(mapped, DiscoveryObservation{Dimension: DiscoveryDimension(observation.Dimension), State: state, Observed: observation.Observed, Limit: observation.Limit, Omitted: observation.Omitted})
	}
	return NewDiscoveryDiagnosticForRequest(mapped, request)
}

func NewDiscoveryDiagnostic(observations []DiscoveryObservation) (DiscoveryDiagnostic, error) {
	return NewDiscoveryDiagnosticForRequest(observations, censusrequest.SemanticFields{MaxNodes: censusrequest.DefaultMaxNodes, TimeoutMS: censusrequest.DefaultTimeoutMS, RequestTimeoutMS: censusrequest.DefaultRequestTimeoutMS})
}

func NewDiscoveryDiagnosticForRequest(observations []DiscoveryObservation, request censusrequest.SemanticFields) (DiscoveryDiagnostic, error) {
	fragment, err := recoveryFragment(observations, request)
	if err != nil {
		return DiscoveryDiagnostic{}, err
	}
	d := DiscoveryDiagnostic{
		SchemaVersion: DiscoveryDiagnosticSchemaVersion,
		Status:        "FAILED", Stage: "discovery", Code: "DISCOVERY_BOUNDED_INCOMPLETE",
		CallerAction: "REQUIRED", Retry: true,
		Invariant:          "bounded discovery observations do not establish absence or completeness",
		CorrectionFragment: fragment,
		Observations:       append([]DiscoveryObservation(nil), observations...),
	}
	if err := validateDiscoveryDiagnostic(d); err != nil {
		return DiscoveryDiagnostic{}, err
	}
	return d, nil
}

func recoveryFragment(observations []DiscoveryObservation, request censusrequest.SemanticFields) (string, error) {
	fragment := struct {
		MaxNodes         *uint64 `json:"max_nodes,omitempty"`
		TimeoutMS        *uint64 `json:"timeout_ms,omitempty"`
		RequestTimeoutMS *uint64 `json:"request_timeout_ms,omitempty"`
	}{}
	for _, observation := range observations {
		if observation.State != ObservationKnown {
			continue
		}
		switch observation.Dimension {
		case DimensionNodeCeiling:
			if request.MaxNodes < censusrequest.DefaultMaxNodes {
				value := request.MaxNodes * 2
				if value > censusrequest.DefaultMaxNodes {
					value = censusrequest.DefaultMaxNodes
				}
				fragment.MaxNodes = &value
			}
		case DimensionTimeoutRequestBudget:
			if request.TimeoutMS < censusrequest.DefaultTimeoutMS {
				value := request.TimeoutMS * 2
				if value > censusrequest.DefaultTimeoutMS {
					value = censusrequest.DefaultTimeoutMS
				}
				fragment.TimeoutMS = &value
			}
			if request.RequestTimeoutMS < request.TimeoutMS {
				value := request.RequestTimeoutMS * 2
				if value > request.TimeoutMS {
					value = request.TimeoutMS
				}
				fragment.RequestTimeoutMS = &value
			}
		}
	}
	raw, err := json.Marshal(fragment)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func validateDiscoveryDiagnostic(d DiscoveryDiagnostic) error {
	if d.SchemaVersion != DiscoveryDiagnosticSchemaVersion || d.Status != "FAILED" || d.Stage != "discovery" || d.Code != "DISCOVERY_BOUNDED_INCOMPLETE" || d.CallerAction != "REQUIRED" || !d.Retry || d.Invariant == "" || d.CorrectionFragment == "" || len(d.Observations) == 0 || len(d.Observations) > 6 {
		return errors.New("invalid discovery diagnostic")
	}
	allowed := map[DiscoveryDimension]bool{DimensionNodeCeiling: true, DimensionDepthBoundary: true, DimensionTargetSourceOmissions: true, DimensionProviderCompleteness: true, DimensionTimeoutRequestBudget: true, DimensionMessageByteBudget: true}
	seen := map[DiscoveryDimension]bool{}
	for _, o := range d.Observations {
		if !allowed[o.Dimension] || seen[o.Dimension] {
			return errors.New("invalid discovery observation dimension")
		}
		seen[o.Dimension] = true
		if o.State == ObservationUnknown {
			if o.Observed != nil || o.Limit != nil || o.Omitted != nil {
				return errors.New("unknown observation has values")
			}
		} else if o.State != ObservationKnown || o.Observed == nil && o.Limit == nil && o.Omitted == nil {
			return errors.New("invalid discovery observation")
		}
	}
	return nil
}

func MarshalDiscoveryDiagnostic(d DiscoveryDiagnostic) ([]byte, error) {
	if err := validateDiscoveryDiagnostic(d); err != nil {
		return nil, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func DecodeDiscoveryDiagnostic(raw []byte) (DiscoveryDiagnostic, error) {
	var d DiscoveryDiagnostic
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return d, errors.New("invalid discovery diagnostic JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, errors.New("invalid discovery diagnostic JSON")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return d, errors.New("trailing discovery diagnostic JSON")
	}
	if err := validateDiscoveryDiagnostic(d); err != nil {
		return d, err
	}
	return d, nil
}
