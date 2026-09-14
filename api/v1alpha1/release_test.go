package v1alpha1

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestReleaseSpecJSONTags(t *testing.T) {
	spec := ReleaseSpec{
		MatrixVersion: "2026.4",
		Source:        &ReleaseMatrixSource{OCIRef: "oci://quay.io/platform9/pcd-release-matrix:2026.4"},
		ChartOverride: &ChartRef{
			URL:           "https://example.invalid/chart.tgz",
			Version:       "1.2.3",
			PullSecretRef: &corev1.LocalObjectReference{Name: "pull"},
		},
		ImageRegistry: &RegistrySpec{Host: "registry.example.invalid", PathPrefix: "pcd"},
	}

	assertJSONKeys(t, spec, "matrixVersion", "source", "chartOverride", "imageRegistry")
}

func TestReleaseSpecOmitsEmptyOptionalFields(t *testing.T) {
	b, err := json.Marshal(ReleaseSpec{MatrixVersion: "2026.4"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"matrixVersion":"2026.4"}`; string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}
