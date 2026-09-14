# Milestone 001 — Repo scaffolding, `make` targets, `lint-leak`, CI

> **For agentic workers:** REQUIRED SUB-SKILL: use `superpowers:subagent-driven-development`
> or `superpowers:executing-plans` to implement this plan task by task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the Go module, the `make` surface every later milestone depends
on, and a working `lint-leak` that mechanically enforces INV-1 and INV-6.

**Architecture:** `lint-leak` is a Go program, not a shell grep, because SPEC §3
INV-1 requires catching banned words in *string literals passed to an error or
event constructor* — that needs the Go AST, which `grep` cannot do. It is built as
a pure, table-tested library (`internal/lintleak`) with a thin `cmd/lint-leak`
wrapper, so the hard part is testable without running the binary.

**Tech Stack:** Go 1.25, `go/ast` + `go/parser` (stdlib), GNU-make, GitHub Actions.

**Requirements covered:** SPEC §3 INV-1, INV-6. SPEC §8 milestone 001.

---

## Decisions made outside the spec (confirm before executing)

These are not in `docs/SPEC.md`. They were agreed with the human before this plan
was written; an executing agent MUST NOT change them.

| Decision | Value | Why it is not a §9 open decision |
|---|---|---|
| Go module path | `platform9.com/pcd-operator` | Repo identity, no behavioural consequence |
| Go directive `1.25.0` | kept, despite controller-tools v0.22.0 needing >= 1.26.0 | Go downloads the newer toolchain on demand; go.mod states the language version this code needs, not the tools' |
| CI platform | GitHub Actions, `.github/workflows/ci.yml` | Milestone 001 says "CI" without naming one |

## Dependencies introduced

CLAUDE.md requires stopping before introducing a dependency. This milestone
introduces **none beyond the Go standard library**. The go.mod created in Task 1.1
declares only the module and Go version; the controller-runtime / Kubernetes
dependency set is introduced in milestone 002, where it is listed for approval.

## File structure

| File | Responsibility |
|---|---|
| `go.mod` | Module identity and Go version |
| `.gitignore` | Keep `bin/` and local envtest assets out of git |
| `hack/boilerplate.go.txt` | Licence header controller-gen stamps on generated files |
| `internal/lintleak/terms.go` | The banned-term table and why each term is banned |
| `internal/lintleak/scan.go` | Plain-text scan of a file tree |
| `internal/lintleak/ast.go` | AST scan for banned words in error/event constructor arguments |
| `internal/lintleak/scan_test.go` | Table tests for the text scan, incl. false-positive guards |
| `internal/lintleak/ast_test.go` | Table tests for the AST scan, driven by source strings not real code |
| `cmd/lint-leak/main.go` | CLI wrapper: run both scans, print findings, exit 1 on any |
| `Makefile` | Every command in CLAUDE.md "Stack and commands" |
| `.golangci.yml` | Linter configuration |
| `.github/workflows/ci.yml` | Runs `make verify` on push and PR |

---

### Task 1.1 — Bootstrap the Go module and repo skeleton

**Requirement:** SPEC §8 milestone 001 (prerequisite for everything else)
**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `hack/boilerplate.go.txt`
**Depends on:** none

**RED**

None. This task creates no logic, only module identity. TDD applies from Task 1.2
onward, where the first behaviour appears. Do not invent a test for `go.mod`.

**GREEN**

- [ ] **Step 1: Create `go.mod`**

```
module platform9.com/pcd-operator

go 1.25.0
```

- [ ] **Step 2: Create `.gitignore`**

```
# Claude Code worktrees live inside the repo; never track their contents
.claude/worktrees/

# Tool binaries installed by the Makefile
bin/
# envtest control-plane assets
testbin/
# Test and coverage output
cover.out
*.test
# Editor and OS noise
.DS_Store
.idea/
.vscode/
```

- [ ] **Step 3: Create `hack/boilerplate.go.txt`**

```
/*
Copyright 2026 Platform9 Systems, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

- [ ] **Step 4: Verify the module builds**

Run: `go build ./...`
Expected: no output, exit 0. (There are no packages yet; this confirms go.mod parses.)

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore hack/boilerplate.go.txt
git commit -m "chore: bootstrap go module platform9.com/pcd-operator"
```

**VERIFY**
- [ ] `go build ./...` exits 0
- [ ] `api/` not touched, so no `make manifests generate` needed
- [ ] Invariants checked: none reachable yet

