module kubevirt.io/containerized-data-importer

go 1.14

require (
	github.com/RHsyseng/operator-utils v0.0.0-20190906175225-942a3f9c85a9
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30
	github.com/aws/aws-sdk-go v1.15.77
	github.com/containers/image/v5 v5.5.1
	github.com/coreos/go-semver v0.3.0
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190911111923-ecfe977594f1 // indirect
	github.com/emicklei/go-restful v2.10.0+incompatible
	github.com/emicklei/go-restful-openapi v1.2.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.3
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/mock v1.3.1 // indirect
	github.com/golang/snappy v0.0.2
	github.com/google/uuid v1.1.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/kubernetes-csi/external-snapshotter/v2 v2.1.1
	github.com/mrnold/go-libnbd v1.4.1-cdi
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0
	github.com/openshift/client-go v0.0.0
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/openshift/library-go v0.0.0-20200821154433-215f00df72cc
	github.com/operator-framework/go-appr v0.0.0-20180917210448-f2aef88446f2 // indirect
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190725173916-b56e63a643cc
	github.com/operator-framework/operator-marketplace v0.0.0-20190617165322-1cbd32624349
	github.com/ovirt/go-ovirt v4.3.4+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.2.0
	github.com/rs/cors v1.7.0
	github.com/ulikunitz/xz v0.5.7
	github.com/vmware/govmomi v0.23.1
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f // indirect
	golang.org/x/sys v0.0.0-20200519105757-fe76b779f299
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/square/go-jose.v2 v2.3.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.0-rc.2
	k8s.io/apiextensions-apiserver v0.19.0-rc.2
	k8s.io/apimachinery v0.19.0-rc.2
	k8s.io/apiserver v0.19.0-rc.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.0.0
	k8s.io/code-generator v0.18.6
	k8s.io/klog/v2 v2.0.0
	k8s.io/kube-aggregator v0.18.6
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/utils v0.0.0-20200720150651-0bdb4ca86cbc // indirect
	kubevirt.io/controller-lifecycle-operator-sdk v0.1.2
	kubevirt.io/qe-tools v0.1.6
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200526144822-34f54f12813a
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
	github.com/openshift/library-go => github.com/mhenriks/library-go v0.0.0-20200804184258-4fc3a5379c7a
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a

	k8s.io/api => k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.6
	k8s.io/apiserver => k8s.io/apiserver v0.18.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.6
	k8s.io/client-go => k8s.io/client-go v0.18.6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.6
	k8s.io/code-generator => k8s.io/code-generator v0.18.6
	k8s.io/component-base => k8s.io/component-base v0.18.6
	k8s.io/cri-api => k8s.io/cri-api v0.18.6
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.6
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.6
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.6
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.6
	k8s.io/kubectl => k8s.io/kubectl v0.18.6
	k8s.io/kubelet => k8s.io/kubelet v0.18.6
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.6
	k8s.io/metrics => k8s.io/metrics v0.18.6
	k8s.io/node-api => k8s.io/node-api v0.18.6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.6
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.18.6
	k8s.io/sample-controller => k8s.io/sample-controller v0.18.6

	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.0
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)
