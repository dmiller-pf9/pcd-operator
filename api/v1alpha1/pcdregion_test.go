package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPCDRegionRegistersInScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	for _, kind := range []string{"PCDRegion", "PCDRegionList"} {
		if !s.Recognizes(GroupVersion.WithKind(kind)) {
			t.Errorf("scheme does not recognize %s", kind)
		}
	}
}

func TestPCDRegionSpecJSONTags(t *testing.T) {
	spec := PCDRegionSpec{
		InstallationRef:   corev1.LocalObjectReference{Name: "acme"},
		RegionName:        "Region-One",
		RegionInstance:    "region-one",
		FQDN:              "acme-region-one.pcd.example.invalid",
		Release:           &ReleaseSpec{MatrixVersion: "2026.4"},
		ExternalServices:  &ExternalServices{},
		Networking:        &RegionNetworkingSpec{},
		Features:          &RegionFeatures{},
		Size:              SizeSmall,
		ComponentDefaults: &ComponentSpec{},
		Components:        map[string]ComponentSpec{"nova": {}},
		Protection:        &ProtectionSpec{PreventDeletion: true},
		UpgradePolicy:     &UpgradePolicy{Trigger: "Immediate"},
		Paused:            true,
	}
	assertJSONKeys(t, spec,
		"installationRef", "regionName", "regionInstance", "fqdn", "release",
		"externalServices", "networking", "features", "size",
		"componentDefaults", "components", "protection", "upgradePolicy", "paused",
	)
}

// SPEC §10: PCDHostPool is deferred and MUST NOT be stubbed in PCDRegion
// meanwhile. A half-modelled field is harder to remove than to add.
func TestPCDRegionHasNoHostManagementStanza(t *testing.T) {
	b, err := json.Marshal(PCDRegionSpec{
		InstallationRef: corev1.LocalObjectReference{Name: "acme"},
		RegionName:      "Region-One",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, banned := range []string{"hostManagement", "hostPool", "hosts", "hostAgent"} {
		if strings.Contains(string(b), banned) {
			t.Errorf("PCDRegionSpec contains %q: host lifecycle is deferred to PCDHostPool", banned)
		}
	}
}

func TestRegionFeaturesJSONTags(t *testing.T) {
	on := true
	f := RegionFeatures{
		Orchestration: &on, IaC: &on, LoadBalancing: &on, DNSaaS: &on,
		KeyManagement: &on, Optimization: &on, HighAvailability: &on,
		AppCatalog: &on, Audit: &on, Kubernetes: &on,
	}
	assertJSONKeys(t, f,
		"orchestration", "iac", "loadBalancing", "dnsaas", "keyManagement",
		"optimization", "highAvailability", "appCatalog", "audit", "kubernetes",
	)
}