**Do not:** add any `require` lines to `go.mod`. Dependencies arrive in milestone
002 where they are listed for human approval. Do not run `kubebuilder init` — it
would scaffold a `PROJECT` file, a `main.go`, and a Makefile that this plan
replaces, and its dependency set has not been approved.

---

### Task 1.2 — Banned-term table and plain-text tree scan

**Requirement:** SPEC §3 INV-1, INV-6
**Files:**
- Create: `internal/lintleak/terms.go`
- Create: `internal/lintleak/scan.go`
- Test: `internal/lintleak/scan_test.go`
**Depends on:** 1.1

**RED**

Test file: `internal/lintleak/scan_test.go`
Test name: `TestScanText_FlagsEveryBannedTerm`

```go
package lintleak

import "testing"

func TestScanText_FlagsEveryBannedTerm(t *testing.T) {
	cases := []struct {
		name string
		text string
		term string
	}{
		{"backend name", "// the bork adapter\n", "bork"},
		{"backend name capitalised", "// The Bork adapter\n", "bork"},
		{"usermgr", "Component: kplane-usermgr\n", "kplane"},
		{"namespace prefix", "namespace := \"x-acme\"\n", "x-namespace"},
		{"task state value", "if s == waiting_apps {\n", "waiting_apps"},
		{"install job", "job := du-install-acme\n", "du-install"},
		{"upgrade job", "job := du-upgrade-acme\n", "du-upgrade"},
		{"teardown job", "job := du-teardown-acme\n", "du-teardown"},
		{"raw state field", "record.task_state\n", "task_state"},
		{"protection flag", "metadata.dont_delete = true\n", "dont_delete"},
		{"bootstrap component", "Component: deccaxon\n", "deccaxon"},
		{"cluster selection", "TargetCluster string\n", "targetCluster"},
		{"cluster selection lower", "targetcluster: foo\n", "targetCluster"},
		{"kubeconfig", "KubeconfigPath string\n", "kubeconfig"},
		{"aim", "aim: management\n", "aim"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanText("fake.go", tc.text)
			if len(got) != 1 {
				t.Fatalf("ScanText(%q) returned %d findings, want 1: %+v", tc.text, len(got), got)
			}
			if got[0].Term != tc.term {
				t.Errorf("term = %q, want %q", got[0].Term, tc.term)
			}
			if got[0].Line != 1 {
				t.Errorf("line = %d, want 1", got[0].Line)
			}
		})
	}
}

func TestScanText_ReportsLineNumbers(t *testing.T) {
	text := "package v1alpha1\n\n// clean line\n// the bork adapter\n"
	got := ScanText("api/v1alpha1/thing.go", text)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Line != 4 {
		t.Errorf("line = %d, want 4", got[0].Line)
	}
	if got[0].Path != "api/v1alpha1/thing.go" {
		t.Errorf("path = %q, want %q", got[0].Path, "api/v1alpha1/thing.go")
	}
}
```

Run: `make test-one T=TestScanText` — the Makefile does not exist yet, so run
`go test ./internal/lintleak/ -run TestScanText -v` for this task only.

Expected failure:
```
internal/lintleak/scan_test.go:29:11: undefined: ScanText
FAIL	platform9.com/pcd-operator/internal/lintleak [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `internal/lintleak/terms.go`**

```go
// Package lintleak enforces INV-1 and INV-6 from docs/SPEC.md: no backend
// identifier and no cluster-selection concept may reach a user through the API
// surface, an error message, or an event.
package lintleak

import "regexp"

// Term is one banned word. Patterns are explicit regexps rather than plain
// substrings because two of the banned words collide with legitimate API
// vocabulary, and a linter that fires on valid code gets switched off. See
// TestScanText_DoesNotFlagLegitimateVocabulary for the collisions.
type Term struct {
	// Name is the stable identifier reported in findings and asserted in tests.
	Name string
	// Pattern matches the banned word.
	Pattern *regexp.Regexp
	// Why explains the ban, and is printed with the finding so the person who
	// tripped it does not have to go read the spec to understand it.
	Why string
}

