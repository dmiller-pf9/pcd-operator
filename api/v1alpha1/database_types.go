package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseBinding wraps the MariaDB Kubernetes operator
// (k8s.mariadb.com/v1alpha1). There is exactly one database technology across
// every PCD flavor, so this type is a thin adapter over that operator's API
// rather than an abstraction over several backends. The typed fields below
// exist only because Size drives them; everything else reaches the MariaDB CR
// through Template.
type DatabaseBinding struct {
	// Provisioning decides whether this scope gets its own MariaDB instance or
	// attaches to one provisioned at a higher scope. A PCDRegion set to Shared
	// with no InstanceRef resolves to its PCDInstallation's instance.
	// +kubebuilder:validation:Enum=Dedicated;Shared
	// +kubebuilder:default=Dedicated
	Provisioning string `json:"provisioning"`

	// InstanceRef names an existing MariaDB object. Required when Provisioning
	// is Shared and the target is not the parent scope's instance. This is the
	// only supported way to point PCD at a database the operator did not create
	// — there is no free-form host/port/credentials escape hatch.
	// +optional
	InstanceRef *MariaDBRef `json:"instanceRef,omitempty"`

	// Topology maps to the MariaDB CR's HA stanza: Standalone leaves both
	// unset, Replication sets spec.replication, Galera sets spec.galera.
	// +kubebuilder:validation:Enum=Standalone;Replication;Galera
	// +optional
	Topology string `json:"topology,omitempty"`

	// Replicas overrides the node count implied by Size. Galera requires an odd
	// count of at least 3; the webhook rejects even counts under Galera rather
	// than letting the MariaDB operator discover it later.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Storage maps to MariaDB spec.storage.
	// +optional
	Storage *MariaDBStorage `json:"storage,omitempty"`

	// MyCnf is server configuration, merged into MariaDB spec.myCnf. This is
	// where Size-driven tuning lands — buffer pool, max_connections, and the
	// rest.
	// +optional
	MyCnf map[string]map[string]string `json:"myCnf,omitempty"`

	// Metrics enables the operator's built-in exporter (MariaDB spec.metrics).
	// +optional
	Metrics *MariaDBMetrics `json:"metrics,omitempty"`

	// MaxScale enables a MaxScale object in front of the instance for
	// connection routing and failover. Standalone and small sizes skip it.
	// +optional
	MaxScale *MaxScaleSpec `json:"maxScale,omitempty"`

	// Schema controls whether the operator emits Database/User/Grant objects
	// for each PCD service.
	// +optional
	Schema *SchemaPolicy `json:"schema,omitempty"`

	// Backup maps to Backup / PhysicalBackup objects and their schedule.
	// +optional
	Backup *DatabaseBackupPolicy `json:"backup,omitempty"`

	// BootstrapFrom maps to MariaDB spec.bootstrapFrom, restoring a new
	// instance from a backup or S3 source.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	BootstrapFrom *apiextensionsv1.JSON `json:"bootstrapFrom,omitempty"`

	// Template is deep-merged into the generated MariaDB object's spec, last,
	// after every field above. It is the full k8s.mariadb.com/v1alpha1 MariaDB
	// API surface.
	//
	// Deliberately unvalidated here (docs/SPEC.md §3 INV-8): the MariaDB
	// operator's own webhook is the authority on its schema, and duplicating
	// that validation would guarantee drift between versions. The cost is that
	// a malformed Template fails upstream, so DB-007 requires the upstream
	// error be surfaced verbatim in the component's Faulted message.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Template *apiextensionsv1.JSON `json:"template,omitempty"`
}

// MariaDBRef names an existing MariaDB object.
type MariaDBRef struct {
	Name string `json:"name"`

	// +optional
	Namespace string `json:"namespace,omitempty"`

	// WaitForIt mirrors the MariaDB operator's own mariaDbRef.waitForIt
	// semantics.
	// +kubebuilder:default=true
	// +optional
	WaitForIt *bool `json:"waitForIt,omitempty"`
}

// SchemaPolicy controls database, user and grant management.
type SchemaPolicy struct {
	// Manage=true has the operator emit a Database, User and Grant object per
	// PCD service rather than leaving CREATE DATABASE / CREATE USER / GRANT to
	// each service's init job. Those objects are declarative and idempotent by
	// construction, which an init job is only if someone wrote it that way.
	// +kubebuilder:default=true
	// +optional
	Manage *bool `json:"manage,omitempty"`

	// +kubebuilder:default="utf8mb4"
	// +optional
	Charset string `json:"charset,omitempty"`

	// +optional
	Collate string `json:"collate,omitempty"`

	// MaxUserConnections per service user. Size-driven; a shared instance with
	// no per-user cap is how one service starves the rest.
	// +optional
	MaxUserConnections *int32 `json:"maxUserConnections,omitempty"`
}

// MariaDBStorage maps to MariaDB spec.storage.
type MariaDBStorage struct {
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`

	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// ResizeInUseVolumes requires a StorageClass with
	// allowVolumeExpansion=true. Preflight asserts this before dispatching a
	// Size increase.
	// +optional
	ResizeInUseVolumes *bool `json:"resizeInUseVolumes,omitempty"`

	// +optional
	WaitForVolumeResize *bool `json:"waitForVolumeResize,omitempty"`

	// Ephemeral provisions without a PVC. Community edition and test only; the
	// webhook rejects it on any deployment whose profile is not
	// community-edition (DB-006).
	// +optional
	Ephemeral *bool `json:"ephemeral,omitempty"`
}

// MariaDBMetrics enables the MariaDB operator's built-in exporter, replacing
// the separately-deployed exporter component.
type MariaDBMetrics struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ServiceMonitor emits a Prometheus ServiceMonitor alongside the exporter.
	// +optional
	ServiceMonitor *bool `json:"serviceMonitor,omitempty"`

	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`
}

// MaxScaleSpec fronts the database with MaxScale for connection routing and
// failover.
type MaxScaleSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Template is merged into the generated MaxScale object's spec, with the
	// same passthrough contract as DatabaseBinding.Template — unvalidated here
	// by design (docs/SPEC.md §3 INV-8).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Template *apiextensionsv1.JSON `json:"template,omitempty"`
}

// DatabaseBackupPolicy schedules database backups.
type DatabaseBackupPolicy struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Logical emits Backup objects; Physical emits PhysicalBackup objects.
	// +kubebuilder:validation:Enum=Logical;Physical
	// +kubebuilder:default=Physical
	Type string `json:"type"`

	// Schedule is a cron expression. Omit for on-demand only.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// +optional
	Retention *metav1.Duration `json:"retention,omitempty"`

	// Storage is passed through to the generated object's spec.storage
	// (s3 / persistentVolumeClaim / volume), unmodified.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Storage *apiextensionsv1.JSON `json:"storage,omitempty"`
}
