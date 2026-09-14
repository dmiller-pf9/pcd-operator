package v1alpha1

import "testing"

func TestInfraNetworkingSpecJSONTags(t *testing.T) {
	n := InfraNetworkingSpec{
		HostedZone:   "pcd.example.invalid",
		IngressClass: "nginx",
		LoadBalancer: &LoadBalancerSpec{Provider: "metallb", AddressPool: []string{"10.0.0.0/24"}},
		VirtualIPs:   &VirtualIPSpec{ManagementCluster: "10.0.0.1", DeploymentUnit: "10.0.0.2"},
		Proxy:        &ProxySpec{HTTPProxy: "http://proxy.invalid:3128"},
	}
	assertJSONKeys(t, n, "hostedZone", "ingressClass", "loadBalancer", "virtualIPs", "proxy")
}

func TestRegionNetworkingSpecJSONTags(t *testing.T) {
	mtu := int32(9000)
	replicas := int32(3)
	segID := int32(101)
	n := RegionNetworkingSpec{
		OVN:              &OVNSpec{NorthboundDBReplicas: &replicas},
		ProviderNetworks: []ProviderNetwork{{Name: "physnet1", Type: "vlan", SegmentationID: &segID}},
		MTU:              &mtu,
		FloatingIPPools:  []string{"10.1.0.0/24"},
		MetadataService:  &MetadataServiceSpec{},
	}
	assertJSONKeys(t, n, "ovn", "providerNetworks", "mtu", "floatingIPPools", "metadataService")
}
