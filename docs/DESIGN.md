# PCD Unified Installer Operator — API Kind Skeleton

**Status:** draft skeleton for review
**Scope:** `PCDUnderlay`, `PCDInstallation`, `PCDRegion`

---

## 1. What this is grounded in

Sources consulted before drafting:

| Source | What it contributed |
|---|---|
| *PCD Unification and Cloud-Agnostic Enablement Roadmap* (OP space) | Strategic intent: CRD-based abstractions for external integrations, one Helm artifact for SaaS/on-prem/CE, Bork as a first-class operator reacting to "install" CRDs. Your design is the concrete instantiation of Phases 1–4. |
| *Bork3 Operations & Architecture Guide* (OP space) | The backend contract this operator drives: the `x-<shortname>` / `<shortname>` namespace pair, ~77 secrets per region, task states (`waiting_apps`/`ready`/`deleting`/`error`), the async job pattern (`du-install-*`, `du-upgrade-*`, `du-teardown-*`), the `BORK_USE_INCLUSTER` topology switch, and the customer→infra-region→secondary-region ordering constraint. Also the source of `aims.bork.pf9.io` / `targetclusters.bork.pf9.io`, which stay internal to Bork and are **not** surfaced in this API. |
| *Kubernetes DU Architecture and Management Overview* (OP space) | Deploy/upgrade/delete flows, `init-region` job sequencing, deccaxon bootstrap, k8s-helm-runner. |
| *Getting Started with PCD*, *OnPrem PCD Steps* (PCD space) | The `create-customer` → `create-kdu-region` → `deploy-kdu-region` shape, airctl config surface (master IPs, VIPs, FQDN, storage provider, regions list), infra-region-holds-only-Keystone model. |
| *PCD on prem installation using NFS* (SRE1 space) | Full running-pod inventory for a real on-prem deployment — used to build the component catalogs in §6–§8. |
| *PCD Releases* (eng space) | The release matrix shape: a named release (e.g. `2026.4`) maps to a SAAS helm chart, airctl artifact, KAAPI chart, pcdctl, Serenity UI tag. This is what `releaseMatrix` resolves against. |
| *Community Edition Resource Minimization Plan* (PCD space) | `skip_components` precedent — components must be individually disableable, not just version-pinnable. |

### Settled decisions

These are decided, not open, and the API below reflects them:

1. **The operator wraps Bork and hides its API completely.** Bork is an internal implementation detail. No CRD field, status field, event, or error message exposes a Bork endpoint, verb, token, `x-` namespace, or task-state string as such. Consumers of these CRDs must be able to remain ignorant that Bork exists.
2. **Single-cluster only.** There are no multi-cluster deployment modes. `targetCluster` and `Aim` are removed from the API; the Bork adapter always calls in-cluster (`BORK_USE_INCLUSTER=true`) and supplies whatever Aim value the backend still requires internally.
3. **`PCDInstallation` and the Infra Region stay 1:1.** No separate infra-region kind. `PCDInstallation` remains both the tenancy record and the infra region's deployment spec.
4. **Bork3 is a hard prerequisite.** The adapter targets the noconsul backend only. Bork1/Bork2 and their Consul dependency are out of scope. This keeps the adapter single-path and lets Consul and Vault be treated as absent rather than optional throughout the design.
5. **Host-side lifecycle gets its own kind later.** A `PCDHostPool` kind is desired for `pcdctl prep-node`, host agent packages, and `UPGRADEHOSTS`, but is out of scope for the initial implementation. See §11.

---

## 2. API surface

### 2.1 Bork as a hidden backend

Bork is the backend for **installation and region lifecycle only**. Infrastructure prerequisites are not Bork's concern and never pass through it: the `PCDUnderlay` controller applies them directly, as Helm releases or plain manifests, using the operator's own cluster client. Bork has no notion of cert-manager or ingress and should not acquire one.

That gives the operator two distinct execution paths:

| Path | Driven by | Mechanism |
|---|---|---|
| Prerequisites | `PCDUnderlay` | Direct Helm/manifest apply against the cluster |
| Installation and region lifecycle | `PCDInstallation`, `PCDRegion` | Bork adapter → Bork3 |

The prerequisite path is the simpler of the two and worth keeping that way. It is ordinary Kubernetes reconciliation with no async job indirection, which is why `PrerequisitesMet` can be a synchronous, trustworthy gate on everything downstream.

For the Bork path, the operator is the only client. Everything Bork exposes is translated at an adapter boundary, and nothing leaks upward.

```
        ┌─────────────────────────────────────────────────────┐
        │  install.pcd.platform9.com/v1alpha1 CRDs             │  ← the only supported interface
        └──────────────┬────────────────────────┬─────────────┘
                       │                        │
            PCDUnderlay│                        │PCDInstallation / PCDRegion
                       │                        │
        ┌──────────────▼─────────────┐  ┌───────▼─────────────────────┐
        │  Chart / manifest applier  │  │  Bork adapter               │
        │  operator's cluster client │  │  (pkg/backend/bork)         │
        └──────────────┬─────────────┘  │  refactored in place later  │
                       │                └───────┬─────────────────────┘
                       │                        │
              Helm releases and         Bork3 HTTP API (in-cluster)
              manifests applied
              directly to the cluster
```

What "hidden" means concretely:

| Bork concept | How it surfaces in the CRD API |
|---|---|
| `POST /api/v1/customers`, `POST/DEPLOY/UPGRADE/BURN/DELETE /api/v1/regions/<fqdn>` | Not surfaced. Driven by spec/generation deltas. |
| `task_state` (`waiting_apps`, `ready`, `deleting`, `error`) | Translated to `status.phase` (`Installing`, `Ready`, `Deleting`, `Error`) and conditions. The raw string never appears. |
| `x-<shortname>` namespace, `region`/`customer`/`region-metadata` secrets | Not referenced. Managed entirely by the adapter. |
| `du-install-*` / `du-upgrade-*` / `du-teardown-*` Jobs | Abstracted behind `status.activeOperation`, which reports a neutral operation type and a log hint. |
| `options.chart_url` | `spec.release.chartOverride`. |
| `skip_components` | `spec.components[name].enabled: false`. |
| `metadata.dont_delete` | `spec.protection.preventDeletion`. |
| Bearer token / `admin-tokens` secret | Adapter-internal. Never a CRD field. |
| `aim`, target cluster / kubeconfig selection | Removed. Always in-cluster. |

The adapter targets **Bork3 (noconsul) only**. Bork1/2 are not supported: their Consul dependency would force the adapter to carry two state models. The `PCDUnderlay` controller enforces this as a preflight check — if the backend does not present a Bork3-compatible surface, `PrerequisitesMet` goes `False` with an explicit "requires Bork3" reason and nothing downstream reconciles. Failing loudly up front is much better than discovering it halfway through an installation.

Three consequences worth building in from day one:

- **The adapter interface is the seam for refactoring Bork.** Roadmap Phase 3 wants native reconciliation, and the route there is refactoring Bork's logic behind this interface rather than reimplementing it alongside. If every backend call goes through one Go interface, that refactor is contained to a package and never reaches the CRD API. Design the interface around *intent* (`EnsureInstallation`, `EnsureRegion`, `Teardown`) rather than around Bork's verbs, or it will calcify into a Bork-shaped hole.
- **Single-path is the point.** Because there is exactly one supported backend generation, the adapter should not grow capability detection, feature flags, or conditional branches for older behaviour. If Bork1/2 support is ever needed it belongs in a second adapter implementation behind the same interface, not in `if` statements inside this one.
- **Bork's async pattern must not leak as eventual weirdness.** A Bork `200` means "submitted," not "done," and the monitor goroutine dies on pod restart — a region can sit in `deleting` forever. The controller has to own reconvergence itself: re-issue on restart, treat submission as idempotent, and drive `status.phase` from observed cluster state rather than from the last Bork response.

### 2.2 Group, versions, naming

```
Group:   install.pcd.platform9.com
Version: v1alpha1
Scope:   PCDUnderlay     — Cluster-scoped
         PCDInstallation — Namespaced (operator namespace)
         PCDRegion       — Namespaced (same namespace as its PCDInstallation)
```

Short names: `pcdunderlay`, `pcdinstall`, `pcdregion`.

Rationale for scoping: PCD management runs on-cluster — the operator and every component it deploys share the cluster with the PCD infrastructure they manage. The underlay describes that whole cluster and there is exactly one per cluster, so cluster-scoped avoids ambiguity. Customers and regions are multi-tenant records that benefit from namespace-level RBAC — on the multi-customer AWS/OCI SaaS platforms you want per-customer RBAC boundaries to be at least *possible*.

---

## 3. Object model

```
PCDUnderlay  (cluster-scoped, one per cluster)
  │  prerequisites: cert-manager, ingress, secrets mgmt, storage, DNS, CNI addons, kaapi
  │  declares: platform profile, external service bindings, release matrix
  │
  ├── PCDInstallation  "acme"
  │     │  owns the Infra Region (Keystone + identity/auth dependencies)
  │     │  FQDN: acme.<hostedZone>
  │     │
  │     ├── PCDRegion  "acme-region-one"    FQDN: acme-region-one.<hostedZone>
  │     └── PCDRegion  "acme-region-two"
  │
  └── PCDInstallation  "globex"
        └── PCDRegion  "globex-us-west"
```

Ownership is enforced with owner references and finalizers so that teardown ordering matches the backend's hard constraint: **secondary regions must be deleted before the infra region**, because the infra region holds customer-level credential state that secondary regions read while building their own deployment configuration. Today that ordering is a documented runbook step; here it becomes structural.

---

## 4. The override engine (shared types)

This is where design principle #2 lives. Everything below is reused verbatim by all three kinds, so there is exactly one mental model for "how do I override a thing."

