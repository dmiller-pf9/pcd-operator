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
