package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentPhase is the per-component state machine vocabulary from
// docs/DESIGN.md §12.3.
//
// This is the ONLY definition of the component phase enum anywhere in the tree
// (docs/SPEC.md §3 INV-7). All three kinds reach it through CommonStatus, so
// adding a second +kubebuilder:validation:Enum for component phase elsewhere
// would let the two drift apart silently.
//
// Ready is the only stable state. Everything else is either transient or
// explicitly awaiting a decision, which is what makes "is this converged?"
// answerable by inspection.
//
// +kubebuilder:validation:Enum=Absent;Pending;Preflight;Applying;Initializing;PostConfiguring;RollingOut;Ready;Degraded;Faulted;BackingOut;Quarantined
type ComponentPhase string

const (
	PhaseAbsent          ComponentPhase = "Absent"
	PhasePending         ComponentPhase = "Pending"
	PhasePreflight       ComponentPhase = "Preflight"
	PhaseApplying        ComponentPhase = "Applying"
	PhaseInitializing    ComponentPhase = "Initializing"
	PhasePostConfiguring ComponentPhase = "PostConfiguring"
	PhaseRollingOut      ComponentPhase = "RollingOut"
	PhaseReady           ComponentPhase = "Ready"
	PhaseDegraded        ComponentPhase = "Degraded"
	PhaseFaulted         ComponentPhase = "Faulted"
	PhaseBackingOut      ComponentPhase = "BackingOut"
	PhaseQuarantined     ComponentPhase = "Quarantined"
)

// AllComponentPhases returns every component phase in state-machine order. It
// exists so a test can assert the enum marker and the constants agree; keeping
// them in sync by eye is exactly the INV-7 failure.
func AllComponentPhases() []ComponentPhase {
	return []ComponentPhase{
		PhaseAbsent, PhasePending, PhasePreflight, PhaseApplying,
		PhaseInitializing, PhasePostConfiguring, PhaseRollingOut, PhaseReady,
		PhaseDegraded, PhaseFaulted, PhaseBackingOut, PhaseQuarantined,
	}
}

// FaultClass is how a failure is classified before any recovery action is
// chosen (docs/DESIGN.md §12.4). Recovery differs by class, so classification
// has to happen first.
//
// +kubebuilder:validation:Enum=Transient;Blocked;DriftConflict;PartialCommit
type FaultClass string

const (
	// FaultTransient failed for a reason unrelated to desired state. Retry in
	// place with backoff, bounded, then Quarantined.
	FaultTransient FaultClass = "Transient"

	// FaultBlocked cannot complete because something it needs does not exist.
	// It MUST NOT retry: name the unmet dependency in BlockedOn and fail at
	// the deadline. An unbounded retry loop is indistinguishable from progress.
	FaultBlocked FaultClass = "Blocked"

	// FaultDriftConflict means live state was changed out of band and the
	// apply was rejected. Retrying unchanged fails identically every time.
	FaultDriftConflict FaultClass = "DriftConflict"

	// FaultPartialCommit means the step failed after an irreversible side
	// effect. Recovery depends entirely on the rollback safety class.
	FaultPartialCommit FaultClass = "PartialCommit"
)

// AllFaultClasses returns every fault class.
func AllFaultClasses() []FaultClass {
	return []FaultClass{FaultTransient, FaultBlocked, FaultDriftConflict, FaultPartialCommit}
}

// OperationType names an in-flight operation neutrally. Backend job names are
// never surfaced here (docs/SPEC.md §3 INV-1, §6 STATE-010).
//
// +kubebuilder:validation:Enum=Install;Upgrade;Resize;Teardown
type OperationType string

const (
	OpInstall  OperationType = "Install"
	OpUpgrade  OperationType = "Upgrade"
	OpResize   OperationType = "Resize"
	OpTeardown OperationType = "Teardown"
)

// AllOperationTypes returns every operation type.
func AllOperationTypes() []OperationType {
	return []OperationType{OpInstall, OpUpgrade, OpResize, OpTeardown}
}

