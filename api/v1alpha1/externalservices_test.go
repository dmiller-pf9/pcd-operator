package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestExternalServicesJSONTags(t *testing.T) {
	es := ExternalServices{
		Database:     &DatabaseBinding{Provisioning: "Dedicated"},
		Storage:      &StorageBinding{Provider: "csi"},
		ObjectStore:  &ObjectStoreBinding{Provider: "s3", Bucket: "pcd", CredentialsSecretRef: &corev1.LocalObjectReference{Name: "s3"}},
		MessageQueue: &MessageQueueBinding{Provisioning: "Dedicated"},
		Secrets:      &SecretsBinding{Provider: "kubernetes"},
		Certificates: &CertificateBinding{Provider: "cert-manager"},
		DNS:          &DNSBinding{Provider: "route53"},
		Identity:     &IdentityBinding{Provider: "local"},
		Monitoring:   &MonitoringBinding{Mode: "in-cluster"},
		Logging:      &LoggingBinding{Provider: "fluent-bit"},
		Backup:       &BackupBinding{},
	}

	assertJSONKeys(t, es,
		"database", "storage", "objectStore", "messageQueue", "secrets",
		"certificates", "dns", "identity", "monitoring", "logging", "backup",
	)
}

// DESIGN §6: cert-manager is the PKI root for host onboarding as well as the
// wildcard TLS provider, so the two issuer roles are separate fields — they
// usually want different issuers.
func TestCertificateBindingSplitsIssuerRoles(t *testing.T) {
	cb := CertificateBinding{
		Provider:      "cert-manager",
		IngressIssuer: &IssuerRef{Kind: "ClusterIssuer", Name: "letsencrypt"},
		HostPKIIssuer: &IssuerRef{Kind: "Issuer", Name: "host-ca"},
	}
	assertJSONKeys(t, cb, "provider", "ingressIssuer", "hostPKIIssuer")
}
