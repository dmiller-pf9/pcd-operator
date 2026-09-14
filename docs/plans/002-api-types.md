# Milestone 002 — API types, deepcopy, CRD generation

> **For agentic workers:** REQUIRED SUB-SKILL: use `superpowers:subagent-driven-development`
> or `superpowers:executing-plans` to implement this plan task by task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Define `PCDUnderlay`, `PCDInstallation` and `PCDRegion` and every type
they reference, generate deepcopy functions and CRD manifests, and prove the
generated output matches SPEC §4.1.

**Architecture:** One Go package, `api/v1alpha1`, split by subject rather than by
kind — the override engine, the external-service bindings and the status surface
are shared verbatim by all three kinds (DESIGN §4, §5), so they live in shared
files and each kind file holds only its own spec. Nothing here has behaviour:
the tests assert the *shape* of the generated artifacts, which is the only thing
that can be wrong at this stage and the only thing later milestones depend on.

**Tech Stack:** Go 1.25, controller-gen v0.22.0, controller-runtime v0.25.1.

**Requirements covered:** SPEC §4.1 API-001, API-002, API-003, API-005.
Invariants INV-1, INV-6, INV-7, INV-8.

---

## Scope notes — read before starting

**API-004 is declared here and enforced in milestone 006. This is settled — do
not re-litigate it.** SPEC §8 reaches API-004 twice, and both are correct:
milestone 002 owns `API-001…005` as a range because it creates the fields the
requirement is about, and milestone 006 lists it explicitly because its
acceptance criterion in §4.1 is a *webhook* test. The split is: this plan adds
`shortName`, `regionName` and `installationRef` and documents them as immutable
in their Go doc comments; milestone 006 enforces that immutability at admission.

**Do not** add a CEL `+kubebuilder:validation:XValidation` immutability rule to
"finish" API-004 here. That would put enforcement somewhere the spec's
acceptance criterion does not look, and milestone 006 would then either
duplicate it or find its own test already passing for the wrong reason. This is
not an open question and is not a reason to stop and ask.

**No component catalog.** DESIGN §6, §7 and §8 each list a component catalog
table. Those are data for milestone 004 (release matrix) and milestone 005
(rollback classes), not types. `spec.components` is `map[string]ComponentSpec`
with no enum of valid keys.

**Two design comments must be reworded, not transcribed.** `docs/DESIGN.md`
contains backend vocabulary in two Go doc comments. Copying them verbatim into
`api/` fails `make lint-leak`. The replacement wording is given in the tasks that
create those types (2.4 `PlatformSpec`, 2.9 `PCDInstallationSpec.ShortName`).

## Dependencies introduced

CLAUDE.md requires stopping before introducing a dependency. This milestone
introduces the controller-runtime stack. **The human approved this set when the
plan was commissioned; an executing agent must not add to it.**

| Module | Version | Why |
|---|---|---|
| `k8s.io/apimachinery` | v0.37.0 | `metav1`, `intstr`, `resource` |
| `k8s.io/api` | v0.37.0 | `corev1`, `autoscalingv2` |
| `k8s.io/apiextensions-apiserver` | v0.37.0 | `apiextensionsv1.JSON` for the passthrough fields (INV-8) |
| `sigs.k8s.io/controller-runtime` | v0.25.1 | `scheme.Builder` |
| `sigs.k8s.io/yaml` | (transitive, promoted to direct) | reading generated CRDs in tests |

`k8s.mariadb.com/v1alpha1` is **not** a dependency. SPEC API-005 permits its
types, but DESIGN §4.3 reaches the MariaDB API through `apiextensionsv1.JSON`
passthrough (INV-8), so nothing in `api/` references it. Adding it would be a new
dependency for no benefit.

## File structure

| File | Responsibility |
|---|---|
| `api/v1alpha1/groupversion_info.go` | Group, version, scheme builder |
| `api/v1alpha1/release_types.go` | `ReleaseSpec` and artifact resolution (DESIGN §4.2) |
| `api/v1alpha1/component_types.go` | `ComponentSpec` and workload/config shaping (DESIGN §4.2, §4.4) |
| `api/v1alpha1/common_types.go` | `PCDSize`, policy and shared leaf types (DESIGN §4.4, §4.5) |
| `api/v1alpha1/networking_types.go` | Infra and region networking (DESIGN §4.4) |
| `api/v1alpha1/database_types.go` | `DatabaseBinding` and the MariaDB wrapper leaves (DESIGN §4.3, §4.4) |
| `api/v1alpha1/externalservices_types.go` | `ExternalServices` and the remaining bindings (DESIGN §4.3, §4.4) |
| `api/v1alpha1/status_types.go` | `CommonStatus`, `ComponentStatus`, the two phase enums (DESIGN §5, §12.7) |
| `api/v1alpha1/pcdunderlay_types.go` | `PCDUnderlay` + `PlatformSpec` (DESIGN §6) |
| `api/v1alpha1/pcdinstallation_types.go` | `PCDInstallation` (DESIGN §7) |
| `api/v1alpha1/pcdregion_types.go` | `PCDRegion` + `RegionFeatures` (DESIGN §8) |
| `api/v1alpha1/zz_generated.deepcopy.go` | generated — never hand-edited |
| `api/v1alpha1/helpers_test.go` | shared `assertJSONKeys` test helper |
| `api/v1alpha1/apishape_test.go` | json-tag and reachability tests (API-005) |
| `api/v1alpha1/passthrough_test.go` | INV-8 round-trip |
| `internal/crdcheck/crd_test.go` | assertions on generated CRDs (API-001, 002, 003, INV-7) |
| `config/crd/bases/*.yaml` | generated — never hand-edited |

---

### Task 2.1 — Group, version and the scheme builder

**Requirement:** SPEC §4.1 `API-001`
**Files:**
- Modify: `go.mod`
- Create: `api/v1alpha1/groupversion_info.go`
- Test: `api/v1alpha1/groupversion_test.go`
- Test: `api/v1alpha1/helpers_test.go` (shared `assertJSONKeys`, used by every later task)
**Depends on:** milestone 001 complete

**RED**

Test file: `api/v1alpha1/groupversion_test.go`
Test name: `TestGroupVersion`

```go
package v1alpha1

import "testing"

func TestGroupVersion(t *testing.T) {
	if got, want := GroupVersion.Group, "install.pcd.platform9.com"; got != want {
		t.Errorf("group = %q, want %q", got, want)
	}
	if got, want := GroupVersion.Version, "v1alpha1"; got != want {
		t.Errorf("version = %q, want %q", got, want)
	}
}
```

Run: `make test-one T=TestGroupVersion`

Expected failure:
```
api/v1alpha1/groupversion_test.go:6:16: undefined: GroupVersion
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Add the approved dependency set**

```bash
go get k8s.io/apimachinery@v0.37.0
go get k8s.io/api@v0.37.0
go get k8s.io/apiextensions-apiserver@v0.37.0
go get sigs.k8s.io/controller-runtime@v0.25.1
go get sigs.k8s.io/yaml
```

- [ ] **Step 3: Create `api/v1alpha1/groupversion_info.go`**

```go
// Package v1alpha1 contains the API schema for install.pcd.platform9.com/v1alpha1:
// PCDUnderlay, PCDInstallation and PCDRegion.
//
// This package is the only supported interface to the operator. No backend
// vocabulary appears in it — see docs/SPEC.md §3 INV-1, enforced by
// `make lint-leak`.
//
// +kubebuilder:object:generate=true
// +groupName=install.pcd.platform9.com
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group and version for this API (SPEC API-001).
	GroupVersion = schema.GroupVersion{Group: "install.pcd.platform9.com", Version: "v1alpha1"}

	// SchemeBuilder registers this API's types with a runtime.Scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds this API's types to a runtime.Scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
```

- [ ] **Step 4: Create `api/v1alpha1/helpers_test.go`**

Every later task in this milestone asserts the JSON tag spelling of a struct,
because a json tag *is* the CRD field name. The helper lives in its own file,
created here, so that no task has to depend on another task purely for a test
helper.

```go
package v1alpha1

import (
	"encoding/json"
	"testing"
)

