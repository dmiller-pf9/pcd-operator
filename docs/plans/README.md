# Plans

One file per milestone from `docs/SPEC.md` §8, named `NNN-<slug>.md` matching the
milestone number. Generate them with `/writing-plans` one milestone at a time —
not all at once. A plan written before its dependency milestone is implemented
will be wrong about the interfaces it calls.

## Generating a plan

```
/writing-plans

Read docs/SPEC.md and CLAUDE.md. Write the plan for milestone NNN only.
Cover exactly the requirement IDs listed for that milestone in §8 — no more.
Every task must follow the template in docs/plans/README.md.
Stop and ask if the milestone touches anything in §9.
```

## Task template

Superpowers asks for tasks sized for "an enthusiastic junior engineer with poor
taste, no judgement, no project context, and an aversion to testing." For this
repo that means every task carries its own requirement ID and its own failure
expectation, because the agent executing it will not have read this conversation.

```markdown
### Task N.M — <imperative summary>

**Requirement:** SPEC §X, ID `ABC-000`
**Files:** exact paths, created or modified
**Depends on:** task IDs, or "none"

**RED**
Test file: `<exact path>`
Test name: `<exact name>`
Test content: <complete code, not a sketch>
Run: `make test-one T=<name>`
Expected failure: `<the actual message, e.g. "undefined: MergeLayers">`

**GREEN**
Implementation: <complete code>
Run: `make test-one T=<name>` — expect pass

**VERIFY**
- [ ] `make verify` passes
- [ ] `make manifests generate` run (if api/ changed)
- [ ] Invariants checked: <the specific INV ids this task could violate>

**Do not:** <the specific wrong turn available here>
```

The `Expected failure` line matters more than it looks. It is the only mechanical
check that RED actually happened — an agent that skips the failing run cannot
produce the right message, and review can catch that.

The `Do not` line is where project context goes that a fresh subagent lacks.
Examples that have already bitten this design:

- *Do not* add a `targetCluster` field even though the backend has the concept (INV-6).
- *Do not* fold the phase enum into a second definition (INV-7).
- *Do not* validate the MariaDB `template` passthrough (INV-8).
- *Do not* pick a deadline value (§9-3) — take it as a required parameter.
- *Do not* make the underlay controller import the backend package (INV-2).

## Ordering

Milestones 001–009 need no cluster. Do them first, in order. They also contain
most of the decidable logic in the operator, so they are where TDD pays best.

010–013 need envtest and are slower. By the time you reach them, the merge
engine, size presets, fault classification and state machine are already tested
in isolation, so a controller test only has to prove wiring — not behaviour that
was already proven cheaply.

## Branching

One worktree per milestone via `/using-git-worktrees`, branch `milestone/NNN-<slug>`.
Merge to `main` only with `make verify` green. Milestones are dependency-ordered,
so do not run two in parallel unless §8 shows no dependency between them —
002 and nothing else can start until 001 lands.
