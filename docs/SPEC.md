# PCD Installer Operator — Development Specification

Normative companion to `docs/DESIGN.md`. The design explains *why*; this document
states *what*, in a form that can be turned into plans and checked in review.

Every requirement has an ID, a normative statement, and an acceptance criterion
that can be written as a test before the code exists. If a requirement cannot be
tested, it is not a requirement — it is design commentary and belongs in
`DESIGN.md`.

---

## 1. Scope

**In scope.** Three CRDs and their controllers; direct Helm/manifest application
of cluster prerequisites; a Bork adapter for installation and region lifecycle; a
per-component reconciliation state machine; a MariaDB operator wrapper for all
database configuration.

**Out of scope for v1.** `PCDHostPool` and all host-side lifecycle. Multi-cluster
of any kind. Bork1/Bork2 support. Any database technology other than the MariaDB
operator. Adoption of pre-existing deployments (§9-2).

## 2. Normative language

MUST / MUST NOT — required; a violation is a bug.
SHOULD — required unless there is a recorded reason not to.
MAY — genuinely optional.

## 3. Invariants

Restated from `CLAUDE.md` with acceptance criteria. These are cross-cutting; they
apply to every task, not to one milestone.

| ID | Requirement | Acceptance |
|---|---|---|
| INV-1 | No Bork identifier MUST appear in any CRD field name, enum value, status string, event reason, or error message that reaches a user. | `make lint-leak` fails on a banned wordlist (`bork`, `kplane`, `x-`, `waiting_apps`, `du-install`, `du-upgrade`, `du-teardown`, `task_state`, `dont_delete`, `deccaxon`) found in `api/`, or in any string literal passed to an error or event constructor. |
| INV-2 | The underlay controller MUST NOT reach the Bork adapter. | Import-graph test: `internal/controller/underlay` importing `internal/backend` fails the build. |
| INV-3 | Components of rollback class C or D MUST NOT be rolled back automatically, regardless of `UpgradePolicy.AutoRollback`. | Table test over all classes × `AutoRollback` true/false asserts no rollback action is produced for C and D. |
| INV-4 | Every wait MUST have a deadline. | No unbounded poll or retry loop. Lint rule plus a test asserting every state with a wait has a non-nil deadline in `OperationStatus`. |
| INV-5 | `cert-manager` MUST NOT be disableable. | Webhook test: `components["cert-manager"].enabled: false` is rejected with a reason naming host PKI. |
| INV-6 | The API MUST NOT contain cluster-selection fields. | `lint-leak` wordlist includes `targetCluster`, `kubeconfig`, `aim`. |
| INV-7 | Component phase MUST have exactly one enum definition. | Test greps the generated CRDs for phase enums and asserts one distinct set. |
| INV-8 | `DatabaseBinding.Template` and `MaxScaleSpec.Template` MUST NOT be schema-validated by this operator. | Test asserts an arbitrary unknown field survives round-trip unmodified. |

## 4. API requirements

### 4.1 Kinds

| ID | Requirement | Acceptance |
|---|---|---|
| API-001 | Group `install.pcd.platform9.com`, version `v1alpha1`. | Generated CRD manifests assert group/version. |
| API-002 | `PCDUnderlay` is cluster-scoped; `PCDInstallation` and `PCDRegion` are namespaced. | CRD scope assertions. |
| API-003 | Short names `pcdunderlay`, `pcdinstall`, `pcdregion`. | CRD `shortNames` assertion. |
| API-004 | `PCDInstallation.spec.shortName`, `PCDRegion.spec.regionName`, and `PCDRegion.spec.installationRef` MUST be immutable after creation. | Webhook test: update attempt rejected. |
| API-005 | Every type referenced in `api/` MUST be defined in `api/` or be a standard Kubernetes or `k8s.mariadb.com/v1alpha1` type. | `go build ./...` plus a test that no type is declared and unused. |

### 4.2 Override engine

| ID | Requirement | Acceptance |
|---|---|---|
| API-010 | Layers MUST merge in exactly this order, later winning per-key: release matrix → profile → size → `componentDefaults` → `components[name]` → `strategicMergePatches`. | Table test with a distinct marker value injected at each layer; asserts the highest-numbered layer wins and lower layers survive where not overridden. |
| API-011 | Structured fields merge by strategic merge; scalars are last-write-wins. | Table test including a list field with a merge key. |
| API-012 | The fully merged result MUST be written to a ConfigMap referenced by `status.renderedValuesRef` on every successful reconcile. | envtest asserts the ConfigMap exists and matches the merge engine's output for the same input. |
| API-013 | `components[name].enabled: false` MUST remove the component entirely, not scale it to zero. | Test asserts no workload manifest is produced for a disabled component. |

### 4.3 Size