// assertJSONKeys marshals v and fails if any of keys is absent from the result.
func assertJSONKeys(t *testing.T, v any, keys ...string) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range keys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %s", k, b)
		}
	}
}
```

- [ ] **Step 5: Run the test and observe green**

Run: `make test-one T=TestGroupVersion`
Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum api/v1alpha1/
git commit -m "feat(api): group install.pcd.platform9.com, version v1alpha1 (API-001)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run — produces nothing yet, no root object exists
- [ ] Invariants checked: INV-1, INV-6 (`make lint-leak` now scans a non-empty `api/`)

**Do not:** add any module beyond the five listed in "Dependencies introduced".
**Do not:** run `kubebuilder create api` — it rewrites the Makefile from milestone 001.

---

### Task 2.2 — Release and artifact resolution types

**Requirement:** SPEC §4.1 `API-005`; DESIGN §4.2
**Files:**
- Create: `api/v1alpha1/release_types.go`
- Test: `api/v1alpha1/release_test.go`
**Depends on:** 2.1

**RED**

Test file: `api/v1alpha1/release_test.go`
Test name: `TestReleaseSpecJSONTags`

```go
package v1alpha1

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestReleaseSpecJSONTags(t *testing.T) {
	spec := ReleaseSpec{
		MatrixVersion: "2026.4",
		Source:        &ReleaseMatrixSource{OCIRef: "oci://quay.io/platform9/pcd-release-matrix:2026.4"},
		ChartOverride: &ChartRef{
			URL:           "https://example.invalid/chart.tgz",
			Version:       "1.2.3",
			PullSecretRef: &corev1.LocalObjectReference{Name: "pull"},
		},
		ImageRegistry: &RegistrySpec{Host: "registry.example.invalid", PathPrefix: "pcd"},
	}

	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"matrixVersion", "source", "chartOverride", "imageRegistry"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in %s", key, b)
		}
	}
}

