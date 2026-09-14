package v1alpha1

import "testing"

// STATE-001: exactly these twelve phases, in this order. The generated CRD enum
// is asserted in internal/crdcheck; this pins the Go constants that the state
// machine in milestone 009 switches on.
func TestComponentPhaseVocabulary(t *testing.T) {
	want := []ComponentPhase{
		"Absent", "Pending", "Preflight", "Applying", "Initializing",
		"PostConfiguring", "RollingOut", "Ready", "Degraded", "Faulted",
		"BackingOut", "Quarantined",
	}
	got := AllComponentPhases()

	if len(got) != len(want) {
		t.Fatalf("got %d phases, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phase[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// STATE-004: exactly these four fault classes.
func TestFaultClassVocabulary(t *testing.T) {
	want := []FaultClass{"Transient", "Blocked", "DriftConflict", "PartialCommit"}
	got := AllFaultClasses()

	if len(got) != len(want) {
		t.Fatalf("got %d fault classes, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("faultClass[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// STATE-010: the operation type vocabulary is neutral. A backend job name here
// would be an INV-1 violation, which is why the set is closed.
func TestOperationTypeVocabulary(t *testing.T) {
	want := []OperationType{"Install", "Upgrade", "Resize", "Teardown"}
	got := AllOperationTypes()

	if len(got) != len(want) {
		t.Fatalf("got %d operation types, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("operationType[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCommonStatusJSONTags(t *testing.T) {
	s := CommonStatus{
		ObservedGeneration: 4,
		Phase:              "Ready",
		AppliedRelease:     &AppliedRelease{MatrixVersion: "2026.4"},
		Components:         []ComponentStatus{{Name: "keystone", Phase: PhaseReady}},
		ReadyComponents:    45,
		DesiredComponents:  45,
		ActiveOperation:    &OperationStatus{Type: OpInstall},
	}
	assertJSONKeys(t, s,
		"observedGeneration", "phase", "appliedRelease", "components",
		"readyComponents", "desiredComponents", "activeOperation",
	)
}
