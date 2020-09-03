module github.com/open-cluster-management/klusterlet-addon-lease-controller

go 1.15

require (
	github.com/go-logr/logr v0.2.1-0.20200730175230-ee2de8da5be6
	github.com/google/martian v2.1.0+incompatible // indirect
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/open-cluster-management/library-go v0.0.0-20200828173847-299c21e6c3fc
	// github.com/onsi/ginkgo v1.12.1
	// github.com/onsi/gomega v1.10.1
	github.com/openshift/library-go v0.0.0-20200831114015-2ab0c61c15de
	github.com/operator-framework/operator-sdk v1.0.0 // indirect
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.6.2
)

replace github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0