func TestReleaseSpecOmitsEmptyOptionalFields(t *testing.T) {
	b, err := json.Marshal(ReleaseSpec{MatrixVersion: "2026.4"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"matrixVersion":"2026.4"}`; string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}
```

Run: `make test-one T=TestReleaseSpec`

Expected failure:
```
api/v1alpha1/release_test.go:12:10: undefined: ReleaseSpec
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/release_types.go`**

```go
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// ReleaseSpec pins the collective release and allows per-component escape
// hatches. It is override layer 1 — the lowest — in the precedence order
// documented in docs/DESIGN.md §4.1.
type ReleaseSpec struct {
	// MatrixVersion is the collective PCD release, e.g. "2026.4",
	// "2026.4-patch2", "2026.8". Resolved via Source into a concrete artifact
	// set.
	// +kubebuilder:validation:Required
	MatrixVersion string `json:"matrixVersion"`

	// Source of the release matrix. Defaults to the operator's embedded matrix
	// for the version it shipped with; override for airgapped or custom-build
	// cases.
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

// ReleaseMatrixSource locates the release matrix. Exactly one field is expected;
// the webhook in milestone 006 enforces that.
type ReleaseMatrixSource struct {
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// OCIRef is an OCI artifact reference, e.g.
	// "oci://quay.io/platform9/pcd-release-matrix:2026.4".
	// +optional
	OCIRef string `json:"ociRef,omitempty"`

	// +optional
	URL string `json:"url,omitempty"`
}

// ChartRef identifies a chart artifact.
type ChartRef struct {
	// +optional
	OCIRef string `json:"ociRef,omitempty"`

	// +optional
	URL string `json:"url,omitempty"`

	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}

// RegistrySpec redirects component images at a different registry.
type RegistrySpec struct {
	Host string `json:"host"`

	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T=TestReleaseSpec`
Expected: `PASS` (2 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): release and artifact resolution types"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6

**Do not:** add a `chartRepository` or credentials-by-value field. Pull secrets are
references only.

---

### Task 2.3 — Component override and workload shaping types

**Requirement:** SPEC §4.1 `API-005`; DESIGN §4.2, §4.4
**Files:**
- Create: `api/v1alpha1/component_types.go`
- Test: `api/v1alpha1/component_test.go`
**Depends on:** 2.1

**RED**

Test file: `api/v1alpha1/component_test.go`
Test name: `TestComponentSpecJSONTags`

```go
package v1alpha1

import (
	"encoding/json"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestComponentSpecJSONTags(t *testing.T) {
	enabled := false
	replicas := int32(3)
	size := resource.MustParse("20Gi")
	minAvail := intstr.FromInt32(1)

	spec := ComponentSpec{
		Enabled:      &enabled,
		ImageTag:     "v1.2.3",
		Image:        "registry.example.invalid/nova:v1.2.3",
		ChartVersion: "0.4.1",
		Workload: &WorkloadOverride{
			Replicas:            &replicas,
			Resources:           map[string]corev1.ResourceRequirements{"*": {}},
			NodeSelector:        map[string]string{"role": "control"},
			Tolerations:         []corev1.Toleration{{Key: "dedicated"}},
			PodDisruptionBudget: &PDBSpec{MinAvailable: &minAvail},
			Autoscaling:         &HPASpec{MaxReplicas: 8},
			PriorityClassName:   "system-cluster-critical",
			Storage:             &StorageOverride{Size: &size},
		},
		Config: &ComponentConfig{
			INI: map[string]map[string]map[string]string{
				"nova.conf": {"DEFAULT": {"cpu_allocation_ratio": "8.0"}},
			},
			Files:           map[string]string{"/etc/neutron/x.ini": "[ml2]\n"},
			FilesFrom:       []FileSource{{MountPath: "/etc/extra"}},
			PolicyOverrides: map[string]string{"policy.yaml": "{}"},
		},
		HelmValues:            &apiextensionsv1.JSON{Raw: []byte(`{"key":"value"}`)},
		StrategicMergePatches: []apiextensionsv1.JSON{{Raw: []byte(`{"spec":{}}`)}},
	}

	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"enabled", "imageTag", "image", "chartVersion",
		"workload", "config", "helmValues", "strategicMergePatches",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in %s", key, b)
		}
	}
}

// API-013 depends on Enabled being a pointer: the merge engine in milestone 003
// must distinguish "not set at this layer" from "explicitly false".
func TestComponentSpecEnabledIsNilable(t *testing.T) {
	var spec ComponentSpec
	if spec.Enabled != nil {
		t.Fatalf("zero ComponentSpec.Enabled = %v, want nil", spec.Enabled)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "{}" {
		t.Errorf("zero ComponentSpec marshals to %s, want {}", b)
	}
}
```

Run: `make test-one T=TestComponentSpec`

Expected failure:
```
api/v1alpha1/component_test.go:19:10: undefined: ComponentSpec
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/component_types.go`**

```go
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
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T=TestComponentSpec`
Expected: `PASS` (2 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): component override and workload shaping types"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6

**Do not:** make `Enabled` a plain `bool`. API-013 and the layer-4/5 merge in
milestone 003 both need to distinguish unset from false, and a plain bool makes
that undecidable.
**Do not:** add schema validation to `HelmValues`, `Structured` or
`StrategicMergePatches` — they are passthrough by design.

---

### Task 2.4 — Size, policy and shared leaf types

**Requirement:** SPEC §4.3 `API-020` (enum only); DESIGN §4.4, §4.5
**Files:**
- Create: `api/v1alpha1/common_types.go`
- Test: `api/v1alpha1/common_test.go`
**Depends on:** 2.1

**RED**

Test file: `api/v1alpha1/common_test.go`
Test name: `TestPCDSizeValues`

```go
package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// API-020: spec.size accepts exactly these five values. The CRD enum is asserted
// in internal/crdcheck; this asserts the Go constants agree with it.
func TestPCDSizeValues(t *testing.T) {
	want := []PCDSize{"X-Small", "Small", "Medium", "Large", "X-Large"}
	got := []PCDSize{SizeXSmall, SizeSmall, SizeMedium, SizeLarge, SizeXLarge}

	if len(got) != len(want) {
		t.Fatalf("got %d sizes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("size[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// There is deliberately no Custom size — docs/DESIGN.md §4.5. Omitting size and
// setting componentDefaults says the same thing without a second way to say it.
func TestPCDSizeHasNoCustomValue(t *testing.T) {
	for _, s := range []PCDSize{SizeXSmall, SizeSmall, SizeMedium, SizeLarge, SizeXLarge} {
		if s == "Custom" || s == "custom" {
			t.Errorf("found a Custom size value: %q", s)
		}
	}
}

func TestUpgradePolicyJSONTags(t *testing.T) {
	autoRollback := true
	maxConcurrent := int32(2)
	p := UpgradePolicy{
		Trigger:       "Window",
		Window:        &MaintenanceWindow{Schedule: "0 2 * * SAT", Duration: metav1.Duration{Duration: 4 * time.Hour}},
		MaxConcurrent: &maxConcurrent,
		AutoRollback:  &autoRollback,
	}

	assertJSONKeys(t, p, "trigger", "window", "maxConcurrent", "autoRollback")

	// SPEC §9-3 and §9-4: neither timeout nor a concurrency budget has an
	// agreed value. An unset timeout must stay absent rather than acquire a
	// default nobody chose.
	b, err := json.Marshal(UpgradePolicy{Trigger: "Immediate"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "timeout") {
		t.Errorf("unset timeout should be omitted, got %s", b)
	}
	if strings.Contains(string(b), "maxConcurrent") {
		t.Errorf("unset maxConcurrent should be omitted, got %s", b)
	}
}
```

Run: `make test-one T=TestPCDSize`

Expected failure:
```
api/v1alpha1/common_test.go:11:10: undefined: PCDSize
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/common_types.go`**

```go
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PCDSize expresses the intended scale of a deployment once, instead of
// requiring an operator to hand-tune two dozen components to say the same
// thing. It is override layer 3 in docs/DESIGN.md §4.1: a default, not a
// constraint, so anything set in componentDefaults or components[name] wins
// over it.
//
// The presets themselves are release-matrix data, not code (API-022), so this
// type carries the vocabulary and nothing else.
//
// +kubebuilder:validation:Enum=X-Small;Small;Medium;Large;X-Large
type PCDSize string

const (
	SizeXSmall PCDSize = "X-Small"
	SizeSmall  PCDSize = "Small"
	SizeMedium PCDSize = "Medium"
	SizeLarge  PCDSize = "Large"
	SizeXLarge PCDSize = "X-Large"
)

// TLSSettings configures transport security for an external service binding.
type TLSSettings struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	CASecretRef *corev1.LocalObjectReference `json:"caSecretRef,omitempty"`

	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// IssuerRef names a cert-manager Issuer or ClusterIssuer.
type IssuerRef struct {
	// +kubebuilder:validation:Enum=Issuer;ClusterIssuer
	Kind string `json:"kind"`

	Name string `json:"name"`

	// +kubebuilder:default="cert-manager.io"
	// +optional
	Group string `json:"group,omitempty"`
}

// MaintenanceWindow bounds when an upgrade may start.
type MaintenanceWindow struct {
	// Schedule is a cron expression marking the start of the window.
	Schedule string `json:"schedule"`

	Duration metav1.Duration `json:"duration"`

	// +kubebuilder:default="UTC"
	// +optional
	TimeZone string `json:"timeZone,omitempty"`
}

// UpgradePolicy governs how upgrades roll.
type UpgradePolicy struct {
	// +kubebuilder:validation:Enum=Immediate;Window;Manual
	// +kubebuilder:default=Immediate
	Trigger string `json:"trigger"`

	// +optional
	Window *MaintenanceWindow `json:"window,omitempty"`

	// MaxConcurrent caps how many child objects upgrade at once. On a
	// PCDUnderlay this is the fleet-wide budget.
	//
	// No default is agreed (docs/SPEC.md §9-4). It is nil and unenforced until
	// one is specified — do not invent a value.
	// +optional
	MaxConcurrent *int32 `json:"maxConcurrent,omitempty"`

	// Timeout after which an in-flight operation is treated as Faulted.
	//
	// No default is agreed (docs/SPEC.md §9-3). Deadlines belong in the release
	// matrix as per-component data seeded from observed p99 durations — do not
	// invent a value here.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// AutoRollback permits automatic back-out for components whose rollback
	// safety class allows it. Components of class C (schema-bearing) and D
	// (quorum/stateful) never roll back automatically regardless of this
	// setting — see docs/SPEC.md §3 INV-3.
	// +kubebuilder:default=true
	// +optional
	AutoRollback *bool `json:"autoRollback,omitempty"`
}

// ProtectionSpec prevents accidental deletion. Enforced at the admission
// webhook so the block happens before any teardown work is dispatched.
type ProtectionSpec struct {
	PreventDeletion bool `json:"preventDeletion"`

	// +optional
	Reason string `json:"reason,omitempty"`

	// +optional
	Owner string `json:"owner,omitempty"`
}
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T='TestPCDSize|TestUpgradePolicy'`
Expected: `PASS` (3 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): size, upgrade policy and shared leaf types"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6

**Do not:** add a `Custom` size value (DESIGN §4.5 removed it deliberately).
**Do not:** give `UpgradePolicy.Timeout` or `MaxConcurrent` a
`+kubebuilder:default`. Both are SPEC §9 open decisions — CLAUDE.md "Stop and ask".

---

### Task 2.5 — Networking types

**Requirement:** SPEC §4.1 `API-005`; DESIGN §4.4
**Files:**
- Create: `api/v1alpha1/networking_types.go`
- Test: `api/v1alpha1/networking_test.go`
**Depends on:** 2.1

**RED**

Test file: `api/v1alpha1/networking_test.go`
Test name: `TestNetworkingJSONTags`

```go
package v1alpha1

import (
	"encoding/json"
	"testing"
)

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
	segID := int32(101)
	n := RegionNetworkingSpec{
		OVN:              &OVNSpec{NorthboundDBReplicas: &mtu},
		ProviderNetworks: []ProviderNetwork{{Name: "physnet1", Type: "vlan", SegmentationID: &segID}},
		MTU:              &mtu,
		FloatingIPPools:  []string{"10.1.0.0/24"},
		MetadataService:  &MetadataServiceSpec{},
	}
	assertJSONKeys(t, n, "ovn", "providerNetworks", "mtu", "floatingIPPools", "metadataService")
}
```

Run: `make test-one T=NetworkingSpecJSONTags`

Expected failure:
```
api/v1alpha1/networking_test.go:10:7: undefined: InfraNetworkingSpec
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/networking_types.go`**

```go
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
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T=NetworkingSpecJSONTags`
Expected: `PASS` (2 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): infra and region networking types"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6

**Do not:** add a host-management or host-networking stanza to
`RegionNetworkingSpec`. Host-side lifecycle is deferred to `PCDHostPool`
(SPEC §10) and MUST NOT be stubbed here.

---

### Task 2.6 — Database binding and MariaDB wrapper leaves

**Requirement:** SPEC §4.4 (types only); DESIGN §4.3, §4.4; INV-8
**Files:**
- Create: `api/v1alpha1/database_types.go`
- Test: `api/v1alpha1/database_test.go`
**Depends on:** 2.3, 2.4

**RED**

Test file: `api/v1alpha1/database_test.go`
Test name: `TestDatabaseBindingJSONTags`

```go
package v1alpha1

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDatabaseBindingJSONTags(t *testing.T) {
	replicas := int32(3)
	size := resource.MustParse("100Gi")
	b := DatabaseBinding{
		Provisioning: "Dedicated",
		InstanceRef:  &MariaDBRef{Name: "pcd-db"},
		Topology:     "Galera",
		Replicas:     &replicas,
		Storage:      &MariaDBStorage{Size: &size},
		MyCnf:        map[string]map[string]string{"mysqld": {"max_connections": "500"}},
		Metrics:      &MariaDBMetrics{},
		MaxScale:     &MaxScaleSpec{},
		Schema:       &SchemaPolicy{},
		Backup:       &DatabaseBackupPolicy{Type: "Physical"},
		BootstrapFrom: &apiextensionsv1.JSON{Raw: []byte(`{"s3":{}}`)},
		Template:      &apiextensionsv1.JSON{Raw: []byte(`{"podTemplate":{}}`)},
	}

	assertJSONKeys(t, b,
		"provisioning", "instanceRef", "topology", "replicas", "storage",
		"myCnf", "metrics", "maxScale", "schema", "backup",
		"bootstrapFrom", "template",
	)
}

// DB-002 depends on Topology being exactly these three values. The CRD enum is
// asserted in internal/crdcheck; this pins the vocabulary the wrapper in
// milestone 008 will switch on.
func TestDatabaseBindingTopologyVocabulary(t *testing.T) {
	for _, topology := range []string{"Standalone", "Replication", "Galera"} {
		b := DatabaseBinding{Provisioning: "Dedicated", Topology: topology}
		if b.Topology != topology {
			t.Errorf("topology = %q, want %q", b.Topology, topology)
		}
	}
}
```

Run: `make test-one T=TestDatabaseBinding`

Expected failure:
```
api/v1alpha1/database_test.go:12:7: undefined: DatabaseBinding
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/database_types.go`**

```go
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
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T=TestDatabaseBinding`
Expected: `PASS` (2 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): database binding and MariaDB wrapper leaves"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6, INV-8

**Do not:** add a typed struct for `Template`, `BootstrapFrom` or
`Backup.Storage`. INV-8 forbids re-validating the MariaDB passthrough, and a
typed struct is validation.
**Do not:** add `host`, `port`, `username` or `password` fields. SPEC §10 settled
that arbitrary external endpoints are no longer expressible.
**Do not:** import `k8s.mariadb.com/v1alpha1` — it is not an approved dependency
and nothing here needs it.

---

### Task 2.7 — External service bindings

**Requirement:** SPEC §4.1 `API-005`; DESIGN §4.3, §4.4
**Files:**
- Create: `api/v1alpha1/externalservices_types.go`
- Test: `api/v1alpha1/externalservices_test.go`
**Depends on:** 2.4, 2.6

**RED**

Test file: `api/v1alpha1/externalservices_test.go`
Test name: `TestExternalServicesJSONTags`

```go
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
```

Run: `make test-one T='TestExternalServices|TestCertificateBinding'`

Expected failure:
```
api/v1alpha1/externalservices_test.go:11:8: undefined: ExternalServices
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/externalservices_types.go`**

```go
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
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T='TestExternalServices|TestCertificateBinding'`
Expected: `PASS` (2 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): external service bindings"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6

**Do not:** add a `vault` or `consul` option to `SecretsBinding.Provider`. SPEC §10
settled that they are absent by construction and the enum deliberately gives no
way to reintroduce them.

---

### Task 2.8 — Status surface and the two phase enums

**Requirement:** SPEC §6 `STATE-001`, `STATE-004`, `STATE-010`; INV-7; DESIGN §5, §12.7
**Files:**
- Create: `api/v1alpha1/status_types.go`
- Test: `api/v1alpha1/status_test.go`
**Depends on:** 2.2

**RED**

Test file: `api/v1alpha1/status_test.go`
Test name: `TestComponentPhaseVocabulary`

```go
package v1alpha1

import (
	"testing"
)

// STATE-001: exactly these twelve phases, in this order. The generated CRD enum
// is asserted in internal/crdcheck; this pins the Go constants that the state
// machine in milestone 009 switches on.
func TestComponentPhaseVocabulary(t *testing.T) {
	want := []ComponentPhase{
		"Absent", "Pending", "Preflight", "Applying", "Initializing",
		"PostConfiguring", "RollingOut", "Ready", "Degraded", "Faulted",
		"BackingOut", "Quarantined",
	}
	got := AllComponentPhases()

	if len(got) != len(want) {
		t.Fatalf("got %d phases, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phase[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// STATE-004: exactly these four fault classes.
func TestFaultClassVocabulary(t *testing.T) {
	want := []FaultClass{"Transient", "Blocked", "DriftConflict", "PartialCommit"}
	got := AllFaultClasses()

	if len(got) != len(want) {
		t.Fatalf("got %d fault classes, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("faultClass[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// STATE-010: the operation type vocabulary is neutral. A backend job name here
// would be an INV-1 violation, which is why the set is closed.
func TestOperationTypeVocabulary(t *testing.T) {
	want := []OperationType{"Install", "Upgrade", "Resize", "Teardown"}
	got := AllOperationTypes()

	if len(got) != len(want) {
		t.Fatalf("got %d operation types, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("operationType[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCommonStatusJSONTags(t *testing.T) {
	s := CommonStatus{
		ObservedGeneration: 4,
		Phase:              "Ready",
		AppliedRelease:     &AppliedRelease{MatrixVersion: "2026.4"},
		Components:         []ComponentStatus{{Name: "keystone", Phase: PhaseReady}},
		ReadyComponents:    45,
		DesiredComponents:  45,
		ActiveOperation:    &OperationStatus{Type: OpInstall},
	}
	assertJSONKeys(t, s,
		"observedGeneration", "phase", "appliedRelease", "components",
		"readyComponents", "desiredComponents", "activeOperation",
	)
}
```

Run: `make test-one T='TestComponentPhaseVocabulary|TestFaultClassVocabulary|TestOperationTypeVocabulary|TestCommonStatus'`

Expected failure:
```
api/v1alpha1/status_test.go:11:11: undefined: ComponentPhase
api/v1alpha1/status_test.go:16:9: undefined: AllComponentPhases
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/status_types.go`**

```go
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentPhase is the per-component state machine vocabulary from
// docs/DESIGN.md §12.3.
//
// This is the ONLY definition of the component phase enum anywhere in the tree
// (docs/SPEC.md §3 INV-7). All three kinds reach it through CommonStatus, so
// adding a second +kubebuilder:validation:Enum for component phase elsewhere
// would let the two drift apart silently.
//
// Ready is the only stable state. Everything else is either transient or
// explicitly awaiting a decision, which is what makes "is this converged?"
// answerable by inspection.
//
// +kubebuilder:validation:Enum=Absent;Pending;Preflight;Applying;Initializing;PostConfiguring;RollingOut;Ready;Degraded;Faulted;BackingOut;Quarantined
type ComponentPhase string

const (
	PhaseAbsent          ComponentPhase = "Absent"
	PhasePending         ComponentPhase = "Pending"
	PhasePreflight       ComponentPhase = "Preflight"
	PhaseApplying        ComponentPhase = "Applying"
	PhaseInitializing    ComponentPhase = "Initializing"
	PhasePostConfiguring ComponentPhase = "PostConfiguring"
	PhaseRollingOut      ComponentPhase = "RollingOut"
	PhaseReady           ComponentPhase = "Ready"
	PhaseDegraded        ComponentPhase = "Degraded"
	PhaseFaulted         ComponentPhase = "Faulted"
	PhaseBackingOut      ComponentPhase = "BackingOut"
	PhaseQuarantined     ComponentPhase = "Quarantined"
)

// AllComponentPhases returns every component phase in state-machine order. It
// exists so a test can assert the enum marker and the constants agree; keeping
// them in sync by eye is exactly the INV-7 failure.
func AllComponentPhases() []ComponentPhase {
	return []ComponentPhase{
		PhaseAbsent, PhasePending, PhasePreflight, PhaseApplying,
		PhaseInitializing, PhasePostConfiguring, PhaseRollingOut, PhaseReady,
		PhaseDegraded, PhaseFaulted, PhaseBackingOut, PhaseQuarantined,
	}
}

// FaultClass is how a failure is classified before any recovery action is
// chosen (docs/DESIGN.md §12.4). Recovery differs by class, so classification
// has to happen first.
//
// +kubebuilder:validation:Enum=Transient;Blocked;DriftConflict;PartialCommit
type FaultClass string

const (
	// FaultTransient failed for a reason unrelated to desired state. Retry in
	// place with backoff, bounded, then Quarantined.
	FaultTransient FaultClass = "Transient"

	// FaultBlocked cannot complete because something it needs does not exist.
	// It MUST NOT retry: name the unmet dependency in BlockedOn and fail at
	// the deadline. An unbounded retry loop is indistinguishable from progress.
	FaultBlocked FaultClass = "Blocked"

	// FaultDriftConflict means live state was changed out of band and the
	// apply was rejected. Retrying unchanged fails identically every time.
	FaultDriftConflict FaultClass = "DriftConflict"

	// FaultPartialCommit means the step failed after an irreversible side
	// effect. Recovery depends entirely on the rollback safety class.
	FaultPartialCommit FaultClass = "PartialCommit"
)

// AllFaultClasses returns every fault class.
func AllFaultClasses() []FaultClass {
	return []FaultClass{FaultTransient, FaultBlocked, FaultDriftConflict, FaultPartialCommit}
}

// OperationType names an in-flight operation neutrally. Backend job names are
// never surfaced here (docs/SPEC.md §3 INV-1, §6 STATE-010).
//
// +kubebuilder:validation:Enum=Install;Upgrade;Resize;Teardown
type OperationType string

const (
	OpInstall  OperationType = "Install"
	OpUpgrade  OperationType = "Upgrade"
	OpResize   OperationType = "Resize"
	OpTeardown OperationType = "Teardown"
)

// AllOperationTypes returns every operation type.
func AllOperationTypes() []OperationType {
	return []OperationType{OpInstall, OpUpgrade, OpResize, OpTeardown}
}

// CommonStatus is the status surface shared by all three kinds.
type CommonStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a coarse rollup, derived from observed cluster state rather than
	// from any single backend response. Backend-internal state vocabulary is
	// translated into this enum by the adapter and never surfaced verbatim.
	//
	// This is the object-level rollup and is deliberately a different, smaller
	// vocabulary from ComponentPhase — do not conflate the two.
	// +kubebuilder:validation:Enum=Pending;Installing;Upgrading;Ready;Degraded;Deleting;Error
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions carries Reconciling, Available, Progressing, Degraded,
	// ReleaseResolved, PrerequisitesMet and UpgradeInProgress.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AppliedRelease is what is actually running, which may lag spec.release
	// mid-upgrade. Comparing the two is how you answer "is this on 2026.4?"
	// +optional
	AppliedRelease *AppliedRelease `json:"appliedRelease,omitempty"`

	// Components carries per-component health so a single unhealthy service
	// does not have to be inferred from a rolled-up phase.
	// +listType=map
	// +listMapKey=name
	// +optional
	Components []ComponentStatus `json:"components,omitempty"`

	// +optional
	ReadyComponents int32 `json:"readyComponents,omitempty"`

	// +optional
	DesiredComponents int32 `json:"desiredComponents,omitempty"`

	// RenderedValuesRef points at a ConfigMap holding the fully-merged values
	// after all six override layers, for diffing and support (API-012).
	// +optional
	RenderedValuesRef *corev1.LocalObjectReference `json:"renderedValuesRef,omitempty"`

	// ActiveOperation reports the in-flight operation, when it started, and
	// where to find its logs, so `kubectl describe` is enough to debug.
	// +optional
	ActiveOperation *OperationStatus `json:"activeOperation,omitempty"`
}

// ComponentStatus is per-component health and state. Defined exactly once
// (docs/SPEC.md §3 INV-7) and reached by all three kinds through CommonStatus.
type ComponentStatus struct {
	Name string `json:"name"`

	// Phase is the per-component state machine state.
	Phase ComponentPhase `json:"phase"`

	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// FaultClass is set whenever Phase is Faulted, BackingOut or Quarantined
	// (STATE-004).
	// +optional
	FaultClass FaultClass `json:"faultClass,omitempty"`

	// BlockedOn names the unmet dependencies when FaultClass is Blocked.
	// Without this a deadlock is indistinguishable from slow progress.
	// +optional
	BlockedOn []string `json:"blockedOn,omitempty"`

	// RollbackClass is the component's declared back-out safety class (A-D).
	// It comes from the component catalog data, not from code (STATE-008).
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

// AppliedRelease records what was actually resolved and applied.
type AppliedRelease struct {
	MatrixVersion string `json:"matrixVersion"`

	// Chart is the artifact actually resolved and applied, which may differ
	// from the matrix default when ChartOverride was set.
	// +optional
	Chart *ChartRef `json:"chart,omitempty"`

	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`
}

// OperationStatus reports the in-flight operation.
type OperationStatus struct {
	// Type is a neutral operation name. Backend job names are never surfaced
	// here (INV-1, STATE-010).
	Type OperationType `json:"type"`

	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// Deadline after which the operation is treated as Faulted rather than
	// left to run indefinitely. INV-4 requires every wait to have one.
	//
	// The *values* are a SPEC §9-3 open decision and belong in the release
	// matrix as per-component data. This field only records the deadline that
	// was chosen; do not compile a default in.
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

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T='TestComponentPhaseVocabulary|TestFaultClassVocabulary|TestOperationTypeVocabulary|TestCommonStatus'`
Expected: `PASS` (4 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): status surface, component phase and fault class (INV-7)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6, INV-7

**Do not:** add a second `+kubebuilder:validation:Enum` for component phase
anywhere, including on a per-kind status type. INV-7 exists because two enums
that mean the same thing drift, and the drift is invisible until a controller
writes a value one CRD rejects.
**Do not:** widen `CommonStatus.Phase` to the component vocabulary. They are
deliberately different: the rollup has seven values, the component machine has
twelve.

---

### Task 2.9 — `PCDUnderlay`

**Requirement:** SPEC §4.1 `API-001`, `API-002`, `API-003`; DESIGN §6
**Files:**
- Create: `api/v1alpha1/pcdunderlay_types.go`
- Test: `api/v1alpha1/pcdunderlay_test.go`
**Depends on:** 2.2, 2.3, 2.5, 2.7, 2.8

**RED**

Test file: `api/v1alpha1/pcdunderlay_test.go`
Test name: `TestPCDUnderlayRegistersInScheme`

```go
package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestPCDUnderlayRegistersInScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	gvk := GroupVersion.WithKind("PCDUnderlay")
	if !s.Recognizes(gvk) {
		t.Errorf("scheme does not recognize %s", gvk)
	}
	if !s.Recognizes(GroupVersion.WithKind("PCDUnderlayList")) {
		t.Errorf("scheme does not recognize PCDUnderlayList")
	}
}

func TestPCDUnderlaySpecJSONTags(t *testing.T) {
	spec := PCDUnderlaySpec{
		Profile:           "on-prem",
		Release:           ReleaseSpec{MatrixVersion: "2026.4"},
		Platform:          PlatformSpec{Kind: "nodelet"},
		ExternalServices:  &ExternalServices{},
		Networking:        InfraNetworkingSpec{HostedZone: "pcd.example.invalid"},
		ComponentDefaults: &ComponentSpec{},
		Components:        map[string]ComponentSpec{"cert-manager": {}},
		UpgradePolicy:     &UpgradePolicy{Trigger: "Immediate"},
		Paused:            true,
	}
	assertJSONKeys(t, spec,
		"profile", "release", "platform", "externalServices", "networking",
		"componentDefaults", "components", "upgradePolicy", "paused",
	)
}

// INV-6: the underlay describes the one cluster the operator runs in. There is
// no cluster selection anywhere in it.
func TestPlatformSpecHasNoClusterSelection(t *testing.T) {
	assertJSONKeys(t, PlatformSpec{Kind: "aws-eks", Region: "us-west-2", Airgapped: true},
		"kind", "region", "airgapped")
}
```

Run: `make test-one T=TestPCDUnderlay`

Expected failure:
```
api/v1alpha1/pcdunderlay_test.go:16:24: undefined: PCDUnderlay
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/pcdunderlay_types.go`**

Note the `PlatformSpec` doc comment below. `docs/DESIGN.md` §6 words it using
backend vocabulary; that wording fails `make lint-leak` and is replaced here.
Do not restore the original.

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	SchemeBuilder.Register(&PCDUnderlay{}, &PCDUnderlayList{})
}
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T='TestPCDUnderlay|TestPlatformSpec'`
Expected: `PASS` (3 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): PCDUnderlay, cluster-scoped (API-001, API-002, API-003)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run — `config/crd/bases/install.pcd.platform9.com_pcdunderlays.yaml` now exists
- [ ] Invariants checked: INV-1, INV-6, INV-7

**Do not:** transcribe the `PlatformSpec` doc comment from `docs/DESIGN.md`
verbatim. It names the backend and two placement concepts, and `make lint-leak`
will reject it. The replacement above says the same thing without them.
**Do not:** make `PCDUnderlay` namespaced. API-002 is explicit.

---

### Task 2.10 — `PCDInstallation`

**Requirement:** SPEC §4.1 `API-001`, `API-002`, `API-003`; DESIGN §7
**Files:**
- Create: `api/v1alpha1/pcdinstallation_types.go`
- Test: `api/v1alpha1/pcdinstallation_test.go`
**Depends on:** 2.9

**RED**

Test file: `api/v1alpha1/pcdinstallation_test.go`
Test name: `TestPCDInstallationRegistersInScheme`

```go
package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestPCDInstallationRegistersInScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	for _, kind := range []string{"PCDInstallation", "PCDInstallationList"} {
		if !s.Recognizes(GroupVersion.WithKind(kind)) {
			t.Errorf("scheme does not recognize %s", kind)
		}
	}
}

func TestPCDInstallationSpecJSONTags(t *testing.T) {
	spec := PCDInstallationSpec{
		ShortName:         "acme",
		DisplayName:       "Acme Corp",
		AdminEmail:        "admin@acme.invalid",
		FQDN:              "acme.pcd.example.invalid",
		Release:           &ReleaseSpec{MatrixVersion: "2026.4"},
		ExternalServices:  &ExternalServices{},
		SSO:               &IdentityBinding{Provider: "oidc"},
		ComponentDefaults: &ComponentSpec{},
		Components:        map[string]ComponentSpec{"keystone": {}},
		Size:              SizeMedium,
		Protection:        &ProtectionSpec{PreventDeletion: true},
		UpgradePolicy:     &UpgradePolicy{Trigger: "Immediate"},
		Paused:            true,
	}
	assertJSONKeys(t, spec,
		"shortName", "displayName", "adminEmail", "fqdn", "release",
		"externalServices", "sso", "componentDefaults", "components", "size",
		"protection", "upgradePolicy", "paused",
	)
}

// DESIGN §8: a PCDRegion inherits its installation's size when unset, so an
// unset size must be distinguishable from any valid value. The empty string is
// not one of the five.
func TestPCDInstallationSizeIsOmittedWhenUnset(t *testing.T) {
	spec := PCDInstallationSpec{ShortName: "acme", AdminEmail: "a@b.invalid"}
	if spec.Size != "" {
		t.Fatalf("unset size = %q, want empty", spec.Size)
	}
	for _, s := range []PCDSize{SizeXSmall, SizeSmall, SizeMedium, SizeLarge, SizeXLarge} {
		if s == "" {
			t.Fatalf("a valid size is the empty string, which makes inheritance undecidable")
		}
	}
}
```

Run: `make test-one T=TestPCDInstallation`

Expected failure:
```
api/v1alpha1/pcdinstallation_test.go:19:10: undefined: PCDInstallationSpec
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/pcdinstallation_types.go`**

Note the `ShortName` doc comment. `docs/DESIGN.md` §7 explains immutability by
naming the backend's namespace prefix; that wording fails `make lint-leak` and is
replaced here.

```go
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
	SchemeBuilder.Register(&PCDInstallation{}, &PCDInstallationList{})
}
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T=TestPCDInstallation`
Expected: `PASS` (3 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): PCDInstallation (API-001, API-002, API-003)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (api/ changed)
- [ ] Invariants checked: INV-1, INV-6

**Do not:** name this kind `PCDCustomer`. That name was retired; "customer"
refers only to the tenant an installation belongs to (CLAUDE.md "Naming").
**Do not:** add a CEL immutability rule for `shortName` — see "Scope notes".

---

### Task 2.11 — `PCDRegion`

**Requirement:** SPEC §4.1 `API-001`, `API-002`, `API-003`; DESIGN §8
**Files:**
- Create: `api/v1alpha1/pcdregion_types.go`
- Test: `api/v1alpha1/pcdregion_test.go`
**Depends on:** 2.10

**RED**

Test file: `api/v1alpha1/pcdregion_test.go`
Test name: `TestPCDRegionRegistersInScheme`

```go
package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPCDRegionRegistersInScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	for _, kind := range []string{"PCDRegion", "PCDRegionList"} {
		if !s.Recognizes(GroupVersion.WithKind(kind)) {
			t.Errorf("scheme does not recognize %s", kind)
		}
	}
}

func TestPCDRegionSpecJSONTags(t *testing.T) {
	spec := PCDRegionSpec{
		InstallationRef:   corev1.LocalObjectReference{Name: "acme"},
		RegionName:        "Region-One",
		RegionInstance:    "region-one",
		FQDN:              "acme-region-one.pcd.example.invalid",
		Release:           &ReleaseSpec{MatrixVersion: "2026.4"},
		ExternalServices:  &ExternalServices{},
		Networking:        &RegionNetworkingSpec{},
		Features:          &RegionFeatures{},
		Size:              SizeSmall,
		ComponentDefaults: &ComponentSpec{},
		Components:        map[string]ComponentSpec{"nova": {}},
		Protection:        &ProtectionSpec{PreventDeletion: true},
		UpgradePolicy:     &UpgradePolicy{Trigger: "Immediate"},
		Paused:            true,
	}
	assertJSONKeys(t, spec,
		"installationRef", "regionName", "regionInstance", "fqdn", "release",
		"externalServices", "networking", "features", "size",
		"componentDefaults", "components", "protection", "upgradePolicy", "paused",
	)
}

// SPEC §10: PCDHostPool is deferred and MUST NOT be stubbed in PCDRegion
// meanwhile. A half-modelled field is harder to remove than to add.
func TestPCDRegionHasNoHostManagementStanza(t *testing.T) {
	b, err := json.Marshal(PCDRegionSpec{
		InstallationRef: corev1.LocalObjectReference{Name: "acme"},
		RegionName:      "Region-One",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, banned := range []string{"hostManagement", "hostPool", "hosts", "hostAgent"} {
		if strings.Contains(string(b), banned) {
			t.Errorf("PCDRegionSpec contains %q: host lifecycle is deferred to PCDHostPool", banned)
		}
	}
}

func TestRegionFeaturesJSONTags(t *testing.T) {
	on := true
	f := RegionFeatures{
		Orchestration: &on, IaC: &on, LoadBalancing: &on, DNSaaS: &on,
		KeyManagement: &on, Optimization: &on, HighAvailability: &on,
		AppCatalog: &on, Audit: &on, Kubernetes: &on,
	}
	assertJSONKeys(t, f,
		"orchestration", "iac", "loadBalancing", "dnsaas", "keyManagement",
		"optimization", "highAvailability", "appCatalog", "audit", "kubernetes",
	)
}
```

Run: `make test-one T='TestPCDRegion|TestRegionFeatures'`

Expected failure:
```
api/v1alpha1/pcdregion_test.go:27:10: undefined: PCDRegionSpec
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `api/v1alpha1/pcdregion_types.go`**

```go
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
	SchemeBuilder.Register(&PCDRegion{}, &PCDRegionList{})
}
```

- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T='TestPCDRegion|TestRegionFeatures'`
Expected: `PASS` (4 tests)

