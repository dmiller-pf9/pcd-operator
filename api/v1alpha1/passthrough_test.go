package v1alpha1

import (
	"encoding/json"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// INV-8: DatabaseBinding.Template and MaxScaleSpec.Template are not
// schema-validated by this operator. An arbitrary unknown field must survive a
// round trip byte-for-byte — if it does not, something is parsing the
// passthrough, which is the drift this invariant exists to prevent.
func TestTemplatePassthroughSurvivesRoundTrip(t *testing.T) {
	// Deliberately includes a field no MariaDB version has, nested three deep,
	// and leads with a key that sorts last. Any code that parsed this into a
	// map[string]any and re-marshalled it would emit the keys in alphabetical
	// order, moving "zzzLast" to the end and failing the comparison below. That
	// is what makes this an assertion about bytes rather than about content.
	raw := `{"zzzLast":1,"podTemplate":{"nodeSelector":{"role":"db"}},"someFieldUpstreamAddedLastWeek":{"nested":{"deep":true}},"tls":{"enabled":true}}`

	binding := DatabaseBinding{
		Provisioning: "Dedicated",
		Template:     &apiextensionsv1.JSON{Raw: []byte(raw)},
		MaxScale: &MaxScaleSpec{
			Template: &apiextensionsv1.JSON{Raw: []byte(raw)},
		},
	}

	encoded, err := json.Marshal(binding)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DatabaseBinding
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Template == nil {
		t.Fatal("Template is nil after round trip")
	}
	if got := string(decoded.Template.Raw); got != raw {
		t.Errorf("DatabaseBinding.Template round trip:\n got %s\nwant %s", got, raw)
	}
	if decoded.MaxScale == nil || decoded.MaxScale.Template == nil {
		t.Fatal("MaxScale.Template is nil after round trip")
	}
	if got := string(decoded.MaxScale.Template.Raw); got != raw {
		t.Errorf("MaxScaleSpec.Template round trip:\n got %s\nwant %s", got, raw)
	}
}

// The deepcopy generated for the passthrough must copy the bytes, not alias
// them — an aliased Raw slice would let a status update mutate spec.
func TestTemplatePassthroughDeepCopyIsNotAliased(t *testing.T) {
	original := DatabaseBinding{
		Provisioning: "Dedicated",
		Template:     &apiextensionsv1.JSON{Raw: []byte(`{"a":1}`)},
	}

	clone := original.DeepCopy()
	if clone.Template == original.Template {
		t.Fatal("DeepCopy returned the same *JSON pointer")
	}
	clone.Template.Raw[0] = 'X'
	if string(original.Template.Raw) != `{"a":1}` {
		t.Errorf("mutating the clone changed the original: %s", original.Template.Raw)
	}
}