| ID | Requirement | Acceptance |
|---|---|---|
| API-020 | `spec.size` accepts exactly `X-Small`, `Small`, `Medium`, `Large`, `X-Large`. | CRD enum assertion; webhook rejects anything else. |
| API-021 | `PCDRegion` MUST inherit `spec.size` from its `PCDInstallation` when unset, and MUST use its own when set. | Table test over both cases. |
| API-022 | Size presets MUST be loaded as data from the release matrix, not compiled in. | Test swaps the preset source and asserts different output with no code change. |
| API-023 | A size change that crosses a stateful topology boundary (Galera node count change, or Galera↔Standalone) MUST produce a webhook warning naming the affected component. | Webhook test per boundary. |

### 4.4 Database

| ID | Requirement | Acceptance |
|---|---|---|
| DB-001 | `DatabaseBinding` MUST generate a `MariaDB` object in `k8s.mariadb.com/v1alpha1`. No other database backend is supported. | Golden-file test: binding in, MariaDB object out. |
| DB-002 | `topology: Galera` MUST set `spec.galera`; `Replication` MUST set `spec.replication`; `Standalone` MUST set neither. | Table test over all three. |
| DB-003 | `spec.replicas` under Galera MUST be odd and ≥3. | Webhook test rejects even and <3. |
| DB-004 | `template` MUST be deep-merged into the generated MariaDB spec **last**, after every typed field. | Test asserts a `template` value overrides a typed field that writes the same path. |
| DB-005 | With `schema.manage: true`, the operator MUST emit one `Database`, one `User` and one `Grant` per enabled PCD service that needs a database. | Golden-file test over a known service list. |
| DB-006 | `storage.ephemeral: true` MUST be rejected unless `PCDUnderlay.spec.profile` is `community-edition`. | Webhook test both ways. |
| DB-007 | A MariaDB webhook rejection MUST be surfaced verbatim in the component's `Faulted` message. | Adapter test with a fake rejection asserts the upstream text is present. |

## 5. Controller requirements

| ID | Requirement | Acceptance |
|---|---|---|
| CTRL-001 | All three controllers MUST run in one Deployment under one manager, with leader election. | Test asserts one manager and three registered controllers. |
| CTRL-002 | The underlay controller MUST apply prerequisites via Helm/manifests using the cluster client, never via the backend. | INV-2 import test, plus a test asserting the applier is called. |
| CTRL-003 | `PrerequisitesMet=True` MUST be set only when every enabled prerequisite is `Ready`. | envtest: one prerequisite held not-ready keeps the condition False. |
| CTRL-004 | The installation controller MUST NOT call the backend while `PrerequisitesMet != True`. | Test with a fake backend asserts zero calls. |
| CTRL-005 | The region controller MUST NOT call the backend while its installation is not `Available`. | Same shape. |
| CTRL-006 | A backend `200` MUST NOT be treated as completion. Phase is driven from observed cluster state. | Test: fake backend returns success immediately, observed state is not ready, phase stays `Applying`. |
| CTRL-007 | Submission MUST be idempotent across controller restarts. | Test: re-reconcile after simulated restart produces no duplicate operation. |
| CTRL-008 | A malformed backend record for one object MUST NOT break reconciliation of others. | Test: one bad record, others still reconcile; bad one goes `Degraded` with an actionable message. |
| CTRL-009 | Deletion MUST be ordered by finalizer: regions before installation. | envtest: installation deletion blocks while a region exists. |
| CTRL-010 | `spec.protection.preventDeletion` MUST block deletion at the webhook. | Webhook test. |
| CTRL-011 | `spec.paused` MUST stop reconciliation without deleting or modifying anything. | envtest: mutate spec while paused, assert no action. |
| CTRL-012 | A `PCDRegion` release ahead of its `PCDInstallation` release MUST be rejected. | Webhook test. |

## 6. Per-component state machine

| ID | Requirement | Acceptance |
|---|---|---|
| STATE-001 | `ComponentStatus.Phase` MUST be one of: `Absent`, `Pending`, `Preflight`, `Applying`, `Initializing`, `PostConfiguring`, `RollingOut`, `Ready`, `Degraded`, `Faulted`, `BackingOut`, `Quarantined`. | CRD enum assertion. |
| STATE-002 | Each transition MUST be driven by the observable signal named in DESIGN §12.3, not by elapsed time. | Table test per transition with the signal mocked. |
| STATE-003 | `Pending` MUST evaluate declared dependencies, not chart group numbering. | Test: a component whose declared deps are met proceeds even though a lower-numbered group has not. |
| STATE-004 | `faultClass` MUST be one of `Transient`, `Blocked`, `DriftConflict`, `PartialCommit`, and MUST be set whenever phase is `Faulted`, `BackingOut` or `Quarantined`. | Table test. |
| STATE-005 | `Blocked` MUST NOT retry. It MUST set `blockedOn` and fail at its deadline. | Test asserts zero retry attempts and a populated `blockedOn`. |
| STATE-006 | `DriftConflict` MUST be detected in `Preflight`, before any apply is dispatched. | Test: drifted live object produces `DriftConflict` with no apply call. |
| STATE-007 | `Transient` MUST retry with backoff up to a bounded attempt count, then `Quarantined`. | Test asserts the bound is enforced. |
| STATE-008 | Rollback class MUST come from the component catalog data, not from code. | Test swaps catalog data and asserts different behaviour with no code change. |
| STATE-009 | A `PCDRegion` MUST be `Ready` only when every enabled component is `Ready`; any `Quarantined` component makes it `Degraded`, not `Error`. | Table test. |
| STATE-010 | `status.activeOperation.type` MUST be one of `Install`, `Upgrade`, `Resize`, `Teardown` and MUST NOT contain a backend job name. | INV-1 plus enum assertion. |

