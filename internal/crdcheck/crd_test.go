// Package crdcheck asserts properties of the generated CRD manifests that
// cannot be asserted from the Go types alone. It has no non-test source: the
// artifacts under test are the YAML files controller-gen produces.
package crdcheck

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

const crdDir = "../../config/crd/bases"

func loadCRDs(t *testing.T) map[string]apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	entries, err := os.ReadDir(crdDir)
	if err != nil {
		t.Fatalf("read %s: %v (run `make manifests`)", crdDir, err)
	}

	out := map[string]apiextensionsv1.CustomResourceDefinition{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(crdDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal(b, &crd); err != nil {
			t.Fatalf("unmarshal %s: %v", e.Name(), err)
		}
		out[crd.Spec.Names.Kind] = crd
	}
	return out
}

func TestAllThreeKindsAreGenerated(t *testing.T) {
	crds := loadCRDs(t)
	for _, kind := range []string{"PCDUnderlay", "PCDInstallation", "PCDRegion"} {
		if _, ok := crds[kind]; !ok {
			t.Errorf("no generated CRD for kind %s", kind)
		}
	}
	if len(crds) != 3 {
		t.Errorf("got %d CRDs, want exactly 3: %v", len(crds), keys(crds))
	}
}

// API-001
func TestCRDGroupAndVersion(t *testing.T) {
	for kind, crd := range loadCRDs(t) {
		t.Run(kind, func(t *testing.T) {
			if got, want := crd.Spec.Group, "install.pcd.platform9.com"; got != want {
				t.Errorf("group = %q, want %q", got, want)
			}
			if len(crd.Spec.Versions) != 1 {
				t.Fatalf("got %d versions, want 1", len(crd.Spec.Versions))
			}
			v := crd.Spec.Versions[0]
			if v.Name != "v1alpha1" {
				t.Errorf("version = %q, want v1alpha1", v.Name)
			}
			if !v.Served || !v.Storage {
				t.Errorf("version served=%v storage=%v, want both true", v.Served, v.Storage)
			}
			if v.Subresources == nil || v.Subresources.Status == nil {
				t.Errorf("status subresource is not enabled")
			}
		})
	}
}

// API-002
func TestCRDScopes(t *testing.T) {
	want := map[string]apiextensionsv1.ResourceScope{
		"PCDUnderlay":     apiextensionsv1.ClusterScoped,
		"PCDInstallation": apiextensionsv1.NamespaceScoped,
		"PCDRegion":       apiextensionsv1.NamespaceScoped,
	}
	crds := loadCRDs(t)
	for kind, wantScope := range want {
		crd, ok := crds[kind]
		if !ok {
			t.Errorf("no CRD for %s", kind)
			continue
		}
		if crd.Spec.Scope != wantScope {
			t.Errorf("%s scope = %q, want %q", kind, crd.Spec.Scope, wantScope)
		}
	}
}

// API-003
func TestCRDShortNames(t *testing.T) {
	want := map[string]string{
		"PCDUnderlay":     "pcdunderlay",
		"PCDInstallation": "pcdinstall",
		"PCDRegion":       "pcdregion",
	}
	crds := loadCRDs(t)
	for kind, wantShort := range want {
		crd, ok := crds[kind]
		if !ok {
			t.Errorf("no CRD for %s", kind)
			continue
		}
		if !contains(crd.Spec.Names.ShortNames, wantShort) {
			t.Errorf("%s shortNames = %v, want to contain %q", kind, crd.Spec.Names.ShortNames, wantShort)
		}
	}
}

// INV-7: component phase has exactly one enum definition. The test targets
// status.components[].phase specifically — status.phase is the object-level
// rollup with a deliberately different, smaller vocabulary, and conflating the
// two is the mistake this test is shaped to avoid.
func TestComponentPhaseEnumIsDefinedOnce(t *testing.T) {
	want := []string{
		"Absent", "Pending", "Preflight", "Applying", "Initializing",
		"PostConfiguring", "RollingOut", "Ready", "Degraded", "Faulted",
		"BackingOut", "Quarantined",
	}

	seen := map[string][]string{} // joined enum -> kinds that carry it
	for kind, crd := range loadCRDs(t) {
		got := componentPhaseEnum(t, kind, crd)
		if !equal(got, want) {
			t.Errorf("%s: component phase enum = %v, want %v", kind, got, want)
		}
		seen[join(got)] = append(seen[join(got)], kind)
	}
	if len(seen) != 1 {
		t.Errorf("found %d distinct component phase enums, want 1: %v", len(seen), seen)
	}
}