- [ ] **Step 4: Regenerate and commit**

```bash
make manifests generate
git add api/v1alpha1/ config/
git commit -m "feat(api): PCDRegion (API-001, API-002, API-003)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run — all three CRDs now exist in `config/crd/bases/`
- [ ] Invariants checked: INV-1, INV-6, INV-7

**Do not:** add a `hostManagement`, `hostPool` or `hosts` field, even as a
placeholder. SPEC §10 is explicit, and `TestPCDRegionHasNoHostManagementStanza`
is there to catch it.

---

### Task 2.12 — Assertions on the generated CRDs

**Requirement:** SPEC §4.1 `API-001`, `API-002`, `API-003`; §3 INV-7
**Files:**
- Create: `internal/crdcheck/crd_test.go`
**Depends on:** 2.11

**RED**

Test file: `internal/crdcheck/crd_test.go`
Test name: `TestCRD...`

```go
// Package crdcheck asserts properties of the generated CRD manifests that
// cannot be asserted from the Go types alone. It has no non-test source: the
// artifacts under test are the YAML files controller-gen produces.
package crdcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

const crdDir = "../../config/crd/bases"

func loadCRDs(t *testing.T) map[string]apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	entries, err := os.ReadDir(crdDir)
	if err != nil {
		t.Fatalf("read %s: %v (run `make manifests`)", crdDir, err)
	}

	out := map[string]apiextensionsv1.CustomResourceDefinition{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(crdDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal(b, &crd); err != nil {
			t.Fatalf("unmarshal %s: %v", e.Name(), err)
		}
		out[crd.Spec.Names.Kind] = crd
	}
	return out
}

