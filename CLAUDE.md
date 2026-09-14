# CLAUDE.md — pcd-operator

Operating instructions for any agent working in this repository. Read this before
any task. It is short on purpose; everything normative lives in `docs/SPEC.md`.

## What this is

A Kubernetes operator that is the unified installer and upgrade manager for PCD.
Three CRDs — `PCDUnderlay`, `PCDInstallation`, `PCDRegion` — reconciled by three
controllers in one Deployment.

- Design (why): `docs/DESIGN.md`
- Specification (what, normatively): `docs/SPEC.md`
- Plans (how, task by task): `docs/plans/`

`docs/SPEC.md` wins over `docs/DESIGN.md` on any conflict. If a plan conflicts
with the spec, stop and ask — do not reconcile it yourself.

## Non-negotiable invariants

These are the failure modes that do not announce themselves. A change that
violates one will usually pass its own tests. Check them on every task.

| ID | Invariant | How to check |
|---|---|---|
| INV-1 | No Bork identifier appears in any CRD field, status value, event, or error message reaching a user | `make lint-leak` — greps the api/ tree and all user-facing strings for a banned wordlist |
| INV-2 | Prerequisites are applied directly; they never traverse the Bork adapter | `internal/controller/underlay` must not import `internal/backend` |
| INV-3 | Rollback class C and D components never roll back automatically | unit test asserts `AutoRollback` is ignored for those classes |
| INV-4 | Every wait has a deadline | no `wait.PollUntilContextCancel` without a deadline; no unbounded `for` retry |
| INV-5 | `cert-manager` cannot be disabled | webhook rejects `components["cert-manager"].enabled: false` |
| INV-6 | Single cluster only — no kubeconfig, targetCluster, or aim anywhere in the API | `make lint-leak` covers these words too |
| INV-7 | `ComponentStatus.Phase` is defined exactly once | one `+kubebuilder:validation:Enum` for phase in the whole tree |
| INV-8 | MariaDB passthrough is never re-validated by us | no schema validation on `DatabaseBinding.Template` |

If a task appears to require violating one of these, that is a spec bug. Stop and
ask. Do not work around it.

## Stack and commands

Go + kubebuilder/controller-runtime. Everything runs through `make`.

```
make test           # unit tests only, no cluster needed
make test-envtest   # controller tests against envtest
make test-one T=... # single test, use this during red/green
make manifests      # regenerate CRDs after api/ changes
make generate       # regenerate deepcopy after api/ changes
make lint           # golangci-lint
make lint-leak      # invariant wordlist check (INV-1, INV-6)
make verify         # manifests + generate + lint + lint-leak + test; CI runs this
```

After **any** edit under `api/`, run `make manifests generate` in the same task.
A task that changes types without regenerating is incomplete and will fail review.

## TDD in this repo

RED-GREEN-REFACTOR is not optional. The order for every task:

1. Write the test. Run `make test-one T=<name>`. **Paste the failure output into
   your task notes.** A test that has not been observed failing does not count.
2. Write the minimum code to pass. Run it again. Observe green.
3. Refactor only with tests green.

What to test with what:

| Layer | Test type | Why |
|---|---|---|
| Override merge engine (§4.1 precedence) | pure table tests, no cluster | It is a pure function. Highest test value per minute in the repo — start here. |
| Release matrix resolution | pure table tests | Same. |
| Rollback class lookup, fault classification | pure table tests | Pure decision logic. |
| Webhooks | unit tests on the admission handler | No cluster needed. |
| Controllers | envtest | Needs a real API server for status/conditions. |
| Bork adapter | fake backend implementing the interface | Never hit a real Bork in tests. |

Do not write a controller test where a pure test would do. Most of the logic in
this operator is decidable without a cluster, and envtest is slow enough that
agents start skipping it.

## Definition of done for a task

- [ ] Test written first, observed failing, failure output recorded
- [ ] Implementation passes that test
- [ ] `make verify` passes
- [ ] `make manifests generate` run if `api/` changed
- [ ] No invariant from the table above violated
- [ ] No new open decision introduced (see below)

## Stop and ask

Superpowers will happily make a reasonable-looking choice where the spec is
silent. For this project those choices are expensive. **Stop and ask the human**
if a task requires you to:

- decide anything in `docs/SPEC.md` §9 (Open decisions)
- add a field to a CRD that the spec does not name
- introduce a new dependency
- change an invariant
- pick a timeout, deadline, or retry limit that the spec does not give
- decide a component's rollback safety class
- weaken a webhook rejection to make a test pass

"The plan told me to" is not sufficient. Plans are generated; the spec is agreed.

## Naming

- Kinds: `PCDUnderlay`, `PCDInstallation`, `PCDRegion`. Never "customer" for the
  kind — that name was retired. "Customer" refers only to the tenant an
  installation belongs to.
- The thing under everything is the *underlay*, not "infra" or "prereqs".
- Never write "Bork" in user-facing output. Internally, `internal/backend/bork`.