## 7. Backend adapter

| ID | Requirement | Acceptance |
|---|---|---|
| BACK-001 | The adapter MUST expose only intent-shaped methods: `EnsureInstallation`, `EnsureRegion`, `Teardown`, `Observe`. No backend verbs. | Interface assertion test. |
| BACK-002 | The adapter MUST be the only package importing the backend client. | Import-graph test. |
| BACK-003 | A backend that does not present a Bork3-compatible surface MUST set `PrerequisitesMet=False` with a reason requiring Bork3, and MUST block all downstream reconciliation. | Test with a fake non-Bork3 backend. |
| BACK-004 | The adapter MUST NOT contain version detection or conditional branches for older backend generations. | Code review checklist item; no test. |
| BACK-005 | Backend calls MUST always use in-cluster configuration. | Test asserts no kubeconfig path is read. |

## 8. Milestones

Ordered by dependency. Each is one plan file in `docs/plans/`. The first three
need no cluster, which makes them the right place for a fresh agent to build
confidence and for TDD to be fastest.

| # | Milestone | Depends on | Requirements |
|---|---|---|---|
| 001 | Repo scaffolding, `make` targets, `lint-leak` with the banned wordlist, CI | — | INV-1, INV-6 |
| 002 | API types, deepcopy, CRD generation | 001 | API-001…005 |
| 003 | Override merge engine (pure) | 002 | API-010…013 |
| 004 | Release matrix resolution and size presets (pure) | 003 | API-020…023 |
| 005 | Fault classification and rollback class lookup (pure) | 002 | STATE-004, 007, 008, INV-3 |
| 006 | Webhooks | 002, 004 | API-004, DB-003, DB-006, CTRL-010, CTRL-012, INV-5, API-023 |
| 007 | Backend adapter interface + fake | 002 | BACK-001…005 |
| 008 | MariaDB wrapper: binding → MariaDB/Database/User/Grant | 003, 006 | DB-001…007 |
| 009 | Component state machine (pure, driven by injected signals) | 005 | STATE-001…003, 005, 006, 009, 010 |
| 010 | Underlay controller + chart applier | 003, 006 | CTRL-002, 003, INV-2 |
| 011 | Installation controller | 007, 009, 010 | CTRL-004, 006, 007, 011 |
| 012 | Region controller, finalizers, deletion ordering | 011 | CTRL-005, 008, 009 |
| 013 | Manager wiring, leader election, RBAC | 010–012 | CTRL-001 |

## 9. Open decisions — STOP, do not guess

These are unresolved. An agent that encounters one MUST stop and ask rather than
choose. Each is listed with what a wrong guess costs, because "just pick
something sensible" is exactly the failure mode.

**9-1. Config override validation.** Whether `config.ini` / `config.files` are
validated against a per-component schema, linted for known-dangerous keys, or
accepted unchecked. *Cost of guessing:* choosing "validate" builds a schema
registry nobody asked for; choosing "accept" ships a way to break a service that
starts cleanly and then misbehaves.

**9-2. Adoption of pre-existing deployments.** Out of scope for v1, but no code
should assume greenfield in a way that is expensive to undo — specifically, do
not assume the operator created every Helm release or MariaDB instance it sees.
*Cost of guessing:* ownership assumptions spread through the applier and are hard
to retract later.

**9-3. Deadline values.** INV-4 requires every wait to have a deadline. **No
deadline values are specified.** Do not invent them. They belong in the release
matrix as per-component data seeded from observed p99 durations. Until they
exist, take the deadline as a required constructor parameter with no default, so
the absence is a compile error rather than a silent 30 seconds.

**9-4. Upgrade concurrency budget.** `UpgradePolicy.MaxConcurrent` exists in the
API but no default is agreed. Leave it nil and unenforced until specified.

**9-5. Host CA lifecycle.** cert-manager is now the hostagent PKI root.
Provisioning, rotation and trust distribution for that CA are unspecified and
likely belong with `PCDHostPool`. Do not implement rotation.

## 10. Settled — do not re-litigate

An agent proposing any of these in review is wrong; the decision was made with
context this document does not carry.

- This spec uses the community `mariadb-operator`, not the enterprise operator
- Bork is wrapped and fully hidden, not replaced. It will be refactored behind
  the adapter interface later.
- Single cluster only. No multi-cluster, no `targetCluster`, no `aim`.
- Bork3 is a hard prerequisite. No Bork1/2 support.
- `PCDInstallation` is both the tenancy record and the infra region spec. It is
  not being split.
- `PCDHostPool` is deferred, and MUST NOT be stubbed in `PCDRegion` meanwhile.
- One database technology. No provider abstraction, no external endpoints, no
  RDS.
- Prerequisites are applied directly, never through the backend.