// CommonStatus is the status surface shared by all three kinds.
type CommonStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a coarse rollup, derived from observed cluster state rather than
	// from any single backend response. Backend-internal state vocabulary is
	// translated into this enum by the adapter and never surfaced verbatim.
	//
	// This is the object-level rollup and is deliberately a different, smaller
	// vocabulary from ComponentPhase — do not conflate the two.
	// +kubebuilder:validation:Enum=Pending;Installing;Upgrading;Ready;Degraded;Deleting;Error
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions carries Reconciling, Available, Progressing, Degraded,
	// ReleaseResolved, PrerequisitesMet and UpgradeInProgress.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AppliedRelease is what is actually running, which may lag spec.release
	// mid-upgrade. Comparing the two is how you answer "is this on 2026.4?"
	// +optional
	AppliedRelease *AppliedRelease `json:"appliedRelease,omitempty"`

	// Components carries per-component health so a single unhealthy service
	// does not have to be inferred from a rolled-up phase.
	// +listType=map
	// +listMapKey=name
	// +optional
	Components []ComponentStatus `json:"components,omitempty"`

	// +optional
	ReadyComponents int32 `json:"readyComponents,omitempty"`

	// +optional
	DesiredComponents int32 `json:"desiredComponents,omitempty"`

	// RenderedValuesRef points at a ConfigMap holding the fully-merged values
	// after all six override layers, for diffing and support (API-012).
	// +optional
	RenderedValuesRef *corev1.LocalObjectReference `json:"renderedValuesRef,omitempty"`

	// ActiveOperation reports the in-flight operation, when it started, and
	// where to find its logs, so `kubectl describe` is enough to debug.
	// +optional
	ActiveOperation *OperationStatus `json:"activeOperation,omitempty"`
}

// ComponentStatus is per-component health and state. Defined exactly once
// (docs/SPEC.md §3 INV-7) and reached by all three kinds through CommonStatus.
type ComponentStatus struct {
	Name string `json:"name"`

	// Phase is the per-component state machine state.
	Phase ComponentPhase `json:"phase"`

	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// FaultClass is set whenever Phase is Faulted, BackingOut or Quarantined
	// (STATE-004).
	// +optional
	FaultClass FaultClass `json:"faultClass,omitempty"`

	// BlockedOn names the unmet dependencies when FaultClass is Blocked.
	// Without this a deadlock is indistinguishable from slow progress.
	// +optional
	BlockedOn []string `json:"blockedOn,omitempty"`

	// RollbackClass is the component's declared back-out safety class (A-D).
	// It comes from the component catalog data, not from code (STATE-008).
	// +optional
	RollbackClass string `json:"rollbackClass,omitempty"`

	// Attempts is the retry count for the current desired version. Resets when
	// the desired version changes, not when a retry succeeds.
	// +optional
	Attempts int32 `json:"attempts,omitempty"`

	// ObservedVersion is what is actually running; compare against desired to
	// answer "did this component take the upgrade?"
	// +optional
	ObservedVersion string `json:"observedVersion,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// AppliedRelease records what was actually resolved and applied.
type AppliedRelease struct {
	MatrixVersion string `json:"matrixVersion"`

	// Chart is the artifact actually resolved and applied, which may differ
	// from the matrix default when ChartOverride was set.
	// +optional
	Chart *ChartRef `json:"chart,omitempty"`

	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`
}

// OperationStatus reports the in-flight operation.
type OperationStatus struct {
	// Type is a neutral operation name. Backend job names are never surfaced
	// here (INV-1, STATE-010).
	Type OperationType `json:"type"`

	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// Deadline after which the operation is treated as Faulted rather than
	// left to run indefinitely. INV-4 requires every wait to have one.
	//
	// The *values* are a SPEC §9-3 open decision and belong in the release
	// matrix as per-component data. This field only records the deadline that
	// was chosen; do not compile a default in.
	// +optional
	Deadline *metav1.Time `json:"deadline,omitempty"`

	// Attempt counts retries of the current desired state.
	// +optional
	Attempt int32 `json:"attempt,omitempty"`

	// LogHint points at where to look, in terms the person reading it can use
	// without knowing what the backend is.
	// +optional
	LogHint string `json:"logHint,omitempty"`
}
