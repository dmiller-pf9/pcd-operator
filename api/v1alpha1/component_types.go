package v1alpha1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ComponentSpec is the single unit of customization, keyed by component name in
// a map so that per-component patches are additive and do not require list
// merge keys. The same type is override layer 4 (as componentDefaults) and
// layer 5 (as components[name]) in docs/DESIGN.md §4.1.
type ComponentSpec struct {
	// Enabled=false removes the component entirely from the deployment; it does
	// not scale it to zero. Nil means "not specified at this layer", which is
	// what lets the merge engine tell an unset value from an explicit false.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ImageTag overrides just the tag for this component's image(s), leaving the
	// rest of the release matrix intact.
	// +optional
	ImageTag string `json:"imageTag,omitempty"`

	// Image fully overrides the image reference (registry/repo:tag).
	// +optional
	Image string `json:"image,omitempty"`

	// ChartVersion overrides the subchart version where the component is
	// packaged as its own chart.
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`

	// Workload shapes the generated Kubernetes resources.
	// +optional
	Workload *WorkloadOverride `json:"workload,omitempty"`

	// Config carries service-level configuration file overrides.
	// +optional
	Config *ComponentConfig `json:"config,omitempty"`

	// HelmValues is free-form passthrough merged into the component's values
	// subtree. The escape hatch for anything the typed fields do not reach.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	HelmValues *apiextensionsv1.JSON `json:"helmValues,omitempty"`

	// StrategicMergePatches are applied to rendered manifests post-templating.
	// Deliberately last-resort: every use is a gap in the typed API above and
	// should be tracked as such. This is override layer 6, the highest.
	// +optional
	StrategicMergePatches []apiextensionsv1.JSON `json:"strategicMergePatches,omitempty"`
}

// WorkloadOverride shapes the Kubernetes objects a component renders to.
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
	Storage *StorageOverride `json:"storage,omitempty"`
}

// ComponentConfig reaches into the service's own configuration files.
type ComponentConfig struct {
	// INI-style overrides, the common case for OpenStack services, keyed
	// file -> section -> key -> value:
	//
	//	ini:
	//	  nova.conf:
	//	    DEFAULT:
	//	      cpu_allocation_ratio: "8.0"
	//
	// +optional
	INI map[string]map[string]map[string]string `json:"ini,omitempty"`

	// Files is whole-file replacement, keyed by in-container path. Use
	// sparingly — it opts the file out of all future release-matrix updates.
	// +optional
	Files map[string]string `json:"files,omitempty"`

	// FilesFrom pulls file content from ConfigMaps or Secrets instead of
	// inlining it.
	// +optional
	FilesFrom []FileSource `json:"filesFrom,omitempty"`

	// Structured is deep-merged config for components whose configuration is
	// not INI (grafana, prometheus, fluent-bit, ...).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Structured *apiextensionsv1.JSON `json:"structured,omitempty"`

	// PolicyOverrides carries OpenStack policy.yaml / policy.json RBAC rules.
	// +optional
	PolicyOverrides map[string]string `json:"policyOverrides,omitempty"`
}

// PDBSpec configures a PodDisruptionBudget. Exactly one of MinAvailable and
// MaxUnavailable may be set; the webhook in milestone 006 rejects both.
type PDBSpec struct {
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// HPASpec configures a HorizontalPodAutoscaler for a component that can scale
// horizontally. Size (docs/DESIGN.md §4.5) supplies the defaults.
type HPASpec struct {
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	MaxReplicas int32 `json:"maxReplicas"`

	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`

	// Behavior is passed through to the generated HPA unchanged.
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// StorageOverride shapes a component's persistent volume claim.
type StorageOverride struct {
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`

	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// FileSource projects configuration from a ConfigMap or Secret into a
// component's filesystem. Exactly one of ConfigMapRef and SecretRef is expected.
type FileSource struct {
	// MountPath is the in-container directory the source is projected into.
	MountPath string `json:"mountPath"`

	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// +optional
	Items []corev1.KeyToPath `json:"items,omitempty"`
}
