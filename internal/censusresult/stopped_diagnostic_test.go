package censusresult

import (
	"bytes"
	"lsp-trace/sessionruntime"
	"testing"
)

func TestStopped66DTO(t *testing.T) {
	for _, code := range []Code{CodeDiscoveryRetained, CodeDiscoveryIndeterminate} {
		t.Run(string(code), func(t *testing.T) {
			d, _ := NewDiagnostic(StageDiscovery, nil)
			d.Code = code
			d.Retry = false
			raw, e := MarshalDiagnostic(d)
			if e != nil {
				t.Fatal("CLOSED_STOP_MARSHAL", e)
			}
			got, e := DecodeDiagnostic(raw)
			if e != nil || got.Code != code || got.Retry || got.Status != "FAILED" {
				t.Fatal("CLOSED_STOP_ROUNDTRIP", e)
			}
			for _, mutate := range []func(*Diagnostic){
				func(v *Diagnostic) { v.Retry = true }, func(v *Diagnostic) { v.Stage = StageConfig }, func(v *Diagnostic) { v.Status = "PARTIAL" }, func(v *Diagnostic) { v.Detail = "secret://source" }, func(v *Diagnostic) { n := 0; v.BatchOrdinal = &n }, func(v *Diagnostic) { v.Code = "DISCOVERY_FUTURE" },
			} {
				bad := d
				mutate(&bad)
				if _, e := MarshalDiagnostic(bad); e == nil {
					t.Fatal("INVALID_STOP_REJECTED")
				}
			}
			for _, ref := range []sessionruntime.PrivateDiscoveryRefusal{
				{Stage: sessionruntime.PrivateDiscoveryRead, Currency: sessionruntime.PrivateDiscoveryResponseBytes, Requested: 1, Remaining: 0, AmountsAvailable: true, CleanupUncertain: true},
				{Stage: sessionruntime.PrivateDiscoveryUnknownStage},
			} {
				d.PrivateDiscoveryRefusal = &ref
				raw, e = MarshalDiagnostic(d)
				if e != nil {
					t.Fatal(e)
				}
				got, e = DecodeDiagnostic(raw)
				if e != nil || got.PrivateDiscoveryRefusal == nil || *got.PrivateDiscoveryRefusal != ref || got.Code != code {
					t.Fatal("STOP_AND_FIRST_REFUSAL", e)
				}
				for _, bad := range [][]byte{
					bytes.Replace(raw, []byte(`"stage":"`), []byte(`"unknown":1,"stage":"`), 1),
					bytes.Replace(raw, []byte(`"amounts_available":`), []byte(`"amounts_available":false,"amounts_available":`), 1),
					append(append([]byte(nil), raw...), []byte(`{}`)...),
					bytes.Replace(raw, []byte(`"currency":"`), []byte(`"currency":"future`), 1),
				} {
					if _, e := DecodeDiagnostic(bad); e == nil {
						t.Fatal("STRICT_STOP_JSON")
					}
				}
			}
		})
	}
	for _, stage := range []Stage{StageSyntax, StageConfig, StageDiscovery, StageAcquisition, StageAssembly, StagePublication, StageCommitted} {
		d, e := NewDiagnostic(stage, nil)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = MarshalDiagnostic(d); e != nil {
			t.Fatal(e)
		}
		if d.Retry != (stage != StageCommitted) {
			t.Fatal("ORDINARY_RETRY_DEFAULT")
		}
		if stage != StageCommitted {
			d.Retry = false
			if _, e = MarshalDiagnostic(d); e == nil {
				t.Fatal("NO_GLOBAL_RETRY_EXCEPTION")
			}
		}
	}
}
