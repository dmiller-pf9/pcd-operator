package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestPCDUnderlayRegistersInScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	gvk := GroupVersion.WithKind("PCDUnderlay")
	if !s.Recognizes(gvk) {
		t.Errorf("scheme does not recognize %s", gvk)
	}
	if !s.Recognizes(GroupVersion.WithKind("PCDUnderlayList")) {
		t.Errorf("scheme does not recognize PCDUnderlayList")
	}
}

func TestPCDUnderlaySpecJSONTags(t *testing.T) {
	spec := PCDUnderlaySpec{
		Profile:           "on-prem",
		Release:           ReleaseSpec{MatrixVersion: "2026.4"},
		Platform:          PlatformSpec{Kind: "nodelet"},
		ExternalServices:  &ExternalServices{},
		Networking:        InfraNetworkingSpec{HostedZone: "pcd.example.invalid"},
		ComponentDefaults: &ComponentSpec{},
		Components:        map[string]ComponentSpec{"cert-manager": {}},
		UpgradePolicy:     &UpgradePolicy{Trigger: "Immediate"},
		Paused:            true,
	}
	assertJSONKeys(t, spec,
		"profile", "release", "platform", "externalServices", "networking",
		"componentDefaults", "components", "upgradePolicy", "paused",
	)
}

// INV-6: the underlay describes the one cluster the operator runs in. There is
// no cluster selection anywhere in it.
func TestPlatformSpecHasNoClusterSelection(t *testing.T) {
	assertJSONKeys(t, PlatformSpec{Kind: "aws-eks", Region: "us-west-2", Airgapped: true},
		"kind", "region", "airgapped")
}