// terms is the wordlist from SPEC §3, INV-1 and INV-6.
var terms = []Term{
	{
		Name:    "bork",
		Pattern: regexp.MustCompile(`(?i)bork`),
		Why:     "backend identifier; the backend is wrapped and hidden (SPEC §2.1)",
	},
	{
		Name:    "kplane",
		Pattern: regexp.MustCompile(`(?i)kplane`),
		Why:     "backend component name",
	},
	{
		// Case-sensitive and anchored on a word boundary followed by an
		// identifier character: the ban is on the backend's `x-<shortname>`
		// namespace prefix. A case-insensitive `x-` would match the legitimate
		// size enum values `X-Small` and `X-Large`, and an unanchored one would
		// match `nginx-ingress`.
		Name:    "x-namespace",
		Pattern: regexp.MustCompile(`\bx-[a-z0-9]`),
		Why:     "backend namespace prefix; namespace layout is adapter-internal",
	},
	{
		Name:    "waiting_apps",
		Pattern: regexp.MustCompile(`(?i)waiting_apps`),
		Why:     "raw backend task state; translate to status.phase instead",
	},
	{
		Name:    "du-install",
		Pattern: regexp.MustCompile(`(?i)du-install`),
		Why:     "backend job name; use status.activeOperation.type Install",
	},
	{
		Name:    "du-upgrade",
		Pattern: regexp.MustCompile(`(?i)du-upgrade`),
		Why:     "backend job name; use status.activeOperation.type Upgrade",
	},
	{
		Name:    "du-teardown",
		Pattern: regexp.MustCompile(`(?i)du-teardown`),
		Why:     "backend job name; use status.activeOperation.type Teardown",
	},
	{
		Name:    "task_state",
		Pattern: regexp.MustCompile(`(?i)task_state`),
		Why:     "raw backend state field; translate to status.phase",
	},
	{
		Name:    "dont_delete",
		Pattern: regexp.MustCompile(`(?i)dont_delete`),
		Why:     "backend flag; use spec.protection.preventDeletion",
	},
	{
		Name:    "deccaxon",
		Pattern: regexp.MustCompile(`(?i)deccaxon`),
		Why:     "backend bootstrap component name",
	},
	{
		Name:    "targetCluster",
		Pattern: regexp.MustCompile(`(?i)targetcluster`),
		Why:     "cluster selection; single cluster only (INV-6)",
	},
	{
		Name:    "kubeconfig",
		Pattern: regexp.MustCompile(`(?i)kubeconfig`),
		Why:     "cluster selection; backend calls always use in-cluster config (INV-6)",
	},
	{
		// Word-anchored: an unanchored `aim` matches `PersistentVolumeClaim`,
		// which appears legitimately in the database backup passthrough docs.
		Name:    "aim",
		Pattern: regexp.MustCompile(`(?i)\baim\b`),
		Why:     "cluster placement concept; single cluster only (INV-6)",
	},
}

// Terms returns the banned-term table. Exported so a test can assert the list
// matches SPEC §3 without reaching into package internals.
func Terms() []Term {
	out := make([]Term, len(terms))
	copy(out, terms)
	return out
}
```

- [ ] **Step 3: Create `internal/lintleak/scan.go`**

```go
package lintleak

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Finding is one banned word in one place.
type Finding struct {
	Path string
	Line int
	Term string
	Why  string
	Text string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: banned term %q (%s): %s", f.Path, f.Line, f.Term, f.Why, f.Text)
}

// ScanText reports every banned term in text. Path is used only for reporting.
func ScanText(path, text string) []Finding {
	var found []Finding
	for i, line := range strings.Split(text, "\n") {
		for _, term := range terms {
			if term.Pattern.MatchString(line) {
				found = append(found, Finding{
					Path: path,
					Line: i + 1,
					Term: term.Name,
					Why:  term.Why,
					Text: strings.TrimSpace(line),
				})
			}
		}
	}
	return found
}

// ScanTree reports every banned term in every regular file under root.
// Generated output directories are deliberately not scanned by callers — see
// the comment on ScanAll in cmd/lint-leak.
//
// A missing root is not an error: the api/ tree does not exist until milestone
// 002, and milestone 001 must still pass its own verification.
func ScanTree(root string) ([]Finding, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	var found []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if skipFile(d.Name()) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found = append(found, ScanText(path, string(b))...)
		return nil
	})
	return found, err
}

// skipDir excludes trees whose contents are not ours to police. `.claude`
// matters because worktrees are created at .claude/worktrees/ inside the repo;
// without it the scan walks a nested copy of the whole tree.
func skipDir(name string) bool {
	switch name {
	case ".git", ".claude", "bin", "testbin", "vendor", "testdata":
		return true
	}
	return false
}