func TestAllThreeKindsAreGenerated(t *testing.T) {
	crds := loadCRDs(t)
	for _, kind := range []string{"PCDUnderlay", "PCDInstallation", "PCDRegion"} {
		if _, ok := crds[kind]; !ok {
			t.Errorf("no generated CRD for kind %s", kind)
		}
	}
	if len(crds) != 3 {
		t.Errorf("got %d CRDs, want exactly 3: %v", len(crds), keys(crds))
	}
}

// API-001
func TestCRDGroupAndVersion(t *testing.T) {
	for kind, crd := range loadCRDs(t) {
		t.Run(kind, func(t *testing.T) {
			if got, want := crd.Spec.Group, "install.pcd.platform9.com"; got != want {
				t.Errorf("group = %q, want %q", got, want)
			}
			if len(crd.Spec.Versions) != 1 {
				t.Fatalf("got %d versions, want 1", len(crd.Spec.Versions))
			}
			v := crd.Spec.Versions[0]
			if v.Name != "v1alpha1" {
				t.Errorf("version = %q, want v1alpha1", v.Name)
			}
			if !v.Served || !v.Storage {
				t.Errorf("version served=%v storage=%v, want both true", v.Served, v.Storage)
			}
			if v.Subresources == nil || v.Subresources.Status == nil {
				t.Errorf("status subresource is not enabled")
			}
		})
	}
}

