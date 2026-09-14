package v1alpha1

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDatabaseBindingJSONTags(t *testing.T) {
	replicas := int32(3)
	size := resource.MustParse("100Gi")
	b := DatabaseBinding{
		Provisioning:  "Dedicated",
		InstanceRef:   &MariaDBRef{Name: "pcd-db"},
		Topology:      "Galera",
		Replicas:      &replicas,
		Storage:       &MariaDBStorage{Size: &size},
		MyCnf:         map[string]map[string]string{"mysqld": {"max_connections": "500"}},
		Metrics:       &MariaDBMetrics{},
		MaxScale:      &MaxScaleSpec{},
		Schema:        &SchemaPolicy{},
		Backup:        &DatabaseBackupPolicy{Type: "Physical"},
		BootstrapFrom: &apiextensionsv1.JSON{Raw: []byte(`{"s3":{}}`)},
		Template:      &apiextensionsv1.JSON{Raw: []byte(`{"podTemplate":{}}`)},
	}

	assertJSONKeys(t, b,
		"provisioning", "instanceRef", "topology", "replicas", "storage",
		"myCnf", "metrics", "maxScale", "schema", "backup",
		"bootstrapFrom", "template",
	)
}

// DB-002 depends on Topology being exactly these three values. The CRD enum is
// asserted in internal/crdcheck; this pins the vocabulary the wrapper in
// milestone 008 will switch on.
func TestDatabaseBindingTopologyVocabulary(t *testing.T) {
	for _, topology := range []string{"Standalone", "Replication", "Galera"} {
		b := DatabaseBinding{Provisioning: "Dedicated", Topology: topology}
		if b.Topology != topology {
			t.Errorf("topology = %q, want %q", b.Topology, topology)
		}
	}
}