// skipFile excludes test sources. Test fixtures legitimately contain banned
// words — that is how the linter proves it catches them — and no string in a
// test reaches a user.
func skipFile(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}
```

- [ ] **Step 4: Run the test and observe green**

Run: `go test ./internal/lintleak/ -run TestScanText -v`
Expected: `PASS` — 15 subtests in `TestScanText_FlagsEveryBannedTerm`, plus
`TestScanText_ReportsLineNumbers`.

- [ ] **Step 5: Commit**

```bash
git add internal/lintleak/terms.go internal/lintleak/scan.go internal/lintleak/scan_test.go
git commit -m "feat(lint-leak): banned-term table and text tree scan (INV-1, INV-6)"
```

**VERIFY**
- [ ] `go test ./internal/lintleak/` passes
- [ ] `api/` not touched, no regeneration needed
- [ ] Invariants checked: this task *is* the INV-1 / INV-6 mechanism

**Do not:** implement this as a shell script around `grep`. INV-1 covers string
literals passed to error and event constructors, which `grep` cannot identify, and
a half-implementation that only greps will be believed to be complete.
**Do not:** make the `x-namespace` pattern case-insensitive — it will match the
`X-Small` and `X-Large` size enum values that milestone 002 adds, and the first
person to hit that will delete the term from the list rather than debug it.

---

### Task 1.3 — Guard the two false positives the wordlist creates

**Requirement:** SPEC §3 INV-1, INV-6 (the linter must be trustworthy enough to keep)
**Files:**
- Modify: `internal/lintleak/scan_test.go`
**Depends on:** 1.2

**RED**

Test file: `internal/lintleak/scan_test.go`
Test name: `TestScanText_DoesNotFlagLegitimateVocabulary`

```go
func TestScanText_DoesNotFlagLegitimateVocabulary(t *testing.T) {
	// Every string here is real API vocabulary that milestone 002 introduces or
	// that the design already uses. A finding on any of them is a linter bug,
	// not a spec violation.
	clean := []string{
		`// +kubebuilder:validation:Enum=X-Small;Small;Medium;Large;X-Large`,
		`SizeXSmall PCDSize = "X-Small"`,
		`SizeXLarge PCDSize = "X-Large"`,
		`// Storage is passed through to spec.storage (s3 / persistentVolumeClaim / volume)`,
		`PersistentVolumeClaim corev1.PersistentVolumeClaim`,
		`Components map[string]ComponentSpec `,
		`// ingress-nginx is the default ingress controller`,
		`IngressClass string `,
		`// MaxUserConnections per service user`,
		`// domain and maintenance windows are unaffected`,
		`MaintenanceWindow struct {`,
	}

	for _, line := range clean {
		t.Run(line, func(t *testing.T) {
			if got := ScanText("api/v1alpha1/x.go", line); len(got) != 0 {
				t.Errorf("ScanText(%q) flagged %+v, want no findings", line, got)
			}
		})
	}
}
```

Run: `go test ./internal/lintleak/ -run TestScanText_DoesNotFlagLegitimateVocabulary -v`

Expected failure — if Task 1.2 was implemented with naive substring matching this
fails with findings on `X-Small`, `X-Large` and `persistentVolumeClaim`:
```
--- FAIL: TestScanText_DoesNotFlagLegitimateVocabulary/SizeXSmall_PCDSize_=_"X-Small"
    scan_test.go:NN: ScanText(...) flagged [{... Term:x-namespace ...}], want no findings
```

If Task 1.2's patterns were written as specified, this test passes immediately.
**That is an acceptable outcome for this task only** — the test is a regression
guard on a decision already made, not a new behaviour. Record in your task notes
which of the two happened. If it passed first time, prove the guard works by
temporarily changing the `x-namespace` pattern to `(?i)x-`, observing the failure,
and reverting.

**GREEN**

- [ ] **Step 1: Run the test. If it fails, fix the patterns in `terms.go` to match Task 1.2 exactly**
- [ ] **Step 2: Run the test and observe green**

Run: `go test ./internal/lintleak/ -v`
Expected: `PASS`

- [ ] **Step 3: Commit**

```bash
git add internal/lintleak/scan_test.go internal/lintleak/terms.go
git commit -m "test(lint-leak): guard X-Small and PersistentVolumeClaim false positives"
```

**VERIFY**
- [ ] `go test ./internal/lintleak/` passes
- [ ] Invariants checked: INV-1, INV-6 — this test is what stops the wordlist
      being weakened later to silence a false positive

**Do not:** delete a term from the list to make this pass. The terms are from
SPEC §3 and removing one is a spec change (CLAUDE.md "Stop and ask").

---

### Task 1.4 — AST scan of error and event constructor arguments

**Requirement:** SPEC §3 INV-1 ("...or in any string literal passed to an error or event constructor")
**Files:**
- Create: `internal/lintleak/ast.go`
- Test: `internal/lintleak/ast_test.go`
**Depends on:** 1.2

**RED**

Test file: `internal/lintleak/ast_test.go`
Test name: `TestScanGoSource`

```go
package lintleak