// API-002
func TestCRDScopes(t *testing.T) {
	want := map[string]apiextensionsv1.ResourceScope{
		"PCDUnderlay":     apiextensionsv1.ClusterScoped,
		"PCDInstallation": apiextensionsv1.NamespaceScoped,
		"PCDRegion":       apiextensionsv1.NamespaceScoped,
	}
	crds := loadCRDs(t)
	for kind, wantScope := range want {
		crd, ok := crds[kind]
		if !ok {
			t.Errorf("no CRD for %s", kind)
			continue
		}
		if crd.Spec.Scope != wantScope {
			t.Errorf("%s scope = %q, want %q", kind, crd.Spec.Scope, wantScope)
		}
	}
}

// API-003
func TestCRDShortNames(t *testing.T) {
	want := map[string]string{
		"PCDUnderlay":     "pcdunderlay",
		"PCDInstallation": "pcdinstall",
		"PCDRegion":       "pcdregion",
	}
	crds := loadCRDs(t)
	for kind, wantShort := range want {
		crd, ok := crds[kind]
		if !ok {
			t.Errorf("no CRD for %s", kind)
			continue
		}
		if !contains(crd.Spec.Names.ShortNames, wantShort) {
			t.Errorf("%s shortNames = %v, want to contain %q", kind, crd.Spec.Names.ShortNames, wantShort)
		}
	}
}

