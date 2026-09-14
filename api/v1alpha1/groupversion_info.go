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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the group and version for this API (SPEC API-001).
	GroupVersion = schema.GroupVersion{Group: "install.pcd.platform9.com", Version: "v1alpha1"}

	// SchemeBuilder registers this API's types with a runtime.Scheme.
	//
	// This deliberately uses apimachinery's runtime.SchemeBuilder rather than
	// controller-runtime's pkg/scheme.Builder, which is deprecated precisely
	// because it drags controller-runtime into an api package. Keeping this
	// package's dependencies to apimachinery alone means anything can import
	// the types cheaply.
	SchemeBuilder = runtime.NewSchemeBuilder()

	// AddToScheme adds this API's types to a runtime.Scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
