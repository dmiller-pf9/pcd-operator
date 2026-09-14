package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PCDInstallationSpec is one PCD installation: a customer plus its infra
// region, the region that runs identity and auth dependencies only.
//
// This kind deliberately does two jobs — it is both the tenancy record and the
// infra region's deployment spec. That is settled (docs/SPEC.md §10): the two
// are 1:1, there is no known requirement for an installation with zero or two
// infra regions, and splitting them would add a fourth object to model a
// relationship with no degrees of freedom.
type PCDInstallationSpec struct {
	// UnderlayRef binds to the cluster-scoped underlay. Defaults to the single
	// underlay if exactly one exists.
	// +optional
	UnderlayRef *corev1.LocalObjectReference `json:"underlayRef,omitempty"`

	// ShortName is the customer identifier. It determines namespace names and
	// FQDNs, so renaming would orphan every resource created under the old
	// name.
	//
	// Immutable after creation (API-004). Enforced by the webhook in milestone
	// 006, not here.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=48
	ShortName string `json:"shortName"`

	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// AdminEmail seeds the initial identity-service admin.
	AdminEmail string `json:"adminEmail"`

	// AdminCredentialsRef holds the initial admin password. If unset, the
	// operator generates one and writes it here.
	// +optional
	AdminCredentialsRef *corev1.LocalObjectReference `json:"adminCredentialsRef,omitempty"`

	// FQDN of the infra region. Defaults to "<shortName>.<hostedZone>".
	// +optional
	FQDN string `json:"fqdn,omitempty"`

	// Release for the infra region. Defaults to the underlay's release. Carried
	// separately so one installation can be pinned behind the fleet during a
	// staged rollout.
	// +optional
	Release *ReleaseSpec `json:"release,omitempty"`

	// +optional
	ExternalServices *ExternalServices `json:"externalServices,omitempty"`

	// SSO configuration for this installation. Same type as
	// ExternalServices.Identity — an installation-scoped declaration overrides
	// the underlay default rather than sitting beside it.
	// +optional
	SSO *IdentityBinding `json:"sso,omitempty"`

	// ComponentDefaults applies to every component in this installation.
	// Override layer 4.
	// +optional
	ComponentDefaults *ComponentSpec `json:"componentDefaults,omitempty"`

	// Components overrides individual components by name. Override layer 5.
	// +optional
	Components map[string]ComponentSpec `json:"components,omitempty"`

	// Size selects the deployment scale preset: fixed resource footprints and
	// replica counts for components that cannot autoscale, tuned autoscaling
	// bounds for those that can. Override layer 3.
	//
	// Unset means the profile's default size. The empty string is deliberately
	// not one of the valid values, so "unset" stays distinguishable — a
	// PCDRegion inherits this value when its own is unset (API-021).
	// +optional
	Size PCDSize `json:"size,omitempty"`

	// Protection prevents accidental deletion, enforced at the admission
	// webhook so the block happens before any teardown work is dispatched
	// (CTRL-010).
	// +optional
	Protection *ProtectionSpec `json:"protection,omitempty"`

	// +optional
	UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`

	// Paused halts reconciliation without deleting or modifying anything
	// (CTRL-011).
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// PCDInstallationStatus is the observed state of a PCDInstallation.
type PCDInstallationStatus struct {
	CommonStatus `json:",inline"`
}

// PCDInstallation is a customer and its infra region.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=pcdinstall
// +kubebuilder:printcolumn:name="Short Name",type=string,JSONPath=`.spec.shortName`
// +kubebuilder:printcolumn:name="FQDN",type=string,JSONPath=`.spec.fqdn`
// +kubebuilder:printcolumn:name="Size",type=string,JSONPath=`.spec.size`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyComponents`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredComponents`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type PCDInstallation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCDInstallationSpec   `json:"spec,omitempty"`
	Status PCDInstallationStatus `json:"status,omitempty"`
}

// PCDInstallationList contains a list of PCDInstallation.
//
// +kubebuilder:object:root=true
type PCDInstallationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PCDInstallation `json:"items"`
}

func init() {
	registerKinds(&PCDInstallation{}, &PCDInstallationList{})
}