// INV-7: component phase has exactly one enum definition. The test targets
// status.components[].phase specifically — status.phase is the object-level
// rollup with a deliberately different, smaller vocabulary, and conflating the
// two is the mistake this test is shaped to avoid.
func TestComponentPhaseEnumIsDefinedOnce(t *testing.T) {
	want := []string{
		"Absent", "Pending", "Preflight", "Applying", "Initializing",
		"PostConfiguring", "RollingOut", "Ready", "Degraded", "Faulted",
		"BackingOut", "Quarantined",
	}

	seen := map[string][]string{} // joined enum -> kinds that carry it
	for kind, crd := range loadCRDs(t) {
		got := componentPhaseEnum(t, kind, crd)
		if !equal(got, want) {
			t.Errorf("%s: component phase enum = %v, want %v", kind, got, want)
		}
		key := join(got)
		seen[key] = append(seen[key], kind)
	}
	if len(seen) != 1 {
		t.Errorf("found %d distinct component phase enums, want 1: %v", len(seen), seen)
	}
}

func componentPhaseEnum(t *testing.T, kind string, crd apiextensionsv1.CustomResourceDefinition) []string {
	t.Helper()

	root := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	status, ok := root.Properties["status"]
	if !ok {
		t.Fatalf("%s: no status property", kind)
	}
	components, ok := status.Properties["components"]
	if !ok {
		t.Fatalf("%s: no status.components property", kind)
	}
	if components.Items == nil || components.Items.Schema == nil {
		t.Fatalf("%s: status.components has no item schema", kind)
	}
	phase, ok := components.Items.Schema.Properties["phase"]
	if !ok {
		t.Fatalf("%s: no status.components[].phase property", kind)
	}

	out := make([]string, 0, len(phase.Enum))
	for _, raw := range phase.Enum {
		var s string
		if err := json.Unmarshal(raw.Raw, &s); err != nil {
			t.Fatalf("%s: enum value %s is not a string: %v", kind, raw.Raw, err)
		}
		out = append(out, s)
	}
	return out
}

// INV-8: the MariaDB passthrough fields must preserve unknown fields and must
// not carry a schema of their own.
func TestPassthroughFieldsPreserveUnknownFields(t *testing.T) {
	for kind, crd := range loadCRDs(t) {
		found := 0
		walkSchema(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, "", func(path string, s apiextensionsv1.JSONSchemaProps) {
			if filepath.Base(path) != "template" {
				return
			}
			found++
			if s.XPreserveUnknownFields == nil || !*s.XPreserveUnknownFields {
				t.Errorf("%s: %s does not preserve unknown fields (INV-8)", kind, path)
			}
			if len(s.Properties) != 0 {
				t.Errorf("%s: %s has a typed schema with %d properties; the passthrough must not be validated (INV-8)", kind, path, len(s.Properties))
			}
		})
		// All three kinds reach DatabaseBinding and MaxScaleSpec through
		// spec.externalServices, so all three must carry both templates.
		if found < 2 {
			t.Errorf("%s: found %d template passthrough fields, want at least 2 (database and maxScale)", kind, found)
		}
	}
}

// walkSchema visits every property schema, passing a slash-separated path.
func walkSchema(s *apiextensionsv1.JSONSchemaProps, path string, fn func(string, apiextensionsv1.JSONSchemaProps)) {
	if s == nil {
		return
	}
	for name, child := range s.Properties {
		childPath := path + "/" + name
		fn(childPath, child)
		c := child
		walkSchema(&c, childPath, fn)
	}
	if s.Items != nil && s.Items.Schema != nil {
		walkSchema(s.Items.Schema, path+"/[]", fn)
	}
	if s.AdditionalProperties != nil && s.AdditionalProperties.Schema != nil {
		walkSchema(s.AdditionalProperties.Schema, path+"/{}", fn)
	}
}

func keys(m map[string]apiextensionsv1.CustomResourceDefinition) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func join(s []string) string {
	out := ""
	for _, v := range s {
		out += v + ","
	}
	return out
}
```

Run: `make test-one T=TestCRD`

Expected failure — before `make manifests` has been run against all three kinds,
or if any assertion is wrong:
```
--- FAIL: TestCRDShortNames
    crd_test.go:NN: PCDInstallation shortNames = [], want to contain "pcdinstall"
```

If every assertion passes on the first run, that is expected: tasks 2.9–2.11
already set these markers. Record that in your task notes, then prove the test
has teeth by temporarily changing `shortName=pcdinstall` to `shortName=pcdinst`
in `pcdinstallation_types.go`, running `make manifests && make test-one T=TestCRDShortNames`,
observing the failure, and reverting.

**GREEN**

- [ ] **Step 1: Write the test above**
- [ ] **Step 2: Run `make manifests` then the test**

Run: `make manifests && make test-one T=TestCRD`
Expected: `PASS` (5 tests)

- [ ] **Step 3: Prove the test has teeth (the exercise described under RED)**

- [ ] **Step 4: Commit**

```bash
git add internal/crdcheck/
git commit -m "test(api): assert generated CRD group, scope, short names and phase enum"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] Invariants checked: INV-7, INV-8

**Do not:** relax `TestComponentPhaseEnumIsDefinedOnce` to compare sets instead
of ordered lists. Order is part of the state machine's documentation and a
reordered enum is a review signal.

---

### Task 2.13 — API-005: every type is reachable and every field is tagged

**Requirement:** SPEC §4.1 `API-005`
**Files:**
- Create: `api/v1alpha1/apishape_test.go`
**Depends on:** 2.11

**RED**

Test file: `api/v1alpha1/apishape_test.go`
Test name: `TestEveryDeclaredTypeIsReachableFromARootKind`

```go
package v1alpha1

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// parseAPIPackage parses every non-generated, non-test source file in this
// package. parser.ParseFile is used rather than parser.ParseDir because the
// latter is deprecated and `make lint` rejects deprecated calls.
func parseAPIPackage(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || strings.HasPrefix(base, "zz_generated.") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("parsed no source files")
	}
	return fset, files
}

// declaredTypes returns every type declared in the package, by name.
func declaredTypes(files []*ast.File) map[string]*ast.TypeSpec {
	out := map[string]*ast.TypeSpec{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					out[ts.Name.Name] = ts
				}
			}
		}
	}
	return out
}

// localRefs collects the names of package-local types referenced by expr.
// Struct *field names* are deliberately not visited — only field types — so a
// field named the same as a type cannot create a false edge.
func localRefs(expr ast.Expr, declared map[string]*ast.TypeSpec, out map[string]bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		if _, ok := declared[t.Name]; ok {
			out[t.Name] = true
		}
	case *ast.StarExpr:
		localRefs(t.X, declared, out)
	case *ast.ArrayType:
		localRefs(t.Elt, declared, out)
	case *ast.MapType:
		localRefs(t.Key, declared, out)
		localRefs(t.Value, declared, out)
	case *ast.StructType:
		for _, field := range t.Fields.List {
			localRefs(field.Type, declared, out)
		}
	case *ast.SelectorExpr:
		// A type from another package. Nothing local to record.
	}
}

// API-005: a type declared in api/ and reachable from no root Kind is dead API
// surface. `go vet` and golangci-lint will not find it, because exported types
// in a library package are never "unused".
func TestEveryDeclaredTypeIsReachableFromARootKind(t *testing.T) {
	_, files := parseAPIPackage(t)
	declared := declaredTypes(files)

	edges := map[string]map[string]bool{}
	for name, ts := range declared {
		refs := map[string]bool{}
		localRefs(ts.Type, declared, refs)
		edges[name] = refs
	}

	roots := []string{
		"PCDUnderlay", "PCDUnderlayList",
		"PCDInstallation", "PCDInstallationList",
		"PCDRegion", "PCDRegionList",
	}
	for _, r := range roots {
		if _, ok := declared[r]; !ok {
			t.Fatalf("root kind %s is not declared", r)
		}
	}

	reached := map[string]bool{}
	queue := append([]string{}, roots...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if reached[name] {
			continue
		}
		reached[name] = true
		for ref := range edges[name] {
			if !reached[ref] {
				queue = append(queue, ref)
			}
		}
	}

	for name := range declared {
		if !reached[name] {
			t.Errorf("type %s is declared in api/v1alpha1 but not reachable from any root Kind", name)
		}
	}
}

// A field without a json tag becomes a CRD property named after the Go field,
// which is capitalised and wrong, and nothing else catches it.
func TestEveryFieldHasALowerCamelJSONTag(t *testing.T) {
	fset, files := parseAPIPackage(t)

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				pos := fset.Position(field.Pos())
				if field.Tag == nil {
					t.Errorf("%s: %s has a field with no struct tag", pos, ts.Name.Name)
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					t.Errorf("%s: unparsable struct tag %s", pos, field.Tag.Value)
					continue
				}
				jsonTag := reflect.StructTag(raw).Get("json")
				if jsonTag == "" {
					t.Errorf("%s: %s has a field with no json tag", pos, ts.Name.Name)
					continue
				}
				name := strings.Split(jsonTag, ",")[0]
				if name == "" {
					// Embedded/inline field, e.g. `json:",inline"`.
					continue
				}
				if c := name[0]; c >= 'A' && c <= 'Z' {
					t.Errorf("%s: %s json tag %q is not lowerCamelCase", pos, ts.Name.Name, name)
				}
			}
			return true
		})
	}
}
```

