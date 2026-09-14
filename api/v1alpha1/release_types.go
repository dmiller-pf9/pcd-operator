package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// ReleaseSpec pins the collective release and allows per-component escape
// hatches. It is override layer 1 — the lowest — in the precedence order
// documented in docs/DESIGN.md §4.1.
type ReleaseSpec struct {
	// MatrixVersion is the collective PCD release, e.g. "2026.4",
	// "2026.4-patch2", "2026.8". Resolved via Source into a concrete artifact
	// set.
	// +kubebuilder:validation:Required
	MatrixVersion string `json:"matrixVersion"`

	// Source of the release matrix. Defaults to the operator's embedded matrix
	// for the version it shipped with; override for airgapped or custom-build
	// cases.
	// +optional
	Source *ReleaseMatrixSource `json:"source,omitempty"`

	// ChartOverride replaces the whole chart artifact resolved from the matrix.
	// The supported way to run a custom or pre-release build.
	// +optional
	ChartOverride *ChartRef `json:"chartOverride,omitempty"`

	// ImageRegistry overrides the default registry for all component images.
	// Required for airgapped installs.
	// +optional
	ImageRegistry *RegistrySpec `json:"imageRegistry,omitempty"`
}

// ReleaseMatrixSource locates the release matrix. Exactly one field is expected;
// the webhook in milestone 006 enforces that.
type ReleaseMatrixSource struct {
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// OCIRef is an OCI artifact reference, e.g.
	// "oci://quay.io/platform9/pcd-release-matrix:2026.4".
	// +optional
	OCIRef string `json:"ociRef,omitempty"`

	// +optional
	URL string `json:"url,omitempty"`
}

// ChartRef identifies a chart artifact.
type ChartRef struct {
	// +optional
	OCIRef string `json:"ociRef,omitempty"`

	// +optional
	URL string `json:"url,omitempty"`

	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}

// RegistrySpec redirects component images at a different registry.
type RegistrySpec struct {
	Host string `json:"host"`

	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}
