package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PCDSize expresses the intended scale of a deployment once, instead of
// requiring an operator to hand-tune two dozen components to say the same
// thing. It is override layer 3 in docs/DESIGN.md §4.1: a default, not a
// constraint, so anything set in componentDefaults or components[name] wins
// over it.
//
// The presets themselves are release-matrix data, not code (API-022), so this
// type carries the vocabulary and nothing else.
//
// +kubebuilder:validation:Enum=X-Small;Small;Medium;Large;X-Large
type PCDSize string

const (
	SizeXSmall PCDSize = "X-Small"
	SizeSmall  PCDSize = "Small"
	SizeMedium PCDSize = "Medium"
	SizeLarge  PCDSize = "Large"
	SizeXLarge PCDSize = "X-Large"
)

// TLSSettings configures transport security for an external service binding.
type TLSSettings struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	CASecretRef *corev1.LocalObjectReference `json:"caSecretRef,omitempty"`

	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// IssuerRef names a cert-manager Issuer or ClusterIssuer.
type IssuerRef struct {
	// +kubebuilder:validation:Enum=Issuer;ClusterIssuer
	Kind string `json:"kind"`

	Name string `json:"name"`

	// +kubebuilder:default="cert-manager.io"
	// +optional
	Group string `json:"group,omitempty"`
}

// MaintenanceWindow bounds when an upgrade may start.
type MaintenanceWindow struct {
	// Schedule is a cron expression marking the start of the window.
	Schedule string `json:"schedule"`

	Duration metav1.Duration `json:"duration"`

	// +kubebuilder:default="UTC"
	// +optional
	TimeZone string `json:"timeZone,omitempty"`
}

// UpgradePolicy governs how upgrades roll.
type UpgradePolicy struct {
	// +kubebuilder:validation:Enum=Immediate;Window;Manual
	// +kubebuilder:default=Immediate
	Trigger string `json:"trigger"`

	// +optional
	Window *MaintenanceWindow `json:"window,omitempty"`

	// MaxConcurrent caps how many child objects upgrade at once. On a
	// PCDUnderlay this is the fleet-wide budget.
	//
	// No default is agreed (docs/SPEC.md §9-4). It is nil and unenforced until
	// one is specified — do not invent a value.
	// +optional
	MaxConcurrent *int32 `json:"maxConcurrent,omitempty"`

	// Timeout after which an in-flight operation is treated as Faulted.
	//
	// No default is agreed (docs/SPEC.md §9-3). Deadlines belong in the release
	// matrix as per-component data seeded from observed p99 durations — do not
	// invent a value here.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// AutoRollback permits automatic back-out for components whose rollback
	// safety class allows it. Components of class C (schema-bearing) and D
	// (quorum/stateful) never roll back automatically regardless of this
	// setting — see docs/SPEC.md §3 INV-3.
	// +kubebuilder:default=true
	// +optional
	AutoRollback *bool `json:"autoRollback,omitempty"`
}

// ProtectionSpec prevents accidental deletion. Enforced at the admission
// webhook so the block happens before any teardown work is dispatched.
type ProtectionSpec struct {
	PreventDeletion bool `json:"preventDeletion"`

	// +optional
	Reason string `json:"reason,omitempty"`

	// +optional
	Owner string `json:"owner,omitempty"`
}