Run: `make test-one T='TestEveryDeclaredTypeIsReachable|TestEveryFieldHasALowerCamel'`

Expected failure — one of two things. If every type from tasks 2.2–2.11 is
wired up, both pass first time; record that. If a type was added and never
referenced, you get:
```
--- FAIL: TestEveryDeclaredTypeIsReachableFromARootKind
    apishape_test.go:NN: type MaxScaleSpec is declared in api/v1alpha1 but not reachable from any root Kind
```

Prove the reachability test has teeth: temporarily add
`type Orphan struct{ Name string \`json:"name"\` }` to `api/v1alpha1/common_types.go`,
run the test, observe the failure naming `Orphan`, then delete it.

**GREEN**

- [ ] **Step 1: Write the test above and run it**
- [ ] **Step 2: Fix anything it reports. An unreachable type is either a missing wire-up or a type that should not exist**
- [ ] **Step 3: Prove the test has teeth with the `Orphan` exercise above, then delete `Orphan`**
- [ ] **Step 4: Run the full suite**

Run: `make test`
Expected: `ok platform9.com/pcd-operator/api/v1alpha1` and
`ok platform9.com/pcd-operator/internal/crdcheck`

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/apishape_test.go
git commit -m "test(api): every type reachable from a root Kind, every field json-tagged (API-005)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] Invariants checked: none directly; this is API-005

**Do not:** add a type to `roots` to silence a failure. If a type is only
reachable from something that is not a Kind, it is dead surface — delete it, or
wire it in where the design says it belongs.

---

### Task 2.14 — INV-8: the passthrough survives a round trip unmodified

**Requirement:** SPEC §3 `INV-8`
**Files:**
- Create: `api/v1alpha1/passthrough_test.go`
**Depends on:** 2.6

**RED**

Test file: `api/v1alpha1/passthrough_test.go`
Test name: `TestTemplatePassthroughSurvivesRoundTrip`

```go
package v1alpha1

import (
	"encoding/json"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// INV-8: DatabaseBinding.Template and MaxScaleSpec.Template are not
// schema-validated by this operator. An arbitrary unknown field must survive a
// round trip byte-for-byte — if it does not, something is parsing the
// passthrough, which is the drift this invariant exists to prevent.
func TestTemplatePassthroughSurvivesRoundTrip(t *testing.T) {
	// Deliberately includes a field no MariaDB version has, nested three deep.
	raw := `{"podTemplate":{"nodeSelector":{"role":"db"}},"someFieldUpstreamAddedLastWeek":{"nested":{"deep":true}},"tls":{"enabled":true}}`

	binding := DatabaseBinding{
		Provisioning: "Dedicated",
		Template:     &apiextensionsv1.JSON{Raw: []byte(raw)},
		MaxScale: &MaxScaleSpec{
			Template: &apiextensionsv1.JSON{Raw: []byte(raw)},
		},
	}

	encoded, err := json.Marshal(binding)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DatabaseBinding
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Template == nil {
		t.Fatal("Template is nil after round trip")
	}
	if got := string(decoded.Template.Raw); got != raw {
		t.Errorf("DatabaseBinding.Template round trip:\n got %s\nwant %s", got, raw)
	}
	if decoded.MaxScale == nil || decoded.MaxScale.Template == nil {
		t.Fatal("MaxScale.Template is nil after round trip")
	}
	if got := string(decoded.MaxScale.Template.Raw); got != raw {
		t.Errorf("MaxScaleSpec.Template round trip:\n got %s\nwant %s", got, raw)
	}
}

// The deepcopy generated for the passthrough must copy the bytes, not alias
// them — an aliased Raw slice would let a status update mutate spec.
func TestTemplatePassthroughDeepCopyIsNotAliased(t *testing.T) {
	original := DatabaseBinding{
		Provisioning: "Dedicated",
		Template:     &apiextensionsv1.JSON{Raw: []byte(`{"a":1}`)},
	}

	clone := original.DeepCopy()
	if clone.Template == original.Template {
		t.Fatal("DeepCopy returned the same *JSON pointer")
	}
	clone.Template.Raw[0] = 'X'
	if string(original.Template.Raw) != `{"a":1}` {
		t.Errorf("mutating the clone changed the original: %s", original.Template.Raw)
	}
}
```

Run: `make test-one T=TestTemplatePassthrough`

Expected failure, before `make generate` has produced `DeepCopy`:
```
api/v1alpha1/passthrough_test.go:NN:17: original.DeepCopy undefined (type DatabaseBinding has no field or method DeepCopy)
FAIL	platform9.com/pcd-operator/api/v1alpha1 [build failed]
```

If `make generate` has already been run in an earlier task, both tests pass
immediately — record that. Prove the round-trip test has teeth by temporarily
changing `Template *apiextensionsv1.JSON` to `Template *DatabaseBinding` and
observing the compile failure, then reverting.

**GREEN**

- [ ] **Step 1: Write the test above and run it, paste the result into your task notes**
- [ ] **Step 2: Run `make generate` if `DeepCopy` is missing**
- [ ] **Step 3: Run the test and observe green**

Run: `make test-one T=TestTemplatePassthrough`
Expected: `PASS` (2 tests)

- [ ] **Step 4: Commit**

```bash
git add api/v1alpha1/passthrough_test.go
git commit -m "test(api): MariaDB passthrough survives round trip unmodified (INV-8)"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] Invariants checked: INV-8

**Do not:** "fix" a round-trip mismatch by normalising the JSON. If the bytes
change, something is parsing the passthrough and that is the bug.

---

## Milestone exit criteria

- [ ] `make verify` passes from a clean checkout
- [ ] `config/crd/bases/` contains exactly three CRDs, group
      `install.pcd.platform9.com`, version `v1alpha1`, status subresource enabled
- [ ] `PCDUnderlay` is cluster-scoped; `PCDInstallation` and `PCDRegion` are
      namespaced (API-002)
- [ ] Short names are `pcdunderlay`, `pcdinstall`, `pcdregion` (API-003)
- [ ] `make lint-leak` is clean against a fully-populated `api/` tree
- [ ] Exactly one component phase enum across all three CRDs (INV-7)
- [ ] No type in `api/v1alpha1` is unreachable from a root Kind (API-005)
- [ ] `zz_generated.deepcopy.go` is committed and `make check-generated` is clean
- [ ] No dependency beyond the five approved in "Dependencies introduced"

## Deliberately not done here

| Requirement | Where it belongs | Why not here |
|---|---|---|
| API-004 immutability | 006 (webhooks) | Acceptance criterion is a webhook test |
| API-010…013 merge engine | 003 | Pure logic, no types needed beyond these |
| API-020…023 size presets | 004 | Presets are release-matrix data, not code |
| DB-001…007 MariaDB generation | 008 | Needs the merge engine and the webhooks |
| STATE-002…009 state machine | 009 | This milestone defines the vocabulary only |
| Component catalogs | 004, 005 | Data, not types |