### 4.1 Precedence

Lowest to highest. Later layers win on a per-key basis (strategic merge for structured fields, last-write-wins for scalars):

```
1. Release matrix defaults        spec.release.matrixVersion
2. Deployment profile defaults    spec.profile  (saas-aws | saas-oci | on-prem | community-edition)
3. Size preset                    spec.size     (see §4.5)
4. Global component defaults      spec.componentDefaults
5. Per-component override         spec.components[name]
6. Inline emergency patch         spec.components[name].strategicMergePatches
```

The rendered result is written to `status.renderedValuesRef` (a ConfigMap) on every reconcile so operators can diff what actually got applied without reading Helm.

### 4.2 Types

```go
// ReleaseSpec pins the collective release and allows per-component escape hatches.
type ReleaseSpec struct {
    // MatrixVersion is the collective PCD release, e.g. "2026.4", "2026.4-patch2",
    // "2026.8". Resolved via ReleaseMatrixSource into a concrete artifact set.
    // +kubebuilder:validation:Required
    MatrixVersion string `json:"matrixVersion"`

    // Source of the release matrix. Defaults to the operator's embedded matrix for
    // the version it shipped with; override for airgapped or custom-build cases
    // (e.g. the 2026.4-chitale custom customer build).
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

type ReleaseMatrixSource struct {
    // +optional
    ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`
    // +optional
    OCIRef string `json:"ociRef,omitempty"`   // oci://quay.io/platform9/pcd-release-matrix:2026.4
    // +optional
    URL string `json:"url,omitempty"`
}

type ChartRef struct {
    // +optional
    OCIRef string `json:"ociRef,omitempty"`   // oci://quay.io/platform9/kaapi/capi-management-chart
    // +optional
    URL string `json:"url,omitempty"`         // https://.../kdu-156-pcd-v2026.4.tgz
    // +optional
    Version string `json:"version,omitempty"`
    // +optional
    PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}

type RegistrySpec struct {
    Host string `json:"host"`
    // +optional
    PathPrefix string `json:"pathPrefix,omitempty"`
    // +optional
    PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}
```

```go
// ComponentSpec is the single unit of customization. Keyed by component name in a
// map so that per-component patches are additive and don't require list-merge keys.
type ComponentSpec struct {
    // Enabled=false removes the component entirely from the deployment.
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // ---- version overrides ----

    // ImageTag overrides just the tag for this component's image(s), leaving the
    // rest of the release matrix intact.
    // +optional
    ImageTag string `json:"imageTag,omitempty"`

    // Image fully overrides the image reference (registry/repo:tag).
    // +optional
    Image string `json:"image,omitempty"`

    // ChartVersion overrides the subchart version where the component is packaged
    // as its own chart (kaapi, mariadb-operator, cert-manager, ...).
    // +optional
    ChartVersion string `json:"chartVersion,omitempty"`

    // ---- Kubernetes resource shaping ----

    // +optional
    Workload *WorkloadOverride `json:"workload,omitempty"`

    // ---- service configuration ----

    // Config carries service-level configuration file overrides — this is the
    // "down to nova.conf [DEFAULT] key=value" requirement.
    // +optional
    Config *ComponentConfig `json:"config,omitempty"`

    // HelmValues is free-form passthrough merged into the component's values
    // subtree. The escape hatch for anything the typed fields don't reach.
    // +kubebuilder:pruning:PreserveUnknownFields
    // +optional
    HelmValues *apiextensionsv1.JSON `json:"helmValues,omitempty"`

    // StrategicMergePatches are applied to rendered manifests post-templating.
    // Deliberately last-resort: every use is a gap in the typed API above and
    // should be tracked as such.
    // +optional
    StrategicMergePatches []apiextensionsv1.JSON `json:"strategicMergePatches,omitempty"`
}

type WorkloadOverride struct {
    // +optional
    Replicas *int32 `json:"replicas,omitempty"`
    // Resources per named container. Key "" or "*" applies to all containers.
    // +optional
    Resources map[string]corev1.ResourceRequirements `json:"resources,omitempty"`
    // +optional
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`
    // +optional
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
    // +optional
    Affinity *corev1.Affinity `json:"affinity,omitempty"`
    // +optional
    TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
    // +optional
    PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`
    // +optional
    Autoscaling *HPASpec `json:"autoscaling,omitempty"`
    // +optional
    PriorityClassName string `json:"priorityClassName,omitempty"`
    // +optional
    ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
    // +optional
    PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
    // +optional
    PodLabels map[string]string `json:"podLabels,omitempty"`
    // +optional
    Storage *StorageOverride `json:"storage,omitempty"`   // PVC size / storageClass
}

// ComponentConfig reaches into the service's own configuration files.
type ComponentConfig struct {
    // INI-style overrides, the common case for OpenStack services.
    //   ini:
    //     nova.conf:
    //       DEFAULT:
    //         cpu_allocation_ratio: "8.0"
    //       libvirt:
    //         live_migration_permit_auto_converge: "true"
    // +optional
    INI map[string]map[string]map[string]string `json:"ini,omitempty"`

    // Whole-file replacement, keyed by in-container path. Use sparingly — it opts
    // the file out of all future release-matrix updates.
    //   files:
    //     /etc/neutron/plugins/ml2/ml2_conf.ini: |
    //       [ml2]
    //       ...
    // +optional
    Files map[string]string `json:"files,omitempty"`

    // FilesFrom pulls file content from ConfigMaps/Secrets instead of inlining it.
    // +optional
    FilesFrom []FileSource `json:"filesFrom,omitempty"`

    // Structured (YAML/JSON) config overrides, deep-merged. For components whose
    // config is not INI (grafana, prometheus, fluent-bit, ...).
    // +kubebuilder:pruning:PreserveUnknownFields
    // +optional
    Structured *apiextensionsv1.JSON `json:"structured,omitempty"`

    // PolicyOverrides for OpenStack policy.yaml / policy.json RBAC rules.
    // +optional
    PolicyOverrides map[string]string `json:"policyOverrides,omitempty"`
}
```

### 4.3 External service bindings

Direct implementation of Phase 1 of the unification roadmap. Same struct is embeddable at infra, customer, and region level; the nearest declaration wins, so a region can point at a different database than its customer default.

```go
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

// DatabaseBinding wraps the MariaDB Kubernetes operator (k8s.mariadb.com/v1alpha1).
// There is exactly one database technology across every PCD flavor, so this type
// is a thin adapter over that operator's API rather than an abstraction over
// several backends. The typed fields below exist only because Size drives them
// (§4.4); everything else reaches the MariaDB CR through Template.
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

    // Topology maps to the MariaDB CR's HA stanza: Standalone leaves both unset,
    // Replication sets spec.replication, Galera sets spec.galera.
    // +kubebuilder:validation:Enum=Standalone;Replication;Galera
    // +optional
    Topology string `json:"topology,omitempty"`

    // Replicas overrides the node count implied by Size. Galera requires an odd
    // count of at least 3; the webhook rejects even counts under Galera rather
    // than letting the MariaDB operator discover it later.
    // +optional
    Replicas *int32 `json:"replicas,omitempty"`

    // Storage maps to MariaDB spec.storage. ResizeInUseVolumes requires a
    // StorageClass with allowVolumeExpansion=true; preflight checks this before
    // a Size increase is dispatched.
    // +optional
    Storage *MariaDBStorage `json:"storage,omitempty"`

    // MyCnf is server configuration, merged into MariaDB spec.myCnf. This is
    // where Size-driven tuning lands — buffer pool, max_connections, and the
    // rest. Per-service overrides follow the §4.1 precedence like any other
    // component config.
    // +optional
    MyCnf map[string]map[string]string `json:"myCnf,omitempty"`

    // Metrics enables the operator's built-in exporter (MariaDB spec.metrics).
    // Replaces the separately-deployed mysqld-exporter component.
    // +optional
    Metrics *MariaDBMetrics `json:"metrics,omitempty"`

    // MaxScale enables a MaxScale object in front of the instance for connection
    // routing and failover. Optional; Standalone and small sizes skip it.
    // +optional
    MaxScale *MaxScaleSpec `json:"maxScale,omitempty"`

    // Schema controls whether the operator emits Database/User/Grant objects for
    // each PCD service. See §4.6 — this is the part that changes what init-region
    // jobs are responsible for.
    // +optional
    Schema *SchemaPolicy `json:"schema,omitempty"`

    // Backup maps to Backup / PhysicalBackup objects and their schedule.
    // +optional
    Backup *DatabaseBackupPolicy `json:"backup,omitempty"`

    // BootstrapFrom maps to MariaDB spec.bootstrapFrom, restoring a new instance
    // from a backup or S3 source. The supported path for migrating an existing
    // PCD database onto operator management, and for DR rebuilds.
    // +kubebuilder:pruning:PreserveUnknownFields
    // +optional
    BootstrapFrom *apiextensionsv1.JSON `json:"bootstrapFrom,omitempty"`

    // Template is deep-merged into the generated MariaDB object's spec, last,
    // after every field above. It is the full k8s.mariadb.com/v1alpha1 MariaDB
    // API surface — podTemplate, tls, affinity, updateStrategy, galera recovery
    // tuning, anything the typed fields don't reach. Deliberately unvalidated
    // here: the MariaDB operator's own webhook is the authority on its schema,
    // and duplicating that validation would guarantee drift between versions.
    // +kubebuilder:pruning:PreserveUnknownFields
    // +optional
    Template *apiextensionsv1.JSON `json:"template,omitempty"`
}

type MariaDBRef struct {
    Name string `json:"name"`
    // +optional
    Namespace string `json:"namespace,omitempty"`
    // WaitForIt mirrors the operator's own mariaDbRef.waitForIt semantics.
    // +kubebuilder:default=true
    // +optional
    WaitForIt *bool `json:"waitForIt,omitempty"`
}

