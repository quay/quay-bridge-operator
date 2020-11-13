module github.com/redhat-cop/quay-openshift-registry-operator

go 1.14

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/onsi/ginkgo v1.12.1 // indirect
	github.com/openshift/api v0.0.0-20200527184302-a843dc3262a0
	github.com/operator-framework/operator-lib v0.2.0
	github.com/operator-framework/operator-sdk v1.2.0 // indirect
	github.com/redhat-cop/operator-utils v0.0.0-20190530184149-66ee667a40b2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	k8s.io/code-generator v0.18.8
	k8s.io/gengo v0.0.0-20190128074634-0689ccc1d7d6
	k8s.io/kube-openapi v0.0.0-20200410163147-594e756bea31
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19 // indirect
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.3.0
	sigs.k8s.io/structured-merge-diff/v2 v2.0.1 // indirect
)

// Pinned to kubernetes-1.16.15 + OpenShift v4.3
replace (
	k8s.io/api => k8s.io/api v0.0.0-20200831051839-f197499901bd
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.16-rc.0
	k8s.io/client-go => k8s.io/client-go v0.16.15
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.5.11
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200527184302-a843dc3262a0
)

replace (
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.29.0
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410163147-594e756bea31
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.1.11-0.20190411181648-9d55346c2bde
)
