package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// registerKinds queues a group of root objects for scheme registration. Each
// kind file calls it from its own init, so adding a kind touches one file.
func registerKinds(objects ...runtime.Object) {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, objects...)
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})
}

// PCDUnderlaySpec describes the substrate every PCDInstallation is laid on top
// of: the cluster itself and the prerequisites that must exist before any
// installation can be created.
//
// Everything in this kind is reconciled by applying Helm releases and manifests
// directly against the cluster with the operator's own client. The backend
// adapter is not involved at any point — see docs/SPEC.md §3 INV-2, which the
// import graph enforces.
type PCDUnderlaySpec struct {
	// Profile seeds defaults appropriate to the deployment context. Four very
	// different installations differ mostly in defaults, not in structure.
	// This is override layer 2.
	// +kubebuilder:validation:Enum=saas-aws;saas-oci;on-prem;community-edition
	Profile string `json:"profile"`

	// Release pins prerequisite software versions collectively.
	Release ReleaseSpec `json:"release"`

	// Platform describes the underlying cluster and cloud.
	Platform PlatformSpec `json:"platform"`

	// ExternalServices declares platform-level integrations, inherited as
	// defaults by every PCDInstallation and PCDRegion beneath this underlay.
	// +optional
	ExternalServices *ExternalServices `json:"externalServices,omitempty"`

	// Networking carries the hosted zone, ingress class, virtual IPs, load
	// balancer provider and proxy configuration.
	Networking InfraNetworkingSpec `json:"networking"`

	// ComponentDefaults applies to every prerequisite component. Override
	// layer 4.
	// +optional
	ComponentDefaults *ComponentSpec `json:"componentDefaults,omitempty"`

	// Components overrides individual prerequisites by name. Override layer 5.
	//
	// cert-manager cannot be disabled here: it is the PKI root for host
	// onboarding, not merely the wildcard TLS provider, so disabling it would
	// silently break host onboarding rather than trim a component. The webhook
	// in milestone 006 rejects it (docs/SPEC.md §3 INV-5).
	// +optional
	Components map[string]ComponentSpec `json:"components,omitempty"`

	// UpgradePolicy governs how prerequisite upgrades roll.
	// +optional
	UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`

	// Paused halts reconciliation without deleting or modifying anything
	// (CTRL-011). Essential for incident response on a live platform.
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// PlatformSpec describes the cluster the operator runs in.
//
// There is no cluster selection field here, and there will not be one: the
// operator and everything it manages share a single cluster (docs/SPEC.md §3
// INV-6, §10). Placement concepts belong inside the backend adapter and are
// deliberately absent from this API.
type PlatformSpec struct {
	// +kubebuilder:validation:Enum=aws-eks;oci-oke;azure-aks;gke;nodelet;k3s;generic
	Kind string `json:"kind"`

	// +optional
	Region string `json:"region,omitempty"`

	// KubernetesVersionConstraint is a semver range, e.g. ">=1.30.0".
	// +optional
	KubernetesVersionConstraint string `json:"kubernetesVersionConstraint,omitempty"`

	// +optional
	Airgapped bool `json:"airgapped,omitempty"`
}

// PCDUnderlayStatus is the observed state of a PCDUnderlay.
type PCDUnderlayStatus struct {
	CommonStatus `json:",inline"`
}

// PCDUnderlay is the cluster-scoped entry point. There is exactly one per
// cluster, which is why it is cluster-scoped rather than namespaced.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pcdunderlay
// +kubebuilder:printcolumn:name="Profile",type=string,JSONPath=`.spec.profile`
// +kubebuilder:printcolumn:name="Release",type=string,JSONPath=`.spec.release.matrixVersion`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyComponents`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredComponents`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type PCDUnderlay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCDUnderlaySpec   `json:"spec,omitempty"`
	Status PCDUnderlayStatus `json:"status,omitempty"`
}

// PCDUnderlayList contains a list of PCDUnderlay.
//
// +kubebuilder:object:root=true
type PCDUnderlayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PCDUnderlay `json:"items"`
}

func init() {
	registerKinds(&PCDUnderlay{}, &PCDUnderlayList{})
}
