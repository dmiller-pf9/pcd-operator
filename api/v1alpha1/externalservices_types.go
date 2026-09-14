package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalServices declares platform integrations. The same struct is
// embeddable at underlay, installation and region level; the nearest
// declaration wins, so a region can point at a different database than its
// installation default.
type ExternalServices struct {
	// +optional
	Database *DatabaseBinding `json:"database,omitempty"`

	// +optional
	Storage *StorageBinding `json:"storage,omitempty"`

	// +optional
	ObjectStore *ObjectStoreBinding `json:"objectStore,omitempty"`

	// +optional
	MessageQueue *MessageQueueBinding `json:"messageQueue,omitempty"`

	// +optional
	Secrets *SecretsBinding `json:"secrets,omitempty"`

	// +optional
	Certificates *CertificateBinding `json:"certificates,omitempty"`

	// +optional
	DNS *DNSBinding `json:"dns,omitempty"`

	// +optional
	Identity *IdentityBinding `json:"identity,omitempty"`

	// +optional
	Monitoring *MonitoringBinding `json:"monitoring,omitempty"`

	// +optional
	Logging *LoggingBinding `json:"logging,omitempty"`

	// +optional
	Backup *BackupBinding `json:"backup,omitempty"`
}

// CertificateBinding covers two distinct certificate roles. They are separated
// because they usually want different issuers: ingress TLS is typically a
// public ACME issuer, while host PKI must be a private CA.
type CertificateBinding struct {
	// +kubebuilder:validation:Enum=cert-manager;self-signed;provided
	Provider string `json:"provider"`

	// IngressIssuer issues the public-facing TLS certificates for endpoints.
	// +optional
	IngressIssuer *IssuerRef `json:"ingressIssuer,omitempty"`

	// HostPKIIssuer is the private CA that issues host agent certificates. It
	// is required for host onboarding, which is why cert-manager cannot be
	// disabled (docs/SPEC.md §3 INV-5).
	// +optional
	HostPKIIssuer *IssuerRef `json:"hostPKIIssuer,omitempty"`

	// WildcardSecretRef supplies a pre-issued wildcard certificate when
	// Provider is "provided".
	// +optional
	WildcardSecretRef *corev1.SecretReference `json:"wildcardSecretRef,omitempty"`

	// ReplicateToNamespaces mirrors the issued certificate into every
	// namespace that needs it, which is how per-domain issuance rate limits
	// are avoided.
	// +optional
	ReplicateToNamespaces bool `json:"replicateToNamespaces,omitempty"`
}

// StorageBinding selects the persistent volume provider.
type StorageBinding struct {
	// +kubebuilder:validation:Enum=hostpath;nfs;csi;cloud
	Provider string `json:"provider"`

	// StorageClassName is the class PCD workloads request.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// +optional
	NFS *NFSSpec `json:"nfs,omitempty"`

	// Parameters are passed to the generated StorageClass.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// +optional
	DefaultClass *bool `json:"defaultClass,omitempty"`
}

// NFSSpec configures an NFS-backed StorageClass.
type NFSSpec struct {
	Server string `json:"server"`
	Path   string `json:"path"`

	// +optional
	MountOptions []string `json:"mountOptions,omitempty"`
}

// ObjectStoreBinding configures S3-compatible object storage.
type ObjectStoreBinding struct {
	// +kubebuilder:validation:Enum=s3;minio;oci-objectstore
	Provider string `json:"provider"`

	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// +optional
	Region string `json:"region,omitempty"`

	Bucket string `json:"bucket"`

	// +optional
	Prefix string `json:"prefix,omitempty"`

	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// +optional
	TLS *TLSSettings `json:"tls,omitempty"`
}

// MessageQueueBinding configures the AMQP broker.
type MessageQueueBinding struct {
	// +kubebuilder:validation:Enum=Dedicated;Shared
	// +kubebuilder:default=Dedicated
	Provisioning string `json:"provisioning"`

	// +optional
	InstanceRef *corev1.LocalObjectReference `json:"instanceRef,omitempty"`

	// Replicas defaults from Size; the broker is quorum-bound and does not
	// autoscale.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// +optional
	Storage *StorageOverride `json:"storage,omitempty"`

	// +optional
	TLS *TLSSettings `json:"tls,omitempty"`
}

