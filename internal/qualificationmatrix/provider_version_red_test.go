package qualificationmatrix

import "testing"

func TestProviderVersionAxisRequired(t *testing.T) {
	for _, axis := range mandatoryAxes {
		if axis == "provider_version" {
			return
		}
	}
	t.Fatal("ASSERT_PROVIDER_VERSION_AXIS_REQUIRED: provider_version is absent from mandatory axes")
}
