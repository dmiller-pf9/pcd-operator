package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PCDRegionSpec is a workload region attached to a PCDInstallation, running the
// OpenStack API service suite.
type PCDRegionSpec struct {
	// InstallationRef names the owning PCDInstallation, which must be in the
	// same namespace.
	//
	// Immutable after creation (API-004). Enforced by the webhook in milestone
	// 006, not here.
	// +kubebuilder:validation:Required
	InstallationRef corev1.LocalObjectReference `json:"installationRef"`

	// RegionName is the region as OpenStack sees it, e.g. "Region-One".
	//
	// Immutable after creation (API-004). Enforced by the webhook in milestone
	// 006, not here.
	// +kubebuilder:validation:Required
	RegionName string `json:"regionName"`

	// RegionInstance is the FQDN slug:
	// "<shortName>-<regionInstance>.<hostedZone>".
	// +optional
	RegionInstance string `json:"regionInstance,omitempty"`

	// +optional
	FQDN string `json:"fqdn,omitempty"`

	// Release defaults to the installation's release. A region may be pinned
	// behind its installation during a staged upgrade, but a region release
	// *ahead* of its installation is rejected — the infra region must upgrade
	// first (CTRL-012).
	// +optional
	Release *ReleaseSpec `json:"release,omitempty"`

	// +optional
	ExternalServices *ExternalServices `json:"externalServices,omitempty"`

	// Networking configures the region's data plane: OVN, provider networks,
	// MTU, metadata service and floating IP pools.
	// +optional
	Networking *RegionNetworkingSpec `json:"networking,omitempty"`

	// Features toggles optional capability sets as a unit. Sugar over
	// components{} for the common minimization and feature-gating cases.
	// +optional
	Features *RegionFeatures `json:"features,omitempty"`

	// NOTE: host-side lifecycle (node preparation, host agent packages, host
	// upgrades) is intentionally absent. It lands in a future PCDHostPool kind.
	// Do not add a hostManagement stanza here in the interim — a half-modelled
	// field is harder to remove than to add (docs/SPEC.md §10).

	// Size selects the deployment scale preset for this region. Inherits the
	// parent PCDInstallation's size when unset (API-021). A region may
	// legitimately differ from its installation — identity load does not track
	// hypervisor count.
	// +optional
	Size PCDSize `json:"size,omitempty"`

	// ComponentDefaults applies to every component in this region. Override
	// layer 4.
	// +optional
	ComponentDefaults *ComponentSpec `json:"componentDefaults,omitempty"`

	// Components overrides individual components by name. Override layer 5.
	// +optional
	Components map[string]ComponentSpec `json:"components,omitempty"`

	// +optional
	Protection *ProtectionSpec `json:"protection,omitempty"`

	// +optional
	UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`

	// Paused halts reconciliation without deleting or modifying anything
	// (CTRL-011).
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// RegionFeatures toggles optional capability sets as a unit. Each field gates a
// group of components; the mapping from feature to components is catalog data,
// not code.
type RegionFeatures struct {
	// Orchestration gates the heat services.
	// +optional
	Orchestration *bool `json:"orchestration,omitempty"`

	// IaC gates the infrastructure-as-code services.
	// +optional
	IaC *bool `json:"iac,omitempty"`

	// LoadBalancing gates octavia.
	// +optional
	LoadBalancing *bool `json:"loadBalancing,omitempty"`

	// DNSaaS gates designate.
	// +optional
	DNSaaS *bool `json:"dnsaas,omitempty"`

	// KeyManagement gates barbican.
	// +optional
	KeyManagement *bool `json:"keyManagement,omitempty"`

	// Optimization gates watcher.
	// +optional
	Optimization *bool `json:"optimization,omitempty"`

	// HighAvailability gates the instance HA services.
	// +optional
	HighAvailability *bool `json:"highAvailability,omitempty"`

	// +optional
	AppCatalog *bool `json:"appCatalog,omitempty"`

	// +optional
	Audit *bool `json:"audit,omitempty"`

	// Kubernetes gates the Kubernetes management plane in this region.
	// +optional
	Kubernetes *bool `json:"kubernetes,omitempty"`
}

// PCDRegionStatus is the observed state of a PCDRegion.
type PCDRegionStatus struct {
	CommonStatus `json:",inline"`
}

// PCDRegion is a workload region.
//
// A PCDRegion is Ready only when every enabled component is Ready. Any
// component in Quarantined makes the region Degraded rather than Error, because
// the rest of the region is still serving (STATE-009).
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=pcdregion
// +kubebuilder:printcolumn:name="Installation",type=string,JSONPath=`.spec.installationRef.name`
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.regionName`
// +kubebuilder:printcolumn:name="Size",type=string,JSONPath=`.spec.size`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyComponents`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredComponents`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type PCDRegion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCDRegionSpec   `json:"spec,omitempty"`
	Status PCDRegionStatus `json:"status,omitempty"`
}

// PCDRegionList contains a list of PCDRegion.
//
// +kubebuilder:object:root=true
type PCDRegionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PCDRegion `json:"items"`
}

func init() {
	registerKinds(&PCDRegion{}, &PCDRegionList{})
}
