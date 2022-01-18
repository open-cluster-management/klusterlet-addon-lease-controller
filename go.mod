module github.com/stolostron/klusterlet-addon-lease-controller

go 1.16

require (
	github.com/go-logr/logr v0.2.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/stolostron/library-go v0.0.0-20220112062416-536980fdb526
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	k8s.io/api v0.19.0
	k8s.io/apiextensions-apiserver v0.19.0 // indirect
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.3.0 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
)

replace github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0
