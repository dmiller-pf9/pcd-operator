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
