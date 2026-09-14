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
