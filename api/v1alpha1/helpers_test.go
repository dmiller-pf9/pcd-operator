package v1alpha1

import (
	"encoding/json"
	"testing"
)

// assertJSONKeys marshals v and fails if any of keys is absent from the result.
//
// A json tag IS the CRD field name, so these assertions are how a typo in a tag
// gets caught before controller-gen bakes it into a schema.
func assertJSONKeys(t *testing.T, v any, keys ...string) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range keys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %s", k, b)
		}
	}
}
