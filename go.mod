module github.com/exoscale/exoscale-cloud-controller-manager

go 1.14

require (
	github.com/deepmap/oapi-codegen v1.3.8 // indirect
	github.com/exoscale/egoscale v0.28.0
	github.com/gofrs/uuid v3.3.0+incompatible // indirect
	github.com/google/go-cmp v0.4.0
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/cloud-provider v0.19.3
	k8s.io/component-base v0.19.3
	k8s.io/klog/v2 v2.2.0
	k8s.io/kubernetes v1.19.3
)

replace (
	k8s.io/api => k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.3
	k8s.io/apiserver => k8s.io/apiserver v0.19.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.3
	k8s.io/client-go => k8s.io/client-go v0.19.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.3
	k8s.io/code-generator => k8s.io/code-generator v0.19.3
	k8s.io/component-base => k8s.io/component-base v0.19.3
	k8s.io/cri-api => k8s.io/cri-api v0.19.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.3
	k8s.io/klog => k8s.io/klog v1.0.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.3
	k8s.io/kubectl => k8s.io/kubectl v0.19.3
	k8s.io/kubelet => k8s.io/kubelet v0.19.3
	k8s.io/kubernetes => k8s.io/kubernetes v1.19.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.3
	k8s.io/metrics => k8s.io/metrics v0.19.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.3
)
