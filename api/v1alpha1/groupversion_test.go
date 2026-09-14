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
