package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestPCDInstallationRegistersInScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	for _, kind := range []string{"PCDInstallation", "PCDInstallationList"} {
		if !s.Recognizes(GroupVersion.WithKind(kind)) {
			t.Errorf("scheme does not recognize %s", kind)
		}
	}
}

func TestPCDInstallationSpecJSONTags(t *testing.T) {
	spec := PCDInstallationSpec{
		ShortName:         "acme",
		DisplayName:       "Acme Corp",
		AdminEmail:        "admin@acme.invalid",
		FQDN:              "acme.pcd.example.invalid",
		Release:           &ReleaseSpec{MatrixVersion: "2026.4"},
		ExternalServices:  &ExternalServices{},
		SSO:               &IdentityBinding{Provider: "oidc"},
		ComponentDefaults: &ComponentSpec{},
		Components:        map[string]ComponentSpec{"keystone": {}},
		Size:              SizeMedium,
		Protection:        &ProtectionSpec{PreventDeletion: true},
		UpgradePolicy:     &UpgradePolicy{Trigger: "Immediate"},
		Paused:            true,
	}
	assertJSONKeys(t, spec,
		"shortName", "displayName", "adminEmail", "fqdn", "release",
		"externalServices", "sso", "componentDefaults", "components", "size",
		"protection", "upgradePolicy", "paused",
	)
}

// DESIGN §8: a PCDRegion inherits its installation's size when unset, so an
// unset size must be distinguishable from any valid value. The empty string is
// not one of the five.
func TestPCDInstallationSizeIsOmittedWhenUnset(t *testing.T) {
	spec := PCDInstallationSpec{ShortName: "acme", AdminEmail: "a@b.invalid"}
	if spec.Size != "" {
		t.Fatalf("unset size = %q, want empty", spec.Size)
	}
	for _, s := range []PCDSize{SizeXSmall, SizeSmall, SizeMedium, SizeLarge, SizeXLarge} {
		if s == "" {
			t.Fatalf("a valid size is the empty string, which makes inheritance undecidable")
		}
	}
}
