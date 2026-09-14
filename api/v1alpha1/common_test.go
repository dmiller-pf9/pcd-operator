package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// API-020: spec.size accepts exactly these five values. The CRD enum is asserted
// in internal/crdcheck; this asserts the Go constants agree with it.
func TestPCDSizeValues(t *testing.T) {
	want := []PCDSize{"X-Small", "Small", "Medium", "Large", "X-Large"}
	got := []PCDSize{SizeXSmall, SizeSmall, SizeMedium, SizeLarge, SizeXLarge}

	if len(got) != len(want) {
		t.Fatalf("got %d sizes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("size[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// There is deliberately no Custom size — docs/DESIGN.md §4.5. Omitting size and
// setting componentDefaults says the same thing without a second way to say it.
func TestPCDSizeHasNoCustomValue(t *testing.T) {
	for _, s := range []PCDSize{SizeXSmall, SizeSmall, SizeMedium, SizeLarge, SizeXLarge} {
		if s == "Custom" || s == "custom" {
			t.Errorf("found a Custom size value: %q", s)
		}
	}
}

func TestUpgradePolicyJSONTags(t *testing.T) {
	autoRollback := true
	maxConcurrent := int32(2)
	p := UpgradePolicy{
		Trigger:       "Window",
		Window:        &MaintenanceWindow{Schedule: "0 2 * * SAT", Duration: metav1.Duration{Duration: 4 * time.Hour}},
		MaxConcurrent: &maxConcurrent,
		AutoRollback:  &autoRollback,
	}

	assertJSONKeys(t, p, "trigger", "window", "maxConcurrent", "autoRollback")

	// SPEC §9-3 and §9-4: neither timeout nor a concurrency budget has an
	// agreed value. An unset timeout must stay absent rather than acquire a
	// default nobody chose.
	b, err := json.Marshal(UpgradePolicy{Trigger: "Immediate"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "timeout") {
		t.Errorf("unset timeout should be omitted, got %s", b)
	}
	if strings.Contains(string(b), "maxConcurrent") {
		t.Errorf("unset maxConcurrent should be omitted, got %s", b)
	}
}