type SchemaPolicy struct {
    // Manage=true has the operator emit a Database, User and Grant object per
    // PCD service rather than leaving CREATE DATABASE / CREATE USER / GRANT to
    // each service's init-region job.
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

// CertificateBinding covers two distinct certificate roles. They are separated
// because they usually want different issuers: ingress TLS is typically a public
// ACME issuer, while hostagent PKI must be a private CA.
type CertificateBinding struct {
    // +kubebuilder:validation:Enum=cert-manager;self-signed;provided
    Provider string `json:"provider"`

    // IngressIssuer issues the public-facing TLS certificates for DU endpoints.
    // +optional
    IngressIssuer *IssuerRef `json:"ingressIssuer,omitempty"`

    // HostPKIIssuer is the private CA that issues Vouch hostagent certificates.
    // In the noconsul topology this replaces Vault as the PKI backend, which
    // makes it required for host onboarding rather than optional.
    // +optional
    HostPKIIssuer *IssuerRef `json:"hostPKIIssuer,omitempty"`

    // WildcardSecretRef supplies a pre-issued wildcard cert when provider=provided.
    // +optional
    WildcardSecretRef *corev1.SecretReference `json:"wildcardSecretRef,omitempty"`

    // ReplicateToNamespaces mirrors the kubernetes-replicator pattern used today
    // to work around LetsEncrypt per-domain rate limits.
    // +optional
    ReplicateToNamespaces bool `json:"replicateToNamespaces,omitempty"`
}
```

### 4.4 Supporting types

Everything referenced by §4.1–§4.3 that is not a Kubernetes API type or a
`k8s.mariadb.com/v1alpha1` type. Imports assumed throughout: `corev1`, `metav1`,
`apiextensionsv1`, `resource` (`k8s.io/apimachinery/pkg/api/resource`),
`intstr`, `autoscalingv2`.

```go
// ---- component shaping (§4.2) ----

type PDBSpec struct {
    // Exactly one of MinAvailable / MaxUnavailable. The webhook rejects both.
    // +optional
    MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
    // +optional
    MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

type HPASpec struct {
    // +optional
    MinReplicas *int32 `json:"minReplicas,omitempty"`
    MaxReplicas int32  `json:"maxReplicas"`
    // +optional
    TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
    // +optional
    TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
    // Behavior is passed through to the generated HPA unchanged.
    // +optional
    Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

type StorageOverride struct {
    // +optional
    Size *resource.Quantity `json:"size,omitempty"`
    // +optional
    StorageClassName *string `json:"storageClassName,omitempty"`
    // +optional
    AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

type FileSource struct {
    // MountPath is the in-container directory the source is projected into.
    MountPath string `json:"mountPath"`
    // Exactly one of ConfigMapRef / SecretRef.
    // +optional
    ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`
    // +optional
    SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
    // +optional
    Items []corev1.KeyToPath `json:"items,omitempty"`
}

// ---- shared leaf types ----

type TLSSettings struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`
    // +optional
    CASecretRef *corev1.LocalObjectReference `json:"caSecretRef,omitempty"`
    // +optional
    InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

type IssuerRef struct {
    // +kubebuilder:validation:Enum=Issuer;ClusterIssuer
    Kind string `json:"kind"`
    Name string `json:"name"`
    // +kubebuilder:default="cert-manager.io"
    // +optional
    Group string `json:"group,omitempty"`
}

type MaintenanceWindow struct {
    // Schedule is a cron expression marking the start of the window.
    Schedule string `json:"schedule"`
    Duration metav1.Duration `json:"duration"`
    // +kubebuilder:default="UTC"
    // +optional
    TimeZone string `json:"timeZone,omitempty"`
}

type UpgradePolicy struct {
    // +kubebuilder:validation:Enum=Immediate;Window;Manual
    // +kubebuilder:default=Immediate
    Trigger string `json:"trigger"`
    // +optional
    Window *MaintenanceWindow `json:"window,omitempty"`
    // MaxConcurrent caps how many child objects upgrade at once. On a
    // PCDUnderlay this is the fleet-wide budget (§10, open question 4).
    // +optional
    MaxConcurrent *int32 `json:"maxConcurrent,omitempty"`
    // Timeout after which an in-flight operation is treated as Faulted.
    // +optional
    Timeout *metav1.Duration `json:"timeout,omitempty"`
    // AutoRollback permits automatic back-out for components whose rollback
    // safety class allows it (§12.5). Class C and D never roll back
    // automatically regardless of this setting.
    // +kubebuilder:default=true
    // +optional
    AutoRollback *bool `json:"autoRollback,omitempty"`
}

// ---- networking (§6, §8) ----

type InfraNetworkingSpec struct {
    // HostedZone is the DNS suffix every DU FQDN is built under.
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

type LoadBalancerSpec struct {
    // +kubebuilder:validation:Enum=aws-nlb;oci-lb;metallb;none
    Provider string `json:"provider"`
    // +optional
    Annotations map[string]string `json:"annotations,omitempty"`
    // AddressPool is required when Provider is metallb.
    // +optional
    AddressPool []string `json:"addressPool,omitempty"`
}

type VirtualIPSpec struct {
    // +optional
    ManagementCluster string `json:"managementCluster,omitempty"`
    // +optional
    DeploymentUnit string `json:"deploymentUnit,omitempty"`
}

type ProxySpec struct {
    // +optional
    HTTPProxy string `json:"httpProxy,omitempty"`
    // +optional
    HTTPSProxy string `json:"httpsProxy,omitempty"`
    // +optional
    NoProxy string `json:"noProxy,omitempty"`
}

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

type OVNSpec struct {
    // Replica counts default from Size (§4.5); OVN cannot autoscale.
    // +optional
    NorthboundDBReplicas *int32 `json:"northboundDBReplicas,omitempty"`
    // +optional
    SouthboundDBReplicas *int32 `json:"southboundDBReplicas,omitempty"`
    // +optional
    RelayReplicas *int32 `json:"relayReplicas,omitempty"`
}

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

type MetadataServiceSpec struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`
    // +optional
    Port *int32 `json:"port,omitempty"`
}

// ---- MariaDB wrapper leaves (§4.3) ----

type MariaDBStorage struct {
    // +optional
    Size *resource.Quantity `json:"size,omitempty"`
    // +optional
    StorageClassName string `json:"storageClassName,omitempty"`
    // ResizeInUseVolumes requires a StorageClass with allowVolumeExpansion=true.
    // Preflight asserts this before dispatching a Size increase.
    // +optional
    ResizeInUseVolumes *bool `json:"resizeInUseVolumes,omitempty"`
    // +optional
    WaitForVolumeResize *bool `json:"waitForVolumeResize,omitempty"`
    // Ephemeral provisions without a PVC. CE and test only; the webhook
    // rejects it on any deployment whose profile is not community-edition.
    // +optional
    Ephemeral *bool `json:"ephemeral,omitempty"`
}

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

type MaxScaleSpec struct {
    // +kubebuilder:default=false
    // +optional
    Enabled *bool `json:"enabled,omitempty"`
    // +optional
    Replicas *int32 `json:"replicas,omitempty"`
    // Template is merged into the generated MaxScale object's spec, same
    // passthrough contract as DatabaseBinding.Template.
    // +kubebuilder:pruning:PreserveUnknownFields
    // +optional
    Template *apiextensionsv1.JSON `json:"template,omitempty"`
}

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

// ---- remaining external service bindings (§4.3) ----

type StorageBinding struct {
    // +kubebuilder:validation:Enum=hostpath;nfs;csi;cloud
    Provider string `json:"provider"`
    // StorageClassName is the class PCD workloads request. PCD has used
    // `pcd-sc` by convention.
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

type NFSSpec struct {
    Server string `json:"server"`
    Path   string `json:"path"`
    // +optional
    MountOptions []string `json:"mountOptions,omitempty"`
}

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

type MessageQueueBinding struct {
    // +kubebuilder:validation:Enum=Dedicated;Shared
    // +kubebuilder:default=Dedicated
    Provisioning string `json:"provisioning"`
    // +optional
    InstanceRef *corev1.LocalObjectReference `json:"instanceRef,omitempty"`
    // Replicas defaults from Size; RabbitMQ is quorum-bound and does not
    // autoscale (§4.5, §12.5 class D).
    // +optional
    Replicas *int32 `json:"replicas,omitempty"`
    // +optional
    Storage *StorageOverride `json:"storage,omitempty"`
    // +optional
    TLS *TLSSettings `json:"tls,omitempty"`
}

type SecretsBinding struct {
    // kubernetes stores credentials as plain Secrets; external-secrets syncs
    // them from an upstream store. Vault is deliberately absent — the noconsul
    // topology removed it (§6).
    // +kubebuilder:validation:Enum=kubernetes;external-secrets
    // +kubebuilder:default=kubernetes
    Provider string `json:"provider"`
    // +optional
    ExternalSecrets *ExternalSecretsSpec `json:"externalSecrets,omitempty"`
}

type ExternalSecretsSpec struct {
    // SecretStoreRef names a SecretStore or ClusterSecretStore.
    SecretStoreRef corev1.TypedLocalObjectReference `json:"secretStoreRef"`
    // +optional
    RefreshInterval *metav1.Duration `json:"refreshInterval,omitempty"`
}

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

type IdentityBinding struct {
    // +kubebuilder:validation:Enum=local;saml;oidc
    // +kubebuilder:default=local
    Provider string `json:"provider"`
    // IDPMetadataSecretRef is required when Provider is saml.
    // +optional
    IDPMetadataSecretRef *corev1.LocalObjectReference `json:"idpMetadataSecretRef,omitempty"`
    // +optional
    OIDC *OIDCSpec `json:"oidc,omitempty"`
    // DefaultRole is the Keystone role assigned to users with no mapping.
    // +optional
    DefaultRole string `json:"defaultRole,omitempty"`
    // AttributeMapping maps IdP assertion attributes to Keystone attributes.
    // +optional
    AttributeMapping map[string]string `json:"attributeMapping,omitempty"`
}

type OIDCSpec struct {
    IssuerURL string `json:"issuerURL"`
    ClientID  string `json:"clientID"`
    ClientSecretRef *corev1.LocalObjectReference `json:"clientSecretRef"`
    // +optional
    Scopes []string `json:"scopes,omitempty"`
}

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

type RemoteWriteTarget struct {
    URL string `json:"url"`
    // +optional
    CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`
    // +optional
    Headers map[string]string `json:"headers,omitempty"`
}

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

// BackupBinding covers management-plane backup (the mgmt-plane-backup
// component). Database backups are DatabaseBinding.Backup, not this.
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

// ---- status leaves (§5) ----

type AppliedRelease struct {
    MatrixVersion string `json:"matrixVersion"`
    // Chart is the artifact actually resolved and applied, which may differ
    // from the matrix default when ChartOverride was set.
    // +optional
    Chart *ChartRef `json:"chart,omitempty"`
    // +optional
    AppliedAt *metav1.Time `json:"appliedAt,omitempty"`
}

type OperationStatus struct {
    // Type is a neutral operation name. Backend job names are never surfaced
    // here — see the leak table in §2.1.
    // +kubebuilder:validation:Enum=Install;Upgrade;Resize;Teardown
    Type string `json:"type"`
    // +optional
    StartedAt *metav1.Time `json:"startedAt,omitempty"`
    // Deadline after which the operation is treated as Faulted rather than
    // left to run indefinitely (§12.4, Blocked).
    // +optional
    Deadline *metav1.Time `json:"deadline,omitempty"`
    // Attempt counts retries of the current desired state.
    // +optional
    Attempt int32 `json:"attempt,omitempty"`
    // LogHint points at where to look, in terms the person reading it can use
    // without knowing what the backend is.
    // +optional
    LogHint string `json:"logHint,omitempty"`
}
```

### 4.5 Size

`spec.size` is a top-level field on `PCDInstallation` and `PCDRegion`. It expresses the intended scale of a deployment once, instead of requiring the operator to hand-tune two dozen components to say the same thing. It supersedes the earlier `spec.sizing` draft field, and is the natural home for the standard-size work tracked in SRE-376.

```go
// +kubebuilder:validation:Enum=X-Small;Small;Medium;Large;X-Large
type PCDSize string

const (
    SizeXSmall PCDSize = "X-Small"
    SizeSmall  PCDSize = "Small"
    SizeMedium PCDSize = "Medium"
    SizeLarge  PCDSize = "Large"
    SizeXLarge PCDSize = "X-Large"
)
```

It does two different jobs, and keeping them distinct matters when writing the presets:

**1. Fixed sizing for components that cannot autoscale seamlessly.** Stateful and quorum-bound services can't be handed to an HPA — MariaDB (node count, buffer pool, `max_connections`), the OVN databases (northbound/southbound replica counts, relay fan-out), RabbitMQ, and memcached. For these, size selects a concrete resource footprint and replica count. This is the harder half of the presets, because the values are not a smooth curve: a 3-node Galera cluster at Medium and a 5-node one at X-Large is a topology change, not a resource bump.

**2. HPA shaping for components that can autoscale.** Nova, Keystone, Cinder, Neutron, Glance, Placement and the rest of the API tier scale horizontally, but a single HPA configuration serves small and large deployments badly. Size tunes `minReplicas`, `maxReplicas`, and the utilisation target — a small deployment wants a low floor and a conservative ceiling; a large one wants enough headroom at the floor to absorb a burst without a scale-up round trip.

Illustrative shape (values are placeholders pending load data, not proposals):

| Size | Autoscaling API tier | Non-autoscaling tier |
|---|---|---|
| X-Small | `min 1`, `max 2`, high util target | Standalone MariaDB, single OVN NB/SB, no relay |
| Small | `min 1`, `max 4` | Standalone MariaDB, single OVN NB/SB |
| Medium | `min 2`, `max 8` | 3-node Galera, no MaxScale, 3× OVN NB/SB |
| Large | `min 3`, `max 12` | 3-node Galera + MaxScale, 3× OVN NB/SB, OVN relays |
| X-Large | `min 4`, `max 20`, low util target | 5-node Galera + MaxScale, tuned relay fan-out |

Rules that keep it predictable:

- **Size is a default layer, not a constraint.** It sits at layer 3 in §4.1, so anything set in `componentDefaults` or `components[name]` wins over it. Choosing `Large` and then pinning `nova-api-osapi` to five replicas is legal and does exactly what it says.
- **The two levels are independent.** A `PCDRegion` inherits its `PCDInstallation`'s size when `spec.size` is unset, but may declare its own. An installation with a `Large` infra region and a `Small` workload region is a real configuration — Keystone load does not track hypervisor count.
- **Size is mutable.** Changing it is a resize, reconciled like any other spec change and subject to `UpgradePolicy`. Resizing downward across a topology boundary (5-node Galera to 3-node, or Galera to Standalone) is destructive to quorum and should be gated by a webhook warning rather than silently applied.
- **Presets are release-matrix data, not code.** They ship alongside the chart set so a preset can be retuned in a patch release without an operator rebuild.
- **There is no `Custom` value.** An earlier draft carried a `custom` t-shirt size; it is redundant, since omitting `size` and setting `componentDefaults` achieves the same thing without a second way to say it.

### 4.6 What the MariaDB wrapper replaces

Every PCD flavor now uses one database technology, so the operator no longer carries a provider abstraction. `DatabaseBinding` (§4.3) is a wrapper over `k8s.mariadb.com/v1alpha1`, not a layer above several backends.

| Previous configuration | Now |
|---|---|
| `provider: percona-pxc` | `topology: Galera` |
| `provider: local-mysql` | `topology: Standalone` |
| `provider: external` (host/port/credentials) | `provisioning: Shared` + `instanceRef` to a MariaDB object. Arbitrary endpoints are no longer expressible. |
| `mysqld-exporter` as a deployed component | `metrics.enabled` on the MariaDB object |
| haproxy fronting PXC | `maxScale.enabled` |
| Per-provider backup stanzas | `backup`, emitting Backup / PhysicalBackup objects |

Two consequences worth deciding on deliberately rather than inheriting.

**Wrap, don't re-model.** The typed fields on `DatabaseBinding` exist only because `Size` drives them. Everything else goes through `template`, deep-merged into the generated MariaDB object's spec and deliberately left unvalidated on our side. The MariaDB operator's own webhook is the authority on its schema; duplicating that validation here would guarantee drift the first time upstream adds a field. The cost is that a malformed `template` fails at the MariaDB webhook rather than at ours, so the upstream error has to be passed through verbatim into the component's `Faulted` message rather than swallowed.

**Schema management moves out of `init-region` jobs.** This is the larger change. Today each service's init job runs `CREATE DATABASE`, creates its user, and grants. With `schema.manage: true` the operator emits a `Database`, `User` and `Grant` object per service instead, and the MariaDB operator reconciles them. That is a direct improvement against §12.6's idempotency requirement: those objects are declarative and idempotent by construction, where an init job is idempotent only if someone wrote it that way. It also changes what a component waits on — a service in `Pending` blocks on its `Grant` reporting Ready, which is an observable object, rather than on an init job exit code.

It does not eliminate init jobs. Schema *migrations* stay with the services, because only the service knows its own migration chain. The split is: database existence and access rights belong to the database operator, table structure remains the service's.

---

## 5. Status conventions (all three kinds)

```go
type CommonStatus struct {
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // Phase is a coarse rollup, derived from observed cluster state rather than
    // from any single backend response. Backend-internal state vocabulary is
    // translated into this enum by the adapter and never surfaced verbatim.
    // +kubebuilder:validation:Enum=Pending;Installing;Upgrading;Ready;Degraded;Deleting;Error
    // +optional
    Phase string `json:"phase,omitempty"`

    // Conditions: Reconciling, Available, Progressing, Degraded, ReleaseResolved,
    // PrerequisitesMet, UpgradeInProgress
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // AppliedRelease is what is actually running, which may lag spec.release
    // mid-upgrade. Comparing the two is how you answer "is this DU on 2026.4?"
    // +optional
    AppliedRelease *AppliedRelease `json:"appliedRelease,omitempty"`

    // Components carries per-component health so a single unhealthy service
    // doesn't have to be inferred from a rolled-up phase.
    // +optional
    Components []ComponentStatus `json:"components,omitempty"`

    // ReadyComponents / DesiredComponents mirror `airctl status`
    // ("desired services: 45 / ready services: 45").
    // +optional
    ReadyComponents int32 `json:"readyComponents,omitempty"`
    // +optional
    DesiredComponents int32 `json:"desiredComponents,omitempty"`

    // RenderedValuesRef points at a ConfigMap holding the fully-merged values
    // after all six override layers, for diffing and support.
    // +optional
    RenderedValuesRef *corev1.LocalObjectReference `json:"renderedValuesRef,omitempty"`

    // ActiveOperation reports the in-flight operation (Install/Upgrade/Teardown),
    // when it started, and where to find its logs, so `kubectl describe` is
    // enough to debug. Names a neutral operation type, not a backend job name.
    // +optional
    ActiveOperation *OperationStatus `json:"activeOperation,omitempty"`
}

// ComponentStatus is defined once, in §12.7. Its Phase vocabulary IS the
// per-component state machine, so keeping the type next to the machine that
// defines it avoids two enums drifting apart.
```

---

## 6. `PCDUnderlay`

Cluster-scoped entry point. Owns everything that must exist *before* any `PCDInstallation` can be created — the substrate the rest of PCD is laid on top of, which is what the name is meant to convey.

Everything in this kind is reconciled by applying Helm releases and manifests directly against the cluster. Bork is not involved at any point — it neither knows about these components nor is asked to install them. This is why the prerequisite catalog below is expressed as chart/component names rather than anything backend-shaped, and why `spec.components` overrides here resolve straight to Helm values.

```go
type PCDUnderlaySpec struct {
    // Profile seeds defaults appropriate to the deployment context. This is what
    // makes principle #1 tractable: four very different installations differ
    // mostly in defaults, not in structure.
    // +kubebuilder:validation:Enum=saas-aws;saas-oci;on-prem;community-edition
    Profile string `json:"profile"`

    // Release pins prerequisite software versions collectively.
    Release ReleaseSpec `json:"release"`

    // Platform describes the underlying cluster and cloud.
    Platform PlatformSpec `json:"platform"`

    // ExternalServices declares platform-level integrations. Inherited as
    // defaults by every PCDInstallation and PCDRegion beneath this underlay.
    // +optional
    ExternalServices *ExternalServices `json:"externalServices,omitempty"`

    // Networking: hosted zone, ingress class, VIPs, LB provider, proxy config.
    Networking InfraNetworkingSpec `json:"networking"`

    // ComponentDefaults applies to every prerequisite component.
    // +optional
    ComponentDefaults *ComponentSpec `json:"componentDefaults,omitempty"`

    // Components keys off the catalog below.
    // +optional
    Components map[string]ComponentSpec `json:"components,omitempty"`

    // UpgradePolicy governs how prerequisite upgrades roll.
    // +optional
    UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`

    // Paused halts reconciliation without deleting anything. Essential for
    // incident response on a live SaaS platform.
    // +optional
    Paused bool `json:"paused,omitempty"`
}

// PlatformSpec describes the cluster the operator runs in. There is no cluster
// selection: the operator and every DU it manages share one cluster. Cluster
// placement concepts (Aim, TargetCluster) are handled inside the Bork adapter and
// are deliberately absent here.
type PlatformSpec struct {
    // +kubebuilder:validation:Enum=aws-eks;oci-oke;azure-aks;gke;nodelet;k3s;generic
    Kind string `json:"kind"`

    // +optional
    Region string `json:"region,omitempty"`
    // +optional
    KubernetesVersionConstraint string `json:"kubernetesVersionConstraint,omitempty"` // ">=1.30.0"
    // +optional
    Airgapped bool `json:"airgapped,omitempty"`
}
```

### Component catalog — infra prerequisites

Derived from the running-pod inventory of a real on-prem install plus the SaaS/EKS stack. Grouped by function; every key is valid in `spec.components`.

| Function | Component keys |
|---|---|
| Certificates | `cert-manager`, `cert-manager-cainjector`, `cert-manager-webhook`, `kubernetes-replicator` |
| Ingress / L4 | `ingress-nginx`, `k8sniff`, `metallb`, `haproxy-ingress`, `external-dns`, `external-dns-crd` |
| Secrets mgmt | `external-secrets`, `sealed-secrets` |
| Storage | `hostpath-provisioner`, `csi-nfs`, `snapshot-controller`, `ebs-csi`, `oci-csi`, `minio` |
| Database operator | `mariadb-operator` |
| Observability | `metrics-server`, `kube-state-metrics`, `fluent-bit`, `prometheus-operator-crds` |
| Kubernetes mgmt plane (kaapi) | `kaapi`, `capi`, `capi-kubeadm-bootstrap`, `capo`, `byoh`, `kamaji`, `kamaji-etcd`, `projectsveltos`, `addon-controller`, `classifier-manager`, `sc-manager`, `hc-manager`, `event-manager`, `access-manager`, `shard-controller`, `barista` |
| PCD control plane | `bork`, `kplane-usermgr`, `k8s-helm-runner`, `deccaxon` |
| Utilities | `mgmt-plane-backup`, `node-taint-check` |

> **No Consul, no Vault, no decco.** Because Bork3 is a prerequisite, none of these are components the operator installs, tolerates, or offers a toggle for. They are absent by construction, and the catalog above deliberately gives no way to reintroduce them.
>
> Vouch hostagent certificate issuance, which Vault previously backed, is handled by cert-manager. That makes `cert-manager` load-bearing in a way it was not before: it is no longer only the wildcard-TLS provider but also the PKI root for host onboarding. Two consequences, both reflected above. `CertificateBinding` (§4.3) splits into `ingressIssuer` and `hostPKIIssuer`, because a deployment will typically want a public ACME issuer for endpoint TLS and a private CA for hostagent certs. And `cert-manager` is non-disableable: setting `enabled: false` on it would silently break host onboarding rather than trimming a component, so the webhook rejects it.
>
> The region-scoped `pf9-vault` component in §7 and §8 is a different thing and stays.

### Example

```yaml
apiVersion: install.pcd.platform9.com/v1alpha1
kind: PCDUnderlay
metadata:
  name: pcd-saas-us-west-2
spec:
  profile: saas-aws
  paused: false

  release:
    matrixVersion: "2026.4-patch2"
    imageRegistry:
      host: quay.io
      pathPrefix: platform9
      pullSecretRef: { name: pf9-registry }

  platform:
    kind: aws-eks
    region: us-west-2
    kubernetesVersionConstraint: ">=1.30.0"

  networking:
    hostedZone: app.qa-pcd.platform9.com
    ingressClass: nginx
    loadBalancer:
      provider: aws-nlb
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-type: nlb

  externalServices:
    certificates:
      provider: cert-manager
      ingressIssuer: { kind: ClusterIssuer, name: letsencrypt-prod }
      hostPKIIssuer: { kind: ClusterIssuer, name: pcd-host-ca }
      replicateToNamespaces: true
    dns:
      provider: route53
      hostedZoneID: Z0123456789ABCDEF
    database:
      provisioning: Dedicated
      topology: Galera
      storage:
        size: 500Gi
        storageClassName: gp3
        resizeInUseVolumes: true

  componentDefaults:
    workload:
      priorityClassName: pcd-infra

  components:
    ingress-nginx:
      workload:
        replicas: 3
        resources:
          controller:
            requests: { cpu: "500m", memory: "512Mi" }
            limits:   { memory: "2Gi" }
        topologySpreadConstraints:
          - maxSkew: 1
            topologyKey: topology.kubernetes.io/zone
            whenUnsatisfiable: DoNotSchedule
            labelSelector:
              matchLabels: { app.kubernetes.io/name: ingress-nginx }
    metallb:
      enabled: false          # cloud LB instead
    hostpath-provisioner:
      enabled: false
    kaapi:
      chartVersion: "v2026.4.2"
```

The Community Edition contrast, showing how much of principle #1 the profile absorbs:

```yaml
apiVersion: install.pcd.platform9.com/v1alpha1
kind: PCDUnderlay
metadata:
  name: pcd-ce
spec:
  profile: community-edition
  release:
    matrixVersion: "2026.4"
  platform:
    kind: k3s
    airgapped: false
  networking:
    hostedZone: pcd-community.localnet
    ingressClass: nginx
    virtualIPs:
      managementCluster: 10.0.0.10
      deploymentUnit: 10.0.0.11
  externalServices:
    certificates: { provider: self-signed }
    storage:
      provider: hostpath
      storageClassName: pcd-sc
  components:
    minio: { enabled: false }
    fluent-bit: { enabled: false }
    kamaji-etcd:
      workload:
        replicas: 1
```

---

## 7. `PCDInstallation`

A PCD installation: one customer plus its Infra Region — the region that runs Keystone and identity/auth dependencies only. Renamed from `PCDCustomer` because the object is the installation, not the commercial relationship; "customer" survives in this document only where it means the tenant a given installation belongs to.

This kind deliberately does two jobs: it is both the tenancy record and the infra region's deployment spec. That is a settled decision, not an oversight. The two are 1:1 today, there is no known requirement for a customer with zero or two infra regions, and splitting them would add a fourth object to every install to model a relationship that has no degrees of freedom. If that ever changes, a separate infra-region kind can be introduced later with `PCDInstallation` retaining an inline default for compatibility.

Creating a `PCDInstallation` provisions the customer record and the infra region as one atomic unit; the backend namespace layout that results is an implementation detail of the adapter.

```go
type PCDInstallationSpec struct {
    // UnderlayRef binds to the cluster-scoped underlay. Defaults to
    // the single installation if exactly one exists.
    // +optional
    UnderlayRef *corev1.LocalObjectReference `json:"underlayRef,omitempty"`

    // ShortName is the customer identifier. Immutable — it determines namespace
    // names and FQDNs, and renaming would orphan the x- namespace.
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    // +kubebuilder:validation:MaxLength=48
    ShortName string `json:"shortName"`

    // +optional
    DisplayName string `json:"displayName,omitempty"`

    // AdminEmail seeds the initial Keystone admin.
    AdminEmail string `json:"adminEmail"`

    // AdminCredentialsRef holds the initial admin password. If unset, the
    // operator generates one and writes it here.
    // +optional
    AdminCredentialsRef *corev1.LocalObjectReference `json:"adminCredentialsRef,omitempty"`

    // FQDN of the infra region. Defaults to "<shortName>.<hostedZone>".
    // +optional
    FQDN string `json:"fqdn,omitempty"`

    // Release for the infra region. Defaults to the installation's release.
    // Carried separately so a customer can be pinned behind the fleet during a
    // staged rollout.
    // +optional
    Release *ReleaseSpec `json:"release,omitempty"`

    // +optional
    ExternalServices *ExternalServices `json:"externalServices,omitempty"`

    // SSO configuration for this installation (dex, shibboleth/envoy, oidc-proxy,
    // hagrid). Same type as ExternalServices.Identity — an installation-scoped
    // declaration overrides the underlay default rather than sitting beside it.
    // +optional
    SSO *IdentityBinding `json:"sso,omitempty"`

    // +optional
    ComponentDefaults *ComponentSpec `json:"componentDefaults,omitempty"`
    // +optional
    Components map[string]ComponentSpec `json:"components,omitempty"`

    // Size selects the deployment scale preset. Drives fixed resource footprints
    // and replica counts for components that cannot autoscale (MariaDB,
    // OVN, RabbitMQ, memcached) and tuned HPA bounds for those that can
    // (Keystone and the API tier). Override layer 3 — see §4.5.
    // Unset means the profile's default size.
    // +optional
    Size PCDSize `json:"size,omitempty"`

    // Protection prevents accidental deletion, enforced at the admission webhook
    // so the block happens before any teardown work is dispatched.
    // +optional
    Protection *ProtectionSpec `json:"protection,omitempty"`

    // +optional
    UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`
    // +optional
    Paused bool `json:"paused,omitempty"`
}

type ProtectionSpec struct {
    PreventDeletion bool   `json:"preventDeletion"`
    Reason          string `json:"reason,omitempty"`
    Owner           string `json:"owner,omitempty"`
}
```

### Component catalog — infra region

| Function | Component keys |
|---|---|
| Identity | `keystone`, `vouch-keystone`, `vouch-noauth`, `dex`, `envoy-shibboleth`, `oidc-proxy`, `hagrid`, `ssohagrid` |
| Data | `mariadb` (MariaDB CR), `maxscale`, `mysql-forwarder`, `memcached`, `rabbitmq` |
| Platform services | `resmgr`, `preference-store`, `sidekickserver`, `pf9-notifications`, `pf9-vault`, `sentinel`, `serenity`, `clarity` |
| Ingress | `pf9-nginx`, `ingress-nginx` |
| Observability | `prometheus`, `alertmanager`, `blackbox-exporter`, `forwarder` |
| Bootstrap (jobs) | `deccaxon`, `*-init`, `*-ks-user` |

### Example

```yaml
apiVersion: install.pcd.platform9.com/v1alpha1
kind: PCDInstallation
metadata:
  name: acme
  namespace: pcd-system
spec:
  underlayRef: { name: pcd-saas-us-west-2 }
  shortName: acme
  displayName: "Acme Corporation"
  adminEmail: cloud-admin@acme.example
  fqdn: acme.app.qa-pcd.platform9.com
  size: Large

  release:
    matrixVersion: "2026.4-patch2"
    components: {}

  externalServices:
    database:
      provisioning: Dedicated
      topology: Galera            # replicas default from size: Large
      myCnf:
        mysqld:
          max_connections: "5000"
          innodb_buffer_pool_size: "12G"
      metrics:
        enabled: true
      maxScale:
        enabled: true
      schema:
        manage: true
        maxUserConnections: 200
      backup:
        type: Physical
        schedule: "0 0 * * *"
        retention: 336h
        storage:
          s3:
            bucket: pcd-acme-db-backups
            prefix: acme
            endpoint: s3.us-west-2.amazonaws.com
            region: us-west-2
      template:                   # raw MariaDB spec passthrough
        podTemplate:
          topologySpreadConstraints:
            - maxSkew: 1
              topologyKey: topology.kubernetes.io/zone
              whenUnsatisfiable: DoNotSchedule

  sso:
    provider: saml
    idpMetadataSecretRef: { name: acme-idp-metadata }
    defaultRole: _member_

  protection:
    preventDeletion: true
    reason: "production customer"
    owner: sre-team

  components:
    keystone:
      workload:
        replicas: 4
        resources:
          keystone-api:
            requests: { cpu: "1", memory: "2Gi" }
            limits:   { memory: "4Gi" }
      config:
        ini:
          keystone.conf:
            token:
              expiration: "7200"
            cache:
              enabled: "true"
              backend: dogpile.cache.memcached
    resmgr:
      config:
        ini:
          resmgr.conf:
            database:
              max_connections: "5000"     # cf. the max_connections issue in OnPrem PCD Steps
    clarity:
      imageTag: "2026.4.2-2401"           # UI ships ahead of the matrix (roadmap Phase 5)
```

---

## 8. `PCDRegion`

A workload/DU region attached to a `PCDInstallation`, running the OpenStack API service suite.

```go
type PCDRegionSpec struct {
    // InstallationRef is required and immutable.
    // +kubebuilder:validation:Required
    InstallationRef corev1.LocalObjectReference `json:"installationRef"`

    // RegionName as OpenStack sees it, e.g. "Region-One". Immutable.
    // +kubebuilder:validation:Required
    RegionName string `json:"regionName"`

    // RegionInstance is the FQDN slug: "<shortName>-<regionInstance>.<hostedZone>".
    // +optional
    RegionInstance string `json:"regionInstance,omitempty"`

    // +optional
    FQDN string `json:"fqdn,omitempty"`

    // Release defaults to the customer's release. A region may be pinned behind
    // its customer during a staged upgrade, but the operator rejects a region
    // release *ahead* of its customer's — the infra region must upgrade first.
    // +optional
    Release *ReleaseSpec `json:"release,omitempty"`

    // +optional
    ExternalServices *ExternalServices `json:"externalServices,omitempty"`

    // Networking for the region's data plane: OVN, provider networks, MTU,
    // metadata service, floating IP pools.
    // +optional
    Networking *RegionNetworkingSpec `json:"networking,omitempty"`

    // Features toggles optional capability sets as a unit. Sugar over
    // components{} for the common CE-minimization and feature-gating cases.
    // +optional
    Features *RegionFeatures `json:"features,omitempty"`

    // NOTE: host-side lifecycle (pcdctl prep-node, host agent packages, host
    // upgrades) is intentionally absent. It lands in a future PCDHostPool kind —
    // see §11. Do not add a hostManagement stanza here in the interim; a
    // half-modelled field is harder to remove than to add.

    // Size selects the deployment scale preset for this region. Inherits the
    // parent PCDInstallation's size when unset. A region may legitimately differ
    // from its installation — Keystone load does not track hypervisor count.
    // See §4.5.
    // +optional
    Size PCDSize `json:"size,omitempty"`

    // +optional
    ComponentDefaults *ComponentSpec `json:"componentDefaults,omitempty"`
    // +optional
    Components map[string]ComponentSpec `json:"components,omitempty"`

    // +optional
    Protection *ProtectionSpec `json:"protection,omitempty"`
    // +optional
    UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`
    // +optional
    Paused bool `json:"paused,omitempty"`
}

type RegionFeatures struct {
    // +optional
    Orchestration *bool `json:"orchestration,omitempty"`     // heat-api, heat-cfn, heat-engine
    // +optional
    IaC *bool `json:"iac,omitempty"`                         // terrakube-*
    // +optional
    LoadBalancing *bool `json:"loadBalancing,omitempty"`     // octavia
    // +optional
    DNSaaS *bool `json:"dnsaas,omitempty"`                   // designate
    // +optional
    KeyManagement *bool `json:"keyManagement,omitempty"`     // barbican
    // +optional
    Optimization *bool `json:"optimization,omitempty"`       // watcher
    // +optional
    HighAvailability *bool `json:"highAvailability,omitempty"` // masakari, hamgr, pf9-ha
    // +optional
    AppCatalog *bool `json:"appCatalog,omitempty"`
    // +optional
    Audit *bool `json:"audit,omitempty"`
    // +optional
    Kubernetes *bool `json:"kubernetes,omitempty"`           // PCD-K / kaapi enablement
}
```

### Component catalog — workload region

| Function | Component keys |
|---|---|
| Compute | `nova-api-osapi`, `nova-api-metadata`, `nova-conductor`, `nova-scheduler`, `nova-cell-setup`, `nova-archive-deleted-rows` |
| Networking | `neutron-server`, `ovn-northd`, `ovn-ovsdb-nb`, `ovn-ovsdb-sb`, `ovn-ovsdb-relay`, `designate-api`, `designate-central`, `designate-producer` |
| Images / storage | `glance-api`, `cinder-api`, `cinder-scheduler` |
| Placement | `placement-api` |
| Secrets / LB | `barbican-api`, `octavia-api` |
| Orchestration | `heat-api`, `heat-cfn`, `heat-engine`, `heat-engine-cleaner`, `heat-purge-deleted`, `terrakube-api`, `terrakube-executor`, `terrakube-redis`, `terrakube-registry` |
| HA / optimization | `masakari-api`, `masakari-engine`, `hamgr`, `pf9-ha-slave`, `watcher-api`, `watcher-applier`, `watcher-decision-engine`, `watcher-predictions` |
| PCD services | `resmgr`, `cqrs`, `mors`, `sentinel`, `serenity`, `sidekickserver`, `preference-store`, `pf9-notifications`, `pf9-vault`, `appcatalog`, `horizon`, `clarity` |
| Data | `mariadb` (MariaDB CR), `maxscale`, `mysql-forwarder`, `memcached`, `rabbitmq` |
| Ingress | `pf9-nginx`, `ingress-nginx` |
| Observability | `prometheus`, `prometheusopenstack`, `alertmanager`, `grafana`, `blackbox-exporter`, `openstackexporter`, `kube-state-metrics`, `forwarder` |
| Bootstrap (jobs) | `deccaxon`, `*-init`, `*-ks-user` |

### Example

```yaml
apiVersion: install.pcd.platform9.com/v1alpha1
kind: PCDRegion
metadata:
  name: acme-region-one
  namespace: pcd-system
spec:
  installationRef: { name: acme }
  regionName: Region-One
  regionInstance: region-one
  size: Large

  release:
    matrixVersion: "2026.4-patch2"

  features:
    orchestration: true
    loadBalancing: true
    highAvailability: true
    dnsaas: false
    iac: false            # skip terrakube
    audit: false

  networking:
    ovn:
      relayReplicas: 3
      northboundDBReplicas: 3
    providerNetworks:
      - name: external
        type: flat
        physicalNetwork: physnet1
    mtu: 9000

  components:
    nova-api-osapi:
      workload:
        replicas: 4
        autoscaling:
          minReplicas: 4
          maxReplicas: 12
          targetCPUUtilizationPercentage: 70
      config:
        ini:
          nova.conf:
            DEFAULT:
              cpu_allocation_ratio: "8.0"
              ram_allocation_ratio: "1.2"
              osapi_compute_workers: "8"
            api_database:
              max_pool_size: "30"
        policyOverrides:
          policy.yaml: |
            "os_compute_api:servers:show:host_status": "rule:admin_api"

    neutron-server:
      config:
        ini:
          neutron.conf:
            DEFAULT:
              global_physnet_mtu: "9000"
          plugins/ml2/ml2_conf.ini:
            ml2:
              path_mtu: "9000"

    cinder-api:
      config:
        files:
          /etc/cinder/cinder.conf: |
            [DEFAULT]
            enabled_backends = ceph
            [ceph]
            volume_driver = cinder.volume.drivers.rbd.RBDDriver
            rbd_pool = volumes

    grafana:
      enabled: false

    prometheus:
      workload:
        storage:
          size: 500Gi
          storageClassName: gp3
```

---

## 9. Reconciliation ordering and gates

The operator must respect the ordering constraints that exist today, or it will reproduce the failure modes documented in the Bork3 guide.

**Install**

1. `PCDUnderlay` reconciles prerequisites by applying Helm releases and manifests directly. `PrerequisitesMet=True` is a hard gate — no `PCDInstallation` proceeds without it.
2. `PCDInstallation` provisions the customer record, seeds identity and credential state, runs the certificate/metadata bootstrap, then installs the infra-region chart. Gate: infra region `Available=True`.
3. `PCDRegion` installs only after its `PCDInstallation` is Available, because the region reads installation-level credential state while building its own deployment configuration.

**Upgrade**

Same order — infra region first, then regions, mirroring the documented `upgrade-kdu-region.sh` sequence. The operator should refuse a `PCDRegion` release ahead of its `PCDInstallation` release rather than attempting it and failing mid-chart.

**Delete**

Strict reverse order, enforced by finalizers:

1. `PCDRegion` finalizers block `PCDInstallation` deletion until all child regions are gone.
2. `PCDInstallation` teardown runs only after that, and the adapter performs the backend's own two-stage cleanup in the required order.
3. `spec.protection.preventDeletion` blocks the whole chain at the webhook.

This is the single highest-value thing the operator buys us over the current scripts: the ordering constraint stops being a runbook step that someone can skip.

**What the adapter must absorb**

Because Bork is hidden, its failure modes become the operator's responsibility rather than the user's. Three that come straight out of the Bork3 runbook and must not surface as CRD-level mysteries:

- A region stuck mid-teardown after a backend restart is re-driven by the controller, not by a human re-issuing a delete.
- Partial installs whose backend records are missing required keys are reconciled or reported as a clear `Degraded` condition with an actionable message — not a raw HTTP 500.
- A single malformed backend record must not break listing for the whole fleet. The controller reconciles per-object and degrades one CR, never all of them.

---

## 10. Decision log and remaining open questions

### Resolved

| Question | Decision |
|---|---|
| Wrap Bork, or refactor its reconciliation loop into the operator now? | **Wrap it, and hide it completely.** Bork is a backend behind an adapter interface (§2.1). Refactoring it toward native reconciliation happens behind that interface, incrementally, without an API break. |
| Adopt or supersede the `bork.pf9.io` group (`Aim`, `TargetCluster`)? | **Neither — they are not part of this API.** They remain internal to Bork. No collision, because the operator never exposes cluster placement. |
| Multi-cluster: fleet controller or per-dataplane controller? | **Per-cluster, single-cluster only.** Operator and DUs always share one cluster. `targetCluster` and `Aim` removed from all three kinds. |
| Split `PCDInstallation` into customer + infra region? | **No.** 1:1 stands. `PCDInstallation` keeps both roles. |
| Bork version skew across the fleet? | **Bork3 required.** The adapter is single-path, enforced by a `PrerequisitesMet` preflight check. |
| Cluster-level Vault after noconsul? | **Removed.** The noconsul path drops Vault along with Consul. Vouch hostagent PKI moves to cert-manager, surfaced as `certificates.hostPKIIssuer`. |
| Where does host-side lifecycle live? | **A future `PCDHostPool` kind**, out of scope for the initial implementation. Not stubbed in `PCDRegion` in the meantime. |

### Still open

1. **Config override validation.** `config.ini` and `config.files` can produce a service that starts cleanly and then misbehaves. Decide whether the operator validates against a per-component schema, lints known-dangerous keys, or accepts anything and relies on status reporting. Worth resolving before the first customer writes a `cinder.conf` by hand.
2. **Adapter interface shape.** The interface should be expressed in terms of intent (`EnsureInstallation`, `EnsureRegion`, `Teardown`, `Observe`) rather than Bork's verbs. Getting this wrong is the main way the "refactor Bork behind the interface later" plan quietly fails. Worth a short design pass of its own before code.
3. **Host CA lifecycle.** With cert-manager as the hostagent PKI root, the private CA's own provisioning, rotation, and trust distribution to hosts becomes an operator concern. Rotating it invalidates every issued hostagent cert, so the rotation story needs designing rather than inheriting. This overlaps `PCDHostPool` (§11) and may be better resolved there.
4. **Backpressure on fleet-wide upgrades.** Nothing in the API currently limits how many customers or regions upgrade concurrently. A cluster-level concurrency budget on `PCDUnderlay` is probably needed before this drives the SaaS platforms.
5. **Considerations for migration of existing PCD clusters to operator-managed installations.** Every cluster the operator will manage already exists and is already running PCD. Adoption is therefore the normal case, not the exception, and the API above is written as though it were greenfield. Specifically unresolved:
   - **Helm release ownership.** Prerequisites like cert-manager, ingress-nginx, and the CSI drivers are already installed and working. Does the underlay reconciler adopt those releases in place (annotating them and leaving them running), or reinstall them under its own ownership? Adoption risks inheriting drift the operator did not create; reinstallation means churning components that are currently healthy on a live platform.
   - **Reconstructing spec from running state.** An adopted cluster's actual configuration is the accumulation of years of manual overrides. Producing a `PCDUnderlay` / `PCDInstallation` / `PCDRegion` set that renders to what is *already deployed* is a discovery problem. A `--dry-run`-style differ that reports where a proposed spec diverges from live state is probably a prerequisite tool, not a nice-to-have.
   - **First-reconcile safety.** The most dangerous moment is the first reconcile after adoption, where any gap between reconstructed spec and reality becomes an unintended change to a production region. `spec.paused` plus a plan/diff mode that reports without applying would let adoption be verified before it is enacted.
   - **Partial adoption.** Whether a cluster can be adopted region by region, or must be adopted wholesale. Incremental is much safer operationally but means the operator has to tolerate regions it does not own sitting alongside ones it does.
   - **Rollback.** What "un-adopting" looks like if adoption goes wrong mid-way, and whether removing the CRs can be made to leave the underlying deployment untouched.
   - **Database migration onto the MariaDB operator.** Existing regions run Percona PXC or a local MySQL, both in-cluster. Each must land on an operator-managed MariaDB object before adoption (§4.6). `bootstrapFrom` covers the restore mechanics; the open part is whether an already-running in-cluster instance can be adopted in place, which is the same take-ownership question as the Helm releases above.

---

## 11. Deferred: `PCDHostPool`

Out of scope for the initial implementation, recorded here so the gap is explicit and nothing gets half-modelled into `PCDRegion` in the meantime.

The kind would cover the region-scoped but non-Kubernetes side of a PCD deployment:

- host onboarding (`pcdctl prep-node`, host agent and `pf9-*` package installation, CA trust distribution)
- role assignment (hypervisor, glance, designate, and so on)
- host upgrade orchestration and batching — the `UPGRADEHOSTS` path, which today is a separate manual step after a region upgrade completes
- cluster blueprint / host configuration alignment
- host-level health and drain semantics

Likely shape: namespaced, `regionRef` to a `PCDRegion`, a host selector or explicit inventory, a role set, and an upgrade strategy block. Until it exists, hosts continue to be managed by `pcdctl` and the existing `UPGRADEHOSTS` call, outside the operator's reconciliation.

One thing to settle when it is specced: whether `PCDRegion` should refuse to report `Ready` while its host pool is mid-upgrade, or whether the two lifecycles stay fully independent. The current region-then-hosts upgrade sequence suggests a dependency exists, and it is easier to model it deliberately than to discover it during an incident.

---

## 12. Per-component reconciliation state machine

§9 covers ordering between the three kinds. This section covers the level below it: the state each individual component moves through during an install or upgrade of a DU, what the operator observes to know a transition happened, and what may safely be undone when one fails. It is the contract the eventual-consistency logic is written against.

### 12.1 Why per-component, and not per-region

Today a DU install or upgrade is a single Kubernetes Job (`du-install-*` / `du-upgrade-*`) whose outcome collapses to one region-level task state — `waiting_apps`, then `ready` or `error`. That is too coarse to reconcile against. An `error` tells you the walk stopped; it does not tell you which chart group it stopped in, whether the failed component's `init-region` job had already committed a schema migration, or whether re-running the walk from the top is safe.

The operator needs per-component state for three reasons:

1. **Detection.** A failure inside one chart's init job is currently found by reading Job logs. It should be a condition on an object.
2. **Resumption.** A retry that re-walks all thirteen groups from the start is expensive and, for non-idempotent steps, dangerous. Resuming at the failed component requires knowing where it stopped.
3. **Back-out.** Whether a failed component can be reverted depends entirely on which component it is. That decision cannot be made at region granularity.

### 12.2 The chart walk this models

The workload region's OpenStack services ship as numbered chart groups, `charts/NNN_<component>/`. The runner walks groups in order and waits for every chart in a group to complete before starting the next. That numbering has grown to thirteen serial groups, and PCD-8676 is open specifically because many of those barriers encode directory numbering rather than real dependency — barbican waiting on nova, monitoring waiting on storage.

Two consequences for this design:

- **Group membership is not the dependency graph.** The operator should model each component's *declared* dependencies (what it actually needs to exist) rather than inheriting the numeric barrier. That makes the walk narrower and, more importantly, makes `Pending` a meaningful state: a component is Pending because a named dependency is unmet, not because a directory sorts later.
- **Parallelism within a group is conditional today** and has failed open. REQ-8931 records `PARALLEL` arriving empty in the du-install pod, silently dropping the walk onto the sequential path. Any parallelism the operator relies on must be explicit in the CR and asserted at preflight, never inferred from an environment variable.

### 12.3 States

| State | Entered when | Observable signal | Success exit |
|---|---|---|---|
| `Absent` | No Helm release exists for the component | `helm status` returns not-found | → `Pending` when the component is enabled |
| `Pending` | Enabled, desired version resolved, dependencies not yet satisfied | Declared dependency set evaluated against live state, including the component's MariaDB `Database`/`Grant` objects reporting Ready | → `Preflight` when all deps satisfied |
| `Preflight` | Dependencies satisfied | Drift check, credential/secret presence, quota and capacity check | → `Applying` when clean |
| `Applying` | Preflight clean | Helm release in `pending-install` / `pending-upgrade` | → `Initializing` when release reaches `deployed` |
| `Initializing` | Chart applied | `<component>-init` / `<component>-ks-user` Job status | → `PostConfiguring` on Job `Complete` |
| `PostConfiguring` | Init complete | Keystone catalog entries, endpoints, quota calls reconciled | → `RollingOut` when catalog matches desired |
| `RollingOut` | Post-config complete | Deployment `readyReplicas == desired` for every workload | → `Ready` |
| `Ready` | All of the above | Observed version == desired, all probes passing | → `Pending` when desired changes |
| `Degraded` | Was `Ready`, readiness subsequently lost | Probe failures with release still `deployed` | → `Pending` to re-apply |
| `Faulted` | Any step failed or exceeded its deadline | Classified reason (§12.4) | → `BackingOut` |
| `BackingOut` | Fault classified as recoverable | Rollback executed per safety class (§12.5) | → `Pending` to retry |
| `Quarantined` | Back-out unsafe or back-out itself failed | Terminal until a human acts | — |

`Ready` is the only stable state. Everything else is either transient or explicitly awaiting a decision, which is what makes "is this DU converged?" answerable by inspection rather than by reading logs.

### 12.4 Fault classification

Recovery differs by fault class, so classification has to happen before any recovery action. Four classes, each drawn from a failure that has actually occurred:

**Transient.** The operation failed for a reason unrelated to desired state — image pull, API server 503 during validation (PMK-6856), a Job OOM that a resource bump fixes (terrakube-init, PCD-8181). Recovery: retry in place with backoff, no rollback. Bounded attempt count, then escalate to `Quarantined`.

**Blocked.** The step cannot complete because something it needs does not exist yet, and it will wait indefinitely. This is the class that produced the REQ-8931 deadlock: cinder's post-install `openstack quota set` retried forever against a nova compute endpoint that would never appear, because nova was queued behind cinder in the same sequential walk. Recovery: **do not retry**. Name the unmet dependency, apply a deadline, and fail into `Faulted` rather than looping. Every wait in `Pending` and `PostConfiguring` needs a deadline for exactly this reason — an unbounded retry loop is indistinguishable from progress, and that is what made this a deadlock rather than an error.

**Drift conflict.** The Helm upgrade is rejected because live state was changed out of band. The cinder-api case is the canonical one: a manual `kubectl patch` set a `tcpSocket` liveness probe, Helm's last-applied annotation never learned about it, and the next upgrade merged its own `httpGet` handler on top, producing an object with two handlers that the API server refused outright. Recovery: reconcile the drift first — adopt the live value into the spec or revert it — then re-apply. Retrying unchanged will fail identically every time. This class is why `Preflight` exists as a distinct state rather than being folded into `Applying`: drift is cheaper to find before dispatch than to diagnose after rejection.

**Partial commit.** The step failed after committing an irreversible side effect — a schema migration partly applied, a Keystone service user created, a region record written. Recovery depends entirely on the safety class below, and this is the class most likely to end in `Quarantined`.

### 12.5 Rollback safety classes

This is the part that cannot be generic, and the reason `BackingOut` consults a per-component property rather than always running `helm rollback`.

| Class | Components | Back-out |
|---|---|---|
| **A — Stateless** | pf9-nginx, memcached, horizon, clarity, the exporters, blackbox, grafana | `helm rollback` is safe and sufficient. Automatic. |
| **B — Stateless with catalog side effects** | resmgr, sidekickserver, pf9-notifications, preference-store, appcatalog | Roll back the workload. Keystone catalog entries persist, which is harmless because catalog writes are idempotent and re-converged in `PostConfiguring`. Automatic. |
| **C — Schema-bearing** | keystone, nova, neutron, cinder, glance, placement, barbican, octavia, designate, masakari, heat, watcher | **Do not roll back.** DB migrations are forward-only; `helm rollback` restores pods but not schema, leaving an older image against a newer database. Roll *forward* to a corrected version, or `Quarantine`. |
| **D — Quorum / stateful** | mariadb (Galera), OVN NB/SB, rabbitmq | Back-out is a topology operation, not a chart operation. Never automatic. Always `Quarantine` with a named runbook. |

The class belongs in the component catalog alongside the chart reference, not in controller code — it is data about the component, and getting it wrong for one component should not require an operator release to correct.

### 12.6 What the charts must guarantee

The state machine is only enforceable if the underlying steps are re-runnable. Three requirements fall out, and they are requirements on the charts rather than on the operator:

- **`init-region` jobs must be idempotent.** Re-running one on an already-initialised component must be a no-op, not an error. Resumption depends on this. Moving database creation, users and grants onto MariaDB `Database`/`User`/`Grant` objects (§4.6) removes the largest non-idempotent chunk of these jobs; what remains is schema migration, which each service must make re-runnable itself.
- **Post-install OpenStack calls must be bounded.** Every `openstack` call in a post-install hook needs a deadline and a distinguishable "dependency absent" exit code. Unbounded retry is the REQ-8931 failure.
- **Nothing may be configured out of band.** Any approved manual patch must land in the chart values in the same change, or it becomes a drift conflict at the next upgrade. CM-1178 is the worked example of the cost when it doesn't.

Where a chart doesn't meet these today, the operator can still model the component — it just means that component's realistic recovery path is `Quarantined` rather than automatic, and the gap is visible rather than latent.

### 12.7 Status surface

Per-component state is reported through the existing `status.components[]` array from §5, extended with the fields the machine needs:

```go
type ComponentStatus struct {
    Name string `json:"name"`

    // Phase is the §12.3 state. This enum is the single source of truth for
    // component phase across all three kinds.
    // +kubebuilder:validation:Enum=Absent;Pending;Preflight;Applying;Initializing;PostConfiguring;RollingOut;Ready;Degraded;Faulted;BackingOut;Quarantined
    Phase string `json:"phase"`

    // +optional
    ReadyReplicas int32 `json:"readyReplicas,omitempty"`
    // +optional
    DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

    // FaultClass is set whenever Phase is Faulted, BackingOut or Quarantined.
    // +kubebuilder:validation:Enum=Transient;Blocked;DriftConflict;PartialCommit
    // +optional
    FaultClass string `json:"faultClass,omitempty"`

    // BlockedOn names the unmet dependency when FaultClass is Blocked. Without
    // this a deadlock is indistinguishable from slow progress.
    // +optional
    BlockedOn []string `json:"blockedOn,omitempty"`

    // RollbackClass is the component's declared back-out safety class (A-D).
    // +optional
    RollbackClass string `json:"rollbackClass,omitempty"`

    // Attempts is the retry count for the current desired version. Resets when
    // the desired version changes, not when a retry succeeds.
    // +optional
    Attempts int32 `json:"attempts,omitempty"`

    // ObservedVersion is what is actually running; compare against desired to
    // answer "did this component take the upgrade?"
    // +optional
    ObservedVersion string `json:"observedVersion,omitempty"`

    // +optional
    Message string `json:"message,omitempty"`
    // +optional
    LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}
```

A `PCDRegion` is `Ready` only when every enabled component is `Ready`. Any component in `Quarantined` makes the region `Degraded` rather than `Error`, because the rest of the region is still serving — which is the distinction the current single region-level task state cannot express.

### 12.8 Open questions from this section

1. **Where does the dependency graph live?** Modelling declared dependencies rather than numeric groups is the right call, but the graph has to come from somewhere. PCD-8676 is deriving the real dependencies already; the operator should consume that output rather than maintain a second copy.
2. **Resumption granularity while Bork owns the walk.** The adapter dispatches a whole install/upgrade Job today. Per-component resumption implies either the walk becomes restartable at a named component, or the operator drives components individually. That is a Bork refactor, and it is the largest single piece of work implied by this section.
3. **Deadline values.** Every wait needs one, and none are known. They should be per-component data in the release matrix, seeded from observed p99 durations rather than guessed.