// The object-level rollup is deliberately a different, smaller vocabulary. If
// this ever equals the component phase set, the two have been conflated.
func TestRollupPhaseEnumIsDistinctFromComponentPhase(t *testing.T) {
	wantRollup := []string{
		"Pending", "Installing", "Upgrading", "Ready", "Degraded", "Deleting", "Error",
	}
	for kind, crd := range loadCRDs(t) {
		root := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
		status, ok := root.Properties["status"]
		if !ok {
			t.Fatalf("%s: no status property", kind)
		}
		phase, ok := status.Properties["phase"]
		if !ok {
			t.Fatalf("%s: no status.phase property", kind)
		}
		got := enumStrings(t, kind, phase)
		if !equal(got, wantRollup) {
			t.Errorf("%s: status.phase enum = %v, want %v", kind, got, wantRollup)
		}
	}
}

func componentPhaseEnum(t *testing.T, kind string, crd apiextensionsv1.CustomResourceDefinition) []string {
	t.Helper()

	root := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	status, ok := root.Properties["status"]
	if !ok {
		t.Fatalf("%s: no status property", kind)
	}
	components, ok := status.Properties["components"]
	if !ok {
		t.Fatalf("%s: no status.components property", kind)
	}
	if components.Items == nil || components.Items.Schema == nil {
		t.Fatalf("%s: status.components has no item schema", kind)
	}
	phase, ok := components.Items.Schema.Properties["phase"]
	if !ok {
		t.Fatalf("%s: no status.components[].phase property", kind)
	}
	return enumStrings(t, kind, phase)
}

func enumStrings(t *testing.T, kind string, s apiextensionsv1.JSONSchemaProps) []string {
	t.Helper()
	out := make([]string, 0, len(s.Enum))
	for _, raw := range s.Enum {
		var v string
		if err := json.Unmarshal(raw.Raw, &v); err != nil {
			t.Fatalf("%s: enum value %s is not a string: %v", kind, raw.Raw, err)
		}
		out = append(out, v)
	}
	return out
}

// INV-8: the MariaDB passthrough fields must preserve unknown fields and must
// not carry a schema of their own.
func TestPassthroughFieldsPreserveUnknownFields(t *testing.T) {
	for kind, crd := range loadCRDs(t) {
		found := 0
		walkSchema(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, "", func(p string, s apiextensionsv1.JSONSchemaProps) {
			if path.Base(p) != "template" {
				return
			}
			found++
			if s.XPreserveUnknownFields == nil || !*s.XPreserveUnknownFields {
				t.Errorf("%s: %s does not preserve unknown fields (INV-8)", kind, p)
			}
			if len(s.Properties) != 0 {
				t.Errorf("%s: %s has a typed schema with %d properties; the passthrough must not be validated (INV-8)", kind, p, len(s.Properties))
			}
		})
		// All three kinds reach DatabaseBinding and MaxScaleSpec through
		// spec.externalServices, so all three must carry both templates.
		if found < 2 {
			t.Errorf("%s: found %d template passthrough fields, want at least 2 (database and maxScale)", kind, found)
		}
	}
}

// walkSchema visits every property schema, passing a slash-separated path.
func walkSchema(s *apiextensionsv1.JSONSchemaProps, p string, fn func(string, apiextensionsv1.JSONSchemaProps)) {
	if s == nil {
		return
	}
	for name, child := range s.Properties {
		childPath := p + "/" + name
		fn(childPath, child)
		c := child
		walkSchema(&c, childPath, fn)
	}
	if s.Items != nil && s.Items.Schema != nil {
		walkSchema(s.Items.Schema, p+"/[]", fn)
	}
	if s.AdditionalProperties != nil && s.AdditionalProperties.Schema != nil {
		walkSchema(s.AdditionalProperties.Schema, p+"/{}", fn)
	}
}

func keys(m map[string]apiextensionsv1.CustomResourceDefinition) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func join(s []string) string {
	out := ""
	for _, v := range s {
		out += v + ","
	}
	return out
}
