package v1alpha1

// InfraNetworkingSpec describes cluster-level networking for the underlay.
type InfraNetworkingSpec struct {
	// HostedZone is the DNS suffix every deployment FQDN is built under.
	HostedZone string `json:"hostedZone"`

	// +kubebuilder:default="nginx"
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`

	// +optional
	LoadBalancer *LoadBalancerSpec `json:"loadBalancer,omitempty"`

	// +optional
	VirtualIPs *VirtualIPSpec `json:"virtualIPs,omitempty"`

	// +optional
	Proxy *ProxySpec `json:"proxy,omitempty"`
}

// LoadBalancerSpec selects how services are exposed.
type LoadBalancerSpec struct {
	// +kubebuilder:validation:Enum=aws-nlb;oci-lb;metallb;none
	Provider string `json:"provider"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// AddressPool is required when Provider is metallb.
	// +optional
	AddressPool []string `json:"addressPool,omitempty"`
}

// VirtualIPSpec pins the virtual IPs used by the cluster and the deployment.
type VirtualIPSpec struct {
	// +optional
	ManagementCluster string `json:"managementCluster,omitempty"`

	// +optional
	DeploymentUnit string `json:"deploymentUnit,omitempty"`
}

// ProxySpec configures egress through an HTTP proxy.
type ProxySpec struct {
	// +optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// +optional
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// +optional
	NoProxy string `json:"noProxy,omitempty"`
}

// RegionNetworkingSpec describes a region's data-plane networking.
type RegionNetworkingSpec struct {
	// +optional
	OVN *OVNSpec `json:"ovn,omitempty"`

	// +optional
	ProviderNetworks []ProviderNetwork `json:"providerNetworks,omitempty"`

	// MTU is applied to both neutron global_physnet_mtu and ml2 path_mtu.
	// +optional
	MTU *int32 `json:"mtu,omitempty"`

	// +optional
	FloatingIPPools []string `json:"floatingIPPools,omitempty"`

	// +optional
	MetadataService *MetadataServiceSpec `json:"metadataService,omitempty"`
}

// OVNSpec sizes the OVN control plane. Replica counts default from Size
// (docs/DESIGN.md §4.5); OVN cannot autoscale.
type OVNSpec struct {
	// +optional
	NorthboundDBReplicas *int32 `json:"northboundDBReplicas,omitempty"`

	// +optional
	SouthboundDBReplicas *int32 `json:"southboundDBReplicas,omitempty"`

	// +optional
	RelayReplicas *int32 `json:"relayReplicas,omitempty"`
}

// ProviderNetwork declares a neutron provider network.
type ProviderNetwork struct {
	Name string `json:"name"`

	// +kubebuilder:validation:Enum=flat;vlan;vxlan;geneve
	Type string `json:"type"`

	// +optional
	PhysicalNetwork string `json:"physicalNetwork,omitempty"`

	// +optional
	SegmentationID *int32 `json:"segmentationID,omitempty"`

	// +optional
	Shared *bool `json:"shared,omitempty"`
}

// MetadataServiceSpec configures the instance metadata service.
type MetadataServiceSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	Port *int32 `json:"port,omitempty"`
}
