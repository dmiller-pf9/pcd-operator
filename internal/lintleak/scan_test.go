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