import "testing"

func TestScanGoSource_FlagsErrorAndEventConstructors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "fmt.Errorf",
			src: `package p
import "fmt"
func f() error { return fmt.Errorf("the bork backend rejected the request") }
`,
			want: 1,
		},
		{
			name: "errors.New",
			src: `package p
import "errors"
func f() error { return errors.New("task_state was not ready") }
`,
			want: 1,
		},
		{
			name: "recorder.Event",
			src: `package p
func f(r Recorder, o Object) { r.Event(o, "Warning", "Failed", "du-install-acme did not complete") }
`,
			want: 1,
		},
		{
			name: "recorder.Eventf",
			src: `package p
func f(r Recorder, o Object) { r.Eventf(o, "Warning", "Failed", "namespace %s missing", "x-acme") }
`,
			want: 1,
		},
		{
			name: "wrapped error, banned word in the format string",
			src: `package p
import "fmt"
func f(err error) error { return fmt.Errorf("kplane-usermgr call failed: %w", err) }
`,
			want: 1,
		},
		{
			name: "clean error is not flagged",
			src: `package p
import "fmt"
func f() error { return fmt.Errorf("installation is not yet Available") }
`,
			want: 0,
		},
		{
			name: "banned word outside a constructor is not flagged here",
			src: `package p
const internalPackage = "internal/backend/bork"
`,
			want: 0,
		},
		{
			name: "non-constructor call with a banned literal is not flagged",
			src: `package p
func f(c Client) { c.Get("x-acme") }
`,
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ScanGoSource("fake.go", tc.src)
			if err != nil {
				t.Fatalf("ScanGoSource returned error: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("got %d findings, want %d: %+v", len(got), tc.want, got)
			}
		})
	}
}