// SecretsBinding selects how credentials are stored. "kubernetes" stores them
// as plain Secrets; "external-secrets" syncs them from an upstream store.
type SecretsBinding struct {
	// +kubebuilder:validation:Enum=kubernetes;external-secrets
	// +kubebuilder:default=kubernetes
	Provider string `json:"provider"`

	// +optional
	ExternalSecrets *ExternalSecretsSpec `json:"externalSecrets,omitempty"`
}

// ExternalSecretsSpec configures the external-secrets provider.
type ExternalSecretsSpec struct {
	// SecretStoreRef names a SecretStore or ClusterSecretStore.
	SecretStoreRef corev1.TypedLocalObjectReference `json:"secretStoreRef"`

	// +optional
	RefreshInterval *metav1.Duration `json:"refreshInterval,omitempty"`
}

// DNSBinding configures record management for deployment FQDNs.
type DNSBinding struct {
	// +kubebuilder:validation:Enum=route53;oci-dns;cloudflare;coredns;none
	Provider string `json:"provider"`

	// +optional
	HostedZoneID string `json:"hostedZoneID,omitempty"`

	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`

	// +optional
	RecordTTL *int64 `json:"recordTTL,omitempty"`
}

// IdentityBinding configures the identity provider.
type IdentityBinding struct {
	// +kubebuilder:validation:Enum=local;saml;oidc
	// +kubebuilder:default=local
	Provider string `json:"provider"`

	// IDPMetadataSecretRef is required when Provider is saml.
	// +optional
	IDPMetadataSecretRef *corev1.LocalObjectReference `json:"idpMetadataSecretRef,omitempty"`

	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`

	// DefaultRole is the role assigned to users with no mapping.
	// +optional
	DefaultRole string `json:"defaultRole,omitempty"`

	// AttributeMapping maps identity-provider assertion attributes to identity
	// service attributes.
	// +optional
	AttributeMapping map[string]string `json:"attributeMapping,omitempty"`
}

// OIDCSpec configures an OIDC identity provider.
type OIDCSpec struct {
	IssuerURL string `json:"issuerURL"`
	ClientID  string `json:"clientID"`

	ClientSecretRef *corev1.LocalObjectReference `json:"clientSecretRef"`

	// +optional
	Scopes []string `json:"scopes,omitempty"`
}

// MonitoringBinding configures metrics collection and shipping.
type MonitoringBinding struct {
	// +kubebuilder:validation:Enum=in-cluster;remote-write;both
	// +kubebuilder:default=in-cluster
	Mode string `json:"mode"`

	// +optional
	RemoteWrite []RemoteWriteTarget `json:"remoteWrite,omitempty"`

	// +optional
	Retention *metav1.Duration `json:"retention,omitempty"`

	// +optional
	Storage *StorageOverride `json:"storage,omitempty"`
}

// RemoteWriteTarget is one Prometheus remote-write destination.
type RemoteWriteTarget struct {
	URL string `json:"url"`

	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`

	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// LoggingBinding configures log shipping.
type LoggingBinding struct {
	// +kubebuilder:validation:Enum=none;fluent-bit
	// +kubebuilder:default=fluent-bit
	Provider string `json:"provider"`

	// Outputs is passed through to the log shipper's output configuration
	// unmodified — the shipper owns that schema, not this API.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Outputs *apiextensionsv1.JSON `json:"outputs,omitempty"`

	// +optional
	Retention *metav1.Duration `json:"retention,omitempty"`
}

// BackupBinding covers management-plane backup. Database backups are
// DatabaseBinding.Backup, not this.
type BackupBinding struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	Schedule string `json:"schedule,omitempty"`

	// +optional
	Retention *metav1.Duration `json:"retention,omitempty"`

	// +optional
	Destination *ObjectStoreBinding `json:"destination,omitempty"`
}
