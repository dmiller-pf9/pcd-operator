package v1alpha1

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestComponentSpecJSONTags(t *testing.T) {
	enabled := false
	replicas := int32(3)
	size := resource.MustParse("20Gi")
	minAvail := intstr.FromInt32(1)

	spec := ComponentSpec{
		Enabled:      &enabled,
		ImageTag:     "v1.2.3",
		Image:        "registry.example.invalid/nova:v1.2.3",
		ChartVersion: "0.4.1",
		Workload: &WorkloadOverride{
			Replicas:            &replicas,
			Resources:           map[string]corev1.ResourceRequirements{"*": {}},
			NodeSelector:        map[string]string{"role": "control"},
			Tolerations:         []corev1.Toleration{{Key: "dedicated"}},
			PodDisruptionBudget: &PDBSpec{MinAvailable: &minAvail},
			Autoscaling:         &HPASpec{MaxReplicas: 8},
			PriorityClassName:   "system-cluster-critical",
			Storage:             &StorageOverride{Size: &size},
		},
		Config: &ComponentConfig{
			INI: map[string]map[string]map[string]string{
				"nova.conf": {"DEFAULT": {"cpu_allocation_ratio": "8.0"}},
			},
			Files:           map[string]string{"/etc/neutron/x.ini": "[ml2]\n"},
			FilesFrom:       []FileSource{{MountPath: "/etc/extra"}},
			PolicyOverrides: map[string]string{"policy.yaml": "{}"},
		},
		HelmValues:            &apiextensionsv1.JSON{Raw: []byte(`{"key":"value"}`)},
		StrategicMergePatches: []apiextensionsv1.JSON{{Raw: []byte(`{"spec":{}}`)}},
	}

	assertJSONKeys(t, spec,
		"enabled", "imageTag", "image", "chartVersion",
		"workload", "config", "helmValues", "strategicMergePatches",
	)
}

// API-013 depends on Enabled being a pointer: the merge engine in milestone 003
// must distinguish "not set at this layer" from "explicitly false".
func TestComponentSpecEnabledIsNilable(t *testing.T) {
	var spec ComponentSpec
	if spec.Enabled != nil {
		t.Fatalf("zero ComponentSpec.Enabled = %v, want nil", spec.Enabled)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "{}" {
		t.Errorf("zero ComponentSpec marshals to %s, want {}", b)
	}
}