func TestScanGoSource_ReportsTheConstructorLine(t *testing.T) {
	src := `package p

import "fmt"

func f() error {
	return fmt.Errorf("the bork backend rejected the request")
}
`
	got, err := ScanGoSource("internal/controller/thing.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Line != 6 {
		t.Errorf("line = %d, want 6", got[0].Line)
	}
	if got[0].Term != "bork" {
		t.Errorf("term = %q, want %q", got[0].Term, "bork")
	}
}
```

Run: `go test ./internal/lintleak/ -run TestScanGoSource -v`

Expected failure:
```
internal/lintleak/ast_test.go:NN:15: undefined: ScanGoSource
FAIL	platform9.com/pcd-operator/internal/lintleak [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Create `internal/lintleak/ast.go`**

```go
package lintleak

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// isLeakyConstructor reports whether a call is one whose string arguments reach
// a user. Matching is by callee name rather than by resolved type: the linter
// runs on source that may not compile yet, so it cannot depend on type
// information, and the name set below is small enough that a false match is
// cheaper than a miss.
func isLeakyConstructor(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	name := sel.Sel.Name

	// Event recorder methods, on any receiver.
	switch name {
	case "Event", "Eventf", "AnnotatedEventf":
		return true
	}

	// Error constructors, qualified by package name.
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch {
	case pkg.Name == "fmt" && (name == "Errorf" || name == "Sprintf"):
		return true
	case pkg.Name == "errors" && name == "New":
		return true
	}
	return false
}

// ScanGoSource reports banned terms in string literals passed to error and event
// constructors in src. It takes source text rather than a path so tests can use
// fixture strings — a fixture file containing banned words would otherwise be
// caught by the linter's own tree scan.
func ScanGoSource(path, src string) ([]Finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var found []Finding
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isLeakyConstructor(call) {
			return true
		}
		for _, arg := range call.Args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil {
				text = lit.Value
			}
			line := fset.Position(lit.Pos()).Line
			for _, term := range terms {
				if term.Pattern.MatchString(text) {
					found = append(found, Finding{
						Path: path,
						Line: line,
						Term: term.Name,
						Why:  term.Why,
						Text: text,
					})
				}
			}
		}
		return true
	})
	return found, nil
}

// ScanGoTree runs ScanGoSource over every non-test Go file under root.
func ScanGoTree(root string) ([]Finding, error) {
	var found []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || skipFile(d.Name()) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := ScanGoSource(path, string(b))
		if err != nil {
			// A file that does not parse is a compile error, not a leak. Let
			// `go build` report it rather than failing here with a worse message.
			return nil
		}
		found = append(found, f...)
		return nil
	})
	return found, err
}
```

- [ ] **Step 3: Run the test and observe green**

Run: `go test ./internal/lintleak/ -v`
Expected: `PASS`

- [ ] **Step 4: Commit**

```bash
git add internal/lintleak/ast.go internal/lintleak/ast_test.go
git commit -m "feat(lint-leak): flag banned terms in error and event constructors (INV-1)"
```

**VERIFY**
- [ ] `go test ./internal/lintleak/` passes
- [ ] Invariants checked: INV-1

**Do not:** use `go/types` or `golang.org/x/tools/go/packages` to resolve the
receiver of `Event`. That is a new dependency (CLAUDE.md "Stop and ask") and it
requires the tree to compile, which it will not during a red phase.

---

### Task 1.5 — `cmd/lint-leak` and the `lint-leak` make target

**Requirement:** SPEC §3 INV-1, INV-6 — `make lint-leak` must exist and fail on a violation
**Files:**
- Create: `cmd/lint-leak/main.go`
- Create: `Makefile`
- Test: `internal/lintleak/scan_test.go` (append)
**Depends on:** 1.3, 1.4

**RED**

Test file: `internal/lintleak/scan_test.go`
Test name: `TestScanAll_IsCleanOnThisRepo`

```go
func TestScanAll_IsCleanOnThisRepo(t *testing.T) {
	// Runs the exact scan `make lint-leak` runs, against the repo root, so a
	// leak fails `go test` as well as the dedicated target.
	found, err := ScanAll("../..")
	if err != nil {
		t.Fatalf("ScanAll returned error: %v", err)
	}
	for _, f := range found {
		t.Errorf("%s", f)
	}
}
```

Run: `go test ./internal/lintleak/ -run TestScanAll -v`

Expected failure:
```
internal/lintleak/scan_test.go:NN:17: undefined: ScanAll
FAIL	platform9.com/pcd-operator/internal/lintleak [build failed]
```

**GREEN**

- [ ] **Step 1: Write the failing test above, run it, paste the failure into your task notes**

- [ ] **Step 2: Append `ScanAll` to `internal/lintleak/scan.go`**

```go
// ScanAll is the whole of `make lint-leak`: the API tree is scanned in full,
// and the rest of the module is scanned for banned words in error and event
// constructors.
//
// config/crd/ is deliberately not scanned. Generated CRD manifests contain
// `x-kubernetes-preserve-unknown-fields`, `x-kubernetes-list-type` and friends,
// which the x-namespace term legitimately matches. Those files are generated
// from api/, which *is* scanned, so nothing is lost by excluding them.
func ScanAll(root string) ([]Finding, error) {
	apiFindings, err := ScanTree(filepath.Join(root, "api"))
	if err != nil {
		return nil, err
	}
	goFindings, err := ScanGoTree(root)
	if err != nil {
		return nil, err
	}
	return append(apiFindings, goFindings...), nil
}
```

- [ ] **Step 3: Create `cmd/lint-leak/main.go`**

```go
// Command lint-leak enforces INV-1 and INV-6 from docs/SPEC.md.
package main

import (
	"flag"
	"fmt"
	"os"

	"platform9.com/pcd-operator/internal/lintleak"
)

func main() {
	root := flag.String("root", ".", "module root to scan")
	flag.Parse()

	found, err := lintleak.ScanAll(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-leak: %v\n", err)
		os.Exit(2)
	}
	for _, f := range found {
		fmt.Fprintln(os.Stderr, f.String())
	}
	if len(found) > 0 {
		fmt.Fprintf(os.Stderr, "\nlint-leak: %d violation(s). See docs/SPEC.md §3 INV-1 and INV-6.\n", len(found))
		os.Exit(1)
	}
	fmt.Println("lint-leak: clean")
}
```

- [ ] **Step 4: Create `Makefile` with the `lint-leak` target only**

The remaining targets are added in Task 1.6. Creating the file here keeps
`make lint-leak` runnable at the end of this task.

```make
SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

##@ Invariants

.PHONY: lint-leak
lint-leak: ## Enforce INV-1 and INV-6: no backend or cluster-selection identifier reaches a user
	go run ./cmd/lint-leak -root .

##@ Help

.PHONY: help
help: ## Print this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
```

- [ ] **Step 5: Run the test and the target, observe green**

Run: `go test ./internal/lintleak/ -v`
Expected: `PASS`

Run: `make lint-leak`
Expected: `lint-leak: clean`

- [ ] **Step 6: Prove the target actually fails on a violation**

```bash
mkdir -p api/v1alpha1
printf 'package v1alpha1\n\n// TargetCluster selects the cluster.\ntype Leak struct{}\n' > api/v1alpha1/leak.go
make lint-leak; echo "exit=$?"
rm -rf api
```
Expected: a finding naming `targetCluster`, and a non-zero exit. Note that
`make` reports `Error 1` from the recipe but itself exits **2** — the binary
exits 1 and make wraps it. Paste this into your task notes; it is the only
evidence the target is wired up. `go test ./internal/lintleak/ -run TestScanAll`
fails on the same fixture, which is the second half of the evidence.

- [ ] **Step 7: Commit**

```bash
git add cmd/lint-leak/main.go internal/lintleak/scan.go internal/lintleak/scan_test.go Makefile
git commit -m "feat(lint-leak): cmd wrapper and make target"
```

**VERIFY**
- [ ] `make lint-leak` prints `lint-leak: clean` and exits 0
- [ ] `make lint-leak` exits 1 on the injected violation from Step 6
- [ ] Invariants checked: INV-1, INV-6

**Do not:** leave the injected `api/v1alpha1/leak.go` behind. Step 6 removes it.

---

### Task 1.6 — The rest of the `make` surface

**Requirement:** CLAUDE.md "Stack and commands"; SPEC §8 milestone 001
**Files:**
- Modify: `Makefile`
- Create: `.golangci.yml`
**Depends on:** 1.5

**RED**

None. A Makefile is a command surface, not logic; its test is that each target
runs. Step 5 below is that check, and it is mandatory.

**GREEN**

- [ ] **Step 1: Replace `Makefile` with the full version**

```make
SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# ---- pinned tool versions -------------------------------------------------
# Bump deliberately, never implicitly. `go install ...@latest` in a Makefile
# makes CI non-reproducible.
CONTROLLER_TOOLS_VERSION ?= v0.22.0
GOLANGCI_LINT_VERSION    ?= v2.13.2
SETUP_ENVTEST_VERSION    ?= v0.25.1
# The control-plane version envtest downloads. Confirmed against the available
# envtest releases when milestone 010 adds the first envtest test.
ENVTEST_K8S_VERSION      ?= 1.34.1

LOCALBIN         ?= $(shell pwd)/bin
CONTROLLER_GEN   ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT    ?= $(LOCALBIN)/golangci-lint
SETUP_ENVTEST    ?= $(LOCALBIN)/setup-envtest

# T is the -run pattern for `make test-one`.
T ?=

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

##@ Code generation

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Regenerate CRD manifests from api/
	@if [ -z "$$(find api -name '*.go' 2>/dev/null)" ]; then 		echo "manifests: api/ has no Go types yet, nothing to generate"; 	else 		$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases; 	fi

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Regenerate deepcopy functions from api/
	@if [ -z "$$(find api -name '*.go' 2>/dev/null)" ]; then 		echo "generate: api/ has no Go types yet, nothing to generate"; 	else 		$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."; 	fi

##@ Test

.PHONY: test
test: ## Unit tests only, no cluster needed
	go test ./... -count=1

.PHONY: test-one
test-one: ## Run a single test: make test-one T=TestName
	@test -n "$(T)" || { echo "usage: make test-one T=TestName"; exit 2; }
	go test ./... -run '$(T)' -count=1 -v

.PHONY: test-envtest
test-envtest: $(SETUP_ENVTEST) ## Controller tests against envtest
	KUBEBUILDER_ASSETS="$$($(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./... -count=1 -tags envtest

##@ Lint

.PHONY: lint
lint: $(GOLANGCI_LINT) ## golangci-lint
	$(GOLANGCI_LINT) run

.PHONY: lint-leak
lint-leak: ## Enforce INV-1 and INV-6: no backend or cluster-selection identifier reaches a user
	go run ./cmd/lint-leak -root .

##@ Verify

.PHONY: verify
verify: manifests generate check-generated lint lint-leak test ## Everything CI runs

.PHONY: check-generated
check-generated: ## Fail if manifests/generate produced uncommitted changes
	@if ! git diff --quiet -- api config; then \
		echo "ERROR: generated files are out of date. Run 'make manifests generate' and commit the result."; \
		git --no-pager diff --stat -- api config; \
		exit 1; \
	fi

##@ Tools

# Order-only prerequisites (`| $(LOCALBIN)`). With a normal prerequisite,
# installing one tool bumps bin/'s mtime and invalidates the other two, so every
# `make verify` reinstalls them.
$(CONTROLLER_GEN): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

$(GOLANGCI_LINT): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(SETUP_ENVTEST): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)

##@ Help

.PHONY: help
help: ## Print this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
```

- [ ] **Step 2: Create `.golangci.yml`**

```yaml
version: "2"

run:
  timeout: 5m

linters:
  default: standard
  exclusions:
    generated: lax
    paths:
      - zz_generated.*\.go$
```

- [ ] **Step 3: Install the tools and confirm they resolve**

```bash
make manifests   # installs controller-gen as a side effect
make lint        # installs golangci-lint as a side effect
./bin/controller-gen --version
./bin/golangci-lint version
```
Expected: `Version: v0.22.0` and a golangci-lint v2.13.2 banner.

The tool targets are absolute paths (`$(LOCALBIN)/controller-gen`), so
`make bin/controller-gen` will **not** match them — invoke the target that needs
the tool instead, as above.

- [ ] **Step 4: Run each target**

```bash
make lint-leak
make test
make lint
```
Expected: `lint-leak: clean`; `go test` reports `ok platform9.com/pcd-operator/internal/lintleak`
and `no test files` for `cmd/lint-leak`; `golangci-lint run` exits 0 with no output.

Note: `make manifests` and `make generate` print
`api/ has no Go types yet, nothing to generate` until milestone 002 creates the
types. The guard exists because `controller-gen` errors on a path set that
matches no package, which would make `make verify` fail for the whole of
milestone 001.

- [ ] **Step 5: Run `make verify` end to end**

Run: `make verify`
Expected: exit 0. Paste the output into your task notes.

- [ ] **Step 6: Commit**

```bash
git add Makefile .golangci.yml
git commit -m "build: make targets for test, generate, lint and verify"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `api/` not touched by this task
- [ ] Invariants checked: none new; `verify` is what enforces the rest from here

**Do not:** use `@latest` for any tool version. **Do not** add `-race` or
coverage flags to `test` — milestone 001 is not the place to decide the test
performance budget, and a slower `make test` is how agents start skipping it.

---

### Task 1.7 — GitHub Actions CI

**Requirement:** SPEC §8 milestone 001 ("...and CI")
**Files:**
- Create: `.github/workflows/ci.yml`
**Depends on:** 1.6

**RED**

None. CI configuration is verified by running the same command locally; there is
no remote to push to yet.

**GREEN**

- [ ] **Step 1: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  verify:
    name: make verify
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Cache tool binaries
        uses: actions/cache@v4
        with:
          path: bin
          key: tools-${{ runner.os }}-${{ hashFiles('Makefile') }}

      - name: make verify
        run: make verify
```

- [ ] **Step 2: Confirm the workflow parses**

Use whichever of these is installed — `pyyaml` is often absent on macOS:

```bash
yq '.jobs.verify.steps[] | .uses // .name' .github/workflows/ci.yml
# or
ruby -ryaml -e 'YAML.load_file(".github/workflows/ci.yml"); puts "ci.yml parses"'
```
Expected: the four step identifiers, or `ci.yml parses`

- [ ] **Step 3: Confirm the command CI runs actually passes locally**

Run: `make verify`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run make verify on push and pull request"
```

**VERIFY**
- [ ] `make verify` passes
- [ ] `.github/workflows/ci.yml` parses as YAML
- [ ] Invariants checked: none

**Do not:** add a second job that runs `make test-envtest`. No envtest test exists
until milestone 010, and a job that passes because it ran nothing is worse than no
job. Add it in 010, with the test.
**Do not:** pin action versions to `@main` or a floating tag.

---

## Milestone exit criteria

- [ ] `make verify` passes from a clean checkout
- [ ] `make lint-leak` exits 1 on an injected `targetCluster` field in `api/`
      (Task 1.5 Step 6) and 0 otherwise
- [ ] Every command in the CLAUDE.md "Stack and commands" block exists as a target
- [ ] No dependency beyond the Go standard library in `go.mod`
- [ ] Milestone 002 can start: `api/` is empty, `make manifests generate` are wired
